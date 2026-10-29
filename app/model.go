package app

import (
	"context"
	"crypto/tls"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"

	"amazzo/agents"
	"amazzo/commands"
	"amazzo/config"
	"amazzo/filesystem"
	"amazzo/logs"
	"amazzo/mcp"
	"amazzo/permissions"
	"amazzo/tasks"
	"amazzo/themes"
	"amazzo/ui"

	"github.com/alecthomas/chroma/v2/quick"
	"github.com/charmbracelet/bubbles/spinner"
	"github.com/charmbracelet/bubbles/textarea"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/glamour"
	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"
)

// remoteModels sao os modelos servidos pelo router do Hugging Face.
var remoteModels = []agents.ModelConfig{
	// === TIER 1: STATE-OF-THE-ART (Melhores modelos para código) ===
	{
		Name:        "qwen3-coder-480b",
		DisplayName: "🏆 Qwen3-Coder 480B (MoE Massivo)",
		Endpoint:    "Qwen/Qwen3-Coder-480B-A35B-Instruct:fastest",
	},
	{
		Name:        "qwen3-coder-30b",
		DisplayName: "⚡ Qwen3-Coder 30B (Rápido)",
		Endpoint:    "Qwen/Qwen3-Coder-30B-A3B-Instruct:fastest",
	},
	{
		Name:        "deepseek-r1",
		DisplayName: "🧠 DeepSeek-R1 (Reasoning)",
		Endpoint:    "deepseek-ai/DeepSeek-R1:fastest",
	},
	{
		Name:        "kimi-k2",
		DisplayName: "🤖 Kimi-K2 Thinking (Moonshot AI)",
		Endpoint:    "moonshotai/Kimi-K2-Thinking:nebius",
	},

	// === TIER 2: ALTERNATIVAS (Outros providers testados e funcionando) ===
	{
		Name:        "qwen3-coder-30b-nebius",
		DisplayName: "Qwen3-Coder 30B (Nebius)",
		Endpoint:    "Qwen/Qwen3-Coder-30B-A3B-Instruct:nebius",
	},
	{
		Name:        "gpt-oss",
		DisplayName: "GPT-OSS 120B (OpenAI)",
		Endpoint:    "openai/gpt-oss-120b:fastest",
	},
}

// fallbackLocalModel aparece quando o Ollama nao esta acessivel, para que a
// entrada local nunca suma da lista.
var fallbackLocalModel = agents.ModelConfig{
	Name:        "gemma3",
	DisplayName: "🏠 gemma3 (Ollama Local)",
	Endpoint:    "gemma3",
}

// availableModels e montada no arranque: modelos locais realmente instalados
// no Ollama primeiro, depois os remotos.
var availableModels = append([]agents.ModelConfig{fallbackLocalModel}, remoteModels...)

// loadAvailableModels pergunta ao Ollama quais modelos existem nesta maquina.
// Antes a lista era fixa em "gemma3", entao qualquer outro modelo baixado com
// `ollama pull` ficava invisivel para o Amazzo.
func loadAvailableModels() {
	local, err := agents.FetchOllamaModels()
	if err != nil || len(local) == 0 {
		return
	}
	availableModels = append(local, remoteModels...)
}

var commandSuggestions = []string{
	"/ajuda",
	"/help",
	"/limpar",
	"/clear",
	"/modelo",
	"/model",
	"/agente",
	"/agentes",
	"/multiagente",
	"/temas",
	"/theme",
	"/themes",
	"/sobre",
	"/about",
	"/logs",
	"/salvar",
	"/arquivar",
	"/copiar",
	"/erro",
	"/metricas",
	"/metrics",
}

// Model agrega o estado principal do Bubble Tea.
type Model struct {
	viewport viewport.Model
	textarea textarea.Model
	spinner  spinner.Model

	messages            []string
	conversationHistory []agents.Message

	currentModel int
	currentTheme string
	theme        themes.ThemeConfig

	hfToken       string
	fsManager     *filesystem.Manager
	logSaver      *logs.Saver
	workspaceRoot string

	mcpServer        *mcp.Server
	permManager      *permissions.Manager
	awaitingApproval bool
	pendingRequests  []mcp.Request
	pendingResults   []mcp.Result
	deniedRequests   map[string]time.Time // Rastreia requisições negadas recentemente para evitar loops

	width          int
	height         int
	loading        bool
	err            error
	lastWasCommand bool
	commandHistory []string
	historyIndex   int
	savedInput     string
	lastError      string
	lastAIResponse string
	prevAIResponse string
	// Ultimas respostas normalizadas, para detectar o agente girando em falso.
	recentResponses  []string
	showBanner       bool
	showSplash       bool
	splashStartTime  time.Time
	tickCount        int // Contador de ticks para prevenir loops infinitos
	firstMessageSent bool
	cancelFunc       context.CancelFunc // Para cancelar requisições em andamento com ESC

	// Autocomplete de arquivos com @
	fileAutocomplete       bool
	fileAutocompleteList   []string
	fileAutocompleteIdx    int
	fileAutocompletePrefix string

	commandDeps commands.Dependencies

	toolRuns int

	// Contadores de tokens
	inputTokens  int
	outputTokens int
	totalTokens  int

	// Sistema multi-agente
	agentManager             *agents.Manager
	multiAgentMode           bool
	agentAPI                 *agents.DefaultAPI
	specializedAgentRegistry *agents.SpecializedAgentRegistry

	// Cache e métricas
	cache        *ResponseCache
	metrics      *Metrics
	taskTracker  *tasks.Tracker
	taskView     *ui.TaskView
	agentCreator *ui.AgentCreator
	retryClient  *RetryableHTTPClient
	debugMode    bool
	toolRegistry *agents.ToolRegistry

	// Configurações do usuário
	configManager *config.Manager
	fontSize      int

	// Glamour renderer para markdown elegante
	glamourRenderer *glamour.TermRenderer
	glamourWidth    int

	// Streaming do Ollama: canal de chunks e texto parcial ja recebido.
	streamCh         chan tea.Msg
	streamPartial    string
	streamThinking   string
	lastThinking     string
	lastThinkingTime time.Duration

	// chamadasFeitas guarda a assinatura de cada chamada ja executada e o
	// resultado que ela deu, para avisar o modelo quando ele se repete.
	chamadasFeitas map[string]string

	// toolCallsTotal conta as ferramentas usadas na conversa inteira.
	// toolRuns e o contador de CICLOS do turno atual e volta a zero ao fim de
	// cada resposta -- mostra-lo no header fazia a barra dizer "0 ferramentas"
	// logo depois de o agente ter rodado quatro.
	toolCallsTotal int

	// lastToolCalls guarda as chamadas do canal nativo da ultima resposta. Elas
	// precisam entrar no historico como tool_calls, nao como texto: e isso que
	// faz o template de chat do modelo abrir um turno de ferramenta.
	lastToolCalls   []agents.ToolCall
	showThinking    bool
	emptyRetries    int
	thoughts        []thought
	streamStart     time.Time
	lastOllamaStats agents.OllamaStats

	// Estilos modernos (Warp-like)
	modernStyles ModernStyles

	// Estilos ULTRA modernos e extraordinários
	ultimateStyles UltimateStyles

	// Modo ultra visual ativado
	ultraMode bool
}

type apiResponseMsg struct {
	response     string
	err          error
	wasTruncated bool
}

type tickMsg time.Time

type splashDoneMsg struct{}

// New cria um bubbletea model configurado.
func New(hfToken, workspace, tokenStatus string) (*Model, error) {
	debug := os.Getenv("DEBUG") == "1"

	// Descobre os modelos locais instalados no Ollama.
	loadAvailableModels()

	if debug {
		fmt.Fprintf(os.Stderr, "[DEBUG] app.New() iniciando...\n")
	}

	if workspace == "" {
		cwd, err := os.Getwd()
		if err != nil {
			return nil, fmt.Errorf("falha ao localizar diretório atual: %w", err)
		}
		workspace = cwd
	}

	if debug {
		fmt.Fprintf(os.Stderr, "[DEBUG] Workspace: %s\n", workspace)
		fmt.Fprintf(os.Stderr, "[DEBUG] Criando filesystem manager...\n")
	}

	fsManager, err := filesystem.NewManager(workspace)
	if err != nil {
		return nil, err
	}

	if debug {
		fmt.Fprintf(os.Stderr, "[DEBUG] Filesystem manager criado\n")
		fmt.Fprintf(os.Stderr, "[DEBUG] Criando logs saver...\n")
	}

	saver := logs.NewSaver(fsManager)

	if debug {
		fmt.Fprintf(os.Stderr, "[DEBUG] Logs saver criado\n")
		fmt.Fprintf(os.Stderr, "[DEBUG] Carregando configurações...\n")
	}

	// Carrega configurações do usuário
	configMgr, err := config.NewManager(workspace)
	if err != nil {
		return nil, fmt.Errorf("falha ao carregar configurações: %w", err)
	}

	if debug {
		fmt.Fprintf(os.Stderr, "[DEBUG] Configurações carregadas\n")
	}

	ta := novoCampoDeTexto(110)

	vp := viewport.New(110, 24)
	vp.SetContent("")
	vp.MouseWheelEnabled = true

	// Inicializa spinner
	s := spinner.New()
	s.Spinner = spinner.Dot
	s.Style = themes.Available[themes.DefaultName()].InfoStyle

	// Usa tema salvo nas preferências, ou default se não houver
	savedThemeName := configMgr.GetTheme()
	defaultTheme, ok := themes.Available[savedThemeName]
	if !ok {
		savedThemeName = themes.DefaultName()
		defaultTheme = themes.Available[savedThemeName]
	}

	// Carrega último modelo usado
	lastModel := configMgr.GetLastModel()
	if lastModel < 0 || lastModel >= len(availableModels) {
		lastModel = 0
	}

	// O prompt antigo mandava "NUNCA PARE NO MEIO", "EXECUTE!", "SEMPRE crie
	// uma lista de TODO" -- e o modelo obedecia até para um "olá", saindo a
	// listar diretórios sem ninguém pedir. Um assistente decide o que a
	// pergunta exige; agir sempre não é iniciativa, é ruído.
	conversationHistory := []agents.Message{
		{
			Role:    "system",
			Content: remoteSystemPrompt,
		},
	}

	if debug {
		fmt.Fprintf(os.Stderr, "[DEBUG] Criando registry de agentes especializados...\n")
	}

	// Inicializa o registry de agentes especializados
	specializedRegistry := agents.NewSpecializedAgentRegistry(workspace)

	if debug {
		fmt.Fprintf(os.Stderr, "[DEBUG] Carregando agentes especializados...\n")
	}

	// Carrega agentes especializados (cria padrão se não existirem)
	if err := specializedRegistry.LoadAgents(); err != nil {
		return nil, fmt.Errorf("falha ao carregar agentes especializados: %w", err)
	}

	if debug {
		fmt.Fprintf(os.Stderr, "[DEBUG] Agentes especializados carregados\n")
		fmt.Fprintf(os.Stderr, "[DEBUG] Inicializando gerenciador de permissões...\n")
	}

	// Inicializa gerenciador de permissões
	permMgr, err := permissions.NewManager()
	if err != nil {
		return nil, fmt.Errorf("falha ao inicializar gerenciador de permissões: %w", err)
	}

	if debug {
		fmt.Fprintf(os.Stderr, "[DEBUG] Gerenciador de permissões inicializado\n")
	}

	// Atualiza estilo do spinner com tema correto
	s.Style = defaultTheme.InfoStyle

	// Inicializa Glamour renderer com tema dark
	glamourStyle := "dark"
	isDarkTheme := true
	if savedThemeName == "light" || savedThemeName == "solarized" {
		glamourStyle = "light"
		isDarkTheme = false
	}
	_ = glamourStyle // largura/estilo passam a ser definidos por setGlamourWidth

	// Inicializa estilos modernos
	modernStyles := NewModernStyles(isDarkTheme)

	m := &Model{
		textarea:                 ta,
		viewport:                 vp,
		spinner:                  s,
		messages:                 []string{},
		conversationHistory:      conversationHistory,
		currentModel:             lastModel,
		currentTheme:             savedThemeName,
		theme:                    defaultTheme,
		hfToken:                  hfToken,
		fsManager:                fsManager,
		logSaver:                 saver,
		workspaceRoot:            fsManager.Root(),
		mcpServer:                mcp.NewServer(fsManager),
		permManager:              permMgr,
		deniedRequests:           make(map[string]time.Time), // Inicializa rastreador de requisições negadas
		loading:                  false,
		showBanner:               false,                                // Começa oculto durante splash
		showSplash:               os.Getenv("AMAZZO_NO_SPLASH") != "1", // AMAZZO_NO_SPLASH=1 abre direto
		splashStartTime:          time.Now(),
		firstMessageSent:         false,
		cache:                    NewResponseCache(30*time.Minute, 100),
		metrics:                  NewMetrics(),
		taskTracker:              tasks.NewTracker(),
		retryClient:              NewRetryableHTTPClient(90*time.Second, DefaultRetryConfig()),
		debugMode:                os.Getenv("DEBUG") == "1",
		specializedAgentRegistry: specializedRegistry,
		configManager:            configMgr,
		fontSize:                 configMgr.GetFontSize(),
		modernStyles:             modernStyles,
		ultimateStyles:           NewUltimateStyles(),
		ultraMode:                false, // Modo normal por padrão, /ultra para ativar
	}

	// Renderer de markdown na largura padrao; ajustado em WindowSizeMsg.
	m.setGlamourWidth(100)

	// Inicializa task view
	// Adiciona status do token às mensagens iniciais
	if tokenStatus != "" {
		m.addMessage(m.theme.InfoStyle.Render(tokenStatus))
		m.addMessage("")
	}
	m.taskView = ui.NewTaskView(m.taskTracker, m.theme)

	// Inicializa agent creator
	m.agentCreator = ui.NewAgentCreator(m.theme)

	if debug {
		fmt.Fprintf(os.Stderr, "[DEBUG] Configurando command dependencies...\n")
	}

	m.setupCommandDeps()

	// NÃO adiciona mensagens iniciais - elas serão adicionadas após splash screen

	if debug {
		fmt.Fprintf(os.Stderr, "[DEBUG] app.New() concluído com sucesso\n")
	}

	return m, nil
}

