package app

import (
	"fmt"
	"strings"
	"testing"
	"time"

	"amazzo/agents"
	"amazzo/filesystem"
	"amazzo/mcp"
	"amazzo/permissions"

	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"

	"github.com/charmbracelet/x/ansi"
)

// O trilho substituiu as meias-caixas justamente porque elas desalinhavam.
// Toda linha do bloco precisa começar com o trilho e caber na largura.
func TestBlocoTemTrilhoEmTodasAsLinhas(t *testing.T) {
	corpo := "primeira linha\nsegunda linha um pouco mais longa para forçar a quebra do texto no limite"
	out := renderAssistantEntry("lfm2.5:8b", "17 tok/s · 14s", corpo, 60)

	for i, linha := range strings.Split(out, "\n") {
		limpa := ansi.Strip(linha)
		if !strings.HasPrefix(limpa, railGlyph) {
			t.Errorf("linha %d sem trilho: %q", i, limpa)
		}
		if w := ansi.StringWidth(limpa); w > 60 {
			t.Errorf("linha %d com largura %d > 60: %q", i, w, limpa)
		}
	}
}

// As métricas ficam à direita, sem colidir com o nome do modelo.
func TestCabecalhoAlinhaMetricasADireita(t *testing.T) {
	out := renderAssistantEntry("modelo", "9 tok/s", "corpo", 40)
	head := ansi.Strip(strings.Split(out, "\n")[0])

	if !strings.HasSuffix(head, "9 tok/s") {
		t.Errorf("métricas deveriam terminar a linha: %q", head)
	}
	if ansi.StringWidth(head) > 40 {
		t.Errorf("cabeçalho estourou a largura: %q", head)
	}
}

// Nome comprido demais não pode empurrar as métricas para fora nem quebrar.
func TestCabecalhoSuportaLarguraApertada(t *testing.T) {
	out := renderAssistantEntry(strings.Repeat("modelo-", 8), "123 tok/s", "x", 30)
	for _, linha := range strings.Split(out, "\n") {
		if w := ansi.StringWidth(ansi.Strip(linha)); w > 30 {
			t.Errorf("largura %d > 30: %q", w, ansi.Strip(linha))
		}
	}
}

// Colapsado, o raciocínio ocupa uma linha; aberto, mostra o texto inteiro.
func TestRaciocinioColapsaEExpande(t *testing.T) {
	texto := "O usuário quer saber o diretório.\nPreciso usar list_dir."

	fechado := renderThought(texto, 12*time.Second, false, 60)
	if strings.Count(fechado, "\n") != 0 {
		t.Errorf("fechado deveria ser uma linha só:\n%s", fechado)
	}
	if !strings.Contains(ansi.Strip(fechado), "pensou 12s") {
		t.Errorf("resumo deveria dizer quanto tempo pensou: %q", ansi.Strip(fechado))
	}
	if strings.Contains(ansi.Strip(fechado), "list_dir") {
		t.Error("fechado não pode mostrar o texto do raciocínio")
	}

	aberto := renderThought(texto, 12*time.Second, true, 60)
	if !strings.Contains(ansi.Strip(aberto), "list_dir") {
		t.Errorf("aberto deveria mostrar o raciocínio:\n%s", ansi.Strip(aberto))
	}
}

// A barra de status pode ficar sem espaço; o modelo ativo é o que não some,
// porque é ele que muda o comportamento de tudo.
func TestBarraDeStatusPreservaOModelo(t *testing.T) {
	out := renderStatusBar("lfm2.5:8b", "/home/daniel/projeto/muito/fundo", "17 tok/s · 1.2k tokens", 30)

	if w := ansi.StringWidth(ansi.Strip(out)); w > 30 {
		t.Errorf("barra estourou: largura %d, %q", w, ansi.Strip(out))
	}
	if !strings.Contains(ansi.Strip(out), "lfm2.5") {
		t.Errorf("o modelo ativo não pode sumir: %q", ansi.Strip(out))
	}
}

// Com espaço sobrando, os dois lados aparecem.
func TestBarraDeStatusMostraTudoQuandoCabe(t *testing.T) {
	out := ansi.Strip(renderStatusBar("modelo", "pasta", "métricas", 60))
	if !strings.Contains(out, "modelo") || !strings.Contains(out, "métricas") {
		t.Errorf("faltou informação: %q", out)
	}
	if ansi.StringWidth(out) > 60 {
		t.Errorf("barra estourou: %q", out)
	}
}

func TestRodapeNaoEstouraALargura(t *testing.T) {
	itens := []string{"enter enviar", "↑↓ histórico", "@ arquivos", "ctrl+r raciocínio", "ctrl+c sair"}
	for _, w := range []int{30, 60, 120} {
		out := ansi.Strip(renderFooter(itens, w))
		if got := ansi.StringWidth(out); got > w {
			t.Errorf("largura %d: rodapé com %d células: %q", w, got, out)
		}
	}
}

