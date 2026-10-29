package app

import (
	"github.com/charmbracelet/lipgloss"
)

// WarpStyles - Sistema de design profissional inspirado no Warp Terminal
// Cores, tipografia e espaçamento cuidadosamente escolhidos
type WarpStyles struct {
	// Paleta de cores profissional
	Primary       lipgloss.Color // Cor principal (texto)
	Secondary     lipgloss.Color // Texto secundário
	Muted         lipgloss.Color // Texto desabilitado
	Accent        lipgloss.Color // Destaque (links, ações)
	Success       lipgloss.Color // Verde para sucesso
	Warning       lipgloss.Color // Amarelo para avisos
	Error         lipgloss.Color // Vermelho para erros
	Background    lipgloss.Color // Fundo principal
	Surface       lipgloss.Color // Fundo de cards/blocos
	Border        lipgloss.Color // Bordas sutis
	BorderSubtle  lipgloss.Color // Bordas muito sutis

	// Estilos base
	TextStyle        lipgloss.Style
	MutedTextStyle   lipgloss.Style
	AccentTextStyle  lipgloss.Style
	SuccessTextStyle lipgloss.Style
	WarningTextStyle lipgloss.Style
	ErrorTextStyle   lipgloss.Style

	// Componentes
	StatusBarStyle      lipgloss.Style
	MessageBlockStyle   lipgloss.Style
	UserMessageStyle    lipgloss.Style
	AIMessageStyle      lipgloss.Style
	InputContainerStyle lipgloss.Style
	SeparatorStyle      lipgloss.Style
	HelpBarStyle        lipgloss.Style
}

// NewWarpStyles cria o sistema de design Warp-like profissional
func NewWarpStyles(isDark bool) WarpStyles {
	var s WarpStyles

	if isDark {
		// Paleta Dark Mode (Warp oficial)
		s.Background = lipgloss.Color("#0D0D0D")    // Preto profundo
		s.Surface = lipgloss.Color("#161616")       // Cinza muito escuro
		s.Primary = lipgloss.Color("#E6E6E6")       // Texto principal claro
		s.Secondary = lipgloss.Color("#A8A8A8")     // Texto secundário
		s.Muted = lipgloss.Color("#6E6E6E")         // Texto muted
		s.Accent = lipgloss.Color("#00D9FF")        // Ciano elétrico
		s.Success = lipgloss.Color("#00FF94")       // Verde neon
		s.Warning = lipgloss.Color("#FFD600")       // Amarelo vibrante
		s.Error = lipgloss.Color("#FF4757")         // Vermelho forte
		s.Border = lipgloss.Color("#2A2A2A")        // Borda sutil
		s.BorderSubtle = lipgloss.Color("#1A1A1A")  // Borda quase invisível
	} else {
		// Light Mode
		s.Background = lipgloss.Color("#FFFFFF")
		s.Surface = lipgloss.Color("#F8F8F8")
		s.Primary = lipgloss.Color("#1A1A1A")
		s.Secondary = lipgloss.Color("#5A5A5A")
		s.Muted = lipgloss.Color("#8A8A8A")
		s.Accent = lipgloss.Color("#0080FF")
		s.Success = lipgloss.Color("#00A86B")
		s.Warning = lipgloss.Color("#FFA500")
		s.Error = lipgloss.Color("#DC143C")
		s.Border = lipgloss.Color("#E0E0E0")
		s.BorderSubtle = lipgloss.Color("#F0F0F0")
	}

	// Estilos base de texto
	s.TextStyle = lipgloss.NewStyle().
		Foreground(s.Primary)

	s.MutedTextStyle = lipgloss.NewStyle().
		Foreground(s.Muted)

	s.AccentTextStyle = lipgloss.NewStyle().
		Foreground(s.Accent).
		Bold(true)

	s.SuccessTextStyle = lipgloss.NewStyle().
		Foreground(s.Success)

	s.WarningTextStyle = lipgloss.NewStyle().
		Foreground(s.Warning)

	s.ErrorTextStyle = lipgloss.NewStyle().
		Foreground(s.Error)

	// Status bar minimalista (topo)
	s.StatusBarStyle = lipgloss.NewStyle().
		Foreground(s.Secondary).
		Padding(0, 2)

	// Bloco de mensagem genérico (sem bordas)
	s.MessageBlockStyle = lipgloss.NewStyle().
		Padding(1, 2).
		MarginBottom(1)

	// Mensagem do usuário (destaque sutil)
	s.UserMessageStyle = lipgloss.NewStyle().
		Foreground(s.Primary).
		Padding(1, 2).
		MarginBottom(1).
		BorderLeft(true).
		BorderStyle(lipgloss.ThickBorder()).
		BorderForeground(s.Accent)

	// Mensagem da AI (neutra)
	s.AIMessageStyle = lipgloss.NewStyle().
		Foreground(s.Primary).
		Padding(1, 2).
		MarginBottom(1).
		BorderLeft(true).
		BorderStyle(lipgloss.NormalBorder()).
		BorderForeground(s.Border)

	// Container do input (sem borda pesada)
	s.InputContainerStyle = lipgloss.NewStyle().
		Padding(1, 2).
		MarginTop(1).
		BorderTop(true).
		BorderStyle(lipgloss.NormalBorder()).
		BorderForeground(s.Border)

	// Separador horizontal sutil
	s.SeparatorStyle = lipgloss.NewStyle().
		Foreground(s.BorderSubtle).
		MarginTop(1).
		MarginBottom(1)

	// Barra de ajuda (rodapé)
	s.HelpBarStyle = lipgloss.NewStyle().
		Foreground(s.Muted).
		Padding(1, 2).
		BorderTop(true).
		BorderStyle(lipgloss.NormalBorder()).
		BorderForeground(s.BorderSubtle)

	return s
}