func (m *Model) setupCommandDeps() {
	m.commandDeps = commands.Dependencies{
		AddMessage: func(text string) {
			m.addMessage(text)
		},
		AddInfo: func(text string) {
			m.addMessage(m.theme.InfoStyle.Render(text))
		},
		AddHelp: func(text string) {
			m.addMessage(m.theme.HelpStyle.Render(text))
		},
		AddError: func(text string) {
			m.addMessage(m.theme.ErrorStyle.Render(text))
		},
		AddSuccess: func(text string) {
			m.addMessage(m.theme.InfoStyle.Render(text))
		},
		AddBlank: func() {
			m.addMessage("")
		},
		ClearMessages: func() {
			m.messages = []string{}
			m.viewport.SetContent("")
		},
		ShowAbout: m.showAbout,
		GetCurrentModelIndex: func() int {
			return m.currentModel
		},
		ListModels: func() []commands.ModelInfo {
			result := make([]commands.ModelInfo, len(availableModels))
			for i, model := range availableModels {
				result[i] = commands.ModelInfo{
					DisplayName: model.DisplayName,
					Endpoint:    model.Endpoint,
				}
			}
			return result
		},
		WorkspaceRoot: func() string {
			return m.workspaceRoot
		},
		SetWorkspaceRoot: func(caminho string) error {
			// O comando /pasta é você trocando de projeto: a conversa
			// anterior falava de arquivos que não estão mais à vista.
			return m.mudaWorkspace(caminho, true)
		},
		RefreshLocalModels: func() (int, error) {
			local, err := agents.FetchOllamaModels()
			if err != nil {
				return 0, err
			}

			// Preserva o modelo selecionado pelo nome: a posicao dele na
			// lista muda quando modelos novos entram.
			atual := availableModels[m.currentModel].Name
			availableModels = append(local, remoteModels...)

			m.currentModel = 0
			for i, cfg := range availableModels {
				if cfg.Name == atual {
					m.currentModel = i
					break
				}
			}
			m.configManager.SetLastModel(m.currentModel)

			return len(local), nil
		},
		SetModel: func(idx int) error {
			if idx < 0 || idx >= len(availableModels) {
				return fmt.Errorf("modelo inválido: %d", idx)
			}
			m.currentModel = idx
			m.configManager.SetLastModel(idx) // Salva último modelo usado
			m.resetConversation()
			return nil
		},
		GetCurrentThemeKey: func() string {
			return m.currentTheme
		},
		ListThemes: func() []commands.ThemeInfo {
			keys := themes.SortedKeys()
			result := make([]commands.ThemeInfo, 0, len(keys))
			for _, key := range keys {
				result = append(result, commands.ThemeInfo{
					Key:  key,
					Name: themes.Available[key].Name,
				})
			}
			return result
		},
		SetTheme: func(name string) error {
			theme, ok := themes.Available[name]
			if !ok {
				return fmt.Errorf("tema não encontrado: %s", name)
			}
			m.currentTheme = name
			m.theme = theme
			return nil
		},
		SaveTheme: func(name string) error {
			return m.configManager.SetTheme(name)
		},
		LastError: func() string {
			return m.lastError
		},
		LastResponse: func() string {
			return m.lastAIResponse
		},
		SaveConversation: func(filename string) (string, error) {
			return m.logSaver.SaveConversation(filename, m.messages)
		},
		SaveReadableConversation: func(filename string) (string, error) {
			return m.logSaver.SaveReadableConversation(filename, m.messages)
		},
		SaveLastResponse: func() (string, error) {
			if m.lastAIResponse == "" {
				return "", fmt.Errorf("nenhuma resposta salva ainda")
			}
			return m.logSaver.SaveTempResponse(m.lastAIResponse)
		},
		WorkingDir: func() string {
			return m.workspaceRoot
		},
		IsMultiAgentMode: func() bool {
			return m.multiAgentMode
		},
		ToggleMultiAgentMode: func() bool {
			m.multiAgentMode = !m.multiAgentMode

			// Inicializa agentManager na primeira vez que for ativado
			if m.multiAgentMode && m.agentManager == nil {
				m.initializeAgentManager()
			}

			return m.multiAgentMode
		},
		GetMetrics: func() string {
			return m.formatMetrics()
		},
		GetSpecializedAgentRegistry: func() interface{} {
			return m.specializedAgentRegistry
		},
		UpdateToken: func(newToken string) error {
			// Atualiza o token em tempo real
			m.hfToken = newToken
			return nil
		},
		GetTaskStatus: func() string {
			return m.taskTracker.GetSummary()
		},
		StartAgentCreation: func() {
			m.agentCreator.Start(
				// onComplete: quando o agente é criado com sucesso
				func(agentConfig *agents.AgentConfig) {
					// Registra o agente no registry
					m.specializedAgentRegistry.Register(agentConfig)

					// Mostra mensagem de sucesso
					m.addMessage(m.theme.InfoStyle.Render(fmt.Sprintf("✅ Agente '%s' criado com sucesso!", agentConfig.Name)))
					m.addMessage("")
				},
				// onCancel: quando o usuário cancela com ESC
				func() {
					m.addMessage(m.theme.HelpStyle.Render("⚠️  Criação de agente cancelada"))
					m.addMessage("")
				},
			)
		},
		IsUltraMode: func() bool {
			return m.ultraMode
		},
		ToggleUltraMode: func() bool {
			m.ultraMode = !m.ultraMode
			return m.ultraMode
		},
	}
}

// initializeAgentManager inicializa o sistema multi-agente sob demanda.
func (m *Model) initializeAgentManager() {
	// Cria API que conecta agentes ao sistema
	m.agentAPI = agents.NewDefaultAPI(
		// CallModel func
		func(ctx context.Context, model string, prompt string) (string, error) {
			// Implementação simplificada - delega para callAPI
			// TODO: Implementar chamada real ao modelo
			return "mock response", nil
		},
		// ExecuteTool func
		func(ctx context.Context, tool string, params map[string]interface{}) (interface{}, error) {
			// Cria requisição MCP a partir dos params
			req := mcp.Request{
				Tool: tool,
			}

			if path, ok := params["path"].(string); ok {
				req.Path = path
			}
			if content, ok := params["content"].(string); ok {
				req.Content = content
			}

			// Executa via MCP server
			results, err := m.mcpServer.Handle(ctx, []mcp.Request{req})
			if err != nil {
				return nil, err
			}
			if len(results) == 0 {
				return nil, fmt.Errorf("nenhum resultado")
			}

			return results[0], nil
		},
		// LogFunc
		func(event string) {
			m.addMessage(m.theme.InfoStyle.Render(fmt.Sprintf("[Multi-Agente] %s", event)))
		},
	)

	// Cria manager com 4 agentes no máximo
	m.agentManager = agents.NewManager(m.agentAPI, 4)

	m.addMessage(m.theme.InfoStyle.Render("✅ Sistema Multi-Agente inicializado"))
}

// resetConversation comeca uma conversa nova, mantendo apenas o system prompt.
//
// Tudo o que a barra de status mostra e da conversa: deixar os contadores do
// modelo anterior seria exibir o custo de um modelo como se fosse do outro.
func (m *Model) resetConversation() {
	m.conversationHistory = []agents.Message{
		m.conversationHistory[0],
	}

	m.recentResponses = nil
	m.chamadasFeitas = nil
	m.toolRuns = 0
	m.toolCallsTotal = 0
	m.emptyRetries = 0

	m.inputTokens = 0
	m.outputTokens = 0
	m.totalTokens = 0
	m.lastOllamaStats = agents.OllamaStats{}

	m.thoughts = nil
	m.lastThinking = ""
	m.lastThinkingTime = 0
	m.lastToolCalls = nil
	m.lastAIResponse = ""
	m.prevAIResponse = ""
}

func (m *Model) addMessage(msg string) {
	// Se o usuário subiu para reler, mensagem nova não pode puxá-lo de volta
	// para o fim no meio da leitura. Acompanhar sozinho só faz sentido para
	// quem já está no fim.
	acompanhando := m.viewport.AtBottom()

	wrapped := wrapText(msg, m.viewport.Width)
	m.messages = append(m.messages, wrapped)
	m.viewport.SetContent(strings.Join(m.messages, "\n"))

	if acompanhando {
		m.viewport.GotoBottom()
	}
}

// handleZoom ajusta o tamanho da fonte (viewport e textarea).
func (m *Model) handleZoom(delta int) {
	if err := m.configManager.AdjustFontSize(delta); err == nil {
		m.fontSize = m.configManager.GetFontSize()
		// Calcula novo tamanho baseado no percentual
		baseWidth := 110
		baseHeight := 24
		scale := float64(m.fontSize) / 100.0

		newWidth := int(float64(baseWidth) * scale)
		newHeight := int(float64(baseHeight) * scale)

		// Limita dimensões mínimas e máximas
		if newWidth < 60 {
			newWidth = 60
		}
		if newWidth > 220 {
			newWidth = 220
		}
		if newHeight < 12 {
			newHeight = 12
		}
		if newHeight > 48 {
			newHeight = 48
		}

		m.viewport.Width = newWidth
		m.viewport.Height = newHeight
		m.textarea.SetWidth(newWidth)

		m.addMessage(m.theme.InfoStyle.Render(fmt.Sprintf("🔍 Zoom: %d%%", m.fontSize)))
		m.addMessage("")
	}
}

// handleZoomReset reseta o zoom para 100%.
func (m *Model) handleZoomReset() {
	if err := m.configManager.SetFontSize(100); err == nil {
		m.fontSize = 100
		m.viewport.Width = 110
		m.viewport.Height = 24
		m.textarea.SetWidth(110)

		m.addMessage(m.theme.InfoStyle.Render("🔍 Zoom resetado para 100%"))
		m.addMessage("")
	}
}

// showWelcomeTutorial mostra uma mensagem simples e amigável para novos usuários.
func (m *Model) showWelcomeTutorial() {
	m.addMessage("")
	m.addMessage(m.theme.InfoStyle.Render("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"))
	m.addMessage("")
	m.addMessage(m.theme.LogoStyle.Render("        👋 Olá! Sou o Amazzo, seu assistente de IA."))
	m.addMessage("")
	m.addMessage("  💬 Converse comigo naturalmente - faça perguntas, peça ajuda com código,")
	m.addMessage("     ou solicite que eu crie arquivos e execute comandos.")
	m.addMessage("")
	m.addMessage(m.theme.HelpStyle.Render("  Dicas rápidas:"))
	m.addMessage("     • Digite /ajuda para ver todos os comandos")
	m.addMessage("     • Use @arquivo.txt para incluir conteúdo de arquivos")
	m.addMessage("     • Ctrl+L para limpar a tela")
	m.addMessage("")
	m.addMessage(m.theme.InfoStyle.Render("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"))
	m.addMessage("")
}

// wrapText quebra texto longo com base na largura do viewport.
// wrapText quebra linhas respeitando a LARGURA VISUAL (celulas do terminal),
// nao a contagem de bytes, e preservando as sequencias ANSI intactas.
//
// A versao anterior usava len() em bytes e strings.Fields(), o que:
//   - cortava sequencias como \x1b[38;5;252m no meio, vazando ";252m" na tela;
//   - quebrava linhas cedo demais (emoji/acentos ocupam varios bytes);
//   - descartava a indentacao, destruindo caixas desenhadas com ┌ │ └.
func wrapText(text string, maxWidth int) string {
	if maxWidth <= 0 {
		maxWidth = 80
	}

	// Conteudo que ja cabe na largura nao deve ser tocado: reprocessar
	// destroi alinhamento de tabelas, caixas e blocos de codigo.
	fits := true
	for _, line := range strings.Split(text, "\n") {
		if ansi.StringWidth(line) > maxWidth {
			fits = false
			break
		}
	}
	if fits {
		return text
	}

	// ansi.Wrap quebra em espacos e so parte palavras maiores que o limite,
	// sempre reabrindo os estilos ativos na linha seguinte.
	return ansi.Wrap(text, maxWidth, "")
}

// adjustTextareaHeight faz o campo de digitação crescer com o texto.
//
// Contava só as quebras de linha explícitas, então um pedido longo numa linha
// só continuava com altura 1: o texto rolava dentro do campo e a primeira
// parte sumia enquanto você ainda escrevia. Agora conta as linhas COMO SÃO
// EXIBIDAS, quebradas na largura do campo.
func (m *Model) adjustTextareaHeight() {
	// Conta as linhas COMO SÃO EXIBIDAS. Duas armadilhas aqui:
	// o textarea quebra por palavra (não a cada N colunas), e o prompt "► "
	// come colunas da largura útil. Subcontar deixava a altura menor que o
	// texto, e a primeira linha do que você escreveu ficava fora da janela.
	util := m.textarea.Width() - ansi.StringWidth(m.textarea.Prompt)
	if util < 10 {
		util = 10
	}

	linhas := 0
	for _, linha := range strings.Split(m.textarea.Value(), "\n") {
		if strings.TrimSpace(linha) == "" {
			linhas++
			continue
		}
		linhas += strings.Count(ansi.Wrap(linha, util, ""), "\n") + 1
	}
	if linhas < 1 {
		linhas = 1
	}

	// Um terço da tela: além disso o campo passaria a engolir a conversa que
	// você está respondendo.
	teto := m.height / 3
	if teto < 1 {
		teto = 5 // antes do primeiro WindowSizeMsg não sabemos a altura
	}
	if linhas > teto {
		linhas = teto
	}

	if m.textarea.Height() != linhas {
		m.textarea.SetHeight(linhas)
	}
}

func (m *Model) interceptKey(msg tea.KeyMsg) bool {
	switch msg.Type {
	// Rolagem da conversa. Existia só Alt+setas, linha a linha: inútil para
	// reler ou copiar uma conversa longa.
	case tea.KeyPgUp:
		m.scrollPage(-1)
		return true

	case tea.KeyPgDown:
		m.scrollPage(1)
		return true

	case tea.KeyHome:
		m.viewport.GotoTop()
		return true

	case tea.KeyEnd:
		m.viewport.GotoBottom()
		return true

	case tea.KeyUp:
		if msg.Alt {
			// Alt+Up = scroll viewport
			if m.viewport.YOffset > 0 {
				m.viewport.LineUp(1)
			}
			return true
		}
		// Up sozinho = deixa o textarea processar (navegação dentro do texto)
		// Só usa histórico se estiver na primeira linha do textarea
		lines := strings.Split(m.textarea.Value(), "\n")
		if m.textarea.Line() == 0 && len(lines) > 0 {
			m.handleHistoryNavigation(-1)
			return true
		}
		return false // Deixa textarea processar

	case tea.KeyDown:
		if msg.Alt {
			// Alt+Down = scroll viewport
			if m.viewport.YOffset < m.viewport.TotalLineCount()-m.viewport.VisibleLineCount() {
				m.viewport.LineDown(1)
			}
			return true
		}
		// Down sozinho = deixa o textarea processar
		// Só usa histórico se estiver na última linha do textarea
		lines := strings.Split(m.textarea.Value(), "\n")
		if m.textarea.Line() == len(lines)-1 {
			m.handleHistoryNavigation(1)
			return true
		}
		return false // Deixa textarea processar

	case tea.KeyTab:
		return m.autocompleteCommand()

	case tea.KeyEsc:
		// Interrompe a geração em andamento.
		//
		// Sem teto de tokens, o Esc é o freio: o modelo local pode escrever
		// por minutos. O que ele já produziu é preservado — costuma ser o
		// que interessa, e jogar fora obrigaria a começar do zero.
		if m.loading && m.cancelFunc != nil {
			m.cancelFunc()
			m.loading = false

			parcial := strings.TrimSpace(m.streamPartial)
			m.streamPartial = ""
			m.streamThinking = ""

			if parcial != "" {
				m.addMessage(renderAssistantEntry(
					availableModels[m.currentModel].DisplayName+" (interrompido)",
					m.responseMeta(), m.renderMarkdown(parcial), m.viewport.Width))
				m.conversationHistory = append(m.conversationHistory, agents.Message{
					Role: "assistant", Content: parcial,
				})
			} else {
				m.addMessage(renderNotice("■", "Geração interrompida.", colGraphite))
			}
			m.addMessage("")
			return true
		}
		return false

	case tea.KeyCtrlC:
		return false
	}

	// Zoom in e Zoom out via Ctrl+ com runes
	if msg.Alt == false {
		switch msg.String() {
		case "ctrl++", "ctrl+=":
			m.handleZoom(10)
			return true
		case "ctrl+-", "ctrl+_":
			m.handleZoom(-10)
			return true
		case "ctrl+0":
			m.handleZoomReset()
			return true
		case "ctrl+t":
			// Toggle visualização de tarefas
			if m.taskView != nil {
				m.taskView.Toggle()
			}
			return true
		}
	}

	return false
}

