package mcp

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"amazzo/filesystem"
)

func servidorDeTeste(t *testing.T) (*Server, *filesystem.Manager) {
	t.Helper()
	dir := t.TempDir()
	fs, err := filesystem.NewManager(dir)
	if err != nil {
		t.Fatal(err)
	}
	return NewServer(fs), fs
}

// Um lote de ferramentas é uma sequência, não um conjunto: o modelo emite
// write_file e depois edit_file no mesmo arquivo. Executando em paralelo, o
// mesmo lote dava cinco resultados diferentes em cinco tentativas — é essa
// não-determinismo que faz "editar arquivos" parecer quebrado.
func TestLoteExecutaEmOrdem(t *testing.T) {
	for i := 0; i < 20; i++ {
		s, fs := servidorDeTeste(t)

		res, err := s.Handle(context.Background(), []Request{
			{Tool: "make_dir", Path: "docs"},
			{Tool: "write_file", Path: "docs/notas.md", Content: "alfa\n"},
			{Tool: "edit_file", Path: "docs/notas.md", Search: "alfa", Replace: "beta"},
			{Tool: "read_file", Path: "docs/notas.md"},
		})
		if err != nil {
			t.Fatalf("tentativa %d: %v", i, err)
		}

		for j, r := range res {
			if r.Status != "success" {
				t.Fatalf("tentativa %d: passo %d (%s) falhou: %s", i, j, r.Tool, r.Error)
			}
		}
		if got := res[3].Content; got != "beta\n" {
			t.Fatalf("tentativa %d: leitura devolveu %q, esperava %q", i, got, "beta\n")
		}
		if b, _ := fs.ReadFile("docs/notas.md"); string(b) != "beta\n" {
			t.Fatalf("tentativa %d: arquivo em disco é %q", i, string(b))
		}
	}
}

// Criar um arquivo vazio é uma operação legítima (um __init__.py, um .gitkeep,
// um esqueleto). Recusar por "conteúdo vazio" travava o agente sem motivo.
func TestArquivoVazioPodeSerCriado(t *testing.T) {
	s, fs := servidorDeTeste(t)

	res := s.dispatch(Request{Tool: "write_file", Path: "vazio.txt", Content: ""})
	if res.Status != "success" {
		t.Fatalf("criar arquivo vazio falhou: %s", res.Error)
	}
	if _, _, err := fs.FileInfo("vazio.txt"); err != nil {
		t.Errorf("o arquivo não foi criado: %v", err)
	}
}

// Quando o `search` não casa, o modelo precisa de pista para se corrigir.
// "padrão não encontrado" sozinho fazia ele repetir a mesma chamada errada.
func TestEditFileAjudaQuandoOTrechoNaoCasa(t *testing.T) {
	s, _ := servidorDeTeste(t)
	s.dispatch(Request{Tool: "write_file", Path: "main.go",
		Content: "package main\n\nfunc main() {\n\tprintln(\"oi\")\n}\n"})

	res := s.dispatch(Request{Tool: "edit_file", Path: "main.go",
		Search: "println( \"oi\" )", Replace: "println(\"olá\")"})

	if res.Status == "success" {
		t.Fatal("um trecho que não existe não pode ser trocado")
	}
	if !strings.Contains(res.Error, "println") {
		t.Errorf("o erro não mostra a linha parecida, o modelo não consegue se corrigir: %q", res.Error)
	}
}

