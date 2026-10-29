package themes

import (
	"sort"

	"github.com/charmbracelet/lipgloss"
)

// ThemeConfig representa um conjunto de estilos para a interface.
type ThemeConfig struct {
	Name       string
	UserStyle  lipgloss.Style
	AIStyle    lipgloss.Style
	InfoStyle  lipgloss.Style
	ErrorStyle lipgloss.Style
	HelpStyle  lipgloss.Style
	LogoStyle  lipgloss.Style
}

// Available lista os temas registrados.
var Available = map[string]ThemeConfig{
	"default": {
		Name:       "Default",
		UserStyle:  lipgloss.NewStyle().Foreground(lipgloss.Color("#00FFFF")).Bold(true),
		AIStyle:    lipgloss.NewStyle().Foreground(lipgloss.Color("#FF00FF")).Bold(true),
		InfoStyle:  lipgloss.NewStyle().Foreground(lipgloss.Color("#FFFF00")),
		ErrorStyle: lipgloss.NewStyle().Foreground(lipgloss.Color("#FF0000")).Bold(true),
		HelpStyle:  lipgloss.NewStyle().Foreground(lipgloss.Color("#00FF00")),
		LogoStyle:  lipgloss.NewStyle().Foreground(lipgloss.Color("#00FFFF")).Bold(true),
	},
	"ocean": {
		Name:       "Ocean",
		UserStyle:  lipgloss.NewStyle().Foreground(lipgloss.Color("#0088FF")).Bold(true),
		AIStyle:    lipgloss.NewStyle().Foreground(lipgloss.Color("#00DDFF")).Bold(true),
		InfoStyle:  lipgloss.NewStyle().Foreground(lipgloss.Color("#00AAFF")),
		ErrorStyle: lipgloss.NewStyle().Foreground(lipgloss.Color("#FF3333")).Bold(true),
		HelpStyle:  lipgloss.NewStyle().Foreground(lipgloss.Color("#00DDDD")),
		LogoStyle:  lipgloss.NewStyle().Foreground(lipgloss.Color("#0088FF")).Bold(true),
	},
	"forest": {
		Name:       "Forest",
		UserStyle:  lipgloss.NewStyle().Foreground(lipgloss.Color("#00FF00")).Bold(true),
		AIStyle:    lipgloss.NewStyle().Foreground(lipgloss.Color("#88FF88")).Bold(true),
		InfoStyle:  lipgloss.NewStyle().Foreground(lipgloss.Color("#AAFF00")),
		ErrorStyle: lipgloss.NewStyle().Foreground(lipgloss.Color("#FF3333")).Bold(true),
		HelpStyle:  lipgloss.NewStyle().Foreground(lipgloss.Color("#66FF66")),
		LogoStyle:  lipgloss.NewStyle().Foreground(lipgloss.Color("#00FF00")).Bold(true),
	},
	"sunset": {
		Name:       "Sunset",
		UserStyle:  lipgloss.NewStyle().Foreground(lipgloss.Color("#FF6600")).Bold(true),
		AIStyle:    lipgloss.NewStyle().Foreground(lipgloss.Color("#FFAA00")).Bold(true),
		InfoStyle:  lipgloss.NewStyle().Foreground(lipgloss.Color("#FFDD00")),
		ErrorStyle: lipgloss.NewStyle().Foreground(lipgloss.Color("#FF0000")).Bold(true),
		HelpStyle:  lipgloss.NewStyle().Foreground(lipgloss.Color("#FF88FF")),
		LogoStyle:  lipgloss.NewStyle().Foreground(lipgloss.Color("#FF6600")).Bold(true),
	},
	"monochrome": {
		Name:       "Monochrome",
		UserStyle:  lipgloss.NewStyle().Foreground(lipgloss.Color("#FFFFFF")).Bold(true),
		AIStyle:    lipgloss.NewStyle().Foreground(lipgloss.Color("#AAAAAA")).Bold(true),
		InfoStyle:  lipgloss.NewStyle().Foreground(lipgloss.Color("#CCCCCC")),
		ErrorStyle: lipgloss.NewStyle().Foreground(lipgloss.Color("#FF5555")).Bold(true),
		HelpStyle:  lipgloss.NewStyle().Foreground(lipgloss.Color("#DDDDDD")),
		LogoStyle:  lipgloss.NewStyle().Foreground(lipgloss.Color("#FFFFFF")).Bold(true),
	},
}

// DefaultName retorna o identificador do tema padrão.
func DefaultName() string {
	return "default"
}

// SortedKeys retorna os nomes dos temas em ordem alfabética.
func SortedKeys() []string {
	keys := make([]string, 0, len(Available))
	for key := range Available {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
}