// Modelos pequenos erram a tag da cerca: escrevem ```json, ``` ou nada onde o
// prompt pedia ```tool. O que identifica uma chamada é o JSON ter a chave
// "tool", não o modelo ter acertado o rótulo.
func TestChamadaDeFerramentaAceitaQualquerTagDeCerca(t *testing.T) {
	json := `{"tool": "read_file", "path": "main.go"}`

	casos := map[string]string{
		"tag tool":   "```tool\n" + json + "\n```",
		"tag json":   "```json\n" + json + "\n```",
		"sem tag":    "```\n" + json + "\n```",
		"tag maiúsc": "```JSON\n" + json + "\n```",
		"multilinha": "```json\n{\n  \"tool\": \"read_file\",\n  \"path\": \"main.go\"\n}\n```",
	}

	m := &Model{}
	for nome, texto := range casos {
		reqs := m.parseToolRequests(texto)
		if len(reqs) != 1 {
			t.Errorf("%s: esperava 1 chamada, obtive %d", nome, len(reqs))
			continue
		}
		if reqs[0].Tool != "read_file" {
			t.Errorf("%s: ferramenta errada: %q", nome, reqs[0].Tool)
		}
	}
}

// Bloco de código comum não pode ser confundido com chamada de ferramenta.
func TestBlocoDeCodigoNaoViraChamada(t *testing.T) {
	m := &Model{}

	casos := []string{
		"```go\nfunc main() {}\n```",
		"```json\n{\"nome\": \"exemplo\", \"valor\": 1}\n```",
		"```\ntexto solto\n```",
		"```json\n{\"tool\": \"\"}\n```",
	}

	for _, texto := range casos {
		if reqs := m.parseToolRequests(texto); len(reqs) != 0 {
			t.Errorf("bloco comum virou chamada: %q → %+v", texto, reqs)
		}
	}
}

// O lfm2.5 emite a chamada como JSON puro, sem cerca nenhuma:
//
//	{"tool": "read_file", "path": "main.go"}
//
// Exigir ``` fazia a chamada ser ignorada e aparecer como texto na tela.
func TestChamadaSemCerca(t *testing.T) {
	m := &Model{}

	casos := map[string]string{
		"json puro":    `{"tool": "read_file", "path": "main.go"}`,
		"com texto":    "Vou ler o arquivo:\n{\"tool\": \"read_file\", \"path\": \"main.go\"}",
		"multilinha":   "{\n  \"tool\": \"read_file\",\n  \"path\": \"main.go\"\n}",
		"com espaços":  "   {\"tool\": \"read_file\", \"path\": \"main.go\"}   ",
		"depois de nl": "Ok.\n\n{\"tool\":\"read_file\",\"path\":\"main.go\"}\n\nPronto.",
	}

	for nome, texto := range casos {
		reqs := m.parseToolRequests(texto)
		if len(reqs) != 1 {
			t.Errorf("%s: esperava 1 chamada, obtive %d", nome, len(reqs))
			continue
		}
		if reqs[0].Tool != "read_file" || reqs[0].Path != "main.go" {
			t.Errorf("%s: chamada errada: %+v", nome, reqs[0])
		}
	}
}

// JSON solto que não é chamada de ferramenta continua sendo texto.
func TestJsonComumNaoViraChamada(t *testing.T) {
	m := &Model{}

	casos := []string{
		`{"nome": "exemplo", "valor": 1}`,
		"Use um objeto como {\"chave\": \"valor\"} no seu código.",
		`{"tool": ""}`,
		"não é json { isto aqui }",
	}

	for _, texto := range casos {
		if reqs := m.parseToolRequests(texto); len(reqs) != 0 {
			t.Errorf("texto comum virou chamada: %q → %+v", texto, reqs)
		}
	}
}

// Uma chamada sem cerca não pode aparecer como JSON cru na tela: o usuário
// quer ver a ação, não a implementação.
func TestChamadaSemCercaViraResumoNaTela(t *testing.T) {
	m := novoModelDeTeste(t)
	out := ansi.Strip(m.renderMarkdown(
		"Vou ler o arquivo.\n{\"tool\": \"read_file\", \"path\": \"main.go\"}"))

	if !strings.Contains(out, "leu main.go") {
		t.Errorf("esperava o resumo da ação:\n%s", out)
	}
	if strings.Contains(out, `"tool"`) {
		t.Errorf("o JSON não pode aparecer na tela:\n%s", out)
	}
}