func (m *Model) handleHistoryNavigation(direction int) bool {
	if len(m.commandHistory) == 0 || !m.textarea.Focused() {
		return false
	}

	if direction < 0 {
		// Going backward in history (Up arrow)
		return m.stepHistoryBackward()
	}

	if direction > 0 {
		// Going forward in history (Down arrow)
		// Only navigate if we're already in history mode
		if m.historyIndex > 0 {
			return m.stepHistoryForward()
		}
	}

	return false
}

func (m *Model) stepHistoryBackward() bool {
	if len(m.commandHistory) == 0 {
		return false
	}

	// Save current input when starting to navigate history
	if m.historyIndex == 0 {
		m.savedInput = m.textarea.Value()
	}

	if m.historyIndex >= len(m.commandHistory) {
		return false
	}

	m.historyIndex++
	idx := len(m.commandHistory) - m.historyIndex
	if idx < 0 {
		idx = 0
		m.historyIndex = len(m.commandHistory)
	}

	m.textarea.SetValue(m.commandHistory[idx])
	m.textarea.CursorEnd()
	m.adjustTextareaHeight()
	return true
}

func (m *Model) autocompleteCommand() bool {
	value := m.textarea.Value()
	trimmed := strings.TrimSpace(value)
	if !strings.HasPrefix(trimmed, "/") {
		return false
	}
	if strings.ContainsAny(trimmed, " \n\t") {
		return false
	}

	var matches []string
	for _, cmd := range commandSuggestions {
		if strings.HasPrefix(cmd, trimmed) {
			matches = append(matches, cmd)
		}
	}

	if len(matches) == 0 {
		return false
	}

	suggestion := matches[0]
	m.textarea.SetValue(suggestion + " ")
	m.textarea.CursorEnd()
	m.adjustTextareaHeight()

	if len(matches) > 1 {
		m.addMessage(m.theme.HelpStyle.Render("Sugestões: " + strings.Join(matches, ", ")))
		m.addMessage("")
	}
	return true
}

func (m *Model) stepHistoryForward() bool {
	if m.historyIndex == 0 {
		return false
	}

	m.historyIndex--
	if m.historyIndex == 0 {
		// Restore the saved input when returning to present
		m.textarea.SetValue(m.savedInput)
		m.savedInput = ""
	} else {
		idx := len(m.commandHistory) - m.historyIndex
		if idx < 0 {
			idx = 0
		}
		if idx < len(m.commandHistory) {
			m.textarea.SetValue(m.commandHistory[idx])
		}
	}

	m.textarea.CursorEnd()
	m.adjustTextareaHeight()
	return true
}

// detectFileAutocomplete detecta @ no input e ativa autocomplete
func (m *Model) detectFileAutocomplete() {
	value := m.textarea.Value()

	// Procura por @ seguido de caracteres opcionais
	lastAt := strings.LastIndex(value, "@")
	if lastAt == -1 {
		m.fileAutocomplete = false
		return
	}

	// Extrai o texto após o @ até o cursor ou espaço
	afterAt := value[lastAt+1:]
	spaceIdx := strings.IndexAny(afterAt, " \n\t")
	if spaceIdx != -1 {
		afterAt = afterAt[:spaceIdx]
	}

	// Ativa autocomplete e busca arquivos
	m.fileAutocompletePrefix = afterAt
	m.fileAutocomplete = true
	m.fileAutocompleteIdx = 0
	m.fileAutocompleteList = m.searchFilesRecursive(m.workspaceRoot, m.fileAutocompletePrefix)
}

// searchFilesRecursive busca arquivos recursivamente
func (m *Model) searchFilesRecursive(root, prefix string) []string {
	var files []string
	prefix = strings.ToLower(prefix)

	filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return nil
		}

		// Ignora diretórios ocultos e específicos
		if info.IsDir() {
			name := info.Name()
			if strings.HasPrefix(name, ".") || name == "node_modules" || name == "vendor" {
				return filepath.SkipDir
			}
			return nil
		}

		// Calcula caminho relativo
		relPath, err := filepath.Rel(root, path)
		if err != nil {
			return nil
		}

		// Filtra por prefixo se houver
		fileName := strings.ToLower(info.Name())
		if prefix == "" || strings.HasPrefix(fileName, prefix) {
			files = append(files, relPath)
		}

		// Limita a 50 arquivos
		if len(files) >= 50 {
			return filepath.SkipAll
		}

		return nil
	})

	return files
}

// handleFileAutocompleteNav navega na lista de autocomplete
func (m *Model) handleFileAutocompleteNav(direction int) bool {
	if !m.fileAutocomplete || len(m.fileAutocompleteList) == 0 {
		return false
	}

	m.fileAutocompleteIdx += direction
	if m.fileAutocompleteIdx < 0 {
		m.fileAutocompleteIdx = len(m.fileAutocompleteList) - 1
	} else if m.fileAutocompleteIdx >= len(m.fileAutocompleteList) {
		m.fileAutocompleteIdx = 0
	}

	return true
}

// completeFileSelection insere arquivo selecionado
func (m *Model) completeFileSelection() bool {
	if !m.fileAutocomplete || len(m.fileAutocompleteList) == 0 {
		return false
	}

	selectedFile := m.fileAutocompleteList[m.fileAutocompleteIdx]
	value := m.textarea.Value()

	// Substitui @prefix pelo arquivo selecionado
	lastAt := strings.LastIndex(value, "@")
	if lastAt == -1 {
		return false
	}

	// Extrai parte antes do @
	beforeAt := value[:lastAt]

	// Encontra o fim do prefixo atual (até espaço ou fim da linha)
	afterAt := value[lastAt+1:]
	endOfPrefix := strings.IndexAny(afterAt, " \t\n")

	var afterPrefix string
	if endOfPrefix != -1 {
		// Preserva tudo após o prefixo
		afterPrefix = afterAt[endOfPrefix:]
	} else {
		// Não há texto depois, adiciona espaço
		afterPrefix = " "
	}

	// Monta novo valor preservando texto depois do arquivo
	newValue := beforeAt + "@" + selectedFile + afterPrefix
	m.textarea.SetValue(newValue)

	// Posiciona cursor logo após o arquivo selecionado
	cursorPos := len(beforeAt) + len("@") + len(selectedFile) + 1
	m.textarea.SetCursor(cursorPos)
	m.adjustTextareaHeight()

	// Desativa autocomplete
	m.fileAutocomplete = false
	m.fileAutocompleteList = nil

	return true
}

func (m *Model) Init() tea.Cmd {
	return tea.Batch(
		textarea.Blink,
		tickCmd(), // Inicia ticker para splash screen
	)
}

func tickCmd() tea.Cmd {
	return tea.Tick(100*time.Millisecond, func(t time.Time) tea.Msg {
		return tickMsg(t)
	})
}

func (m *Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var (
		tiCmd tea.Cmd
		vpCmd tea.Cmd
	)

	switch msg := msg.(type) {
	case spinner.TickMsg:
		// Atualiza spinner se estiver carregando
		var cmd tea.Cmd
		m.spinner, cmd = m.spinner.Update(msg)
		return m, cmd

	case tickMsg:
		m.tickCount++

		// SEGURANÇA: Timeout de segurança para evitar loops infinitos
		// 100 ticks = 10 segundos (já que cada tick é 100ms)
		if m.tickCount > 100 {
			// Se passou muito tempo, força o término do splash
			if m.showSplash {
				m.showSplash = false
				m.addMessage("")
				m.addMessage(m.theme.ErrorStyle.Render("⚠️  Splash screen expirou por timeout (10s)"))
				m.addMessage("")
				if m.debugMode {
					m.addMessage(m.theme.HelpStyle.Render("  DEBUG: Forçando término do splash após timeout"))
					m.addMessage("")
				}
			}
			// Reseta contador e para o loop de ticks
			m.tickCount = 0
			return m, nil
		}

		// Verifica se splash screen deve terminar (após 3 segundos)
		if m.showSplash && time.Since(m.splashStartTime) >= 3*time.Second {
			m.showSplash = false
			m.tickCount = 0 // Reseta contador após splash bem-sucedido
			m.finishSplash()
		}

		// Continua fazendo tick enquanto splash está ativa OU há tarefas ativas
		if m.showSplash || (m.taskTracker != nil && m.taskTracker.IsActive()) {
			return m, tickCmd()
		}

		// Para o loop de ticks e reseta contador
		m.tickCount = 0
		return m, nil

	case tea.KeyMsg:
		// Qualquer tecla encerra o splash. É a saída de emergência: se o
		// ticker que o encerra sozinho não rodar, o usuário não fica preso
		// numa tela de abertura sem nada para fazer.
		if m.showSplash {
			m.showSplash = false
			m.tickCount = 0
			m.finishSplash()
			return m, textarea.Blink
		}

		// Ctrl+R abre e fecha o raciocínio dos modelos que pensam.
		if msg.Type == tea.KeyCtrlR {
			m.toggleThinking()
			return m, nil
		}

		// Se agentCreator está ativo, delega input para ele
		if m.agentCreator != nil && m.agentCreator.IsActive() {
			cmd := m.agentCreator.Update(msg)
			return m, cmd
		}

		// Se autocomplete de arquivo está ativo, captura navegação
		if m.fileAutocomplete {
			switch msg.Type {
			case tea.KeyEsc:
				// Cancela autocomplete
				m.fileAutocomplete = false
				m.fileAutocompleteList = nil
				return m, nil

			case tea.KeyEnter, tea.KeyTab:
				// Seleciona arquivo
				if m.completeFileSelection() {
					return m, nil
				}

			case tea.KeyUp:
				m.handleFileAutocompleteNav(-1)
				return m, nil

			case tea.KeyDown:
				m.handleFileAutocompleteNav(1)
				return m, nil
			}
		}

		if m.interceptKey(msg) {
			return m, nil
		}

		m.textarea, tiCmd = m.textarea.Update(msg)

		// Detecta @ para autocomplete de arquivos
		if strings.Contains(m.textarea.Value(), "@") {
			m.detectFileAutocomplete()
		} else {
			m.fileAutocomplete = false
		}

		m.adjustTextareaHeight()

		switch msg.Type {
		case tea.KeyCtrlC, tea.KeyEsc:
			if m.fileAutocomplete {
				m.fileAutocomplete = false
				m.fileAutocompleteList = nil
				return m, nil
			}
			return m, tea.Quit

		case tea.KeyCtrlP:
			m.stepHistoryBackward()
			return m, nil

		case tea.KeyCtrlN:
			m.stepHistoryForward()
			return m, nil

		case tea.KeyEnter:
			// Se Alt+Enter (algumas TEs interpretam Shift+Enter como Alt+Enter), permite quebra de linha
			if msg.Alt {
				// Deixa o textarea processar normalmente (adiciona \n)
				return m, tea.Batch(tiCmd, vpCmd)
			}

			if m.loading {
				return m, nil
			}

			input := strings.TrimSpace(m.textarea.Value())
			if input == "" {
				return m, nil
			}

			if m.awaitingApproval {
				m.limpaCampoDeTexto()
				return m, tea.Batch(tiCmd, vpCmd, m.handleApprovalResponse(input))
			}

			m.commandHistory = append(m.commandHistory, input)
			m.historyIndex = 0

			m.limpaCampoDeTexto()

			if commands.Handle(input, m.commandDeps) {
				m.lastWasCommand = true
				return m, tea.Batch(tiCmd, vpCmd)
			}

			if strings.EqualFold(input, "sair") || strings.EqualFold(input, "exit") {
				return m, tea.Quit
			}

			if m.lastWasCommand {
				m.messages = []string{}
				m.viewport.SetContent("")
				m.lastWasCommand = false
			}

			if !m.firstMessageSent {
				m.showBanner = false
				m.firstMessageSent = true
			}

			processedInput, err := m.processFileReferences(input)
			if err != nil {
				m.addMessage(renderNotice("✗", fmt.Sprintf("Não consegui ler o arquivo: %v", err), colAlarm))
				m.addMessage("")
				return m, tea.Batch(tiCmd, vpCmd)
			}

			// Compacta texto longo (>5 linhas) para exibição
			displayInput := input
			lineCount := strings.Count(input, "\n") + 1
			if lineCount > 5 {
				lines := strings.Split(input, "\n")
				firstLines := strings.Join(lines[:3], "\n")
				charCount := len(input)
				displayInput = fmt.Sprintf("%s\n... [%d linhas, %d caracteres]", firstLines, lineCount, charCount)
			}

			// Renderiza mensagem do usuário com visual melhorado
			if m.ultraMode {
				userMsg := m.ultimateStyles.RenderGlowingUserMessage(displayInput, m.viewport.Width)
				m.addMessage(userMsg)
			} else {
				m.addMessage(renderUserEntry(displayInput, m.viewport.Width))
			}
			m.addMessage("")

			m.loading = true

			return m, tea.Batch(
				tiCmd,
				vpCmd,
				m.spinner.Tick,
				m.callAPI(processedInput),
			)

		case tea.KeyCtrlL:
			// Limpar tela completamente
			m.messages = []string{}
			m.viewport.SetContent("")
			m.viewport.GotoTop()
			// Não adiciona mensagem para manter tela realmente limpa
			return m, nil

		case tea.KeyCtrlD:
			// Sair (como terminal Unix)
			return m, tea.Quit

		case tea.KeyCtrlR:
			// Resetar conversa completamente
			m.messages = []string{}
			m.viewport.SetContent("")
			m.viewport.GotoTop()
			m.conversationHistory = m.conversationHistory[:1] // Mantém apenas system prompt
			m.toolRuns = 0
			m.inputTokens = 0
			m.outputTokens = 0
			m.totalTokens = 0
			m.addMessage(m.theme.InfoStyle.Render("🔄 Conversa e tokens resetados."))
			m.addMessage("")
		}

	case tea.MouseMsg:
		// Mouse wheel sempre controla o viewport
		if msg.Type == tea.MouseWheelUp || msg.Type == tea.MouseWheelDown {
			m.viewport, vpCmd = m.viewport.Update(msg)
			return m, tea.Batch(tiCmd, vpCmd)
		}
		// Outros eventos de mouse são ignorados
		return m, tea.Batch(tiCmd, vpCmd)

	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height

		// Layout compacto (Tododo/Gonzo style) - margem mínima
		headerHeight := 2 // Status bar apenas (reduzido de 3)
		if m.showBanner {
			headerHeight = 14
		}
		footerHeight := 3 // Input + shortcuts compactos (reduzido de 4)

		// Usa largura máxima (margem mínima de 2)
		newViewportWidth := msg.Width - 2
		if newViewportWidth < 60 {
			newViewportWidth = 60
		}

		// Maximiza altura do viewport (mais espaço)
		newViewportHeight := msg.Height - headerHeight - footerHeight
		if newViewportHeight < 10 {
			newViewportHeight = 10
		}

		m.viewport.Width = newViewportWidth
		m.viewport.Height = newViewportHeight
		// -6 = borda (2) + padding (2) da moldura + margem (2). Precisa casar
		// com o espaco interno de renderInputFrame: se sobrar, o lipgloss
		// quebra o texto de novo e a primeira linha some.
		//
		// Fica aqui, e nao no View: mexer no textarea durante a renderizacao
		// faz o quadro discordar do estado que ele mesmo acabou de mudar, e a
		// linha de cima era sobrescrita no quadro seguinte.
		m.textarea.SetWidth(m.larguraDoCampo())
		m.adjustTextareaHeight()

		// O markdown precisa ser reflowado na largura nova: com largura fixa
		// o texto estourava em terminais estreitos e deixava faixa morta nos
		// largos. -4 desconta a margem que o Glamour ja aplica.
		m.setGlamourWidth(newViewportWidth - 4)

	case ollamaChunkMsg:
		// Acumula o texto parcial e volta a esperar o proximo pedaco.
		// O raciocinio vai para uma previa separada: ele mostra que o modelo
		// esta trabalhando, mas nao faz parte da resposta.
		if msg.thinking {
			m.streamThinking += msg.text
		} else {
			m.streamPartial += msg.text
		}
		return m, waitForOllama(m.streamCh)

	case ollamaDoneMsg:
		// Guarda o raciocínio para exibi-lo colapsado junto da resposta.
		//
		// O tempo vem das métricas do Ollama, não do relógio da UI: as duas
		// fontes já discordaram em 80x na mesma resposta ("pensou 116m24s"
		// logo acima de "1m26s"). total_duration é o que o servidor realmente
		// gastou, e é a mesma fonte que a linha da resposta usa — assim os
		// dois números na tela não podem se contradizer. O relógio fica só
		// como reserva, para quando não houver métricas.
		m.lastThinking = m.streamThinking
		if msg.stats.Total > 0 {
			m.lastThinkingTime = msg.stats.Total
		} else {
			m.lastThinkingTime = time.Since(m.streamStart)
		}
		m.streamPartial = ""
		m.streamThinking = ""
		m.streamCh = nil
		m.cancelFunc = nil

		if msg.err != nil {
			m.metrics.RecordRequest(time.Since(m.streamStart), false, 0)
			return m.Update(apiResponseMsg{err: fmt.Errorf("Erro Ollama: %w", msg.err)})
		}

		m.metrics.RecordRequest(time.Since(m.streamStart), true, msg.stats.GenTokens)
		m.lastOllamaStats = msg.stats
		m.lastToolCalls = msg.toolCalls
		return m.Update(apiResponseMsg{response: msg.full})

	case apiResponseMsg:
		m.loading = false
		m.removePlaceholders()

		var nextCmd tea.Cmd
		if msg.err != nil {
			errorMsg := msg.err.Error()
			m.lastError = errorMsg
			m.addMessage(entry{
				rail:  colAlarm,
				label: "erro",
				body:  errorMsg + "\n\n/erro mostra os detalhes · /logs exporta",
				width: m.viewport.Width,
			}.render())
			m.addMessage("")
		} else {
			nextCmd = m.handleAIResponse(msg.response)

			// Se foi truncada e ainda há ciclos disponíveis, continua automaticamente
			if msg.wasTruncated && m.toolRuns < 50 {
				m.addMessage(m.theme.InfoStyle.Render("⚠️  Resposta truncada - continuando automaticamente..."))
				m.loading = true
				m.toolRuns++
				nextCmd = m.callAPI("Continue de onde parou.")
			}
		}

		if nextCmd != nil {
			return m, tea.Batch(tiCmd, vpCmd, m.spinner.Tick, nextCmd)
		}
		return m, tea.Batch(tiCmd, vpCmd)

	case error:
		m.err = msg
		return m, nil
	}

	return m, tea.Batch(tiCmd, vpCmd)
}

