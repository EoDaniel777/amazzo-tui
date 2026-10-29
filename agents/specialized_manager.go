package agents

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
)

// SpecializedAgentRegistry gerencia agentes especializados do usuário.
type SpecializedAgentRegistry struct {
	agents     map[string]*SpecializedAgent
	configPath string
	mu         sync.RWMutex
}

// AgentProfile define o perfil de um agente especializado salvo.
type AgentProfile struct {
	Name              string        `json:"name"`
	Expertise         ExpertiseArea `json:"expertise"`
	ModelName         string        `json:"model_name"`
	EnableWebSearch   bool          `json:"enable_web_search"`
	DocumentationURLs []string      `json:"documentation_urls"`
	CustomInstructions string       `json:"custom_instructions"`
	IsActive          bool          `json:"is_active"`
}

// NewSpecializedAgentRegistry cria um novo registry de agentes.
func NewSpecializedAgentRegistry(workspaceDir string) *SpecializedAgentRegistry {
	configPath := filepath.Join(workspaceDir, ".amazzo", "agents.json")

	return &SpecializedAgentRegistry{
		agents:     make(map[string]*SpecializedAgent),
		configPath: configPath,
	}
}

// LoadAgents carrega agentes do arquivo de configuração.
func (sar *SpecializedAgentRegistry) LoadAgents() error {
	sar.mu.Lock()
	defer sar.mu.Unlock()

	// Verifica se arquivo existe
	if _, err := os.Stat(sar.configPath); os.IsNotExist(err) {
		// Cria agentes padrão
		return sar.createDefaultAgents()
	}

	data, err := os.ReadFile(sar.configPath)
	if err != nil {
		return fmt.Errorf("erro ao ler configuração: %w", err)
	}

	var profiles []AgentProfile
	if err := json.Unmarshal(data, &profiles); err != nil {
		return fmt.Errorf("erro ao parsear configuração: %w", err)
	}

	// Cria agentes a partir dos profiles
	for _, profile := range profiles {
		if !profile.IsActive {
			continue
		}

		config := SpecializedAgentConfig{
			Expertise:          profile.Expertise,
			ModelName:          profile.ModelName,
			EnableWebSearch:    profile.EnableWebSearch,
			DocumentationURLs:  profile.DocumentationURLs,
			CustomInstructions: profile.CustomInstructions,
		}

		agent := NewSpecializedAgent(profile.Name, config)
		sar.agents[profile.Name] = agent
	}

	return nil
}

// SaveAgents salva os agentes no arquivo de configuração.
func (sar *SpecializedAgentRegistry) SaveAgents() error {
	sar.mu.RLock()
	defer sar.mu.RUnlock()
	return sar.saveAgentsUnsafe()
}

// saveAgentsUnsafe salva os agentes sem adquirir lock (uso interno).
func (sar *SpecializedAgentRegistry) saveAgentsUnsafe() error {
	// Converte agentes para profiles
	var profiles []AgentProfile
	for name, agent := range sar.agents {
		profile := AgentProfile{
			Name:               name,
			Expertise:          agent.Expertise,
			ModelName:          agent.ModelName,
			EnableWebSearch:    agent.CanSearchWeb,
			DocumentationURLs:  agent.DocumentationURLs,
			CustomInstructions: "", // TODO: extrair de SystemPrompt
			IsActive:           true,
		}
		profiles = append(profiles, profile)
	}

	// Serializa para JSON
	data, err := json.MarshalIndent(profiles, "", "  ")
	if err != nil {
		return fmt.Errorf("erro ao serializar: %w", err)
	}

	// Cria diretório se não existir
	dir := filepath.Dir(sar.configPath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("erro ao criar diretório: %w", err)
	}

	// Salva arquivo
	if err := os.WriteFile(sar.configPath, data, 0644); err != nil {
		return fmt.Errorf("erro ao salvar: %w", err)
	}

	return nil
}

