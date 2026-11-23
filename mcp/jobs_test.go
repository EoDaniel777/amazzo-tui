package mcp

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"amazzo/filesystem"
)

func servidorTemp(t *testing.T) *Server {
	t.Helper()
	fs, err := filesystem.NewManager(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	return NewServer(fs)
}

func TestJobEmSegundoPlano(t *testing.T) {
	s := servidorTemp(t)
	r, _ := s.Handle(context.Background(), []Request{{Tool: "execute_command", Command: "echo ola; sleep 6; echo fim", Background: true}})
	if !strings.Contains(r[0].Content, "job #") {
		t.Fatalf("sem job: %s", r[0].Content)
	}
	id := 0
	for _, j := range jobs.byID {
		if j.EmSegundoPlano && strings.Contains(j.Command, "echo ola") {
			id = j.ID
		}
	}
	if id == 0 {
		t.Fatal("job não registrado")
	}
	out := s.jobOutput(Request{Tool: "job_output", JobID: id})
	if !strings.Contains(out.Content, "ola") || !strings.Contains(out.Content, "rodando") {
		t.Fatalf("saída: %s", out.Content)
	}
	for i := 0; i < 100; i++ {
		if j, _ := achaJob(id); j.Done {
			break
		}
		time.Sleep(100 * time.Millisecond)
	}
	if c := JobsConcluidos(); len(c) == 0 {
		t.Fatal("término não avisado")
	}
	if c := JobsConcluidos(); len(c) != 0 {
		t.Fatal("aviso repetido")
	}
}

func TestChecagemDeSintaxeGo(t *testing.T) {
	s := servidorTemp(t)
	r, _ := s.Handle(context.Background(), []Request{{Tool: "write_file", Path: "a.go", Content: "func x() {}\n"}})
	if !strings.Contains(r[0].Content, "ATENÇÃO") {
		t.Fatalf("não avisou arquivo Go quebrado: %s", r[0].Content)
	}
	r, _ = s.Handle(context.Background(), []Request{{Tool: "write_file", Path: "b.go", Content: "package b\n"}})
	if strings.Contains(r[0].Content, "ATENÇÃO") {
		t.Fatalf("falso alarme: %s", r[0].Content)
	}
	_ = os.Remove(filepath.Join("a.go"))
}

func TestSaidaSemAnsi(t *testing.T) {
	if got := LimpaSaida("\x1b[32mok\x1b[0m\n10%\r100%\n"); got != "ok\n100%\n" {
		t.Fatalf("%q", got)
	}
}
