package app

import (
	"fmt"

	"github.com/charmbracelet/lipgloss"
)

// ModernStyles define estilos modernos inspirados no Warp Terminal
type ModernStyles struct {
	// Blocos principais
	BlockStyle      lipgloss.Style
	BlockTitleStyle lipgloss.Style
	BlockBodyStyle  lipgloss.Style

	// Input/Output específicos
	InputBlockStyle  lipgloss.Style
	OutputBlockStyle lipgloss.Style

	// Cores modernas
	AccentColor     lipgloss.Color
	BackgroundColor lipgloss.Color
	BorderColor     lipgloss.Color
	ShadowColor     lipgloss.Color

	// Texto
	TextPrimary   lipgloss.Color
	TextSecondary lipgloss.Color
	TextMuted     lipgloss.Color
}

// NewModernStyles cria estilos modernos baseados no tema
func NewModernStyles(isDark bool) ModernStyles {
	var s ModernStyles

	if isDark {
		// Tema escuro (llm-term/waveterm-like) - cores mais vibrantes
		s.AccentColor = lipgloss.Color("#00FFD9")     // Ciano elétrico (mais vibrante)
		s.BackgroundColor = lipgloss.Color("#0D0D0D") // Preto profundo
		s.BorderColor = lipgloss.Color("#50FA7B")     // Verde néon para bordas
		s.ShadowColor = lipgloss.Color("#000000")     // Preto puro
		s.TextPrimary = lipgloss.Color("#F8F8F2")     // Branco quente
		s.TextSecondary = lipgloss.Color("#8BE9FD")   // Ciano claro
		s.TextMuted = lipgloss.Color("#6272A4")       // Azul acinzentado
	} else {
		// Tema claro
		s.AccentColor = lipgloss.Color("#0080FF")
		s.BackgroundColor = lipgloss.Color("#FFFFFF")
		s.BorderColor = lipgloss.Color("#E0E0E0")
		s.ShadowColor = lipgloss.Color("#F0F0F0")
		s.TextPrimary = lipgloss.Color("#1E1E2E")
		s.TextSecondary = lipgloss.Color("#45475A")
		s.TextMuted = lipgloss.Color("#7F849C")
	}

	// Estilo de bloco base - espaçamento compacto (Tododo style)
	s.BlockStyle = lipgloss.NewStyle().
		Padding(1, 2).
		Margin(0, 0, 0, 0).
		BorderStyle(lipgloss.RoundedBorder()).
		BorderForeground(s.BorderColor)

	// Título do bloco - mais destaque
	s.BlockTitleStyle = lipgloss.NewStyle().
		Bold(true).
		Foreground(s.AccentColor).
		Padding(0, 1).
		MarginBottom(0)

	// Corpo do bloco
	s.BlockBodyStyle = lipgloss.NewStyle().
		Foreground(s.TextPrimary).
		Padding(0, 0)

	// Bloco de input - borda dupla vibrante e compacto
	s.InputBlockStyle = s.BlockStyle.
		BorderForeground(s.AccentColor).
		BorderStyle(lipgloss.DoubleBorder()).
		Padding(0, 1)

	// Bloco de output - borda verde néon sutil e compacto
	s.OutputBlockStyle = s.BlockStyle.
		BorderStyle(lipgloss.RoundedBorder()).
		BorderForeground(s.BorderColor).
		Padding(1, 2)

	return s
}

// RenderInputBlock renderiza um bloco de input moderno estilo Warp
func (s *ModernStyles) RenderInputBlock(title, content string, width int) string {
	// Título com ícone mais moderno e espaçamento
	titleBar := s.BlockTitleStyle.Render("▸ " + title)

	// Bloco de input com borda dupla para destaque
	box := lipgloss.NewStyle().
		Width(width).
		Padding(1, 2).
		BorderStyle(lipgloss.DoubleBorder()).
		BorderForeground(s.AccentColor).
		Render(titleBar + "\n" + content)

	return box
}

