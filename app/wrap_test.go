package app

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"amazzo/agents"
	"amazzo/tasks"
	"amazzo/themes"

	"github.com/charmbracelet/bubbles/textarea"
	"github.com/charmbracelet/bubbles/viewport"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/charmbracelet/x/ansi"
)

// Reproduz o artefato ";252m" visto no terminal: wrapText fatiava a
// sequencia de escape \x1b[38;5;252m no meio.
func TestWrapTextNaoQuebraSequenciaANSI(t *testing.T) {
	// Escape no meio de uma palavra longa: o corte por bytes cai dentro
	// da sequencia e vaza ";252m" na tela.
	styled := strings.Repeat("a", 35) + "\x1b[38;5;252m" + strings.Repeat("b", 200) + "\x1b[0m"
	got := wrapText(styled, 40)

	if strings.Contains(got, ";252m") && !strings.Contains(got, "\x1b[38;5;252m") {
		t.Fatalf("sequencia ANSI foi cortada, sobrou lixo visivel: %q", got)
	}
	for _, line := range strings.Split(got, "\n") {
		if w := ansi.StringWidth(line); w > 40 {
			t.Errorf("linha com largura %d > 40: %q", w, line)
		}
	}
}

// Emojis e acentos ocupam varios bytes: wrapText usava len() em bytes,
// entao quebrava linhas muito antes do necessario.
func TestWrapTextUsaLarguraVisualNaoBytes(t *testing.T) {
	// 33 celulas de largura visual, mas 55 bytes por causa de emoji/acento.
	line := "📋 Tarefas concluídas ✅ e prontas 🚀"
	got := wrapText(line, 40)

	if strings.Contains(got, "\n") {
		t.Fatalf("linha de %d celulas foi quebrada em limite 40: %q",
			ansi.StringWidth(line), got)
	}
}

// A caixa de tarefas era destruida porque strings.Fields() descartava
// a indentacao e juntava as linhas.
func TestWrapTextPreservaCaixaDeTarefas(t *testing.T) {
	box := "┌─────────────────────────────────────────\n" +
		"│ ⏳ Criar estrutura do projeto\n" +
		"│ ⏳ Inicializar um repositório Git\n" +
		"└─────────────────────────────────────────"
	got := wrapText(box, 120)

	if got != box {
		t.Fatalf("caixa alterada apesar de caber em 120 colunas:\nquero:\n%s\nobtive:\n%s", box, got)
	}
}

// A caixa de tarefas era injetada no markdown ANTES do Glamour, que a
// reflowava como paragrafo e juntava as linhas.
func TestRenderMarkdownPreservaBlocoTodo(t *testing.T) {
	m := newTestModel(t)
	out := m.renderMarkdown("Vou fazer isso:\n\n```todo\n[ ] Criar estrutura do projeto\n[ ] Inicializar um repositório Git\n```\n\nPronto.")

	plain := ansi.Strip(out)

	// Cada tarefa precisa ficar na própria linha: o Glamour as fundia num
	// parágrafo só quando o bloco passava por ele.
	var tarefas []string
	for _, line := range strings.Split(plain, "\n") {
		t := strings.TrimSpace(line)
		if strings.HasPrefix(t, "·") || strings.HasPrefix(t, "✓") || strings.HasPrefix(t, "▶") {
			tarefas = append(tarefas, t)
		}
	}

	if len(tarefas) != 2 {
		t.Errorf("esperava 2 tarefas em linhas separadas, achei %d:\n%s", len(tarefas), plain)
	}
	for _, linha := range tarefas {
		if strings.Count(linha, "·") > 1 {
			t.Errorf("tarefas fundidas na mesma linha: %q", linha)
		}
	}
	if !strings.Contains(plain, "plano · 0 de 2") {
		t.Errorf("faltou o resumo do plano:\n%s", plain)
	}
}

