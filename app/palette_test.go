package app

import (
	"fmt"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/x/ansi"
)

func modeloParaPalette(t *testing.T) *Model {
	t.Helper()
	m, err := New("", t.TempDir(), "")
	if err != nil {
		t.Fatal(err)
	}
	m.Update(tea.WindowSizeMsg{Width: 140, Height: 36})
	m.showSplash = false
	m.finishSplash()
	return m
}

func digita(m *Model, s string) {
	for _, r := range s {
		m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{r}})
	}
}

func TestAjudaAbrePaletteComBusca(t *testing.T) {
	m := modeloParaPalette(t)
	digita(m, "/ajuda")
	m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	if !m.pal.active {
		t.Fatal("/ajuda deveria abrir a paleta")
	}
	if v := ansi.Strip(m.View()); !strings.Contains(v, "Comandos") || !strings.Contains(v, "/explorador") {
		t.Fatalf("popup não apareceu:\n%s", v)
	}

	digita(m, "pens")
	itens := m.pal.filtered()
	if len(itens) == 0 || itens[0].Cmd != "/pensar" {
		t.Fatalf("busca por 'pens' deveria achar /pensar primeiro: %+v", itens)
	}

	// Enter executa o item; o popup fecha e o raciocínio alterna.
	m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	if m.pal.active {
		t.Fatal("enter deveria fechar a paleta")
	}
}

func TestPaletteNavegaComSetasETabInsere(t *testing.T) {
	m := modeloParaPalette(t)
	m.openPalette()
	m.Update(tea.KeyMsg{Type: tea.KeyDown})
	if m.pal.idx != 1 {
		t.Fatalf("seta para baixo: idx=%d", m.pal.idx)
	}
	m.Update(tea.KeyMsg{Type: tea.KeyUp})
	m.Update(tea.KeyMsg{Type: tea.KeyUp}) // volta para o último (dá a volta)
	if m.pal.idx != len(m.pal.filtered())-1 {
		t.Fatalf("seta para cima no topo deveria ir ao fim: idx=%d", m.pal.idx)
	}

	m.pal.query, m.pal.idx = "pasta", 0
	m.Update(tea.KeyMsg{Type: tea.KeyEnter}) // /pasta pede argumento: insere no campo
	if got := m.textarea.Value(); got != "/pasta " {
		t.Fatalf("campo = %q, queria %q", got, "/pasta ")
	}
}

func TestPaletteEscFecha(t *testing.T) {
	m := modeloParaPalette(t)
	m.openPalette()
	m.Update(tea.KeyMsg{Type: tea.KeyEsc})
	if m.pal.active {
		t.Fatal("esc deveria fechar")
	}
}

// O bug do usuário: comando de barra ignorado com o modelo ocupado.
func TestComandoFuncionaComModeloOcupado(t *testing.T) {
	m := modeloParaPalette(t)
	m.loading = true
	digita(m, "/modelo")
	n := len(m.messages)
	m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	if len(m.messages) <= n {
		t.Fatal("/modelo deveria responder mesmo com o modelo ocupado")
	}
}

// O outro bug: conversa rolada para cima escondia a resposta do comando.
func TestComandoLevaTelaParaOFim(t *testing.T) {
	m := modeloParaPalette(t)
	for i := 0; i < 80; i++ {
		m.addMessage(fmt.Sprintf("linha %d", i))
	}
	m.viewport.GotoTop()
	digita(m, "/modelo")
	m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	if !m.viewport.AtBottom() {
		t.Fatal("depois de um comando a conversa deveria ir para o fim")
	}
}

func TestPaletteTemasPrevisaoESelecao(t *testing.T) {
	m := modeloParaPalette(t)
	m.openPalette()
	m.pal.query = "trocar tema"
	m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	if m.pal.mode != "tema" || len(m.pal.items) < 10 {
		t.Fatalf("deveria abrir a lista de temas: modo=%q itens=%d", m.pal.mode, len(m.pal.items))
	}
	orig := m.currentTheme

	// Descer na lista aplica o tema ao vivo (prévia), sem salvar.
	m.Update(tea.KeyMsg{Type: tea.KeyDown})
	if m.currentTheme == orig {
		t.Fatal("a prévia deveria trocar o tema")
	}
	// Esc volta ao tema original.
	m.Update(tea.KeyMsg{Type: tea.KeyEsc})
	if m.currentTheme != orig || m.pal.mode != "" {
		t.Fatalf("esc deveria desfazer a prévia: tema=%q modo=%q", m.currentTheme, m.pal.mode)
	}

	// Enter confirma.
	m.pal.query = "trocar tema"
	m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m.Update(tea.KeyMsg{Type: tea.KeyDown})
	escolhido := m.currentTheme
	m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	if m.currentTheme != escolhido || m.pal.active {
		t.Fatalf("enter deveria manter %q e fechar", escolhido)
	}
}

func TestPaletteBuscaTemaEModeloSemSubmenu(t *testing.T) {
	m := modeloParaPalette(t)
	m.openPalette()
	m.pal.query = "dracula"
	if l := m.pal.filtered(); len(l) == 0 || !strings.Contains(l[0].Cmd, "dracula") {
		t.Fatalf("busca 'dracula' deveria achar o tema: %+v", l)
	}
	m.pal.query = "qwen"
	if len(m.pal.filtered()) == 0 {
		t.Fatal("busca 'qwen' deveria achar modelos")
	}
}

func TestTemaPorNumero(t *testing.T) {
	m := modeloParaPalette(t)
	digita(m, "/temas dracula")
	m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	if m.currentTheme == "default" {
		t.Fatal("/temas dracula deveria trocar o tema")
	}
}
