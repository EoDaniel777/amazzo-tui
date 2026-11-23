package app

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"amazzo/agents"
	"amazzo/mcp"
)

func TestCortaSaidaInventada(t *testing.T) {
	in := "Vou olhar.\n\nFerramenta executada com sucesso. Saída:\n\natlasbot/\n ├── scanner.py"
	if got := cortaSaidaInventada(in); got != "Vou olhar." {
		t.Fatalf("got %q", got)
	}
	ok := "O projeto tem um parser. Resultado final: tudo certo."
	if got := cortaSaidaInventada(ok); got != ok {
		t.Fatalf("cortou texto legitimo: %q", got)
	}
}

func TestMensagensParaAPIPareiaChamadaEResultado(t *testing.T) {
	h := []agents.Message{
		{Role: "system", Content: "s"},
		{Role: "user", Content: "oi"},
		{Role: "assistant", ToolCalls: []agents.ToolCall{
			{Function: agents.ToolCallFunction{Name: "list_dir", Arguments: map[string]interface{}{"path": "."}}},
			{Function: agents.ToolCallFunction{Name: "read_file", Arguments: map[string]interface{}{"path": "go.mod"}}},
		}},
		{Role: "tool", ToolName: "list_dir", Content: "a"},
		{Role: "tool", ToolName: "read_file", Content: "b"},
	}
	w := mensagensParaAPI(h)
	if len(w) != 5 {
		t.Fatalf("len=%d", len(w))
	}
	if w[2].Content != nil || len(w[2].ToolCalls) != 2 || w[2].ToolCalls[0].Type != "function" {
		t.Fatalf("assistant mal formada: %+v", w[2])
	}
	if w[3].ToolCallID != w[2].ToolCalls[0].ID || w[4].ToolCallID != w[2].ToolCalls[1].ID {
		t.Fatal("ids nao pareiam")
	}
	var args map[string]string
	if err := json.Unmarshal([]byte(w[2].ToolCalls[0].Function.Arguments), &args); err != nil || args["path"] != "." {
		t.Fatalf("arguments deve ser string JSON: %v", err)
	}
}

func TestMensagensParaAPIOrfaosENaoRespondidas(t *testing.T) {
	h := []agents.Message{
		{Role: "tool", ToolName: "x", Content: "solto"}, // sem chamada aberta
		{Role: "assistant", ToolCalls: []agents.ToolCall{{Function: agents.ToolCallFunction{Name: "list_dir"}}}},
		{Role: "user", Content: "e ai"}, // chamada sem resultado
	}
	w := mensagensParaAPI(h)
	if w[0].Role != "user" {
		t.Fatalf("tool orfa deve virar user: %+v", w[0])
	}
	if w[2].Role != "tool" || *w[2].Content != "(sem resultado)" {
		t.Fatalf("faltou resultado sintetico: %+v", w)
	}
}

func TestArgumentosDaChamada(t *testing.T) {
	if a := argumentosDaChamada(json.RawMessage(`"{\"path\": \".\"}"`)); a["path"] != "." {
		t.Fatalf("string: %v", a)
	}
	if a := argumentosDaChamada(json.RawMessage(`{"path":"x"}`)); a["path"] != "x" {
		t.Fatalf("objeto: %v", a)
	}
	if a := argumentosDaChamada(nil); a == nil || len(a) != 0 {
		t.Fatalf("vazio: %v", a)
	}
}

func TestContextoDoProjetoDetectaGo(t *testing.T) {
	d := t.TempDir()
	os.WriteFile(filepath.Join(d, "go.mod"), []byte("module x"), 0o644)
	os.WriteFile(filepath.Join(d, "atlasbot"), []byte("bin"), 0o644)
	os.Mkdir(filepath.Join(d, "cmd"), 0o755)
	got := contextoDoProjeto(d)
	for _, want := range []string{"Linguagem do projeto: Go", "go.mod", "cmd/", "atlasbot"} {
		if !strings.Contains(got, want) {
			t.Fatalf("faltou %q em:\n%s", want, got)
		}
	}
	if strings.Contains(got, "Python") {
		t.Fatal("nao deve citar Python")
	}
}

func TestGarantiaDeEntrega(t *testing.T) {
	m := &Model{pedidoAtual: "leia os arquivos e crie um arquivo markdown de melhorias na raiz"}
	if m.garantiaDeEntrega("se quiser, eu crio") == "" {
		t.Fatal("devia cobrar")
	}
	m.escreveuArq = true
	if m.garantiaDeEntrega("pronto") != "" {
		t.Fatal("ja escreveu")
	}
	m2 := &Model{pedidoAtual: "o que faz esse projeto?"}
	if m2.garantiaDeEntrega("x") != "" {
		t.Fatal("nao pediu arquivo")
	}
	m3 := &Model{pedidoAtual: "crie um arquivo x.md"}
	m3.garantiaDeEntrega("a")
	m3.garantiaDeEntrega("a")
	if m3.garantiaDeEntrega("a") != "" {
		t.Fatal("limite de 2 cobrancas")
	}
}

