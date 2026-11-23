package mcp

import (
	"os"
	"testing"
)

func TestPesquisaWebAoVivo(t *testing.T) {
	if os.Getenv("AMAZZO_LIVE") == "" {
		t.Skip("defina AMAZZO_LIVE=1 para testar com a internet")
	}
	r := pesquisaWeb(Request{Tool: "web_search", Query: "golang bubbletea tutorial"})
	t.Log(r.Content)
	if r.Status != "success" || len(r.Content) < 100 {
		t.Fatalf("%+v", r)
	}
}
