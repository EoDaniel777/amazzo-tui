package app

import (
	"fmt"
)

// validateAPIResponse verifica se a resposta da API é válida e completa.
func validateAPIResponse(resp ChatCompletionResponse) error {
	if len(resp.Choices) == 0 {
		return fmt.Errorf("resposta vazia da API")
	}

	choice := resp.Choices[0]

	// Verificar finish_reason
	switch choice.FinishReason {
	case "stop":
		// Resposta completa - tudo OK
		return nil
	case "length":
		// TRUNCADO: Não retorna erro - deixa o sistema lidar com continuação
		// O handleAIResponse vai detectar e continuar automaticamente
		return nil
	case "content_filter":
		return fmt.Errorf("resposta bloqueada pelo filtro de conteúdo da API")
	case "":
		// Alguns modelos não retornam finish_reason
		return nil
	default:
		return fmt.Errorf("finish_reason desconhecido: %s", choice.FinishReason)
	}
}

// validateRequest verifica se a requisição está bem formada.
func validateRequest(req ChatCompletionRequest) error {
	if req.Model == "" {
		return fmt.Errorf("modelo não especificado")
	}

	if len(req.Messages) == 0 {
		return fmt.Errorf("nenhuma mensagem na requisição")
	}

	if req.MaxTokens < 0 {
		return fmt.Errorf("max_tokens deve ser positivo")
	}

	if req.Temperature < 0 || req.Temperature > 2 {
		return fmt.Errorf("temperature deve estar entre 0 e 2")
	}

	if req.TopP < 0 || req.TopP > 1 {
		return fmt.Errorf("top_p deve estar entre 0 e 1")
	}

	return nil
}