// Para copiar a conversa inteira é preciso poder subir por ela. PgUp/PgDn
// rolam uma tela; Home/End vão ao começo e ao fim.
func TestTeclasDeRolagem(t *testing.T) {
	m := &Model{}
	m.viewport = viewport.New(40, 5)
	var conteudo []string
	for i := 0; i < 60; i++ {
		conteudo = append(conteudo, fmt.Sprintf("linha %d", i))
	}
	m.viewport.SetContent(strings.Join(conteudo, "\n"))
	m.viewport.GotoBottom()

	fim := m.viewport.YOffset
	if fim == 0 {
		t.Fatal("conteúdo curto demais para testar rolagem")
	}

	if !m.interceptKey(tea.KeyMsg{Type: tea.KeyPgUp}) {
		t.Error("PgUp deveria rolar")
	}
	if m.viewport.YOffset >= fim {
		t.Errorf("PgUp não subiu: %d → %d", fim, m.viewport.YOffset)
	}

	m.interceptKey(tea.KeyMsg{Type: tea.KeyHome})
	if m.viewport.YOffset != 0 {
		t.Errorf("Home deveria ir ao começo, ficou em %d", m.viewport.YOffset)
	}

	m.interceptKey(tea.KeyMsg{Type: tea.KeyPgDown})
	if m.viewport.YOffset == 0 {
		t.Error("PgDn deveria descer")
	}

	m.interceptKey(tea.KeyMsg{Type: tea.KeyEnd})
	if m.viewport.YOffset != fim {
		t.Errorf("End deveria voltar ao fim (%d), ficou em %d", fim, m.viewport.YOffset)
	}
}

// Percorre o caminho real: chunks de raciocínio chegam, a resposta vem vazia,
// e o Amazzo precisa tentar de novo. O teste unitário do retry chamava
// handleAIResponse direto e não pegava o que acontece no Update.
func TestFluxoCompletoRespostaVaziaTentaDeNovo(t *testing.T) {
	m := novoModelDeTeste(t)
	m.currentModel = 0
	m.conversationHistory = []agents.Message{{Role: "system", Content: "sys"}}
	m.streamStart = time.Now()
	m.loading = true

	// O modelo raciocina...
	m.Update(ollamaChunkMsg{text: "vou pensar sobre isso", thinking: true})
	if m.streamThinking == "" {
		t.Fatal("o raciocínio não foi acumulado")
	}

	// ...e encerra sem escrever nada.
	_, cmd := m.Update(ollamaDoneMsg{full: "", stats: agents.OllamaStats{GenTokens: 206}})

	if m.emptyRetries != 1 {
		t.Errorf("esperava uma nova tentativa, emptyRetries=%d", m.emptyRetries)
	}
	if cmd == nil {
		t.Error("deveria devolver um comando para tentar de novo")
	}
	juntas := strings.Join(m.messages, "\n")
	if strings.Contains(juntas, "sem produzir resposta") {
		t.Errorf("desistiu na primeira vez:\n%s", juntas)
	}
}

// A chegada da resposta não pode apagar a última mensagem da conversa.
//
// O handler removia a última linha sem olhar, porque assumia que era o
// placeholder "⏳ Processando..." — que deixou de existir quando a prévia do
// streaming passou a mostrar a fase. Desde então, cada resposta comia uma
// mensagem real.
func TestRespostaNaoApagaMensagemAnterior(t *testing.T) {
	m := novoModelDeTeste(t)
	m.conversationHistory = []agents.Message{{Role: "system", Content: "sys"}}
	m.addMessage("▸ leu main.go")
	m.addMessage("importante: não pode sumir")

	m.Update(apiResponseMsg{response: "resposta do modelo"})

	juntas := strings.Join(m.messages, "\n")
	if !strings.Contains(juntas, "importante: não pode sumir") {
		t.Errorf("a mensagem anterior foi apagada:\n%s", juntas)
	}
	if !strings.Contains(juntas, "leu main.go") {
		t.Errorf("o evento de ferramenta foi apagado:\n%s", juntas)
	}
}

// O placeholder de espera, esse sim, precisa sair quando a resposta chega.
func TestPlaceholderDeEsperaSaiQuandoARespostaChega(t *testing.T) {
	m := novoModelDeTeste(t)
	m.conversationHistory = []agents.Message{{Role: "system", Content: "sys"}}
	m.addMessage("mensagem real")
	m.addMessage("⏳ Processando...")

	m.Update(apiResponseMsg{response: "pronto"})

	juntas := strings.Join(m.messages, "\n")
	if strings.Contains(juntas, "⏳ Processando") {
		t.Errorf("o placeholder ficou na tela:\n%s", juntas)
	}
	if !strings.Contains(juntas, "mensagem real") {
		t.Errorf("apagou demais:\n%s", juntas)
	}
}

