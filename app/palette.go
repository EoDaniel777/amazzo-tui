package app

import (
	"fmt"
	"sort"
	"strings"

	"amazzo/commands"
	"amazzo/themes"

	"github.com/charmbracelet/bubbles/textarea"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"
)

// Paleta de comandos: um popup por cima do chat, com busca. Abre com /ajuda,
// ctrl+k ou F1. Digite para filtrar, ↑↓ para navegar, enter para executar,
// tab para só inserir o comando no campo, esc para fechar.

type paletteItem struct {
	Cmd     string // o que será executado, ou inserido no campo
	Title   string
	Desc    string
	NeedArg bool   // se true, enter insere "Cmd " no campo em vez de executar
	Group   string // cabeçalho de seção (opcional)
}

type palette struct {
	active    bool
	query     string
	idx       int
	items     []paletteItem
	extra     []paletteItem // só aparecem na busca (temas e modelos)
	mode      string        // "", "tema" ou "modelo"
	origTheme string        // tema ao abrir a lista de temas, para desfazer a prévia
}

const paletteRows = 12

// paletteCatalog lista tudo que o usuário pode querer fazer.
func (m *Model) paletteCatalog() []paletteItem {
	items := []paletteItem{
		{"/explorador", "Explorador de arquivos", "Mostra ou oculta o explorador (ctrl+b)", false, ""},
		{"/painel", "Painel de saída", "Mostra ou oculta o painel de comandos (ctrl+g)", false, ""},
		{"/foco", "Só o chat", "Oculta explorador e painel (/foco off mostra de novo)", false, ""},
		{"/pensar", "Raciocínio do modelo", "Liga ou desliga o raciocínio (desligado é bem mais rápido)", false, ""},
		{"/limpar", "Limpar a tela", "Apaga as mensagens da tela (ctrl+l)", false, ""},
		{"/pasta", "Trocar a pasta de trabalho", "Ex.: /pasta ~/Documentos/projeto", true, ""},
		{"/ollama", "Atualizar modelos locais", "Relê os modelos baixados no Ollama", false, ""},
		{"/mcp", "Servidores MCP", "Lista, recarrega, liga ou desliga servidores MCP configurados", false, ""},
		{"@rodar", "Rodar o projeto…", "Lista binários, go run, make e npm detectados nesta pasta (ou use !comando)", false, ""},
		{"@tema", "Trocar tema…", "Escolha entre os temas de cores, com prévia ao vivo", false, ""},
		{"@modelo", "Trocar modelo…", "Escolha o modelo de IA que responde (local ou remoto)", false, ""},
		{"/agentes", "Agentes especializados", "Gerencia agentes (list, create, ...)", true, ""},
		{"/criar-agente", "Criar agente", "Assistente para criar um agente personalizado", false, ""},
		{"/multiagente", "Modo multi-agente", "Liga ou desliga vários agentes em paralelo", false, ""},
		{"/tarefas", "Tarefas atuais", "Mostra o progresso das tarefas", false, ""},
		{"/metricas", "Métricas", "Estatísticas de desempenho e cache", false, ""},
		{"/erro", "Último erro", "Mostra o último erro, copiável", false, ""},
		{"/copiar", "Copiar última resposta", "Copia a última resposta do modelo", false, ""},
		{"/salvar", "Salvar conversa", "Salva em formato legível (output/)", false, ""},
		{"/logs", "Exportar logs", "Salva a conversa em formato técnico", true, ""},
		{"/permitir", "Permissões das ferramentas", "Gerencia o que o agente pode fazer sem perguntar", true, ""},
		{"/key", "Token do Hugging Face", "Configura o token dos modelos remotos", false, ""},
		{"/sobre", "Sobre o Amazzo", "Informações sobre o assistente", false, ""},
		{"/ajuda completa", "Lista completa de comandos", "Mostra a ajuda inteira em texto", false, ""},
	}
	return items
}

