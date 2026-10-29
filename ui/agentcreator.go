package ui

import (
	"fmt"
	"strings"

	"amazzo/agents"
	"amazzo/themes"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
)

// AgentCreatorState representa o estado atual da criação
type AgentCreatorState int

const (
	StateInactive AgentCreatorState = iota
	StateName
	StateDescription
	StateLanguages
	StateFrameworks
	StateDocURLs
	StateExpertise
	StateConfirm
	StateDone
)

// AgentCreator é o modelo para criar agentes interativamente
type AgentCreator struct {
	state        AgentCreatorState
	textInput    textinput.Model
	config       *agents.AgentConfig
	docURLs      []string
	theme        themes.ThemeConfig
	onComplete   func(*agents.AgentConfig)
	onCancel     func()
}

// NewAgentCreator cria um novo criador de agentes
func NewAgentCreator(theme themes.ThemeConfig) *AgentCreator {
	ti := textinput.New()
	ti.Focus()
	ti.CharLimit = 200
	ti.Width = 50

	return &AgentCreator{
		state:     StateInactive,
		textInput: ti,
		config:    &agents.AgentConfig{},
		docURLs:   make([]string, 0),
		theme:     theme,
	}
}

// Start inicia o processo de criação
func (ac *AgentCreator) Start(onComplete func(*agents.AgentConfig), onCancel func()) {
	ac.state = StateName
	ac.config = &agents.AgentConfig{}
	ac.docURLs = make([]string, 0)
	ac.textInput.SetValue("")
	ac.textInput.Placeholder = "Ex: Bubble Plugin Expert"
	ac.onComplete = onComplete
	ac.onCancel = onCancel
}

// Update atualiza o estado
func (ac *AgentCreator) Update(msg tea.Msg) tea.Cmd {
	if ac.state == StateInactive {
		return nil
	}

	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "esc":
			// Cancela criação
			ac.state = StateInactive
			if ac.onCancel != nil {
				ac.onCancel()
			}
			return nil

		case "enter":
			return ac.handleEnter()
		}
	}

	// Atualiza textinput
	var cmd tea.Cmd
	ac.textInput, cmd = ac.textInput.Update(msg)
	return cmd
}

// handleEnter processa Enter em cada estado
func (ac *AgentCreator) handleEnter() tea.Cmd {
	value := strings.TrimSpace(ac.textInput.Value())

	switch ac.state {
	case StateName:
		if value == "" {
			return nil
		}
		ac.config.Name = value
		ac.state = StateDescription
		ac.textInput.SetValue("")
		ac.textInput.Placeholder = "Ex: Especialista em criar plugins para Bubble.io"

	case StateDescription:
		if value == "" {
			return nil
		}
		ac.config.Description = value
		ac.state = StateLanguages
		ac.textInput.SetValue("")
		ac.textInput.Placeholder = "Ex: JavaScript, HTML, CSS (separados por vírgula)"

	case StateLanguages:
		if value == "" {
			return nil
		}
		ac.config.Languages = ac.parseList(value)
		ac.state = StateFrameworks
		ac.textInput.SetValue("")
		ac.textInput.Placeholder = "Ex: Vue.js, React (ou Enter para pular)"

	case StateFrameworks:
		ac.config.Frameworks = ac.parseList(value)
		ac.state = StateDocURLs
		ac.textInput.SetValue("")
		ac.textInput.Placeholder = "URL de documentação (ou Enter para pular)"

	case StateDocURLs:
		if value != "" {
			ac.docURLs = append(ac.docURLs, value)
			ac.textInput.SetValue("")
			ac.textInput.Placeholder = "Outra URL (ou Enter para continuar)"
		} else {
			ac.config.DocURLs = ac.docURLs
			ac.state = StateExpertise
			ac.textInput.SetValue("")
			ac.textInput.Placeholder = "Ex: UI/UX, APIs REST, autenticação"
		}

	case StateExpertise:
		if value == "" {
			return nil
		}
		ac.config.Expertise = value
		ac.generateSystemPrompt()
		ac.state = StateConfirm
		ac.textInput.Blur()

	case StateConfirm:
		if value == "s" || value == "S" {
			ac.state = StateDone
			if ac.onComplete != nil {
				ac.onComplete(ac.config)
			}
			ac.state = StateInactive
		} else {
			ac.state = StateInactive
			if ac.onCancel != nil {
				ac.onCancel()
			}
		}
	}

	return nil
}

// parseList parseia lista separada por vírgulas
func (ac *AgentCreator) parseList(input string) []string {
	if input == "" {
		return []string{}
	}

	parts := strings.Split(input, ",")
	result := make([]string, 0)

	for _, part := range parts {
		trimmed := strings.TrimSpace(part)
		if trimmed != "" {
			result = append(result, trimmed)
		}
	}

	return result
}