// Modelos pequenos escrevem "1." ou "-" onde o prompt pede "[ ]". A intenção
// é a mesma, e a lista precisa aparecer como etapas, não como texto solto.
func TestListaAceitaMarcadoresAlternativos(t *testing.T) {
	casos := map[string]string{
		"numerada com ponto":   "1. Definir a estrutura\n2. Implementar Insert\n3. Percorrer a árvore",
		"numerada com parênt.": "1) Definir\n2) Implementar\n3) Percorrer",
		"com hífen":            "- Definir\n- Implementar\n- Percorrer",
		"padrão do prompt":     "[ ] Definir\n[ ] Implementar\n[ ] Percorrer",
	}

	for nome, texto := range casos {
		out := ansi.Strip(formatTodoList(texto))
		if !strings.Contains(out, "plano · 0 de 3") {
			t.Errorf("%s: esperava 3 etapas reconhecidas:\n%s", nome, out)
		}
		if strings.Contains(out, "1.") || strings.Contains(out, "1)") {
			t.Errorf("%s: o marcador original deveria ser substituído:\n%s", nome, out)
		}
	}
}

// Reproduz a rolagem como acontece de verdade: o View() recalcula a altura do
// viewport a cada quadro, então PgUp/PgDn precisam continuar funcionando entre
// renderizações.
func TestRolagemFuncionaEntreRenderizacoes(t *testing.T) {
	m := novoModelDeTeste(t)
	m.showSplash = false
	m.Update(tea.WindowSizeMsg{Width: 150, Height: 45})

	for i := 0; i < 200; i++ {
		m.addMessage(fmt.Sprintf("linha de conversa número %d", i))
	}
	m.View() // o View ajusta a altura do viewport

	fim := m.viewport.YOffset
	if fim == 0 {
		t.Fatalf("conteúdo curto demais: altura=%d linhas=%d",
			m.viewport.Height, m.viewport.TotalLineCount())
	}

	m.interceptKey(tea.KeyMsg{Type: tea.KeyPgUp})
	m.View()
	subiu := m.viewport.YOffset
	if subiu >= fim {
		t.Fatalf("PgUp não subiu: %d → %d", fim, subiu)
	}

	m.interceptKey(tea.KeyMsg{Type: tea.KeyPgDown})
	m.View()
	if m.viewport.YOffset <= subiu {
		t.Errorf("PgDn não desceu: ficou em %d (era %d, fim é %d)",
			m.viewport.YOffset, subiu, fim)
	}
}

// A altura do viewport muda a cada quadro (a prévia de streaming cresce e
// some). Se YOffset ficar além do novo fim, ViewDown() acha que já está no
// fundo e não faz nada — era o PgDn "morto". Rolar por SetYOffset com clamp
// não tem esse ponto cego.
func TestPgDnFuncionaMesmoComOffsetAlemDoFim(t *testing.T) {
	m := novoModelDeTeste(t)
	m.viewport = viewport.New(40, 10)
	var linhas []string
	for i := 0; i < 100; i++ {
		linhas = append(linhas, fmt.Sprintf("linha %d", i))
	}
	m.viewport.SetContent(strings.Join(linhas, "\n"))

	// Simula o estado inconsistente: offset além do fim para a altura atual.
	m.viewport.YOffset = 95
	m.viewport.Height = 30

	m.scrollPage(-1)
	subiu := m.viewport.YOffset
	m.scrollPage(1)

	if m.viewport.YOffset <= subiu {
		t.Errorf("PgDn não desceu a partir de um offset inconsistente: %d → %d",
			subiu, m.viewport.YOffset)
	}
}

// Quem está lendo mais acima não pode ser puxado para o fim quando chega
// mensagem nova.
func TestNovaMensagemNaoPuxaQuemEstaLendo(t *testing.T) {
	m := novoModelDeTeste(t)
	m.viewport = viewport.New(40, 10)
	for i := 0; i < 60; i++ {
		m.addMessage(fmt.Sprintf("linha %d", i))
	}

	m.viewport.SetYOffset(5) // o usuário subiu para reler
	m.addMessage("mensagem nova chegou")

	if m.viewport.YOffset != 5 {
		t.Errorf("a leitura foi interrompida: offset %d, esperava 5", m.viewport.YOffset)
	}

	// Já no fim, a conversa continua acompanhando sozinha.
	m.viewport.GotoBottom()
	fim := m.viewport.YOffset
	m.addMessage("outra mensagem")
	if m.viewport.YOffset <= fim {
		t.Error("no fim da conversa, deveria continuar acompanhando")
	}
}