// estimateTokens estima quantidade de tokens (aproximadamente 1 token = 4 chars)
func estimateTokens(text string) int {
	return len(text) / 4
}

// preRenderedBlock guarda um bloco ja formatado (caixa de tarefas, resumo de
// tool) que NAO pode passar pelo reflow do Glamour.
var preRenderedRef = regexp.MustCompile(`AMZBLK([0-9]+)AMZ`)

// setGlamourWidth (re)cria o renderer do Glamour para a largura atual do
// terminal. Antes a largura era fixa em 110 colunas, entao o texto estourava
// em terminais estreitos e ficava com uma faixa morta nos largos.
func (m *Model) setGlamourWidth(width int) {
	if width < 40 {
		width = 40
	}
	if width > 200 {
		width = 200
	}
	if m.glamourRenderer != nil && m.glamourWidth == width {
		return
	}

	// NUNCA usar glamour.WithAutoStyle() aqui: ele chama
	// termenv.HasDarkBackground(), que escreve uma consulta OSC 11 no
	// terminal e espera a resposta pelo stdin. Como esta funcao roda dentro
	// do Update (no WindowSizeMsg), quem le o stdin e o Bubble Tea: ele
	// consome a resposta, o termenv fica esperando e a TUI congela no
	// primeiro quadro. O tema ja diz se o fundo e claro ou escuro.
	renderer, err := glamour.NewTermRenderer(
		glamour.WithStandardStyle(m.glamourStyleName()),
		glamour.WithWordWrap(width),
	)
	if err != nil {
		return
	}

	m.glamourRenderer = renderer
	m.glamourWidth = width
}

// renderMarkdown renderiza markdown com Glamour para visual moderno e elegante.
//
// Blocos ```tool e ```todo ja saem prontos (com caixa e alinhamento proprios).
// Se forem inseridos no markdown antes do Glamour, ele os trata como paragrafo
// comum: junta as linhas, reflowa no limite de coluna e corta as sequencias
// ANSI no meio (era a origem do lixo ";252m" e da caixa de tarefas quebrada).
// Por isso trocamos cada bloco por um marcador de uma palavra, renderizamos o
// markdown, e so entao devolvemos os blocos no lugar dos marcadores.
func (m *Model) renderMarkdown(text string) string {
	todoBlockRegex := regexp.MustCompile("(?s)```todo[ \t]*\r?\n(.*?)```")

	var blocks []string
	stash := func(rendered string) string {
		blocks = append(blocks, rendered)
		// Marcador sem espacos nem pontuacao: o Glamour o mantem em uma
		// linha propria e nao o quebra.
		return fmt.Sprintf("\n\nAMZBLK%dAMZ\n\n", len(blocks)-1)
	}

	text = todoBlockRegex.ReplaceAllStringFunc(text, func(match string) string {
		content := strings.TrimPrefix(strings.TrimSuffix(match, "```"), "```todo\n")
		return stash(formatTodoList(content))
	})

	// Chamadas de ferramenta viram um resumo de uma linha. A deteccao e a
	// mesma do parser: por conteudo, com ou sem cerca -- o usuario quer ver
	// a acao, nao o JSON.
	text = substituiChamadas(text, func(payload string) string {
		return stash(formatToolCallClean(payload))
	})

	rendered := ""
	if m.glamourRenderer != nil {
		var err error
		rendered, err = m.glamourRenderer.Render(text)
		if err != nil {
			rendered = ""
		}
	}
	if rendered == "" {
		// Fallback se o Glamour nao estiver disponivel ou falhar.
		rendered = highlightCode(text)
	}

	rendered = restorePreRendered(rendered, blocks)

	return strings.TrimSpace(rendered)
}

// restorePreRendered troca cada linha que contem um marcador pelo bloco
// original. Substituimos a LINHA inteira porque o Glamour a envolve em
// sequencias de cor e a preenche com espacos ate a margem direita.
func restorePreRendered(rendered string, blocks []string) string {
	if len(blocks) == 0 {
		return rendered
	}

	lines := strings.Split(rendered, "\n")
	out := make([]string, 0, len(lines))
	for _, line := range lines {
		match := preRenderedRef.FindStringSubmatch(ansi.Strip(line))
		if match == nil {
			out = append(out, line)
			continue
		}
		idx, err := strconv.Atoi(match[1])
		if err != nil || idx < 0 || idx >= len(blocks) {
			out = append(out, line)
			continue
		}
		out = append(out, strings.TrimRight(blocks[idx], "\n"))
	}

	return strings.Join(out, "\n")
}

// highlightCode aplica syntax highlighting em blocos de código
func highlightCode(text string) string {
	// Regex para detectar blocos ```linguagem\ncodigo\n```
	codeBlockRegex := regexp.MustCompile("(?s)```(\\w+)?\\n(.*?)```")

	result := codeBlockRegex.ReplaceAllStringFunc(text, func(match string) string {
		// Extrai linguagem e código
		parts := codeBlockRegex.FindStringSubmatch(match)
		if len(parts) < 3 {
			return match
		}

		language := parts[1]
		code := parts[2]

		// Se não especificou linguagem, tenta detectar
		if language == "" {
			language = "text"
		}

		// TRATAMENTO ESPECIAL PARA BLOCOS ```tool - visualização limpa
		if language == "tool" {
			return formatToolCallClean(code)
		}

		// TRATAMENTO ESPECIAL PARA BLOCOS ```todo - visualização de tarefas
		if language == "todo" {
			return formatTodoList(code)
		}

		// Aplica highlighting
		var highlighted strings.Builder
		err := quick.Highlight(&highlighted, code, language, "terminal256", "monokai")
		if err != nil {
			// Se falhar, retorna sem highlighting
			return match
		}

		return "```" + language + "\n" + highlighted.String() + "```"
	})

	return result
}

// formatTodoList desenha o plano do agente.
//
// Era uma caixa fechada, mas ela vive DENTRO do trilho da mensagem: caixa
// dentro de caixa, com duas molduras competindo pela mesma informação. Uma
// lista com marcadores de estado diz o mesmo com menos tinta, e acompanha a
// largura sem precisar de conta nenhuma.
func formatTodoList(todoText string) string {
	type item struct {
		glifo string
		texto string
		cor   lipgloss.Color
	}

	var itens []item
	feitos := 0

	for _, line := range strings.Split(todoText, "\n") {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" {
			continue
		}

		switch {
		case strings.HasPrefix(trimmed, "[x]"), strings.HasPrefix(trimmed, "[X]"):
			itens = append(itens, item{"✓", strings.TrimSpace(trimmed[3:]), colAct})
			feitos++
		case strings.HasPrefix(trimmed, "[-]"):
			itens = append(itens, item{"▶", strings.TrimSpace(trimmed[3:]), colEmber})
		case strings.HasPrefix(trimmed, "[ ]"):
			itens = append(itens, item{"·", strings.TrimSpace(trimmed[3:]), colGraphite})
		default:
			// Modelos pequenos escrevem "1." ou "-" em vez de "[ ]". A
			// intencao e a mesma: uma etapa pendente.
			if texto, ok := etapaSemMarcador(trimmed); ok {
				itens = append(itens, item{"·", texto, colGraphite})
				continue
			}
			itens = append(itens, item{" ", trimmed, colGraphite})
		}
	}

	if len(itens) == 0 {
		return ""
	}

	dim := lipgloss.NewStyle().Foreground(colGraphite)
	linhas := []string{dim.Render(fmt.Sprintf("plano · %d de %d", feitos, len(itens)))}

	for _, it := range itens {
		glifo := lipgloss.NewStyle().Foreground(it.cor).Render(it.glifo)
		texto := it.texto
		if it.glifo == "✓" {
			texto = dim.Render(texto) // concluído recua para o fundo
		}
		linhas = append(linhas, "  "+glifo+" "+texto)
	}

	return "\n" + strings.Join(linhas, "\n") + "\n"
}

// formatToolCallClean formata blocos ```tool de forma limpa e resumida
func formatToolCallClean(jsonStr string) string {
	return renderToolEvent(descreveTool(jsonStr))
}

// descreveTool traduz a chamada em uma frase curta no vocabulário do usuário:
// ele controla arquivos e comandos, não "tools".
func descreveTool(jsonStr string) string {
	// Usa a mesma normalizacao do parser: chamada nativa
	// ({"name":..., "arguments":{...}}) tambem precisa virar um resumo
	// legivel, senao toda acao aparece como "ferramenta usada".
	req, ok := requestDoPayload(jsonStr)
	if !ok {
		return "ferramenta usada"
	}

	toolReq := map[string]interface{}{
		"tool": req.Tool, "path": req.Path,
		"command": req.Command, "url": req.URL,
	}
	toolName := req.Tool

	// Retorna mensagem resumida baseada no tipo de ferramenta
	switch toolName {
	case "read_file":
		if path, ok := toolReq["path"].(string); ok {
			return "leu " + filepath.Base(path)
		}
		return "leu um arquivo"
	case "write_file":
		if path, ok := toolReq["path"].(string); ok {
			return "escreveu " + filepath.Base(path)
		}
		return "escreveu um arquivo"
	case "edit_file":
		if path, ok := toolReq["path"].(string); ok {
			return "editou " + filepath.Base(path)
		}
		return "editou um arquivo"
	case "list_dir":
		if path, ok := toolReq["path"].(string); ok {
			return "listou " + path
		}
		return "listou um diretório"
	case "make_dir":
		if path, ok := toolReq["path"].(string); ok {
			return "criou a pasta " + filepath.Base(path)
		}
		return "criou um diretório"
	case "execute_command":
		if cmd, ok := toolReq["command"].(string); ok {
			return "rodou: " + cmd
		}
		return "rodou um comando"
	case "search_files":
		return "procurou nos arquivos"
	case "web_fetch":
		if url, ok := toolReq["url"].(string); ok {
			return "buscou " + url
		}
		return "buscou conteúdo online"
	case "set_workspace":
		return "abriu a pasta " + req.Path
	default:
		return toolName
	}
}