// Register registra um novo agente a partir de AgentConfig (do UI).
func (sar *SpecializedAgentRegistry) Register(config *AgentConfig) error {
	sar.mu.Lock()
	defer sar.mu.Unlock()

	// Verifica se já existe
	if _, exists := sar.agents[config.Name]; exists {
		return fmt.Errorf("agente '%s' já existe", config.Name)
	}

	// Converte para SpecializedAgentConfig
	specializedConfig := SpecializedAgentConfig{
		Expertise:          ExpertiseArea(config.Expertise),
		ModelName:          "qwen2.5-coder-32b", // modelo padrão
		EnableWebSearch:    len(config.DocURLs) > 0,
		DocumentationURLs:  config.DocURLs,
		CustomInstructions: config.SystemPrompt,
	}

	agent := NewSpecializedAgent(config.Name, specializedConfig)
	sar.agents[config.Name] = agent

	// Salva no disco
	if err := sar.SaveAgents(); err != nil {
		return fmt.Errorf("erro ao salvar: %w", err)
	}

	return nil
}

// CreateAgent cria um novo agente especializado.
func (sar *SpecializedAgentRegistry) CreateAgent(name string, config SpecializedAgentConfig) (*SpecializedAgent, error) {
	sar.mu.Lock()
	defer sar.mu.Unlock()

	// Verifica se já existe
	if _, exists := sar.agents[name]; exists {
		return nil, fmt.Errorf("agente '%s' já existe", name)
	}

	agent := NewSpecializedAgent(name, config)
	sar.agents[name] = agent

	// Salva no disco
	if err := sar.SaveAgents(); err != nil {
		return nil, fmt.Errorf("erro ao salvar: %w", err)
	}

	return agent, nil
}

// GetAgent retorna um agente pelo nome.
func (sar *SpecializedAgentRegistry) GetAgent(name string) (*SpecializedAgent, bool) {
	sar.mu.RLock()
	defer sar.mu.RUnlock()

	agent, exists := sar.agents[name]
	return agent, exists
}

// ListAgents retorna todos os agentes.
func (sar *SpecializedAgentRegistry) ListAgents() []*SpecializedAgent {
	sar.mu.RLock()
	defer sar.mu.RUnlock()

	agents := make([]*SpecializedAgent, 0, len(sar.agents))
	for _, agent := range sar.agents {
		agents = append(agents, agent)
	}
	return agents
}

// DeleteAgent remove um agente.
func (sar *SpecializedAgentRegistry) DeleteAgent(name string) error {
	sar.mu.Lock()
	defer sar.mu.Unlock()

	if _, exists := sar.agents[name]; !exists {
		return fmt.Errorf("agente '%s' não encontrado", name)
	}

	delete(sar.agents, name)

	return sar.SaveAgents()
}

// GetBestAgentForTask retorna o melhor agente para uma tarefa.
func (sar *SpecializedAgentRegistry) GetBestAgentForTask(task *Task) *SpecializedAgent {
	sar.mu.RLock()
	defer sar.mu.RUnlock()

	// Analisa a descrição e filepath para determinar expertise necessária
	description := task.Description + " " + task.FilePath

	// Mapeamento de keywords para expertise
	expertiseKeywords := map[ExpertiseArea][]string{
		ExpertiseBubblePlugins: {"bubble", "plugin", "iframe"},
		ExpertiseGo:            {".go", "golang", "go "},
		ExpertiseRust:          {".rs", "rust", "cargo"},
		ExpertisePython:        {".py", "python", "django", "flask"},
		ExpertiseJavaScript:    {".js", ".ts", "javascript", "typescript", "react", "vue", "node"},
		ExpertiseFrontend:      {".html", ".css", "frontend", "ui"},
		ExpertiseDocumentation: {".md", "readme", "documentation", "docs", "documenta", "api.md", "architecture", "contributing"},
	}

	// Conta matches
	scores := make(map[string]int)
	for name, agent := range sar.agents {
		if !agent.IsAvailable {
			continue
		}

		keywords := expertiseKeywords[agent.Expertise]
		score := 0
		for _, keyword := range keywords {
			if containsCaseInsensitive(description, keyword) {
				score++
			}
		}
		scores[name] = score
	}

	// Retorna o agente com maior score
	var bestAgent *SpecializedAgent
	maxScore := 0
	for name, score := range scores {
		if score > maxScore {
			maxScore = score
			bestAgent = sar.agents[name]
		}
	}

	return bestAgent
}

