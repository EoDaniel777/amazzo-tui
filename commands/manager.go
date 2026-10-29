package commands

import (
	"fmt"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"

	"amazzo/credentials"
	"amazzo/permissions"
	"golang.org/x/term"
)

// ModelInfo representa um modelo disponível.
type ModelInfo struct {
	DisplayName string
	Endpoint    string
}

// ThemeInfo representa um tema disponível.
type ThemeInfo struct {
	Key  string
	Name string
}

// Dependencies encapsula callbacks necessários para executar comandos.
type Dependencies struct {
	AddMessage func(string)
	AddInfo    func(string)
	AddHelp    func(string)
	AddError   func(string)
	AddSuccess func(string)
	AddBlank   func()

	ClearMessages func()
	ShowAbout     func()

	GetCurrentModelIndex func() int
	ListModels           func() []ModelInfo
	SetModel             func(int) error
	RefreshLocalModels   func() (int, error)
	WorkspaceRoot        func() string
	SetWorkspaceRoot     func(string) error

	GetCurrentThemeKey func() string
	ListThemes         func() []ThemeInfo
	SetTheme           func(string) error
	SaveTheme          func(string) error

	LastError                func() string
	LastResponse             func() string
	SaveConversation         func(string) (string, error)
	SaveReadableConversation func(string) (string, error)
	SaveLastResponse         func() (string, error)
	WorkingDir               func() string

	// Multi-agente
	IsMultiAgentMode     func() bool
	ToggleMultiAgentMode func() bool

	// Métricas
	GetMetrics func() string

	// Agentes Especializados
	GetSpecializedAgentRegistry func() interface{} // Retorna *agents.SpecializedAgentRegistry

	// Credenciais
	UpdateToken func(string) error

	// Tarefas
	GetTaskStatus func() string

	// Criação de agentes
	StartAgentCreation func()

	// Modo Ultra Visual
	IsUltraMode     func() bool
	ToggleUltraMode func() bool
}

// Manager gerencia comandos complexos.
type Manager struct {
	deps Dependencies
}

