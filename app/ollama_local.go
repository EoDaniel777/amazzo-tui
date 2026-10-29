package app

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"sync"
	"time"

	"amazzo/agents"
	"amazzo/mcp"

	tea "github.com/charmbracelet/bubbletea"
)

// ollamaChunkMsg carrega um pedaco da resposta local ainda em andamento.
// thinking marca o raciocinio de modelos como lfm2.5: aparece na previa,
// mas nao faz parte da resposta.
type ollamaChunkMsg struct {
	text     string
	thinking bool
}

// ollamaDoneMsg encerra o streaming local.
//
// toolCalls vem separado do texto de proposito: e o canal nativo do modelo, e
// e nele que a chamada precisa entrar no historico para o template de chat
// abrir um turno de ferramenta.
type ollamaDoneMsg struct {
	full      string
	toolCalls []agents.ToolCall
	stats     agents.OllamaStats
	err       error
}

// localSystemPrompt e a versao enxuta do prompt do agente, usada com modelos
// rodando no Ollama.
//
// Motivo: nesta maquina o prompt e processado a ~20 tok/s. O prompt completo
// tem ~950 tokens, ou seja ~50s so para ser lido sempre que o cache de prefixo
// do Ollama esfria. Alem disso, modelos pequenos (4B) seguem instrucoes curtas
// e diretas com bem mais fidelidade do que um texto longo e repetitivo.
//
// As assinaturas das ferramentas sao identicas as do prompt completo: mudar
// uma chave aqui quebra o parser de blocos ```tool.
const localSystemPrompt = "Você é o Amazzo, um assistente de programação que trabalha no terminal.\n" +
	"Responda SEMPRE em Português do Brasil.\n" +
	"\n" +
	"QUANDO USAR FERRAMENTA\n" +
	"A pergunta é sobre CONHECIMENTO ou sobre o ESTADO DESTE PROJETO?\n" +
	"- Conhecimento (o que é X, como funciona Y, escreva um exemplo, cumprimento, conversa)\n" +
	"  → você já sabe. Responda em texto, NÃO use ferramenta.\n" +
	"- Estado do projeto (o que tem nesta pasta, o que esse arquivo faz, esse teste passa)\n" +
	"  → você NÃO sabe sem olhar. Use ferramenta antes de responder.\n" +
	"- Pedido para ler, criar, alterar, rodar ou procurar → use ferramenta.\n" +
	"\n" +
	"MOSTRAR NÃO É SALVAR\n" +
	"\"me mostre\", \"exiba\", \"escreva um exemplo\", \"como se faz\" → escreva o\n" +
	"código na resposta, dentro de um bloco de código. NÃO crie arquivo.\n" +
	"Só use write_file quando o usuário pedir para criar, salvar ou gravar algo.\n" +
	"Na dúvida, mostre o código e pergunte se ele quer que você salve.\n" +
	"Você nunca sai explorando o projeto por conta própria: só olha o que a\n" +
	"pergunta exige. Mas o que a pergunta exige, você OLHA: antes de descrever,\n" +
	"resumir, documentar ou corrigir um arquivo, leia ele com read_file. Nunca\n" +
	"deduza o conteúdo pelo nome.\n" +
	"\n" +
	"FERRAMENTAS\n" +
	"Para agir, emita UM bloco e espere o resultado antes do próximo:\n" +
	"\n" +
	"```tool\n" +
	"{\"tool\": \"list_dir\", \"path\": \".\"}\n" +
	"```\n" +
	"\n" +
	"Use exatamente estas chaves:\n" +
	"{\"tool\":\"read_file\",\"path\":\"...\"}\n" +
	"{\"tool\":\"write_file\",\"path\":\"...\",\"content\":\"...\"}\n" +
	"{\"tool\":\"edit_file\",\"path\":\"...\",\"search\":\"...\",\"replace\":\"...\"}\n" +
	"{\"tool\":\"list_dir\",\"path\":\".\"}\n" +
	"{\"tool\":\"search_files\",\"pattern\":\"...\",\"path\":\".\"}\n" +
	"{\"tool\":\"make_dir\",\"path\":\"...\"}\n" +
	"{\"tool\":\"execute_command\",\"command\":\"go test ./...\"}\n" +
	"{\"tool\":\"web_fetch\",\"url\":\"https://...\"}\n" +
	"{\"tool\":\"set_workspace\",\"path\":\"/caminho/do/projeto\"}\n" +
	"\n" +
	"PASTA DE TRABALHO\n" +
	"Você só enxerga dentro da pasta de trabalho atual. Se o usuário citar um\n" +
	"projeto em outro caminho, ABRA a pasta primeiro com set_workspace e só\n" +
	"depois use as outras ferramentas — os caminhos passam a ser relativos a ela.\n" +
	"Nunca diga que não pode acessar um caminho: abra a pasta.\n" +
	"\n" +
	"REGRAS\n" +
	"- Baseie o que você diz no resultado das ferramentas. Nunca invente caminhos, nomes de arquivo ou conteúdo.\n" +
	"- NÃO afirme que algo falta (teste, README, lint, CI) sem ter verificado\n" +
	"  com uma ferramenta. Se o arquivo apareceu numa listagem, ele EXISTE:\n" +
	"  não sugira criá-lo.\n" +
	"- Recomende só o que serve para a linguagem DESTE projeto. Olhe as\n" +
	"  extensões antes: num projeto Go não existe type hint, black nem pylint.\n" +
	"- A lista ```todo é só para trabalho que exige VÁRIAS ferramentas seguidas\n" +
	"  (criar vários arquivos, montar um projeto). Cada linha é \"[ ] etapa\".\n" +
	"- Escrever código na resposta é UMA etapa. Nunca faça lista para isso:\n" +
	"  escreva o código direto.\n" +
	"- Seja conciso: sem preâmbulo, sem repetir o pedido, sem resumo final longo.\n" +
	"\n" +
	"EXEMPLOS\n" +
	"Usuário: \"olá\" → \"Olá! O que você quer fazer no projeto?\" (nenhuma ferramenta)\n" +
	"Usuário: \"o que é uma goroutine?\" → explique em texto (nenhuma ferramenta)\n" +
	"Usuário: \"o que tem nesta pasta?\" → um bloco ```tool com list_dir\n" +
	"Usuário: \"me mostre uma árvore binária em Go\" → o código na resposta, sem ferramenta\n" +
	"Usuário: \"crie uma API REST em Go\" → um bloco ```todo com as etapas, e só depois as ferramentas"

