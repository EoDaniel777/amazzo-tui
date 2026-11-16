package app

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"amazzo/agents"
	"amazzo/mcp"

	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"
)

// Layout estilo VS Code: explorador à esquerda, chat no centro, painel de
// saída embaixo e barra de status. Com os dois painéis ocultos (padrão) a tela
// é o chat de sempre, sem nenhuma diferença.
//
// Convenção: m.width/m.height sempre descrevem a área do CHAT. O tamanho real
// do terminal fica em termW/termH. Assim o resto do código não precisa saber
// que existem painéis ao redor.

// Paleta GitHub Dark.
var (
	ghBg     = lipgloss.Color("#0d1117")
	ghPanel  = lipgloss.Color("#161b22")
	ghLine   = lipgloss.Color("#30363d")
	ghFg     = lipgloss.Color("#e6edf3")
	ghDim    = lipgloss.Color("#8b949e")
	ghBlue   = lipgloss.Color("#58a6ff")
	ghGreen  = lipgloss.Color("#3fb950")
	ghRed    = lipgloss.Color("#f85149")
	ghYellow = lipgloss.Color("#d29922")
	ghSel    = lipgloss.Color("#1f6feb")
)

const (
	explorerWidth = 30
	panelHeight   = 8 // linhas do painel, contando a linha de abas
	// Abaixo disso não sobra espaço útil para o chat com o explorador aberto.
	minTermForExplorer = 100
	minTermForPanel    = 24
)

// ideState guarda o que o layout precisa lembrar entre quadros.
type ideState struct {
	showExplorer bool
	showPanel    bool
	termW, termH int

	clock clockState

	lastCmd    string
	lastOutput string
	lastExit   string

	tree        []string
	branch      string
	changes     map[string]string
	refreshedAt time.Time

	mcpOn, mcpTools int
}

func (m *Model) explorerOn() bool {
	return m.ide.showExplorer && m.ide.termW >= minTermForExplorer
}

func (m *Model) panelOn() bool {
	return m.ide.showPanel && m.ide.termH >= minTermForPanel
}

func (m *Model) ideActive() bool { return m.explorerOn() || m.panelOn() }

// applyLayout recalcula m.width/m.height (área do chat) e reajusta o que
// depende deles: viewport, campo de texto e quebra do markdown.
func (m *Model) applyLayout() {
	tw, th := m.ide.termW, m.ide.termH
	if tw == 0 || th == 0 {
		return
	}
	// O estado do layout é sempre a verdade; w/h abaixo descontam o que está visível.
	w, h := tw, th
	if m.explorerOn() {
		w -= explorerWidth + 1
	}
	if m.panelOn() {
		h -= panelHeight
	}
	h-- // barra de status, sempre visível
	m.width, m.height = w, h

	headerHeight := 2
	if m.showBanner {
		headerHeight = 14
	}
	footerHeight := 3

	vw := w - 2
	if vw < 40 {
		vw = 40
	}
	vh := h - headerHeight - footerHeight
	if vh < 6 {
		vh = 6
	}
	m.viewport.Width = vw
	m.viewport.Height = vh
	m.textarea.SetWidth(m.larguraDoCampo())
	m.adjustTextareaHeight()
	m.setGlamourWidth(vw - 4)
	if len(m.rerender) > 0 {
		m.rerenderHistory() // reflui o histórico na largura nova
	}
}

// toggleExplorer e togglePanel devolvem o novo estado.
func (m *Model) toggleExplorer() bool {
	m.ide.showExplorer = !m.ide.showExplorer
	m.ide.refreshedAt = time.Time{}
	m.applyLayout()
	return m.ide.showExplorer
}

func (m *Model) togglePanel() bool {
	m.ide.showPanel = !m.ide.showPanel
	m.applyLayout()
	return m.ide.showPanel
}

// setFocusMode oculta tudo (true) ou mostra explorador e painel (false).
func (m *Model) setFocusMode(focus bool) {
	m.ide.showExplorer = !focus
	m.ide.showPanel = !focus
	m.ide.refreshedAt = time.Time{}
	m.applyLayout()
}

// recordCommand guarda a última saída de comando para o painel.
func (m *Model) recordCommand(cmd, output, exit string) {
	m.ide.lastCmd = cmd
	m.ide.lastOutput = output
	m.ide.lastExit = exit
}