// Handle processa um comando. Retorna true quando o input corresponde a um comando.
func Handle(input string, deps Dependencies) bool {
	parts := strings.Fields(input)
	if len(parts) == 0 {
		return false
	}

	command := strings.ToLower(strings.TrimPrefix(parts[0], "/"))

	switch command {
	case "ajuda", "help":
		showHelp(deps)
		return true

	case "limpar", "clear":
		deps.ClearMessages()
		deps.AddInfo("✨ Tela limpa.")
		deps.AddBlank()
		return true

	case "ultra", "ultramode":
		if deps.ToggleUltraMode != nil {
			enabled := deps.ToggleUltraMode()
			if enabled {
				deps.AddSuccess("🎨 ✨ MODO ULTRA VISUAL ATIVADO! ✨")
				deps.AddInfo("    Interface extraordinária com:")
				deps.AddInfo("    • Cores néon vibrantes e gradientes")
				deps.AddInfo("    • Animações e efeitos especiais")
				deps.AddInfo("    • Bordas artísticas e banners")
				deps.AddInfo("    • Visual de última geração!")
			} else {
				deps.AddInfo("🎨 Modo visual padrão restaurado.")
			}
		} else {
			deps.AddError("❌ Comando /ultra não disponível.")
		}
		deps.AddBlank()
		return true

	case "agente", "agent", "modelo", "model":
		return handleModelChange(parts, deps)

	case "ollama", "local":
		return handleOllamaRefresh(deps)

	case "pasta", "cd", "workspace":
		return handleWorkspaceChange(parts, deps)

	case "temas", "theme", "themes":
		return handleThemeChange(parts, deps)

	case "sobre", "about":
		deps.ShowAbout()
		return true

	case "erro", "error":
		last := deps.LastError()
		if strings.TrimSpace(last) == "" {
			deps.AddInfo("┌─ Nenhum erro recente")
			deps.AddInfo("└─")
		} else {
			deps.AddError("┌─ Último erro:")
			deps.AddMessage(last)
			deps.AddInfo("└─ (Use /logs para salvar ou /copiar para exportar)")
		}
		deps.AddBlank()
		return true

	case "salvar", "arquivar", "save", "archive":
		// Salva em formato legível para o usuário (sem códigos ANSI)
		filename := ""
		if len(parts) > 1 {
			filename = strings.Join(parts[1:], " ")
		}
		path, err := deps.SaveReadableConversation(filename)
		if err != nil {
			deps.AddError(fmt.Sprintf("❌ Erro ao salvar: %v", err))
		} else {
			wd := deps.WorkingDir()
			if wd != "" && strings.HasPrefix(path, wd) {
				rel, relErr := filepath.Rel(wd, path)
				if relErr == nil {
					path = rel
				}
			}
			deps.AddSuccess(fmt.Sprintf("✅ Conversa salva em formato legível: %s", path))
			deps.AddInfo("💡 Dica: Use /logs para salvar com formatação técnica")
		}
		deps.AddBlank()
		return true

	case "logs", "exportar":
		// Salva em formato técnico (com códigos ANSI) para logs
		filename := ""
		if len(parts) > 1 {
			filename = strings.Join(parts[1:], " ")
		}
		path, err := deps.SaveConversation(filename)
		if err != nil {
			deps.AddError(fmt.Sprintf("❌ Erro ao salvar: %v", err))
		} else {
			wd := deps.WorkingDir()
			if wd != "" && strings.HasPrefix(path, wd) {
				rel, relErr := filepath.Rel(wd, path)
				if relErr == nil {
					path = rel
				}
			}
			deps.AddSuccess(fmt.Sprintf("✅ Conversa salva em formato técnico: %s", path))
			deps.AddInfo("💡 Dica: Use /salvar para formato legível")
		}
		deps.AddBlank()
		return true

	case "copiar", "copy":
		path, err := deps.SaveLastResponse()
		if err != nil {
			deps.AddError(fmt.Sprintf("❌ Erro ao salvar resposta: %v", err))
		} else {
			wd := deps.WorkingDir()
			if wd != "" && strings.HasPrefix(path, wd) {
				rel, relErr := filepath.Rel(wd, path)
				if relErr == nil {
					path = rel
				}
			}
			deps.AddSuccess(fmt.Sprintf("✅ Última resposta salva em: %s", path))
		}
		deps.AddBlank()
		return true

	case "multiagente", "multi-agente", "multiagent", "multi-agent":
		isActive := deps.ToggleMultiAgentMode()
		if isActive {
			deps.AddSuccess("┌─ 🤖 Modo Multi-Agente ATIVADO")
			deps.AddInfo("│")
			deps.AddInfo("│  Sistema multi-agente inicializado!")
			deps.AddInfo("│")
			deps.AddInfo("│  Agora quando você fizer requisições complexas:")
			deps.AddInfo("│  • Coordenador analisa e cria plano de tarefas")
			deps.AddInfo("│  • Codificadores trabalham em paralelo")
			deps.AddInfo("│  • Revisor valida qualidade do código")
			deps.AddInfo("│  • Executor roda testes automaticamente")
			deps.AddInfo("│")
			deps.AddInfo("│  💡 Locks automáticos previnem conflitos")
			deps.AddInfo("│  ⚡ Execução paralela para máxima velocidade")
			deps.AddSuccess("└─")
		} else {
			deps.AddInfo("┌─ Modo Multi-Agente DESATIVADO")
			deps.AddInfo("│  Voltando ao modo normal (um agente)")
			deps.AddInfo("└─")
		}
		deps.AddBlank()
		return true

	case "metricas", "metrics", "estatisticas", "stats":
		metricsOutput := deps.GetMetrics()
		deps.AddMessage(metricsOutput)
		deps.AddBlank()
		return true

	case "agentes", "agents":
		// Novo comando para gerenciar agentes especializados
		if deps.GetSpecializedAgentRegistry == nil {
			deps.AddError("❌ Sistema de agentes especializados não disponível")
			deps.AddBlank()
			return true
		}
		args := ""
		if len(parts) > 1 {
			args = strings.Join(parts[1:], " ")
		}
		cm := &Manager{deps: deps}
		result := cm.handleAgentsCommand(args)
		deps.AddMessage(result)
		deps.AddBlank()
		return true

	case "key", "token", "chave":
		return handleKeyCommand(deps)

	case "permitir", "permissoes", "permissions", "allow":
		return handlePermissionsCommand(parts, deps)

	case "tarefas", "tasks", "todo":
		if deps.GetTaskStatus == nil {
			deps.AddError("❌ Sistema de tarefas não disponível")
			deps.AddBlank()
			return true
		}
		status := deps.GetTaskStatus()
		deps.AddInfo(status)
		deps.AddBlank()
		return true

	case "criar-agente", "new-agent":
		return handleCreateAgentCommand(deps)

	case "editar-agente", "edit-agent":
		return handleEditAgentCommand(parts, deps)

	case "deletar-agente", "delete-agent", "remover-agente":
		return handleDeleteAgentCommand(parts, deps)

	default:
		return false
	}
}

