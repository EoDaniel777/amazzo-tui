package app

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
)

// UltimateStyles - O estilo mais incrível e moderno possível
type UltimateStyles struct {
	// Paleta de cores vibrantes
	NeonCyan    lipgloss.Color
	NeonGreen   lipgloss.Color
	NeonPurple  lipgloss.Color
	NeonPink    lipgloss.Color
	NeonOrange  lipgloss.Color
	DeepBlack   lipgloss.Color
	BrightWhite lipgloss.Color
	
	// Gradientes simulados
	GradientColors []lipgloss.Color
}

// NewUltimateStyles cria estilos de última geração
func NewUltimateStyles() UltimateStyles {
	return UltimateStyles{
		NeonCyan:    lipgloss.Color("#00FFFF"),
		NeonGreen:   lipgloss.Color("#39FF14"),
		NeonPurple:  lipgloss.Color("#BF00FF"),
		NeonPink:    lipgloss.Color("#FF10F0"),
		NeonOrange:  lipgloss.Color("#FF6600"),
		DeepBlack:   lipgloss.Color("#000000"),
		BrightWhite: lipgloss.Color("#FFFFFF"),
		GradientColors: []lipgloss.Color{
			lipgloss.Color("#FF006E"),
			lipgloss.Color("#8338EC"),
			lipgloss.Color("#3A86FF"),
			lipgloss.Color("#06FFA5"),
			lipgloss.Color("#FFBE0B"),
		},
	}
}

// RenderAwesomeBanner cria um banner ASCII art incrível
func (u *UltimateStyles) RenderAwesomeBanner(width int) string {
	banner := `
    ▄▀█ █▀▄▀█ ▄▀█ ▀█ ▀█ █▀█
    █▀█ █░▀░█ █▀█ █▄ █▄ █▄█
    
    ╔═══════════════════════════════════╗
    ║  🤖 AI Chat Assistant Deluxe ✨  ║
    ╚═══════════════════════════════════╝
`
	
	// Estiliza com cores vibrantes
	style := lipgloss.NewStyle().
		Foreground(u.NeonCyan).
		Bold(true).
		Align(lipgloss.Center).
		Width(width)
	
	return style.Render(banner)
}

// RenderGlowingUserMessage renderiza mensagem do usuário com efeito glow
func (u *UltimateStyles) RenderGlowingUserMessage(content string, width int) string {
	// Valida largura mínima
	if width < 20 {
		width = 60
	}

	// Header da mensagem
	header := lipgloss.NewStyle().
		Foreground(u.NeonCyan).
		Background(lipgloss.Color("#001F3F")).
		Bold(true).
		Padding(0, 2).
		Width(width - 4).
		Render("👤 VOCÊ")
	
	// Conteúdo da mensagem
	contentStyle := lipgloss.NewStyle().
		Foreground(u.BrightWhite).
		Background(lipgloss.Color("#0A1929")).
		Padding(1, 2).
		Width(width - 4)
	
	// Borda com efeito glow
	border := lipgloss.NewStyle().
		BorderStyle(lipgloss.DoubleBorder()).
		BorderForeground(u.NeonCyan).
		Width(width)
	
	message := lipgloss.JoinVertical(lipgloss.Left,
		header,
		contentStyle.Render(content),
	)
	
	return border.Render(message)
}

// RenderGlowingAssistantMessage renderiza mensagem do assistente com efeito premium
func (u *UltimateStyles) RenderGlowingAssistantMessage(content string, width int) string {
	// Valida largura mínima
	if width < 20 {
		width = 60
	}

	// Header da mensagem com gradiente simulado
	header := lipgloss.NewStyle().
		Foreground(u.NeonPurple).
		Background(lipgloss.Color("#1A0A2E")).
		Bold(true).
		Padding(0, 2).
		Width(width - 4).
		Render("🤖 AMAZZO AI")
	
	// Conteúdo da mensagem
	contentStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("#E0E0E0")).
		Background(lipgloss.Color("#0F0A1E")).
		Padding(1, 2).
		Width(width - 4)
	
	// Borda com efeito glow roxo/rosa
	border := lipgloss.NewStyle().
		BorderStyle(lipgloss.ThickBorder()).
		BorderForeground(u.NeonPurple).
		Width(width)
	
	message := lipgloss.JoinVertical(lipgloss.Left,
		header,
		contentStyle.Render(content),
	)
	
	return border.Render(message)
}

// RenderAwesomeProgressBar barra de progresso com gradiente
func (u *UltimateStyles) RenderAwesomeProgressBar(current, total, width int) string {
	if total == 0 {
		return ""
	}

	// Garante largura mínima
	if width < 10 {
		width = 20
	}

	percentage := float64(current) / float64(total)
	filled := int(percentage * float64(width))
	empty := width - filled

	// Protege contra valores negativos
	if filled < 0 {
		filled = 0
	}
	if empty < 0 {
		empty = 0
	}
	
	// Caracteres com efeito gradiente
	var bar strings.Builder
	
	// Parte preenchida com gradiente
	for i := 0; i < filled; i++ {
		colorIndex := (i * len(u.GradientColors)) / width
		if colorIndex >= len(u.GradientColors) {
			colorIndex = len(u.GradientColors) - 1
		}
		
		char := lipgloss.NewStyle().
			Foreground(u.GradientColors[colorIndex]).
			Render("█")
		bar.WriteString(char)
	}
	
	// Parte vazia
	emptyChar := lipgloss.NewStyle().
		Foreground(lipgloss.Color("#333333")).
		Render("░")
	bar.WriteString(strings.Repeat(emptyChar, empty))
	
	// Adiciona percentual
	percentText := fmt.Sprintf(" %.0f%%", percentage*100)
	percentStyle := lipgloss.NewStyle().
		Foreground(u.NeonGreen).
		Bold(true).
		Render(percentText)
	
	return bar.String() + percentStyle
}

