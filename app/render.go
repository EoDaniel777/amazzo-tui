package app

import (
	"fmt"
	"strings"
	"time"

	"amazzo/mcp"

	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"
)

// Paleta térmica: o humano é quente, a máquina é fria, o pensamento fica
// apagado por ser interior. Nenhuma cor de fundo é pintada — o terminal do
// usuário continua sendo dele.
var (
	colEmber    = lipgloss.Color("#3fb950") // a voz do usuário (verde GitHub)
	colSignal   = lipgloss.Color("#58a6ff") // a voz do modelo (azul GitHub)
	colThought  = lipgloss.Color("#8b949e") // raciocínio
	colAct      = lipgloss.Color("#d29922") // ação no mundo (ferramentas)
	colAlarm    = lipgloss.Color("#f85149") // erro
	colGraphite = lipgloss.Color("#8b949e") // estrutura e métricas
	colPath     = lipgloss.Color("#ffa657") // caminhos e comandos em destaque
)

// railGlyph e o trilho vertical que substitui as meias-caixas antigas. Sem
// lado direito nao ha o que desalinhar, e o prefixo e aplicado linha a linha.
const railGlyph = "▌"

// entry desenha um bloco de transcrição: trilho colorido à esquerda, um
// cabeçalho com o autor e (opcionalmente) métricas alinhadas à direita.
type entry struct {
	bar   bool // filete colorido à esquerda (mensagens do usuário)
	rail  lipgloss.Color
	label string
	meta  string // métricas à direita do cabeçalho; pode ser vazio
	body  string
	width int
}

// render monta o bloco completo: cabeçalho (autor, hora e métricas à direita)
// e o corpo logo abaixo, sem moldura. A cor do autor é o que separa as falas.
func (e entry) render() string {
	dim := lipgloss.NewStyle().Foreground(colGraphite)

	width := e.width
	if width < 24 {
		width = 24
	}
	inner := width - 2

	var out []string

	if e.label != "" {
		label, meta := e.label, e.meta
		hora := time.Now().Format("15:04")

		// Sem espaço para os dois, as métricas cedem primeiro: elas são
		// referência, o autor da fala é identidade.
		if ansi.StringWidth(label)+ansi.StringWidth(hora)+ansi.StringWidth(meta)+2 > inner {
			meta = ""
		}
		if ansi.StringWidth(label)+ansi.StringWidth(hora)+1 > inner {
			hora = ""
		}
		if ansi.StringWidth(label) > inner {
			label = ansi.Truncate(label, inner, "…")
		}

		head := lipgloss.NewStyle().Foreground(e.rail).Bold(true).Render(label)
		used := ansi.StringWidth(label)
		if hora != "" {
			head += " " + dim.Render(hora)
			used += 1 + ansi.StringWidth(hora)
		}
		if meta != "" {
			gap := inner - used - ansi.StringWidth(meta)
			if gap < 1 {
				gap = 1
			}
			head += strings.Repeat(" ", gap) + dim.Render(meta)
		}
		out = append(out, head)
	}

	out = append(out, strings.Split(wrapText(e.body, inner-2), "\n")...)
	if e.bar {
		filete := lipgloss.NewStyle().Foreground(e.rail).Render("▎") + " "
		for i := range out {
			out[i] = filete + out[i]
		}
	}
	return strings.Join(out, "\n")
}

// shortModelName tira o emoji e a tag ":latest" do nome para exibição.
func shortModelName(name string) string {
	name = strings.TrimSpace(name)
	if i := strings.Index(name, " "); i > 0 && !isASCII(name[:i]) {
		name = strings.TrimSpace(name[i:])
	}
	name = strings.TrimSuffix(name, ":latest")
	// "dono/modelo:4b-instruct-2507-q4_K_M" vira "modelo 4b".
	if i := strings.LastIndex(name, "/"); i >= 0 {
		name = name[i+1:]
	}
	if i := strings.Index(name, ":"); i > 0 {
		tag := name[i+1:]
		if j := strings.Index(tag, "-"); j > 0 {
			tag = tag[:j]
		}
		name = name[:i] + " " + tag
	}
	return name
}

func isASCII(s string) bool {
	for _, r := range s {
		if r > 127 {
			return false
		}
	}
	return true
}

// renderUserEntry desenha o que o usuário enviou.
func renderUserEntry(text string, width int) string {
	return entry{bar: true, rail: colEmber, label: "você", body: text, width: width}.render()
}

// renderAssistantEntry desenha a resposta do modelo. meta carrega o custo real
// da geração: com modelo local isso é informação, não enfeite.
func renderAssistantEntry(model, meta, body string, width int) string {
	return entry{rail: colSignal, label: shortModelName(model), meta: meta, body: body, width: width}.render()
}

// renderThoughtLive desenha o raciocínio enquanto ele acontece.
func renderThoughtLive(text string, elapsed time.Duration, width int) string {
	meta := fmt.Sprintf("%s · %s", palavraDeEspera(time.Now().Add(-elapsed), time.Now()), formatElapsed(elapsed))
	return entry{rail: colThought, label: "raciocínio", meta: meta, body: text, width: width}.render()
}

