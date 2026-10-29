package app

import (
	"strings"
	"testing"
	"time"

	"amazzo/agents"
	"amazzo/filesystem"
	"amazzo/mcp"
)

// O parser de texto e a rede de segurança para quando o modelo não usa o canal
// nativo de tool_calls. Medimos com o lfm2.5:8b que ele cai nestes formatos, e
// cada um que passa batido vira JSON na tela e a tarefa para.
func TestParserAceitaOsFormatosQueOsModelosEmitem(t *testing.T) {
	m := &Model{}

	casos := []struct {
		nome  string
		texto string
	}{
		{"array de chamadas",
			`[{"tool":"list_dir","path":"."},{"tool":"read_file","path":"go.mod"}]`},
		{"array no formato nativo",
			`[{"name":"list_dir","arguments":{"path":"."}}]`},
		{"envelope tool_calls",
			`{"tool_calls":[{"function":{"name":"list_dir","arguments":{"path":"."}}}]}`},
		{"inline depois de prosa",
			`Vou listar a pasta: {"tool": "list_dir", "path": "."}`},
		{"item de lista markdown",
			`- {"tool":"list_dir","path":"."}`},
		{"cercado com tag tool",
			"```tool\n{\"tool\":\"list_dir\",\"path\":\".\"}\n```"},
		{"depois de dois pontos e nova linha",
			"Chamada:\n{\"tool\":\"list_dir\",\"path\":\".\"}"},
	}

	for _, c := range casos {
		reqs := m.parseToolRequests(c.texto)
		if len(reqs) == 0 {
			t.Errorf("%s: nenhuma chamada reconhecida em %q", c.nome, c.texto)
			continue
		}
		if reqs[0].Tool != "list_dir" {
			t.Errorf("%s: primeira chamada é %q, esperava list_dir", c.nome, reqs[0].Tool)
		}
	}

	// O array com duas chamadas precisa render as duas.
	reqs := m.parseToolRequests(casos[0].texto)
	if len(reqs) != 2 {
		t.Errorf("array com 2 chamadas virou %d pedido(s)", len(reqs))
	}
}

// Um objeto JSON qualquer no meio da resposta não é chamada de ferramenta. O
// caso que dói é o modelo MOSTRAR um package.json: ele tem "name" e viraria
// uma chamada para uma ferramenta inexistente.
func TestJSONComumNaoViraChamada(t *testing.T) {
	m := &Model{}

	naoSaoChamadas := []string{
		`{"name":"meu-projeto","version":"1.0.0"}`,
		"```json\n{\"name\": \"exemplo\", \"dependencies\": {}}\n```",
		`{"tool":"ferramenta_que_nao_existe","path":"."}`,
		`{"status":"success","content":"algo"}`,
	}

	for _, texto := range naoSaoChamadas {
		if reqs := m.parseToolRequests(texto); len(reqs) != 0 {
			t.Errorf("%q virou %d chamada(s): %+v", texto, len(reqs), reqs)
		}
	}
}

// O resultado da ferramenta precisa voltar no canal que o template de chat do
// modelo entende (role "tool"), e em texto puro.
//
// Medido com lfm2.5:8b sobre a mesma listagem de 30 entradas:
//   - como role "user" com JSON escapado: o modelo devolveu 22 das 30 entradas,
//     omitindo agents/ e mcp/ — ele parafraseia em vez de ler;
//   - como role "tool" com texto puro: 30 das 30, na ordem exata, e com 54
//     tokens de prompt a menos.
func TestResultadoVoltaComoMensagemDeFerramenta(t *testing.T) {
	m := novoModelDeTeste(t)
	m.conversationHistory = []agents.Message{
		{Role: "system", Content: "sys"},
		{Role: "user", Content: "o que tem nesta pasta?"},
		{Role: "assistant", Content: "", ToolCalls: []agents.ToolCall{{
			Function: agents.ToolCallFunction{
				Name:      "list_dir",
				Arguments: map[string]interface{}{"path": "."},
			},
		}}},
	}

	m.registraResultadosDeFerramenta([]mcp.Result{
		{Tool: "list_dir", Path: ".", Status: "success", Content: "📁 .\n - main.go\n - agents/"},
	})

	ultima := m.conversationHistory[len(m.conversationHistory)-1]
	if ultima.Role != "tool" {
		t.Fatalf("resultado entrou como role %q, esperava \"tool\"", ultima.Role)
	}
	if ultima.ToolName != "list_dir" {
		t.Errorf("tool_name é %q, esperava list_dir", ultima.ToolName)
	}
	if !strings.Contains(ultima.Content, "agents/") {
		t.Errorf("conteúdo do resultado não chegou: %q", ultima.Content)
	}
	// Texto puro: sem cerca de markdown e sem JSON escapado.
	if strings.Contains(ultima.Content, "```") || strings.Contains(ultima.Content, `\n`) {
		t.Errorf("o resultado foi embrulhado em JSON/markdown: %q", ultima.Content)
	}
}

