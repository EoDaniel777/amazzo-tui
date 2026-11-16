package app

import (
	"amazzo/mcpclient"
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"amazzo/mcp"

	tea "github.com/charmbracelet/bubbletea"
)

// esperaToolsDone executa os comandos (em paralelo, como o Bubble Tea faz) e
// devolve a mensagem toolsDoneMsg.
func esperaToolsDone(t *testing.T, cmd tea.Cmd) toolsDoneMsg {
	t.Helper()
	ch := make(chan tea.Msg, 16)
	var run func(c tea.Cmd)
	run = func(c tea.Cmd) {
		if c == nil {
			return
		}
		go func() {
			msg := c()
			if b, ok := msg.(tea.BatchMsg); ok {
				for _, sub := range b {
					run(sub)
				}
				return
			}
			ch <- msg
		}()
	}
	run(cmd)
	deadline := time.After(15 * time.Second)
	for {
		select {
		case msg := <-ch:
			if d, ok := msg.(toolsDoneMsg); ok {
				return d
			}
		case <-deadline:
			t.Fatal("toolsDoneMsg não chegou")
		}
	}
}

// O comando demora, mas a chamada que o dispara volta na hora: é isso que
// mantém a TUI viva enquanto "go build" roda.
func TestFerramentasRodamEmSegundoPlano(t *testing.T) {
	m := modeloParaPalette(t)

	inicio := time.Now()
	cmd := m.startTools([]mcp.Request{{Tool: "execute_command", Command: "sleep 1; echo pronto"}}, nil)
	if d := time.Since(inicio); d > 300*time.Millisecond {
		t.Fatalf("startTools bloqueou por %v", d)
	}
	if !m.toolsRunning || !m.loading {
		t.Fatal("devia marcar que as ferramentas estão rodando")
	}
	if !strings.Contains(m.streamingPreview(), "executando") {
		t.Errorf("a tela devia mostrar a execução:\n%s", m.streamingPreview())
	}

	msg := esperaToolsDone(t, cmd)
	m.finishTools(msg)
	if m.toolsRunning {
		t.Fatal("devia ter terminado")
	}
	if len(msg.results) != 1 || !strings.Contains(msg.results[0].Content, "pronto") {
		t.Fatalf("resultado inesperado: %+v", msg.results)
	}
}

func TestEscInterrompeFerramentas(t *testing.T) {
	m := modeloParaPalette(t)
	m.startTools([]mcp.Request{{Tool: "execute_command", Command: "sleep 30"}}, nil)
	if !m.interrompeFerramentas() {
		t.Fatal("devia interromper")
	}
	if m.loading {
		t.Fatal("o turno devia ter terminado")
	}
}

// fakeExt simula um servidor MCP externo.
type fakeExt struct{ chamadas []map[string]interface{} }

func (f *fakeExt) Has(n string) bool { return n == "srv_listar_itens" }
func (f *fakeExt) Call(_ context.Context, _ string, a map[string]interface{}) (string, error) {
	f.chamadas = append(f.chamadas, a)
	return "3 itens", nil
}

func TestFerramentaExternaChegaComArgumentos(t *testing.T) {
	f := &fakeExt{}
	mcp.SetExternal(f)
	defer mcp.SetExternal(nil)

	m := modeloParaPalette(t)
	reqs := m.parseToolRequests(`{"name":"srv_listar_itens","arguments":{"limite":5}}`)
	if len(reqs) != 1 || reqs[0].Args["limite"] != float64(5) {
		t.Fatalf("pedido externo mal interpretado: %+v", reqs)
	}
	msg := esperaToolsDone(t, m.startTools(reqs, nil))
	if len(msg.results) != 1 || msg.results[0].Content != "3 itens" || msg.results[0].Status != "success" {
		t.Fatalf("resultado: %+v", msg.results)
	}
}

