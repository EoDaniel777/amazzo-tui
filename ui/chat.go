package ui

import (
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/textarea"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// Message representa uma mensagem no chat
type Message struct {
	Role      string    // "user" ou "assistant"
	Content   string
	Timestamp time.Time
}

// ChatComponent representa o componente de chat
type ChatComponent struct {
	textarea  textarea.Model
	viewport  viewport.Model
	messages  []Message
	width     int
	height    int
	focused   bool
	showHelp  bool
}

// NewChatComponent cria um novo componente de chat
func NewChatComponent() *ChatComponent {
	ta := textarea.New()
	ta.Placeholder = "Digite sua mensagem aqui..."
	ta.Focus()

	ta.Prompt = "┃ "
	ta.Cursor.Style = lipgloss.NewStyle().Foreground(lipgloss.Color("212"))

	vp := viewport.New(80, 10)
	vp.Style = lipgloss.NewStyle().Padding(1, 2)

	return &ChatComponent{
		textarea: ta,
		viewport: vp,
		messages: []Message{},
		focused:  true,
		showHelp: true,
	}
}

// SetSize define o tamanho do componente
func (cc *ChatComponent) SetSize(width, height int) {
	cc.width = width
	cc.height = height
	
	// Calcular altura para viewport e textarea
	viewportHeight := height - 8 // Reservar espaço para entrada e outros elementos
	if viewportHeight < 5 {
		viewportHeight = 5
	}
	
	cc.viewport.Height = viewportHeight
	cc.viewport.Width = width - 4
	
	cc.textarea.SetWidth(width - 4)
}

// AddMessage adiciona uma mensagem ao histórico
func (cc *ChatComponent) AddMessage(role, content string) {
	message := Message{
		Role:      role,
		Content:   content,
		Timestamp: time.Now(),
	}
	cc.messages = append(cc.messages, message)
	cc.updateViewportContent()
}

// updateViewportContent atualiza o conteúdo do viewport com as mensagens
func (cc *ChatComponent) updateViewportContent() {
	var content strings.Builder
	
	for _, msg := range cc.messages {
		var roleStyle lipgloss.Style
		var roleText string
		
		if msg.Role == "user" {
			roleStyle = lipgloss.NewStyle().Foreground(lipgloss.Color(AccentColor)).Bold(true)
			roleText = "Você:"
		} else {
			roleStyle = lipgloss.NewStyle().Foreground(lipgloss.Color(SecondaryColor)).Bold(true)
			roleText = "Amazzo:"
		}
		
		content.WriteString(roleStyle.Render(roleText))
		content.WriteString("\n")
		content.WriteString(ContentStyle.Copy().Width(cc.viewport.Width-4).Render(msg.Content))
		content.WriteString("\n\n")
	}
	
	cc.viewport.SetContent(content.String())
	cc.viewport.GotoBottom()
}

// Focus coloca o foco no textarea
func (cc *ChatComponent) Focus() {
	cc.focused = true
	cc.textarea.Focus()
}

// Blur remove o foco do textarea
func (cc *ChatComponent) Blur() {
	cc.focused = false
	cc.textarea.Blur()
}

// IsFocused retorna se o componente está focado
func (cc *ChatComponent) IsFocused() bool {
	return cc.focused
}

// GetValue retorna o valor atual do textarea
func (cc *ChatComponent) GetValue() string {
	return cc.textarea.Value()
}

// SetValue define o valor do textarea
func (cc *ChatComponent) SetValue(value string) {
	cc.textarea.SetValue(value)
}

// ClearInput limpa o textarea
func (cc *ChatComponent) ClearInput() {
	cc.textarea.SetValue("")
}

// Update atualiza o componente
func (cc *ChatComponent) Update(msg tea.Msg) tea.Cmd {
	var cmds []tea.Cmd
	
	// Atualizar textarea
	ta, cmd := cc.textarea.Update(msg)
	cc.textarea = ta
	if cmd != nil {
		cmds = append(cmds, cmd)
	}

	// Atualizar viewport
	vp, cmd := cc.viewport.Update(msg)
	cc.viewport = vp
	if cmd != nil {
		cmds = append(cmds, cmd)
	}

	return tea.Batch(cmds...)
}

// View renderiza o componente de chat
func (cc *ChatComponent) View() string {
	// Cabeçalho do chat
	header := lipgloss.NewStyle().
		Foreground(lipgloss.Color(PrimaryColor)).
		Bold(true).
		Padding(0, 1).
		Render("💬 Chat com Amazzo")

	// Juntar todos os elementos
	elements := []string{header, cc.viewport.View(), cc.textarea.View()}

	if cc.showHelp {
		helpText := "\n" + lipgloss.NewStyle().
			Foreground(lipgloss.Color("#6272A4")).
			Italic(true).
			Render("Pressione Enter para enviar, Ctrl+C para sair")
		elements = append(elements, helpText)
	}

	return lipgloss.JoinVertical(lipgloss.Left, elements...)
}

// GetTextarea retorna o textarea para tratamento de eventos específicos
func (cc *ChatComponent) GetTextarea() textarea.Model {
	return cc.textarea
}

// ToggleHelp alterna exibição de ajuda
func (cc *ChatComponent) ToggleHelp() {
	cc.showHelp = !cc.showHelp
}