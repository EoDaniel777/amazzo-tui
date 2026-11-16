package mcpclient

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"sync"
	"time"
)

// ServerConfig descreve um servidor MCP vindo de um arquivo de configuração.
type ServerConfig struct {
	Name    string
	Remote  bool
	Command []string
	URL     string
	Env     map[string]string
	Headers map[string]string
	Enabled bool
	Source  string // arquivo de onde veio
}

// Tool é uma ferramenta de um servidor, com o nome já qualificado.
type Tool struct {
	Server      string
	Remote      string // nome original no servidor
	Name        string // nome exposto ao modelo: servidor_ferramenta
	Description string
	Schema      map[string]interface{}
	ReadOnly    bool
}

// Status resume um servidor para a tela /mcp.
type Status struct {
	Name    string
	State   string // conectado | falhou | desligado
	Tools   int
	Err     string
	Source  string
	Trusted bool
}

type server struct {
	cfg   ServerConfig
	conn  transport
	tools []Tool
	err   error
}

// Manager mantém os servidores vivos e recarrega a configuração sem reiniciar
// a sessão -- o equivalente a um "source ~/.bashrc".
type Manager struct {
	mu           sync.RWMutex
	servers      map[string]*server
	order        []string
	paths        []string
	mtimes       map[string]time.Time
	settings     Settings
	dir          string
	logDir       string
	settingsFile string
}

// Settings são as escolhas do usuário que sobrevivem entre sessões.
type Settings struct {
	Disabled []string `json:"desligados,omitempty"`
	Trusted  []string `json:"confiaveis,omitempty"`
}

// NewManager cria o gerenciador. paths são os arquivos de configuração, em
// ordem de prioridade crescente (o último vence); settingsPath guarda
// desligados/confiáveis.
func NewManager(dir string, paths []string, settingsPath string) *Manager {
	m := &Manager{servers: map[string]*server{}, paths: paths, dir: dir,
		mtimes: map[string]time.Time{}, logDir: filepath.Join(os.TempDir(), "amazzo-mcp")}
	_ = os.MkdirAll(m.logDir, 0o700)
	m.settingsPath(settingsPath)
	return m
}

func (m *Manager) settingsPath(p string) {
	m.settingsFile = p
	if b, err := os.ReadFile(p); err == nil {
		_ = json.Unmarshal(b, &m.settings)
	}
}

func (m *Manager) saveSettings() {
	if m.settingsFile == "" {
		return
	}
	_ = os.MkdirAll(filepath.Dir(m.settingsFile), 0o755)
	if b, err := json.MarshalIndent(m.settings, "", "  "); err == nil {
		_ = os.WriteFile(m.settingsFile, b, 0o600)
	}
}

func contains(l []string, s string) bool {
	for _, x := range l {
		if x == s {
			return true
		}
	}
	return false
}

func remove(l []string, s string) []string {
	var out []string
	for _, x := range l {
		if x != s {
			out = append(out, x)
		}
	}
	return out
}

// LoadConfigs lê servidores de vários arquivos. Entende o formato do opencode
// ("mcp": {nome: {type, command[], url, environment}}) e o do Claude/.mcp.json
// ("mcpServers": {nome: {command, args, env}}).
func LoadConfigs(paths []string) []ServerConfig {
	byName := map[string]ServerConfig{}
	var order []string
	for _, p := range paths {
		raw, err := os.ReadFile(p)
		if err != nil {
			continue
		}
		var doc map[string]json.RawMessage
		if json.Unmarshal(raw, &doc) != nil {
			continue
		}
		for _, key := range []string{"mcp", "mcpServers"} {
			blob, ok := doc[key]
			if !ok {
				continue
			}
			var entries map[string]map[string]interface{}
			if json.Unmarshal(blob, &entries) != nil {
				continue
			}
			for name, e := range entries {
				cfg := ServerConfig{Name: name, Enabled: true, Source: p,
					Env: map[string]string{}, Headers: map[string]string{}}
				if v, ok := e["enabled"].(bool); ok {
					cfg.Enabled = v
				}
				if u, ok := e["url"].(string); ok && u != "" {
					cfg.Remote, cfg.URL = true, u
				}
				switch c := e["command"].(type) {
				case []interface{}:
					for _, x := range c {
						cfg.Command = append(cfg.Command, fmt.Sprint(x))
					}
				case string:
					cfg.Command = []string{c}
					if args, ok := e["args"].([]interface{}); ok {
						for _, x := range args {
							cfg.Command = append(cfg.Command, fmt.Sprint(x))
						}
					}
				}
				for _, k := range []string{"environment", "env"} {
					if env, ok := e[k].(map[string]interface{}); ok {
						for ek, ev := range env {
							cfg.Env[ek] = os.ExpandEnv(fmt.Sprint(ev))
						}
					}
				}
				if h, ok := e["headers"].(map[string]interface{}); ok {
					for hk, hv := range h {
						cfg.Headers[hk] = os.ExpandEnv(fmt.Sprint(hv))
					}
				}
				if !cfg.Remote && len(cfg.Command) == 0 {
					continue
				}
				if _, seen := byName[name]; !seen {
					order = append(order, name)
				}
				byName[name] = cfg
			}
		}
	}
	sort.Strings(order)
	out := make([]ServerConfig, 0, len(order))
	for _, n := range order {
		out = append(out, byName[n])
	}
	return out
}