// Blocos ```tool viram uma linha resumida e nao podem ser reflowados.
func TestRenderMarkdownPreservaBlocoTool(t *testing.T) {
	m := newTestModel(t)
	out := m.renderMarkdown("Lendo:\n\n```tool\n{\"tool\":\"read_file\",\"path\":\"/tmp/a/b/main.go\"}\n```\n")

	plain := ansi.Strip(out)
	if !strings.Contains(plain, "leu main.go") {
		t.Errorf("resumo da ferramenta perdido:\n%s", plain)
	}
}

func newTestModel(t *testing.T) *Model {
	t.Helper()
	m := &Model{}
	m.setGlamourWidth(100)
	if m.glamourRenderer == nil {
		t.Fatal("renderer nao inicializado")
	}
	return m
}

// localHistory corta o historico pelo orcamento de contexto, mas nunca pode
// descartar a mensagem que o usuario acabou de enviar.
func TestLocalHistoryPreservaUltimaMensagem(t *testing.T) {
	gigante := strings.Repeat("x", 200_000)

	m := &Model{}
	m.conversationHistory = []agents.Message{
		{Role: "system", Content: "sys"},
		{Role: "user", Content: gigante},
	}

	got := m.localHistory()
	if len(got) != 2 {
		t.Fatalf("esperava system + mensagem do usuário, obtive %d mensagens", len(got))
	}
	if got[1].Content != gigante {
		t.Error("a mensagem do usuário foi descartada")
	}
}

// O historico antigo deve ser cortado quando nao cabe no contexto.
func TestLocalHistoryCortaHistoricoAntigo(t *testing.T) {
	grande := strings.Repeat("y", 40_000) // ~10k tokens, acima do orçamento

	m := &Model{}
	m.conversationHistory = []agents.Message{{Role: "system", Content: "sys"}}
	for i := 0; i < 6; i++ {
		m.conversationHistory = append(m.conversationHistory,
			agents.Message{Role: "user", Content: grande})
	}

	got := m.localHistory()
	if len(got) >= len(m.conversationHistory) {
		t.Fatalf("histórico não foi cortado: %d de %d", len(got), len(m.conversationHistory))
	}
	if got[0].Role != "system" {
		t.Error("o system prompt precisa continuar em primeiro para o cache de prefixo funcionar")
	}
}

// Historico so com o system prompt nao pode quebrar o corte por orçamento.
func TestLocalHistorySoComSystemPrompt(t *testing.T) {
	m := &Model{}
	m.conversationHistory = []agents.Message{{Role: "system", Content: "sys"}}

	got := m.localHistory()
	if len(got) != 1 || got[0].Role != "system" {
		t.Fatalf("esperava apenas o system prompt, obtive %#v", got)
	}
}

// O agente entrava em loop repetindo a mesma resposta dezenas de vezes: o
// guard so comparava com a resposta imediatamente anterior e exigia igualdade
// exata, entao qualquer variacao de espaco em branco o desarmava.
func TestDeteccaoDeLoopIgnoraEspacos(t *testing.T) {
	m := &Model{}

	if m.respostaRepetida("Ok! Entendido.") {
		t.Fatal("a primeira resposta não pode contar como repetição")
	}
	if !m.respostaRepetida("Ok!   Entendido.\n\n") {
		t.Error("a mesma resposta com espaçamento diferente deveria ser detectada")
	}
}

// Repeticoes alternadas (A, B, A, B) tambem sao loop, e escapavam do guard
// antigo porque a resposta anterior nunca era igual a atual.
func TestDeteccaoDeLoopAlternado(t *testing.T) {
	m := &Model{}

	m.respostaRepetida("resposta A")
	m.respostaRepetida("resposta B")

	if !m.respostaRepetida("resposta A") {
		t.Error("alternância A/B/A deveria ser detectada como loop")
	}
}

// Progresso real nao pode ser confundido com loop.
func TestDeteccaoDeLoopNaoBloqueiaProgresso(t *testing.T) {
	m := &Model{}

	for _, r := range []string{"passo 1", "passo 2", "passo 3", "passo 4"} {
		if m.respostaRepetida(r) {
			t.Errorf("resposta nova %q marcada como repetida", r)
		}
	}
}

