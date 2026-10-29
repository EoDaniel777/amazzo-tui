package app

import (
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"
)

// Paleta térmica: o humano é quente, a máquina é fria, o pensamento fica
// apagado por ser interior. Nenhuma cor de fundo é pintada — o terminal do
// usuário continua sendo dele.
var (
	colEmber    = lipgloss.Color("#E8A33D") // a voz do usuário
	colSignal   = lipgloss.Color("#5FD4C4") // a voz do modelo
	colThought  = lipgloss.Color("#9B87C4") // raciocínio
	colAct      = lipgloss.Color("#7FBF5F") // ação no mundo (ferramentas)
	colAlarm    = lipgloss.Color("#E5484D") // erro
	colGraphite = lipgloss.Color("#7A8394") // estrutura e métricas
)

// railGlyph e o trilho vertical que substitui as meias-caixas antigas. Sem
// lado direito nao ha o que desalinhar, e o prefixo e aplicado linha a linha.
const railGlyph = "▌"

// entry desenha um bloco de transcrição: trilho colorido à esquerda, um
// cabeçalho com o autor e (opcionalmente) métricas alinhadas à direita.
type entry struct {
	rail  lipgloss.Color
	label string
	meta  string // métricas à direita do cabeçalho; pode ser vazio
	body  string
	width int
}

// render monta o bloco completo.
func (e entry) render() string {
	rail := lipgloss.NewStyle().Foreground(e.rail).Render(railGlyph)
	dim := lipgloss.NewStyle().Foreground(colGraphite)

	width := e.width
	if width < 24 {
		width = 24
	}
	// 2 células do trilho + o espaço depois dele.
	inner := width - 2

	var out []string

	if e.label != "" {
		label, meta := e.label, e.meta

		// Sem espaço para os dois, as métricas cedem primeiro: elas são
		// referência, o autor da fala é identidade.
		if ansi.StringWidth(label)+ansi.StringWidth(meta)+1 > inner {
			meta = ""
		}
		if ansi.StringWidth(label) > inner {
			label = ansi.Truncate(label, inner, "…")
		}

		head := lipgloss.NewStyle().Foreground(e.rail).Bold(true).Render(label)
		if meta != "" {
			gap := inner - ansi.StringWidth(label) - ansi.StringWidth(meta)
			if gap < 1 {
				gap = 1
			}
			head += strings.Repeat(" ", gap) + dim.Render(meta)
		}
		out = append(out, rail+" "+head)
	}

	for _, line := range strings.Split(wrapText(e.body, inner), "\n") {
		out = append(out, rail+" "+line)
	}

	return strings.Join(out, "\n")
}

// renderUserEntry desenha o que o usuário enviou.
func renderUserEntry(text string, width int) string {
	return entry{rail: colEmber, label: "você", body: text, width: width}.render()
}

// renderAssistantEntry desenha a resposta do modelo. meta carrega o custo real
// da geração: com modelo local isso é informação, não enfeite.
func renderAssistantEntry(model, meta, body string, width int) string {
	return entry{rail: colSignal, label: model, meta: meta, body: body, width: width}.render()
}

// renderThoughtLive desenha o raciocínio enquanto ele acontece.
func renderThoughtLive(text string, elapsed time.Duration, width int) string {
	meta := fmt.Sprintf("pensando · %s", formatElapsed(elapsed))
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
	return lipgloss.NewStyle().Foreground(colAct).Render("▸ ") +
		lipgloss.NewStyle().Foreground(colAct).Render(text)
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
	borda := colGraphite
	if ativo {
		borda = colEmber
	}

	return lipgloss.NewStyle().
		Width(width-2).
		Padding(0, 1).
		BorderStyle(lipgloss.RoundedBorder()).
		BorderForeground(borda).
		Render(content)
}
