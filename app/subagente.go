package app

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"amazzo/agents"
	"amazzo/mcp"
)

// Subagente: uma conversa própria com o mesmo modelo, só com ferramentas de
// leitura. Ele pesquisa o projeto sozinho e devolve a conclusão; o que ele leu
// não entra no contexto da conversa principal. Vários no mesmo lote rodam em
// paralelo (ver mcp.Server.Handle).

var ferramentasDoSubagente = map[string]bool{
	"read_file": true, "list_dir": true, "search_files": true, "file_info": true,
	"web_fetch": true, "web_search": true,
}

const promptDoSubagente = `Você é um subagente de pesquisa do Amazzo. Recebeu uma tarefa de outro agente.
Investigue com as ferramentas (só leitura: read_file, list_dir, search_files, file_info, web_fetch, web_search).
Use search_files para achar e read_file com start_line/end_line para ler só o que importa.
Não escreva arquivos, não tente rodar comandos. Quando tiver a resposta, pare de chamar ferramentas e responda
com um relatório curto e concreto: o que descobriu, com caminhos de arquivo e números de linha. Sem rodeios.
Nunca escreva resultados de ferramenta no seu texto: o Amazzo os devolve a você.`

func ferramentasSomenteLeitura() []agents.Tool {
	var out []agents.Tool
	for _, t := range agents.DefaultTools() {
		if ferramentasDoSubagente[t.Function.Name] {
			out = append(out, t)
		}
	}
	return out
}

// delegadoParaSubagente devolve a função que o servidor MCP chama em subagent.
// Recebe só valores prontos (nada do Model), porque roda em goroutines.
func delegadoParaSubagente(srv *mcp.Server, url, token, modelo, contexto string) func(context.Context, string) (string, error) {
	return func(ctx context.Context, tarefa string) (string, error) {
		return rodaSubagente(ctx, srv, url, token, modelo, contexto, tarefa)
	}
}

func rodaSubagente(ctx context.Context, srv *mcp.Server, url, token, modelo, contexto, tarefa string) (string, error) {
	sys := promptDoSubagente + contexto
	msgs := []wireMessage{
		{Role: "system", Content: strPtr(sys)},
		{Role: "user", Content: strPtr(tarefa)},
	}
	tools := ferramentasSomenteLeitura()
	client := &http.Client{Timeout: 180 * time.Second}

	for rodada := 0; ; rodada++ {
		if err := ctx.Err(); err != nil {
			return "", err
		}
		corpo, err := json.Marshal(wireRequest{Model: modelo, Messages: msgs, Tools: tools, MaxTokens: 8192, Temperature: 0.2})
		if err != nil {
			return "", err
		}
		req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, strings.NewReader(string(corpo)))
		if err != nil {
			return "", err
		}
		req.Header.Set("Authorization", "Bearer "+token)
		req.Header.Set("Content-Type", "application/json")
		resp, err := client.Do(req)
		if err != nil {
			return "", err
		}
		b, err := io.ReadAll(resp.Body)
		resp.Body.Close()
		if err != nil {
			return "", err
		}
		if resp.StatusCode >= 300 {
			return "", fmt.Errorf("API respondeu %d: %s", resp.StatusCode, limitaTexto(string(b), 200))
		}
		var wr wireResponse
		if err := json.Unmarshal(b, &wr); err != nil || len(wr.Choices) == 0 {
			return "", fmt.Errorf("resposta inválida da API")
		}
		msg := wr.Choices[0].Message
		if len(msg.ToolCalls) == 0 {
			texto := strings.TrimSpace(cortaSaidaInventada(msg.Content))
			if texto == "" {
				texto = "(o subagente terminou sem relatório)"
			}
			return fmt.Sprintf("%s\n\n(subagente: %d consultas)", texto, rodada), nil
		}

		am := wireMessage{Role: "assistant"}
		if strings.TrimSpace(msg.Content) != "" {
			am.Content = strPtr(msg.Content)
		}
		var reqs []mcp.Request
		var ids []string
		for i, c := range msg.ToolCalls {
			id := c.ID
			if id == "" {
				id = fmt.Sprintf("sub_%d_%d", rodada, i)
			}
			args := argumentosDaChamada(c.Function.Arguments)
			ab, _ := json.Marshal(args)
			am.ToolCalls = append(am.ToolCalls, wireToolCall{ID: id, Type: "function",
				Function: wireToolFunction{Name: c.Function.Name, Arguments: string(ab)}})
			ids = append(ids, id)
			payload, _ := json.Marshal(map[string]interface{}{"name": c.Function.Name, "arguments": args})
			r, ok := requestDoPayload(string(payload))
			if !ok || !ferramentasDoSubagente[strings.ToLower(c.Function.Name)] {
				r = mcp.Request{Tool: "__negada__:" + c.Function.Name}
			}
			reqs = append(reqs, r)
		}
		msgs = append(msgs, am)

		for i, r := range reqs {
			var saida string
			if strings.HasPrefix(r.Tool, "__negada__:") {
				saida = "Ferramenta não disponível para subagente (só leitura): " + strings.TrimPrefix(r.Tool, "__negada__:")
			} else {
				res, _ := srv.Handle(ctx, []mcp.Request{r})
				if len(res) > 0 {
					saida = res[0].Content
					if res[0].Status != "success" {
						saida = "ERRO: " + res[0].Error + "\n" + saida
					}
				}
			}
			msgs = append(msgs, wireMessage{Role: "tool", ToolCallID: ids[i], Content: strPtr(limitaTexto(saida, 30000))})
		}
	}
}

func limitaTexto(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "\n… (cortado)"
}