// search_files não pode ler binários nem entrar em pastas de build: nesta
// pasta há um binário de 23 MB, e o resultado vira lixo ilegível que ainda
// gasta contexto do modelo.
func TestBuscaIgnoraBinariosEPastasDeBuild(t *testing.T) {
	s, fs := servidorDeTeste(t)
	raiz := fs.Root()

	os.WriteFile(filepath.Join(raiz, "codigo.go"), []byte("func Alvo() {}\n"), 0o644)
	// Binário: NUL no meio, com o padrão presente.
	os.WriteFile(filepath.Join(raiz, "programa.bin"),
		[]byte("\x00\x01func Alvo() {}\x00"), 0o644)
	os.MkdirAll(filepath.Join(raiz, "build"), 0o755)
	os.WriteFile(filepath.Join(raiz, "build", "copia.go"), []byte("func Alvo() {}\n"), 0o644)
	os.MkdirAll(filepath.Join(raiz, ".git"), 0o755)
	os.WriteFile(filepath.Join(raiz, ".git", "COMMIT_EDITMSG"), []byte("func Alvo() {}\n"), 0o644)

	res := s.dispatch(Request{Tool: "search_files", Pattern: "func Alvo", Path: "."})
	if res.Status != "success" {
		t.Fatalf("busca falhou: %s", res.Error)
	}
	for _, proibido := range []string{"programa.bin", "build/", ".git"} {
		if strings.Contains(res.Content, proibido) {
			t.Errorf("a busca entrou em %s:\n%s", proibido, res.Content)
		}
	}
	if !strings.Contains(res.Content, "codigo.go") {
		t.Errorf("a busca perdeu o resultado legítimo:\n%s", res.Content)
	}
}

// O nome da ferramenta é o que separa uma chamada de um JSON qualquer no meio
// da resposta. O app precisa poder perguntar isso ao MCP.
func TestFerramentaSuportada(t *testing.T) {
	suportadas := []string{"read_file", "write_file", "edit_file", "list_dir",
		"make_dir", "search_files", "execute_command", "web_fetch", "set_workspace",
		"LIST_DIR", "mkdir", "grep"}
	for _, nome := range suportadas {
		if !FerramentaSuportada(nome) {
			t.Errorf("%q deveria ser suportada", nome)
		}
	}

	naoSuportadas := []string{"", "meu-projeto", "exemplo", "ferramenta_que_nao_existe"}
	for _, nome := range naoSuportadas {
		if FerramentaSuportada(nome) {
			t.Errorf("%q não deveria ser suportada", nome)
		}
	}
}

// Modelos pequenos usam search_files das duas maneiras. Observado com o
// lfm2.5:8b: ele pediu o padrão "*.go" esperando a lista de fontes, recebeu
// "nenhum resultado" da busca por conteúdo e seguiu sem nunca ver o código.
func TestBuscaComCuringaProcuraNomeDeArquivo(t *testing.T) {
	s, fs := servidorDeTeste(t)
	raiz := fs.Root()
	os.WriteFile(filepath.Join(raiz, "soma.go"), []byte("package main\n"), 0o644)
	os.WriteFile(filepath.Join(raiz, "README.md"), []byte("texto\n"), 0o644)
	os.MkdirAll(filepath.Join(raiz, "interno"), 0o755)
	os.WriteFile(filepath.Join(raiz, "interno", "util.go"), []byte("package interno\n"), 0o644)
	os.MkdirAll(filepath.Join(raiz, "build"), 0o755)
	os.WriteFile(filepath.Join(raiz, "build", "copia.go"), []byte("package main\n"), 0o644)

	res := s.dispatch(Request{Tool: "search_files", Pattern: "*.go", Path: "."})
	if res.Status != "success" {
		t.Fatalf("busca por nome falhou: %s", res.Error)
	}
	for _, esperado := range []string{"soma.go", "util.go"} {
		if !strings.Contains(res.Content, esperado) {
			t.Errorf("%s não apareceu:\n%s", esperado, res.Content)
		}
	}
	if strings.Contains(res.Content, "README.md") {
		t.Errorf("o glob casou um arquivo que não é .go:\n%s", res.Content)
	}
	if strings.Contains(res.Content, "build/") {
		t.Errorf("a busca por nome entrou em build/:\n%s", res.Content)
	}
}

// Sem curinga, o comportamento é o de sempre: procurar texto no conteúdo.
func TestBuscaSemCuringaProcuraConteudo(t *testing.T) {
	s, fs := servidorDeTeste(t)
	os.WriteFile(filepath.Join(fs.Root(), "soma.go"), []byte("func Soma(a, b int) int\n"), 0o644)

	res := s.dispatch(Request{Tool: "search_files", Pattern: "func Soma", Path: "."})
	if !strings.Contains(res.Content, "soma.go:1:") {
		t.Errorf("a busca por conteúdo parou de funcionar:\n%s", res.Content)
	}
}

