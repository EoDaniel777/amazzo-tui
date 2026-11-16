package app

import (
	"strings"
	"testing"

	"amazzo/mcp"

	"github.com/charmbracelet/lipgloss"
	"github.com/muesli/termenv"
)

func TestTrocarTemaRedesenhaHistorico(t *testing.T) {
	lipgloss.SetColorProfile(termenv.TrueColor)
	defer lipgloss.SetColorProfile(termenv.Ascii)
	m := modeloParaPalette(t)
	m.addRerenderable(func() string { return renderUserEntry("ola", m.viewport.Width) })
	m.addRerenderable(func() string {
		return renderAssistantEntry("qwen3-4b", "", m.renderMarkdown("# Titulo\n\nTexto com `codigo` aqui."), m.viewport.Width)
	})
	antes := strings.Join(m.messages, "\n")
	m.commandDeps.SetTheme("dracula")
	depois := strings.Join(m.messages, "\n")
	if antes == depois {
		t.Fatal("o histórico deveria ser redesenhado com as cores do novo tema")
	}
	// Cor do usuário do Dracula: #50fa7b = 80;250;123.
	if !strings.Contains(depois, "80;250;123") {
		t.Fatal("cor do tema novo não apareceu no histórico")
	}
	m.commandDeps.SetTheme("default")
}

func TestResultadoDeFerramentaCompacto(t *testing.T) {
	lista := toolResultBlock(mcp.Result{Tool: "list_dir", Path: ".", Status: "success",
		Content: "📁 Conteúdo de /x\n - a/\n - b/\n - c.go\n"}, 100)
	if !strings.Contains(lista, "3 itens") || strings.Contains(lista, "📁") {
		t.Fatalf("lista: %q", lista)
	}
	busca := toolResultBlock(mcp.Result{Tool: "search_files", Status: "success",
		Content: "📄 200 arquivo(s) com nome casando '*.go':\n\na.go (1 bytes)\nb.go (2 bytes)\nc.go (3 bytes)\nd.go (4 bytes)\ne.go (5 bytes)\nf.go (6 bytes)\n"}, 100)
	if !strings.Contains(busca, "200 resultados") || !strings.Contains(busca, "+1") || strings.Contains(busca, "📄") {
		t.Fatalf("busca: %q", busca)
	}
	fundo := toolResultBlock(mcp.Result{Tool: "execute_command", Status: "success",
		Content: "O comando continua rodando em segundo plano (pid 42), depois de 20s.\nSaída até agora:\nx"}, 100)
	if !strings.Contains(fundo, "segundo plano") {
		t.Fatalf("segundo plano: %q", fundo)
	}
}

func TestStripEmoji(t *testing.T) {
	if got := stripEmoji("📁 Conteúdo ✅ ok"); got != "Conteúdo  ok" {
		t.Fatalf("got %q", got)
	}
}