// createDefaultAgents cria agentes padrão na primeira execução.
// IMPORTANTE: Esta função assume que o caller já adquiriu o lock!
func (sar *SpecializedAgentRegistry) createDefaultAgents() error {
	// Agente Bubble Plugin Expert
	bubbleAgent := NewSpecializedAgent("bubble-expert", SpecializedAgentConfig{
		Expertise:   ExpertiseBubblePlugins,
		ModelName:   "qwen2.5-coder-32b",
		EnableWebSearch: true,
		DocumentationURLs: []string{
			"https://manual.bubble.io",
			"https://manual.bubble.io/core-resources/api/the-bubble-api",
			"https://manual.bubble.io/help-guides/building-plugins",
			"https://forum.bubble.io/",
		},
		CustomInstructions: "Especialista em criar e debugar plugins Bubble.io com JavaScript, HTML e CSS.",
	})
	sar.agents["bubble-expert"] = bubbleAgent

	// Agente Go Expert
	goAgent := NewSpecializedAgent("go-expert", SpecializedAgentConfig{
		Expertise:   ExpertiseGo,
		ModelName:   "qwen2.5-coder-32b",
		EnableWebSearch: true,
		DocumentationURLs: []string{
			"https://go.dev/doc/",
			"https://pkg.go.dev/std",
		},
		CustomInstructions: "Especialista em Go idiomático com foco em performance e concurrency.",
	})
	sar.agents["go-expert"] = goAgent

	// Agente Documentation Expert
	docAgent := NewSpecializedAgent("doc-expert", SpecializedAgentConfig{
		Expertise:   ExpertiseDocumentation,
		ModelName:   "qwen2.5-coder-32b",
		EnableWebSearch: false,
		DocumentationURLs: []string{},
		CustomInstructions: "Especialista em criar documentação técnica completa, clara e profissional.",
	})
	sar.agents["doc-expert"] = docAgent

	// Usa versão sem lock já que o caller (LoadAgents) já tem o lock
	return sar.saveAgentsUnsafe()
}

// containsCaseInsensitive verifica se uma string contém outra (case insensitive).
func containsCaseInsensitive(s, substr string) bool {
	s = toLower(s)
	substr = toLower(substr)
	return len(s) >= len(substr) &&
		(s == substr || len(s) > len(substr) && indexOf(s, substr) >= 0)
}

func toLower(s string) string {
	result := make([]rune, len(s))
	for i, r := range s {
		if r >= 'A' && r <= 'Z' {
			result[i] = r + 32
		} else {
			result[i] = r
		}
	}
	return string(result)
}

func indexOf(s, substr string) int {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return i
		}
	}
	return -1
}

// ExecuteTaskWithSpecializedAgent executa uma tarefa com o melhor agente especializado.
func (sar *SpecializedAgentRegistry) ExecuteTaskWithSpecializedAgent(
	ctx context.Context,
	task *Task,
	api AgentAPI,
) error {
	// Encontra o melhor agente
	agent := sar.GetBestAgentForTask(task)
	if agent == nil {
		return fmt.Errorf("nenhum agente especializado disponível")
	}

	api.LogEvent(fmt.Sprintf("🎯 Usando agente especializado: %s (%s)", agent.ID, agent.Expertise))

	// Executa a tarefa
	return agent.ExecuteTask(ctx, task, api)
}