func showHelp(deps Dependencies) {
	lines := []string{
		"┌─ 🤖 COMANDOS DO AMAZZO AGENT:",
		"│",
		"│  /ajuda ou /help           - Mostra esta ajuda",
		"│  /modelo [número]          - Lista ou troca entre modelos LLM",
		"│  /ollama                   - Reexamina os modelos baixados no Ollama",
		"│  /pasta [caminho]          - Mostra ou troca a pasta de trabalho",
		"│  /limpar ou /clear         - Limpa a tela",
		"│",
		"│  🎯 Recursos Avançados:",
		"│  /agentes [subcomando]     - Gerencia agentes especializados",
		"│  /criar-agente             - ✨ Cria agente personalizado (TUI interativa)",
		"│  /multiagente              - 🤖 Ativa/desativa modo multi-agente",
		"│",
		"│  🎨 Personalização:",
		"│  /temas [nome]             - Lista ou troca o tema de cores",
		"│  /ultra                    - ✨ Ativa MODO ULTRA VISUAL (extraordinário!)",
		"│  /key ou /token            - 🔐 Atualiza token do Hugging Face",
		"│",
		"│  📊 Monitoramento:",
		"│  /metricas ou /metrics     - Mostra estatísticas de performance",
		"│  /tarefas ou /tasks        - 📋 Mostra progresso das tarefas atuais",
		"│  /erro                     - Mostra o último erro (copiável)",
		"│",
		"│  📁 Gerenciamento de Arquivos:",
		"│  @arquivo.ext              - Incluir conteúdo de arquivo",
		"│  /copiar                   - Salva última resposta em arquivo",
		"│  /salvar ou /arquivar      - 📄 Salva conversa em formato legível (output/)",
		"│  /logs [arquivo]           - Salva conversa em formato técnico (logs/)",
		"│",
		"│  ⚙️  Configurações:",
		"│  /permitir [modo]          - 🔓 Gerencia permissões de ferramentas MCP",
		"│  /sobre ou /about          - Informações sobre o assistente",
		"│",
		"│  🎹 Atalhos do Terminal:",
		"│  Ctrl+C ou Esc             - Sai do programa",
		"│  Ctrl+L                    - Limpa a tela",
		"│  Tab                       - Autocompleta /comandos",
		"│  Ctrl+T                    - Toggle visualização de tarefas",
		"│  Ctrl++ / Ctrl+-           - Ajustar zoom da interface",
		"│",
		"│  🤖 Recursos Inteligentes:",
		"│  'Crie arquivo X'          - O modelo criará automaticamente",
		"│  'Preciso de ajuda com Y'  - O assistente detecta contexto automaticamente",
		"│  🤖 /multiagente           - Múltiplos agentes trabalhando em paralelo!",
		"│  📊 /metricas              - Veja estatísticas e cache de respostas!",
		"│",
		"│  ✅ Dica: Digite normalmente para conversar com o modelo atual.",
		"│      Use /agente para alternar entre diferentes modelos LLM.",
		"└─",
	}

	for _, line := range lines {
		deps.AddHelp(line)
	}
	deps.AddBlank()
}

