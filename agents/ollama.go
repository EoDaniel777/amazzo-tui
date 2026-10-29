package agents

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"regexp"
	"strconv"
	"strings"
	"time"
)

// Nesta maquina (i5-10210U, sem GPU) o Ollama processa o prompt a ~20 tokens/s.
// Isso torna o TAMANHO DO PROMPT o fator dominante de latencia: cada 1000
// tokens de contexto custam ~50s antes do primeiro token de resposta.
// As opcoes abaixo existem para (a) limitar o que e enviado, (b) manter o
// modelo carregado e (c) evitar o context shift, que invalida o cache de
// prefixo do Ollama e obriga a reprocessar tudo.

type OllamaModel struct {
	Name string `json:"name"`
}

type OllamaTagsResponse struct {
	Models []OllamaModel `json:"models"`
}

// OllamaOptions mapeia as opcoes de runtime aceitas pelo Ollama.
type OllamaOptions struct {
	NumCtx        int     `json:"num_ctx,omitempty"`
	NumPredict    int     `json:"num_predict,omitempty"`
	NumThread     int     `json:"num_thread,omitempty"`
	Temperature   float64 `json:"temperature"`
	TopK          int     `json:"top_k,omitempty"`
	TopP          float64 `json:"top_p,omitempty"`
	RepeatPenalty float64 `json:"repeat_penalty,omitempty"`
}

type OllamaChatRequest struct {
	Model     string        `json:"model"`
	Messages  []Message     `json:"messages"`
	Stream    bool          `json:"stream"`
	KeepAlive string        `json:"keep_alive,omitempty"`
	Options   OllamaOptions `json:"options"`
	Tools     []Tool        `json:"tools,omitempty"`
}

// OllamaMessage e a mensagem devolvida pelo Ollama. Modelos com capability
// "thinking" (lfm2.5, deepseek-r1, ...) mandam o raciocinio em um campo
// separado e deixam Content vazio ate terminarem de pensar -- ler so Content
// fazia a resposta chegar vazia.
type OllamaMessage struct {
	Role      string     `json:"role"`
	Content   string     `json:"content"`
	Thinking  string     `json:"thinking"`
	ToolCalls []ToolCall `json:"tool_calls,omitempty"`
}

type OllamaChatResponse struct {
	Message OllamaMessage `json:"message"`
	Done    bool          `json:"done"`
	Error   string        `json:"error,omitempty"`

	// Metricas devolvidas quando done=true (nanossegundos).
	PromptEvalCount    int   `json:"prompt_eval_count"`
	PromptEvalDuration int64 `json:"prompt_eval_duration"`
	EvalCount          int   `json:"eval_count"`
	EvalDuration       int64 `json:"eval_duration"`
	TotalDuration      int64 `json:"total_duration"`
}

// OllamaStats resume o custo de uma chamada, para exibir na TUI.
type OllamaStats struct {
	PromptTokens int
	PromptTime   time.Duration
	GenTokens    int
	GenTime      time.Duration
	Total        time.Duration
}

// TokensPerSecond devolve a velocidade de geracao.
func (s OllamaStats) TokensPerSecond() float64 {
	if s.GenTime <= 0 {
		return 0
	}
	return float64(s.GenTokens) / s.GenTime.Seconds()
}

func envInt(key string, def int) int {
	if v := os.Getenv(key); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n >= -2 {
			return n
		}
	}
	return def
}

func envFloat(key string, def float64) float64 {
	if v := os.Getenv(key); v != "" {
		if f, err := strconv.ParseFloat(v, 64); err == nil {
			return f
		}
	}
	return def
}

// OllamaHost devolve o endereco do servidor Ollama.
func OllamaHost() string {
	host := os.Getenv("AMAZZO_OLLAMA_HOST")
	if host == "" {
		host = os.Getenv("OLLAMA_HOST")
	}
	if host == "" {
		return "http://localhost:11434"
	}
	if !strings.HasPrefix(host, "http://") && !strings.HasPrefix(host, "https://") {
		host = "http://" + host
	}
	return strings.TrimRight(host, "/")
}