// RenderStatusBar renderiza barra de status minimalista do Warp
func (s *WarpStyles) RenderStatusBar(width int, items ...string) string {
	var parts []string

	for i, item := range items {
		if i > 0 {
			parts = append(parts, s.MutedTextStyle.Render(" │ "))
		}
		parts = append(parts, s.StatusBarStyle.Render(item))
	}

	content := lipgloss.JoinHorizontal(lipgloss.Left, parts...)

	return lipgloss.NewStyle().
		Width(width).
		Padding(1, 2).
		Background(s.Surface).
		Render(content)
}

// RenderUserMessage renderiza mensagem do usuário (com borda esquerda)
func (s *WarpStyles) RenderUserMessage(content string, width int) string {
	header := s.AccentTextStyle.Render("Você")

	body := s.UserMessageStyle.
		Width(width - 4).
		Render(header + "\n" + content)

	return body
}

// RenderAIMessage renderiza mensagem da AI (limpa, profissional)
func (s *WarpStyles) RenderAIMessage(modelName, content string, width int) string {
	header := s.TextStyle.
		Bold(true).
		Render(modelName)

	body := s.AIMessageStyle.
		Width(width - 4).
		Render(header + "\n\n" + content)

	return body
}

// RenderInputContainer renderiza container do input (minimalista)
func (s *WarpStyles) RenderInputContainer(content string, width int) string {
	return s.InputContainerStyle.
		Width(width - 4).
		Render(content)
}

// RenderHelpBar renderiza barra de ajuda (rodapé)
func (s *WarpStyles) RenderHelpBar(width int, items ...string) string {
	var parts []string

	for i, item := range items {
		if i > 0 {
			parts = append(parts, s.MutedTextStyle.Render("  "))
		}
		parts = append(parts, s.MutedTextStyle.Render(item))
	}

	content := lipgloss.JoinHorizontal(lipgloss.Left, parts...)

	return s.HelpBarStyle.
		Width(width - 4).
		Render(content)
}

// RenderSeparator renderiza separador horizontal sutil
func (s *WarpStyles) RenderSeparator(width int) string {
	line := lipgloss.NewStyle().
		Foreground(s.BorderSubtle).
		Render(lipgloss.PlaceHorizontal(width-4, lipgloss.Left, ""))

	return s.SeparatorStyle.Render(line)
}

// RenderLoadingIndicator renderiza indicador de carregamento inline
func (s *WarpStyles) RenderLoadingIndicator(spinner, text string) string {
	return s.AccentTextStyle.
		Padding(1, 2).
		Render(spinner + " " + text)
}

// RenderTaskCard renderiza card de tarefas
func (s *WarpStyles) RenderTaskCard(content string, width int) string {
	return lipgloss.NewStyle().
		Width(width - 4).
		Padding(2, 3).
		Background(s.Surface).
		BorderStyle(lipgloss.RoundedBorder()).
		BorderForeground(s.Border).
		Render(content)
}