// Modelo pequeno erra o escape do JSON com frequência: manda "\n" literal
// (barra invertida + n) onde queria uma quebra de linha. Medido com o
// lfm2.5:8b tentando corrigir uma função de três linhas — a busca nunca casava
// e ele desistia dizendo que tinha corrigido.
func TestEditFileAceitaEscapeLiteralDoModelo(t *testing.T) {
	s, fs := servidorDeTeste(t)
	original := "package main\n\nfunc Soma(a, b int) int {\n\treturn a - b\n}\n"
	s.dispatch(Request{Tool: "write_file", Path: "soma.go", Content: original})

	// Exatamente o que o lfm2.5 emitiu: \n e \t como dois caracteres.
	res := s.dispatch(Request{Tool: "edit_file", Path: "soma.go",
		Search:  `func Soma(a, b int) int {\n\treturn a - b\n}`,
		Replace: `func Soma(a, b int) int {\n\treturn a + b\n}`,
	})

	if res.Status != "success" {
		t.Fatalf("a edição falhou: %s", res.Error)
	}
	b, _ := fs.ReadFile("soma.go")
	if !strings.Contains(string(b), "return a + b") {
		t.Errorf("o arquivo não foi corrigido:\n%s", b)
	}
	if strings.Contains(string(b), `\n`) {
		t.Errorf("escape literal foi gravado no arquivo:\n%s", b)
	}
}

// O texto exato tem prioridade: um arquivo Go pode conter "\n" de verdade,
// dentro de um literal de string, e trocá-lo por uma quebra real quebraria o
// código.
func TestEditFileNaoDesescapaQuandoOExatoCasa(t *testing.T) {
	s, fs := servidorDeTeste(t)
	original := "package main\n\nfunc main() {\n\tprint(\"oi\\n\")\n}\n"
	s.dispatch(Request{Tool: "write_file", Path: "main.go", Content: original})

	res := s.dispatch(Request{Tool: "edit_file", Path: "main.go",
		Search:  `print("oi\n")`,
		Replace: `print("olá\n")`,
	})

	if res.Status != "success" {
		t.Fatalf("a edição do literal falhou: %s", res.Error)
	}
	b, _ := fs.ReadFile("main.go")
	if !strings.Contains(string(b), `print("olá\n")`) {
		t.Errorf("o \\n dentro do literal de string foi destruído:\n%q", string(b))
	}
	if strings.Count(string(b), "\n") != strings.Count(original, "\n") {
		t.Errorf("o número de linhas mudou:\n%q", string(b))
	}
}

// Um arquivo inteiro numa linha, com "\n" literal no lugar das quebras, é o
// que o modelo produz quando erra o escape. Gravar assim dá um arquivo que nem
// compila, e ele acha que deu certo.
func TestWriteFileCorrigeArquivoTodoEscapado(t *testing.T) {
	s, fs := servidorDeTeste(t)

	res := s.dispatch(Request{Tool: "write_file", Path: "novo.go",
		Content: `package main\n\nimport "fmt"\n\nfunc main() {\n\tfmt.Println("oi")\n}`,
	})
	if res.Status != "success" {
		t.Fatalf("falhou: %s", res.Error)
	}

	b, _ := fs.ReadFile("novo.go")
	if strings.Count(string(b), "\n") < 5 {
		t.Errorf("o arquivo continuou numa única linha:\n%q", string(b))
	}
	if strings.Contains(string(b), `\n`) {
		t.Errorf("sobrou escape literal:\n%q", string(b))
	}
	if !strings.Contains(res.Content, "interpretei os escapes") {
		t.Errorf("o modelo não foi avisado da correção: %q", res.Content)
	}
}