// Um erro de ferramenta também é informação: o modelo precisa dele para se
// corrigir (foi assim que ele aprendeu a chamar set_workspace).
func TestErroDeFerramentaChegaAoModelo(t *testing.T) {
	m := novoModelDeTeste(t)
	m.conversationHistory = []agents.Message{{Role: "system", Content: "sys"}}

	m.registraResultadosDeFerramenta([]mcp.Result{
		{Tool: "list_dir", Path: "/fora", Status: "error", Error: "está fora da pasta de trabalho"},
	})

	ultima := m.conversationHistory[len(m.conversationHistory)-1]
	if ultima.Role != "tool" {
		t.Fatalf("erro entrou como role %q", ultima.Role)
	}
	if !strings.Contains(ultima.Content, "fora da pasta de trabalho") {
		t.Errorf("o motivo do erro não chegou ao modelo: %q", ultima.Content)
	}
}

// Uma resposta só com tool_calls tem content vazio: isso é trabalho normal, não
// "o modelo terminou sem produzir resposta". Tratar como vazio disparava um
// pedido de repetição e jogava fora a chamada.
//
// toolRuns no limite evita que o teste dispare o loop de ferramentas de
// verdade: aqui interessa só como a resposta vazia é classificada.
func TestRespostaSoComToolCallsNaoContaComoVazia(t *testing.T) {
	m := novoModelDeTeste(t)
	m.conversationHistory = []agents.Message{{Role: "system", Content: "sys"}}
	m.toolRuns = 15
	m.lastToolCalls = []agents.ToolCall{{
		Function: agents.ToolCallFunction{
			Name:      "list_dir",
			Arguments: map[string]interface{}{"path": "."},
		},
	}}

	m.handleAIResponse("")

	for _, msg := range m.messages {
		if strings.Contains(msg, "terminou sem produzir resposta") {
			t.Error("resposta só com tool_calls foi tratada como vazia")
		}
		if strings.Contains(msg, "pensou mas não escreveu") {
			t.Error("resposta só com tool_calls disparou pedido de repetição")
		}
	}
}

// A chamada nativa precisa ficar registrada no histórico como tool_calls, não
// como texto: é o que faz o template de chat abrir um turno de ferramenta.
func TestChamadaNativaEntraNoHistoricoComoToolCalls(t *testing.T) {
	m := novoModelDeTeste(t)
	m.conversationHistory = []agents.Message{{Role: "system", Content: "sys"}}
	m.toolRuns = 15
	m.lastToolCalls = []agents.ToolCall{{
		Function: agents.ToolCallFunction{
			Name:      "list_dir",
			Arguments: map[string]interface{}{"path": "."},
		},
	}}

	m.handleAIResponse("")

	var assistente *agents.Message
	for i := range m.conversationHistory {
		if m.conversationHistory[i].Role == "assistant" {
			assistente = &m.conversationHistory[i]
		}
	}
	if assistente == nil {
		t.Fatal("nenhuma mensagem do assistente no histórico")
	}
	if len(assistente.ToolCalls) != 1 {
		t.Fatalf("a chamada não virou tool_calls: %+v", *assistente)
	}
	if assistente.ToolCalls[0].Function.Name != "list_dir" {
		t.Errorf("chamada errada: %+v", assistente.ToolCalls[0])
	}
	// lastToolCalls e um buffer de um turno: deixa-lo cheio repetiria a chamada
	// na resposta seguinte.
	if len(m.lastToolCalls) != 0 {
		t.Error("lastToolCalls não foi consumido, a chamada vai repetir no próximo turno")
	}
}