// RenderNeonBox cria uma box com efeito neon
func (u *UltimateStyles) RenderNeonBox(title, content string, color lipgloss.Color, width int) string {
	// Valida largura mínima
	if width < 20 {
		width = 60
	}

	// Título com background escuro
	titleStyle := lipgloss.NewStyle().
		Foreground(color).
		Background(lipgloss.Color("#0A0A0A")).
		Bold(true).
		Padding(0, 2).
		Width(width - 4).
		Align(lipgloss.Center)
	
	// Conteúdo
	contentStyle := lipgloss.NewStyle().
		Foreground(u.BrightWhite).
		Background(lipgloss.Color("#050505")).
		Padding(1, 2).
		Width(width - 4)
	
	// Box com borda neon
	box := lipgloss.NewStyle().
		BorderStyle(lipgloss.RoundedBorder()).
		BorderForeground(color).
		Width(width)
	
	combined := lipgloss.JoinVertical(lipgloss.Left,
		titleStyle.Render(title),
		contentStyle.Render(content),
	)
	
	return box.Render(combined)
}

// RenderAwesomeStatusBar status bar futurista
func (u *UltimateStyles) RenderAwesomeStatusBar(items ...string) string {
	var parts []string
	
	colors := []lipgloss.Color{u.NeonCyan, u.NeonGreen, u.NeonPurple, u.NeonOrange}
	
	for i, item := range items {
		color := colors[i%len(colors)]
		
		styled := lipgloss.NewStyle().
			Foreground(color).
			Bold(true).
			Padding(0, 1).
			Render(item)
		
		parts = append(parts, styled)
		
		if i < len(items)-1 {
			sep := lipgloss.NewStyle().
				Foreground(lipgloss.Color("#444444")).
				Render("│")
			parts = append(parts, sep)
		}
	}
	
	bar := lipgloss.NewStyle().
		Background(lipgloss.Color("#0A0A0A")).
		Padding(0, 1).
		Render(lipgloss.JoinHorizontal(lipgloss.Left, parts...))
	
	return bar
}

// RenderPulsingLoader loader animado com efeito pulsante
func (u *UltimateStyles) RenderPulsingLoader(spinnerView string) string {
	loader := fmt.Sprintf("⚡ %s Processando com IA de última geração... ⚡", spinnerView)
	
	style := lipgloss.NewStyle().
		Foreground(u.NeonCyan).
		Background(lipgloss.Color("#001F3F")).
		Bold(true).
		Padding(1, 3).
		BorderStyle(lipgloss.DoubleBorder()).
		BorderForeground(u.NeonCyan).
		Align(lipgloss.Center)
	
	return style.Render(loader)
}

// RenderSeparator linha separadora com estilo
func (u *UltimateStyles) RenderSeparator(width int, color lipgloss.Color) string {
	// Garante que width seja sempre positivo
	if width < 1 {
		width = 40 // Largura mínima padrão
	}

	line := strings.Repeat("─", width)

	return lipgloss.NewStyle().
		Foreground(color).
		Render(line)
}

// RenderAwesomeInput input box futurista
func (u *UltimateStyles) RenderAwesomeInput(content string, width int) string {
	// Valida largura mínima
	if width < 20 {
		width = 60
	}

	// Título do input
	title := lipgloss.NewStyle().
		Foreground(u.NeonGreen).
		Background(lipgloss.Color("#0A2A0A")).
		Bold(true).
		Padding(0, 2).
		Width(width - 4).
		Render("⚡ DIGITE SUA MENSAGEM")
	
	// Área de input
	inputStyle := lipgloss.NewStyle().
		Foreground(u.BrightWhite).
		Background(lipgloss.Color("#050A05")).
		Padding(0, 1).
		Width(width - 4)
	
	// Borda dupla verde neon
	border := lipgloss.NewStyle().
		BorderStyle(lipgloss.DoubleBorder()).
		BorderForeground(u.NeonGreen).
		Width(width)
	
	combined := lipgloss.JoinVertical(lipgloss.Left,
		title,
		inputStyle.Render(content),
	)
	
	return border.Render(combined)
}

// RenderCodeBlockAwesome bloco de código ultra estilizado
func (u *UltimateStyles) RenderCodeBlockAwesome(language, code string, width int) string {
	// Valida largura mínima
	if width < 20 {
		width = 60
	}

	// Header com linguagem
	header := lipgloss.NewStyle().
		Foreground(u.NeonOrange).
		Background(lipgloss.Color("#2A1A0A")).
		Bold(true).
		Padding(0, 2).
		Width(width - 4).
		Render(fmt.Sprintf("📝 %s", strings.ToUpper(language)))
	
	// Código com syntax highlight simulado
	codeStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("#F8F8F2")).
		Background(lipgloss.Color("#1E1E1E")).
		Padding(1, 2).
		Width(width - 4)
	
	// Borda laranja neon
	border := lipgloss.NewStyle().
		BorderStyle(lipgloss.RoundedBorder()).
		BorderForeground(u.NeonOrange).
		Width(width)
	
	combined := lipgloss.JoinVertical(lipgloss.Left,
		header,
		codeStyle.Render(code),
	)
	
	return border.Render(combined)
}
