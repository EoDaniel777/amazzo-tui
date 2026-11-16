package themes

import "github.com/charmbracelet/lipgloss"

// VividThemes: temas novos no estilo GitHub Dark e Cyberpunk (fundo profundo,
// acentos saturados). As paletas completas ficam em app/uipalette_data.go.
var VividThemes = map[string]ThemeConfig{
	"midnight": {
		Name:       "Midnight",
		UserStyle:  lipgloss.NewStyle().Foreground(lipgloss.Color("#5cf2c8")).Bold(true),
		AIStyle:    lipgloss.NewStyle().Foreground(lipgloss.Color("#4da3ff")).Bold(true),
		LogoStyle:  lipgloss.NewStyle().Foreground(lipgloss.Color("#4da3ff")).Bold(true),
		InfoStyle:  lipgloss.NewStyle().Foreground(lipgloss.Color("#ffd24d")),
		HelpStyle:  lipgloss.NewStyle().Foreground(lipgloss.Color("#8d9bd1")),
		ErrorStyle: lipgloss.NewStyle().Foreground(lipgloss.Color("#ff5c7a")).Bold(true),
	},
	"aurora": {
		Name:       "Aurora",
		UserStyle:  lipgloss.NewStyle().Foreground(lipgloss.Color("#7dff9a")).Bold(true),
		AIStyle:    lipgloss.NewStyle().Foreground(lipgloss.Color("#b388ff")).Bold(true),
		LogoStyle:  lipgloss.NewStyle().Foreground(lipgloss.Color("#b388ff")).Bold(true),
		InfoStyle:  lipgloss.NewStyle().Foreground(lipgloss.Color("#ffe066")),
		HelpStyle:  lipgloss.NewStyle().Foreground(lipgloss.Color("#7fc4b5")),
		ErrorStyle: lipgloss.NewStyle().Foreground(lipgloss.Color("#ff6b81")).Bold(true),
	},
	"synthwave": {
		Name:       "Synthwave",
		UserStyle:  lipgloss.NewStyle().Foreground(lipgloss.Color("#72f1b8")).Bold(true),
		AIStyle:    lipgloss.NewStyle().Foreground(lipgloss.Color("#ff7edb")).Bold(true),
		LogoStyle:  lipgloss.NewStyle().Foreground(lipgloss.Color("#ff7edb")).Bold(true),
		InfoStyle:  lipgloss.NewStyle().Foreground(lipgloss.Color("#fede5d")),
		HelpStyle:  lipgloss.NewStyle().Foreground(lipgloss.Color("#b08ad8")),
		ErrorStyle: lipgloss.NewStyle().Foreground(lipgloss.Color("#fe4450")).Bold(true),
	},
	"ember": {
		Name:       "Ember",
		UserStyle:  lipgloss.NewStyle().Foreground(lipgloss.Color("#ffb347")).Bold(true),
		AIStyle:    lipgloss.NewStyle().Foreground(lipgloss.Color("#ff6a3d")).Bold(true),
		LogoStyle:  lipgloss.NewStyle().Foreground(lipgloss.Color("#ff6a3d")).Bold(true),
		InfoStyle:  lipgloss.NewStyle().Foreground(lipgloss.Color("#ffd24d")),
		HelpStyle:  lipgloss.NewStyle().Foreground(lipgloss.Color("#c9998a")),
		ErrorStyle: lipgloss.NewStyle().Foreground(lipgloss.Color("#ff3b3b")).Bold(true),
	},
	"ice": {
		Name:       "Ice",
		UserStyle:  lipgloss.NewStyle().Foreground(lipgloss.Color("#7af0ff")).Bold(true),
		AIStyle:    lipgloss.NewStyle().Foreground(lipgloss.Color("#8ab4ff")).Bold(true),
		LogoStyle:  lipgloss.NewStyle().Foreground(lipgloss.Color("#8ab4ff")).Bold(true),
		InfoStyle:  lipgloss.NewStyle().Foreground(lipgloss.Color("#ffe08a")),
		HelpStyle:  lipgloss.NewStyle().Foreground(lipgloss.Color("#8bc0d6")),
		ErrorStyle: lipgloss.NewStyle().Foreground(lipgloss.Color("#ff7a8a")).Bold(true),
	},
	"paper": {
		Name:       "Paper",
		UserStyle:  lipgloss.NewStyle().Foreground(lipgloss.Color("#1b6b2f")).Bold(true),
		AIStyle:    lipgloss.NewStyle().Foreground(lipgloss.Color("#1b4fb3")).Bold(true),
		LogoStyle:  lipgloss.NewStyle().Foreground(lipgloss.Color("#1b4fb3")).Bold(true),
		InfoStyle:  lipgloss.NewStyle().Foreground(lipgloss.Color("#7a5200")),
		HelpStyle:  lipgloss.NewStyle().Foreground(lipgloss.Color("#5a5144")),
		ErrorStyle: lipgloss.NewStyle().Foreground(lipgloss.Color("#b3261e")).Bold(true),
	},
	"sky": {
		Name:       "Sky",
		UserStyle:  lipgloss.NewStyle().Foreground(lipgloss.Color("#0d6b4a")).Bold(true),
		AIStyle:    lipgloss.NewStyle().Foreground(lipgloss.Color("#0b5cbf")).Bold(true),
		LogoStyle:  lipgloss.NewStyle().Foreground(lipgloss.Color("#0b5cbf")).Bold(true),
		InfoStyle:  lipgloss.NewStyle().Foreground(lipgloss.Color("#7a5600")),
		HelpStyle:  lipgloss.NewStyle().Foreground(lipgloss.Color("#46586b")),
		ErrorStyle: lipgloss.NewStyle().Foreground(lipgloss.Color("#b3202e")).Bold(true),
	},
	"mint": {
		Name:       "Mint",
		UserStyle:  lipgloss.NewStyle().Foreground(lipgloss.Color("#0f6b34")).Bold(true),
		AIStyle:    lipgloss.NewStyle().Foreground(lipgloss.Color("#0a5f8a")).Bold(true),
		LogoStyle:  lipgloss.NewStyle().Foreground(lipgloss.Color("#0a5f8a")).Bold(true),
		InfoStyle:  lipgloss.NewStyle().Foreground(lipgloss.Color("#755400")),
		HelpStyle:  lipgloss.NewStyle().Foreground(lipgloss.Color("#41594a")),
		ErrorStyle: lipgloss.NewStyle().Foreground(lipgloss.Color("#ad2230")).Bold(true),
	},
}

func init() {
	for key, theme := range VividThemes {
		Available[key] = theme
	}
}