// useLocalSystemPrompt permite voltar ao prompt completo se necessario.
func useLocalSystemPrompt() bool {
	return os.Getenv("AMAZZO_FULL_PROMPT") != "1"
}

// isLocalModel indica se o modelo selecionado roda no Ollama.
func isLocalModel(cfg agents.ModelConfig) bool {
	return strings.HasPrefix(cfg.DisplayName, "🏠")
}

// localHistory monta a conversa enviada ao Ollama.
//
// O ganho decisivo de velocidade vem do cache de prefixo do Ollama: se o
// inicio da conversa nao muda, ele reaproveita o estado ja processado
// (medimos 62s -> 3s no mesmo prompt). Por isso o system prompt fica fixo e
// so cortamos o historico quando ele realmente ameaca estourar num_ctx --
// cada corte invalida o cache dali para frente.
func (m *Model) localHistory() []agents.Message {
	history := m.conversationHistory
	if len(history) == 0 {
		return history
	}

	system := history[0]
	if useLocalSystemPrompt() && system.Role == "system" {
		system = agents.Message{Role: "system", Content: localSystemPrompt}
	}

	rest := history[1:]
	if len(rest) == 0 {
		return []agents.Message{system}
	}

	opts := agents.DefaultOllamaOptions()
	budget := opts.NumCtx
	// Deixa espaco para a resposta. Sem teto de geracao (-1) reservamos um
	// terco da janela, senao a resposta nao teria para onde crescer.
	if opts.NumPredict > 0 {
		budget -= opts.NumPredict
	} else {
		budget -= opts.NumCtx / 3
	}
	budget -= estimateTokens(system.Content)
	// Os schemas das ferramentas vao no mesmo prompt e nao eram descontados:
	// ~850 tokens nesta maquina, 10% do num_ctx. Ignora-los fazia o corte do
	// historico acontecer tarde, e o Ollama fazia context shift -- que joga
	// fora o cache de prefixo e reprocessa tudo.
	budget -= estimateTokens(textoDosSchemas())
	budget -= budget / 5

	if budget < 0 {
		budget = 0
	}

	// Mantem as mensagens mais recentes que couberem no orcamento.
	used := 0
	cut := 0
	for i := len(rest) - 1; i >= 0; i-- {
		used += estimateTokens(rest[i].Content)
		if used > budget {
			cut = i + 1
			break
		}
	}

	// A ultima mensagem e a pergunta que acabou de ser feita: descarta-la
	// deixaria o modelo sem saber o que responder, mesmo que ela sozinha
	// estoure o orcamento.
	if cut > len(rest)-1 {
		cut = len(rest) - 1
	}

	return append([]agents.Message{system}, rest[cut:]...)
}

