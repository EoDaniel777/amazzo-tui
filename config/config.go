package config

import (
	"encoding/json"
	"os"
	"path/filepath"
)

// UserConfig armazena as preferências do usuário.
type UserConfig struct {
	Theme      string  `json:"theme"`
	LastModel  int     `json:"last_model"`
	FontSize   int     `json:"font_size"`
	ShowTips   bool    `json:"show_tips"`
	FirstRun   bool    `json:"first_run"`
}

// Manager gerencia configurações do usuário.
type Manager struct {
	configPath string
	config     UserConfig
}

// NewManager cria um novo gerenciador de configurações.
func NewManager(workspaceRoot string) (*Manager, error) {
	configDir := filepath.Join(workspaceRoot, ".amazzo")
	configPath := filepath.Join(configDir, "config.json")

	// Cria diretório se não existir
	if err := os.MkdirAll(configDir, 0755); err != nil {
		return nil, err
	}

	m := &Manager{
		configPath: configPath,
		config: UserConfig{
			Theme:     "default",
			LastModel: 0,
			FontSize:  100, // 100% = tamanho padrão
			ShowTips:  true,
			FirstRun:  true,
		},
	}

	// Tenta carregar config existente
	if err := m.Load(); err != nil {
		// Se não existir, cria um novo
		if os.IsNotExist(err) {
			if err := m.Save(); err != nil {
				return nil, err
			}
		}
	}

	return m, nil
}

// Load carrega configurações do disco.
func (m *Manager) Load() error {
	data, err := os.ReadFile(m.configPath)
	if err != nil {
		return err
	}

	return json.Unmarshal(data, &m.config)
}

// Save salva configurações no disco.
func (m *Manager) Save() error {
	data, err := json.MarshalIndent(m.config, "", "  ")
	if err != nil {
		return err
	}

	return os.WriteFile(m.configPath, data, 0644)
}

// GetTheme retorna o tema salvo.
func (m *Manager) GetTheme() string {
	return m.config.Theme
}

// SetTheme salva o tema escolhido.
func (m *Manager) SetTheme(theme string) error {
	m.config.Theme = theme
	return m.Save()
}

// GetLastModel retorna o último modelo usado.
func (m *Manager) GetLastModel() int {
	return m.config.LastModel
}

// SetLastModel salva o último modelo usado.
func (m *Manager) SetLastModel(model int) error {
	m.config.LastModel = model
	return m.Save()
}

// GetFontSize retorna o tamanho da fonte (percentual).
func (m *Manager) GetFontSize() int {
	return m.config.FontSize
}

// SetFontSize salva o tamanho da fonte.
func (m *Manager) SetFontSize(size int) error {
	if size < 50 {
		size = 50
	}
	if size > 200 {
		size = 200
	}
	m.config.FontSize = size
	return m.Save()
}

// AdjustFontSize ajusta o tamanho da fonte por incremento.
func (m *Manager) AdjustFontSize(delta int) error {
	return m.SetFontSize(m.config.FontSize + delta)
}

// IsFirstRun verifica se é a primeira execução.
func (m *Manager) IsFirstRun() bool {
	return m.config.FirstRun
}

// MarkFirstRunComplete marca que o primeiro run foi completo.
func (m *Manager) MarkFirstRunComplete() error {
	m.config.FirstRun = false
	return m.Save()
}

// ShouldShowTips verifica se deve mostrar dicas.
func (m *Manager) ShouldShowTips() bool {
	return m.config.ShowTips
}

// SetShowTips define se deve mostrar dicas.
func (m *Manager) SetShowTips(show bool) error {
	m.config.ShowTips = show
	return m.Save()
}