func (m *Model) callAPI(input string) tea.Cmd {
	// Modelos locais (Ollama) respondem em streaming. A preparacao acontece
	// aqui, fora da goroutine do Cmd, porque grava estado no Model.
	if cfg := availableModels[m.currentModel]; isLocalModel(cfg) {
		m.conversationHistory = append(m.conversationHistory, agents.Message{
			Role:    "user",
			Content: input,
		})
		m.inputTokens += estimateTokens(input)
		m.streamPartial = ""
		m.streamThinking = ""
		m.streamStart = time.Now()
		return m.callOllamaStream(cfg.Endpoint)
	}

	// Modelos do Hugging Face exigem token; sem ele o erro da API e opaco.
	if strings.TrimSpace(m.hfToken) == "" {
		nome := availableModels[m.currentModel].DisplayName
		return func() tea.Msg {
			return apiResponseMsg{err: fmt.Errorf(
				"%s é um modelo remoto e precisa de um token do Hugging Face.\n"+
					"Use /key para configurar, ou /modelo para escolher um modelo local (🏠)", nome)}
		}
	}

	return func() tea.Msg {
		startTime := time.Now()
		modelConfig := availableModels[m.currentModel]
		modelName := modelConfig.Endpoint

		// Sempre adiciona a mensagem do usuário ao histórico primeiro
		m.conversationHistory = append(m.conversationHistory, agents.Message{
			Role:    "user",
			Content: input,
		})

		// Conta tokens de entrada
		inputTokenCount := estimateTokens(input)
		m.inputTokens += inputTokenCount

		// Verificar cache primeiro (apenas para modelos HF)
		if cachedResponse, found := m.cache.Get(input, modelName); found {
			m.metrics.RecordCacheHit()
			if m.debugMode {
				fmt.Fprintf(os.Stderr, "[DEBUG] Cache HIT para modelo %s\n", modelName)
			}
			return apiResponseMsg{response: cachedResponse}
		}
		m.metrics.RecordCacheMiss()

		trimmed := trimHistory(m.conversationHistory, 40) // Aumentado de 24 para 40 para mais contexto
		if len(trimmed) != len(m.conversationHistory) {
			m.conversationHistory = append([]agents.Message(nil), trimmed...)
		}

		// O historico guarda os turnos de ferramenta no formato nativo, que e o
		// que o Ollama entende. A API do Hugging Face nao aceita esse formato,
		// entao a traducao para texto acontece so na hora de enviar -- assim ha
		// uma unica representacao no historico.
		trimmed = achataMensagensDeFerramenta(trimmed)

		request := ChatCompletionRequest{
			Model:       modelName,
			Messages:    trimmed,
			Stream:      false,
			MaxTokens:   16384, // 16k tokens - máximo para evitar truncamento
			Temperature: 0.7,
			TopP:        0.9,
		}

		// Validar requisição
		if err := validateRequest(request); err != nil {
			m.metrics.RecordRequest(time.Since(startTime), false, 0)
			return apiResponseMsg{err: fmt.Errorf("requisição inválida: %w", err)}
		}

		requestBody, err := json.Marshal(request)
		if err != nil {
			m.metrics.RecordRequest(time.Since(startTime), false, 0)
			return apiResponseMsg{err: fmt.Errorf("erro ao converter requisição: %w", err)}
		}

		// Log de debug
		if m.debugMode {
			fmt.Fprintf(os.Stderr, "[DEBUG] REQUEST:\n%s\n", string(requestBody))
		}

		ctx, cancel := context.WithTimeout(context.Background(), 300*time.Second) // 5 minutos
		m.cancelFunc = cancel
		defer func() {
			m.cancelFunc = nil
		}()

		req, err := http.NewRequestWithContext(ctx, http.MethodPost, "https://router.huggingface.co/v1/chat/completions", strings.NewReader(string(requestBody)))
		if err != nil {
			m.metrics.RecordRequest(time.Since(startTime), false, 0)
			return apiResponseMsg{err: fmt.Errorf("erro ao criar requisição: %w", err)}
		}

		req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", m.hfToken))
		req.Header.Set("Content-Type", "application/json")

		// Cliente HTTP otimizado - FORÇA HTTP/1.1 (HTTP/2 causa timeouts)
		// Configuração agressiva para garantir HTTP/1.1
		transport := &http.Transport{
			MaxIdleConns:          100,
			MaxIdleConnsPerHost:   10,
			IdleConnTimeout:       120 * time.Second,
			ResponseHeaderTimeout: 120 * time.Second,
			ExpectContinueTimeout: 1 * time.Second,
			TLSHandshakeTimeout:   15 * time.Second,
			DisableKeepAlives:     false,
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
			return apiResponseMsg{err: fmt.Errorf("erro ao fazer requisição: %w", err)}
		}
		defer resp.Body.Close()

		body, err := io.ReadAll(resp.Body)
		if err != nil {
			m.metrics.RecordRequest(time.Since(startTime), false, 0)
			return apiResponseMsg{err: fmt.Errorf("erro ao ler resposta: %w", err)}
		}

		if resp.StatusCode != http.StatusOK {
			m.metrics.RecordRequest(time.Since(startTime), false, 0)
			errorMsg := cleanAPIError(resp.StatusCode, string(body))
			return apiResponseMsg{err: fmt.Errorf("%s", errorMsg)}
		}

		// Log de debug
		if m.debugMode {
			fmt.Fprintf(os.Stderr, "[DEBUG] RESPONSE:\n%s\n", string(body))
		}

		var completionResp ChatCompletionResponse
		if err := json.Unmarshal(body, &completionResp); err != nil {
			m.metrics.RecordRequest(time.Since(startTime), false, 0)
			return apiResponseMsg{err: fmt.Errorf("erro ao parsear resposta: %w", err)}
		}

		// Validar resposta
		if err := validateAPIResponse(completionResp); err != nil {
			m.metrics.RecordRequest(time.Since(startTime), false, 0)
			return apiResponseMsg{err: err}
		}

		if len(completionResp.Choices) == 0 {
			m.metrics.RecordRequest(time.Since(startTime), false, 0)
			return apiResponseMsg{err: fmt.Errorf("nenhuma resposta recebida da API")}
		}

		response := completionResp.Choices[0].Message.Content
		finishReason := completionResp.Choices[0].FinishReason

		// Atualizar tokens com dados reais da API se disponíveis
		tokensUsed := estimateTokens(response)
		if completionResp.Usage.TotalTokens > 0 {
			tokensUsed = completionResp.Usage.TotalTokens
			// Atualizar contadores com dados precisos da API
			m.inputTokens = completionResp.Usage.PromptTokens
			m.outputTokens = completionResp.Usage.CompletionTokens
		}

		// Registrar métricas
		m.metrics.RecordRequest(time.Since(startTime), true, tokensUsed)

		// Armazenar no cache (apenas se completo)
		if finishReason == "stop" || finishReason == "" {
			m.cache.Set(input, modelName, response)
		}

		return apiResponseMsg{response: response, wasTruncated: finishReason == "length"}
	}
}

func trimHistory(history []agents.Message, maxMessages int) []agents.Message {
	if len(history) <= maxMessages {
		return history
	}
	return append([]agents.Message{history[0]}, history[len(history)-maxMessages+1:]...)
}

func (m *Model) handleAIResponse(response string) tea.Cmd {
	// Modelos com raciocinio (lfm2.5, deepseek-r1) podem consumir todo o
	// num_predict pensando e devolver content vazio. Renderizar a moldura
	// vazia nao diz nada; explique o que houve e como resolver.
	// Uma resposta so com tool_calls tem content vazio -- isso e trabalho
	// normal, nao "o modelo terminou sem produzir resposta". Tratar como vazia
	// disparava um pedido de repeticao e jogava a chamada no lixo.
	if strings.TrimSpace(response) == "" && len(m.lastToolCalls) == 0 {
		// Modelos com raciocinio as vezes encerram o turno depois de pensar,
		// sem escrever nada. Uma segunda tentativa costuma resolver e custa
		// pouco: o cache de prefixo do Ollama ja esta quente.
		//
		// O gatilho e "gastou tokens e nao entregou nada", nao "capturamos o
		// raciocinio". Medido com lfm2.5:8b: no meio de uma tarefa longa ele
		// devolve UMA linha com done_reason "stop", eval_count 678 e content
		// vazio -- sem campo thinking, sem tool_calls, sem nada. Exigir
		// thinking nao-vazio fazia a tentativa nao acontecer justamente no
		// caso pior, e a tarefa morria ali. O mesmo historico, repetido,
		// produziu chamada de ferramenta duas de tres vezes: e variancia do
		// modelo, e pedir de novo e a resposta certa.
		gastouSemEntregar := m.lastOllamaStats.GenTokens > 0
		if (strings.TrimSpace(m.lastThinking) != "" || gastouSemEntregar) && m.emptyRetries < 1 {
			m.emptyRetries++
			m.lastThinking = ""
			m.loading = true
			m.addMessage(renderNotice("↻",
				"O modelo gastou o turno sem escrever nada. Pedindo de novo…", colGraphite))
			return m.callAPI("Continue a tarefa do ponto onde parou. " +
				"Emita a próxima chamada de ferramenta, ou escreva a resposta final.")
		}
		m.emptyRetries = 0

		m.addMessage(m.theme.ErrorStyle.Render("⚠️  O modelo terminou sem produzir resposta."))

		st := m.lastOllamaStats
		teto := agents.DefaultOllamaOptions().NumPredict

		switch {
		case teto > 0 && st.GenTokens >= teto:
			// So faz sentido falar em teto quando existe um.
			m.addMessage(m.theme.HelpStyle.Render(fmt.Sprintf(
				"   Gastou os %d tokens do limite em raciocínio interno.", st.GenTokens)))
			m.addMessage(m.theme.HelpStyle.Render(fmt.Sprintf(
				"   💡 Aumente o limite: AMAZZO_OLLAMA_NUM_PREDICT=%d amazzo", teto*2)))
		case st.GenTokens > 0:
			// Parou sozinho bem antes do teto: aumentar num_predict nao ajuda.
			m.addMessage(m.theme.HelpStyle.Render(fmt.Sprintf(
				"   Gerou apenas %d tokens, todos em raciocínio interno, e parou.", st.GenTokens)))
			m.addMessage(m.theme.HelpStyle.Render(
				"   💡 Reformule o pedido de forma mais direta, ou troque de modelo com /modelo."))
		default:
			m.addMessage(m.theme.HelpStyle.Render(
				"   💡 Tente novamente ou troque de modelo com /modelo."))
		}

		m.addMessage("")
		m.toolRuns = 0
		return nil
	}

	m.emptyRetries = 0
	m.prevAIResponse = m.lastAIResponse
	m.lastAIResponse = response

	// Conta tokens de saída
	outputTokenCount := estimateTokens(response)
	m.outputTokens += outputTokenCount
	m.totalTokens = m.inputTokens + m.outputTokens

	// Renderiza markdown com Glamour para visual moderno e elegante
	renderedResponse := m.renderMarkdown(response)

	// Renderiza mensagem do assistente com visual melhorado
	if m.ultraMode {
		assistantMsg := m.ultimateStyles.RenderGlowingAssistantMessage(renderedResponse, m.viewport.Width)
		m.addMessage(assistantMsg)
	} else {
		modelName := availableModels[m.currentModel].DisplayName

		// O raciocinio entra antes da resposta, colapsado por padrao.
		if strings.TrimSpace(m.lastThinking) != "" {
			m.thoughts = append(m.thoughts, thought{
				index:   len(m.messages),
				text:    strings.TrimSpace(m.lastThinking),
				elapsed: m.lastThinkingTime,
			})
			m.addMessage(renderThought(strings.TrimSpace(m.lastThinking),
				m.lastThinkingTime, m.showThinking, m.viewport.Width))
			m.lastThinking = ""
		}

		m.addMessage(renderAssistantEntry(modelName, m.responseMeta(),
			renderedResponse, m.viewport.Width))
	}
	m.addMessage("")

	chamadasNativas := m.lastToolCalls
	m.lastToolCalls = nil

	m.conversationHistory = append(m.conversationHistory, agents.Message{
		Role:      "assistant",
		Content:   response,
		ToolCalls: chamadasNativas,
	})

	// A chamada pode vir pelo canal nativo ou como JSON no texto. Normalizar as
	// nativas para o mesmo formato deixa um caminho unico daqui para frente.
	textoDasNativas := textoDasToolCalls(chamadasNativas)
	if requests := m.parseToolRequests(response + textoDasNativas); len(requests) > 0 && m.toolRuns < 15 {
		m.toolRuns++

		// A chamada nativa nao esta no texto da resposta, entao o
		// renderMarkdown nao a transformou na linha "listou .". Sem isso voce
		// ve o resultado da acao sem ver qual acao foi pedida.
		m.mostraChamadasNativas(chamadasNativas)
		m.toolCallsTotal += len(requests)
		resultados, pending := m.executeToolRequests(requests)
		if pending {
			return nil
		}
		if len(resultados) == 0 {
			m.toolRuns = 0
			return nil
		}
		m.loading = true

		// Feedback de progresso
		progressMsg := fmt.Sprintf("🔁 Executando ferramentas (%d/%d ciclos)...", m.toolRuns, 15)
		if m.toolRuns >= 12 {
			progressMsg += " ⚠️ Aproximando do limite"
		}
		m.addMessage(m.theme.InfoStyle.Render(progressMsg))

		return tea.Batch(m.spinner.Tick, m.continuaAposFerramentas(resultados))
	}

	// 🚨 STOP HOOK: Verifica se há tarefas pendentes antes de parar
	if m.toolRuns < 15 {
		incompleteTasks := m.parseIncompleteTasks(response)
		if len(incompleteTasks) > 0 {
			// DETECÇÃO de LOOP: se o agente repete uma resposta recente, ele
			// nao esta progredindo e continuar so enche a tela.
			if m.respostaRepetida(response) {
				m.addMessage(m.theme.ErrorStyle.Render("⚠️  Loop detectado: o agente repetiu uma resposta anterior sem avançar. Parando."))
				m.addMessage(m.theme.HelpStyle.Render("   💡 Modelos pequenos travam assim quando a tarefa é ampla demais. Tente pedir uma etapa por vez, ou troque de modelo com /modelo."))
				m.addMessage("")
				m.toolRuns = 0
				m.recentResponses = nil
				return nil
			}
			m.toolRuns++
			m.loading = true

			// Força continuação com mensagem explícita
			continuePrompt := fmt.Sprintf(`⚠️  ATENÇÃO: Você não completou todas as tarefas!

Tarefas ainda pendentes:
%s

**VOCÊ DEVE CONTINUAR TRABALHANDO!**

Pegue a próxima tarefa pendente [ ] e:
1. Marque ela como [-] (em progresso)
2. Execute a tarefa completamente usando as ferramentas necessárias
3. Marque como [x] quando concluir
4. Continue para a próxima até TODAS estarem [x]

Continue agora com a próxima tarefa pendente.`, strings.Join(incompleteTasks, "\n"))

			m.addMessage(m.theme.InfoStyle.Render("🔄 Continuando com tarefas pendentes..."))
			return tea.Batch(m.spinner.Tick, m.callAPI(continuePrompt))
		}
	}

	// Se atingiu limite, avisa usuário
	if m.toolRuns >= 15 {
		m.addMessage(m.theme.InfoStyle.Render("⚠️ Limite de 15 ciclos de ferramentas atingido"))
		m.addMessage(m.theme.HelpStyle.Render("💡 Se a tarefa não está completa, continue pedindo:"))
		m.addMessage(m.theme.HelpStyle.Render("   'continue de onde parou' ou 'complete a tarefa'"))
	}

	m.toolRuns = 0
	return nil
}

// mostraChamadasNativas exibe o que o modelo pediu, no vocabulario do usuario.
func (m *Model) mostraChamadasNativas(chamadas []agents.ToolCall) {
	for _, c := range chamadas {
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
		m.addMessage(formatToolCallClean(string(bruto)))
	}
}

func (m *Model) parseIncompleteTasks(response string) []string {
	var incompleteTasks []string

	lines := strings.Split(response, "\n")
	inTodoBlock := false

	for _, line := range lines {
		trimmed := strings.TrimSpace(line)

		// Detecta início de bloco todo
		if strings.HasPrefix(trimmed, "```todo") {
			inTodoBlock = true
			continue
		}

		// Detecta fim de bloco
		if inTodoBlock && strings.HasPrefix(trimmed, "```") {
			inTodoBlock = false
			continue
		}

		// Dentro do bloco, procura por tarefas pendentes
		if inTodoBlock {
			// Tarefas pendentes: [ ] ou [-]
			if strings.HasPrefix(trimmed, "[ ]") || strings.HasPrefix(trimmed, "[-]") {
				incompleteTasks = append(incompleteTasks, trimmed)
			}
		} else {
			// Também detecta fora de blocos (formato markdown comum)
			if strings.HasPrefix(trimmed, "- [ ]") || strings.HasPrefix(trimmed, "- [-]") {
				incompleteTasks = append(incompleteTasks, trimmed)
			}
		}
	}

	return incompleteTasks
}

func (m *Model) parseToolRequests(response string) []mcp.Request {
	var requests []mcp.Request

	for _, payload := range findToolPayloads(response) {
		req, ok := requestDoPayload(payload)
		if !ok {
			continue
		}
		requests = append(requests, req)
	}

	return requests
}