// Modelos com "thinking" (lfm2.5, deepseek-r1) mandam o raciocínio num campo
// separado e deixam content vazio até terminarem — a resposta chegava vazia.
func TestPreviaMostraRaciocinioAntesDoConteudo(t *testing.T) {
	m := &Model{}
	m.viewport.Width = 80
	m.streamThinking = "O usuário quer ler um arquivo, então devo usar read_file."

	previa := m.streamingPreview()
	if !strings.Contains(previa, "read_file") {
		t.Errorf("o raciocínio deveria aparecer na prévia, obtive: %q", previa)
	}

	// Assim que o conteúdo real começa, ele substitui o raciocínio.
	m.streamPartial = "Vou ler o arquivo."
	previa = m.streamingPreview()
	if strings.Contains(previa, "devo usar") {
		t.Errorf("o raciocínio não deveria aparecer junto do conteúdo: %q", previa)
	}
	if !strings.Contains(previa, "Vou ler o arquivo.") {
		t.Errorf("conteúdo ausente da prévia: %q", previa)
	}
}

// Modelos com raciocínio podem gastar todo o num_predict pensando e devolver
// content vazio. Renderizar uma caixa vazia não diz nada ao usuário.
func TestRespostaVaziaExplicaOMotivo(t *testing.T) {
	teto := agents.DefaultOllamaOptions().NumPredict
	if teto <= 0 {
		t.Skip("sem teto de geração configurado: nada a orientar")
	}

	m := novoModelDeTeste(t)
	m.lastOllamaStats = agents.OllamaStats{GenTokens: teto}
	m.emptyRetries = 1 // já tentou de novo; agora é o diagnóstico que interessa
	m.handleAIResponse("   \n  ")

	juntas := strings.Join(m.messages, "\n")
	if !strings.Contains(juntas, "AMAZZO_OLLAMA_NUM_PREDICT") {
		t.Errorf("com o teto estourado, esperava orientação sobre num_predict:\n%s", juntas)
	}
}

// Sem teto de geração (o padrão local), nunca sugerir aumentá-lo.
func TestSemTetoNaoSugereAumentarLimite(t *testing.T) {
	if agents.DefaultOllamaOptions().NumPredict > 0 {
		t.Skip("há um teto configurado neste ambiente")
	}

	m := novoModelDeTeste(t)
	m.lastOllamaStats = agents.OllamaStats{GenTokens: 5000}
	m.emptyRetries = 1 // já tentou de novo; agora é o diagnóstico que interessa
	m.handleAIResponse("")

	if !strings.Contains(strings.Join(m.messages, "\n"), "sem produzir resposta") {
		t.Fatalf("o teste precisa alcançar o diagnóstico:\n%s", strings.Join(m.messages, "\n"))
	}
	if strings.Contains(strings.Join(m.messages, "\n"), "NUM_PREDICT") {
		t.Errorf("não existe teto para aumentar:\n%s", strings.Join(m.messages, "\n"))
	}
}

// O modelo pode parar sozinho, sem chegar perto do teto. Culpar num_predict aí
// manda o usuário para o ajuste errado.
func TestRespostaVaziaSemEstourarTetoNaoCulpaNumPredict(t *testing.T) {
	m := novoModelDeTeste(t)
	m.lastOllamaStats = agents.OllamaStats{GenTokens: 11}
	m.emptyRetries = 1 // já tentou de novo; agora é o diagnóstico que interessa
	m.handleAIResponse("")

	juntas := strings.Join(m.messages, "\n")
	if strings.Contains(juntas, "AMAZZO_OLLAMA_NUM_PREDICT") {
		t.Errorf("11 tokens não estouram o teto; a dica sobre num_predict é enganosa:\n%s", juntas)
	}
	if !strings.Contains(juntas, "sem produzir resposta") {
		t.Errorf("o usuário precisa saber que não veio resposta:\n%s", juntas)
	}
}

