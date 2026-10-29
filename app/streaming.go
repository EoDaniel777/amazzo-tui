package app

import (
	"bufio"
	"context"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"regexp"
	"strings"
	"time"

	"amazzo/agents"
	tea "github.com/charmbracelet/bubbletea"
)

// StreamChunkMsg representa um chunk de resposta em streaming.
type StreamChunkMsg struct {
	content string
	done    bool
	err     error
}

// callAPIStream faz uma requisição em streaming para a API com fallback automático.
func (m *Model) callAPIStream(input string) tea.Cmd {
	return func() tea.Msg {
		startTime := time.Now()

		// Tenta modelo atual primeiro, depois fallback
		modelIndices := []int{m.currentModel}

		// Adiciona fallbacks em ordem de prioridade
		fallbackOrder := []int{0, 2, 6} // Qwen 32B, DeepSeek V2, Qwen 7B (rápido)
		for _, idx := range fallbackOrder {
			if idx != m.currentModel {
				modelIndices = append(modelIndices, idx)
			}
		}

		// Prepara mensagem do usuário (só uma vez, fora do loop)
		m.conversationHistory = append(m.conversationHistory, agents.Message{
			Role:    "user",
			Content: input,
		})

		inputTokenCount := estimateTokens(input)
		m.inputTokens += inputTokenCount

		trimmed := trimHistory(m.conversationHistory, 40) // Aumentado de 24 para 40 para mais contexto
		if len(trimmed) != len(m.conversationHistory) {
			m.conversationHistory = append([]agents.Message(nil), trimmed...)
		}

		var lastErr error
		for attemptNum, modelIdx := range modelIndices {
			if modelIdx >= len(availableModels) {
				continue
			}

			modelName := availableModels[modelIdx].Endpoint

			request := ChatCompletionRequest{
				Model:       modelName,
				Messages:    trimmed,
				Stream:      true,
				MaxTokens:   16384, // 16k tokens - máximo para evitar truncamento
				Temperature: 0.7,
				TopP:        0.9,
			}

			requestBody, err := json.Marshal(request)
			if err != nil {
				lastErr = fmt.Errorf("erro ao converter requisição: %w", err)
				continue
			}

			ctx, cancel := context.WithTimeout(context.Background(), 300*time.Second) // 5 minutos
			m.cancelFunc = cancel
			defer func() {
				m.cancelFunc = nil
			}()

			req, err := http.NewRequestWithContext(ctx, http.MethodPost, "https://router.huggingface.co/v1/chat/completions", strings.NewReader(string(requestBody)))
			if err != nil {
				lastErr = fmt.Errorf("erro ao criar requisição: %w", err)
				continue
			}

			req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", m.hfToken))
			req.Header.Set("Content-Type", "application/json")
			req.Header.Set("Accept", "text/event-stream")

			transport := &http.Transport{
				MaxIdleConns:          100,
				MaxIdleConnsPerHost:   10,
				IdleConnTimeout:       120 * time.Second,
				ResponseHeaderTimeout: 120 * time.Second,
				ExpectContinueTimeout: 1 * time.Second,
				TLSHandshakeTimeout:   15 * time.Second,
				ForceAttemptHTTP2:     false,
				TLSNextProto:          make(map[string]func(authority string, c *tls.Conn) http.RoundTripper), // BLOQUEIA HTTP/2
			}

			client := &http.Client{
				Timeout:   300 * time.Second,
				Transport: transport,
			}
			resp, err := client.Do(req)
			if err != nil {
				m.metrics.RecordRequest(time.Since(startTime), false, 0)
				lastErr = fmt.Errorf("erro ao fazer requisição: %w", err)
				continue
			}
			defer resp.Body.Close()

			if resp.StatusCode != http.StatusOK {
				body, _ := io.ReadAll(resp.Body)
				m.metrics.RecordRequest(time.Since(startTime), false, 0)

				// Cria mensagem de erro limpa
				errorMsg := cleanAPIError(resp.StatusCode, string(body))
				lastErr = fmt.Errorf("%s", errorMsg)

				// Se houver fallback, tenta próximo modelo
				if attemptNum < len(modelIndices)-1 {
					continue
				}

				// Se esgotou todos os modelos, retorna erro
				return StreamChunkMsg{err: fmt.Errorf("Todos os modelos falharam. Último erro: %s", errorMsg)}
			}

			// Sucesso! Processar stream SSE
			return m.processStream(resp.Body, startTime, modelIdx, attemptNum > 0)
		}

		// Se chegou aqui, todos falharam
		return StreamChunkMsg{err: fmt.Errorf("Falha ao conectar com todos os modelos. Erro: %v", lastErr)}
	}
}

