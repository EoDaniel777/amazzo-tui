package ui

import (
	"context"
	"fmt"
	tea "github.com/charmbracelet/bubbletea"
)

// AIInterface provides an interface to interact with the AI models
type AIInterface interface {
	SendMessage(ctx context.Context, message string) (string, error)
	GetAvailableModels() []string
	GetCurrentModel() string
	SetModel(model string) error
}

// MockAIInterface is a mock implementation for UI testing
type MockAIInterface struct{}

func (m *MockAIInterface) SendMessage(ctx context.Context, message string) (string, error) {
	return "Olá! Esta é uma resposta da IA. Em implementação completa, esta mensagem viria de um modelo de IA real como Qwen, Claude etc.", nil
}

func (m *MockAIInterface) GetAvailableModels() []string {
	return []string{
		"Qwen3-Coder 480B (MoE Massivo)",
		"Qwen3-Coder 30B (Rápido)",
		"DeepSeek-R1 (Reasoning)",
		"Kimi-K2 Thinking (Moonshot AI)",
	}
}

func (m *MockAIInterface) GetCurrentModel() string {
	return "Qwen3-Coder 480B (MoE Massivo)"
}

func (m *MockAIInterface) SetModel(model string) error {
	fmt.Printf("Modelo alterado para: %s\n", model)
	return nil
}

// RealAIInterface wraps the actual app functionality
type RealAIInterface struct {
	// This would contain the reference to the real app model
	// For now, we'll implement this as a placeholder
}

func NewRealAIInterface(hfToken, workspace string) (*RealAIInterface, error) {
	// Initialize the real app model here
	// model, err := app.New(hfToken, workspace)
	// if err != nil {
	//     return nil, err
	// }
	return &RealAIInterface{}, nil
}

func (r *RealAIInterface) SendMessage(ctx context.Context, message string) (string, error) {
	// This would call the actual AI model
	// For now, return a placeholder
	return "Resposta do modelo real de IA", nil
}

func (r *RealAIInterface) GetAvailableModels() []string {
	// Return actual available models
	return []string{
		"Qwen3-Coder 480B (MoE Massivo)",
		"Qwen3-Coder 30B (Rápido)", 
		"DeepSeek-R1 (Reasoning)",
		"Kimi-K2 Thinking (Moonshot AI)",
	}
}

func (r *RealAIInterface) GetCurrentModel() string {
	return "Qwen3-Coder 480B (MoE Massivo)"
}

func (r *RealAIInterface) SetModel(model string) error {
	// This would change the model in the real app
	return nil
}

// ChatHandler handles chat interactions using the AI interface
type ChatHandler struct {
	ai AIInterface
}

func NewChatHandler(ai AIInterface) *ChatHandler {
	return &ChatHandler{
		ai: ai,
	}
}

func (ch *ChatHandler) ProcessUserMessage(message string) (string, error) {
	// In a real implementation, this would send the message to the AI
	// and return the response
	return ch.ai.SendMessage(context.Background(), message)
}

func (ch *ChatHandler) GetAIResponseCmd(userMessage string) tea.Cmd {
	return func() tea.Msg {
		response, err := ch.ProcessUserMessage(userMessage)
		if err != nil {
			return ChatErrorMsg{Err: err}
		}
		return ChatResponseMsg{Response: response}
	}
}

// Message types for Tea communication
type ChatResponseMsg struct {
	Response string
}

type ChatErrorMsg struct {
	Err error
}