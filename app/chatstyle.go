package app

import (
	"fmt"
	"regexp"
	"strings"
	"unicode"

	"amazzo/mcp"

	"github.com/charmbracelet/glamour/ansi"
	"github.com/charmbracelet/glamour/styles"
	"github.com/charmbracelet/lipgloss"
	xansi "github.com/charmbracelet/x/ansi"
)

// ---------- histórico redesenhável ----------

// As mensagens da conversa são guardadas já renderizadas (com cor e largura).
// Para que trocar de tema ou de tamanho de janela refaça tudo, as principais
// registram uma função que sabe se desenhar de novo.

type rerenderItem struct {
	fn   func() string
	last string // o que foi gravado em m.messages; se mudou, o item foi trocado
}

// addRerenderable adiciona uma mensagem que pode ser redesenhada depois.
func (m *Model) addRerenderable(fn func() string) {
	m.addMessage(fn())
	idx := len(m.messages) - 1
	if m.rerender == nil {
		m.rerender = map[int]rerenderItem{}
	}
	m.rerender[idx] = rerenderItem{fn: fn, last: m.messages[idx]}
}

// rerenderHistory redesenha as mensagens registradas, com as cores e a
// largura atuais.
func (m *Model) rerenderHistory() {
	atBottom := m.viewport.AtBottom()
	for idx, it := range m.rerender {
		if idx >= len(m.messages) || m.messages[idx] != it.last {
			delete(m.rerender, idx)
			continue
		}
		s := wrapText(it.fn(), m.viewport.Width)
		m.messages[idx] = s
		m.rerender[idx] = rerenderItem{fn: it.fn, last: s}
	}
	for _, t := range m.thoughts {
		if t.index >= 0 && t.index < len(m.messages) {
			m.messages[t.index] = renderThought(t.text, t.elapsed, m.showThinking, m.viewport.Width)
		}
	}
	m.viewport.SetContent(strings.Join(m.messages, "\n"))
	if atBottom {
		m.viewport.GotoBottom()
	}
}

// ---------- resultado das ferramentas ----------

// stripEmoji tira emoji e símbolos que muitas fontes de terminal não desenham.
func stripEmoji(s string) string {
	var b strings.Builder
	for _, r := range s {
		switch {
		case r >= 0x1F000, r >= 0x2600 && r <= 0x27BF, r == 0xFE0F, r == 0x200D:
			continue
		}
		b.WriteRune(r)
	}
	return strings.TrimSpace(b.String())
}

var numeroRe = regexp.MustCompile(`\d+`)

func linhasUteis(s string) []string {
	var out []string
	for _, l := range strings.Split(s, "\n") {
		if l = strings.TrimSpace(l); l != "" {
			out = append(out, l)
		}
	}
	return out
}

func bytesFmt(n int) string {
	switch {
	case n >= 1<<20:
		return fmt.Sprintf("%.1f MB", float64(n)/(1<<20))
	case n >= 1<<10:
		return fmt.Sprintf("%.1f KB", float64(n)/(1<<10))
	}
	return fmt.Sprintf("%d bytes", n)
}

// toolResultBlock desenha o resultado de uma ferramenta: uma linha "✓ resumo"
// e, quando há lista, uma prévia curta. O conteúdo completo continua indo
// para o modelo; aqui é só o que você precisa ver.
func toolResultBlock(res mcp.Result, width int) string {
	if res.Status != "success" {
		return renderToolErrorCard(res, width)
	}
	ok := lipgloss.NewStyle().Foreground(ghGreen)
	dim := lipgloss.NewStyle().Foreground(colGraphite)
	check := func(resumo string) string { return "  " + ok.Render("✓") + " " + resumo }
	preview := func(itens []string, max int) string {
		if len(itens) == 0 {
			return ""
		}
		mostra := itens
		if len(mostra) > max {
			mostra = mostra[:max]
		}
		txt := "\n" + dim.Render("    "+strings.Join(mostra, "   "))
		if len(itens) > max {
			txt += dim.Render(fmt.Sprintf("   … +%d", len(itens)-max))
		}
		return txt
	}

	tool := strings.ToLower(res.Tool)
	linhas := linhasUteis(res.Content)
	primeira := ""
	if len(linhas) > 0 {
		primeira = stripEmoji(linhas[0])
	}

	switch {
	case strings.Contains(tool, "read"):
		n := strings.Count(res.Content, "\n")
		return check(fmt.Sprintf("%s  %s", res.Path, dim.Render(fmt.Sprintf("%d linhas · %s", n, bytesFmt(len(res.Content))))))

	case strings.Contains(tool, "list"):
		var nomes []string
		for _, l := range strings.Split(res.Content, "\n") {
			if t := strings.TrimSpace(l); strings.HasPrefix(t, "- ") {
				nomes = append(nomes, strings.TrimPrefix(t, "- "))
			}
		}
		if len(nomes) == 0 {
			return check(primeira)
		}
		return check(fmt.Sprintf("%d itens", len(nomes))) + preview(nomes, 8)

	case strings.Contains(tool, "search"):
		total := 0
		if n := numeroRe.FindString(primeira); n != "" {
			fmt.Sscanf(n, "%d", &total)
		}
		var itens []string
		for _, l := range linhas[1:min(len(linhas), 40)] {
			if !strings.HasPrefix(l, "...") && !strings.HasPrefix(l, "⚠") {
				itens = append(itens, stripEmoji(l))
			}
		}
		resumo := fmt.Sprintf("%d resultados", total)
		if total == 0 {
			resumo = primeira
		}
		return check(resumo) + "\n" + dim.Render("    "+strings.Join(limita(itens, 5), "\n    ")) + maisLinhas(len(itens), 5, dim)

	case strings.Contains(tool, "exec"):
		if strings.HasPrefix(res.Content, "O comando continua rodando") {
			return "  " + lipgloss.NewStyle().Foreground(colAct).Render("●") + " " + stripEmoji(strings.SplitN(res.Content, "\n", 2)[0])
		}
		saida := linhas
		resumo := "concluído"
		if len(saida) == 0 {
			return check(resumo + dim.Render(" · sem saída"))
		}
		corpo := limitaFim(saida, 10)
		box := lipgloss.NewStyle().BorderStyle(lipgloss.RoundedBorder()).BorderForeground(ghLine).
			Foreground(colGraphite).Padding(0, 1).Render(xansi.Truncate(strings.Join(corpo, "\n"), max(20, width-8), "…"))
		return check(resumo) + "\n" + indenta(box, "    ")

	case strings.Contains(tool, "fetch"):
		return check(fmt.Sprintf("conteúdo web obtido · %d caracteres", len(res.Content)))

	case strings.Contains(tool, "edit"), strings.Contains(tool, "write"), strings.Contains(tool, "append"),
		strings.Contains(tool, "move"), strings.Contains(tool, "make"), strings.Contains(tool, "dir"):
		return check(xansi.Truncate(primeira, max(20, width-8), "…"))
	}
	return check(xansi.Truncate(primeira, max(20, width-8), "…"))
}