// findToolPayloads acha as chamadas de ferramenta no texto do modelo.
//
// Isto e a rede de seguranca para quando o modelo NAO usa o canal nativo de
// tool_calls -- o que acontece com frequencia em modelo pequeno, sobretudo
// quando a conversa cresce. O que identifica uma chamada e o conteudo: um
// objeto JSON cujo nome de ferramenta o Amazzo sabe executar.
//
// Medimos com o lfm2.5:8b que ele emite, alem do bloco ```tool pedido no
// prompt: array de chamadas, array no formato nativo, envelope "tool_calls",
// JSON inline depois de prosa e item de lista markdown. Exigir que o '{'
// abrisse uma linha descartava todos esses -- a chamada virava texto na tela e
// a tarefa parava.
//
// Validar o nome contra as ferramentas suportadas e o que permite varrer o
// texto inteiro sem falso positivo: um package.json exibido na resposta tem
// "name", mas "meu-projeto" nao e uma ferramenta.
func findToolPayloads(text string) []string {
	var found []string

	for i := 0; i < len(text); i++ {
		if text[i] != '{' && text[i] != '[' {
			continue
		}

		dec := json.NewDecoder(strings.NewReader(text[i:]))
		var bruto interface{}
		if err := dec.Decode(&bruto); err != nil {
			continue
		}

		chamadas := chamadasEm(bruto)
		if len(chamadas) == 0 {
			continue
		}

		for _, c := range chamadas {
			if serializado, err := json.Marshal(c); err == nil {
				found = append(found, string(serializado))
			}
		}

		// Pula o valor inteiro para nao reprocessar objetos aninhados.
		i += int(dec.InputOffset()) - 1
	}

	return found
}

// chamadasEm extrai as chamadas de um valor JSON decodificado, atravessando os
// envelopes que os modelos usam: array de chamadas, {"tool_calls": [...]} e
// {"function": {...}}.
func chamadasEm(valor interface{}) []map[string]interface{} {
	switch v := valor.(type) {
	case []interface{}:
		var todas []map[string]interface{}
		for _, item := range v {
			todas = append(todas, chamadasEm(item)...)
		}
		return todas

	case map[string]interface{}:
		if _, ok := nomeDaFerramenta(v); ok {
			return []map[string]interface{}{v}
		}
		for _, chave := range []string{"tool_calls", "calls", "actions"} {
			if aninhado, existe := v[chave]; existe {
				if todas := chamadasEm(aninhado); len(todas) > 0 {
					return todas
				}
			}
		}
	}

	return nil
}

// executeToolRequests roda o lote e devolve os resultados.
//
// Devolve os Results, e nao um JSON serializado, porque cada resultado precisa
// virar uma mensagem role "tool" separada -- o formato que o template de chat
// do modelo entende.
func (m *Model) executeToolRequests(requests []mcp.Request) ([]mcp.Result, bool) {
	if len(requests) == 0 {
		return nil, false
	}

	immediate, privileged := m.splitRequests(requests)

	if len(privileged) > 0 {
		m.pendingRequests = append([]mcp.Request(nil), privileged...)
		m.awaitingApproval = true
		m.pendingResults = nil

		if len(immediate) > 0 {
			results := m.runMCPRequests(immediate)
			m.pendingResults = append(m.pendingResults, results...)
		}

		m.addMessage(m.theme.HelpStyle.Render("⚠️ Operações MCP aguardando confirmação:"))
		m.addMessage(m.describePendingRequests(privileged))
		m.addMessage(m.theme.HelpStyle.Render("Digite 'permitir' para continuar ou 'negar' para cancelar."))
		m.addMessage("")
		return nil, true
	}

	return m.runMCPRequests(immediate), false
}

func (m *Model) splitRequests(requests []mcp.Request) (immediate []mcp.Request, privileged []mcp.Request) {
	for _, req := range requests {
		tool := strings.ToLower(req.Tool)

		// Cria chave única para rastrear esta requisição específica
		reqKey := m.getRequestKey(req)

		// Verifica se esta requisição foi negada recentemente (últimos 30 segundos)
		if deniedTime, wasDenied := m.deniedRequests[reqKey]; wasDenied {
			if time.Since(deniedTime) < 30*time.Second {
				// Foi negada recentemente - trata como negada automaticamente
				m.addMessage(m.theme.HelpStyle.Render(fmt.Sprintf("⏭️  Pulando requisição negada recentemente: %s %s", req.Tool, req.Path)))
				continue // Pula esta requisição
			} else {
				// Já passou tempo suficiente, remove da lista de negadas
				delete(m.deniedRequests, reqKey)
			}
		}

		// Verifica se o gerenciador de permissões permite automaticamente
		if m.permManager.ShouldAllow(tool) {
			immediate = append(immediate, req)
		} else if m.permManager.ShouldDeny(tool) {
			// Se deveria negar, adiciona como pendente para mostrar que foi negado
			privileged = append(privileged, req)
		} else if m.requiresApproval(req) {
			// Se precisa de aprovação e não tem permissão automática, pergunta
			privileged = append(privileged, req)
		} else {
			// Operações de leitura não precisam de aprovação
			immediate = append(immediate, req)
		}
	}
	return
}

func (m *Model) requiresApproval(req mcp.Request) bool {
	tool := strings.ToLower(req.Tool)

	// Se o gerenciador diz para pular a pergunta, não precisa de aprovação
	if !m.permManager.ShouldAsk(tool) {
		return false
	}

	// Lista negra, não lista branca: por padrão a ferramenta roda. Só paramos
	// no que é irreversível, no que conecta esta máquina a outra, no que abre
	// uma porta aqui, no que pede elevação de privilégio e no que instala
	// software.
	//
	// Confirmar o tempo todo não protege ninguém: vira reflexo de apertar
	// "permitir" sem ler, e aí a confirmação que importa passa despercebida.
	// Buscar, editar, compilar e testar são trabalho normal e rodam direto.
	return m.motivoDaConfirmacao(req) != ""
}

// motivoDaConfirmacao devolve por que este pedido precisa de autorização, ou
// "" quando pode rodar direto. O motivo aparece na tela: "apaga arquivos" e
// "conecta a outra máquina" merecem leituras diferentes.
func (m *Model) motivoDaConfirmacao(req mcp.Request) string {
	switch strings.ToLower(req.Tool) {
	case "delete_file", "remove_file":
		return "apaga um arquivo"

	case "move_file", "mv", "rename":
		return "move e pode sobrescrever"

	case "execute_command", "exec", "run":
		return mcp.MotivoDeConfirmacao(req.Command)

	default:
		// Ler, escrever, editar, criar pasta, buscar, abrir projeto.
		return ""
	}
}

// getRequestKey gera uma chave única para rastrear requisições MCP
func (m *Model) getRequestKey(req mcp.Request) string {
	// Combina tool + path para criar uma chave única
	// Se houver content, inclui um hash dele para diferenciar requisições
	key := fmt.Sprintf("%s:%s", strings.ToLower(req.Tool), req.Path)
	if req.Content != "" {
		// Adiciona os primeiros 50 caracteres do conteúdo para diferenciar
		contentPreview := req.Content
		if len(contentPreview) > 50 {
			contentPreview = contentPreview[:50]
		}
		key = fmt.Sprintf("%s:%s", key, contentPreview)
	}
	return key
}

func (m *Model) runMCPRequests(requests []mcp.Request) []mcp.Result {
	if len(requests) == 0 {
		return nil
	}

	// set_workspace muda o alcance do agente, não um arquivo: é decisão do
	// app, não do servidor MCP (que só enxerga dentro do workspace atual).
	requests, abriuPasta := m.aplicaAberturaDePasta(requests)
	if len(requests) == 0 {
		return abriuPasta
	}

	// O lote roda em ordem, entao o teto dele precisa comportar a soma dos
	// passos -- nunca ser menor que o teto de um comando isolado (120s).
	// Medido nesta maquina: "go build ./..." a frio leva 31s neste projeto, e
	// "go test ./..." mais. Com 60s um lote de build+teste era cancelado no
	// meio, e o modelo recebia "execucao cancelada" sem entender por que.
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()

	results, err := m.mcpServer.Handle(ctx, requests)
	results = append(abriuPasta, results...)
	if err != nil && !errors.Is(err, context.Canceled) {
		m.addMessage(m.theme.ErrorStyle.Render(fmt.Sprintf("⚠️ Erro ao executar MCP: %v", err)))
		m.addMessage("")
	}

	for _, res := range results {
		m.renderToolResult(res)
	}

	return results
}

func (m *Model) renderToolResult(res mcp.Result) {
	if res.Status != "success" {
		m.addMessage(m.theme.ErrorStyle.Render(fmt.Sprintf("❌ %s (%s): %s", strings.ToLower(res.Tool), res.Path, res.Error)))
		m.addMessage("")
		return
	}

	tool := strings.ToLower(res.Tool)
	switch {
	case strings.Contains(tool, "read"):
		// Não exibe conteúdo completo - apenas confirmação
		lines := strings.Count(res.Content, "\n")
		size := len(res.Content)
		m.addMessage(m.theme.InfoStyle.Render(fmt.Sprintf("✅ %s lido (%d linhas, %d bytes)", res.Path, lines, size)))
	case strings.Contains(tool, "list"):
		// Compacta listagem de diretórios
		lines := strings.Split(res.Content, "\n")
		var items []string
		for _, line := range lines {
			line = strings.TrimSpace(line)
			if strings.HasPrefix(line, "- ") {
				items = append(items, strings.TrimPrefix(line, "- "))
			}
		}

		if len(items) > 10 {
			// Se muitos itens, mostra apenas contagem + primeiros/últimos
			preview := strings.Join(items[:5], ", ")
			lastPreview := strings.Join(items[len(items)-3:], ", ")
			m.addMessage(m.theme.InfoStyle.Render(fmt.Sprintf("📁 %s (%d itens)", res.Path, len(items))))
			m.addMessage(m.theme.HelpStyle.Render(fmt.Sprintf("Primeiros: %s", preview)))
			m.addMessage(m.theme.HelpStyle.Render(fmt.Sprintf("... e mais %d itens ...", len(items)-8)))
			m.addMessage(m.theme.HelpStyle.Render(fmt.Sprintf("Últimos: %s", lastPreview)))
		} else if len(items) > 0 {
			// Se poucos itens, mostra inline
			m.addMessage(m.theme.InfoStyle.Render(fmt.Sprintf("📁 %s (%d itens): %s", res.Path, len(items), strings.Join(items, ", "))))
		} else {
			m.addMessage(res.Content)
		}
	case strings.Contains(tool, "edit"):
		// Para edit_file, mostra um diff visual
		m.addMessage(m.theme.InfoStyle.Render(fmt.Sprintf("✏️  %s", res.Content)))
		// Se temos informações de search/replace, mostra diff
		if res.Error == "" {
			m.addMessage(m.theme.HelpStyle.Render("  💡 Use /logs para ver detalhes completos"))
		}
	case strings.Contains(tool, "write"), strings.Contains(tool, "append"):
		m.addMessage(m.theme.InfoStyle.Render(fmt.Sprintf("✅ %s", res.Content)))
	case strings.Contains(tool, "move"):
		m.addMessage(m.theme.InfoStyle.Render(fmt.Sprintf("📦 %s", res.Content)))
	case strings.Contains(tool, "make") || strings.Contains(tool, "dir"):
		m.addMessage(m.theme.InfoStyle.Render(fmt.Sprintf("📁 %s", res.Content)))
	case strings.Contains(tool, "search"):
		// Para search_files, limita o output
		lines := strings.Split(res.Content, "\n")
		if len(lines) > 20 {
			preview := strings.Join(lines[:20], "\n")
			m.addMessage(m.theme.InfoStyle.Render(fmt.Sprintf("🔍 Encontrados %d resultados (exibindo 20):", len(lines))))
			m.addMessage(preview)
			m.addMessage(m.theme.HelpStyle.Render(fmt.Sprintf("... e mais %d resultados", len(lines)-20)))
		} else {
			m.addMessage(res.Content)
		}
	case strings.Contains(tool, "execute"):
		// Para execute_command, limita o output
		if len(res.Content) > 1000 {
			m.addMessage(m.theme.InfoStyle.Render(fmt.Sprintf("✅ %s", res.Content[:1000])))
			m.addMessage(m.theme.HelpStyle.Render(fmt.Sprintf("... (%d bytes omitidos)", len(res.Content)-1000)))
		} else {
			m.addMessage(m.theme.InfoStyle.Render(res.Content))
		}
	case strings.Contains(tool, "web_fetch"), strings.Contains(tool, "fetch"):
		// Para web_fetch, mostra apenas resumo curto ao usuário (modelo recebe conteúdo completo)
		lines := strings.Split(res.Content, "\n")
		contentSize := len(res.Content)

		// Pega apenas as primeiras linhas úteis (skip cabeçalho)
		var preview []string
		for i, line := range lines {
			if i > 50 { // Máximo 50 linhas de preview
				break
			}
			line = strings.TrimSpace(line)
			if line != "" && len(line) > 10 { // Ignora linhas muito curtas
				preview = append(preview, line)
				if len(preview) >= 10 { // Mostra só 10 linhas úteis
					break
				}
			}
		}

		m.addMessage(m.theme.InfoStyle.Render("🌐 Conteúdo web obtido com sucesso"))
		m.addMessage(m.theme.HelpStyle.Render(fmt.Sprintf("   Tamanho: %d caracteres", contentSize)))
		if len(preview) > 0 {
			m.addMessage(m.theme.HelpStyle.Render("   Preview:"))
			for _, line := range preview {
				if len(line) > 100 {
					line = line[:100] + "..."
				}
				m.addMessage(m.theme.HelpStyle.Render("   " + line))
			}
		}
		m.addMessage(m.theme.HelpStyle.Render("   💡 O modelo recebeu o conteúdo completo"))
	default:
		m.addMessage(m.theme.InfoStyle.Render(res.Content))
	}
	m.addMessage("")
}

// describePendingRequests descreve o que será feito se você permitir.
//
// Mostrava só req.Path, que é vazio para execute_command: o comando ficava
// invisível e a confirmação virava um "sim" no escuro. Você precisa ver
// exatamente o que vai acontecer antes de autorizar.
func (m *Model) describePendingRequests(requests []mcp.Request) string {
	var builder strings.Builder

	for i, req := range requests {
		builder.WriteString(fmt.Sprintf(" %d) %s\n", i+1, descrevePedido(req)))
	}

	return strings.TrimSuffix(builder.String(), "\n")
}

// descrevePedido traduz uma operação para uma frase legível.
func descrevePedido(req mcp.Request) string {
	descricao := descreveAcao(req)
	if motivo := motivoDoPedido(req); motivo != "" {
		descricao += "  — " + motivo
	}
	return descricao
}

// motivoDoPedido repete a regra de motivoDaConfirmacao sem depender do Model,
// para a descrição poder ser montada em qualquer lugar.
func motivoDoPedido(req mcp.Request) string {
	switch strings.ToLower(req.Tool) {
	case "delete_file", "remove_file":
		return "apaga um arquivo"
	case "move_file", "mv", "rename":
		return "move e pode sobrescrever"
	case "execute_command", "exec", "run":
		return mcp.MotivoDeConfirmacao(req.Command)
	}
	return ""
}

func descreveAcao(req mcp.Request) string {
	switch req.Tool {
	case "execute_command":
		return "rodar: " + req.Command
	case "write_file":
		return fmt.Sprintf("escrever %s (%d caracteres)", req.Path, len(req.Content))
	case "edit_file":
		return fmt.Sprintf("editar %s", req.Path)
	case "make_dir":
		return "criar a pasta " + req.Path
	case "set_workspace":
		return "abrir a pasta de trabalho em " + req.Path
	case "delete_file", "remove_file":
		return "APAGAR " + req.Path
	default:
		if req.Path != "" {
			return req.Tool + " em " + req.Path
		}
		return req.Tool
	}
}