// Conteúdo normal não é tocado: um Go válido com "\n" dentro de string tem
// quebras reais, então a heurística não dispara.
func TestWriteFileNaoMexeEmConteudoNormal(t *testing.T) {
	s, fs := servidorDeTeste(t)
	original := "package main\n\nfunc main() {\n\tprint(\"linha\\n\")\n\tprint(\"outra\\n\")\n}\n"

	s.dispatch(Request{Tool: "write_file", Path: "ok.go", Content: original})

	b, _ := fs.ReadFile("ok.go")
	if string(b) != original {
		t.Errorf("conteúdo normal foi alterado:\n%q\nesperava:\n%q", string(b), original)
	}
}

// read_file num diretório devolvia o erro cru do OS ("is a directory"), que não
// diz o que fazer. Medido com lfm2.5:8b: ele pediu `read_file docs`, levou o
// erro e gastou o ciclo seguinte tentando outra variação do mesmo caminho.
func TestReadFileEmDiretorioSugereListDir(t *testing.T) {
	s, fs := servidorDeTeste(t)
	os.MkdirAll(filepath.Join(fs.Root(), "docs"), 0o755)

	res := s.dispatch(Request{Tool: "read_file", Path: "docs"})

	if res.Status != "error" {
		t.Fatal("ler um diretório não pode dar sucesso")
	}
	if !strings.Contains(res.Error, "list_dir") {
		t.Errorf("o erro não indica a ferramenta certa: %q", res.Error)
	}
}

// read_file com curinga: o modelo pede "docs/*.md" esperando ler tudo de uma
// vez. "no such file or directory" não sugere o caminho correto.
func TestReadFileComCuringaSugereSearchFiles(t *testing.T) {
	s, _ := servidorDeTeste(t)

	res := s.dispatch(Request{Tool: "read_file", Path: "docs/*.md"})

	if res.Status != "error" {
		t.Fatal("curinga em read_file não pode dar sucesso")
	}
	if !strings.Contains(res.Error, "search_files") {
		t.Errorf("o erro não indica a ferramenta certa: %q", res.Error)
	}
}

// O Amazzo é o agente pessoal do usuário: ele PRECISA ler o .env para saber
// onde as chaves ficam e usá-las. Bloquear a leitura tornava o agente inútil
// para o que ele existe. A proteção contra vazamento vive na escrita.
func TestArquivoDeSegredoPodeSerLido(t *testing.T) {
	s, fs := servidorDeTeste(t)
	raiz := fs.Root()
	os.WriteFile(filepath.Join(raiz, ".env"), []byte("HF_TOKEN=hf_segredo123\n"), 0o600)
	os.WriteFile(filepath.Join(raiz, "chave.pem"), []byte("-----BEGIN PRIVATE KEY-----\n"), 0o600)

	for _, caminho := range []string{".env", "chave.pem"} {
		res := s.dispatch(Request{Tool: "read_file", Path: caminho})
		if res.Status != "success" {
			t.Errorf("%s deveria ser legível: %s", caminho, res.Error)
		}
	}
}

// A busca também enxerga esses arquivos: "onde está minha chave da OpenAI?" é
// uma pergunta legítima para o agente pessoal.
func TestBuscaEnxergaArquivoDeSegredo(t *testing.T) {
	s, fs := servidorDeTeste(t)
	os.WriteFile(filepath.Join(fs.Root(), ".env"), []byte("HF_TOKEN=hf_segredo123\n"), 0o600)

	res := s.dispatch(Request{Tool: "search_files", Pattern: "HF_TOKEN", Path: "."})

	if !strings.Contains(res.Content, ".env") {
		t.Errorf("a busca não achou o .env:\n%s", res.Content)
	}
}

