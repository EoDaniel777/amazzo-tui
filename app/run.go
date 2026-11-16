package app

import (
	"context"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"amazzo/mcp"

	tea "github.com/charmbracelet/bubbletea"
)

// Rodar programas do projeto sem depender do modelo: "!comando" executa direto
// na pasta de trabalho, e /rodar lista como o projeto costuma ser iniciado.
// Modelos pequenos explicam em vez de executar; aqui quem decide é você.

type shellDoneMsg struct {
	cmd     string
	reqs    []mcp.Request
	results []mcp.Result
}

// runShell executa o comando fora do loop do modelo.
func (m *Model) runShell(cmd string) tea.Cmd {
	reqs := []mcp.Request{{Tool: "execute_command", Command: cmd}}
	srv := m.mcpServer
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
		defer cancel()
		res, _ := srv.Handle(ctx, reqs)
		return shellDoneMsg{cmd: cmd, reqs: reqs, results: res}
	}
}

// startShell prepara a tela e dispara o comando.
func (m *Model) startShell(cmd string) tea.Cmd {
	if m.loading {
		m.addMessage(renderNotice("!", "Aguarde o modelo terminar (ou esc) antes de rodar um comando.", colAct))
		m.addMessage("")
		return nil
	}
	m.addMessage(renderToolEvent("rodando: " + cmd))
	m.ide.showPanel = true
	m.applyLayout()
	m.recordCommand(cmd, "executando…", "")
	m.loading = true
	return m.runShell(cmd)
}

func (m *Model) finishShell(msg shellDoneMsg) {
	m.loading = false
	for i, res := range msg.results {
		m.renderToolResult(res)
		m.recordIfCommand(msg.reqs, i, res)
	}
	m.addMessage("")
}

// runCandidate é uma forma provável de iniciar o projeto.
type runCandidate struct{ Cmd, Why string }

// detectRunCommands procura binários, mains de Go, Makefile e scripts npm.
func detectRunCommands(root string) []runCandidate {
	var out []runCandidate

	// Executáveis na raiz.
	if entries, err := os.ReadDir(root); err == nil {
		for _, e := range entries {
			if e.IsDir() || strings.HasPrefix(e.Name(), ".") {
				continue
			}
			info, err := e.Info()
			if err != nil || info.Mode()&0111 == 0 || info.Size() < 100*1024 {
				continue // scripts pequenos (.sh) ficam de fora; binário tem centenas de KB
			}
			out = append(out, runCandidate{"./" + e.Name(), "binário já compilado"})
		}
	}

	// Go: main na raiz e em cmd/*.
	if _, err := os.Stat(filepath.Join(root, "go.mod")); err == nil {
		if _, err := os.Stat(filepath.Join(root, "main.go")); err == nil {
			out = append(out, runCandidate{"go run .", "main.go na raiz"})
		}
		mains, _ := filepath.Glob(filepath.Join(root, "cmd", "*", "main.go"))
		sort.Strings(mains)
		for _, mf := range mains {
			name := filepath.Base(filepath.Dir(mf))
			out = append(out, runCandidate{"go run ./cmd/" + name, "cmd/" + name})
		}
	}

	// Makefile com alvo run.
	if b, err := os.ReadFile(filepath.Join(root, "Makefile")); err == nil {
		if strings.Contains(string(b), "\nrun:") || strings.HasPrefix(string(b), "run:") {
			out = append(out, runCandidate{"make run", "alvo run do Makefile"})
		}
	}

	// npm.
	if b, err := os.ReadFile(filepath.Join(root, "package.json")); err == nil {
		s := string(b)
		if strings.Contains(s, `"start"`) {
			out = append(out, runCandidate{"npm start", "script start"})
		}
		if strings.Contains(s, `"dev"`) {
			out = append(out, runCandidate{"npm run dev", "script dev"})
		}
	}

	// Python.
	for _, f := range []string{"main.py", "app.py"} {
		if _, err := os.Stat(filepath.Join(root, f)); err == nil {
			out = append(out, runCandidate{"python3 " + f, f})
		}
	}
	return out
}

// runItems transforma os candidatos em itens do popup.
func (m *Model) runItems() []paletteItem {
	cands := detectRunCommands(m.workspaceRoot)
	var out []paletteItem
	for _, c := range cands {
		out = append(out, paletteItem{Cmd: "!" + c.Cmd, Title: c.Cmd, Desc: c.Why})
	}
	if len(out) == 0 {
		out = append(out, paletteItem{Cmd: "", Title: "Nada para rodar foi detectado aqui", Desc: "use !comando para rodar qualquer coisa"})
	}
	return out
}