// themeItems lista os temas; o atual vem marcado.
func (m *Model) themeItems() []paletteItem {
	var out []paletteItem
	for _, key := range themes.SortedKeys() {
		t := themes.Available[key]
		desc := "escuro"
		if uiPalettes[paletteKeyFor(key)].Light {
			desc = "claro"
		}
		title := t.Name
		if key == m.currentTheme {
			title = "● " + title + "  (atual)"
		}
		out = append(out, paletteItem{Cmd: "/temas " + key, Title: title, Desc: desc})
	}
	return out
}

// modelItems lista os modelos, com a finalidade de cada um.
func (m *Model) modelItems() []paletteItem {
	var out []paletteItem
	for i, mod := range availableModels {
		title := shortModelName(mod.DisplayName)
		if mod.Provider == providerAmazzo {
			title = strings.TrimSuffix(title, " · só texto")
		}
		if i == m.currentModel {
			title = "● " + title + "  (em uso)"
		}
		out = append(out, paletteItem{
			Cmd:   fmt.Sprintf("/modelo %d", i+1),
			Title: title,
			Desc:  modelPurpose(mod),
			Group: modelGroup(mod),
		})
	}
	return out
}

func commandsHandle(input string, m *Model) bool {
	return commands.Handle(input, m.commandDeps)
}

func (m *Model) openPalette() {
	m.pal = palette{active: true, items: m.paletteCatalog()}
	m.pal.extra = append(m.themeItems(), m.modelItems()...)
}

// enterSubmenu abre a lista de temas ou de modelos dentro do popup.
func (m *Model) enterSubmenu(kind string) {
	p := &m.pal
	p.mode, p.query = kind, ""
	if kind == "tema" {
		p.items = m.themeItems()
		p.origTheme = m.currentTheme
		for i, it := range p.items {
			if strings.HasPrefix(it.Cmd, "/temas "+m.currentTheme) && strings.TrimPrefix(it.Cmd, "/temas ") == m.currentTheme {
				p.idx = i
			}
		}
	} else if kind == "rodar" {
		p.items = m.runItems()
		p.idx = 0
	} else {
		p.items = m.modelItems()
		p.idx = m.currentModel
	}
	p.extra = nil
}

// previewTheme aplica o tema selecionado na lista, sem salvar.
func (m *Model) previewTheme() {
	p := &m.pal
	if p.mode != "tema" {
		return
	}
	list := p.filtered()
	if p.idx < 0 || p.idx >= len(list) {
		return
	}
	key := strings.TrimPrefix(list[p.idx].Cmd, "/temas ")
	if m.commandDeps.SetTheme != nil {
		m.commandDeps.SetTheme(key)
	}
}

// filtered devolve os itens que casam com a busca, os que começam com ela
// primeiro.
func (p *palette) filtered() []paletteItem {
	q := strings.ToLower(strings.TrimSpace(strings.TrimPrefix(p.query, "/")))
	if q == "" {
		return p.items
	}
	all := append(append([]paletteItem{}, p.items...), p.extra...)
	words := strings.Fields(q)
	var starts, rest []paletteItem
	for _, it := range all {
		hay := strings.ToLower(it.Cmd + " " + it.Title + " " + it.Desc)
		ok := true
		for _, w := range words {
			if !strings.Contains(hay, w) {
				ok = false
				break
			}
		}
		if !ok {
			continue
		}
		if strings.HasPrefix(strings.ToLower(strings.TrimLeft(it.Cmd, "/@")), words[0]) {
			starts = append(starts, it)
		} else {
			rest = append(rest, it)
		}
	}
	// Os submenus (@tema, @modelo) vêm antes dos itens avulsos.
	sort.SliceStable(starts, func(i, j int) bool {
		return strings.HasPrefix(starts[i].Cmd, "@") && !strings.HasPrefix(starts[j].Cmd, "@")
	})
	return append(starts, rest...)
}

