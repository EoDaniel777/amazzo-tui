package app

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	"amazzo/agents"
)

// Formato de fio da API compatível com OpenAI (amazzo.tech, Hugging Face).
//
// O histórico interno guarda as chamadas no formato do Ollama (argumentos como
// objeto, sem id, sem "type"). A API OpenAI exige argumentos como string JSON,
// "type":"function" e um tool_call_id que liga cada resultado à sua chamada.
// A tradução acontece só na hora de enviar.

type wireToolCall struct {
	ID       string           `json:"id"`
	Type     string           `json:"type"`
	Function wireToolFunction `json:"function"`
}

type wireToolFunction struct {
	Name      string `json:"name"`
	Arguments string `json:"arguments"`
}

type wireMessage struct {
	Role       string         `json:"role"`
	Content    *string        `json:"content"`
	ToolCalls  []wireToolCall `json:"tool_calls,omitempty"`
	ToolCallID string         `json:"tool_call_id,omitempty"`
}

type wireRequest struct {
	Model       string        `json:"model"`
	Messages    []wireMessage `json:"messages"`
	Tools       []agents.Tool `json:"tools,omitempty"`
	Stream      bool          `json:"stream"`
	MaxTokens   int           `json:"max_tokens,omitempty"`
	Temperature float64       `json:"temperature,omitempty"`
	TopP        float64       `json:"top_p,omitempty"`
}

// wireResponse lê a resposta com os argumentos como vierem (string ou objeto).
type wireResponse struct {
	Choices []struct {
		Message struct {
			Content   string `json:"content"`
			ToolCalls []struct {
				ID       string `json:"id"`
				Function struct {
					Name      string          `json:"name"`
					Arguments json.RawMessage `json:"arguments"`
				} `json:"function"`
			} `json:"tool_calls"`
		} `json:"message"`
		FinishReason string `json:"finish_reason"`
	} `json:"choices"`
	Usage TokenUsage `json:"usage"`
}

func strPtr(s string) *string { return &s }

// argumentosDaChamada aceita os dois formatos que os servidores usam: string
// JSON (OpenAI) ou objeto direto.
func argumentosDaChamada(bruto json.RawMessage) map[string]interface{} {
	args := map[string]interface{}{}
	b := strings.TrimSpace(string(bruto))
	if b == "" || b == "null" {
		return args
	}
	if strings.HasPrefix(b, "\"") {
		var s string
		if json.Unmarshal(bruto, &s) == nil {
			b = strings.TrimSpace(s)
		}
	}
	if b == "" {
		return args
	}
	_ = json.Unmarshal([]byte(b), &args)
	return args
}