func limita(s []string, n int) []string {
	if len(s) > n {
		return s[:n]
	}
	return s
}

func limitaFim(s []string, n int) []string {
	if len(s) > n {
		return s[len(s)-n:]
	}
	return s
}

func maisLinhas(total, mostradas int, st lipgloss.Style) string {
	if total > mostradas {
		return st.Render(fmt.Sprintf("\n    … +%d", total-mostradas))
	}
	return ""
}

func indenta(s, prefixo string) string {
	linhas := strings.Split(s, "\n")
	for i := range linhas {
		linhas[i] = prefixo + linhas[i]
	}
	return strings.Join(linhas, "\n")
}

// ---------- markdown com as cores do tema ----------

func sptr(s string) *string { return &s }
func bptr(b bool) *bool     { return &b }
func uptr(u uint) *uint     { return &u }

// glamourStyleFromPalette monta o estilo do markdown a partir da paleta ativa:
// código inline com fundo suave, blocos de código com moldura de cor, títulos
// sem "###".
func glamourStyleFromPalette() ansi.StyleConfig {
	base := styles.DarkStyleConfig
	if paletteIsLight() {
		base = styles.LightStyleConfig
	}
	cfg := base

	fg, dim := string(ghFg), string(ghDim)
	model, org, panel := string(ghBlue), string(colPath), string(ghPanel)

	cfg.Document.Color = sptr(fg)
	cfg.Text.Color = sptr(fg)
	cfg.Paragraph.Color = sptr(fg)

	titulo := ansi.StyleBlock{StylePrimitive: ansi.StylePrimitive{Color: sptr(model), Bold: bptr(true), BlockSuffix: ""}}
	cfg.Heading = titulo
	cfg.H1, cfg.H2, cfg.H3, cfg.H4, cfg.H5, cfg.H6 = titulo, titulo, titulo, titulo, titulo, titulo

	cfg.Strong = ansi.StylePrimitive{Bold: bptr(true), Color: sptr(fg)}
	cfg.Emph = ansi.StylePrimitive{Italic: bptr(true), Color: sptr(fg)}
	cfg.Link = ansi.StylePrimitive{Color: sptr(model), Underline: bptr(true)}
	cfg.LinkText = ansi.StylePrimitive{Color: sptr(model), Bold: bptr(true)}
	cfg.HorizontalRule = ansi.StylePrimitive{Color: sptr(string(ghLine)), Format: "\n────────\n"}

	cfg.Code = ansi.StyleBlock{StylePrimitive: ansi.StylePrimitive{
		Color: sptr(org), BackgroundColor: sptr(panel), Prefix: " ", Suffix: " ",
	}}

	cfg.CodeBlock.StylePrimitive.Color = sptr(fg)
	cfg.CodeBlock.StylePrimitive.BackgroundColor = sptr(panel)
	cfg.CodeBlock.Margin = uptr(2)
	if cfg.CodeBlock.Chroma != nil {
		c := *cfg.CodeBlock.Chroma
		c.Text.Color = sptr(fg)
		c.Background = ansi.StylePrimitive{BackgroundColor: sptr(panel)}
		c.Comment.Color = sptr(dim)
		c.CommentPreproc.Color = sptr(dim)
		c.Keyword.Color = sptr(model)
		c.KeywordReserved.Color = sptr(model)
		c.KeywordNamespace.Color = sptr(model)
		c.KeywordType.Color = sptr(string(ghYellow))
		c.NameFunction.Color = sptr(string(ghGreen))
		c.NameBuiltin.Color = sptr(string(ghGreen))
		c.LiteralString.Color = sptr(org)
		c.LiteralNumber.Color = sptr(string(ghYellow))
		c.Operator.Color = sptr(dim)
		c.Punctuation.Color = sptr(dim)
		cfg.CodeBlock.Chroma = &c
	}
	return cfg
}

var _ = unicode.IsSpace