// RenderOutputBlock renderiza um bloco de output moderno estilo Warp
func (s *ModernStyles) RenderOutputBlock(title, content string, width int) string {
	// Título com ícone clean
	titleBar := s.BlockTitleStyle.Render("◇ " + title)

	// Bloco de output com borda sutil e padding compacto
	box := lipgloss.NewStyle().
		Width(width).
		Padding(1, 2).
		BorderStyle(lipgloss.RoundedBorder()).
		BorderForeground(s.BorderColor).
		Render(titleBar + "\n" + content)

	return box
}

// RenderStatusBar renderiza uma barra de status minimalista estilo Warp
func (s *ModernStyles) RenderStatusBar(items ...string) string {
	statusStyle := lipgloss.NewStyle().
		Foreground(s.TextSecondary).
		Padding(0, 1)

	separatorStyle := lipgloss.NewStyle().
		Foreground(s.BorderColor).
		Padding(0, 1)

	var parts []string
	for i, item := range items {
		if i > 0 {
			parts = append(parts, separatorStyle.Render("│")) // Separador vertical
		}
		parts = append(parts, statusStyle.Render(item))
	}

	// Barra completa com padding e background sutil
	bar := lipgloss.NewStyle().
		Padding(0, 2).
		Background(s.BackgroundColor).
		Render(lipgloss.JoinHorizontal(lipgloss.Left, parts...))

	return bar
}

// RenderCard renderiza um card com sombra sutil
func (s *ModernStyles) RenderCard(content string, width int) string {
	card := lipgloss.NewStyle().
		Width(width).
		Padding(2, 3).
		Background(s.BackgroundColor).
		BorderStyle(lipgloss.RoundedBorder()).
		BorderForeground(s.BorderColor).
		Render(content)

	return card
}

// RenderUserMessage renderiza uma mensagem do usuário estilo llm-term
func (s *ModernStyles) RenderUserMessage(content string, width int) string {
	// Ícone do usuário com cor destaque
	userIcon := lipgloss.NewStyle().
		Foreground(s.AccentColor).
		Bold(true).
		Render("😊")

	// Prefixo da mensagem
	prefix := lipgloss.NewStyle().
		Foreground(s.AccentColor).
		Bold(true).
		Render("Você:")

	// Conteúdo da mensagem
	messageStyle := lipgloss.NewStyle().
		Foreground(s.TextPrimary).
		PaddingLeft(2)

	// Monta a mensagem
	header := lipgloss.JoinHorizontal(lipgloss.Left, userIcon, " ", prefix)
	message := messageStyle.Render(content)

	return lipgloss.JoinVertical(lipgloss.Left, header, message, "")
}

// RenderAssistantMessage renderiza uma mensagem do assistente estilo llm-term
func (s *ModernStyles) RenderAssistantMessage(content string, width int) string {
	// Ícone do assistente
	assistantIcon := lipgloss.NewStyle().
		Foreground(lipgloss.Color("#FF79C6")). // Rosa/magenta
		Bold(true).
		Render("🤖")

	// Prefixo da mensagem
	prefix := lipgloss.NewStyle().
		Foreground(lipgloss.Color("#FF79C6")).
		Bold(true).
		Render("Amazzo:")

	// Conteúdo da mensagem com card
	messageStyle := lipgloss.NewStyle().
		Foreground(s.TextPrimary).
		PaddingLeft(2).
		PaddingTop(1).
		PaddingBottom(1).
		BorderStyle(lipgloss.RoundedBorder()).
		BorderForeground(s.BorderColor).
		Width(width - 6)

	// Monta a mensagem
	header := lipgloss.JoinHorizontal(lipgloss.Left, assistantIcon, " ", prefix)
	message := messageStyle.Render(content)

	return lipgloss.JoinVertical(lipgloss.Left, header, message, "")
}