// OllamaKeepAlive mantem o modelo residente entre mensagens. Sem isso o
// Ollama o descarrega em 5 minutos e a mensagem seguinte paga de novo os
// ~6s de carga dos 3,3 GB do disco.
func OllamaKeepAlive() string {
	if v := os.Getenv("AMAZZO_OLLAMA_KEEP_ALIVE"); v != "" {
		return v
	}
	return "30m"
}

// DefaultOllamaOptions monta as opcoes, permitindo ajuste por variavel de
// ambiente sem recompilar.
func DefaultOllamaOptions() OllamaOptions {
	return OllamaOptions{
		// Precisa comportar system prompt + historico + resposta. Se o
		// contexto estoura, o Ollama faz context shift: descarta metade da
		// janela, joga fora o cache de prefixo e reprocessa tudo do zero.
		NumCtx: envInt("AMAZZO_OLLAMA_NUM_CTX", 8192),
		// Sem teto de geracao (-1). Um limite so faz sentido quando se paga
		// por token; aqui o modelo e local e cortar no meio so estraga a
		// resposta -- modelos com raciocinio gastam centenas de tokens
		// pensando antes da primeira palavra. Quem limita e o num_ctx, o EOS
		// do modelo, e o Esc do usuario.
		NumPredict: envInt("AMAZZO_OLLAMA_NUM_PREDICT", -1),
		// 0 = deixa o Ollama escolher (medimos que forcar nao ajuda aqui).
		NumThread:     envInt("AMAZZO_OLLAMA_NUM_THREAD", 0),
		Temperature:   envFloat("AMAZZO_OLLAMA_TEMPERATURE", 0.3),
		TopK:          envInt("AMAZZO_OLLAMA_TOP_K", 40),
		TopP:          envFloat("AMAZZO_OLLAMA_TOP_P", 0.9),
		RepeatPenalty: envFloat("AMAZZO_OLLAMA_REPEAT_PENALTY", 1.1),
	}
}

func ollamaClient() *http.Client {
	// Sem timeout no cliente: a geracao local e legitimamente longa e o
	// controle de cancelamento fica no context.
	return &http.Client{}
}

func FetchOllamaModels() ([]ModelConfig, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, OllamaHost()+"/api/tags", nil)
	if err != nil {
		return nil, err
	}

	resp, err := ollamaClient().Do(req)
	if err != nil {
		return nil, fmt.Errorf("falha ao conectar com Ollama em %s: %w", OllamaHost(), err)
	}
	defer resp.Body.Close()

	var tags OllamaTagsResponse
	if err := json.NewDecoder(resp.Body).Decode(&tags); err != nil {
		return nil, fmt.Errorf("falha ao decodificar modelos do Ollama: %w", err)
	}

	var configs []ModelConfig
	for _, m := range tags.Models {
		configs = append(configs, ModelConfig{
			Name:        m.Name,
			DisplayName: "🏠 " + m.Name,
			Endpoint:    m.Name,
		})
	}
	return configs, nil
}

// CallOllama faz uma chamada bloqueante. Mantida para compatibilidade;
// prefira StreamOllama, que mostra a resposta enquanto ela e gerada.
func CallOllama(model string, messages []Message) (string, error) {
	text, _, _, err := StreamOllama(context.Background(), model, messages, nil, nil)
	return text, err
}