// processStream processa o streaming SSE da resposta.
func (m *Model) processStream(body io.ReadCloser, startTime time.Time, modelIdx int, wasFallback bool) tea.Cmd {
	return func() tea.Msg {
		scanner := bufio.NewScanner(body)
		fullResponse := ""

		for scanner.Scan() {
			line := scanner.Text()

			// SSE format: "data: {...}"
			if !strings.HasPrefix(line, "data: ") {
				continue
			}

			data := strings.TrimPrefix(line, "data: ")

			// SSE termination signal
			if data == "[DONE]" {
				break
			}

			// Parse JSON chunk
			var chunk struct {
				Choices []struct {
					Delta struct {
						Content string `json:"content"`
					} `json:"delta"`
					FinishReason string `json:"finish_reason"`
				} `json:"choices"`
			}

			if err := json.Unmarshal([]byte(data), &chunk); err != nil {
				continue
			}

			if len(chunk.Choices) > 0 {
				content := chunk.Choices[0].Delta.Content
				fullResponse += content

				// Enviar chunk para atualização da UI
				if content != "" {
					// Aqui poderíamos enviar chunks individuais
					// mas por simplicidade vamos acumular e enviar no final
				}

				// Verificar se terminou
				if chunk.Choices[0].FinishReason != "" {
					break
				}
			}
		}

		if err := scanner.Err(); err != nil {
			m.metrics.RecordRequest(time.Since(startTime), false, 0)
			return StreamChunkMsg{err: fmt.Errorf("erro ao ler stream: %w", err)}
		}

		// Registrar métricas
		tokensUsed := estimateTokens(fullResponse)
		m.metrics.RecordRequest(time.Since(startTime), true, tokensUsed)

		// Armazenar no cache
		modelName := availableModels[modelIdx].Endpoint
		m.cache.Set(fullResponse, modelName, fullResponse)

		// Se usou fallback, notifica usuário
		if wasFallback {
			fallbackNote := fmt.Sprintf("\n\n💡 Nota: Modelo original indisponível. Resposta gerada por %s",
				availableModels[modelIdx].DisplayName)
			fullResponse += fallbackNote
		}

		return StreamChunkMsg{content: fullResponse, done: true}
	}
}

// cleanAPIError limpa erros da API removendo HTML e extraindo mensagens úteis.
func cleanAPIError(statusCode int, body string) string {
	// Remove todas as tags HTML
	htmlRegex := regexp.MustCompile(`<[^>]*>`)
	cleaned := htmlRegex.ReplaceAllString(body, "")

	// Remove espaços em branco excessivos
	cleaned = strings.TrimSpace(cleaned)
	lines := strings.Split(cleaned, "\n")
	var cleanedLines []string
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line != "" {
			cleanedLines = append(cleanedLines, line)
		}
	}
	cleaned = strings.Join(cleanedLines, " ")

	// Se ainda está muito longo ou é HTML, usa mensagem genérica
	if len(cleaned) > 300 || strings.Contains(body, "<!DOCTYPE") {
		switch statusCode {
		case 400:
			return "Erro 400: Requisição inválida. Verifique se o modelo está disponível."
		case 401:
			return "Erro 401: Token de autenticação inválido. Verifique seu HF_TOKEN."
		case 403:
			return "Erro 403: Acesso negado. Verifique suas permissões na API."
		case 404:
			return "Erro 404: Modelo não encontrado. Tente outro modelo com /agente."
		case 429:
			return "Erro 429: Muitas requisições. Aguarde alguns segundos e tente novamente."
		case 500, 502, 503:
			return fmt.Sprintf("Erro %d: Serviço temporariamente indisponível. Tente novamente em alguns instantes.", statusCode)
		default:
			return fmt.Sprintf("Erro %d: Falha na comunicação com a API.", statusCode)
		}
	}

	// Se conseguiu limpar e ficou razoável, retorna
	if len(cleaned) > 0 && len(cleaned) <= 300 {
		return fmt.Sprintf("Erro %d: %s", statusCode, cleaned)
	}

	// Fallback
	return fmt.Sprintf("Erro %d: Falha na comunicação com a API.", statusCode)
}

// handleStreamChunk processa chunks de streaming.
func (m *Model) handleStreamChunk(msg StreamChunkMsg) tea.Cmd {
	if msg.err != nil {
		m.lastError = msg.err.Error()
		m.addMessage("")
		m.addMessage(m.theme.ErrorStyle.Render("┌─────────────────────────────────────────────────────────────"))
		m.addMessage(m.theme.ErrorStyle.Render(fmt.Sprintf("│ ❌ %s", msg.err)))
		m.addMessage(m.theme.ErrorStyle.Render("│"))
		m.addMessage(m.theme.ErrorStyle.Render("│ 💡 Dicas:"))
		m.addMessage(m.theme.ErrorStyle.Render("│    • Verifique sua conexão de internet"))
		m.addMessage(m.theme.ErrorStyle.Render("│    • Tente trocar de modelo com /agente"))
		m.addMessage(m.theme.ErrorStyle.Render("│    • Use /erro para ver detalhes completos"))
		m.addMessage(m.theme.ErrorStyle.Render("└─────────────────────────────────────────────────────────────"))
		m.addMessage("")
		m.loading = false
		return nil
	}

	if msg.done {
		m.loading = false
		return m.handleAIResponse(msg.content)
	}

	// Atualizar resposta parcial na UI
	// (para implementação futura de streaming em tempo real)
	return nil
}
