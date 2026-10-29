package permissions

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sync"
)

// Mode define o modo de permissão
type Mode string

const (
	// AlwaysAsk sempre pede confirmação (padrão)
	AlwaysAsk Mode = "always_ask"
	// AllowSession permite todas as operações durante a sessão atual
	AllowSession Mode = "allow_session"
	// AllowAlways permite todas as operações permanentemente
	AllowAlways Mode = "allow_always"
	// DenyAlways nega todas as operações permanentemente
	DenyAlways Mode = "deny_always"
)

// Config armazena as configurações de permissões
type Config struct {
	DefaultMode Mode              `json:"default_mode"`
	ToolModes   map[string]Mode   `json:"tool_modes"` // Permissões por ferramenta
}

// Manager gerencia permissões de ferramentas MCP
type Manager struct {
	configPath    string
	config        Config
	sessionMode   Mode // Modo temporário da sessão atual
	mu            sync.RWMutex
}

// NewManager cria um novo gerenciador de permissões
func NewManager() (*Manager, error) {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return nil, err
	}

	configDir := filepath.Join(homeDir, ".config", "amazzo")
	if err := os.MkdirAll(configDir, 0755); err != nil {
		return nil, err
	}

	configPath := filepath.Join(configDir, "permissions.json")

	m := &Manager{
		configPath:  configPath,
		sessionMode: AlwaysAsk, // Padrão: sempre pedir
		config: Config{
			DefaultMode: AlwaysAsk,
			ToolModes:   make(map[string]Mode),
		},
	}

	// Tenta carregar config existente
	if err := m.load(); err != nil && !os.IsNotExist(err) {
		// Se erro não for "arquivo não existe", retorna erro
		return nil, err
	}

	return m, nil
}

// load carrega as permissões do disco
func (m *Manager) load() error {
	data, err := os.ReadFile(m.configPath)
	if err != nil {
		return err
	}

	return json.Unmarshal(data, &m.config)
}

// save salva as permissões no disco
func (m *Manager) save() error {
	data, err := json.MarshalIndent(m.config, "", "  ")
	if err != nil {
		return err
	}

	return os.WriteFile(m.configPath, data, 0644)
}

// ShouldAsk verifica se deve pedir permissão para uma ferramenta
func (m *Manager) ShouldAsk(tool string) bool {
	m.mu.RLock()
	defer m.mu.RUnlock()

	// Verifica modo da sessão primeiro
	if m.sessionMode == AllowSession {
		return false
	}
	if m.sessionMode == DenyAlways {
		return false // Vai negar automaticamente
	}

	// Verifica modo específico da ferramenta
	if mode, ok := m.config.ToolModes[tool]; ok {
		return mode == AlwaysAsk
	}

	// Usa modo padrão
	return m.config.DefaultMode == AlwaysAsk
}

// ShouldAllow verifica se deve permitir automaticamente
func (m *Manager) ShouldAllow(tool string) bool {
	m.mu.RLock()
	defer m.mu.RUnlock()

	// Verifica modo da sessão
	if m.sessionMode == AllowSession || m.sessionMode == AllowAlways {
		return true
	}

	// Verifica modo específico da ferramenta
	if mode, ok := m.config.ToolModes[tool]; ok {
		return mode == AllowAlways
	}

	// Usa modo padrão
	return m.config.DefaultMode == AllowAlways
}

// ShouldDeny verifica se deve negar automaticamente
func (m *Manager) ShouldDeny(tool string) bool {
	m.mu.RLock()
	defer m.mu.RUnlock()

	// Verifica modo da sessão
	if m.sessionMode == DenyAlways {
		return true
	}

	// Verifica modo específico da ferramenta
	if mode, ok := m.config.ToolModes[tool]; ok {
		return mode == DenyAlways
	}

	// Usa modo padrão
	return m.config.DefaultMode == DenyAlways
}

// SetSessionMode define o modo para a sessão atual
func (m *Manager) SetSessionMode(mode Mode) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.sessionMode = mode
}

// SetDefaultMode define o modo padrão (permanente)
func (m *Manager) SetDefaultMode(mode Mode) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.config.DefaultMode = mode
	return m.save()
}

// SetToolMode define o modo para uma ferramenta específica (permanente)
func (m *Manager) SetToolMode(tool string, mode Mode) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.config.ToolModes[tool] = mode
	return m.save()
}

// GetCurrentMode retorna o modo efetivo atual
func (m *Manager) GetCurrentMode() Mode {
	m.mu.RLock()
	defer m.mu.RUnlock()

	if m.sessionMode != AlwaysAsk {
		return m.sessionMode
	}

	return m.config.DefaultMode
}

// Reset reseta todas as permissões para o padrão
func (m *Manager) Reset() error {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.sessionMode = AlwaysAsk
	m.config.DefaultMode = AlwaysAsk
	m.config.ToolModes = make(map[string]Mode)

	return m.save()
}

// GetStatus retorna um resumo das permissões atuais
func (m *Manager) GetStatus() string {
	m.mu.RLock()
	defer m.mu.RUnlock()

	status := "┌─ 🔐 Status de Permissões\n"
	status += "│\n"
	status += "│  Sessão atual: " + string(m.sessionMode) + "\n"
	status += "│  Modo padrão:  " + string(m.config.DefaultMode) + "\n"

	if len(m.config.ToolModes) > 0 {
		status += "│\n"
		status += "│  Ferramentas específicas:\n"
		for tool, mode := range m.config.ToolModes {
			status += "│    " + tool + ": " + string(mode) + "\n"
		}
	}

	status += "└─"
	return status
}