// Reload relê a configuração e reinicia os servidores. Devolve um resumo.
// Servidores cuja configuração não mudou também são reiniciados: é isso que
// permite recuperar um servidor que travou ou foi recompilado.
func (m *Manager) Reload(ctx context.Context) []Status {
	cfgs := LoadConfigs(m.paths)

	m.mu.Lock()
	for _, s := range m.servers {
		if s.conn != nil {
			s.conn.close()
		}
	}
	m.servers = map[string]*server{}
	m.order = nil
	m.mtimes = map[string]time.Time{}
	for _, p := range m.paths {
		if st, err := os.Stat(p); err == nil {
			m.mtimes[p] = st.ModTime()
		}
	}
	for _, c := range cfgs {
		m.servers[c.Name] = &server{cfg: c}
		m.order = append(m.order, c.Name)
	}
	m.mu.Unlock()

	var wg sync.WaitGroup
	for _, name := range m.order {
		s := m.servers[name]
		if !s.cfg.Enabled || contains(m.settings.Disabled, name) {
			continue
		}
		wg.Add(1)
		go func(s *server) {
			defer wg.Done()
			m.connect(ctx, s)
		}(s)
	}
	wg.Wait()
	return m.Statuses()
}

func (m *Manager) connect(ctx context.Context, s *server) {
	cctx, cancel := context.WithTimeout(ctx, 25*time.Second)
	defer cancel()

	var conn transport
	if s.cfg.Remote {
		conn = newHTTP(s.cfg.URL, s.cfg.Headers)
	} else {
		sc, err := startStdio(s.cfg.Command, s.cfg.Env, m.dir,
			filepath.Join(m.logDir, sanitize(s.cfg.Name)+".log"))
		if err != nil {
			s.err = err
			return
		}
		conn = sc
	}

	_, err := conn.call(cctx, "initialize", map[string]interface{}{
		"protocolVersion": protocolVersion,
		"capabilities":    map[string]interface{}{},
		"clientInfo":      map[string]interface{}{"name": "amazzo", "version": "1.0"},
	})
	if err != nil {
		conn.close()
		s.err = fmt.Errorf("initialize: %w", err)
		return
	}
	_ = conn.notify("notifications/initialized", nil)

	var tools []Tool
	cursor := ""
	for page := 0; page < 20; page++ {
		params := map[string]interface{}{}
		if cursor != "" {
			params["cursor"] = cursor
		}
		raw, err := conn.call(cctx, "tools/list", params)
		if err != nil {
			conn.close()
			s.err = fmt.Errorf("tools/list: %w", err)
			return
		}
		var res struct {
			Tools []struct {
				Name        string                 `json:"name"`
				Description string                 `json:"description"`
				InputSchema map[string]interface{} `json:"inputSchema"`
				Annotations struct {
					ReadOnly bool `json:"readOnlyHint"`
				} `json:"annotations"`
			} `json:"tools"`
			Next string `json:"nextCursor"`
		}
		if err := json.Unmarshal(raw, &res); err != nil {
			conn.close()
			s.err = err
			return
		}
		for _, t := range res.Tools {
			tools = append(tools, Tool{
				Server: s.cfg.Name, Remote: t.Name,
				Name:        sanitize(s.cfg.Name) + "_" + sanitize(t.Name),
				Description: t.Description, Schema: t.InputSchema, ReadOnly: t.Annotations.ReadOnly,
			})
		}
		if res.Next == "" {
			break
		}
		cursor = res.Next
	}

	m.mu.Lock()
	s.conn, s.tools, s.err = conn, tools, nil
	m.mu.Unlock()
}

var nonName = regexp.MustCompile(`[^a-zA-Z0-9_]+`)

func sanitize(s string) string { return strings.Trim(nonName.ReplaceAllString(s, "_"), "_") }

// Changed diz se algum arquivo de configuração mudou desde o último Reload.
func (m *Manager) Changed() bool {
	m.mu.RLock()
	defer m.mu.RUnlock()
	for _, p := range m.paths {
		st, err := os.Stat(p)
		if err != nil {
			continue
		}
		if old, ok := m.mtimes[p]; !ok || !st.ModTime().Equal(old) {
			return true
		}
	}
	return false
}

// Tools devolve todas as ferramentas dos servidores conectados.
func (m *Manager) Tools() []Tool {
	m.mu.RLock()
	defer m.mu.RUnlock()
	var out []Tool
	for _, n := range m.order {
		out = append(out, m.servers[n].tools...)
	}
	return out
}

// Has diz se o nome pertence a uma ferramenta externa conectada.
func (m *Manager) Has(name string) bool {
	_, ok := m.find(name)
	return ok
}

