package ui

import (
	"github.com/charmbracelet/lipgloss"
)

// Paleta de cores moderna inspirada no Warp terminal
const (
	PrimaryColor    = "#6A5ACD" // Slate Blue
	SecondaryColor  = "#87CEEB" // Sky Blue
	AccentColor     = "#FF6B6B" // Coral Red
	TextColor       = "#F8F8F2" // Light Text
	BorderColor     = "#4A4A4A" // Dark Gray Border
	BackgroundColor = "#1E1F29" // Dark Background
	SuccessColor    = "#50FA7B" // Green
	ErrorColor      = "#FF5555" // Red
	WarningColor    = "#F1FA8C" // Yellow
)

// Estilos modernos para a interface
var (
	TitleStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color(PrimaryColor)).
			Background(lipgloss.Color(BackgroundColor)).
			Bold(true).
			Padding(0, 1).
			MarginBottom(1)

	SubtitleStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color(SecondaryColor)).
			Bold(true).
			MarginBottom(1)

	ContentStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color(TextColor)).
			Background(lipgloss.Color("#282A36")).
			Padding(1).
			Border(lipgloss.RoundedBorder()).
			BorderForeground(lipgloss.Color(BorderColor))

	ActiveTabStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color(TextColor)).
			Background(lipgloss.Color(PrimaryColor)).
			Padding(0, 2)

	InactiveTabStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#AAAAAA")).
			Background(lipgloss.Color("#3E4057")).
			Padding(0, 2)

	ButtonStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color(TextColor)).
			Background(lipgloss.Color(PrimaryColor)).
			Padding(0, 2).
			MarginRight(1).
			MarginBottom(1).
			BorderStyle(lipgloss.RoundedBorder()).
			BorderForeground(lipgloss.Color(SecondaryColor))

	ActiveButtonStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color(TextColor)).
			Background(lipgloss.Color(AccentColor)).
			Padding(0, 2).
			MarginRight(1).
			MarginBottom(1).
			BorderStyle(lipgloss.ThickBorder()).
			BorderForeground(lipgloss.Color(SecondaryColor)).
			Bold(true)

	ErrorStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color(ErrorColor)).
			Bold(true)

	SuccessStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color(SuccessColor)).
			Bold(true)

	InfoStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color(SecondaryColor))

	HelpStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#6272A4")). // Subtle gray
			Italic(true)

	LogoStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color(AccentColor)).
			Bold(true).
			MarginRight(1)

	StatusBarStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color(TextColor)).
			Background(lipgloss.Color("#282A36")).
			Padding(0, 1).
			Width(80)

	CardStyle = lipgloss.NewStyle().
			Background(lipgloss.Color("#282A36")).
			Border(lipgloss.RoundedBorder()).
			BorderForeground(lipgloss.Color(PrimaryColor)).
			Padding(1).
			Margin(1)

	HighlightStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color(AccentColor)).
			Bold(true)

	ProgressStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color(SecondaryColor)).
			Background(lipgloss.Color("#21222C"))
)