// O orçamento de contexto tem de contar os schemas das ferramentas. São ~850
// tokens (10% do num_ctx desta máquina) que iam para o prompt sem serem
// descontados, e o corte do histórico acontecia tarde demais.
func TestOrcamentoDeContextoContaOsSchemas(t *testing.T) {
	custo := estimateTokens(textoDosSchemas())
	if custo < 400 {
		t.Errorf("os schemas custam %d tokens estimados, valor implausível", custo)
	}

	m := novoModelDeTeste(t)
	m.conversationHistory = []agents.Message{{Role: "system", Content: "sys"}}
	// Muitas mensagens de tamanho realista, bem mais do que cabe na janela.
	media := strings.Repeat("linha de código qualquer\n", 20)
	for i := 0; i < 200; i++ {
		m.conversationHistory = append(m.conversationHistory,
			agents.Message{Role: "user", Content: media})
	}

	enviado := m.localHistory()
	var total int
	for _, msg := range enviado {
		total += estimateTokens(msg.Content)
	}
	limite := agents.DefaultOllamaOptions().NumCtx - custo
	if total > limite {
		t.Errorf("histórico enviado tem %d tokens, acima de num_ctx-schemas (%d)", total, limite)
	}
	if len(enviado) < 2 {
		t.Error("o corte levou a conversa inteira")
	}
}

// Um único resultado grande não pode estourar a janela. app/model.go tem 3280
// linhas (~30 mil tokens contra os 8192 do num_ctx): mandá-lo inteiro fazia o
// Ollama descartar o cache de prefixo e reprocessar tudo — minutos nesta CPU.
func TestResultadoGrandeEhTruncadoComInstrucao(t *testing.T) {
	var sb strings.Builder
	for i := 0; i < 4000; i++ {
		sb.WriteString("essa é a linha de código número ")
		sb.WriteString(strings.Repeat("x", 20))
		sb.WriteString("\n")
	}
	original := sb.String()

	msgs := mensagensDeFerramenta([]mcp.Result{
		{Tool: "read_file", Path: "app/model.go", Status: "success", Content: original},
	})
	if len(msgs) != 1 {
		t.Fatalf("esperava 1 mensagem, obtive %d", len(msgs))
	}

	got := msgs[0].Content
	limite := agents.DefaultOllamaOptions().NumCtx / 4
	if estimateTokens(got) > limite {
		t.Errorf("resultado ficou com %d tokens, acima do orçamento de %d",
			estimateTokens(got), limite)
	}
	if !strings.Contains(got, "linhas omitidas") {
		t.Error("o modelo não foi avisado de que o resultado está incompleto")
	}
	if !strings.Contains(got, "search_files") {
		t.Error("o aviso não diz como buscar o resto")
	}
	// Começo e fim preservados: é o que importa num arquivo de código.
	if !strings.HasPrefix(got, "essa é a linha") {
		t.Error("o começo do arquivo foi perdido")
	}
}

// Resultados pequenos passam intactos: truncar o que já cabe só tiraria
// informação do modelo.
func TestResultadoPequenoPassaIntacto(t *testing.T) {
	conteudo := "📁 Conteúdo de /projeto\n - main.go\n - go.mod"
	msgs := mensagensDeFerramenta([]mcp.Result{
		{Tool: "list_dir", Path: ".", Status: "success", Content: conteudo},
	})
	if msgs[0].Content != conteudo {
		t.Errorf("resultado pequeno foi alterado:\n%q", msgs[0].Content)
	}
}

// Com a chamada fora do texto da resposta, o renderMarkdown não a transforma
// mais na linha "listou .". Sem exibi-la você vê o resultado da ação sem ver
// qual ação foi pedida.
func TestChamadaNativaApareceNaTela(t *testing.T) {
	m := novoModelDeTeste(t)

	m.mostraChamadasNativas([]agents.ToolCall{{
		Function: agents.ToolCallFunction{
			Name:      "list_dir",
			Arguments: map[string]interface{}{"path": "/projeto"},
		},
	}})

	tela := strings.Join(m.messages, "\n")
	if !strings.Contains(tela, "listou") || !strings.Contains(tela, "/projeto") {
		t.Errorf("a chamada não apareceu na tela:\n%s", tela)
	}
}