// callOllamaStream conversa com o Ollama mostrando a resposta enquanto ela e
// gerada, em vez de deixar a tela parada ate o fim.
func (m *Model) callOllamaStream(model string) tea.Cmd {
	messages := m.localHistory()

	ctx, cancel := context.WithCancel(context.Background())
	m.cancelFunc = cancel

	ch := make(chan tea.Msg, 64)
	m.streamCh = ch

	go func() {
		defer close(ch)
		full, toolCalls, stats, err := agents.StreamOllama(ctx, model, messages,
			agents.DefaultTools(), func(chunk string, thinking bool) {
				select {
				case ch <- ollamaChunkMsg{text: chunk, thinking: thinking}:
				case <-ctx.Done():
				}
			})

		ch <- ollamaDoneMsg{full: full, toolCalls: toolCalls, stats: stats, err: err}
	}()

	return waitForOllama(ch)
}

// waitForOllama entrega ao Bubble Tea a proxima mensagem do stream.
func waitForOllama(ch <-chan tea.Msg) tea.Cmd {
	return func() tea.Msg {
		msg, ok := <-ch
		if !ok {
			return nil
		}
		return msg
	}
}

// streamingPreview mostra o que está acontecendo AGORA.
//
// Com um modelo na CPU a resposta leva dezenas de segundos. Um spinner com
// "aguardando requisição" é linguagem de API: aqui não há requisição para
// aguardar, há um modelo escrevendo. A prévia mostra o raciocínio enquanto ele
// pensa e o texto real assim que ele começa a responder.
//
// Mostramos o texto cru (sem Glamour) porque re-renderizar markdown a cada
// token é caro e faz a TUI piscar.
func (m *Model) streamingPreview() string {
	width := m.viewport.Width - 2
	if width < 24 {
		width = 24
	}

	// Já respondendo: o conteúdo real tem prioridade sobre o raciocínio.
	if m.streamPartial != "" {
		return entry{
			rail:  colSignal,
			label: "escrevendo",
			meta:  formatElapsed(time.Since(m.streamStart)),
			body:  ultimasLinhas(m.streamPartial, width-2, 12),
			width: width,
		}.render()
	}

	if m.streamThinking != "" {
		return renderThoughtLive(
			ultimasLinhas(m.streamThinking, width-2, 10),
			time.Since(m.streamStart), width)
	}

	// Ainda lendo o prompt: nesta CPU isso sozinho leva dezenas de segundos,
	// então dizer o que está acontecendo evita a sensação de travamento.
	return entry{
		rail:  colGraphite,
		label: "lendo o pedido",
		meta:  formatElapsed(time.Since(m.streamStart)),
		width: width,
	}.render()
}

// ultimasLinhas mantém só o fim do texto, para a prévia não empurrar o layout.
func ultimasLinhas(texto string, width, max int) string {
	linhas := strings.Split(wrapText(texto, width), "\n")
	if len(linhas) > max {
		linhas = linhas[len(linhas)-max:]
	}
	return strings.Join(linhas, "\n")
}

// textoDasToolCalls serializa as chamadas nativas no mesmo formato JSON que o
// parser já entende.
//
// Os modelos declaram "tools" e o Ollama às vezes devolve tool_calls
// estruturados, às vezes o mesmo JSON solto no texto. Normalizar aqui evita
// dois caminhos diferentes no resto do fluxo.
func textoDasToolCalls(calls []agents.ToolCall) string {
	var sb strings.Builder

	for _, c := range calls {
		if c.Function.Name == "" {
			continue
		}
		bruto, err := json.Marshal(map[string]interface{}{
			"name":      c.Function.Name,
			"arguments": c.Function.Arguments,
		})
		if err != nil {
			continue
		}
		sb.WriteString("\n" + string(bruto) + "\n")
	}

	return sb.String()
}

// schemasEmTexto guarda os schemas serializados: eles nao mudam durante a
// execucao e serializar a cada mensagem seria desperdicio.
var schemasEmTexto struct {
	sync.Once
	texto string
}

// textoDosSchemas devolve os schemas das ferramentas serializados, para medir
// quanto do prompt eles ocupam.
func textoDosSchemas() string {
	schemasEmTexto.Do(func() {
		if bruto, err := json.Marshal(agents.DefaultTools()); err == nil {
			schemasEmTexto.texto = string(bruto)
		}
	})
	return schemasEmTexto.texto
}