func (m *Manager) find(name string) (Tool, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	for _, n := range m.order {
		for _, t := range m.servers[n].tools {
			if strings.EqualFold(t.Name, name) {
				return t, true
			}
		}
	}
	return Tool{}, false
}

// NeedsApproval diz se a chamada deve pedir permissão: sim, a menos que o
// servidor seja confiável ou a ferramenta se declare somente-leitura.
func (m *Manager) NeedsApproval(name string) (bool, string) {
	t, ok := m.find(name)
	if !ok {
		return false, ""
	}
	m.mu.RLock()
	trusted := contains(m.settings.Trusted, t.Server)
	m.mu.RUnlock()
	if trusted || t.ReadOnly {
		return false, ""
	}
	return true, fmt.Sprintf("ferramenta externa %q do servidor %s", t.Remote, t.Server)
}

// Call executa a ferramenta e devolve o texto do resultado.
func (m *Manager) Call(ctx context.Context, name string, args map[string]interface{}) (string, error) {
	t, ok := m.find(name)
	if !ok {
		return "", fmt.Errorf("ferramenta %q não existe (use mcp_reload se o servidor acabou de mudar)", name)
	}
	m.mu.RLock()
	s := m.servers[t.Server]
	conn := s.conn
	m.mu.RUnlock()
	if conn == nil {
		return "", fmt.Errorf("servidor %s está desconectado", t.Server)
	}
	if args == nil {
		args = map[string]interface{}{}
	}
	raw, err := conn.call(ctx, "tools/call", map[string]interface{}{"name": t.Remote, "arguments": args})
	if err != nil {
		return "", err
	}
	var res struct {
		Content []struct {
			Type string `json:"type"`
			Text string `json:"text"`
		} `json:"content"`
		IsError    bool            `json:"isError"`
		Structured json.RawMessage `json:"structuredContent"`
	}
	if err := json.Unmarshal(raw, &res); err != nil {
		return string(raw), nil
	}
	var parts []string
	for _, c := range res.Content {
		switch c.Type {
		case "text", "":
			parts = append(parts, c.Text)
		default:
			parts = append(parts, "["+c.Type+" omitido]")
		}
	}
	text := strings.Join(parts, "\n")
	if text == "" && len(res.Structured) > 0 {
		text = string(res.Structured)
	}
	if res.IsError {
		return text, errors.New(strings.TrimSpace(text))
	}
	return text, nil
}

// Statuses lista todos os servidores conhecidos.
func (m *Manager) Statuses() []Status {
	m.mu.RLock()
	defer m.mu.RUnlock()
	var out []Status
	for _, n := range m.order {
		s := m.servers[n]
		st := Status{Name: n, Tools: len(s.tools), Source: s.cfg.Source, Trusted: contains(m.settings.Trusted, n)}
		switch {
		case !s.cfg.Enabled || contains(m.settings.Disabled, n):
			st.State = "desligado"
		case s.err != nil:
			st.State, st.Err = "falhou", s.err.Error()
		case s.conn != nil:
			st.State = "conectado"
		default:
			st.State = "iniciando"
		}
		out = append(out, st)
	}
	return out
}

// SetEnabled liga/desliga um servidor de forma persistente e reconecta.
func (m *Manager) SetEnabled(ctx context.Context, name string, on bool) error {
	m.mu.Lock()
	s, ok := m.servers[name]
	if !ok {
		m.mu.Unlock()
		return fmt.Errorf("servidor %q não existe", name)
	}
	if on {
		m.settings.Disabled = remove(m.settings.Disabled, name)
	} else if !contains(m.settings.Disabled, name) {
		m.settings.Disabled = append(m.settings.Disabled, name)
	}
	m.saveSettings()
	if s.conn != nil {
		s.conn.close()
		s.conn, s.tools = nil, nil
	}
	m.mu.Unlock()
	if on {
		s.err = nil
		m.connect(ctx, s)
	}
	return nil
}

// SetTrusted marca um servidor como confiável (sem pedir permissão a cada chamada).
func (m *Manager) SetTrusted(name string, on bool) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if _, ok := m.servers[name]; !ok {
		return fmt.Errorf("servidor %q não existe", name)
	}
	if on && !contains(m.settings.Trusted, name) {
		m.settings.Trusted = append(m.settings.Trusted, name)
	} else if !on {
		m.settings.Trusted = remove(m.settings.Trusted, name)
	}
	m.saveSettings()
	return nil
}

// Close encerra todos os servidores.
func (m *Manager) Close() {
	m.mu.Lock()
	defer m.mu.Unlock()
	for _, s := range m.servers {
		if s.conn != nil {
			s.conn.close()
		}
	}
}

// Configure troca a pasta de trabalho e os arquivos de configuração (ao mudar
// de projeto). Vale a partir do próximo Reload.
func (m *Manager) Configure(dir string, paths []string) {
	m.mu.Lock()
	m.dir, m.paths = dir, paths
	m.mu.Unlock()
}