// View monta a tela completa.
func (m *Model) View() string {
	if m.showSplash {
		return m.chatView()
	}

	m.refreshIDE()

	chat := fitHeight(m.chatView(), m.height)
	chat = paint(chat, m.width, ghBg)
	if m.pal.active {
		chat = overlayBox(chat, m.renderPalette(m.width), m.width, m.height, ghBg)
	}

	body := chat
	areaH := m.height
	if m.explorerOn() {
		body = lipgloss.JoinHorizontal(lipgloss.Top, paint(m.renderExplorer(areaH), explorerWidth, ghPanel), m.verticalRule(areaH), chat)
	}

	sections := []string{body}
	if m.panelOn() {
		sections = append(sections, paint(m.renderPanel(m.ide.termW), m.ide.termW, ghBg))
	}
	sections = append(sections, m.renderStatusLine(m.ide.termW))
	return lipgloss.JoinVertical(lipgloss.Left, sections...)
}

func (m *Model) verticalRule(h int) string {
	line := lipgloss.NewStyle().Foreground(ghLine).Render("│")
	return strings.TrimRight(strings.Repeat(line+"\n", h), "\n")
}

// ---------- dados ----------

// refreshIDE atualiza a árvore e o estado do git, no máximo a cada 4s.
func (m *Model) refreshIDE() {
	if time.Since(m.ide.refreshedAt) < 4*time.Second {
		return
	}
	m.ide.refreshedAt = time.Now()
	root := m.workspaceRoot
	m.ide.tree = buildTree(root, 0, 80)

	m.ide.branch = ""
	m.ide.changes = map[string]string{}
	if out, err := runGit(root, "rev-parse", "--abbrev-ref", "HEAD"); err == nil {
		m.ide.branch = strings.TrimSpace(out)
	}
	if out, err := runGit(root, "status", "--porcelain"); err == nil {
		for _, l := range strings.Split(out, "\n") {
			if len(l) < 4 {
				continue
			}
			flag := strings.TrimSpace(l[:2])
			path := strings.TrimSpace(l[3:])
			if flag == "??" {
				flag = "U"
			} else if len(flag) > 1 {
				flag = flag[:1]
			}
			m.ide.changes[filepath.ToSlash(path)] = flag
		}
	}
}

func runGit(dir string, args ...string) (string, error) {
	cmd := exec.Command("git", append([]string{"-C", dir}, args...)...)
	out, err := cmd.Output()
	return string(out), err
}

var ignoredDirs = map[string]bool{
	".git": true, "node_modules": true, "vendor": true, "build": true,
	"dist": true, "target": true, "__pycache__": true, ".venv": true,
}

// buildTree lista a pasta como linhas "d|nome|caminho|nivel" até maxDepth,
// com um teto de entradas para pastas enormes.
func buildTree(root string, maxDepth, maxEntries int) []string {
	var out []string
	var walk func(dir string, depth int)
	walk = func(dir string, depth int) {
		if depth > maxDepth || len(out) >= maxEntries {
			return
		}
		entries, err := os.ReadDir(dir)
		if err != nil {
			return
		}
		sort.SliceStable(entries, func(i, j int) bool {
			if entries[i].IsDir() != entries[j].IsDir() {
				return entries[i].IsDir()
			}
			return entries[i].Name() < entries[j].Name()
		})
		for _, e := range entries {
			if len(out) >= maxEntries {
				return
			}
			name := e.Name()
			if e.IsDir() && ignoredDirs[name] {
				continue
			}
			rel, _ := filepath.Rel(root, filepath.Join(dir, name))
			kind := "f"
			if e.IsDir() {
				kind = "d"
			}
			out = append(out, fmt.Sprintf("%s|%s|%s|%d", kind, name, filepath.ToSlash(rel), depth))
			if e.IsDir() {
				walk(filepath.Join(dir, name), depth+1)
			}
		}
	}
	walk(root, 0)
	return out
}

// ---------- desenho ----------

func fitLine(s string, w int) string {
	if ansi.StringWidth(s) > w {
		s = ansi.Truncate(s, w, "…")
	}
	return s + strings.Repeat(" ", max(0, w-ansi.StringWidth(s)))
}

