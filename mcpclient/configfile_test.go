package mcpclient

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestAddRemoveServer(t *testing.T) {
	p := filepath.Join(t.TempDir(), "sub", "mcp.json")
	cfg, _, err := ParseAddArgs([]string{"context7"})
	if err != nil {
		t.Fatal(err)
	}
	if err := AddServer(p, cfg); err != nil {
		t.Fatal(err)
	}
	c := LoadConfigs([]string{p})
	if len(c) != 1 || c[0].Name != "context7" || strings.Join(c[0].Command, " ") != "npx -y @upstash/context7-mcp" {
		t.Fatalf("config mal gravada: %+v", c)
	}
	if ok, err := RemoveServer(p, "context7"); !ok || err != nil {
		t.Fatalf("remover falhou: %v %v", ok, err)
	}
	if len(LoadConfigs([]string{p})) != 0 {
		t.Fatal("servidor devia ter sumido")
	}
}

func TestParseAddArgs(t *testing.T) {
	c, proj, err := ParseAddArgs([]string{"x", "--projeto", "--env", "K=v", "--", "node", "a.js"})
	if err != nil || !proj || c.Env["K"] != "v" || strings.Join(c.Command, " ") != "node a.js" {
		t.Fatalf("%+v %v %v", c, proj, err)
	}
	c, _, err = ParseAddArgs([]string{"r", "https://e.com/mcp", "--header", "A=b"})
	if err != nil || !c.Remote || c.Headers["A"] != "b" {
		t.Fatalf("%+v %v", c, err)
	}
	if _, _, err = ParseAddArgs([]string{"desconhecido"}); err == nil {
		t.Fatal("nome desconhecido sem comando devia falhar")
	}
}

func TestConfigPathsIgnoraOpencode(t *testing.T) {
	for _, p := range ConfigPaths("/proj") {
		if strings.Contains(p, "opencode") || strings.HasSuffix(p, ".mcp.json") {
			t.Fatalf("não devia ler %s", p)
		}
	}
	_ = os.Stdout
}