// RenderLoadingSpinner renderiza um indicador de carregamento animado
func (s *ModernStyles) RenderLoadingSpinner(spinnerView string) string {
	// Ícone do assistente pensando
	thinkingIcon := lipgloss.NewStyle().
		Foreground(lipgloss.Color("#FF79C6")).
		Bold(true).
		Render("🤖")

	// Texto de carregamento
	loadingText := lipgloss.NewStyle().
		Foreground(s.AccentColor).
		Bold(true).
		Render(fmt.Sprintf("%s trabalhando...", spinnerView))

	// Card de loading
	card := lipgloss.NewStyle().
		Padding(1, 2).
		BorderStyle(lipgloss.RoundedBorder()).
		BorderForeground(s.AccentColor).
		Render(lipgloss.JoinHorizontal(lipgloss.Left, thinkingIcon, " ", loadingText))

	return card
}

// RenderCodeBlock renderiza um bloco de código estilo waveterm
func (s *ModernStyles) RenderCodeBlock(language, code string, width int) string {
	// Header do bloco de código com linguagem
	header := lipgloss.NewStyle().
		Foreground(s.TextSecondary).
		Background(lipgloss.Color("#282A36")). // Fundo mais escuro
		Padding(0, 2).
		Bold(true).
		Render(fmt.Sprintf("📄 %s", language))

	// Conteúdo do código
	codeStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("#F8F8F2")).
		Background(lipgloss.Color("#1E1E2E")).
		Padding(1, 2).
		Width(width - 6)

	// Borda do bloco
	blockStyle := lipgloss.NewStyle().
		BorderStyle(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color("#50FA7B")).
		Width(width - 4)

	content := lipgloss.JoinVertical(lipgloss.Left, header, codeStyle.Render(code))
	return blockStyle.Render(content)
}

// RenderInfoMessage renderiza uma mensagem informativa estilo waveterm
func (s *ModernStyles) RenderInfoMessage(icon, message string) string {
	iconStyle := lipgloss.NewStyle().
		Foreground(s.AccentColor).
		Bold(true).
		Render(icon)

	messageStyle := lipgloss.NewStyle().
		Foreground(s.TextSecondary).
		PaddingLeft(1).
		Render(message)

	return lipgloss.JoinHorizontal(lipgloss.Left, iconStyle, messageStyle)
}

// RenderErrorMessage renderiza uma mensagem de erro estilo waveterm
func (s *ModernStyles) RenderErrorMessage(message string) string {
	errorIcon := lipgloss.NewStyle().
		Foreground(lipgloss.Color("#FF5555")).
		Bold(true).
		Render("❌")

	errorText := lipgloss.NewStyle().
		Foreground(lipgloss.Color("#FF5555")).
		PaddingLeft(1).
		Render(message)

	card := lipgloss.NewStyle().
		Padding(1, 2).
		BorderStyle(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color("#FF5555")).
		Render(lipgloss.JoinHorizontal(lipgloss.Left, errorIcon, errorText))

	return card
}

// RenderSuccessMessage renderiza uma mensagem de sucesso estilo waveterm
func (s *ModernStyles) RenderSuccessMessage(message string) string {
	successIcon := lipgloss.NewStyle().
		Foreground(s.BorderColor).
		Bold(true).
		Render("✅")

	successText := lipgloss.NewStyle().
		Foreground(s.BorderColor).
		PaddingLeft(1).
		Render(message)

	return lipgloss.JoinHorizontal(lipgloss.Left, successIcon, successText)
}

// RenderStreamingPreview mostra o texto que o modelo local esta gerando agora.
//
// Com um modelo rodando na CPU a resposta leva dezenas de segundos; sem essa
// previa a tela fica parada e parece travada.
func (s *ModernStyles) RenderStreamingPreview(content string, width int) string {
	return lipgloss.NewStyle().
		Width(width).
		Padding(0, 2).
		Foreground(s.TextMuted).
		BorderStyle(lipgloss.NormalBorder()).
		BorderLeft(true).
		BorderTop(false).
		BorderRight(false).
		BorderBottom(false).
		BorderForeground(s.AccentColor).
		Render(content)
}
