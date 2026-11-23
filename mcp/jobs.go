package mcp

import (
	"fmt"
	"sort"
	"strings"
	"sync"
	"syscall"
	"time"
)

// Job é um comando que continua rodando em segundo plano. O agente recebe o id
// e consulta a saída quando quiser (job_output), lista (job_list) ou encerra
// (job_stop); o usuário é avisado na tela quando ele termina.
type Job struct {
	ID       int
	Command  string
	PID      int
	LogPath  string
	Started  time.Time
	Ended    time.Time
	Done     bool
	ExitErr  string
	notified bool

	EmSegundoPlano bool
}

var jobs = struct {
	sync.Mutex
	next int
	byID map[int]*Job
}{next: 1, byID: map[int]*Job{}}

func novoJob(cmd string, pid int, log string) *Job {
	jobs.Lock()
	defer jobs.Unlock()
	j := &Job{ID: jobs.next, Command: cmd, PID: pid, LogPath: log, Started: time.Now()}
	jobs.next++
	jobs.byID[j.ID] = j
	return j
}

func (j *Job) termina(err error) {
	jobs.Lock()
	defer jobs.Unlock()
	j.Done = true
	j.Ended = time.Now()
	if err != nil {
		j.ExitErr = err.Error()
	}
}

// esquece remove um job que terminou dentro do tempo de espera normal: ele não
// foi para segundo plano, não há o que consultar depois.
func (j *Job) esquece() {
	jobs.Lock()
	defer jobs.Unlock()
	delete(jobs.byID, j.ID)
}

// marcaSegundoPlano diz que o comando passou do tempo e agora é um job: só a
// partir daí o término gera aviso.
func (j *Job) marcaSegundoPlano() {
	jobs.Lock()
	defer jobs.Unlock()
	j.notified = false
	j.EmSegundoPlano = true
}

// JobsConcluidos devolve, uma única vez cada, os jobs de segundo plano que
// acabaram de terminar. O app chama a cada segundo para avisar na tela.
func JobsConcluidos() []Job {
	jobs.Lock()
	defer jobs.Unlock()
	var out []Job
	for _, j := range jobs.byID {
		if j.Done && j.EmSegundoPlano && !j.notified {
			j.notified = true
			out = append(out, *j)
		}
	}
	sort.Slice(out, func(a, b int) bool { return out[a].ID < out[b].ID })
	return out
}

func resumoDoJob(j Job) string {
	estado := "rodando"
	if j.Done {
		estado = "terminou"
		if j.ExitErr != "" {
			estado = "terminou com erro (" + j.ExitErr + ")"
		}
	}
	return fmt.Sprintf("#%d [%s] pid %d, %s: %s", j.ID, estado, j.PID,
		time.Since(j.Started).Round(time.Second), j.Command)
}

func (s *Server) jobList(req Request) Result {
	jobs.Lock()
	var lista []Job
	for _, j := range jobs.byID {
		if j.EmSegundoPlano {
			lista = append(lista, *j)
		}
	}
	jobs.Unlock()
	if len(lista) == 0 {
		return Result{Tool: req.Tool, Status: "success", Content: "Nenhum comando em segundo plano."}
	}
	sort.Slice(lista, func(a, b int) bool { return lista[a].ID < lista[b].ID })
	var sb strings.Builder
	for _, j := range lista {
		sb.WriteString(resumoDoJob(j) + "\n")
	}
	return Result{Tool: req.Tool, Status: "success", Content: strings.TrimSpace(sb.String())}
}

func achaJob(id int) (Job, bool) {
	jobs.Lock()
	defer jobs.Unlock()
	j, ok := jobs.byID[id]
	if !ok {
		return Job{}, false
	}
	return *j, true
}

func (s *Server) jobOutput(req Request) Result {
	j, ok := achaJob(req.JobID)
	if !ok {
		return Result{Tool: req.Tool, Status: "error", Error: fmt.Sprintf("não existe o job %d (use job_list)", req.JobID)}
	}
	n := req.Lines
	if n <= 0 {
		n = 80
	}
	saida := lerFimDoLog(j.LogPath, 64000)
	linhas := strings.Split(strings.TrimRight(saida, "\n"), "\n")
	if len(linhas) > n {
		linhas = linhas[len(linhas)-n:]
	}
	corpo := strings.Join(linhas, "\n")
	if strings.TrimSpace(corpo) == "" {
		corpo = "(sem saída ainda)"
	}
	return Result{Tool: req.Tool, Status: "success",
		Content: resumoDoJob(j) + "\nÚltimas linhas da saída:\n" + corpo}
}

func (s *Server) jobStop(req Request) Result {
	j, ok := achaJob(req.JobID)
	if !ok {
		return Result{Tool: req.Tool, Status: "error", Error: fmt.Sprintf("não existe o job %d", req.JobID)}
	}
	if j.Done {
		return Result{Tool: req.Tool, Status: "success", Content: "O job já tinha terminado: " + resumoDoJob(j)}
	}
	if err := syscall.Kill(-j.PID, syscall.SIGTERM); err != nil {
		return Result{Tool: req.Tool, Status: "error", Error: "não consegui encerrar: " + err.Error()}
	}
	return Result{Tool: req.Tool, Status: "success", Content: fmt.Sprintf("Enviei SIGTERM ao job #%d (%s).", j.ID, j.Command)}
}

// PararJobs encerra tudo que ficou em segundo plano (ao sair do Amazzo).
func PararJobs() {
	jobs.Lock()
	defer jobs.Unlock()
	for _, j := range jobs.byID {
		if !j.Done && j.PID > 0 {
			_ = syscall.Kill(-j.PID, syscall.SIGTERM)
		}
	}
}