// config/ e credentials/ são pacotes Go neste projeto: bloquear por diretório
// tornaria o próprio código-fonte ilegível para o agente.
func TestCodigoFonteDeConfigContinuaLegivel(t *testing.T) {
	s, fs := servidorDeTeste(t)
	raiz := fs.Root()
	os.MkdirAll(filepath.Join(raiz, "credentials"), 0o755)
	os.MkdirAll(filepath.Join(raiz, "config"), 0o755)
	os.WriteFile(filepath.Join(raiz, "credentials", "manager.go"), []byte("package credentials\n"), 0o644)
	os.WriteFile(filepath.Join(raiz, "config", "config.go"), []byte("package config\n"), 0o644)

	for _, caminho := range []string{"credentials/manager.go", "config/config.go"} {
		if res := s.dispatch(Request{Tool: "read_file", Path: caminho}); res.Status != "success" {
			t.Errorf("%s deveria ser legível: %s", caminho, res.Error)
		}
	}
	// Chave pública não é segredo.
	os.WriteFile(filepath.Join(raiz, "id_rsa.pub"), []byte("ssh-rsa AAAA\n"), 0o644)
	if res := s.dispatch(Request{Tool: "read_file", Path: "id_rsa.pub"}); res.Status != "success" {
		t.Errorf("chave pública deveria ser legível: %s", res.Error)
	}
}

// Visto na TUI: "❌ edit_file (): edit_file requer `search`". Dois defeitos numa
// linha — o erro não dizia o que fazer, e não carregava o Path, então a tela
// mostrava parênteses vazios.
func TestEditFileSemSearchExplicaOQueFazer(t *testing.T) {
	s, _ := servidorDeTeste(t)

	res := s.dispatch(Request{Tool: "edit_file", Path: "MELHORIAS.md", Replace: "novo texto"})

	if res.Status != "error" {
		t.Fatal("edit_file sem search não pode dar sucesso")
	}
	if res.Path != "MELHORIAS.md" {
		t.Errorf("o Path não voltou no resultado (a tela mostra \"()\"): %q", res.Path)
	}
	if !strings.Contains(res.Error, "write_file") {
		t.Errorf("o erro não diz qual ferramenta usar: %q", res.Error)
	}
}

// Quando o modelo manda o arquivo inteiro em `content`, ele queria write_file.
// Dizer isso explicitamente economiza um ciclo de 20–60s.
func TestEditFileComContentApontaParaWriteFile(t *testing.T) {
	s, _ := servidorDeTeste(t)

	res := s.dispatch(Request{Tool: "edit_file", Path: "a.md", Content: "# arquivo inteiro\n"})

	if res.Status != "error" {
		t.Fatal("edit_file com content e sem search não pode dar sucesso")
	}
	if !strings.Contains(res.Error, "write_file") {
		t.Errorf("o erro não redireciona para write_file: %q", res.Error)
	}
}

// Ler o .env é permitido; o segredo SAIR dali não pode passar despercebido.
// Aconteceu de verdade: o HF_TOKEN foi parar num docs/MELHORIAS.md como
// "exemplo de versionamento", e o .gitignore não cobre docs/.
func TestSegredoIndoParaArquivoComumGeraAviso(t *testing.T) {
	s, fs := servidorDeTeste(t)
	os.WriteFile(filepath.Join(fs.Root(), ".env"),
		[]byte("HF_TOKEN=hf_umtokenbemcompridoaqui123\n"), 0o600)

	res := s.dispatch(Request{Tool: "write_file", Path: "docs/NOTAS.md",
		Content: "const Version = \"1.0.0+hf_umtokenbemcompridoaqui123\"\n"})

	// Avisa, mas NÃO bloqueia: a decisão é do usuário.
	if res.Status != "success" {
		t.Fatalf("o aviso virou bloqueio: %s", res.Error)
	}
	if !strings.Contains(res.Content, "ATENÇÃO") || !strings.Contains(res.Content, ".env") {
		t.Errorf("o vazamento passou sem aviso: %q", res.Content)
	}
}

// Gravar a chave no próprio .env é trabalho legítimo do agente pessoal.
func TestEscreverNoProprioEnvNaoAvisa(t *testing.T) {
	s, fs := servidorDeTeste(t)
	os.WriteFile(filepath.Join(fs.Root(), ".env"),
		[]byte("HF_TOKEN=hf_umtokenbemcompridoaqui123\n"), 0o600)

	res := s.dispatch(Request{Tool: "write_file", Path: ".env",
		Content: "HF_TOKEN=hf_umtokenbemcompridoaqui123\nOPENAI_KEY=sk_outracoisa\n"})

	if strings.Contains(res.Content, "ATENÇÃO") {
		t.Errorf("avisou sobre gravar a chave no próprio .env: %q", res.Content)
	}
}