func novoModelDeTeste(t *testing.T) *Model {
	t.Helper()
	m := &Model{}
	m.setGlamourWidth(100)
	m.theme = themes.Available["default"]
	m.viewport.Width = 100
	m.metrics = NewMetrics()
	m.taskTracker = tasks.NewTracker()
	m.textarea = textarea.New()
	m.textarea.ShowLineNumbers = false
	m.textarea.Focus()
	m.textarea.SetWidth(100)
	m.textarea.SetHeight(1)
	m.viewport = viewport.New(100, 20)
	m.modernStyles = NewModernStyles(true)
	m.ultimateStyles = NewUltimateStyles()
	return m
}

// Quem só usa Ollama não tem token do Hugging Face. Escolher um modelo remoto
// nesse estado precisa dar uma mensagem acionável, não um erro de API.
func TestModeloRemotoSemTokenExplicaOQueFazer(t *testing.T) {
	m := novoModelDeTeste(t)
	m.hfToken = ""

	// Primeiro modelo remoto da lista.
	for i, cfg := range availableModels {
		if !isLocalModel(cfg) {
			m.currentModel = i
			break
		}
	}

	msg, ok := m.callAPI("oi")().(apiResponseMsg)
	if !ok || msg.err == nil {
		t.Fatalf("esperava um erro explicativo, obtive %#v", msg)
	}
	if !strings.Contains(msg.err.Error(), "/key") || !strings.Contains(msg.err.Error(), "/modelo") {
		t.Errorf("o erro precisa dizer como resolver: %v", msg.err)
	}
}

// Modelo local não pode ser bloqueado pela falta de token.
func TestModeloLocalFuncionaSemToken(t *testing.T) {
	m := novoModelDeTeste(t)
	m.hfToken = ""
	m.currentModel = 0

	if !isLocalModel(availableModels[0]) {
		t.Skip("nenhum modelo local na lista neste ambiente")
	}
	if m.callAPI("oi") == nil {
		t.Error("modelo local deveria seguir mesmo sem token")
	}
	if m.streamCh == nil {
		t.Error("esperava streaming iniciado para modelo local")
	}
}

// Guarda de código-fonte: nada neste pacote pode chamar
// glamour.WithAutoStyle().
//
// WithAutoStyle chama termenv.HasDarkBackground(), que escreve uma consulta
// OSC 11 no terminal e espera a resposta pelo stdin. Dentro do Update quem lê
// o stdin é o Bubble Tea: ele consome a resposta, o termenv espera para
// sempre e a TUI congela no splash.
//
// A verificação é textual de propósito. Um teste em processo não pega isto:
// o termenv checa isatty antes de consultar, então com stdin redirecionado
// para um pipe ele nem tenta, e o teste passa mesmo com o código quebrado.
// Reproduzir de verdade exigiria um PTY que responde ao OSC 11.
func TestNenhumWithAutoStyleNoPacote(t *testing.T) {
	arquivos, err := filepath.Glob("*.go")
	if err != nil {
		t.Fatal(err)
	}

	for _, arquivo := range arquivos {
		if strings.HasSuffix(arquivo, "_test.go") {
			continue
		}
		conteudo, err := os.ReadFile(arquivo)
		if err != nil {
			t.Fatal(err)
		}
		for i, linha := range strings.Split(string(conteudo), "\n") {
			if strings.HasPrefix(strings.TrimSpace(linha), "//") {
				continue // comentários explicando o porquê são bem-vindos
			}
			if strings.Contains(linha, "WithAutoStyle") {
				t.Errorf("%s:%d chama WithAutoStyle — isso congela a TUI. "+
					"Use WithStandardStyle(m.glamourStyleName()).", arquivo, i+1)
			}
		}
	}
}

// O estilo do markdown vem do tema, não de uma pergunta ao terminal.
func TestEstiloDoMarkdownVemDoTema(t *testing.T) {
	casos := map[string]string{
		"light": "light", "solarized": "light",
		"default": "dark", "dracula": "dark", "": "dark",
	}
	for tema, esperado := range casos {
		m := &Model{currentTheme: tema}
		if got := m.glamourStyleName(); got != esperado {
			t.Errorf("tema %q: quero %q, obtive %q", tema, esperado, got)
		}
	}
}