func TestPalavrasDeEsperaGiram(t *testing.T) {
	ini := time.Now()
	vistas := map[string]bool{}
	for i := 0; i < 12; i++ {
		vistas[palavraDeEspera(ini, ini.Add(time.Duration(i)*3*time.Second))] = true
	}
	if len(vistas) < 8 {
		t.Fatalf("poucas palavras diferentes: %v", vistas)
	}
}

// Servidores grandes expõem 200+ ferramentas: o prompt não pode carregar todas.
func TestSchemasExternosRespeitamOrcamento(t *testing.T) {
	m := modeloParaPalette(t)
	dir := t.TempDir()
	var tools []string
	for i := 0; i < 207; i++ {
		tools = append(tools, fmt.Sprintf(`{"name":"ferr%d","description":"faz a coisa numero %d com um texto de descricao razoavelmente longo para ocupar espaco","inputSchema":{"type":"object","properties":{"alvo":{"type":"string","description":"o alvo"},"modo":{"type":"string"}}}}`, i, i))
	}
	script := filepath.Join(dir, "srv.sh")
	body := `#!/bin/bash
while IFS= read -r l; do
 id=$(echo "$l" | sed -n 's/.*"id":\([0-9]*\).*/\1/p')
 case "$l" in
  *'"initialize"'*) echo '{"jsonrpc":"2.0","id":'$id',"result":{"protocolVersion":"2025-03-26","capabilities":{}}}';;
  *'"tools/list"'*) echo '{"jsonrpc":"2.0","id":'$id',"result":{"tools":[` + strings.Join(tools, ",") + `]}}';;
 esac
done
`
	if err := os.WriteFile(script, []byte(body), 0o755); err != nil {
		t.Fatal(err)
	}
	cfg := filepath.Join(dir, "mcp.json")
	_ = os.WriteFile(cfg, []byte(fmt.Sprintf(`{"mcp":{"grande":{"type":"local","command":["%s"]}}}`, script)), 0o644)
	m.mcpMgr = mcpclient.NewManager(dir, []string{cfg}, "")
	defer m.mcpMgr.Close()
	m.mcpMgr.Reload(context.Background())
	if n := len(m.mcpMgr.Tools()); n != 207 {
		t.Fatalf("servidor falso devia ter 207 ferramentas, tem %d", n)
	}

	schemas := m.toolSchemas()
	var nomes []string
	for _, s := range schemas {
		nomes = append(nomes, s.Function.Name)
	}
	juntos := strings.Join(nomes, " ")
	if strings.Contains(juntos, "grande_ferr0") {
		t.Error("as 207 ferramentas não deviam ir todas para o prompt")
	}
	if !strings.Contains(juntos, "mcp_search") || !strings.Contains(juntos, "mcp_call") {
		t.Errorf("faltam mcp_search/mcp_call: %s", juntos)
	}
	if custo := estimateTokens(m.textoDosSchemasExternos()); custo > orcamentoSchemasExternos+500 {
		t.Errorf("schemas externos custam %d tokens", custo)
	}

	// Busca acha a ferramenta; mcp_call chama e as chaves "tool"/"name" dos
	// argumentos sobrevivem ao parse.
	if r := buscaFerramentasMCP(m.mcpMgr, "numero 42"); !strings.Contains(r, "grande_ferr42") {
		t.Errorf("busca não achou: %s", r)
	}
	mcp.SetExternal(mcpAdapter{m.mcpMgr})
	defer mcp.SetExternal(nil)
	reqs := m.parseToolRequests(`{"name":"mcp_call","arguments":{"tool":"grande_ferr7","arguments":{"name":"x"}}}`)
	if len(reqs) != 1 || reqs[0].Args["tool"] != "grande_ferr7" {
		t.Fatalf("mcp_call mal interpretado: %+v", reqs)
	}
	inner, _ := reqs[0].Args["arguments"].(map[string]interface{})
	if inner["name"] != "x" {
		t.Errorf("argumento 'name' foi perdido: %+v", reqs[0].Args)
	}
}