func (m *Model) handleApprovalResponse(input string) tea.Cmd {
	if !m.awaitingApproval {
		return nil
	}

	switch strings.ToLower(strings.TrimSpace(input)) {
	case "permitir", "/permitir":
		return m.processPendingRequests(true)
	case "negar", "/negar":
		return m.processPendingRequests(false)
	default:
		m.addMessage(m.theme.ErrorStyle.Render("⚠️ Responda com 'permitir' ou 'negar' para continuar."))
		m.addMessage("")
		return nil
	}
}

func (m *Model) processPendingRequests(approved bool) tea.Cmd {
	defer func() {
		m.awaitingApproval = false
		m.pendingRequests = nil
	}()

	if len(m.pendingRequests) == 0 && len(m.pendingResults) == 0 {
		return nil
	}

	if approved {
		m.addMessage(m.theme.InfoStyle.Render("🔐 Permissão concedida. Executando operações pendentes..."))
		results := m.runMCPRequests(m.pendingRequests)
		m.pendingResults = append(m.pendingResults, results...)
	} else {
		m.addMessage(m.theme.ErrorStyle.Render("⛔ Operações MCP negadas pelo usuário."))
		// Registra todas as requisições negadas para evitar loop infinito
		now := time.Now()
		for _, req := range m.pendingRequests {
			reqKey := m.getRequestKey(req)
			m.deniedRequests[reqKey] = now
			m.pendingResults = append(m.pendingResults, mcp.Result{
				Tool:   req.Tool,
				Path:   req.Path,
				Status: "error",
				Error:  "requisição negada pelo usuário",
			})
		}
		m.addMessage(m.theme.HelpStyle.Render("💡 Estas operações serão automaticamente puladas por 30 segundos."))
		m.addMessage("")
	}

	if len(m.pendingResults) == 0 {
		return nil
	}

	resultados := m.pendingResults
	m.pendingResults = nil
	m.loading = true
	return m.continuaAposFerramentas(resultados)
}

// registraResultadosDeFerramenta grava os resultados no historico como turnos
// de ferramenta -- role "tool", texto puro, um por resultado.
//
// O caminho antigo mandava o array de Results em JSON escapado dentro de uma
// mensagem role "user". Medido com lfm2.5:8b sobre a mesma listagem de 30
// entradas: como "user" o modelo devolveu 22, omitindo agents/ e mcp/ (ele
// parafraseia o JSON); como "tool" devolveu as 30 na ordem exata, com 54
// tokens de prompt a menos.
func (m *Model) registraResultadosDeFerramenta(resultados []mcp.Result) {
	resultados = m.marcaRepeticoes(resultados)
	msgs := mensagensDeFerramenta(resultados)
	m.conversationHistory = append(m.conversationHistory, msgs...)

	for _, msg := range msgs {
		m.inputTokens += estimateTokens(msg.Content)
	}
}

// marcaRepeticoes avisa o modelo quando ele repete uma chamada que ja deu o
// mesmo resultado.
//
// Medido com lfm2.5:8b num pedido amplo: ele chamou "list_dir docs" no ciclo 3
// e de novo no ciclo 6, com o mesmo resultado, e depois desistiu sem escrever o
// arquivo. Cada ciclo desses custa 20-60s nesta CPU. O resultado identico nao
// carrega informacao nova nenhuma -- dizer isso em texto e o que o faz seguir
// adiante em vez de girar.
func (m *Model) marcaRepeticoes(resultados []mcp.Result) []mcp.Result {
	if m.chamadasFeitas == nil {
		m.chamadasFeitas = make(map[string]string)
	}

	marcados := make([]mcp.Result, 0, len(resultados))
	for _, res := range resultados {
		// filepath.Clean normaliza o caminho: medimos o modelo chamando
		// "list_dir ./docs" e depois "list_dir ./docs/" -- mesmo diretorio,
		// texto diferente, e a repeticao passava batida.
		assinatura := strings.ToLower(res.Tool) + "\x00" + filepath.Clean(res.Path)
		resposta := res.Status + "\x00" + res.Error + "\x00" + res.Content

		if anterior, jaFez := m.chamadasFeitas[assinatura]; jaFez && anterior == resposta {
			aviso := "[Você já fez esta chamada antes e o resultado foi exatamente o " +
				"mesmo. Não repita: use o que já sabe e siga para o próximo passo da tarefa.]\n"
			if res.Status == "success" {
				res.Content = aviso + res.Content
			} else {
				res.Error = aviso + res.Error
			}
			m.addMessage(m.theme.HelpStyle.Render(
				"↺ chamada repetida (" + strings.ToLower(res.Tool) + " " + res.Path + ") — avisando o modelo"))
		}

		m.chamadasFeitas[assinatura] = resposta
		marcados = append(marcados, res)
	}

	return marcados
}

// continuaAposFerramentas devolve o turno ao modelo com os resultados no
// historico, sem inventar uma mensagem de usuario.
//
// Antes o resultado voltava como um pedido novo ("Resultado das ferramentas:
// ..."), o que fazia o modelo tratar a propria saida da ferramenta como uma
// pergunta -- e as vezes responder "como posso ajudar?" no meio da tarefa.
func (m *Model) continuaAposFerramentas(resultados []mcp.Result) tea.Cmd {
	m.registraResultadosDeFerramenta(resultados)

	cfg := availableModels[m.currentModel]
	if isLocalModel(cfg) {
		m.streamPartial = ""
		m.streamThinking = ""
		m.streamStart = time.Now()
		return m.callOllamaStream(cfg.Endpoint)
	}

	// Caminho remoto: nao ha turno de ferramenta nativo, entao o proprio
	// achatamento em callAPI transforma isso em texto. Basta pedir a continuacao.
	return m.callAPI("Continue a tarefa usando os resultados acima.")
}

func (m *Model) processFileReferences(input string) (string, error) {
	words := strings.Fields(input)
	var processedParts []string
	var fileContents []string
	var filePreviews []string

	for _, word := range words {
		if strings.HasPrefix(word, "@") {
			filePath := strings.TrimPrefix(word, "@")
			if !strings.HasPrefix(filePath, "/") {
				filePath = filepath.Join(".", filePath)
			}

			// Gera preview do arquivo
			info, _, err := m.fsManager.FileInfo(filePath)
			if err == nil {
				size := info.Size()
				sizeStr := fmt.Sprintf("%d bytes", size)
				if size > 1024*1024 {
					sizeStr = fmt.Sprintf("%.1f MB", float64(size)/1024/1024)
				} else if size > 1024 {
					sizeStr = fmt.Sprintf("%.1f KB", float64(size)/1024)
				}

				lines := 0
				content, err := m.fsManager.ReadFile(filePath)
				if err == nil {
					lines = strings.Count(string(content), "\n") + 1
					fileContents = append(fileContents, fmt.Sprintf("\n\n--- Conteúdo de %s ---\n%s\n--- Fim do arquivo ---\n", filePath, string(content)))
					filePreviews = append(filePreviews, fmt.Sprintf("  📄 %s (%s, %d linhas)", filePath, sizeStr, lines))
				}
			} else {
				return "", fmt.Errorf("não foi possível ler o arquivo %s: %w", filePath, err)
			}

			processedParts = append(processedParts, fmt.Sprintf("[arquivo: %s]", filePath))
		} else {
			processedParts = append(processedParts, word)
		}
	}

	// Mostra preview dos arquivos anexados
	if len(filePreviews) > 0 {
		m.addMessage(m.theme.InfoStyle.Render("📎 Arquivos anexados:"))
		for _, preview := range filePreviews {
			m.addMessage(m.theme.HelpStyle.Render(preview))
		}
		m.addMessage("")
	}

	result := strings.Join(processedParts, " ")
	if len(fileContents) > 0 {
		result += strings.Join(fileContents, "\n")
	}

	return result, nil
}

func (m *Model) showAbout() {
	lines := []string{
		"┌─ Sobre Amazzo Agent:",
		"│  Um assistente CLI multi-modelo especializado em geração de código.",
		"│  Integração com múltiplos modelos LLM do Hugging Face.",
		"│  Recursos:",
		"│    • Múltiplos modelos (Qwen, MiniMax, GLM, DeepSeek, etc)",
		"│    • Sistema de temas personalizáveis",
		"│    • Suporte a ferramentas para leitura/escrita de arquivos",
		"│    • Interface Bubble Tea com layout adaptativo",
		"│    • Histórico persistente por sessão",
		"└─",
	}
	for _, line := range lines {
		m.addMessage(m.theme.InfoStyle.Render(line))
	}
	m.addMessage("")
}

func (m *Model) formatMetrics() string {
	stats := m.metrics.GetStats()
	cacheHits, cacheMisses, cacheHitRate := m.cache.Stats()

	var result strings.Builder

	result.WriteString(m.theme.InfoStyle.Render("┌─ 📊 Estatísticas de Performance\n"))
	result.WriteString(m.theme.InfoStyle.Render("│\n"))
	result.WriteString(m.theme.InfoStyle.Render(fmt.Sprintf("│  🔄 Requisições Totais:     %d\n", stats.TotalRequests)))
	result.WriteString(m.theme.HelpStyle.Render(fmt.Sprintf("│     • Bem-sucedidas:      %d\n", stats.SuccessfulRequests)))
	result.WriteString(m.theme.HelpStyle.Render(fmt.Sprintf("│     • Falhadas:           %d\n", stats.FailedRequests)))
	result.WriteString(m.theme.InfoStyle.Render(fmt.Sprintf("│     • Taxa de Sucesso:    %.1f%%\n", stats.SuccessRate)))
	result.WriteString(m.theme.InfoStyle.Render("│\n"))

	if stats.TotalRequests > 0 {
		result.WriteString(m.theme.InfoStyle.Render(fmt.Sprintf("│  ⏱️  Latência:\n")))
		result.WriteString(m.theme.HelpStyle.Render(fmt.Sprintf("│     • Média:              %v\n", stats.AverageLatency.Round(time.Millisecond))))
		result.WriteString(m.theme.HelpStyle.Render(fmt.Sprintf("│     • Mínima:             %v\n", stats.MinLatency.Round(time.Millisecond))))
		result.WriteString(m.theme.HelpStyle.Render(fmt.Sprintf("│     • Máxima:             %v\n", stats.MaxLatency.Round(time.Millisecond))))
		result.WriteString(m.theme.InfoStyle.Render("│\n"))
	}

	result.WriteString(m.theme.InfoStyle.Render(fmt.Sprintf("│  🪙  Tokens Totais:         %d\n", stats.TotalTokens)))
	if stats.TokensPerSecond > 0 {
		result.WriteString(m.theme.HelpStyle.Render(fmt.Sprintf("│     • Tokens/Segundo:     %.1f\n", stats.TokensPerSecond)))
	}
	result.WriteString(m.theme.InfoStyle.Render("│\n"))

	result.WriteString(m.theme.InfoStyle.Render(fmt.Sprintf("│  💾 Cache:\n")))
	result.WriteString(m.theme.HelpStyle.Render(fmt.Sprintf("│     • Hits:               %d\n", cacheHits)))
	result.WriteString(m.theme.HelpStyle.Render(fmt.Sprintf("│     • Misses:             %d\n", cacheMisses)))
	if cacheHits+cacheMisses > 0 {
		result.WriteString(m.theme.HelpStyle.Render(fmt.Sprintf("│     • Taxa de Acerto:     %.1f%%\n", cacheHitRate)))
	}

	result.WriteString(m.theme.InfoStyle.Render("└─"))

	return result.String()
}

// respostaRepetida registra a resposta e diz se ela ja apareceu entre as
// ultimas do agente.
//
// O guard anterior comparava so com a resposta imediatamente anterior e
// exigia igualdade byte a byte: bastava uma linha em branco a mais para o
// agente seguir repetindo a mesma frase dezenas de vezes. Aqui normalizamos
// os espacos e olhamos uma janela curta, o que tambem pega alternancia
// (A, B, A, B...).
func (m *Model) respostaRepetida(response string) bool {
	normalizada := strings.Join(strings.Fields(response), " ")
	if normalizada == "" {
		return false
	}

	for _, anterior := range m.recentResponses {
		if anterior == normalizada {
			return true
		}
	}

	const janela = 4
	m.recentResponses = append(m.recentResponses, normalizada)
	if len(m.recentResponses) > janela {
		m.recentResponses = m.recentResponses[len(m.recentResponses)-janela:]
	}

	return false
}

// glamourStyleName escolhe o estilo do markdown a partir do tema ativo, sem
// consultar o terminal.
func (m *Model) glamourStyleName() string {
	switch m.currentTheme {
	case "light", "solarized":
		return "light"
	default:
		return "dark"
	}
}

// finishSplash escreve as mensagens de boas-vindas ao sair da tela de abertura.
func (m *Model) finishSplash() {
	m.addMessage("")
	m.addMessage(m.theme.InfoStyle.Render("✨ Bem-vindo ao Amazzo Agent!"))
	m.addMessage("")

	if m.configManager != nil && m.configManager.IsFirstRun() {
		m.showWelcomeTutorial()
		m.configManager.MarkFirstRunComplete()
		return
	}

	m.addMessage(m.theme.HelpStyle.Render(fmt.Sprintf("  Modelo: %s • Tema: %s",
		availableModels[m.currentModel].DisplayName, m.theme.Name)))
	m.addMessage("")
	m.addMessage(m.theme.HelpStyle.Render("  💡 Digite /ajuda para ver os comandos • Ctrl+L para limpar"))
	m.addMessage("")
}

// thought guarda um raciocínio já concluído para poder ser aberto e fechado
// depois, sem perder o texto.
type thought struct {
	index   int // posição em m.messages
	text    string
	elapsed time.Duration
}

// responseMeta resume o custo da última resposta local. Com modelo na CPU esse
// número é a diferença entre "está pensando" e "travou".
func (m *Model) responseMeta() string {
	st := m.lastOllamaStats
	if st.GenTokens == 0 {
		return ""
	}
	return fmt.Sprintf("%.0f tok/s · %s", st.TokensPerSecond(), formatElapsed(st.Total))
}

// toggleThinking abre e fecha todos os raciocínios da conversa.
func (m *Model) toggleThinking() {
	m.showThinking = !m.showThinking
	for _, t := range m.thoughts {
		if t.index >= 0 && t.index < len(m.messages) {
			m.messages[t.index] = renderThought(t.text, t.elapsed, m.showThinking, m.viewport.Width)
		}
	}
	m.viewport.SetContent(strings.Join(m.messages, "\n"))
}