// A saída que acompanha um erro também precisa ser truncada: o caso comum é o
// log de um build que falhou, que pode ser maior que a janela inteira.
func TestSaidaDeErroGrandeTambemEhTruncada(t *testing.T) {
	log := strings.Repeat("./main.go:12:5: undefined: alguma coisa muito longa\n", 3000)

	msgs := mensagensDeFerramenta([]mcp.Result{
		{Tool: "execute_command", Status: "error", Error: "Comando falhou: exit status 1", Content: log},
	})

	got := msgs[0].Content
	limite := agents.DefaultOllamaOptions().NumCtx / 4
	if estimateTokens(got) > limite {
		t.Errorf("erro ficou com %d tokens, acima do orçamento de %d", estimateTokens(got), limite)
	}
	if !strings.Contains(got, "ERRO: Comando falhou") {
		t.Errorf("o motivo do erro foi cortado fora: %q", primeirosDoTeste(got, 120))
	}
}

func primeirosDoTeste(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n]
}

// A intenção da chamada já é exibida por mostraChamadasNativas (canal nativo)
// ou por substituiChamadas (JSON em texto). aplicaAberturaDePasta imprimia
// "abriu a pasta X" de novo, e a linha aparecia duplicada na tela.
func TestAberturaDePastaNaoDuplicaOEvento(t *testing.T) {
	m := novoModelDeTeste(t)
	dir := t.TempDir()
	fsm, err := filesystem.NewManager(dir)
	if err != nil {
		t.Fatal(err)
	}
	m.fsManager = fsm
	m.workspaceRoot = fsm.Root()
	m.mcpServer = mcp.NewServer(fsm)

	// O que o loop de verdade faz: exibe a intenção, depois aplica a abertura.
	m.mostraChamadasNativas([]agents.ToolCall{{
		Function: agents.ToolCallFunction{
			Name:      "set_workspace",
			Arguments: map[string]interface{}{"path": dir},
		},
	}})
	restantes, resultados := m.aplicaAberturaDePasta(
		[]mcp.Request{{Tool: "set_workspace", Path: dir}})

	if len(restantes) != 0 {
		t.Errorf("set_workspace deveria ser consumida pelo app, sobrou %d", len(restantes))
	}
	if len(resultados) != 1 || resultados[0].Status != "success" {
		t.Fatalf("abertura falhou: %+v", resultados)
	}

	var vezes int
	for _, msg := range m.messages {
		if strings.Contains(msg, "abriu a pasta") {
			vezes++
		}
	}
	if vezes != 1 {
		t.Errorf("a linha \"abriu a pasta\" apareceu %d vezes, esperava 1:\n%s",
			vezes, strings.Join(m.messages, "\n"))
	}
}

// O header mostrava m.toolRuns, que é o contador de CICLOS do turno e volta a
// zero ao fim de cada resposta — a barra dizia "0 ferramentas" logo depois de
// o agente ter rodado quatro.
func TestContadorDeFerramentasEhCumulativo(t *testing.T) {
	m := novoModelDeTeste(t)
	m.width, m.height = 140, 40
	m.toolCallsTotal = 4
	m.toolRuns = 0

	if !strings.Contains(m.View(), "4 ferramentas") {
		t.Errorf("o header não mostra o total acumulado:\n%s", m.View())
	}
}

// Trocar de conversa zera o total: o contador é da conversa, não da sessão.
func TestContadorDeFerramentasZeraNaNovaConversa(t *testing.T) {
	m := novoModelDeTeste(t)
	m.conversationHistory = []agents.Message{{Role: "system", Content: "sys"}}
	m.toolCallsTotal = 7

	m.resetConversation()

	if m.toolCallsTotal != 0 {
		t.Errorf("total de ferramentas não zerou: %d", m.toolCallsTotal)
	}
}

// Medido com lfm2.5:8b no meio de uma tarefa longa: o Ollama devolve UMA linha
// com done_reason "stop", eval_count 678 e content vazio — sem campo thinking,
// sem tool_calls, sem nada. A retentativa exigia thinking não-vazio, então não
// acontecia justamente no caso pior, e a tarefa morria ali.
func TestGastarTokensSemEntregarNadaPedeDeNovo(t *testing.T) {
	m := novoModelDeTeste(t)
	m.conversationHistory = []agents.Message{{Role: "system", Content: "sys"}}
	m.lastThinking = "" // o Ollama não nos deu raciocínio algum
	m.lastToolCalls = nil
	m.lastOllamaStats = agents.OllamaStats{GenTokens: 678}

	cmd := m.handleAIResponse("")

	if cmd == nil {
		t.Fatal("nenhuma segunda tentativa: a tarefa morre aqui")
	}
	if m.emptyRetries != 1 {
		t.Errorf("emptyRetries = %d, esperava 1", m.emptyRetries)
	}
	tela := strings.Join(m.messages, "\n")
	if !strings.Contains(tela, "sem escrever nada") {
		t.Errorf("o usuário não foi avisado da retentativa:\n%s", tela)
	}
	if strings.Contains(tela, "terminou sem produzir resposta") {
		t.Error("desistiu na primeira tentativa")
	}
}