// mensagensDeFerramenta converte os resultados no formato que o template de
// chat do modelo entende: uma mensagem role "tool" por resultado, em texto
// puro.
//
// O caminho antigo mandava tudo numa mensagem role "user" com o array de
// Results em JSON escapado. Medido com lfm2.5:8b sobre a mesma listagem de 30
// entradas: como "user" o modelo devolveu 22 entradas, omitindo agents/ e
// mcp/ -- ele parafraseava o JSON; como "tool" devolveu as 30 na ordem exata,
// e ainda com 54 tokens de prompt a menos.
func mensagensDeFerramenta(resultados []mcp.Result) []agents.Message {
	msgs := make([]agents.Message, 0, len(resultados))

	// Orcamento do lote inteiro: um quarto da janela. O resto e para o system
	// prompt, os schemas, a conversa e a resposta.
	orcamento := agents.DefaultOllamaOptions().NumCtx / 4
	if n := len(resultados); n > 1 {
		orcamento /= n
	}

	for _, res := range resultados {
		conteudo := res.Content
		if res.Error != "" {
			// O erro tambem e informacao: foi lendo "esta fora da pasta de
			// trabalho" que o modelo aprendeu a chamar set_workspace. O Content
			// que acompanha o erro e a saida parcial do comando -- costuma ser
			// justamente o log grande de um build que falhou.
			conteudo = "ERRO: " + res.Error
			if res.Content != "" {
				conteudo += "\n" + res.Content
			}
		}

		conteudo = truncaResultado(conteudo, orcamento)
		if strings.TrimSpace(conteudo) == "" {
			conteudo = "(sem saida)"
		}

		msgs = append(msgs, agents.Message{
			Role:     "tool",
			ToolName: strings.ToLower(res.Tool),
			Content:  conteudo,
		})
	}

	return msgs
}

// achataMensagensDeFerramenta converte os turnos de ferramenta em texto, para
// as APIs que nao aceitam role "tool" com este formato (o caminho do Hugging
// Face). O historico fica com uma unica representacao -- a nativa -- e a
// traducao acontece na hora de enviar.
func achataMensagensDeFerramenta(historico []agents.Message) []agents.Message {
	var achatado []agents.Message

	for _, msg := range historico {
		switch msg.Role {
		case "tool":
			texto := "Resultado de " + msg.ToolName + ":\n" + msg.Content
			// Junta resultados consecutivos numa unica mensagem de usuario.
			if n := len(achatado); n > 0 && achatado[n-1].Role == "user" &&
				strings.HasPrefix(achatado[n-1].Content, "Resultado de ") {
				achatado[n-1].Content += "\n\n" + texto
				continue
			}
			achatado = append(achatado, agents.Message{Role: "user", Content: texto})

		case "assistant":
			// A chamada nativa volta a ser texto para o modelo remoto ver o
			// que ele mesmo pediu.
			copia := msg
			copia.Content += textoDasToolCalls(msg.ToolCalls)
			copia.ToolCalls = nil
			if strings.TrimSpace(copia.Content) == "" {
				continue
			}
			achatado = append(achatado, copia)

		default:
			achatado = append(achatado, msg)
		}
	}

	return achatado
}

// truncaResultado limita um resultado de ferramenta ao orcamento de tokens,
// mantendo o comeco e o fim.
//
// Sem isso um unico read_file grande estoura o num_ctx: app/model.go tem 3280
// linhas (~30 mil tokens), quase quatro vezes a janela. O Ollama entao faz
// context shift -- descarta metade da janela e joga fora o cache de prefixo --
// e a mensagem seguinte reprocessa tudo do zero, o que nesta CPU (~20 tok/s de
// prompt) custa minutos.
//
// O comeco e o fim sao o que importa num arquivo de codigo (imports e
// assinaturas no topo, o trecho recente no fim) e o aviso no meio diz ao modelo
// como buscar o resto em vez de achar que leu tudo.
func truncaResultado(conteudo string, orcamentoTokens int) string {
	if orcamentoTokens < 256 {
		orcamentoTokens = 256
	}
	limite := orcamentoTokens * 4 // estimateTokens usa ~4 chars por token
	if len(conteudo) <= limite {
		return conteudo
	}

	linhas := strings.Split(conteudo, "\n")
	// Reserva para o aviso do meio, que tambem conta no orcamento.
	const reservaDoAviso = 240
	metade := (limite - reservaDoAviso) / 2
	if metade < 128 {
		metade = 128
	}

	var inicio, fim []string
	var usado int
	for _, l := range linhas {
		if usado+len(l)+1 > metade {
			break
		}
		inicio = append(inicio, l)
		usado += len(l) + 1
	}
	usado = 0
	for i := len(linhas) - 1; i >= len(inicio); i-- {
		if usado+len(linhas[i])+1 > metade {
			break
		}
		fim = append([]string{linhas[i]}, fim...)
		usado += len(linhas[i]) + 1
	}

	omitidas := len(linhas) - len(inicio) - len(fim)
	aviso := fmt.Sprintf(
		"\n[... %d linhas omitidas: o resultado nao cabe na janela de contexto. "+
			"Para ver um trecho especifico use search_files, ou execute_command "+
			"com sed -n 'INICIO,FIMp' ARQUIVO ...]\n", omitidas)

	return strings.Join(inicio, "\n") + aviso + strings.Join(fim, "\n")
}