// remoteSystemPrompt é o prompt dos modelos servidos pelo Hugging Face.
//
// Ele é mais longo que o local porque esses modelos aguentam mais contexto,
// mas segue a mesma regra: ferramenta só quando a pergunta exige olhar o
// projeto.
const remoteSystemPrompt = `Você é o Amazzo, um assistente de programação que trabalha no terminal do usuário.
Responda SEMPRE em Português do Brasil.

## Quando usar ferramenta

A pergunta é sobre CONHECIMENTO ou sobre o ESTADO DESTE PROJETO?

- Conhecimento (o que é X, como funciona Y, escreva um exemplo, conversa,
  cumprimento) → você já sabe. Responda em texto, sem ferramenta.
- Estado do projeto (o que tem nesta pasta, o que esse arquivo faz, esse teste
  passa) → você não sabe sem olhar. Use ferramenta antes de responder.
- Pedido para ler, criar, alterar, rodar ou procurar → use ferramenta.

## Mostrar não é salvar

"me mostre", "exiba", "escreva um exemplo", "como se faz" → escreva o código na
resposta, dentro de um bloco de código. Não crie arquivo. Só use write_file
quando o usuário pedir para criar, salvar ou gravar algo. Na dúvida, mostre o
código e pergunte se ele quer que você salve.

Antes de descrever, resumir, documentar ou corrigir um arquivo, leia ele com
read_file. Nunca deduza o conteúdo pelo nome do arquivo.
Não afirme que algo falta sem ter verificado: se o arquivo apareceu numa
listagem, ele existe e não deve ser sugerido para criação. Recomende só o que
serve para a linguagem deste projeto.
Você nunca sai explorando o projeto por conta própria: olha só o que a pergunta
exige. Nunca invente caminhos, nomes de arquivo ou conteúdo — o que você afirma
vem do resultado das ferramentas.

## Ferramentas

Emita um bloco por vez e espere o resultado antes do próximo:

` + "```tool" + `
{"tool": "list_dir", "path": "."}
` + "```" + `

Use exatamente estas chaves:
{"tool":"read_file","path":"..."}
{"tool":"write_file","path":"...","content":"..."}
{"tool":"edit_file","path":"...","search":"...","replace":"..."}
{"tool":"list_dir","path":"."}
{"tool":"search_files","pattern":"...","path":"."}
{"tool":"make_dir","path":"..."}
{"tool":"execute_command","command":"go test ./..."}
{"tool":"web_fetch","url":"https://..."}
{"tool":"set_workspace","path":"/caminho/do/projeto"}

## Pasta de trabalho

Você só enxerga dentro da pasta de trabalho atual. Se o usuário citar um projeto
em outro caminho, abra a pasta primeiro com set_workspace e só depois use as
outras ferramentas — os caminhos passam a ser relativos a ela. Nunca diga que
não pode acessar um caminho: abra a pasta.

## Tarefas de várias etapas

A lista é só para trabalho que exige várias ferramentas seguidas — criar
vários arquivos, montar um projeto. Escrever código na resposta é uma etapa só:
escreva o código direto, sem lista.

Quando o pedido realmente tiver várias etapas, comece listando-as uma vez:

` + "```todo" + `
[ ] Criar estrutura de pastas
[ ] Implementar o servidor
[ ] Escrever os testes
` + "```" + `

Marque [x] conforme avança e siga até o fim. Tarefa simples e direta não precisa
de lista: faça e pronto.

## Qualidade

Siga as convenções do código que já existe no projeto. Trate erros de forma
apropriada. Seja conciso: sem preâmbulo, sem repetir o pedido, sem resumo final
longo.`

// fencedBlockRe casa qualquer bloco cercado por ``` com uma tag opcional.
//
// Nao exigimos a tag "tool": modelos pequenos escrevem ```json, ```tool,
// ``` ou nada. O que identifica uma chamada e o conteudo ser um objeto JSON
// com a chave "tool" -- essa e a informacao, a tag e so enfeite que o modelo
// erra com frequencia.
var fencedBlockRe = regexp.MustCompile("(?s)```[a-zA-Z]*[ \t]*\r?\n(.*?)```")

// toolCallFromBlock devolve o JSON da chamada se o bloco for uma, e ok=false
// caso contrario.
func toolCallFromBlock(payload string) (map[string]interface{}, bool) {
	trimmed := strings.TrimSpace(payload)
	if !strings.HasPrefix(trimmed, "{") {
		return nil, false
	}

	var obj map[string]interface{}
	if err := json.Unmarshal([]byte(trimmed), &obj); err != nil {
		return nil, false
	}
	if _, ok := nomeDaFerramenta(obj); !ok {
		return nil, false
	}
	return obj, true
}

// nomeDaFerramenta acha o nome da ferramenta nos formatos que os modelos usam.
//
// O prompt pede {"tool": "...", "path": "..."}, mas os modelos caem sozinhos
// no formato padrao de function calling em que foram treinados --
// {"name": "...", "arguments": {...}}, as vezes embrulhado em "function".
// Ignorar esse formato fazia a chamada virar texto na tela e a tarefa parar.
func nomeDaFerramenta(obj map[string]interface{}) (string, bool) {
	// {"function": {"name": ..., "arguments": ...}}
	if fn, ok := obj["function"].(map[string]interface{}); ok {
		return nomeDaFerramenta(fn)
	}

	for _, chave := range []string{"tool", "name"} {
		nome, ok := obj[chave].(string)
		if !ok || nome == "" {
			continue
		}
		// Validar contra as ferramentas suportadas e o que separa uma chamada
		// de um JSON qualquer exibido na resposta: um package.json tem "name",
		// mas "meu-projeto" nao e ferramenta.
		if !mcp.FerramentaSuportada(nome) {
			continue
		}
		return nome, true
	}
	return "", false
}

// requestDoPayload converte o JSON da chamada em uma Request, aceitando tanto
// o formato do prompt quanto o de function calling.
func requestDoPayload(payload string) (mcp.Request, bool) {
	var obj map[string]interface{}
	if err := json.Unmarshal([]byte(payload), &obj); err != nil {
		return mcp.Request{}, false
	}

	if fn, ok := obj["function"].(map[string]interface{}); ok {
		obj = fn
	}

	nome, ok := nomeDaFerramenta(obj)
	if !ok {
		return mcp.Request{}, false
	}

	// No formato de function calling os parametros vem dentro de "arguments",
	// que alguns modelos mandam como objeto e outros como string JSON.
	campos := obj
	if args, existe := obj["arguments"]; existe {
		switch v := args.(type) {
		case map[string]interface{}:
			campos = v
		case string:
			var decodificado map[string]interface{}
			if json.Unmarshal([]byte(v), &decodificado) == nil {
				campos = decodificado
			}
		}
	}

	bruto, err := json.Marshal(campos)
	if err != nil {
		return mcp.Request{}, false
	}

	var req mcp.Request
	if err := json.Unmarshal(bruto, &req); err != nil {
		return mcp.Request{}, false
	}
	req.Tool = nome

	return req, true
}

// substituiChamadas troca cada chamada de ferramenta encontrada no texto pelo
// resultado de repl, preservando o restante intacto.
//
// Remove também a cerca ao redor da chamada, quando existe: deixá-la produziria
// um bloco de código vazio na tela.
func substituiChamadas(text string, repl func(payload string) string) string {
	chamadas := findToolPayloads(text)
	if len(chamadas) == 0 {
		return text
	}

	for _, payload := range chamadas {
		var obj map[string]interface{}
		if err := json.Unmarshal([]byte(payload), &obj); err != nil {
			continue
		}

		// Localiza a ocorrência original (o JSON foi re-serializado, então
		// procuramos pelo bloco que contém a mesma chamada).
		idx := indiceDaChamada(text, obj)
		if idx.inicio < 0 {
			continue
		}
		text = text[:idx.inicio] + repl(payload) + text[idx.fim:]
	}

	return text
}

type trecho struct{ inicio, fim int }

// indiceDaChamada acha onde a chamada aparece no texto original, incluindo a
// cerca ``` ao redor quando ela existe.
func indiceDaChamada(text string, alvo map[string]interface{}) trecho {
	for i := 0; i < len(text); i++ {
		if text[i] != '{' {
			continue
		}

		dec := json.NewDecoder(strings.NewReader(text[i:]))
		var obj map[string]interface{}
		if err := dec.Decode(&obj); err != nil {
			continue
		}
		if fmt.Sprint(obj) != fmt.Sprint(alvo) {
			continue
		}

		inicio, fim := i, i+int(dec.InputOffset())
		if fim > len(text) {
			fim = len(text)
		}

		// Engole a cerca de abertura e fechamento, se houver.
		if abertura := strings.LastIndex(text[:inicio], "```"); abertura >= 0 &&
			strings.TrimSpace(text[abertura:inicio]) == strings.TrimSpace(
				strings.SplitN(text[abertura:inicio], "\n", 2)[0]) {
			inicio = abertura
		}
		resto := text[fim:]
		if corte := strings.Index(resto, "```"); corte >= 0 && strings.TrimSpace(resto[:corte]) == "" {
			fim += corte + 3
		}

		return trecho{inicio, fim}
	}
	return trecho{-1, -1}
}

// removePlaceholders tira da conversa as linhas de espera quando a resposta
// chega.
//
// Antes o código removia a última mensagem SEM olhar o que era, porque
// assumia que seria sempre o placeholder "⏳ Processando...". Quando a prévia
// do streaming passou a mostrar a fase, esse placeholder deixou de existir e
// a remoção cega passou a comer mensagens reais — inclusive eventos de
// ferramenta e avisos do próprio Amazzo.
func (m *Model) removePlaceholders() {
	// A comparacao era sensivel a caixa e a mensagem real e "🔁 Executando
	// ferramentas (N/15 ciclos)..." -- com E maiusculo. O placeholder nunca
	// era removido e as linhas se acumulavam na tela a cada ciclo.
	placeholder := func(s string) bool {
		baixa := strings.ToLower(s)
		return strings.Contains(baixa, "processando") ||
			strings.Contains(baixa, "executando ferramentas")
	}

	mudou := false
	for len(m.messages) > 0 && placeholder(m.messages[len(m.messages)-1]) {
		m.messages = m.messages[:len(m.messages)-1]
		mudou = true
	}

	if mudou {
		m.viewport.SetContent(strings.Join(m.messages, "\n"))
	}
}

// numeroDeEtapa casa "1.", "2)", "-" e "*" no inicio da linha.
var numeroDeEtapa = regexp.MustCompile(`^(?:\d+[.)]|[-*•])\s+`)

// etapaSemMarcador tira o marcador de lista quando o modelo escreve a etapa
// como item numerado em vez do "[ ]" que o prompt pede.
func etapaSemMarcador(linha string) (string, bool) {
	if loc := numeroDeEtapa.FindStringIndex(linha); loc != nil {
		return strings.TrimSpace(linha[loc[1]:]), true
	}
	return linha, false
}

// scrollPage rola a conversa uma tela para cima (-1) ou para baixo (+1).
//
// Não usamos ViewUp/ViewDown do viewport porque elas desistem em silêncio
// quando acham que já estão no topo ou no fundo — e essa conta usa a altura
// atual, que o View() reescreve a cada quadro conforme a prévia de streaming
// cresce e some. Bastava o offset ficar além do novo fim para o PgDn parar de
// responder. SetYOffset com clamp não tem esse ponto cego.
func (m *Model) scrollPage(dir int) {
	passo := m.viewport.Height - 2 // uma linha de sobreposição para não perder o fio
	if passo < 1 {
		passo = 1
	}
	m.viewport.SetYOffset(m.viewport.YOffset + dir*passo)
}

// limpaCampoDeTexto devolve o campo de digitação ao estado inicial.
//
// Reset() apaga o texto mas deixa a janela interna do textarea onde estava.
// Depois de enviar uma mensagem, o próximo texto longo aparecia já rolado e a
// primeira linha ficava escondida. Reaplicar a largura força o textarea a
// refazer as contas e voltar ao topo.
func (m *Model) limpaCampoDeTexto() {
	m.textarea = novoCampoDeTexto(m.larguraDoCampo())
}

// larguraDoCampo devolve a largura que o campo deve ter.
//
// Não dá para perguntar ao textarea: Width() devolve a largura SEM o prompt,
// então reaplicá-la encolhia o campo em duas colunas a cada envio, e o ponto
// de quebra do texto ia andando.
func (m *Model) larguraDoCampo() int {
	largura := m.width - 6 // margem (2) + borda (2) + padding (2) da moldura
	if largura < 20 {
		largura = 20
	}
	return largura
}

// novoCampoDeTexto monta o campo de digitação com a configuração do Amazzo.
//
// Existe como função porque o campo é recriado a cada envio: Reset() apaga o
// texto mas deixa a janela interna do textarea rolada onde estava, e o texto
// longo seguinte aparecia sem a primeira linha.
func novoCampoDeTexto(largura int) textarea.Model {
	ta := textarea.New()
	ta.Placeholder = "Digite sua mensagem... (Shift+Enter para nova linha)"
	ta.Focus()
	ta.Prompt = "► "
	ta.CharLimit = 50000
	ta.ShowLineNumbers = false
	ta.KeyMap.InsertNewline.SetEnabled(true) // Shift+Enter quebra linha
	ta.SetWidth(largura)
	ta.SetHeight(1)
	return ta
}

// aplicaAberturaDePasta executa os pedidos de set_workspace e devolve o
// restante para o MCP.
//
// O Amazzo abre sempre no diretório em que foi lançado — pelo rofi, isso é
// sempre a home. Quando você nomeia um projeto ("analise /caminho/x"), o
// agente precisa poder abrir aquela pasta, como abrir um projeto no editor.
// Sem isso ele batia no limite do workspace e desistia.
func (m *Model) aplicaAberturaDePasta(requests []mcp.Request) (restantes []mcp.Request, resultados []mcp.Result) {
	for _, req := range requests {
		if strings.ToLower(req.Tool) != "set_workspace" {
			restantes = append(restantes, req)
			continue
		}

		// O agente está no meio de uma tarefa: a conversa continua.
		if err := m.mudaWorkspace(req.Path, false); err != nil {
			resultados = append(resultados, mcp.Result{
				Tool: req.Tool, Path: req.Path, Status: "error", Error: err.Error(),
			})
			continue
		}

		// A linha "abriu a pasta X" ja foi exibida como intencao da chamada
		// (mostraChamadasNativas para o canal nativo, substituiChamadas para o
		// JSON em texto). Imprimi-la aqui de novo duplicava o evento na tela.
		resultados = append(resultados, mcp.Result{
			Tool: req.Tool, Path: req.Path, Status: "success",
			// Sem a instrução explícita o modelo abria a pasta e parava com
			// "como posso ajudar?", esquecendo o pedido que o trouxe até aqui.
			Content: "Pasta de trabalho agora é " + m.workspaceRoot +
				". Os caminhos passam a ser relativos a ela. " +
				"Agora continue a tarefa que o usuário pediu, do ponto onde parou.",
		})
	}

	return restantes, resultados
}

// mudaWorkspace troca a pasta de trabalho do agente.
//
// limpaConversa distingue duas situações que parecem a mesma:
//   - você usando /pasta: está trocando de projeto, e o contexto anterior
//     fala de arquivos que não estão mais à vista — a conversa recomeça;
//   - o agente abrindo a pasta no meio de uma tarefa: apagar o histórico aqui
//     apagava o próprio pedido que o levou a abrir a pasta, e ele terminava
//     perguntando "como posso ajudar?".
func (m *Model) mudaWorkspace(caminho string, limpaConversa bool) error {
	// ~ e caminhos relativos são o que a pessoa realmente digita.
	if strings.HasPrefix(caminho, "~") {
		if home, err := os.UserHomeDir(); err == nil {
			caminho = filepath.Join(home, strings.TrimPrefix(caminho, "~"))
		}
	}
	if !filepath.IsAbs(caminho) {
		caminho = filepath.Join(m.workspaceRoot, caminho)
	}

	fs, err := filesystem.NewManager(caminho)
	if err != nil {
		return err
	}

	m.fsManager = fs
	m.workspaceRoot = fs.Root()
	m.mcpServer = mcp.NewServer(fs)

	if limpaConversa {
		m.resetConversation()
	}
	return nil
}
