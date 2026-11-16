package mcpclient

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// Servidor MCP de mentira: o próprio binário de teste, em modo "helper".
func TestMain(m *testing.M) {
	if os.Getenv("FAKE_MCP") == "1" {
		fakeServer()
		return
	}
	os.Exit(m.Run())
}

func fakeServer() {
	in := bufio.NewReaderSize(os.Stdin, 1<<20)
	for {
		line, err := in.ReadBytes('\n')
		if err != nil {
			return
		}
		var req map[string]interface{}
		if json.Unmarshal(line, &req) != nil {
			continue
		}
		id, hasID := req["id"]
		if !hasID {
			continue
		}
		var result interface{}
		switch req["method"] {
		case "initialize":
			result = map[string]interface{}{"protocolVersion": "2025-03-26", "capabilities": map[string]interface{}{}}
		case "tools/list":
			result = map[string]interface{}{"tools": []interface{}{
				map[string]interface{}{"name": "soma", "description": "soma dois numeros",
					"inputSchema": map[string]interface{}{"type": "object"}},
				map[string]interface{}{"name": "ler-status", "description": "so leitura",
					"inputSchema":  map[string]interface{}{"type": "object"},
					"annotations": map[string]interface{}{"readOnlyHint": true}},
			}}
		case "tools/call":
			p := req["params"].(map[string]interface{})
			a, _ := p["arguments"].(map[string]interface{})
			result = map[string]interface{}{"content": []interface{}{
				map[string]interface{}{"type": "text", "text": fmt.Sprintf("%v+%v=%v", a["a"], a["b"], a["a"].(float64)+a["b"].(float64))}}}
		default:
			result = map[string]interface{}{}
		}
		b, _ := json.Marshal(map[string]interface{}{"jsonrpc": "2.0", "id": id, "result": result})
		fmt.Println(string(b))
	}
}

func escreveConfig(t *testing.T, dir string) string {
	exe, _ := os.Executable()
	cfg := map[string]interface{}{"mcp": map[string]interface{}{
		"fake": map[string]interface{}{"type": "local", "command": []string{exe}, "enabled": true,
			"environment": map[string]string{"FAKE_MCP": "1"}},
		"quebrado": map[string]interface{}{"type": "local", "command": []string{"/nao/existe"}, "enabled": true},
	}}
	b, _ := json.Marshal(cfg)
	p := filepath.Join(dir, "opencode.json")
	if err := os.WriteFile(p, b, 0o644); err != nil {
		t.Fatal(err)
	}
	return p
}

func TestConectaListaEChama(t *testing.T) {
	dir := t.TempDir()
	cfg := escreveConfig(t, dir)
	m := NewManager(dir, []string{cfg}, filepath.Join(dir, "set.json"))
	defer m.Close()

	st := m.Reload(context.Background())
	estados := map[string]string{}
	for _, s := range st {
		estados[s.Name] = s.State
	}
	if estados["fake"] != "conectado" || estados["quebrado"] != "falhou" {
		t.Fatalf("estados inesperados: %v", estados)
	}
	if !m.Has("fake_soma") || !m.Has("fake_ler_status") {
		t.Fatalf("ferramentas não registradas: %+v", m.Tools())
	}

	out, err := m.Call(context.Background(), "fake_soma", map[string]interface{}{"a": 2.0, "b": 3.0})
	if err != nil || !strings.Contains(out, "2+3=5") {
		t.Fatalf("chamada falhou: %q %v", out, err)
	}

	// Permissão: ferramenta que escreve pede; somente-leitura não; confiável não.
	if ok, _ := m.NeedsApproval("fake_soma"); !ok {
		t.Fatal("fake_soma devia pedir permissão")
	}
	if ok, _ := m.NeedsApproval("fake_ler_status"); ok {
		t.Fatal("somente-leitura não devia pedir permissão")
	}
	_ = m.SetTrusted("fake", true)
	if ok, _ := m.NeedsApproval("fake_soma"); ok {
		t.Fatal("servidor confiável não devia pedir permissão")
	}
}

func TestReloadAplicaConfigNova(t *testing.T) {
	dir := t.TempDir()
	cfg := escreveConfig(t, dir)
	m := NewManager(dir, []string{cfg}, "")
	defer m.Close()
	m.Reload(context.Background())
	if m.Changed() {
		t.Fatal("não devia detectar mudança logo após o reload")
	}

	// Remove o servidor "fake" do arquivo: depois do reload, sumiu.
	time.Sleep(20 * time.Millisecond)
	_ = os.WriteFile(cfg, []byte(`{"mcp":{}}`), 0o644)
	os.Chtimes(cfg, time.Now().Add(time.Second), time.Now().Add(time.Second))
	if !m.Changed() {
		t.Fatal("devia detectar o arquivo alterado")
	}
	m.Reload(context.Background())
	if m.Has("fake_soma") {
		t.Fatal("ferramenta devia ter sumido depois do reload")
	}
}

func TestDesligarReligar(t *testing.T) {
	dir := t.TempDir()
	cfg := escreveConfig(t, dir)
	m := NewManager(dir, []string{cfg}, filepath.Join(dir, "set.json"))
	defer m.Close()
	m.Reload(context.Background())

	_ = m.SetEnabled(context.Background(), "fake", false)
	if m.Has("fake_soma") {
		t.Fatal("desligado não devia expor ferramentas")
	}
	_ = m.SetEnabled(context.Background(), "fake", true)
	if !m.Has("fake_soma") {
		t.Fatal("religado devia expor ferramentas")
	}
}

func TestFormatoClaudeMcpJson(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, ".mcp.json")
	_ = os.WriteFile(p, []byte(`{"mcpServers":{"x":{"command":"node","args":["a.js"],"env":{"K":"v"}}}}`), 0o644)
	c := LoadConfigs([]string{p})
	if len(c) != 1 || strings.Join(c[0].Command, " ") != "node a.js" || c[0].Env["K"] != "v" {
		t.Fatalf("config mal lida: %+v", c)
	}
}