// handlePaletteKey trata as teclas com o popup aberto.
func (m *Model) handlePaletteKey(msg tea.KeyMsg) tea.Cmd {
	p := &m.pal
	list := p.filtered()

	switch msg.Type {
	case tea.KeyEsc, tea.KeyCtrlC:
		if p.mode != "" && msg.Type == tea.KeyEsc {
			// Volta ao menu principal, desfazendo a prévia do tema.
			if p.mode == "tema" && m.commandDeps.SetTheme != nil {
				m.commandDeps.SetTheme(p.origTheme)
			}
			m.openPalette()
			return nil
		}
		if p.mode == "tema" && m.commandDeps.SetTheme != nil {
			m.commandDeps.SetTheme(p.origTheme)
		}
		p.active = false
	case tea.KeyUp, tea.KeyShiftTab:
		if p.idx > 0 {
			p.idx--
		} else if len(list) > 0 {
			p.idx = len(list) - 1
		}
	case tea.KeyDown:
		if p.idx < len(list)-1 {
			p.idx++
		} else {
			p.idx = 0
		}
	case tea.KeyPgUp:
		p.idx = max(0, p.idx-paletteRows)
	case tea.KeyPgDown:
		p.idx = min(max(0, len(list)-1), p.idx+paletteRows)
	case tea.KeyBackspace:
		if r := []rune(p.query); len(r) > 0 {
			p.query = string(r[:len(r)-1])
			p.idx = 0
		}
	case tea.KeyCtrlU:
		p.query, p.idx = "", 0
	case tea.KeyTab:
		// Insere o comando no campo para completar ou editar antes de enviar.
		if len(list) > 0 {
			it := list[p.idx]
			p.active = false
			m.textarea.SetValue(it.Cmd + " ")
			m.textarea.CursorEnd()
			m.adjustTextareaHeight()
		}
	case tea.KeyEnter:
		if len(list) == 0 {
			return nil
		}
		it := list[p.idx]
		if strings.HasPrefix(it.Cmd, "@") {
			m.enterSubmenu(strings.TrimPrefix(it.Cmd, "@"))
			m.previewTheme()
			return nil
		}
		p.active = false
		if strings.HasPrefix(it.Cmd, "!") {
			return m.startShell(strings.TrimPrefix(it.Cmd, "!"))
		}
		if it.Cmd == "" {
			return nil
		}
		if it.NeedArg {
			m.textarea.SetValue(it.Cmd + " ")
			m.textarea.CursorEnd()
			m.adjustTextareaHeight()
			return textarea.Blink
		}
		m.runSlash(it.Cmd)
	case tea.KeySpace:
		p.query += " "
		p.idx = 0
	case tea.KeyRunes:
		p.query += string(msg.Runes)
		p.idx = 0
	}

	if n := len(p.filtered()); p.idx >= n {
		p.idx = max(0, n-1)
	}
	m.previewTheme()
	return nil
}

// runSlash executa um comando de barra e leva a tela para o fim, onde a
// resposta aparece.
func (m *Model) runSlash(input string) bool {
	if m.handleMCPCommand(input) {
		m.lastWasCommand = true
		return true
	}
	if !commandsHandle(input, m) {
		return false
	}
	m.lastWasCommand = true
	m.viewport.GotoBottom()
	return true
}