// O campo de digitação tinha altura fixa de 1 linha: um pedido de duas linhas
// escondia a primeira, e você escrevia às cegas.
func TestCampoDeTextoCresceComOConteudo(t *testing.T) {
	m := novoModelDeTeste(t)
	m.width, m.height = 150, 45
	m.textarea.SetWidth(60)

	m.adjustTextareaHeight()
	uma := m.textarea.Height()

	m.textarea.SetValue(strings.Repeat("palavra ", 40)) // várias linhas
	m.adjustTextareaHeight()

	if m.textarea.Height() <= uma {
		t.Errorf("o campo não cresceu: %d → %d", uma, m.textarea.Height())
	}

	// Não pode crescer sem limite e engolir a conversa.
	m.textarea.SetValue(strings.Repeat("palavra ", 2000))
	m.adjustTextareaHeight()
	if m.textarea.Height() > m.height/3 {
		t.Errorf("o campo tomou a tela: altura %d de %d", m.textarea.Height(), m.height)
	}

	// Ao limpar, volta ao tamanho de uma linha.
	m.textarea.SetValue("")
	m.adjustTextareaHeight()
	if m.textarea.Height() != uma {
		t.Errorf("não voltou ao tamanho original: %d, esperava %d", m.textarea.Height(), uma)
	}
}

// Trocar de pasta é decisão do usuário, e recomeça a conversa: o contexto
// anterior falava de arquivos que não estão mais à vista.
func TestTrocarDePastaRecomecaAConversa(t *testing.T) {
	dir := t.TempDir()

	m := novoModelDeTeste(t)
	fs, err := filesystem.NewManager(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	m.fsManager = fs
	m.workspaceRoot = fs.Root()
	m.conversationHistory = []agents.Message{
		{Role: "system", Content: "sys"},
		{Role: "user", Content: "sobre o projeto antigo"},
	}
	m.totalTokens = 900

	m.setupCommandDeps()
	if err := m.commandDeps.SetWorkspaceRoot(dir); err != nil {
		t.Fatalf("não trocou de pasta: %v", err)
	}

	if !strings.HasSuffix(m.workspaceRoot, strings.TrimPrefix(dir, "/private")) {
		t.Errorf("pasta não mudou: %q, esperava %q", m.workspaceRoot, dir)
	}
	if len(m.conversationHistory) != 1 {
		t.Errorf("o contexto do projeto anterior continuou: %d mensagens",
			len(m.conversationHistory))
	}
	if m.totalTokens != 0 {
		t.Error("os contadores do projeto anterior continuaram")
	}
}

// O erro de limite precisa dizer o que fazer, não só que negou.
func TestErroDeLimiteEnsinaOComando(t *testing.T) {
	fs, err := filesystem.NewManager(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}

	_, err = fs.ReadFile("/etc/passwd")
	if err == nil {
		t.Fatal("deveria negar caminho fora da pasta")
	}
	if !strings.Contains(err.Error(), "/pasta") {
		t.Errorf("o erro precisa ensinar a saída: %v", err)
	}
}

// Reproduz a digitação como acontece: o texto entra pelo Update, e o campo
// precisa crescer para que a primeira linha continue visível.
func TestCampoCresceAoDigitarPeloUpdate(t *testing.T) {
	m := novoModelDeTeste(t)
	m.showSplash = false
	m.Update(tea.WindowSizeMsg{Width: 150, Height: 45})

	longo := "ok ja tenho nocao do que voce e capaz, agora visite o diretorio " +
		"/mnt/hd1tb/docs/goProjects/vidgen faca uma analise nesse projeto, leia " +
		"todos os arquivos e crie um arquivo com as melhorias"

	m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(longo)})

	if m.textarea.Height() < 2 {
		t.Errorf("texto de %d colunas em campo de %d: altura ficou em %d",
			len(longo), m.textarea.Width(), m.textarea.Height())
	}

	// A primeira linha do que foi digitado precisa aparecer.
	vis := ansi.Strip(m.textarea.View())
	if !strings.Contains(vis, "ok ja tenho") {
		t.Errorf("o começo do texto ficou escondido:\n%s", vis)
	}
}