// Sem tokens gerados e sem raciocínio não há o que repetir: é outro problema
// (conexão, modelo descarregado) e insistir só esconde a causa.
func TestSemTokenGeradoNaoPedeDeNovo(t *testing.T) {
	m := novoModelDeTeste(t)
	m.conversationHistory = []agents.Message{{Role: "system", Content: "sys"}}
	m.lastOllamaStats = agents.OllamaStats{GenTokens: 0}

	if cmd := m.handleAIResponse(""); cmd != nil {
		t.Error("não havia nada a repetir")
	}
}

// A segunda tentativa é única: insistir em loop com um modelo que não entrega
// gasta minutos de CPU sem sair do lugar.
func TestRetentativaNaoEntraEmLoop(t *testing.T) {
	m := novoModelDeTeste(t)
	m.conversationHistory = []agents.Message{{Role: "system", Content: "sys"}}
	m.lastOllamaStats = agents.OllamaStats{GenTokens: 678}
	m.emptyRetries = 1 // já tentou

	if cmd := m.handleAIResponse(""); cmd != nil {
		t.Error("tentou uma segunda vez")
	}
	if !strings.Contains(strings.Join(m.messages, "\n"), "terminou sem produzir resposta") {
		t.Error("o usuário não foi informado de que o modelo desistiu")
	}
}

// A linha "🔁 Executando ferramentas (N/15 ciclos)..." é um placeholder e tem
// de sair quando a resposta chega. A comparação era sensível a caixa e nunca
// casava, então as linhas se acumulavam na tela a cada ciclo.
func TestPlaceholderDeFerramentasEhRemovido(t *testing.T) {
	m := novoModelDeTeste(t)
	m.addMessage("resposta de verdade")
	m.addMessage(m.theme.InfoStyle.Render("🔁 Executando ferramentas (4/15 ciclos)..."))

	m.removePlaceholders()

	tela := strings.Join(m.messages, "\n")
	if strings.Contains(tela, "Executando ferramentas") {
		t.Errorf("o placeholder ficou na tela:\n%s", tela)
	}
	if !strings.Contains(tela, "resposta de verdade") {
		t.Error("a remoção comeu uma mensagem real")
	}
}

// Medido com lfm2.5:8b num pedido amplo: chamou "list_dir docs" no ciclo 3 e de
// novo no ciclo 6, com o mesmo resultado, e depois desistiu sem escrever nada.
// Cada ciclo custa 20–60s nesta CPU, e um resultado idêntico não traz
// informação nova — o modelo precisa ser avisado de que está girando.
func TestChamadaRepetidaAvisaOModelo(t *testing.T) {
	m := novoModelDeTeste(t)
	m.conversationHistory = []agents.Message{{Role: "system", Content: "sys"}}

	resultado := mcp.Result{Tool: "list_dir", Path: "docs", Status: "success",
		Content: "📁 docs\n - NOTAS.md"}

	m.registraResultadosDeFerramenta([]mcp.Result{resultado})
	primeira := m.conversationHistory[len(m.conversationHistory)-1].Content
	if strings.Contains(primeira, "já fez esta chamada") {
		t.Error("a primeira chamada foi marcada como repetida")
	}

	m.registraResultadosDeFerramenta([]mcp.Result{resultado})
	segunda := m.conversationHistory[len(m.conversationHistory)-1].Content
	if !strings.Contains(segunda, "já fez esta chamada") {
		t.Errorf("a repetição não foi sinalizada ao modelo:\n%q", segunda)
	}
	if !strings.Contains(segunda, "NOTAS.md") {
		t.Error("o aviso comeu o conteúdo do resultado")
	}
}