// StreamOllama envia a conversa ao Ollama e entrega cada pedaco da resposta a
// onChunk assim que chega. Devolve o texto completo, as tool calls e as metricas da chamada.
//
// O streaming nao acelera o modelo, mas elimina a espera cega: em vez de
// ~75s de tela parada, o primeiro token aparece assim que o prompt termina de
// ser processado.
func StreamOllama(ctx context.Context, model string, messages []Message, tools []Tool, onChunk func(text string, thinking bool)) (string, []ToolCall, OllamaStats, error) {
	var stats OllamaStats
	var toolCalls []ToolCall

	reqBody := OllamaChatRequest{
		Model:     model,
		Messages:  messages,
		Stream:    true,
		KeepAlive: OllamaKeepAlive(),
		Options:   DefaultOllamaOptions(),
		Tools:     tools,
	}

	jsonBody, err := json.Marshal(reqBody)
	if err != nil {
		return "", nil, stats, fmt.Errorf("erro ao montar requisição do Ollama: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost,
		OllamaHost()+"/api/chat", bytes.NewReader(jsonBody))
	if err != nil {
		return "", nil, stats, err
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := ollamaClient().Do(req)
	if err != nil {
		return "", nil, stats, fmt.Errorf("erro na requisição ao Ollama (%s): %w", OllamaHost(), err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
		return "", nil, stats, fmt.Errorf("Ollama respondeu %s: %s", resp.Status, strings.TrimSpace(string(body)))
	}

	var full strings.Builder
	scanner := bufio.NewScanner(resp.Body)
	// Um chunk pode conter um trecho grande de codigo; o buffer padrao de
	// 64 KB do bufio estoura e a resposta chega truncada.
	scanner.Buffer(make([]byte, 0, 64*1024), 4*1024*1024)

	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}

		var chunk OllamaChatResponse
		if err := json.Unmarshal([]byte(line), &chunk); err != nil {
			// Linha malformada isolada nao deve derrubar a resposta inteira.
			continue
		}
		if chunk.Error != "" {
			return full.String(), nil, stats, fmt.Errorf("Ollama: %s", chunk.Error)
		}

		// O raciocinio nao entra na resposta final, mas alimenta a previa:
		// sem isso a tela fica parada enquanto o modelo pensa.
		if chunk.Message.Thinking != "" && onChunk != nil {
			onChunk(chunk.Message.Thinking, true)
		}

		if chunk.Message.Content != "" {
			full.WriteString(chunk.Message.Content)
			if onChunk != nil {
				onChunk(chunk.Message.Content, false)
			}
		}

		if len(chunk.Message.ToolCalls) > 0 {
			toolCalls = append(toolCalls, chunk.Message.ToolCalls...)
		}

		if chunk.Done {
			stats = OllamaStats{
				PromptTokens: chunk.PromptEvalCount,
				PromptTime:   time.Duration(chunk.PromptEvalDuration),
				GenTokens:    chunk.EvalCount,
				GenTime:      time.Duration(chunk.EvalDuration),
				Total:        time.Duration(chunk.TotalDuration),
			}
		}
	}

	if err := scanner.Err(); err != nil {
		if ctx.Err() != nil {
			return full.String(), nil, stats, ctx.Err()
		}
		return full.String(), nil, stats, fmt.Errorf("erro ao ler stream do Ollama: %w", err)
	}

	return stripThinkTags(full.String()), toolCalls, stats, nil
}

// thinkTagRe casa blocos <think>...</think>. Alguns modelos embutem o
// raciocinio no proprio content em vez de usar o campo separado.
var thinkTagRe = regexp.MustCompile(`(?s)<think>.*?</think>\s*`)

// toolCallTagRe casa as tags <tool_call> que o modelo escreve ao redor de uma
// chamada quando cai no formato de texto.
//
// Elas sao andaime do template de chat, nao conteudo: o lfm2.5 emitiu
// "<tool_call>" e "</tool_call>" em linhas separadas e as duas apareceram
// dentro do balao da resposta na TUI. Removemos so as TAGS, nunca o que esta
// entre elas -- o miolo e a chamada, e o parser precisa dela.
var toolCallTagRe = regexp.MustCompile(`(?i)</?tool_call>`)

// stripThinkTags remove o raciocinio e o andaime de chamada embutidos no texto.
func stripThinkTags(s string) string {
	s = thinkTagRe.ReplaceAllString(s, "")
	s = toolCallTagRe.ReplaceAllString(s, "")
	return strings.TrimSpace(s)
}
