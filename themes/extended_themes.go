package themes

import "github.com/charmbracelet/lipgloss"

// ExtendedThemes contém 10 temas adicionais cuidadosamente selecionados
var ExtendedThemes = map[string]ThemeConfig{
	// === TEMAS ESCUROS ===
	"dracula": {
		Name:       "Dracula",
		UserStyle:  lipgloss.NewStyle().Foreground(lipgloss.Color("#ff79c6")).Bold(true),
		AIStyle:    lipgloss.NewStyle().Foreground(lipgloss.Color("#bd93f9")).Bold(true),
		LogoStyle:  lipgloss.NewStyle().Foreground(lipgloss.Color("#ff79c6")).Bold(true),
		InfoStyle:  lipgloss.NewStyle().Foreground(lipgloss.Color("#8be9fd")),
		HelpStyle:  lipgloss.NewStyle().Foreground(lipgloss.Color("#6272a4")),
		ErrorStyle: lipgloss.NewStyle().Foreground(lipgloss.Color("#ff5555")).Bold(true),
	},

	"monokai": {
		Name:       "Monokai",
		UserStyle:  lipgloss.NewStyle().Foreground(lipgloss.Color("#f92672")).Bold(true),
		AIStyle:    lipgloss.NewStyle().Foreground(lipgloss.Color("#a6e22e")).Bold(true),
		LogoStyle:  lipgloss.NewStyle().Foreground(lipgloss.Color("#f92672")).Bold(true),
		InfoStyle:  lipgloss.NewStyle().Foreground(lipgloss.Color("#66d9ef")),
		HelpStyle:  lipgloss.NewStyle().Foreground(lipgloss.Color("#75715e")),
		ErrorStyle: lipgloss.NewStyle().Foreground(lipgloss.Color("#f92672")).Bold(true),
	},

	"nord": {
		Name:       "Nord",
		UserStyle:  lipgloss.NewStyle().Foreground(lipgloss.Color("#88c0d0")).Bold(true),
		AIStyle:    lipgloss.NewStyle().Foreground(lipgloss.Color("#a3be8c")).Bold(true),
		LogoStyle:  lipgloss.NewStyle().Foreground(lipgloss.Color("#88c0d0")).Bold(true),
		InfoStyle:  lipgloss.NewStyle().Foreground(lipgloss.Color("#81a1c1")),
		HelpStyle:  lipgloss.NewStyle().Foreground(lipgloss.Color("#616e88")),
		ErrorStyle: lipgloss.NewStyle().Foreground(lipgloss.Color("#bf616a")).Bold(true),
	},

	"gruvbox": {
		Name:       "Gruvbox Dark",
		UserStyle:  lipgloss.NewStyle().Foreground(lipgloss.Color("#fe8019")).Bold(true),
		AIStyle:    lipgloss.NewStyle().Foreground(lipgloss.Color("#b8bb26")).Bold(true),
		LogoStyle:  lipgloss.NewStyle().Foreground(lipgloss.Color("#fe8019")).Bold(true),
		InfoStyle:  lipgloss.NewStyle().Foreground(lipgloss.Color("#83a598")),
		HelpStyle:  lipgloss.NewStyle().Foreground(lipgloss.Color("#928374")),
		ErrorStyle: lipgloss.NewStyle().Foreground(lipgloss.Color("#fb4934")).Bold(true),
	},

	"tokyo-night": {
		Name:       "Tokyo Night",
		UserStyle:  lipgloss.NewStyle().Foreground(lipgloss.Color("#7aa2f7")).Bold(true),
		AIStyle:    lipgloss.NewStyle().Foreground(lipgloss.Color("#bb9af7")).Bold(true),
		LogoStyle:  lipgloss.NewStyle().Foreground(lipgloss.Color("#7aa2f7")).Bold(true),
		InfoStyle:  lipgloss.NewStyle().Foreground(lipgloss.Color("#9ece6a")),
		HelpStyle:  lipgloss.NewStyle().Foreground(lipgloss.Color("#565f89")),
		ErrorStyle: lipgloss.NewStyle().Foreground(lipgloss.Color("#f7768e")).Bold(true),
	},

	"catppuccin": {
		Name:       "Catppuccin Mocha",
		UserStyle:  lipgloss.NewStyle().Foreground(lipgloss.Color("#cba6f7")).Bold(true),
		AIStyle:    lipgloss.NewStyle().Foreground(lipgloss.Color("#f5c2e7")).Bold(true),
		LogoStyle:  lipgloss.NewStyle().Foreground(lipgloss.Color("#cba6f7")).Bold(true),
		InfoStyle:  lipgloss.NewStyle().Foreground(lipgloss.Color("#89b4fa")),
		HelpStyle:  lipgloss.NewStyle().Foreground(lipgloss.Color("#6c7086")),
		ErrorStyle: lipgloss.NewStyle().Foreground(lipgloss.Color("#f38ba8")).Bold(true),
	},

	"cyberpunk": {
		Name:       "Cyberpunk",
		UserStyle:  lipgloss.NewStyle().Foreground(lipgloss.Color("#ff00ff")).Bold(true),
		AIStyle:    lipgloss.NewStyle().Foreground(lipgloss.Color("#00ffff")).Bold(true),
		LogoStyle:  lipgloss.NewStyle().Foreground(lipgloss.Color("#ff00ff")).Bold(true),
		InfoStyle:  lipgloss.NewStyle().Foreground(lipgloss.Color("#00ffff")),
		HelpStyle:  lipgloss.NewStyle().Foreground(lipgloss.Color("#ff0080")),
		ErrorStyle: lipgloss.NewStyle().Foreground(lipgloss.Color("#ff0000")).Bold(true),
	},

	"matrix": {
		Name:       "Matrix",
		UserStyle:  lipgloss.NewStyle().Foreground(lipgloss.Color("#00ff00")).Bold(true),
		AIStyle:    lipgloss.NewStyle().Foreground(lipgloss.Color("#00dd00")).Bold(true),
		LogoStyle:  lipgloss.NewStyle().Foreground(lipgloss.Color("#00ff00")).Bold(true),
		InfoStyle:  lipgloss.NewStyle().Foreground(lipgloss.Color("#00cc00")),
		HelpStyle:  lipgloss.NewStyle().Foreground(lipgloss.Color("#008800")),
		ErrorStyle: lipgloss.NewStyle().Foreground(lipgloss.Color("#ff0000")).Bold(true),
	},

	// === TEMAS CLAROS ===
	"solarized-light": {
		Name:       "Solarized Light",
		UserStyle:  lipgloss.NewStyle().Foreground(lipgloss.Color("#268bd2")).Bold(true),
		AIStyle:    lipgloss.NewStyle().Foreground(lipgloss.Color("#2aa198")).Bold(true),
		LogoStyle:  lipgloss.NewStyle().Foreground(lipgloss.Color("#268bd2")).Bold(true),
		InfoStyle:  lipgloss.NewStyle().Foreground(lipgloss.Color("#859900")),
		HelpStyle:  lipgloss.NewStyle().Foreground(lipgloss.Color("#93a1a1")),
		ErrorStyle: lipgloss.NewStyle().Foreground(lipgloss.Color("#dc322f")).Bold(true),
	},

	"github-light": {
		Name:       "GitHub Light",
		UserStyle:  lipgloss.NewStyle().Foreground(lipgloss.Color("#0969da")).Bold(true),
		AIStyle:    lipgloss.NewStyle().Foreground(lipgloss.Color("#8250df")).Bold(true),
		LogoStyle:  lipgloss.NewStyle().Foreground(lipgloss.Color("#0969da")).Bold(true),
		InfoStyle:  lipgloss.NewStyle().Foreground(lipgloss.Color("#1a7f37")),
		HelpStyle:  lipgloss.NewStyle().Foreground(lipgloss.Color("#57606a")),
		ErrorStyle: lipgloss.NewStyle().Foreground(lipgloss.Color("#cf222e")).Bold(true),
	},
}

// MergeExtendedThemes mescla temas estendidos com temas padrão
func init() {
	for key, theme := range ExtendedThemes {
		Available[key] = theme
	}
}