// Mesma ferramenta e mesmo caminho, resultado DIFERENTE, não é repetição: o
// arquivo mudou e essa é informação nova.
func TestResultadoDiferenteNaoEhRepeticao(t *testing.T) {
	m := novoModelDeTeste(t)
	m.conversationHistory = []agents.Message{{Role: "system", Content: "sys"}}

	m.registraResultadosDeFerramenta([]mcp.Result{
		{Tool: "list_dir", Path: "docs", Status: "success", Content: "📁 docs\n(vazio)"}})
	m.registraResultadosDeFerramenta([]mcp.Result{
		{Tool: "list_dir", Path: "docs", Status: "success", Content: "📁 docs\n - NOTAS.md"}})

	ultima := m.conversationHistory[len(m.conversationHistory)-1].Content
	if strings.Contains(ultima, "já fez esta chamada") {
		t.Errorf("resultado diferente foi tratado como repetição:\n%q", ultima)
	}
}

// Um erro repetido também é giro: o modelo insistindo no mesmo caminho errado.
func TestErroRepetidoTambemAvisa(t *testing.T) {
	m := novoModelDeTeste(t)
	m.conversationHistory = []agents.Message{{Role: "system", Content: "sys"}}

	erro := mcp.Result{Tool: "read_file", Path: "docs/*.md", Status: "error",
		Error: "read_file não aceita curinga"}
	m.registraResultadosDeFerramenta([]mcp.Result{erro})
	m.registraResultadosDeFerramenta([]mcp.Result{erro})

	ultima := m.conversationHistory[len(m.conversationHistory)-1].Content
	if !strings.Contains(ultima, "já fez esta chamada") {
		t.Errorf("erro repetido não foi sinalizado:\n%q", ultima)
	}
	if !strings.Contains(ultima, "curinga") {
		t.Error("o aviso comeu o motivo do erro")
	}
}

// "./docs" e "./docs/" são o mesmo diretório com texto diferente. Medido com
// lfm2.5:8b: ele chamou list_dir nas duas formas em ciclos separados e a
// repetição passava batida, porque a assinatura era o texto cru do caminho.
func TestRepeticaoDetectadaComCaminhoEquivalente(t *testing.T) {
	m := novoModelDeTeste(t)
	m.conversationHistory = []agents.Message{{Role: "system", Content: "sys"}}

	conteudo := "📁 docs\n(vazio)"
	m.registraResultadosDeFerramenta([]mcp.Result{
		{Tool: "list_dir", Path: "./docs", Status: "success", Content: conteudo}})
	m.registraResultadosDeFerramenta([]mcp.Result{
		{Tool: "list_dir", Path: "./docs/", Status: "success", Content: conteudo}})

	ultima := m.conversationHistory[len(m.conversationHistory)-1].Content
	if !strings.Contains(ultima, "já fez esta chamada") {
		t.Errorf("./docs e ./docs/ não foram reconhecidos como a mesma chamada:\n%q", ultima)
	}
}

// A TUI mostrou "pensou 116m24s" logo acima de "1m26s" para a MESMA resposta:
// o raciocínio media o relógio da UI e a resposta media as métricas do Ollama.
// Os dois números têm de vir da mesma fonte, e a fonte autoritativa é o
// servidor — imune a qualquer atraso do laço de eventos.
func TestTempoDoRaciocinioVemDasMetricasDoOllama(t *testing.T) {
	m := novoModelDeTeste(t)
	m.conversationHistory = []agents.Message{{Role: "system", Content: "sys"}}
	m.toolRuns = 15 // não dispara o laço de ferramentas
	// Relógio da UI muito atrás: é o cenário que produziu os 116 minutos.
	m.streamStart = time.Now().Add(-2 * time.Hour)
	m.streamThinking = "raciocínio"

	m.Update(ollamaDoneMsg{
		full:  "resposta",
		stats: agents.OllamaStats{GenTokens: 100, GenTime: time.Minute, Total: 86 * time.Second},
	})

	if m.lastThinkingTime != 86*time.Second {
		t.Errorf("tempo do raciocínio = %v, esperava 86s (o total do Ollama)", m.lastThinkingTime)
	}
}

// Sem métricas do Ollama o relógio da UI ainda é melhor que nada.
func TestSemMetricasUsaORelogioComoReserva(t *testing.T) {
	m := novoModelDeTeste(t)
	m.conversationHistory = []agents.Message{{Role: "system", Content: "sys"}}
	m.toolRuns = 15
	m.streamStart = time.Now().Add(-5 * time.Second)

	m.Update(ollamaDoneMsg{full: "resposta", stats: agents.OllamaStats{}})

	if m.lastThinkingTime < 4*time.Second || m.lastThinkingTime > 30*time.Second {
		t.Errorf("reserva do relógio fora do esperado: %v", m.lastThinkingTime)
	}
}
