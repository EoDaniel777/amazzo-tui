package ui

import (
	"fmt"

	"github.com/charmbracelet/bubbles/textarea"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// ModernModel é o modelo principal da interface moderna
type ModernModel struct {
	width         int
	height        int
	tabs          *ModernTabs
	header        *ModernHeader
	statusBar     *ModernStatusBar
	activeContent string
	content       map[string]string
	chat          *ChatComponent  // Componente de chat para interação com IA
	helpVisible   bool
	focused       bool
}

// NewModernModel cria um novo modelo moderno
func NewModernModel() *ModernModel {
	tabs := NewModernTabs([]string{"Chat", "Documentos", "Agentes", "Tarefas", "Configurações"})

	header := NewModernHeader("AMAZZO", "Terminal Agent v1.0.0")

	statusBar := NewModernStatusBar()
	statusBar.SetLeftText("Amazzo Terminal")
	statusBar.SetRightText("Pronto")

	// Conteúdos para cada aba (exceto Chat que tem seu próprio componente)
	content := map[string]string{
		"Documentos": "Visualização de documentos em tempo real\n\n- relatorio-q1.pdf\n- contrato-cliente.docx\n- apresentacao.pptx",
		"Agentes":    "Gerenciamento de agentes especializados\n\nCrie e configure agentes para tarefas específicas",
		"Tarefas":    "Monitoramento de tarefas em execução\n\n- Processando documentos: 2/5\n- Indexando arquivos: 1/3",
		"Configurações": "Personalização do sistema\n\n- Temas\n- Atalhos de teclado\n- Preferências de interface",
	}

	// Inicializar o componente de chat
	chat := NewChatComponent()

	return &ModernModel{
		tabs:          tabs,
		header:        header,
		statusBar:     statusBar,
		content:       content,
		activeContent: "Chat", // Começa na aba de chat
		chat:          chat,
		helpVisible:   false,
		focused:       true,
	}
}

func (m *ModernModel) Init() tea.Cmd {
	return tea.Batch(
		textarea.Blink,
	)
}

func (m *ModernModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmds []tea.Cmd

	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "ctrl+c", "q":
			return m, tea.Quit
		case "tab":
			m.tabs.Next()
			m.activeContent = m.tabs.GetActiveTab()
		case "shift+tab":
			m.tabs.Prev()
			m.activeContent = m.tabs.GetActiveTab()
		case "?", "h":
			m.helpVisible = !m.helpVisible
		case "left":
			m.tabs.Prev()
			m.activeContent = m.tabs.GetActiveTab()
		case "right":
			m.tabs.Next()
			m.activeContent = m.tabs.GetActiveTab()
		case "enter":
			// Na aba de chat, processar a mensagem
			if m.activeContent == "Chat" {
				input := m.chat.GetValue()
				if input != "" {
					// Adiciona mensagem do usuário
					m.chat.AddMessage("user", input)
					// Limpa o input
					m.chat.ClearInput()

					// Aqui você pode adicionar a lógica para enviar para o modelo de IA
					// Por enquanto, vamos simular uma resposta
					// Em implementação completa, isso chamaria o modelo de IA
					m.chat.AddMessage("assistant", "Olá! Esta é uma interface de chat moderna inspirada no Warp terminal. A funcionalidade completa de IA será integrada aqui.")
				}
			}
		}

	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		m.header.SetSize(msg.Width)

		// Atualizar tamanho do chat
		if m.chat != nil {
			m.chat.SetSize(msg.Width, msg.Height-10) // Reservar espaço para outros elementos
		}

	default:
		// Passar mensagens para o componente de chat se estiver na aba correta
		if m.activeContent == "Chat" && m.chat != nil {
			cmd := m.chat.Update(msg)
			if cmd != nil {
				cmds = append(cmds, cmd)
			}
		}
	}

	return m, tea.Batch(cmds...)
}

func (m *ModernModel) View() string {
	if m.width == 0 || m.height == 0 {
		return "Inicializando..."
	}

	var elements []string

	// Renderizar componentes
	header := m.header.RenderWithSubtitle()
	tabs := m.tabs.Render(m.width)
	statusBar := m.statusBar.RenderSimple()

	// Conteúdo baseado na aba ativa
	var content string
	if m.activeContent == "Chat" && m.chat != nil {
		// Na aba de chat, usar o componente de chat
		m.chat.SetSize(m.width, m.height-10) // Ajustar tamanho
		content = m.chat.View()
	} else {
		// Em outras abas, usar conteúdo estático
		contentValue, exists := m.content[m.activeContent]
		if !exists {
			contentValue = fmt.Sprintf("Conteúdo para %s", m.activeContent)
		}

		// Estilizar o conteúdo
		content = ContentStyle.
			Width(m.width-4).
			Height(m.height-12). // Ajustar altura com base nos outros elementos
			Render(contentValue)
	}

	// Adicionar ajuda se visível
	var help string
	if m.helpVisible {
		help = m.renderHelp()
	}

	// Juntar todos os elementos
	elements = []string{header, tabs, content, statusBar}
	if help != "" {
		elements = append(elements, help)
	}

	return lipgloss.JoinVertical(lipgloss.Left, elements...)
}

func (m *ModernModel) renderHelp() string {
	helpText := `
CONTROLES:
  ← →    Mover entre abas
  Tab    Próxima aba
  Shift+Tab  Aba anterior
  Enter    Enviar mensagem (na aba Chat)
  ? ou h Mostrar/ocultar ajuda
  Ctrl+C ou Q  Sair

ATENÇÃO: Interface moderna inspirada no Warp terminal
	`

	helpStyle := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color(WarningColor)).
		Padding(1).
		Foreground(lipgloss.Color(TextColor)).
		Background(lipgloss.Color("#282A36"))

	return helpStyle.Render(helpText)
}

// InitialModel é a função que o comando TUI espera
func InitialModel() *ModernModel {
	return NewModernModel()
}