func handleModelChange(parts []string, deps Dependencies) bool {
	models := deps.ListModels()
	if len(parts) == 1 {
		deps.AddHelp("┌─ Modelos disponíveis:")
		for i, model := range models {
			prefix := "│    "
			if i == deps.GetCurrentModelIndex() {
				prefix = "│  ► "
			}
			deps.AddHelp(fmt.Sprintf("%s[%d] %s", prefix, i+1, model.DisplayName))
		}
		deps.AddHelp("│")
		deps.AddHelp("│  Use: /agente <número> para trocar")
		deps.AddHelp("│  Ex: /agente 1 para usar o primeiro modelo")
		deps.AddHelp("└─")
		deps.AddBlank()
		return true
	}

	index, err := strconv.Atoi(parts[1])
	if err != nil || index < 1 || index > len(models) {
		deps.AddError(fmt.Sprintf("┌─ Erro: número de modelo inválido (1-%d)", len(models)))
		deps.AddError("│  Use /agente para ver os modelos disponíveis")
		deps.AddError("└─")
		deps.AddBlank()
		return true
	}

	if err := deps.SetModel(index - 1); err != nil {
		deps.AddError(fmt.Sprintf("❌ Falha ao trocar modelo: %v", err))
	} else {
		deps.AddSuccess(fmt.Sprintf("┌─ ✅ Modelo alterado para: %s", models[index-1].DisplayName))
		deps.AddSuccess("│")
		deps.AddSuccess("│  Agora você pode fazer perguntas ao novo modelo")
		deps.AddSuccess("│  Use /modelo para trocar novamente quando quiser")
		deps.AddSuccess("└─")
	}
	deps.AddBlank()
	return true
}

func handleThemeChange(parts []string, deps Dependencies) bool {
	themes := deps.ListThemes()
	if len(parts) == 1 {
		deps.AddHelp("┌─ Temas disponíveis:")
		current := deps.GetCurrentThemeKey()
		for _, theme := range themes {
			prefix := "│    "
			if theme.Key == current {
				prefix = "│  ► "
			}
			deps.AddHelp(fmt.Sprintf("%s%s", prefix, theme.Name))
		}
		deps.AddHelp("│")
		deps.AddHelp("│  Use: /temas <nome> para trocar")
		deps.AddHelp("└─")
		deps.AddBlank()
		return true
	}

	themeKey := strings.ToLower(parts[1])
	if err := deps.SetTheme(themeKey); err != nil {
		deps.AddError(fmt.Sprintf("┌─ Erro: %v", err))
		deps.AddError("└─")
	} else {
		// Salva tema nas preferências
		if err := deps.SaveTheme(themeKey); err != nil {
			deps.AddError(fmt.Sprintf("⚠️  Aviso: tema aplicado mas não foi salvo: %v", err))
		}

		var display string
		for _, th := range themes {
			if th.Key == themeKey {
				display = th.Name
				break
			}
		}
		if display == "" {
			display = themeKey
		}
		deps.AddInfo(fmt.Sprintf("✅ Tema alterado para: %s", display))
		deps.AddInfo("💾 Suas preferências foram salvas!")
	}
	deps.AddBlank()
	return true
}

