package app

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"amazzo/agents"
	"amazzo/filesystem"
	"amazzo/mcp"
)

// Roda o loop de ferramentas de verdade contra o Ollama, com os mesmos
// componentes da TUI. É o único teste que prova que o loop FECHA: o modelo
// chama, o resultado volta no formato nativo, e ele continua a tarefa.
//
// Fica atrás de uma variável de ambiente porque depende do Ollama e leva
// minutos nesta CPU:
//
//	AMAZZO_TESTE_OLLAMA=lfm2.5:8b go test ./app/ -run TestLoopFechaComModeloLocal -v -timeout 30m
func TestLoopFechaComModeloLocal(t *testing.T) {
	modelo := os.Getenv("AMAZZO_TESTE_OLLAMA")
	if modelo == "" {
		t.Skip("defina AMAZZO_TESTE_OLLAMA=<modelo> para rodar contra o Ollama")
	}

	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "soma.go"), []byte(
		"package main\n\nimport \"fmt\"\n\nfunc Soma(a, b int) int {\n\treturn a - b\n}\n\nfunc main() {\n\tfmt.Println(Soma(2, 3))\n}\n"), 0o644)
	os.WriteFile(filepath.Join(dir, "go.mod"), []byte("module exemplo\n\ngo 1.21\n"), 0o644)

	fs, err := filesystem.NewManager(dir)
	if err != nil {
		t.Fatal(err)
	}
	srv := mcp.NewServer(fs)

	historico := []agents.Message{
		{Role: "system", Content: localSystemPrompt},
		{Role: "user", Content: "leia o soma.go, a função Soma está errada, corrija ela"},
	}

	usouFerramenta := false

	for ciclo := 1; ciclo <= 8; ciclo++ {
		texto, calls, stats, err := agents.StreamOllama(context.Background(),
			modelo, historico, agents.DefaultTools(), nil)
		if err != nil {
			t.Fatalf("ciclo %d: %v", ciclo, err)
		}

		t.Logf("ciclo %d: prompt=%d tok gen=%d tok %.0fs",
			ciclo, stats.PromptTokens, stats.GenTokens, stats.Total.Seconds())

		// O prompt não pode crescer sem limite: passar do num_ctx faz o Ollama
		// jogar fora o cache de prefixo e reprocessar tudo.
		if limite := agents.DefaultOllamaOptions().NumCtx; stats.PromptTokens > limite {
			t.Errorf("ciclo %d: prompt com %d tokens estourou o num_ctx de %d",
				ciclo, stats.PromptTokens, limite)
		}

		historico = append(historico, agents.Message{
			Role: "assistant", Content: texto, ToolCalls: calls,
		})

		m := &Model{}
		reqs := m.parseToolRequests(texto + textoDasToolCalls(calls))
		if len(reqs) == 0 {
			t.Logf("resposta final: %s", texto)
			break
		}
		usouFerramenta = true

		for _, r := range reqs {
			bruto, _ := json.Marshal(r)
			t.Logf("  → %s", bruto)
		}

		resultados := executaComoOApp(srv, dir, reqs)
		for _, res := range resultados {
			if res.Status == "success" {
				t.Logf("  ← %s ok (%d bytes)", res.Tool, len(res.Content))
			} else {
				t.Logf("  ← %s ERRO: %s", res.Tool, res.Error)
			}
		}

		historico = append(historico, mensagensDeFerramenta(resultados)...)
	}

	if !usouFerramenta {
		t.Fatal("o modelo respondeu sem usar nenhuma ferramenta")
	}

	final, err := fs.ReadFile("soma.go")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(final), "a + b") {
		t.Errorf("o bug não foi corrigido no arquivo:\n%s", final)
	}
}