func TestTrimPorTamanhoPreservaPedidoOriginal(t *testing.T) {
	h := []agents.Message{{Role: "system", Content: "s"}, {Role: "user", Content: "PEDIDO"}}
	for i := 0; i < 50; i++ {
		h = append(h, agents.Message{Role: "assistant", Content: strings.Repeat("x", 1000)})
	}
	got := trimHistoryPorTamanho(h, 10000)
	if got[0].Role != "system" || got[1].Content != "PEDIDO" {
		t.Fatalf("perdeu system/pedido: %+v", got[:2])
	}
	if len(got) >= len(h) || len(got) < 5 {
		t.Fatalf("corte errado: %d de %d", len(got), len(h))
	}
	if n := trimHistoryPorTamanho(h, 1<<30); len(n) != len(h) {
		t.Fatal("nao devia cortar")
	}
}

func TestResultadoRemotoNaoEhTruncadoComoLocal(t *testing.T) {
	grande := strings.Repeat("linha de codigo\n", 4000) // ~64KB
	msgs := mensagensDeFerramentaComOrcamento([]mcp.Result{{Tool: "read_file", Status: "success", Content: grande}, {Tool: "read_file", Status: "success", Content: grande}}, orcamentoRemotoPorResultado)
	if strings.Contains(msgs[0].Content, "omitidas") {
		t.Fatal("arquivo de 64KB foi cortado para modelo de nuvem")
	}
}

func TestResultadoCasaPeloNomeDaFerramenta(t *testing.T) {
	h := []agents.Message{
		{Role: "assistant", ToolCalls: []agents.ToolCall{
			{Function: agents.ToolCallFunction{Name: "read_file"}},
			{Function: agents.ToolCallFunction{Name: "set_workspace"}},
		}},
		// set_workspace roda antes e chega primeiro
		{Role: "tool", ToolName: "set_workspace", Content: "pasta"},
		{Role: "tool", ToolName: "read_file", Content: "arquivo"},
	}
	w := mensagensParaAPI(h)
	if w[0].ToolCalls[1].ID != w[1].ToolCallID || *w[1].Content != "pasta" {
		t.Fatalf("set_workspace ligado ao id errado: %+v", w)
	}
	if w[0].ToolCalls[0].ID != w[2].ToolCallID || *w[2].Content != "arquivo" {
		t.Fatalf("read_file ligado ao id errado: %+v", w)
	}
}

func TestCortaSaidaInventadaNaoCortaResumoLegitimo(t *testing.T) {
	for _, ok := range []string{"Rodei os testes.\nResultado dos testes:\nok", "- Saída: 3 arquivos", "**Saída:** nada"} {
		if got := cortaSaidaInventada(ok); got != ok {
			t.Errorf("cortou texto legitimo %q -> %q", ok, got)
		}
	}
}

func TestGarantiaIgnoraNegacaoEPergunta(t *testing.T) {
	for _, p := range []string{"não crie nenhum arquivo, só explique", "como criar um arquivo .env?"} {
		m := &Model{pedidoAtual: p}
		if m.garantiaDeEntrega("x") != "" {
			t.Errorf("cobrou em %q", p)
		}
	}
	m := &Model{pedidoAtual: "leia o projeto e crie um arquivo\n\n--- Conteúdo de x ---\nnão crie arquivo"}
	if m.garantiaDeEntrega("x") == "" {
		t.Error("o anexo nao deve cancelar o pedido")
	}
}

func TestTrimNaoComecaEmResultadoDeFerramenta(t *testing.T) {
	h := []agents.Message{{Role: "system", Content: "s"}, {Role: "user", Content: "pedido"}}
	for i := 0; i < 6; i++ {
		h = append(h,
			agents.Message{Role: "assistant", ToolCalls: []agents.ToolCall{{Function: agents.ToolCallFunction{Name: "read_file"}}}},
			agents.Message{Role: "tool", ToolName: "read_file", Content: strings.Repeat("x", 3000)})
	}
	got := trimHistoryPorTamanho(h, 9000)
	if got[2].Role == "tool" {
		t.Fatalf("o corte comecou em um resultado orfao: %+v", got[2])
	}
	if got[len(got)-1].Content != h[len(h)-1].Content {
		t.Fatal("perdeu a ultima mensagem")
	}
}