func (m *Model) renderExplorer(h int) string {
	bg := lipgloss.NewStyle().Background(ghPanel).Foreground(ghFg)
	head := lipgloss.NewStyle().Background(ghPanel).Foreground(ghDim).Bold(true)
	w := explorerWidth

	var lines []string
	lines = append(lines, head.Render(fitLine(" EXPLORADOR", w)))
	root := filepath.Base(m.workspaceRoot)
	lines = append(lines, bg.Foreground(ghBlue).Bold(true).Render(fitLine(" ▾ "+root, w)))

	for _, raw := range m.ide.tree {
		p := strings.SplitN(raw, "|", 4)
		if len(p) < 4 {
			continue
		}
		kind, name, rel := p[0], p[1], p[2]
		depth := 0
		fmt.Sscanf(p[3], "%d", &depth)
		indent := strings.Repeat("  ", depth+1)

		icon := " "
		style := bg
		if kind == "d" {
			icon = "▸ "
			style = bg.Foreground(ghFg)
		}
		flag := m.ide.changes[rel]
		tag := ""
		switch flag {
		case "M":
			style = style.Foreground(ghYellow)
			tag = " M"
		case "U", "A":
			style = style.Foreground(ghGreen)
			tag = " U"
		case "D":
			style = style.Foreground(ghRed)
			tag = " D"
		}
		if strings.HasPrefix(name, ".") && flag == "" {
			style = style.Foreground(ghDim)
		}
		label := indent + icon + name
		room := w - ansi.StringWidth(tag)
		line := fitLine(label, room) + tag
		lines = append(lines, style.Render(line))
	}

	// Tarefas: reaproveita o que o rastreador já tem.
	if st := m.taskStatusLines(); len(st) > 0 {
		lines = append(lines, bg.Render(strings.Repeat(" ", w)))
		lines = append(lines, head.Render(fitLine(" TAREFAS", w)))
		for _, s := range st {
			lines = append(lines, bg.Render(fitLine(" "+s, w)))
		}
	}

	if len(lines) > h {
		lines = lines[:h]
	}
	for len(lines) < h {
		lines = append(lines, bg.Render(strings.Repeat(" ", w)))
	}
	return strings.Join(lines, "\n")
}

// taskStatusLines devolve linhas curtas das tarefas atuais, se houver.
func (m *Model) taskStatusLines() []string {
	if m.commandDeps.GetTaskStatus == nil {
		return nil
	}
	raw := strings.TrimSpace(m.commandDeps.GetTaskStatus())
	if raw == "" {
		return nil
	}
	var out []string
	for _, l := range strings.Split(raw, "\n") {
		l = strings.TrimSpace(l)
		if l != "" {
			out = append(out, l)
		}
		if len(out) >= 6 {
			break
		}
	}
	return out
}

func (m *Model) renderPanel(w int) string {
	bg := lipgloss.NewStyle().Background(ghBg).Foreground(ghFg)
	tabs := lipgloss.NewStyle().Background(ghBg).Foreground(ghFg).Bold(true).Underline(true).Render(" SAÍDA ") +
		bg.Foreground(ghDim).Render("  ctrl+g oculta")
	top := lipgloss.NewStyle().Foreground(ghLine).Render(strings.Repeat("─", w))
	lines := []string{top, bg.Render(fitLine(tabs, w))}

	body := panelBody(m.ide, panelHeight-2, w-2)
	for _, l := range body {
		lines = append(lines, bg.Render(fitLine(" "+l, w)))
	}
	for len(lines) < panelHeight {
		lines = append(lines, bg.Render(strings.Repeat(" ", w)))
	}
	return strings.Join(lines[:panelHeight], "\n")
}

// panelBody monta o texto do painel: o comando, as últimas linhas da saída e
// o código de saída. Sem comando executado ainda, mostra uma dica.
func panelBody(s ideState, rows, width int) []string {
	dim := lipgloss.NewStyle().Foreground(ghDim)
	if s.lastCmd == "" {
		return []string{dim.Render("Nenhum comando executado ainda.")}
	}
	out := []string{dim.Render("$ ") + s.lastCmd}
	text := strings.Split(strings.TrimRight(s.lastOutput, "\n"), "\n")
	room := rows - 2
	if room < 1 {
		room = 1
	}
	if len(text) > room {
		text = text[len(text)-room:]
	}
	failed := s.lastExit != "" && s.lastExit != "0"
	for _, l := range text {
		l = ansi.Truncate(strings.ReplaceAll(l, "\t", "  "), width, "…")
		if failed {
			l = lipgloss.NewStyle().Foreground(ghRed).Render(l)
		}
		out = append(out, l)
	}
	if s.lastExit != "" {
		out = append(out, dim.Render("exit "+s.lastExit))
	}
	return out
}