// Depois de enviar uma mensagem, o campo precisa voltar limpo de verdade: com
// resíduo da janela interna, o próximo texto longo aparecia rolado e a
// primeira linha ficava escondida.
func TestCampoVoltaLimpoDepoisDeEnviar(t *testing.T) {
	m := novoModelDeTeste(t)
	m.showSplash = false
	m.textarea.Prompt = "► "
	m.textarea.CharLimit = 50000
	m.conversationHistory = []agents.Message{{Role: "system", Content: "sys"}}
	m.Update(tea.WindowSizeMsg{Width: 150, Height: 45})

	// Escreve e envia algo (o envio limpa o campo).
	for _, r := range "uma mensagem qualquer que ja foi enviada" {
		m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{r}})
	}
	m.limpaCampoDeTexto()

	// Agora digita um pedido longo.
	longo := "ok ja tenho nocao do que voce e capaz, agora visite o diretorio " +
		"/mnt/hd1tb/docs/goProjects/vidgen faca uma analise nesse projeto, leia " +
		"todos os arquivos e crie um arquivo com as melhorias"
	for _, r := range longo {
		m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{r}})
	}

	primeira := ansi.Strip(strings.Split(m.textarea.View(), "\n")[0])
	if !strings.Contains(primeira, "ok ja tenho") {
		t.Errorf("a primeira linha do texto ficou escondida: %q", primeira)
	}
}

// Você não pode aprovar o que não consegue ver. A tela de confirmação só
// mostrava req.Path, que é vazio para execute_command: o comando ficava
// invisível e você dava "permitir" no escuro.
func TestConfirmacaoMostraOQueVaiAcontecer(t *testing.T) {
	m := novoModelDeTeste(t)

	texto := m.describePendingRequests([]mcp.Request{
		{Tool: "execute_command", Command: "rm -rf build && make"},
		{Tool: "write_file", Path: "docs/MELHORIAS.md", Content: "conteúdo longo"},
		{Tool: "make_dir", Path: "novo/diretorio"},
	})

	if !strings.Contains(texto, "rm -rf build && make") {
		t.Errorf("o comando precisa aparecer inteiro:\n%s", texto)
	}
	if !strings.Contains(texto, "docs/MELHORIAS.md") {
		t.Errorf("o arquivo a ser escrito precisa aparecer:\n%s", texto)
	}
	if !strings.Contains(texto, "novo/diretorio") {
		t.Errorf("o diretório a ser criado precisa aparecer:\n%s", texto)
	}
}

