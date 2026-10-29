package enhanced

import (
	"fmt"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// Model representa o estado da interface TUI aprimorada
type Model struct {
	token       string
	cwd         string
	width       int
	height      int
	activeTab   string
	tabs        []string
	statusMsg   string
	showError   bool
}

// NewModel cria uma nova instância do modelo aprimorado
func NewModel(token, cwd string) Model {
	return Model{
		token:     token,
		cwd:       cwd,
		tabs:      []string{"Documentos", "Agentes", "Tarefas", "Configurações"},
		activeTab: "Documentos",
		statusMsg: "Bem-vindo ao Amazzo Terminal Agent",
		showError: false,
	}
}

func (m Model) Init() tea.Cmd {
	return nil
}

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "ctrl+c", "q":
			return m, tea.Quit
		case "tab":
			// Navegar entre abas
			for i, tab := range m.tabs {
				if tab == m.activeTab {
					m.activeTab = m.tabs[(i+1)%len(m.tabs)]
					break
				}
			}
		case "left":
			// Navegar para a aba anterior
			for i, tab := range m.tabs {
				if tab == m.activeTab {
					m.activeTab = m.tabs[(i-1+len(m.tabs))%len(m.tabs)]
					break
				}
			}
		case "right":
			// Navegar para a próxima aba
			for i, tab := range m.tabs {
				if tab == m.activeTab {
					m.activeTab = m.tabs[(i+1)%len(m.tabs)]
					break
				}
			}
		}
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
	}
	return m, nil
}

func (m Model) View() string {
	if m.width == 0 || m.height == 0 {
		return "Inicializando..."
	}

	// Construir a interface
	header := m.renderHeader()
	tabs := m.renderTabs()
	content := m.renderContent()
	statusBar := m.renderStatusBar()

	// Combinar todos os elementos
	view := lipgloss.JoinVertical(lipgloss.Left,
		header,
		tabs,
		content,
		statusBar,
	)

	return view
}

func (m Model) renderHeader() string {
	headerStyle := lipgloss.NewStyle().
		Bold(true).
		Foreground(lipgloss.Color("#6A5ACD")).
		BorderBottom(true).
		BorderStyle(lipgloss.NormalBorder()).
		BorderForeground(lipgloss.Color("#4A4A4A")).
		Align(lipgloss.Center).
		Width(m.width)

	return headerStyle.Render("AMAZZO v1.0.0 - Agente de Terminal Inteligente")
}

func (m Model) renderTabs() string {
	tabContainer := lipgloss.NewStyle().
		Padding(1, 0).
		Align(lipgloss.Center).
		Width(m.width)

	var tabs []string
	for _, tab := range m.tabs {
		var style lipgloss.Style
		if tab == m.activeTab {
			style = lipgloss.NewStyle().
				Background(lipgloss.Color("#FF6B6B")).
				Foreground(lipgloss.Color("#F0F0F0")).
				Padding(0, 2).
				Margin(0, 1)
		} else {
			style = lipgloss.NewStyle().
				Background(lipgloss.Color("#6A5ACD")).
				Foreground(lipgloss.Color("#F0F0F0")).
				Padding(0, 2).
				Margin(0, 1)
		}
		tabs = append(tabs, style.Render(tab))
	}

	return tabContainer.Render(lipgloss.JoinHorizontal(lipgloss.Top, tabs...))
}

func (m Model) renderContent() string {
	contentStyle := lipgloss.NewStyle().
		Padding(2).
		Border(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color("#4A4A4A")).
		Width(m.width - 4).
		Height(m.height - 12)

	var content string
	switch m.activeTab {
	case "Documentos":
		content = m.renderDocumentsView()
	case "Agentes":
		content = "Gerenciamento de agentes inteligentes\n\n" +
			"Os agentes são responsáveis por processar documentos e executar tarefas automatizadas."
	case "Tarefas":
		content = "Visualização de tarefas em execução\n\n" +
			"Monitoramento de processos e histórico de execução."
	case "Configurações":
		content = "Configurações do sistema\n\n" +
			"Personalização de preferências e comportamentos."
	default:
		content = fmt.Sprintf("Conteúdo para %s\n\nToken: %s...\nDiretório: %s", m.activeTab, m.token[:10], m.cwd)
	}

	return contentStyle.Render(content)
}

func (m Model) renderDocumentsView() string {
	return "Documentos Recentes\n\n" +
		"┌─────────────────────┬────────┬──────────┬────────────┐\n" +
		"│ Nome                │ Tipo   │ Tamanho  │ Data       │\n" +
		"├─────────────────────┼────────┼──────────┼────────────┤\n" +
		"│ relatorio-q1.pdf    │ PDF    │ 2.4 MB   │ 2023-04-15 │\n" +
		"│ contrato.docx       │ DOCX   │ 0.8 MB   │ 2023-04-10 │\n" +
		"│ apresentacao.pptx   │ PPTX   │ 5.2 MB   │ 2023-04-05 │\n" +
		"│ dados-analise.xlsx  │ XLSX   │ 1.1 MB   │ 2023-04-01 │\n" +
		"└─────────────────────┴────────┴──────────┴────────────┘\n\n" +
		"Comandos disponíveis:\n" +
		"• /open <nome> - Abrir documento\n" +
		"• /search <termo> - Pesquisar documentos\n" +
		"• /upload - Enviar novo documento"
}

func (m Model) renderStatusBar() string {
	statusStyle := lipgloss.NewStyle().
		Background(lipgloss.Color("#4A4A4A")).
		Foreground(lipgloss.Color("#F0F0F0")).
		Padding(0, 1).
		Width(m.width)

	return statusStyle.Render(m.statusMsg)
}