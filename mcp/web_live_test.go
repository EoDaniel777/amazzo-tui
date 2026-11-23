package mcp

import (
	"os"
	"testing"
)

func TestFetchAoVivo(t *testing.T) {
	if os.Getenv("AMAZZO_LIVE") == "" {
		t.Skip("defina AMAZZO_LIVE=1")
	}
	for _, u := range []string{"https://pkg.go.dev/github.com/charmbracelet/bubbletea", "https://github.com/charmbracelet/bubbletea/blob/main/README.md", "https://example.com"} {
		r := paginaWeb(Request{Tool: "web_fetch", URL: urlDeConteudo(u)})
		c := r.Content
		if len(c) > 900 {
			c = c[:900]
		}
		t.Logf("== %s [%s]\n%s", u, r.Status+r.Error, c)
	}
}
