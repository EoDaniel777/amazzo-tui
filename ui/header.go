package ui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
)

const (
	AppName    = "AMAZZO"
	AppVersion = "v1.0.0"
)

// ModernHeader representa um cabeçalho moderno
type ModernHeader struct {
	title    string
	subtitle string
	width    int
	showLogo bool
}

// NewModernHeader cria um novo cabeçalho
func NewModernHeader(title, subtitle string) *ModernHeader {
	return &ModernHeader{
		title:    title,
		subtitle: subtitle,
		width:    80,
		showLogo: true,
	}
}

// SetSize define o tamanho
func (mh *ModernHeader) SetSize(width int) {
	mh.width = width
}

// SetShowLogo define se mostra o logo
func (mh *ModernHeader) SetShowLogo(show bool) {
	mh.showLogo = show
}

// Render renderiza o cabeçalho
func (mh *ModernHeader) Render() string {
	var elements []string

	// Logo e título
	if mh.showLogo {
		logo := LogoStyle.Render("🤖")
		title := TitleStyle.Render(fmt.Sprintf("%s %s", mh.title, mh.subtitle))
		headerLine := lipgloss.JoinHorizontal(lipgloss.Center, logo, title)
		elements = append(elements, headerLine)
	} else {
		title := TitleStyle.Render(fmt.Sprintf("%s %s", mh.title, mh.subtitle))
		elements = append(elements, title)
	}

	// Linha divisória
	divider := strings.Repeat("─", mh.width)
	dividerStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color(BorderColor))
	elements = append(elements, dividerStyle.Render(divider))

	return lipgloss.JoinVertical(lipgloss.Left, elements...)
}

// RenderCompact renderiza um cabeçalho compacto
func (mh *ModernHeader) RenderCompact() string {
	if mh.showLogo {
		logo := LogoStyle.Render("🤖")
		title := TitleStyle.Render(mh.title)
		return lipgloss.JoinHorizontal(lipgloss.Center, logo, title)
	}
	return TitleStyle.Render(mh.title)
}

// RenderWithSubtitle renderiza com subtítulo embaixo
func (mh *ModernHeader) RenderWithSubtitle() string {
	title := TitleStyle.Bold(true).Render(mh.title)
	subtitle := SubtitleStyle.Italic(true).Render(mh.subtitle)
	
	content := lipgloss.JoinVertical(lipgloss.Center,
		title,
		subtitle,
	)
	
	// Centralizar o conteúdo
	centered := lipgloss.NewStyle().
		Width(mh.width).
		Align(lipgloss.Center).
		Render(content)
	
	return centered
}

// RenderBanner renderiza um banner completo
func (mh *ModernHeader) RenderBanner() string {
	// Criar um banner com bordas
	bannerContent := fmt.Sprintf("%s %s", mh.title, mh.subtitle)
	
	bannerStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color(TextColor)).
		Background(lipgloss.Color(PrimaryColor)).
		Padding(1, 2).
		BorderStyle(lipgloss.DoubleBorder()).
		BorderForeground(lipgloss.Color(AccentColor))
	
	return bannerStyle.Render(bannerContent)
}

// RenderWithBorder renderiza com borda colorida
func (mh *ModernHeader) RenderWithBorder() string {
	content := lipgloss.JoinVertical(lipgloss.Center,
		TitleStyle.Render(mh.title),
		SubtitleStyle.Render(mh.subtitle),
	)
	
	borderStyle := lipgloss.NewStyle().
		Border(lipgloss.NormalBorder()).
		BorderForeground(lipgloss.Color(SecondaryColor)).
		Padding(1).
		Width(mh.width)
	
	return borderStyle.Render(content)
}