func (m *Model) renderStatusLine(w int) string {
	st := lipgloss.NewStyle().Background(ghSel).Foreground(lipgloss.Color("#ffffff"))

	branch := ""
	if m.ide.branch != "" {
		branch = "⎇ " + m.ide.branch
		if len(m.ide.changes) > 0 {
			branch += "*"
		}
	}
	model := ""
	if m.currentModel >= 0 && m.currentModel < len(availableModels) {
		model = shortModelName(availableModels[m.currentModel].DisplayName)
	}
	left := []string{branch, filepath.Base(m.workspaceRoot), model}

	// Contexto: o que o modelo realmente está carregando agora (prompt da
	// última chamada + o que ele gerou), contra o num_ctx configurado.
	used := m.lastOllamaStats.PromptTokens + m.lastOllamaStats.GenTokens
	limit := agentsNumCtx()
	pct := 0
	if limit > 0 {
		pct = used * 100 / limit
	}
	if pct > 100 {
		pct = 100
	}
	const cells = 10
	filled := pct * cells / 100
	meter := strings.Repeat("━", filled) + strings.Repeat("─", cells-filled)

	right := append(m.clockSegments(), fmt.Sprintf("ctx %s %s/%s", meter, kfmt(used), kfmt(limit)))
	if agents.ThinkingEnabled() {
		right = append(right, "pensar on")
	}
	if t := m.lastOllamaStats.TokensPerSecond(); t > 0 {
		right = append(right, fmt.Sprintf("%.0f tok/s", t))
	}
	right = append(right, fmt.Sprintf("%s tokens", kfmt(m.totalTokens)))
	if m.ide.mcpOn > 0 {
		right = append(right, fmt.Sprintf("mcp %d", m.ide.mcpOn))
	}
	if m.toolCallsTotal > 0 {
		right = append(right, fmt.Sprintf("%d ferramentas", m.toolCallsTotal))
	}
	hints := "ctrl+b explorador  ctrl+g painel  /ajuda"

	l := " " + strings.Join(nonEmpty(left), "  ")
	r := strings.Join(right, "  ") + "  "
	// Os atalhos só entram se sobrar espaço: o resto é informação, eles são dica.
	if w-ansi.StringWidth(l)-ansi.StringWidth(r)-ansi.StringWidth(hints)-4 >= 0 {
		r += hints + " "
	}
	gap := w - ansi.StringWidth(l) - ansi.StringWidth(r)
	if gap < 1 {
		r = ""
		gap = max(1, w-ansi.StringWidth(l))
	}
	return st.Render(fitLine(l+strings.Repeat(" ", gap)+r, w))
}

func nonEmpty(in []string) []string {
	var out []string
	for _, s := range in {
		if s != "" {
			out = append(out, s)
		}
	}
	return out
}

func kfmt(n int) string {
	if n >= 1000 {
		return fmt.Sprintf("%.1fk", float64(n)/1000)
	}
	return fmt.Sprintf("%d", n)
}

func agentsNumCtx() int { return agents.DefaultOllamaOptions().NumCtx }

// recordIfCommand alimenta o painel com o resultado de execute_command.
// Os resultados podem vir em ordem diferente das requisições (execução em
// paralelo), então o comando é buscado por posição só quando os tamanhos
// batem; do contrário o painel mostra apenas o nome da ferramenta.
func (m *Model) recordIfCommand(reqs []mcp.Request, i int, res mcp.Result) {
	if !strings.Contains(strings.ToLower(res.Tool), "exec") {
		return
	}
	cmd := "execute_command"
	if len(reqs) > i && strings.Contains(strings.ToLower(reqs[i].Tool), "exec") && reqs[i].Command != "" {
		cmd = reqs[i].Command
	}
	if res.Status != "success" {
		m.recordCommand(cmd, res.Error, "1")
		return
	}
	m.recordCommand(cmd, res.Content, "0")
}

// fitHeight corta ou completa com linhas vazias até ter exatamente h linhas.
func fitHeight(s string, h int) string {
	lines := strings.Split(s, "\n")
	if len(lines) > h {
		lines = lines[len(lines)-h:]
	}
	for len(lines) < h {
		lines = append(lines, "")
	}
	return strings.Join(lines, "\n")
}

// paint pinta o fundo de um bloco inteiro. O lipgloss encerra cada trecho
// colorido com um reset, que apagaria o fundo no meio da linha; por isso o
// fundo é reaplicado depois de cada reset e a linha é completada até w.
func paint(s string, w int, bg lipgloss.Color) string {
	on := bgSeq(bg)

	lines := strings.Split(s, "\n")
	for i, l := range lines {
		l = strings.ReplaceAll(l, "\x1b[0m", "\x1b[0m"+on)
		l = strings.ReplaceAll(l, "\x1b[m", "\x1b[m"+on)
		if pad := w - ansi.StringWidth(l); pad > 0 {
			l += strings.Repeat(" ", pad)
		}
		lines[i] = on + l + "\x1b[0m"
	}
	return strings.Join(lines, "\n")
}

// bgSeq devolve o código ANSI que liga o fundo bg e o texto claro padrão.
func bgSeq(bg lipgloss.Color) string {
	var r, g, b, fr, fg, fb int
	fmt.Sscanf(string(bg), "#%02x%02x%02x", &r, &g, &b)
	fmt.Sscanf(string(ghFg), "#%02x%02x%02x", &fr, &fg, &fb)
	return fmt.Sprintf("\x1b[48;2;%d;%d;%dm\x1b[38;2;%d;%d;%dm", r, g, b, fr, fg, fb)
}
