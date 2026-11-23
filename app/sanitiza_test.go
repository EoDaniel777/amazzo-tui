package app

import (
	"strings"
	"testing"

	"amazzo/mcp"

	xansi "github.com/charmbracelet/x/ansi"
)

func TestLimpaTextoRemoveAnsiTabsECR(t *testing.T) {
	in := "a.go:1:\tx\x1b[31mred\x1b[0m\nprogresso 10%\rprogresso 100%\nfim\x00\x07"
	out := limpaTexto(in)
	if strings.ContainsAny(out, "\x1b\x00\x07\t\r") {
		t.Fatalf("sobrou controle: %q", out)
	}
	if !strings.Contains(out, "progresso 100%") || strings.Contains(out, "10%") {
		t.Fatalf("\\r mal resolvido: %q", out)
	}
	if !strings.Contains(out, "a.go:1: xred") {
		t.Fatalf("tab não expandido até a coluna múltipla de 4: %q", out)
	}
}

func TestCaixaDeComandoNaoVazaDaLargura(t *testing.T) {
	longa := "internal/agent/agent.go:217:\t\tDescription: \"" + strings.Repeat("muito longo ", 40) + "\""
	res := mcp.Result{Tool: "execute_command", Status: "success", Content: longa + "\n" + longa}
	out := toolResultBlock(res, 100)
	for _, l := range strings.Split(out, "\n") {
		if w := xansi.StringWidth(l); w > 100 {
			t.Fatalf("linha com %d colunas passa de 100: %q", w, l)
		}
	}
}

func TestDicaDePathSoParaComando(t *testing.T) {
	res := mcp.Result{Tool: "edit_file", Status: "error", Error: "padrão 'x' não encontrado no arquivo"}
	if strings.Contains(toolResultBlock(res, 100), "PATH") {
		t.Fatal("dica de PATH apareceu num erro de edit_file")
	}
	res = mcp.Result{Tool: "execute_command", Status: "error", Error: "Comando falhou: exit status 127"}
	if !strings.Contains(toolResultBlock(res, 100), "PATH") {
		t.Fatal("dica de PATH sumiu para exit 127")
	}
}