// generateSystemPrompt gera o prompt do sistema
func (ac *AgentCreator) generateSystemPrompt() {
	var builder strings.Builder

	builder.WriteString(fmt.Sprintf("Você é %s, um agente especializado.\n\n", ac.config.Name))
	builder.WriteString(fmt.Sprintf("FUNÇÃO: %s\n\n", ac.config.Description))

	if len(ac.config.Languages) > 0 {
		builder.WriteString(fmt.Sprintf("LINGUAGENS: %s\n", strings.Join(ac.config.Languages, ", ")))
	}

	if len(ac.config.Frameworks) > 0 {
		builder.WriteString(fmt.Sprintf("FRAMEWORKS: %s\n", strings.Join(ac.config.Frameworks, ", ")))
	}

	if ac.config.Expertise != "" {
		builder.WriteString(fmt.Sprintf("EXPERTISE: %s\n", ac.config.Expertise))
	}

	builder.WriteString("\nREGRAS:\n")
	builder.WriteString("1. SEMPRE implemente código funcional e completo\n")
	builder.WriteString("2. Use as melhores práticas das tecnologias listadas\n")
	builder.WriteString("3. NÃO crie documentação a menos que solicitado\n")
	builder.WriteString("4. FOQUE em código prático e direto\n")
	builder.WriteString("5. Responda em Português do Brasil\n")

	if len(ac.config.DocURLs) > 0 {
		builder.WriteString("\nDOCUMENTAÇÃO DE REFERÊNCIA:\n")
		for _, url := range ac.config.DocURLs {
			builder.WriteString(fmt.Sprintf("- %s\n", url))
		}
		builder.WriteString("\nConsulte essas documentações quando necessário.\n")
	}

	ac.config.SystemPrompt = builder.String()
}

// View renderiza a interface
func (ac *AgentCreator) View() string {
	if ac.state == StateInactive {
		return ""
	}

	var builder strings.Builder

	builder.WriteString(ac.theme.LogoStyle.Render("┌─ 🤖 Criar Agente Especializado\n"))
	builder.WriteString("│\n")

	switch ac.state {
	case StateName:
		builder.WriteString("│  Nome do agente:\n")
		builder.WriteString("│  " + ac.textInput.View() + "\n")

	case StateDescription:
		builder.WriteString(ac.theme.InfoStyle.Render(fmt.Sprintf("│  ✓ Nome: %s\n", ac.config.Name)))
		builder.WriteString("│\n")
		builder.WriteString("│  Descrição (o que ele faz):\n")
		builder.WriteString("│  " + ac.textInput.View() + "\n")

	case StateLanguages:
		builder.WriteString(ac.theme.InfoStyle.Render(fmt.Sprintf("│  ✓ Nome: %s\n", ac.config.Name)))
		builder.WriteString(ac.theme.InfoStyle.Render(fmt.Sprintf("│  ✓ Descrição: %s\n", ac.config.Description)))
		builder.WriteString("│\n")
		builder.WriteString("│  Linguagens (separadas por vírgula):\n")
		builder.WriteString("│  " + ac.textInput.View() + "\n")

	case StateFrameworks:
		builder.WriteString(ac.theme.InfoStyle.Render(fmt.Sprintf("│  ✓ Linguagens: %s\n", strings.Join(ac.config.Languages, ", "))))
		builder.WriteString("│\n")
		builder.WriteString("│  Frameworks/Libs (ou Enter para pular):\n")
		builder.WriteString("│  " + ac.textInput.View() + "\n")

	case StateDocURLs:
		if len(ac.docURLs) > 0 {
			builder.WriteString(ac.theme.InfoStyle.Render(fmt.Sprintf("│  ✓ URLs adicionadas: %d\n", len(ac.docURLs))))
		}
		builder.WriteString("│\n")
		builder.WriteString("│  URL de documentação (Enter para continuar):\n")
		builder.WriteString("│  " + ac.textInput.View() + "\n")

	case StateExpertise:
		builder.WriteString("│  Área de expertise:\n")
		builder.WriteString("│  " + ac.textInput.View() + "\n")

	case StateConfirm:
		builder.WriteString(ac.theme.InfoStyle.Render("│  ✓ Agente configurado!\n"))
		builder.WriteString("│\n")
		builder.WriteString(fmt.Sprintf("│  Nome: %s\n", ac.config.Name))
		builder.WriteString(fmt.Sprintf("│  Linguagens: %s\n", strings.Join(ac.config.Languages, ", ")))
		if len(ac.config.Frameworks) > 0 {
			builder.WriteString(fmt.Sprintf("│  Frameworks: %s\n", strings.Join(ac.config.Frameworks, ", ")))
		}
		if len(ac.config.DocURLs) > 0 {
			builder.WriteString(fmt.Sprintf("│  URLs: %d documentações\n", len(ac.config.DocURLs)))
		}
		builder.WriteString("│\n")
		builder.WriteString("│  Confirmar criação? (s/N): ")
	}

	builder.WriteString("│\n")
	builder.WriteString(ac.theme.HelpStyle.Render("│  ESC: Cancelar\n"))
	builder.WriteString("└─\n")

	return builder.String()
}

// IsActive verifica se está ativo
func (ac *AgentCreator) IsActive() bool {
	return ac.state != StateInactive
}