// Valores curtos ("true", "8080") dariam falso positivo o tempo todo.
func TestValorCurtoDoEnvNaoGeraFalsoPositivo(t *testing.T) {
	s, fs := servidorDeTeste(t)
	os.WriteFile(filepath.Join(fs.Root(), ".env"), []byte("DEBUG=true\nPORT=8080\n"), 0o600)

	res := s.dispatch(Request{Tool: "write_file", Path: "docs/NOTAS.md",
		Content: "O servidor sobe na porta 8080 com DEBUG=true.\n"})

	if strings.Contains(res.Content, "ATENÇÃO") {
		t.Errorf("falso positivo com valor curto: %q", res.Content)
	}
}

// Visto no MELHORIAS.md que o agente gerou: `find . -name \"*.py\"` no meio do
// markdown. As quebras de linha vieram certas, então a heurística do \n não
// dispara — mas aspa escapada em prosa nunca é intencional.
func TestAspasEscapadasSaemDoMarkdown(t *testing.T) {
	s, fs := servidorDeTeste(t)

	res := s.dispatch(Request{Tool: "write_file", Path: "NOTAS.md",
		Content: "## Testes\n(executar 'find . -name \\\"*.go\\\"' para verificar)\n"})
	if res.Status != "success" {
		t.Fatal(res.Error)
	}

	b, _ := fs.ReadFile("NOTAS.md")
	if strings.Contains(string(b), `\"`) {
		t.Errorf("aspa escapada ficou no markdown:\n%s", b)
	}
	if !strings.Contains(string(b), `"*.go"`) {
		t.Errorf("as aspas se perderam:\n%s", b)
	}
}

// Em código-fonte \" dentro de um literal de string é legítimo: trocar por "
// quebraria a compilação.
func TestAspasEscapadasEmCodigoGoSaoPreservadas(t *testing.T) {
	s, fs := servidorDeTeste(t)
	original := "package main\n\nfunc main() {\n\tprintln(\"diga \\\"oi\\\"\")\n}\n"

	s.dispatch(Request{Tool: "write_file", Path: "main.go", Content: original})

	b, _ := fs.ReadFile("main.go")
	if string(b) != original {
		t.Errorf("o código foi alterado:\n%q\nesperava:\n%q", string(b), original)
	}
}

// O Content de execute_command é lido pelo MODELO, não só pela tela. Vinha com
// "✅ Comando executado com sucesso:" na frente, e o modelo copiou a linha
// inteira — com emoji — para dentro de um docs/project_improvements.md que
// escreveu. O Status já diz que deu certo.
func TestSaidaDeComandoNaoLevaDecoracaoDeTela(t *testing.T) {
	s, _ := servidorDeTeste(t)

	res := s.dispatch(Request{Tool: "execute_command", Command: "echo alfa"})

	if res.Status != "success" {
		t.Fatalf("comando falhou: %s", res.Error)
	}
	if strings.Contains(res.Content, "✅") || strings.Contains(res.Content, "sucesso") {
		t.Errorf("decoração de tela foi para o dado que o modelo lê: %q", res.Content)
	}
	if strings.TrimSpace(res.Content) != "alfa" {
		t.Errorf("saída crua esperada %q, obtive %q", "alfa", strings.TrimSpace(res.Content))
	}
}

// Comando sem saída precisa dizer isso, senão o modelo recebe vazio e acha que
// a ferramenta falhou.
func TestComandoSemSaidaDizQueNaoTeveSaida(t *testing.T) {
	s, _ := servidorDeTeste(t)

	res := s.dispatch(Request{Tool: "execute_command", Command: "true"})

	if res.Status != "success" {
		t.Fatalf("comando falhou: %s", res.Error)
	}
	if strings.TrimSpace(res.Content) == "" {
		t.Error("o modelo recebeu conteúdo vazio")
	}
}