// O pedido real do usuário: "analise o projeto lendo todos os arquivos, depois
// crie um markdown em docs/ com melhorias". Reproduzido com lfm2.5:8b, ele
// morria no ciclo 3 — o Ollama devolvia uma linha com eval_count 678 e nada
// mais. Este teste verifica que a retentativa resgata o turno em vez de a
// tarefa morrer ali.
//
//	AMAZZO_TESTE_OLLAMA=lfm2.5:8b go test ./app/ -run TestPedidoPesadoNaoMorreNoVazio -v -timeout 40m
func TestPedidoPesadoNaoMorreNoVazio(t *testing.T) {
	modelo := os.Getenv("AMAZZO_TESTE_OLLAMA")
	if modelo == "" {
		t.Skip("defina AMAZZO_TESTE_OLLAMA=<modelo> para rodar contra o Ollama")
	}

	// Cópia enxuta de um projeto Go real, para o pedido ser plausível.
	dir := t.TempDir()
	os.MkdirAll(filepath.Join(dir, "docs"), 0o755)
	os.WriteFile(filepath.Join(dir, "go.mod"), []byte("module exemplo\n\ngo 1.21\n"), 0o644)
	os.WriteFile(filepath.Join(dir, "main.go"), []byte(
		"package main\n\nimport \"fmt\"\n\nfunc main() {\n\tfmt.Println(Soma(2, 3))\n}\n"), 0o644)
	os.WriteFile(filepath.Join(dir, "soma.go"), []byte(
		"package main\n\n// Soma devolve a soma de dois inteiros.\nfunc Soma(a, b int) int {\n\treturn a - b\n}\n"), 0o644)

	fs, err := filesystem.NewManager(dir)
	if err != nil {
		t.Fatal(err)
	}
	srv := mcp.NewServer(fs)

	historico := []agents.Message{
		{Role: "system", Content: localSystemPrompt},
		{Role: "user", Content: "faça uma analise do projeto lendo todos os arquivos, " +
			"depois dentro da pasta docs crie um arquivo markdown contendo melhorias " +
			"para o projeto e erros a corrigir"},
	}

	var ciclosVazios, resgates int
	tentouDeNovo := false
	// Um Model persistente ao longo do laço: e nele que vive o estado de
	// deteccao de chamada repetida.
	app := novoModelDeTeste(t)

	for ciclo := 1; ciclo <= 12; ciclo++ {
		texto, calls, stats, err := agents.StreamOllama(context.Background(),
			modelo, historico, agents.DefaultTools(), nil)
		if err != nil {
			t.Fatalf("ciclo %d: %v", ciclo, err)
		}

		vazio := strings.TrimSpace(texto) == "" && len(calls) == 0
		t.Logf("ciclo %2d: prompt=%d gen=%d %.0fs | content=%d | calls=%d%s",
			ciclo, stats.PromptTokens, stats.GenTokens, stats.Total.Seconds(),
			len(strings.TrimSpace(texto)), len(calls), map[bool]string{true: "  <<< VAZIO"}[vazio])

		if vazio {
			ciclosVazios++
			// Mesma regra do app: gastou tokens e nao entregou nada -> pede de
			// novo, uma vez.
			if stats.GenTokens > 0 && !tentouDeNovo {
				tentouDeNovo = true
				historico = append(historico, agents.Message{Role: "user",
					Content: "Continue a tarefa do ponto onde parou. " +
						"Emita a próxima chamada de ferramenta, ou escreva a resposta final."})
				continue
			}
			t.Logf("  >>> desistiu depois de %d ciclo(s) vazio(s)", ciclosVazios)
			break
		}
		if tentouDeNovo {
			resgates++
			tentouDeNovo = false
		}

		historico = append(historico, agents.Message{Role: "assistant", Content: texto, ToolCalls: calls})

		reqs := app.parseToolRequests(texto + textoDasToolCalls(calls))
		if len(reqs) == 0 {
			t.Logf("  resposta final (%d chars)", len(strings.TrimSpace(texto)))
			break
		}
		for _, r := range reqs {
			t.Logf("  → %s %s", r.Tool, r.Path)
		}
		resultados := executaComoOApp(srv, dir, reqs)
		for _, r := range resultados {
			if r.Status == "success" {
				t.Logf("  ← %s ok (%d bytes)", r.Tool, len(r.Content))
			} else {
				t.Logf("  ← %s ERRO %s", r.Tool, r.Error)
			}
		}
		// Mesmo caminho do app: marca repetições antes de mandar ao modelo.
		resultados = app.marcaRepeticoes(resultados)
		historico = append(historico, mensagensDeFerramenta(resultados)...)
	}

	entradas, _ := os.ReadDir(filepath.Join(dir, "docs"))
	t.Logf("docs/ tem %d arquivo(s)", len(entradas))
	for _, e := range entradas {
		b, _ := os.ReadFile(filepath.Join(dir, "docs", e.Name()))
		t.Logf("CONTEUDO de %s (%d bytes):\n%s", e.Name(), len(b), b)
	}
	if b, err := os.ReadFile(filepath.Join(dir, "soma.go")); err == nil {
		t.Logf("SOMA.GO final:\n%s", b)
	}

	if ciclosVazios > 0 && resgates == 0 {
		t.Errorf("houve %d ciclo(s) vazio(s) e nenhum resgate: a retentativa não funcionou",
			ciclosVazios)
	}
	if len(entradas) == 0 {
		t.Errorf("nada foi escrito em docs/ — a tarefa não chegou ao fim")
	}
}

// executaComoOApp roda o lote do mesmo jeito que a TUI: set_workspace é
// resolvida pelo app (muda o alcance do agente, não um arquivo) e nunca chega
// ao MCP. Sem isso o arnês devolvia "ferramenta não suportada" e o teste
// mentia sobre o comportamento real.
func executaComoOApp(srv *mcp.Server, raiz string, reqs []mcp.Request) []mcp.Result {
	var paraMCP []mcp.Request
	var resultados []mcp.Result

	for _, r := range reqs {
		if strings.EqualFold(r.Tool, "set_workspace") {
			resultados = append(resultados, mcp.Result{
				Tool: r.Tool, Path: r.Path, Status: "success",
				Content: "Pasta de trabalho agora é " + raiz +
					". Os caminhos passam a ser relativos a ela. " +
					"Agora continue a tarefa que o usuário pediu, do ponto onde parou.",
			})
			continue
		}
		paraMCP = append(paraMCP, r)
	}

	out, _ := srv.Handle(context.Background(), paraMCP)
	return append(resultados, out...)
}