func handleKeyCommand(deps Dependencies) bool {
	deps.AddInfo("┌─ 🔐 Atualizar Token do Hugging Face")
	deps.AddInfo("│")
	deps.AddInfo("│  Digite seu novo token abaixo (ou pressione Enter para cancelar):")
	deps.AddInfo("└─")
	deps.AddBlank()

	// Lê o novo token (mostra ******* enquanto digita)
	fmt.Print("🔐 Novo token: ")
	tokenBytes, err := term.ReadPassword(int(syscall.Stdin))
	newToken := strings.TrimSpace(string(tokenBytes))

	// Mostra feedback visual
	if len(newToken) > 0 {
		stars := strings.Repeat("*", len(newToken))
		fmt.Printf("\r🔐 Novo token: %s ✓\n", stars)
	} else {
		fmt.Println()
	}

	if err != nil {
		deps.AddError(fmt.Sprintf("❌ Erro ao ler token: %v", err))
		deps.AddBlank()
		return true
	}
	if newToken == "" {
		deps.AddInfo("⚠️  Operação cancelada (token vazio)")
		deps.AddBlank()
		return true
	}

	// Salva o novo token
	credManager, err := credentials.NewManager()
	if err != nil {
		deps.AddError(fmt.Sprintf("❌ Erro ao inicializar gerenciador: %v", err))
		deps.AddBlank()
		return true
	}

	if err := credManager.SaveToken(newToken); err != nil {
		deps.AddError(fmt.Sprintf("❌ Erro ao salvar token: %v", err))
		deps.AddBlank()
		return true
	}

	// Atualiza token em tempo real (se disponível)
	if deps.UpdateToken != nil {
		if err := deps.UpdateToken(newToken); err != nil {
			deps.AddError(fmt.Sprintf("⚠️  Erro ao atualizar token em tempo real: %v", err))
		}
	}

	deps.AddSuccess("┌─ ✅ Token atualizado com sucesso!")
	deps.AddSuccess("│")
	deps.AddSuccess("│  ✅ Token salvo de forma criptografada")
	deps.AddSuccess("│  ✅ Token ativo nesta sessão")
	deps.AddSuccess("│")
	deps.AddSuccess("│  Você já pode usar o novo token imediatamente!")
	deps.AddSuccess("└─")
	deps.AddBlank()
	return true
}

func handlePermissionsCommand(parts []string, deps Dependencies) bool {
	permManager, err := permissions.NewManager()
	if err != nil {
		deps.AddError(fmt.Sprintf("❌ Erro ao inicializar gerenciador de permissões: %v", err))
		deps.AddBlank()
		return true
	}

	// Se não tem argumentos, mostra status e opções
	if len(parts) == 1 {
		deps.AddInfo("┌─ 🔐 Gerenciamento de Permissões")
		deps.AddInfo("│")
		deps.AddInfo("│  Controle como o Amazzo pede permissão para usar ferramentas MCP.")
		deps.AddInfo("│")
		deps.AddInfo("│  Modos disponíveis:")
		deps.AddInfo("│")
		deps.AddInfo("│  /permitir sessao    - ✅ Permite TUDO durante esta sessão")
		deps.AddInfo("│  /permitir sempre    - ✅ Permite TUDO permanentemente")
		deps.AddInfo("│  /permitir perguntar - ❓ Pergunta antes de cada operação (padrão)")
		deps.AddInfo("│  /permitir negar     - ❌ Nega todas as operações")
		deps.AddInfo("│  /permitir reset     - 🔄 Reseta para o padrão (perguntar)")
		deps.AddInfo("│  /permitir status    - 📊 Mostra configuração atual")
		deps.AddInfo("│")
		deps.AddInfo("│  💡 Dica: Use '/permitir sessao' para evitar confirmar")
		deps.AddInfo("│           cada operação durante o desenvolvimento!")
		deps.AddInfo("└─")
		deps.AddBlank()
		return true
	}

	mode := strings.ToLower(parts[1])

	switch mode {
	case "sessao", "session":
		permManager.SetSessionMode(permissions.AllowSession)
		deps.AddSuccess("┌─ ✅ Permissões atualizadas")
		deps.AddSuccess("│")
		deps.AddSuccess("│  Modo: Permitir tudo durante esta sessão")
		deps.AddSuccess("│")
		deps.AddSuccess("│  ✓ Todas as ferramentas MCP serão executadas")
		deps.AddSuccess("│    automaticamente até você sair do Amazzo.")
		deps.AddSuccess("│")
		deps.AddSuccess("│  ⚠️  Esta configuração NÃO é permanente!")
		deps.AddSuccess("└─")

	case "sempre", "always":
		if err := permManager.SetDefaultMode(permissions.AllowAlways); err != nil {
			deps.AddError(fmt.Sprintf("❌ Erro ao salvar: %v", err))
		} else {
			deps.AddSuccess("┌─ ✅ Permissões atualizadas (PERMANENTE)")
			deps.AddSuccess("│")
			deps.AddSuccess("│  Modo: Permitir tudo sempre")
			deps.AddSuccess("│")
			deps.AddSuccess("│  ✓ Todas as ferramentas MCP serão executadas")
			deps.AddSuccess("│    automaticamente em todas as sessões.")
			deps.AddSuccess("│")
			deps.AddSuccess("│  ⚠️  Esta configuração é PERMANENTE!")
			deps.AddSuccess("│     Use '/permitir perguntar' para voltar ao padrão.")
			deps.AddSuccess("└─")
		}

	case "perguntar", "ask":
		if err := permManager.SetDefaultMode(permissions.AlwaysAsk); err != nil {
			deps.AddError(fmt.Sprintf("❌ Erro ao salvar: %v", err))
		} else {
			permManager.SetSessionMode(permissions.AlwaysAsk)
			deps.AddInfo("┌─ ✅ Permissões atualizadas")
			deps.AddInfo("│")
			deps.AddInfo("│  Modo: Perguntar antes de cada operação")
			deps.AddInfo("│")
			deps.AddInfo("│  ✓ Você será perguntado antes de cada ferramenta MCP")
			deps.AddInfo("└─")
		}

	case "negar", "deny":
		permManager.SetSessionMode(permissions.DenyAlways)
		deps.AddInfo("┌─ ⛔ Permissões atualizadas")
		deps.AddInfo("│")
		deps.AddInfo("│  Modo: Negar todas as operações")
		deps.AddInfo("│")
		deps.AddInfo("│  ✗ Todas as ferramentas MCP serão bloqueadas")
		deps.AddInfo("│    durante esta sessão.")
		deps.AddInfo("└─")

	case "reset":
		if err := permManager.Reset(); err != nil {
			deps.AddError(fmt.Sprintf("❌ Erro ao resetar: %v", err))
		} else {
			deps.AddSuccess("┌─ 🔄 Permissões resetadas")
			deps.AddSuccess("│")
			deps.AddSuccess("│  Todas as configurações voltaram ao padrão:")
			deps.AddSuccess("│  Perguntar antes de cada operação")
			deps.AddSuccess("└─")
		}

	case "status":
		status := permManager.GetStatus()
		deps.AddInfo(status)

	default:
		deps.AddError(fmt.Sprintf("❌ Modo desconhecido: %s", mode))
		deps.AddInfo("Use /permitir sem argumentos para ver os modos disponíveis")
	}

	deps.AddBlank()
	return true
}

