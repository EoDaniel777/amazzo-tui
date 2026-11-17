package mcpclient

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// GlobalConfigPath é o arquivo de servidores MCP do Amazzo (vale para todos os projetos).
func GlobalConfigPath() string {
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".config", "amazzo", "mcp.json")
}

// ProjectConfigPath é o arquivo de servidores MCP de um projeto.
func ProjectConfigPath(workspace string) string {
	return filepath.Join(workspace, ".amazzo", "mcp.json")
}

// ConfigPaths lista os únicos arquivos que o Amazzo lê: o global e o do
// projeto (o do projeto vence). Configurações de outras ferramentas
// (opencode, .mcp.json) não são lidas.
func ConfigPaths(workspace string) []string {
	paths := []string{GlobalConfigPath()}
	if workspace != "" {
		paths = append(paths, ProjectConfigPath(workspace))
	}
	return paths
}

// Presets são servidores conhecidos que dispensam argumentos: `amazzo add mcp context7`.
var Presets = map[string]ServerConfig{
	"context7": {Command: []string{"npx", "-y", "@upstash/context7-mcp"}},
}

// PresetNames devolve os nomes de Presets, em ordem.
func PresetNames() []string {
	var n []string
	for k := range Presets {
		n = append(n, k)
	}
	sort.Strings(n)
	return n
}

func readDoc(path string) (map[string]json.RawMessage, error) {
	doc := map[string]json.RawMessage{}
	raw, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return doc, nil
	}
	if err != nil {
		return nil, err
	}
	if len(strings.TrimSpace(string(raw))) == 0 {
		return doc, nil
	}
	if err := json.Unmarshal(raw, &doc); err != nil {
		return nil, fmt.Errorf("%s não é um JSON válido: %w", path, err)
	}
	return doc, nil
}

func writeDoc(path string, doc map[string]json.RawMessage) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	b, err := json.MarshalIndent(doc, "", "  ")
	if err != nil {
		return err
	}
	// 0600: o arquivo pode ter chaves de API em env/headers.
	return os.WriteFile(path, append(b, '\n'), 0o600)
}

func readServers(doc map[string]json.RawMessage) (map[string]map[string]interface{}, error) {
	servers := map[string]map[string]interface{}{}
	if blob, ok := doc["mcpServers"]; ok {
		if err := json.Unmarshal(blob, &servers); err != nil {
			return nil, fmt.Errorf("chave mcpServers inválida: %w", err)
		}
	}
	return servers, nil
}

// AddServer grava (ou substitui) um servidor no arquivo, no formato
// {"mcpServers": {nome: {command, args, env} | {url, headers}}}.
func AddServer(path string, c ServerConfig) error {
	if strings.TrimSpace(c.Name) == "" {
		return fmt.Errorf("informe o nome do servidor")
	}
	if !c.Remote && len(c.Command) == 0 {
		return fmt.Errorf("informe um comando ou uma URL")
	}
	doc, err := readDoc(path)
	if err != nil {
		return err
	}
	servers, err := readServers(doc)
	if err != nil {
		return err
	}
	e := map[string]interface{}{}
	if c.Remote {
		e["url"] = c.URL
		if len(c.Headers) > 0 {
			e["headers"] = c.Headers
		}
	} else {
		e["command"] = c.Command[0]
		if len(c.Command) > 1 {
			e["args"] = c.Command[1:]
		}
		if len(c.Env) > 0 {
			e["env"] = c.Env
		}
	}
	servers[c.Name] = e
	blob, _ := json.Marshal(servers)
	doc["mcpServers"] = blob
	return writeDoc(path, doc)
}

// RemoveServer apaga um servidor do arquivo. Devolve false se ele não estava lá.
func RemoveServer(path, name string) (bool, error) {
	doc, err := readDoc(path)
	if err != nil {
		return false, err
	}
	servers, err := readServers(doc)
	if err != nil {
		return false, err
	}
	if _, ok := servers[name]; !ok {
		return false, nil
	}
	delete(servers, name)
	blob, _ := json.Marshal(servers)
	doc["mcpServers"] = blob
	return true, writeDoc(path, doc)
}

// ParseAddArgs entende os argumentos de `amazzo add mcp` e de `/mcp add`:
//
//	<nome>                         servidor conhecido (ex.: context7)
//	<nome> <url>                   servidor remoto (HTTP)
//	<nome> [opções] -- cmd args…   servidor local (stdio)
//
// Opções: --url U, --env K=V, --header K=V, --projeto (grava em .amazzo/mcp.json).
func ParseAddArgs(args []string) (cfg ServerConfig, project bool, err error) {
	cfg.Env, cfg.Headers = map[string]string{}, map[string]string{}
	var rest []string
	for i := 0; i < len(args); i++ {
		a := args[i]
		if a == "--" {
			cfg.Command = append(cfg.Command, args[i+1:]...)
			break
		}
		next := func() (string, error) {
			if i+1 >= len(args) {
				return "", fmt.Errorf("%s precisa de um valor", a)
			}
			i++
			return args[i], nil
		}
		switch a {
		case "--projeto", "--project":
			project = true
		case "--url":
			if cfg.URL, err = next(); err != nil {
				return
			}
			cfg.Remote = true
		case "--env", "--header":
			var kv string
			if kv, err = next(); err != nil {
				return
			}
			k, v, ok := strings.Cut(kv, "=")
			if !ok || k == "" {
				err = fmt.Errorf("%s espera CHAVE=VALOR, recebi %q", a, kv)
				return
			}
			if a == "--env" {
				cfg.Env[k] = v
			} else {
				cfg.Headers[k] = v
			}
		default:
			rest = append(rest, a)
		}
	}
	if len(rest) == 0 {
		err = fmt.Errorf("informe o nome do servidor")
		return
	}
	cfg.Name = rest[0]
	if len(rest) > 1 && !cfg.Remote && len(cfg.Command) == 0 {
		if strings.HasPrefix(rest[1], "http://") || strings.HasPrefix(rest[1], "https://") {
			cfg.URL, cfg.Remote = rest[1], true
		} else {
			err = fmt.Errorf("para servidor local use: %s -- <comando> [args…]", cfg.Name)
			return
		}
	}
	if !cfg.Remote && len(cfg.Command) == 0 {
		p, ok := Presets[strings.ToLower(cfg.Name)]
		if !ok {
			err = fmt.Errorf("%q não é um servidor conhecido (%s); passe uma URL ou -- <comando>",
				cfg.Name, strings.Join(PresetNames(), ", "))
			return
		}
		cfg.Command = p.Command
	}
	return
}