// mensagensParaAPI converte o histórico para o formato de fio.
//
// Regras que a API impõe e o histórico (cortado por trimHistory, ou vindo de
// modelos que escreveram a chamada no texto) nem sempre cumpre:
//   - toda tool_call precisa de uma mensagem "tool" com o mesmo id;
//   - toda mensagem "tool" precisa ter uma chamada aberta antes dela.
//
// Resultado sem chamada correspondente vira texto de usuário; chamada sem
// resultado ganha um "(sem resultado)" para a API não recusar o pedido.
func mensagensParaAPI(historico []agents.Message) []wireMessage {
	var out []wireMessage
	type aberta struct{ id, nome string }
	var abertas []aberta // chamadas ainda sem resultado, em ordem
	contador := 0

	fechaPendentes := func() {
		for _, a := range abertas {
			out = append(out, wireMessage{Role: "tool", ToolCallID: a.id, Content: strPtr("(sem resultado)")})
		}
		abertas = nil
	}

	for _, msg := range historico {
		switch msg.Role {
		case "assistant":
			fechaPendentes()
			wm := wireMessage{Role: "assistant"}
			var calls []wireToolCall
			for _, c := range msg.ToolCalls {
				if c.Function.Name == "" {
					continue
				}
				contador++
				id := c.ID
				if id == "" {
					id = fmt.Sprintf("call_%d", contador)
				}
				args, _ := json.Marshal(c.Function.Arguments)
				if len(args) == 0 || string(args) == "null" {
					args = []byte("{}")
				}
				calls = append(calls, wireToolCall{
					ID: id, Type: "function",
					Function: wireToolFunction{Name: c.Function.Name, Arguments: string(args)},
				})
				abertas = append(abertas, aberta{id, strings.ToLower(c.Function.Name)})
			}
			wm.ToolCalls = calls
			if strings.TrimSpace(msg.Content) != "" || len(calls) == 0 {
				wm.Content = strPtr(msg.Content)
			}
			if wm.Content == nil && len(calls) == 0 {
				continue
			}
			out = append(out, wm)

		case "tool":
			if len(abertas) > 0 {
				// Casa pelo nome da ferramenta (a ordem dos resultados pode
				// divergir da ordem das chamadas: set_workspace roda antes,
				// uma chamada invalida e descartada). Sem nome igual, vale a
				// ordem.
				idx := 0
				for i, a := range abertas {
					if msg.ToolName != "" && a.nome == strings.ToLower(msg.ToolName) {
						idx = i
						break
					}
				}
				id := abertas[idx].id
				abertas = append(abertas[:idx], abertas[idx+1:]...)
				out = append(out, wireMessage{Role: "tool", ToolCallID: id, Content: strPtr(msg.Content)})
				continue
			}
			// Sem chamada aberta: o modelo escreveu a chamada no texto.
			out = append(out, wireMessage{Role: "user",
				Content: strPtr("Resultado de " + msg.ToolName + ":\n" + msg.Content)})

		default:
			fechaPendentes()
			out = append(out, wireMessage{Role: msg.Role, Content: strPtr(msg.Content)})
		}
	}
	fechaPendentes()
	return out
}

// Saída inventada: o modelo escreve, na mesma resposta que pede a ferramenta,
// um "resultado" que ele imagina. Foi o que fez o Amazzo descrever um projeto
// Python que não existia -- ele leu a própria alucinação como se fosse a pasta.
var saidaInventadaRe = regexp.MustCompile(
	`(?mi)^[ \t>*_` + "`" + `-]*(ferramenta executada com .*|tool (result|output) ?:)`)

// cortaSaidaInventada descarta tudo a partir do ponto em que o modelo começa a
// fingir um resultado de ferramenta. Quem devolve resultado é o Amazzo.
func cortaSaidaInventada(resposta string) string {
	loc := saidaInventadaRe.FindStringIndex(resposta)
	if loc == nil {
		return resposta
	}
	return strings.TrimRight(resposta[:loc[0]], " \t\r\n")
}