// renderPalette desenha o popup.
func (m *Model) renderPalette(maxW int) string {
	p := &m.pal
	w := min(86, maxW-4)
	if w < 40 {
		w = maxW
	}
	inner := w - 4

	dim := lipgloss.NewStyle().Foreground(ghDim)
	titulo := "Comandos"
	switch p.mode {
	case "tema":
		titulo = "Temas"
	case "modelo":
		titulo = "Modelos"
	case "rodar":
		titulo = "Rodar"
	}
	title := lipgloss.NewStyle().Foreground(ghBlue).Bold(true).Render(titulo) +
		dim.Render("   digite para buscar")

	cursor := lipgloss.NewStyle().Reverse(true).Render(" ")
	search := lipgloss.NewStyle().Foreground(ghBlue).Render("❯ ") +
		lipgloss.NewStyle().Foreground(ghFg).Render(p.query) + cursor

	list := p.filtered()
	// Janela de linhas: cada mudança de seção gasta uma linha de cabeçalho,
	// então a janela avança até o item selecionado caber.
	build := func(start int) (rows []string, last int) {
		prev := ""
		if start > 0 {
			prev = list[start-1].Group
		}
		for i := start; i < len(list); i++ {
			it := list[i]
			need := 1
			if it.Group != "" && it.Group != prev {
				need = 2
			}
			if len(rows)+need > paletteRows {
				break
			}
			if need == 2 {
				rows = append(rows, lipgloss.NewStyle().Foreground(ghBlue).Bold(true).Render(" "+it.Group))
			}
			prev = it.Group
			cmd := it.Cmd
			left := fmt.Sprintf("%-18s", ansi.Truncate(cmd, 18, "…"))
			text := it.Title
			if it.Desc != "" {
				text += "  " + it.Desc
			}
			line := left + " " + text
			if p.mode != "" {
				line = text
			}
			line = ansi.Truncate(line, inner-2, "…")
			line = line + strings.Repeat(" ", max(0, inner-2-ansi.StringWidth(line)))
			if i == p.idx {
				rows = append(rows, lipgloss.NewStyle().Background(ghSel).Foreground(lipgloss.Color("#ffffff")).Render(" "+line+" "))
			} else {
				rows = append(rows, lipgloss.NewStyle().Foreground(ghFg).Render(" "+line+" "))
			}
			last = i
		}
		return rows, last
	}
	start := 0
	if p.idx >= paletteRows {
		start = p.idx - paletteRows + 1
	}
	rows, last := build(start)
	for len(list) > 0 && p.idx > last && start < len(list)-1 {
		start++
		rows, last = build(start)
	}
	if len(list) == 0 {
		rows = append(rows, dim.Render(" Nada encontrado. Tente outra palavra."))
	}
	for len(rows) < paletteRows {
		rows = append(rows, "")
	}

	pos := ""
	if len(list) > paletteRows {
		pos = fmt.Sprintf("  %d/%d", p.idx+1, len(list))
	}
	dica := "↑↓ navegar · enter usar · tab inserir no campo · esc fechar"
	if p.mode != "" {
		dica = "↑↓ navegar · enter escolher · esc voltar"
		if p.mode == "tema" {
			dica = "↑↓ testa o tema ao vivo · enter salva · esc desfaz"
		}
	}
	foot := dim.Render(dica + pos)

	body := strings.Join([]string{title, search, dim.Render(strings.Repeat("─", inner)), strings.Join(rows, "\n"), dim.Render(strings.Repeat("─", inner)), foot}, "\n")
	return lipgloss.NewStyle().
		Width(w-2).
		Padding(0, 1).
		BorderStyle(lipgloss.RoundedBorder()).
		BorderForeground(ghBlue).
		Background(ghPanel).
		Render(body)
}

// overlayBox coloca box por cima de base, centralizado na horizontal e no
// terço superior da altura.
func overlayBox(base, box string, w, h int, bg lipgloss.Color) string {
	lines := strings.Split(base, "\n")
	boxLines := strings.Split(box, "\n")
	bw := 0
	for _, l := range boxLines {
		bw = max(bw, ansi.StringWidth(l))
	}
	x := max(0, (w-bw)/2)
	top := max(0, (h-len(boxLines))/3)

	on := bgSeq(bg)
	for i, bl := range boxLines {
		r := top + i
		if r >= len(lines) {
			break
		}
		l := lines[r]
		left := ansi.Truncate(l, x, "")
		right := ansi.TruncateLeft(l, x+bw, "")
		bl += strings.Repeat(" ", max(0, bw-ansi.StringWidth(bl)))
		lines[r] = left + "\x1b[0m" + bl + "\x1b[0m" + on + right
	}
	return strings.Join(lines, "\n")
}
