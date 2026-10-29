package app

import "amazzo/agents"

// ChatCompletionRequest representa a requisição para completar um chat.
type ChatCompletionRequest struct {
	Model       string             `json:"model"`
	Messages    []agents.Message    `json:"messages"`
	Stream      bool               `json:"stream"`
	MaxTokens   int                `json:"max_tokens,omitempty"`
	Temperature float64            `json:"temperature,omitempty"`
	TopP        float64            `json:"top_p,omitempty"`
}

// ChatCompletionResponse representa a resposta da API.
type ChatCompletionResponse struct {
	ID      string      `json:"id"`
	Object  string      `json:"object"`
	Created int64       `json:"created"`
	Model   string      `json:"model"`
	Choices []Choice    `json:"choices"`
	Usage   TokenUsage  `json:"usage,omitempty"`
}

// TokenUsage representa o uso de tokens da API.
type TokenUsage struct {
	PromptTokens     int `json:"prompt_tokens"`
	CompletionTokens int `json:"completion_tokens"`
	TotalTokens      int `json:"total_tokens"`
}

// Choice representa uma escolha na resposta.
type Choice struct {
	Index        int            `json:"index"`
	Message      agents.Message `json:"message"`
	FinishReason string         `json:"finish_reason"`
}