// renderThought desenha o raciocínio já concluído, colapsado ou aberto.
//
// Colapsado ele vira uma linha só: o pensamento continua disponível sem
// empurrar a resposta para fora da tela.
func renderThought(text string, elapsed time.Duration, expanded bool, width int) string {
	resumo := fmt.Sprintf("pensou %s · %s", formatElapsed(elapsed), contarPalavras(text))

	if !expanded {
		linha := lipgloss.NewStyle().Foreground(colThought).Render("▸ " + resumo)
		hint := lipgloss.NewStyle().Foreground(colGraphite).Render("  ctrl+r abre")
		return linha + hint
	}

	head := lipgloss.NewStyle().Foreground(colThought).Render("▾ " + resumo)
	hint := lipgloss.NewStyle().Foreground(colGraphite).Render("  ctrl+r fecha")
	corpo := entry{rail: colThought, body: text, width: width}.render()
	return head + hint + "\n" + corpo
}

// renderToolEvent desenha uma ação sobre o projeto: uma linha, sem trilho,
// porque é um evento e não uma fala.
func renderToolEvent(text string) string {
	verbo, resto := text, ""
	if i := strings.Index(text, " "); i > 0 {
		verbo, resto = text[:i], text[i:]
	}
	return lipgloss.NewStyle().Foreground(colSignal).Render("◆ ") +
		lipgloss.NewStyle().Foreground(colSignal).Bold(true).Render(verbo) +
		lipgloss.NewStyle().Foreground(colGraphite).Render(resto)
}

// renderNotice desenha um aviso do próprio Amazzo (não do modelo).
func renderNotice(icon, text string, c lipgloss.Color) string {
	return lipgloss.NewStyle().Foreground(c).Render(icon+" ") +
		lipgloss.NewStyle().Foreground(c).Render(text)
}

// formatElapsed encurta durações para caber no cabeçalho.
func formatElapsed(d time.Duration) string {
	switch {
	case d < time.Second:
		return "<1s"
	case d < time.Minute:
		return fmt.Sprintf("%.0fs", d.Seconds())
	default:
		return fmt.Sprintf("%dm%02ds", int(d.Minutes()), int(d.Seconds())%60)
	}
}

// contarPalavras dá uma medida do tamanho do raciocínio que faz sentido para
// quem lê (palavras), em vez de tokens, que só fazem sentido para o modelo.
func contarPalavras(s string) string {
	n := len(strings.Fields(s))
	if n == 1 {
		return "1 palavra"
	}
	return fmt.Sprintf("%d palavras", n)
}

// renderStatusBar desenha o cabeçalho fixo.
//
// A hierarquia importa mais que a decoração: o modelo ativo é o único item
// destacado, porque é o que muda o comportamento de tudo. Workspace e métricas
// ficam apagados — são referência, não decisão.
func renderStatusBar(model, workspace, meta string, width int) string {
	destaque := lipgloss.NewStyle().Foreground(colSignal).Bold(true).Render(model)
	dim := lipgloss.NewStyle().Foreground(colGraphite)

	esquerda := " " + destaque + dim.Render("  "+workspace)
	direita := dim.Render(meta + " ")

	gap := width - ansi.StringWidth(esquerda) - ansi.StringWidth(direita)
	if gap < 1 {
		// Sem espaço para os dois: o modelo ativo é o que não pode sumir.
		return ansi.Truncate(esquerda, width, "…")
	}
	return esquerda + strings.Repeat(" ", gap) + direita
}

// renderFooter desenha a régua de atalhos.
func renderFooter(items []string, width int) string {
	dim := lipgloss.NewStyle().Foreground(colGraphite)
	sep := dim.Render(" · ")

	linha := " " + strings.Join(items, "\x00")
	linha = strings.ReplaceAll(dim.Render(linha), "\x00", sep)
	return ansi.Truncate(linha, width, "")
}

// renderInputFrame desenha a área de digitação.
//
// É a única moldura fechada da interface: o resto é transcrição, aqui é onde
// você age. A borda acende quando o campo está esperando por você.
func renderInputFrame(content string, width int, ativo bool) string {
	borda := ghLine
	if ativo {
		borda = colSignal
	}

	return lipgloss.NewStyle().
		Width(width-2).
		Padding(0, 1).
		BorderStyle(lipgloss.RoundedBorder()).
		BorderForeground(borda).
		Render(content)
}

// renderToolErrorCard desenha a falha de uma ferramenta num cartão de borda
// vermelha, com uma dica quando o erro é de um tipo conhecido.
func renderToolErrorCard(res mcp.Result, width int) string {
	red := lipgloss.NewStyle().Foreground(colAlarm)
	dim := lipgloss.NewStyle().Foreground(colGraphite)

	titulo := red.Render("✗ ") + lipgloss.NewStyle().Bold(true).Render(strings.ToLower(res.Tool))
	if res.Path != "" {
		titulo += " " + dim.Render(res.Path)
	}
	linhas := []string{titulo, dim.Render(strings.TrimSpace(res.Error))}
	// A saída do comando (stderr) é o que explica a falha.
	if saida := strings.Split(strings.TrimSpace(res.Content), "\n"); strings.TrimSpace(res.Content) != "" {
		if len(saida) > 4 {
			saida = saida[len(saida)-4:]
		}
		for _, l := range saida {
			linhas = append(linhas, dim.Render(l))
		}
	}

	erro := strings.ToLower(res.Error)
	if strings.Contains(erro, "127") || strings.Contains(erro, "not found") || strings.Contains(erro, "não encontrado") {
		linhas = append(linhas,
			lipgloss.NewStyle().Foreground(colAct).Render("dica: ")+
				"comando fora do PATH. Instale ou compile o binário antes de rodar.")
	}

	if width < 30 {
		width = 30
	}
	return lipgloss.NewStyle().
		Width(width-4).
		Padding(0, 1).
		BorderStyle(lipgloss.RoundedBorder()).
		BorderForeground(colAlarm).
		Render(strings.Join(linhas, "\n"))
}