func handleCreateAgentCommand(deps Dependencies) bool {
	if deps.StartAgentCreation == nil {
		deps.AddError("❌ Sistema de criação de agentes não disponível")
		deps.AddBlank()
		return true
	}

	deps.StartAgentCreation()
	return true
}

func handleEditAgentCommand(parts []string, deps Dependencies) bool {
	if deps.GetSpecializedAgentRegistry == nil {
		deps.AddError("❌ Sistema de agentes não disponível")
		deps.AddBlank()
		return true
	}

	if len(parts) < 2 {
		deps.AddError("❌ Uso: /editar-agente <nome>")
		deps.AddInfo("💡 Use /agentes para ver agentes disponíveis")
		deps.AddBlank()
		return true
	}

	agentName := strings.Join(parts[1:], " ")
	registry := deps.GetSpecializedAgentRegistry()

	// Type assertion segura
	if reg, ok := registry.(interface {
		GetAgent(string) (interface{}, bool)
		ListAgents() []interface{}
	}); ok {
		_, exists := reg.GetAgent(agentName)
		if !exists {
			deps.AddError(fmt.Sprintf("❌ Agente '%s' não encontrado", agentName))
			deps.AddInfo("💡 Use /agentes para ver agentes disponíveis")
			deps.AddBlank()
			return true
		}

		deps.AddInfo(fmt.Sprintf("📝 Edição de agentes via TUI estará disponível em breve."))
		deps.AddInfo(fmt.Sprintf("Por enquanto, você pode editar manualmente em: .amazzo/agents.json"))
		deps.AddInfo("")
		deps.AddInfo("Campos disponíveis para edição:")
		deps.AddInfo("  • name: Nome do agente")
		deps.AddInfo("  • expertise: Área de especialização")
		deps.AddInfo("  • model_name: Modelo LLM a usar")
		deps.AddInfo("  • enable_web_search: true/false")
		deps.AddInfo("  • documentation_urls: Lista de URLs")
		deps.AddInfo("  • custom_instructions: Instruções específicas")
		deps.AddBlank()
	} else {
		deps.AddError("❌ Erro ao acessar registro de agentes")
		deps.AddBlank()
	}

	return true
}