// O splash não pode prender o usuário: qualquer tecla precisa encerrá-lo,
// mesmo que o ticker que o encerraria sozinho não rode.
func TestQualquerTeclaEncerraOSplash(t *testing.T) {
	m := novoModelDeTeste(t)
	m.showSplash = true
	m.splashStartTime = time.Now() // acabou de começar: o ticker não encerraria

	m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'x'}})

	if m.showSplash {
		t.Error("o splash deveria terminar ao apertar uma tecla")
	}
	if len(m.messages) == 0 {
		t.Error("as boas-vindas deveriam aparecer ao sair do splash")
	}
}

// O lfm2.5 às vezes encerra o turno depois de raciocinar, sem escrever nada.
// Nesse caso vale uma segunda tentativa: o cache de prefixo já está quente,
// então custa pouco, e costuma resolver.
func TestRespostaVaziaTentaDeNovoUmaVez(t *testing.T) {
	m := novoModelDeTeste(t)
	m.currentModel = 0
	m.conversationHistory = []agents.Message{{Role: "system", Content: "sys"}}
	m.lastThinking = "pensei bastante mas não escrevi nada"

	if cmd := m.handleAIResponse(""); cmd == nil {
		t.Fatal("com raciocínio presente, deveria tentar de novo")
	}
	if m.emptyRetries != 1 {
		t.Errorf("esperava 1 tentativa registrada, obtive %d", m.emptyRetries)
	}

	// A segunda vez desiste e explica, em vez de ficar tentando para sempre.
	m.messages = nil
	m.lastThinking = "de novo só raciocínio"
	if cmd := m.handleAIResponse(""); cmd != nil {
		t.Error("não pode tentar uma terceira vez")
	}
	if !strings.Contains(strings.Join(m.messages, "\n"), "sem produzir resposta") {
		t.Errorf("deveria explicar ao desistir:\n%s", strings.Join(m.messages, "\n"))
	}
}

// Sem raciocínio E sem token gerado, uma resposta vazia é outra coisa
// (conexão, modelo descarregado): insistir só esconderia a causa.
func TestRespostaVaziaSemRaciocinioNaoInsiste(t *testing.T) {
	m := novoModelDeTeste(t)
	m.lastThinking = ""
	m.lastOllamaStats = agents.OllamaStats{GenTokens: 0}

	if cmd := m.handleAIResponse(""); cmd != nil {
		t.Error("sem raciocínio não há motivo para tentar de novo")
	}
}

// Trocar de modelo começa uma conversa nova: os contadores da conversa
// anterior não podem continuar na tela, senão o custo exibido é de outro
// modelo.
func TestTrocarDeModeloZeraOsContadores(t *testing.T) {
	m := novoModelDeTeste(t)
	m.conversationHistory = []agents.Message{
		{Role: "system", Content: "sys"},
		{Role: "user", Content: "oi"},
	}
	m.inputTokens, m.outputTokens, m.totalTokens = 700, 1000, 1700
	m.toolRuns = 3
	m.lastOllamaStats = agents.OllamaStats{GenTokens: 500}
	m.thoughts = []thought{{index: 0, text: "algo"}}

	m.resetConversation()

	if m.totalTokens != 0 || m.inputTokens != 0 || m.outputTokens != 0 {
		t.Errorf("tokens não zeraram: entrada=%d saída=%d total=%d",
			m.inputTokens, m.outputTokens, m.totalTokens)
	}
	if m.toolRuns != 0 {
		t.Errorf("contador de ferramentas não zerou: %d", m.toolRuns)
	}
	if m.lastOllamaStats.GenTokens != 0 {
		t.Error("as métricas do modelo anterior continuaram na barra de status")
	}
	if len(m.thoughts) != 0 {
		t.Error("raciocínios da conversa anterior continuaram registrados")
	}
	if len(m.conversationHistory) != 1 {
		t.Errorf("o contexto deveria ficar só com o system prompt, tem %d",
			len(m.conversationHistory))
	}
}