// Quando você nomeia uma pasta ("analise o projeto em /caminho/x"), o agente
// precisa poder abri-la — como abrir uma pasta no editor. Antes ele batia no
// limite do workspace e desistia, porque o Amazzo abre sempre em ~ quando
// lançado pelo rofi.
func TestAgentePodeAbrirPastaQueVoceNomeou(t *testing.T) {
	destino := t.TempDir()

	m := novoModelDeTeste(t)
	fs, err := filesystem.NewManager(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	m.fsManager = fs
	m.workspaceRoot = fs.Root()
	m.mcpServer = mcp.NewServer(fs)
	m.conversationHistory = []agents.Message{{Role: "system", Content: "sys"}}
	m.setupCommandDeps()

	res := m.runMCPRequests([]mcp.Request{{Tool: "set_workspace", Path: destino}})

	if len(res) != 1 || res[0].Status != "success" {
		t.Fatalf("não abriu a pasta: %+v", res)
	}
	if !strings.HasSuffix(m.workspaceRoot, strings.TrimPrefix(destino, "/private")) {
		t.Errorf("workspace continuou em %q, esperava %q", m.workspaceRoot, destino)
	}

	// E o filesystem passa a enxergar lá dentro.
	if _, _, err := m.fsManager.List("."); err != nil {
		t.Errorf("não consegue listar a pasta nova: %v", err)
	}
}

// Abrir uma pasta que não existe precisa falhar com clareza.
func TestAbrirPastaInexistenteFalhaClaro(t *testing.T) {
	m := novoModelDeTeste(t)
	fs, err := filesystem.NewManager(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	m.fsManager = fs
	m.workspaceRoot = fs.Root()
	m.mcpServer = mcp.NewServer(fs)
	m.conversationHistory = []agents.Message{{Role: "system", Content: "sys"}}
	m.setupCommandDeps()

	res := m.runMCPRequests([]mcp.Request{{Tool: "set_workspace", Path: "/nao/existe/mesmo"}})
	if len(res) != 1 || res[0].Status != "error" {
		t.Fatalf("deveria falhar: %+v", res)
	}
}

// Os modelos foram treinados no formato padrão de function calling
// ({"name":..., "arguments":{...}}) e caem nele sozinhos, mesmo quando o
// prompt pede {"tool":..., "path":...}. Ignorar esse formato era o motivo de
// pedidos complexos "não fazerem nada".
func TestAceitaFormatoDeFunctionCalling(t *testing.T) {
	m := &Model{}

	casos := map[string]struct {
		texto string
		tool  string
		path  string
	}{
		"name/arguments": {
			`{"name": "list_dir", "arguments": {"path": "/projeto"}}`,
			"list_dir", "/projeto"},
		"em cerca json": {
			"```json\n{\"name\": \"read_file\", \"arguments\": {\"path\": \"main.go\"}}\n```",
			"read_file", "main.go"},
		"function aninhado": {
			`{"function": {"name": "make_dir", "arguments": {"path": "docs"}}}`,
			"make_dir", "docs"},
		"arguments como texto": {
			`{"name": "read_file", "arguments": "{\"path\": \"go.mod\"}"}`,
			"read_file", "go.mod"},
		"formato do prompt": {
			`{"tool": "list_dir", "path": "."}`,
			"list_dir", "."},
	}

	for nome, c := range casos {
		reqs := m.parseToolRequests(c.texto)
		if len(reqs) != 1 {
			t.Errorf("%s: esperava 1 chamada, obtive %d", nome, len(reqs))
			continue
		}
		if reqs[0].Tool != c.tool || reqs[0].Path != c.path {
			t.Errorf("%s: obtive tool=%q path=%q, esperava %q/%q",
				nome, reqs[0].Tool, reqs[0].Path, c.tool, c.path)
		}
	}
}

// Várias chamadas em sequência, uma por linha — como o qwen realmente emite.
func TestVariasChamadasEmSequencia(t *testing.T) {
	m := &Model{}
	texto := `{"name": "set_workspace", "arguments": {"path": "/projeto"}}
{"name": "list_dir", "arguments": {"path": "/projeto"}}`

	reqs := m.parseToolRequests(texto)
	if len(reqs) != 2 {
		t.Fatalf("esperava 2 chamadas, obtive %d: %+v", len(reqs), reqs)
	}
	if reqs[0].Tool != "set_workspace" || reqs[1].Tool != "list_dir" {
		t.Errorf("ordem ou nomes errados: %+v", reqs)
	}
}

// write_file no formato de function calling precisa trazer o conteúdo.
func TestFunctionCallingComConteudo(t *testing.T) {
	m := &Model{}
	reqs := m.parseToolRequests(
		`{"name":"write_file","arguments":{"path":"MELHORIAS.md","content":"# Melhorias\n- uma"}}`)

	if len(reqs) != 1 {
		t.Fatalf("esperava 1 chamada, obtive %d", len(reqs))
	}
	if reqs[0].Path != "MELHORIAS.md" || !strings.Contains(reqs[0].Content, "Melhorias") {
		t.Errorf("conteúdo perdido: %+v", reqs[0])
	}
}

// tool_calls nativos precisam chegar ao loop de ferramentas. Como o resto do
// fluxo trabalha com JSON no texto, convertemos para o mesmo formato: assim
// tanto faz por qual caminho a chamada veio.
func TestToolCallsNativosViramChamadas(t *testing.T) {
	var chamada agents.ToolCall
	chamada.Function.Name = "list_dir"
	chamada.Function.Arguments = map[string]interface{}{"path": "/projeto"}

	texto := textoDasToolCalls([]agents.ToolCall{chamada})

	m := &Model{}
	reqs := m.parseToolRequests(texto)
	if len(reqs) != 1 {
		t.Fatalf("a chamada nativa não virou pedido: %q → %+v", texto, reqs)
	}
	if reqs[0].Tool != "list_dir" || reqs[0].Path != "/projeto" {
		t.Errorf("chamada errada: %+v", reqs[0])
	}
}

// Sem tool_calls nativos, nada é acrescentado ao texto.
func TestSemToolCallsNativosNadaMuda(t *testing.T) {
	if texto := textoDasToolCalls(nil); texto != "" {
		t.Errorf("esperava vazio, obtive %q", texto)
	}
}

// O resumo da ação precisa funcionar também no formato de function calling,
// senão toda chamada nativa vira "ferramenta usada" e você não vê o que ele fez.
func TestResumoDaAcaoNoFormatoNativo(t *testing.T) {
	casos := map[string]string{
		`{"name":"list_dir","arguments":{"path":"/projeto"}}`:          "listou /projeto",
		`{"name":"read_file","arguments":{"path":"main.go"}}`:          "leu main.go",
		`{"name":"set_workspace","arguments":{"path":"/meu/projeto"}}`: "/meu/projeto",
		`{"tool":"list_dir","path":"/projeto"}`:                        "listou /projeto",
	}

	for payload, esperado := range casos {
		got := descreveTool(payload)
		if !strings.Contains(got, esperado) {
			t.Errorf("%s → %q, esperava conter %q", payload, got, esperado)
		}
		if got == "ferramenta usada" {
			t.Errorf("%s virou resumo genérico", payload)
		}
	}
}

// Trabalho normal não pede confirmação: você não quer autorizar cada leitura,
// cada arquivo criado, cada build.
func TestTrabalhoNormalNaoPedeConfirmacao(t *testing.T) {
	m := novoModelDeTeste(t)
	m.permManager, _ = permissions.NewManager()

	rotina := []mcp.Request{
		{Tool: "read_file", Path: "main.go"},
		{Tool: "list_dir", Path: "."},
		{Tool: "search_files", Pattern: "TODO"},
		{Tool: "write_file", Path: "novo.md", Content: "x"},
		{Tool: "edit_file", Path: "main.go", Search: "a", Replace: "b"},
		{Tool: "make_dir", Path: "docs"},
		{Tool: "set_workspace", Path: "/projeto"},
		{Tool: "execute_command", Command: "go build ./..."},
		{Tool: "execute_command", Command: "make test"},
	}

	for _, req := range rotina {
		if m.requiresApproval(req) {
			t.Errorf("pediria confirmação sem necessidade: %s %s%s",
				req.Tool, req.Path, req.Command)
		}
	}
}

// O que apaga, sim: aí a confirmação protege de verdade.
func TestApagarPedeConfirmacao(t *testing.T) {
	m := novoModelDeTeste(t)
	m.permManager, _ = permissions.NewManager()

	perigosos := []mcp.Request{
		{Tool: "delete_file", Path: "importante.go"},
		{Tool: "execute_command", Command: "rm -rf build"},
		{Tool: "execute_command", Command: "git reset --hard"},
		{Tool: "move_file", Path: "a.go", DestPath: "b.go"},
	}

	for _, req := range perigosos {
		if !m.requiresApproval(req) {
			t.Errorf("deveria pedir confirmação: %s %s%s",
				req.Tool, req.Path, req.Command)
		}
	}
}

// Depois de abrir a pasta, o agente precisa CONTINUAR a tarefa original. Sem
// isso ele abria o projeto e parava com "como posso ajudar?", perdendo o
// pedido que o levou a abrir a pasta.
func TestAbrirPastaMandaContinuarATarefa(t *testing.T) {
	m := novoModelDeTeste(t)
	fs, err := filesystem.NewManager(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	m.fsManager = fs
	m.workspaceRoot = fs.Root()
	m.mcpServer = mcp.NewServer(fs)
	m.conversationHistory = []agents.Message{{Role: "system", Content: "sys"}}
	m.setupCommandDeps()

	res := m.runMCPRequests([]mcp.Request{{Tool: "set_workspace", Path: t.TempDir()}})
	if len(res) != 1 || res[0].Status != "success" {
		t.Fatalf("não abriu: %+v", res)
	}
	if !strings.Contains(strings.ToLower(res[0].Content), "continue") {
		t.Errorf("o resultado precisa mandar continuar a tarefa: %q", res[0].Content)
	}
}

// Quando o AGENTE abre a pasta no meio de uma tarefa, o histórico não pode ser
// apagado: era o que fazia ele abrir o projeto e perguntar "como posso
// ajudar?", tendo esquecido o pedido que o trouxe até ali.
func TestAgenteAbrindoPastaNaoPerdeATarefa(t *testing.T) {
	m := novoModelDeTeste(t)
	fs, err := filesystem.NewManager(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	m.fsManager = fs
	m.workspaceRoot = fs.Root()
	m.mcpServer = mcp.NewServer(fs)
	m.conversationHistory = []agents.Message{
		{Role: "system", Content: "sys"},
		{Role: "user", Content: "analise o projeto e crie um MELHORIAS.md"},
	}
	m.toolRuns = 2
	m.setupCommandDeps()

	m.runMCPRequests([]mcp.Request{{Tool: "set_workspace", Path: t.TempDir()}})

	if len(m.conversationHistory) < 2 {
		t.Fatalf("o pedido do usuário foi apagado: %d mensagens", len(m.conversationHistory))
	}
	if !strings.Contains(m.conversationHistory[1].Content, "MELHORIAS") {
		t.Errorf("a tarefa original sumiu: %+v", m.conversationHistory)
	}
	if m.toolRuns != 2 {
		t.Errorf("o contador de ciclos foi zerado no meio da tarefa: %d", m.toolRuns)
	}
}

// Já o comando /pasta é você trocando de projeto: aí a conversa recomeça.
func TestComandoPastaRecomecaAConversa(t *testing.T) {
	m := novoModelDeTeste(t)
	fs, err := filesystem.NewManager(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	m.fsManager = fs
	m.workspaceRoot = fs.Root()
	m.conversationHistory = []agents.Message{
		{Role: "system", Content: "sys"},
		{Role: "user", Content: "sobre o projeto antigo"},
	}
	m.setupCommandDeps()

	if err := m.commandDeps.SetWorkspaceRoot(t.TempDir()); err != nil {
		t.Fatal(err)
	}
	if len(m.conversationHistory) != 1 {
		t.Errorf("o contexto do projeto anterior continuou: %d mensagens",
			len(m.conversationHistory))
	}
}