func handleDeleteAgentCommand(parts []string, deps Dependencies) bool {
	if deps.GetSpecializedAgentRegistry == nil {
		deps.AddError("❌ Sistema de agentes não disponível")
		deps.AddBlank()
		return true
	}

	if len(parts) < 2 {
		deps.AddError("❌ Uso: /deletar-agente <nome>")
		deps.AddInfo("💡 Use /agentes para ver agentes disponíveis")
		deps.AddBlank()
		return true
	}

	agentName := strings.Join(parts[1:], " ")

	deps.AddInfo(fmt.Sprintf("🗑️  Para deletar o agente '%s':", agentName))
	deps.AddInfo("1. Abra o arquivo: .amazzo/agents.json")
	deps.AddInfo(fmt.Sprintf("2. Remova o objeto com \"name\": \"%s\"", agentName))
	deps.AddInfo("3. Salve o arquivo")
	deps.AddInfo("4. Reinicie o Amazzo")
	deps.AddBlank()
	deps.AddInfo("💡 Interface de exclusão via TUI em breve!")
	deps.AddBlank()

	return true
}

// handleOllamaRefresh reconsulta o Ollama e recarrega a lista de modelos.
//
// Sem isso, um modelo baixado com `ollama pull` depois que o Amazzo abriu só
// aparecia ao reiniciar, porque a lista é lida uma única vez no arranque.
func handleOllamaRefresh(deps Dependencies) bool {
	if deps.RefreshLocalModels == nil {
		deps.AddError("❌ Comando /ollama não disponível nesta versão.")
		deps.AddBlank()
		return true
	}

	encontrados, err := deps.RefreshLocalModels()
	if err != nil {
		deps.AddError(fmt.Sprintf("┌─ Erro ao consultar o Ollama: %v", err))
		deps.AddError("│  Verifique se o serviço está no ar: ollama list")
		deps.AddError("└─")
		deps.AddBlank()
		return true
	}

	deps.AddSuccess(fmt.Sprintf("🔄 %d modelo(s) local(is) encontrado(s) no Ollama", encontrados))
	deps.AddBlank()

	models := deps.ListModels()
	atual := deps.GetCurrentModelIndex()
	for i, model := range models {
		prefix := "│  "
		if i == atual {
			prefix = "│ ▶"
		}
		deps.AddHelp(fmt.Sprintf("%s[%d] %s", prefix, i+1, model.DisplayName))
	}

	deps.AddBlank()
	deps.AddHelp("│  Use /modelo <número> para trocar")
	deps.AddBlank()
	return true
}

// handleWorkspaceChange mostra ou troca a pasta de trabalho.
//
// O agente só enxerga dentro dessa pasta — é o limite que impede o modelo de
// sair passeando pelo disco. Trocá-la é decisão sua, não dele.
func handleWorkspaceChange(parts []string, deps Dependencies) bool {
	if deps.WorkspaceRoot == nil || deps.SetWorkspaceRoot == nil {
		deps.AddError("❌ Comando /pasta não disponível nesta versão.")
		deps.AddBlank()
		return true
	}

	if len(parts) < 2 {
		deps.AddInfo("📁 Pasta de trabalho: " + deps.WorkspaceRoot())
		deps.AddHelp("   O agente só lê e escreve aqui dentro.")
		deps.AddHelp("   Use /pasta <caminho> para trabalhar em outro projeto.")
		deps.AddBlank()
		return true
	}

	destino := strings.Join(parts[1:], " ")
	if err := deps.SetWorkspaceRoot(destino); err != nil {
		deps.AddError(fmt.Sprintf("❌ Não consegui usar essa pasta: %v", err))
		deps.AddBlank()
		return true
	}

	deps.AddSuccess("📁 Pasta de trabalho: " + deps.WorkspaceRoot())
	deps.AddHelp("   A conversa recomeça, porque o contexto era do projeto anterior.")
	deps.AddBlank()
	return true
}