// contextoDoProjeto descreve a pasta de trabalho: tipo do projeto e a raiz.
//
// O modelo errava a linguagem do projeto porque só descobria olhando arquivo
// por arquivo (e a primeira suposição vinha da alucinação). Dizer o que é o
// projeto antes da primeira pergunta elimina essa classe de erro e os ciclos
// gastos em confirmar.
func contextoDoProjeto(raiz string) string {
	if raiz == "" {
		return ""
	}
	entradas, err := os.ReadDir(raiz)
	if err != nil {
		return ""
	}

	marcadores := []struct{ arquivo, tipo string }{
		{"go.mod", "Go"},
		{"Cargo.toml", "Rust"},
		{"package.json", "JavaScript/TypeScript (Node)"},
		{"pyproject.toml", "Python"},
		{"requirements.txt", "Python"},
		{"setup.py", "Python"},
		{"pom.xml", "Java (Maven)"},
		{"build.gradle", "Java/Kotlin (Gradle)"},
		{"Gemfile", "Ruby"},
		{"composer.json", "PHP"},
		{"mix.exs", "Elixir"},
	}
	existe := map[string]bool{}
	var nomes []string
	for _, e := range entradas {
		n := e.Name()
		if strings.HasPrefix(n, ".") && n != ".github" {
			continue
		}
		if e.IsDir() {
			n += "/"
		}
		existe[strings.TrimSuffix(n, "/")] = true
		nomes = append(nomes, n)
	}
	sort.Strings(nomes)

	var tipos []string
	vistos := map[string]bool{}
	var achados []string
	for _, mk := range marcadores {
		if existe[mk.arquivo] {
			achados = append(achados, mk.arquivo)
			if !vistos[mk.tipo] {
				vistos[mk.tipo] = true
				tipos = append(tipos, mk.tipo)
			}
		}
	}

	// Sem marcador: conta extensões em um nível, para não chutar.
	if len(tipos) == 0 {
		contagem := map[string]int{}
		extensoes := map[string]string{".go": "Go", ".py": "Python", ".rs": "Rust",
			".js": "JavaScript", ".ts": "TypeScript", ".java": "Java", ".rb": "Ruby", ".php": "PHP"}
		for _, e := range entradas {
			if !e.IsDir() {
				if t, ok := extensoes[strings.ToLower(filepath.Ext(e.Name()))]; ok {
					contagem[t]++
				}
			}
		}
		for t := range contagem {
			tipos = append(tipos, t)
		}
		sort.Strings(tipos)
	}

	var sb strings.Builder
	sb.WriteString("\n\n## Contexto desta sessão (verificado pelo Amazzo, não precisa conferir de novo)\n\n")
	sb.WriteString("Pasta de trabalho: " + raiz + "\n")
	if len(tipos) > 0 {
		sb.WriteString("Linguagem do projeto: " + strings.Join(tipos, ", "))
		if len(achados) > 0 {
			sb.WriteString(" (detectada por " + strings.Join(achados, ", ") + ")")
		}
		sb.WriteString("\nSugira só o que serve a essa linguagem. Não assuma outra.\n")
	}
	const max = 60
	if len(nomes) > max {
		nomes = append(nomes[:max], fmt.Sprintf("… +%d", len(nomes)-max))
	}
	sb.WriteString("Conteúdo da raiz: " + strings.Join(nomes, "  ") + "\n")
	sb.WriteString("Você já sabe a listagem da raiz: não chame list_dir nela de novo; vá direto aos arquivos que importam.\n")
	return sb.String()
}

// pedidoDeArquivoRe reconhece pedidos que exigem gravar um arquivo: "crie um
// arquivo markdown", "salve em ...", "gere o relatório.md".
var pedidoDeArquivoRe = regexp.MustCompile(
	`(?i)\b(cri[ae]r?|gere|gerar|escrev[ae]r?|salv[ae]r?|grav[ae]r?|monte|montar|adicion[ae]r?)\b[^.\n]{0,80}(arquivo|\.md\b|\.txt\b|markdown|documento|relat[óo]rio|readme|script)`)

// garantiaDeEntrega devolve o texto de cobrança quando o usuário pediu um
// arquivo, o modelo está terminando o turno e nenhuma escrita aconteceu.
// Visto em sessão real: o modelo analisou o projeto inteiro e fechou com "se
// quiser, eu crio o arquivo" -- o pedido era para criar. Cobra no máximo duas
// vezes, para não virar loop se o modelo não consegue mesmo.
func (m *Model) garantiaDeEntrega(resposta string) string {
	if m.escreveuArq || m.cobrancasArq >= 2 || strings.TrimSpace(m.pedidoAtual) == "" {
		return ""
	}
	pedido := m.pedidoAtual
	// O texto de arquivos anexados com @ nao e o pedido do usuario.
	if i := strings.Index(pedido, "--- Conteúdo de"); i >= 0 {
		pedido = pedido[:i]
	}
	if !pedidoDeArquivoRe.MatchString(pedido) || negaOuPergunta.MatchString(pedido) {
		return ""
	}
	m.cobrancasArq++
	return "O usuário pediu para CRIAR um arquivo e nenhum arquivo foi gravado ainda. " +
		"Não pergunte e não descreva: chame write_file agora, com o caminho pedido e o " +
		"conteúdo COMPLETO, usando o que você já levantou. Depois de gravar, diga em uma frase o que contém."
}

// negaOuPergunta: "nao crie arquivo", "como criar um arquivo?" nao sao pedidos
// de criacao.
var negaOuPergunta = regexp.MustCompile(
	`(?i)(\b(n[ãa]o|nunca|sem|evite)\b[^.\n]{0,30}\b(cri|ger|escrev|salv|grav)|^\s*(como|qual|quais|o que|por que|porque)\b)`)
