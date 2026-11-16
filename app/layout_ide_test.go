package app

import (
	"strings"
	"testing"

	"github.com/charmbracelet/lipgloss"
)

func TestIDELayoutDimensions(t *testing.T) {
	m, err := New("", t.TempDir(), "")
	if err != nil {
		t.Fatal(err)
	}
	m.showSplash = false
	m.ide.termW, m.ide.termH = 140, 40
	m.width, m.height = 140, 40
	m.applyLayout()

	// Padrão: só o chat, com a barra de status sempre visível.
	if m.ideActive() {
		t.Fatal("layout deveria começar só com o chat")
	}
	if m.height != 39 {
		t.Fatalf("chat deveria ceder 1 linha à barra de status, altura %d", m.height)
	}
	base := m.View()
	if n := len(strings.Split(base, "\n")); n != 40 {
		t.Fatalf("tela padrão com %d linhas, queria 40", n)
	}

	m.toggleExplorer()
	m.togglePanel()
	if m.width != 140-explorerWidth-1 || m.height != 40-panelHeight-1 {
		t.Fatalf("área do chat errada: %dx%d", m.width, m.height)
	}
	m.recordCommand("meuapp", "bash: meuapp: comando não encontrado", "127")
	view := m.View()
	lines := strings.Split(view, "\n")
	if len(lines) != 40 {
		t.Fatalf("altura total = %d, queria 40", len(lines))
	}
	for i, l := range lines {
		if w := lipgloss.Width(l); w > 140 {
			t.Fatalf("linha %d com largura %d > 140", i, w)
		}
	}
	if !strings.Contains(view, "EXPLORADOR") || !strings.Contains(view, "SAÍDA") {
		t.Fatal("explorador ou painel ausentes")
	}

	// Foco: volta ao tamanho original e à mesma tela do início.
	m.setFocusMode(true)
	if m.width != 140 || m.height != 39 {
		t.Fatalf("foco deveria devolver %dx%d, deu %dx%d", 140, 39, m.width, m.height)
	}
	if m.View() != base {
		t.Fatal("modo foco difere da tela original")
	}
}

func TestIDENarrowTerminalHidesExplorer(t *testing.T) {
	m, err := New("", t.TempDir(), "")
	if err != nil {
		t.Fatal(err)
	}
	m.showSplash = false
	m.ide.termW, m.ide.termH = 80, 30
	m.applyLayout()
	m.toggleExplorer()
	if m.explorerOn() {
		t.Fatal("explorador não deveria abrir em 80 colunas")
	}
}
