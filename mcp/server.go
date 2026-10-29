package mcp

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"amazzo/filesystem"
)

// Request representa uma operação MCP pedida pelo modelo.
type Request struct {
	Tool    string `json:"tool"`
	Path    string `json:"path,omitempty"`
	Content string `json:"content,omitempty"`
	Append  bool   `json:"append,omitempty"`

	// Campos para edit_file
	Search  string `json:"search,omitempty"`
	Replace string `json:"replace,omitempty"`

	// Campos para search_files
	Pattern string `json:"pattern,omitempty"`
	Regex   bool   `json:"regex,omitempty"`

	// Campos para execute_command
	Command string `json:"command,omitempty"`

	// Campos para move_file
	DestPath string `json:"dest_path,omitempty"`

	// Campos para web_fetch/web_search
	URL   string `json:"url,omitempty"`
	Query string `json:"query,omitempty"`
}

// Result descreve o resultado de uma Request.
type Result struct {
	Tool    string `json:"tool"`
	Path    string `json:"path,omitempty"`
	Status  string `json:"status"`
	Content string `json:"content,omitempty"`
	Error   string `json:"error,omitempty"`
}

// Server coordena a execução das requests em paralelo.
type Server struct {
	fs *filesystem.Manager
}

// NewServer cria um MCP Server baseado no filesystem seguro.
func NewServer(fs *filesystem.Manager) *Server {
	return &Server{fs: fs}
}

// aliases mapeia todo nome aceito para a ferramenta canônica.
//
// Os modelos pequenos variam o nome (list_files, mkdir, grep) e antes cada
// variante caía no default do dispatch como "ferramenta não suportada". A
// tabela é a fonte da verdade tanto para o dispatch quanto para o parser de
// texto do app, que usa o nome para separar uma chamada de um JSON qualquer.
var aliases = map[string]string{
	"read_file":   "read_file",
	"write_file":  "write_file",
	"append_file": "append_file",
	"edit_file":   "edit_file",
	"delete_file": "delete_file", "remove_file": "delete_file",
	"list_dir": "list_dir", "list_files": "list_dir", "list_directory": "list_dir",
	"make_dir": "make_dir", "mkdir": "make_dir", "create_dir": "make_dir",
	"search_files": "search_files", "search_in_files": "search_files", "grep": "search_files",
	"execute_command": "execute_command", "exec": "execute_command", "run": "execute_command",
	"file_info": "file_info", "stat": "file_info",
	"move_file": "move_file", "mv": "move_file", "rename": "move_file",
	"web_fetch": "web_fetch", "fetch_url": "web_fetch",
	"web_search": "web_search", "search_web": "web_search",

	// set_workspace é resolvida pelo app (muda o alcance do agente, não um
	// arquivo), mas precisa ser reconhecida como ferramenta válida.
	"set_workspace": "set_workspace",
}

// FerramentaSuportada diz se o nome corresponde a uma ferramenta que o Amazzo
// sabe executar.
func FerramentaSuportada(nome string) bool {
	_, ok := aliases[strings.ToLower(strings.TrimSpace(nome))]
	return ok
}

// Handle executa a lista de requests EM ORDEM e retorna os resultados.
//
// A ordem é o contrato: o modelo emite "cria o arquivo" e depois "edita o
// arquivo", ou "cria a pasta" e depois "escreve dentro dela". Rodando em
// paralelo o mesmo lote dava resultados diferentes a cada tentativa (medimos
// cinco desfechos distintos em cinco execuções do mesmo write+edit). Não há
// nada a ganhar com paralelismo aqui: são operações de disco local, sub-ms.
func (s *Server) Handle(ctx context.Context, requests []Request) ([]Result, error) {
	if len(requests) == 0 {
		return nil, nil
	}

	results := make([]Result, 0, len(requests))

	for _, req := range requests {
		if err := ctx.Err(); err != nil {
			results = append(results, Result{
				Tool:   req.Tool,
				Path:   req.Path,
				Status: "error",
				Error:  "execução cancelada",
			})
			return results, err
		}
		results = append(results, s.dispatch(req))
	}

	return results, nil
}

func (s *Server) dispatch(req Request) Result {
	switch aliases[strings.ToLower(strings.TrimSpace(req.Tool))] {
	case "read_file":
		return s.readFile(req)
	case "write_file":
		return s.writeFile(req, false)
	case "append_file":
		return s.writeFile(req, true)
	case "edit_file":
		return s.editFile(req)
	case "delete_file":
		return s.deleteFile(req)
	case "list_dir":
		return s.listDir(req)
	case "make_dir":
		return s.makeDir(req)
	case "search_files":
		return s.searchFiles(req)
	case "execute_command":
		return s.executeCommand(req)
	case "file_info":
		return s.fileInfo(req)
	case "move_file":
		return s.moveFile(req)
	case "web_fetch":
		return s.webFetch(req)
	case "web_search":
		return s.webSearch(req)
	default:
		return Result{
			Tool:   req.Tool,
			Path:   req.Path,
			Status: "error",
			Error:  "ferramenta não suportada pelo MCP",
		}
	}
}

// arquivosDeSegredo casa nomes que guardam credenciais.
//
// LER esses arquivos e permitido de proposito: o Amazzo e o agente pessoal do
// usuario e precisa saber onde as chaves ficam para usa-las. O que nao pode e
// o segredo SAIR dali -- o agente ja leu o .env deste projeto e colou o
// HF_TOKEN dentro de um docs/MELHORIAS.md que ele mesmo escreveu, como se
// fosse exemplo de versionamento. O .gitignore protegia o .env; nao protegia
// o docs/. Por isso a checagem vive no caminho de ESCRITA (ver segredoVazando).
//
// A checagem e por NOME de arquivo, nunca por diretorio: aqui existem os
// pacotes Go `config/` e `credentials/`, que sao codigo-fonte e precisam ser
// legiveis.
var arquivosDeSegredo = []string{
	".env", ".envrc", ".netrc", ".npmrc", ".pypirc", ".htpasswd",
	".git-credentials", "credentials.json", "secrets.json",
	"secrets.yaml", "secrets.yml", "id_rsa", "id_dsa", "id_ecdsa", "id_ed25519",
}

var sufixosDeSegredo = []string{
	".pem", ".key", ".p12", ".pfx", ".keystore", ".jks", ".ppk",
}

// ehSegredo diz se o caminho aponta para um arquivo de credencial.
func ehSegredo(caminho string) bool {
	nome := strings.ToLower(filepath.Base(caminho))

	// Chave publica nao e segredo, e id_rsa.pub casaria a regra de prefixo
	// abaixo. A checagem vem antes de tudo por isso.
	if strings.HasSuffix(nome, ".pub") {
		return false
	}

	for _, exato := range arquivosDeSegredo {
		if nome == exato || strings.HasPrefix(nome, exato+".") {
			return true
		}
	}
	for _, sufixo := range sufixosDeSegredo {
		if strings.HasSuffix(nome, sufixo) {
			return true
		}
	}
	return false
}

// segredoVazando devolve um aviso quando o conteudo a ser gravado num arquivo
// COMUM contem um valor que veio de um arquivo de credencial do projeto.
//
// Ler o .env e permitido -- o agente precisa disso. O que nao pode passar
// despercebido e o segredo SAIR dali: aconteceu de verdade, o HF_TOKEN foi
// parar dentro de um docs/MELHORIAS.md como "exemplo de versionamento", e o
// .gitignore nao cobre docs/.
//
// E um aviso, nao um bloqueio: gravar a chave no proprio .env e trabalho
// legitimo, e adivinhar a intencao do usuario aqui seria pior que avisar.
func (s *Server) segredoVazando(destino, conteudo string) string {
	if ehSegredo(destino) || len(conteudo) == 0 {
		return ""
	}

	for _, nome := range []string{".env", ".env.local", ".envrc"} {
		dados, err := s.fs.ReadFile(nome)
		if err != nil {
			continue
		}
		for _, linha := range strings.Split(string(dados), "\n") {
			_, valor, achou := strings.Cut(strings.TrimSpace(linha), "=")
			if !achou {
				continue
			}
			valor = strings.Trim(strings.TrimSpace(valor), `"'`+"`")
			// Valores curtos ("true", "8080") dariam falso positivo o tempo todo.
			if len(valor) < 12 || !strings.Contains(conteudo, valor) {
				continue
			}
			return " ⚠️ ATENÇÃO: este conteúdo contém um valor do " + nome +
				". Segredo não deve ser escrito em arquivo que não seja de credencial." +
				" Remova-o e use uma referência (os.Getenv) no lugar."
		}
	}

	return ""
}

func (s *Server) readFile(req Request) Result {
	if strings.TrimSpace(req.Path) == "" {
		return Result{Tool: req.Tool, Status: "error", Error: "read_file requer `path`"}
	}

	// Curinga em read_file: o modelo pede "docs/*.md" esperando ler tudo de uma
	// vez. O erro cru do OS ("no such file or directory") nao diz o que fazer, e
	// ele gastava ciclos tentando variacoes do mesmo caminho.
	if strings.ContainsAny(req.Path, "*?") {
		return Result{Tool: req.Tool, Path: req.Path, Status: "error",
			Error: "read_file não aceita curinga. Use search_files com o padrão " +
				"(ex.: {\"tool\":\"search_files\",\"pattern\":\"*.md\"}) para descobrir " +
				"os nomes, e depois read_file em cada arquivo."}
	}

	// Diretorio em read_file: mesmo problema. "is a directory" nao sugere a
	// ferramenta certa; dizer o nome dela economiza um ciclo de 20-60s.
	if info, _, err := s.fs.FileInfo(req.Path); err == nil && info.IsDir() {
		return Result{Tool: req.Tool, Path: req.Path, Status: "error",
			Error: req.Path + " é um diretório, não um arquivo. Use list_dir para ver o conteúdo dele."}
	}

	content, err := s.fs.ReadFile(req.Path)
	if err != nil {
		return Result{Tool: req.Tool, Path: req.Path, Status: "error", Error: err.Error()}
	}

	return Result{
		Tool:    req.Tool,
		Path:    req.Path,
		Status:  "success",
		Content: string(content),
	}
}

func (s *Server) writeFile(req Request, append bool) Result {
	if strings.TrimSpace(req.Path) == "" {
		return Result{Tool: req.Tool, Status: "error", Error: "write/append_file requer `path`"}
	}
	// Conteúdo vazio é legítimo: um .gitkeep, um esqueleto de arquivo, um
	// __init__.py. Recusar travava o agente numa operação que o usuário
	// realmente pede.

	conteudo := req.Content
	nota := ""
	// Arquivo inteiro numa linha, com "\n" literal no lugar das quebras: o
	// modelo errou o escape do JSON. Gravar assim produz um arquivo ilegível
	// que nem compila, e ele acha que deu certo.
	if pareceEscapeLiteral(conteudo) {
		conteudo = desescapa(conteudo)
		nota = " (o conteúdo vinha com \\n literal em vez de quebras de linha; interpretei os escapes)"
	} else if ehTextoSimples(req.Path) && strings.Contains(conteudo, `\"`) {
		// Aspas escapadas em arquivo de texto: o modelo errou o escape do JSON
		// de novo, so que dessa vez as quebras de linha vieram certas -- entao
		// pareceEscapeLiteral nao dispara. Visto no MELHORIAS.md gerado:
		// `find . -name \"*.py\"` no meio do markdown.
		//
		// So vale para texto puro. Em codigo-fonte \" dentro de um literal de
		// string e legitimo, e trocar por " quebraria a compilacao.
		conteudo = strings.ReplaceAll(conteudo, `\"`, `"`)
		nota = " (as aspas vinham escapadas; corrigi)"
	}

	nota += s.segredoVazando(req.Path, conteudo)

	path, err := s.fs.WriteFile(req.Path, []byte(conteudo), append)
	if err != nil {
		return Result{Tool: req.Tool, Path: req.Path, Status: "error", Error: err.Error()}
	}

	label := "salvo"
	if append {
		label = "atualizado"
	}
	return Result{
		Tool:    req.Tool,
		Path:    req.Path,
		Status:  "success",
		Content: fmt.Sprintf("%s %s em %s%s", req.Path, label, path, nota),
	}
}

// ehTextoSimples diz se o arquivo e prosa, onde uma aspa escapada nunca e
// intencional -- ao contrario de codigo-fonte, em que \" dentro de um literal
// de string e legitimo.
func ehTextoSimples(caminho string) bool {
	switch strings.ToLower(filepath.Ext(caminho)) {
	case ".md", ".markdown", ".txt", ".rst", ".adoc", ".text":
		return true
	}
	return false
}

func (s *Server) listDir(req Request) Result {
	dir := "."
	if strings.TrimSpace(req.Path) != "" {
		dir = req.Path
	}

	entries, resolved, err := s.fs.List(dir)
	if err != nil {
		return Result{Tool: req.Tool, Path: dir, Status: "error", Error: err.Error()}
	}

	var builder strings.Builder
	builder.WriteString(fmt.Sprintf("📁 Conteúdo de %s\n", resolved))
	for _, entry := range entries {
		suffix := ""
		if entry.IsDir() {
			suffix = "/"
		}
		builder.WriteString(" - " + entry.Name() + suffix + "\n")
	}

	content := strings.TrimSuffix(builder.String(), "\n")
	return Result{
		Tool:    req.Tool,
		Path:    dir,
		Status:  "success",
		Content: content,
	}
}

func (s *Server) makeDir(req Request) Result {
	if strings.TrimSpace(req.Path) == "" {
		return Result{Tool: req.Tool, Status: "error", Error: "make_dir requer `path`"}
	}

	path, err := s.fs.Mkdir(req.Path)
	if err != nil {
		return Result{Tool: req.Tool, Path: req.Path, Status: "error", Error: err.Error()}
	}

	return Result{
		Tool:    req.Tool,
		Path:    req.Path,
		Status:  "success",
		Content: fmt.Sprintf("Diretório garantido em %s", path),
	}
}

func (s *Server) editFile(req Request) Result {
	if strings.TrimSpace(req.Path) == "" {
		return Result{Tool: req.Tool, Status: "error",
			Error: "edit_file requer `path`, `search` e `replace`."}
	}

	// `search` vazio quase sempre significa "quero trocar o arquivo inteiro".
	// A mensagem antiga ("edit_file requer `search`") nao dizia o que fazer, e
	// o modelo gastava um ciclo de 20-60s para descobrir sozinho. Ela tambem
	// nao carregava o Path, entao a tela mostrava "edit_file ()".
	if strings.TrimSpace(req.Search) == "" {
		erro := "edit_file requer `search`: o trecho EXATO que deve sair do arquivo. " +
			"Para substituir o arquivo inteiro use write_file com `content`. " +
			"Para trocar um trecho, leia o arquivo com read_file e copie o texto exato para `search`."
		if strings.TrimSpace(req.Content) != "" {
			erro = "edit_file não usa `content`. Você mandou o conteúdo inteiro: " +
				"para isso a ferramenta é write_file com `path` e `content`. " +
				"edit_file serve para trocar um trecho, e precisa de `search` e `replace`."
		}
		return Result{Tool: req.Tool, Path: req.Path, Status: "error", Error: erro}
	}

	message, err := s.fs.EditFile(req.Path, req.Search, req.Replace)

	// Modelo pequeno erra o escape do JSON com frequencia: manda "\\n" (barra
	// invertida + n) onde queria uma quebra de linha real. Medido com o
	// lfm2.5:8b tentando corrigir uma funcao de tres linhas -- a busca nunca
	// casava e ele desistia dizendo que tinha corrigido.
	//
	// Tentamos o texto exato primeiro: um arquivo Go pode conter "\\n" de
	// verdade, dentro de um literal de string. So se o exato falhar
	// interpretamos os escapes -- assim nenhum caso que funcionava se quebra.
	if err != nil {
		if alt := desescapa(req.Search); alt != req.Search {
			if msgAlt, errAlt := s.fs.EditFile(req.Path, alt, desescapa(req.Replace)); errAlt == nil {
				return Result{
					Tool:   req.Tool,
					Path:   req.Path,
					Status: "success",
					Content: fmt.Sprintf("Arquivo editado: %s "+
						"(o `search` vinha com \\n literal em vez de quebra de linha; "+
						"interpretei os escapes)", msgAlt),
				}
			}
		}

		// Um `search` que nao casa e o erro n 1 de modelo pequeno: sobra ou
		// falta um espaco, uma tabulacao, uma quebra de linha. Sem pista ele
		// repete a mesma chamada errada ate estourar o limite de ciclos.
		// Mostrar as linhas parecidas da a ele o texto exato para tentar de novo.
		erro := err.Error()
		if dica := s.linhasParecidas(req.Path, req.Search); dica != "" {
			erro += "\n" + dica
		}
		return Result{Tool: req.Tool, Path: req.Path, Status: "error", Error: erro}
	}

	return Result{
		Tool:    req.Tool,
		Path:    req.Path,
		Status:  "success",
		Content: fmt.Sprintf("Arquivo editado: %s", message),
	}
}

// desescapa interpreta as sequencias de escape que um modelo escreve
// literalmente quando erra o escape do JSON: \n, \t, \r, \" e \\.
func desescapa(s string) string {
	if !strings.Contains(s, `\`) {
		return s
	}

	var sb strings.Builder
	sb.Grow(len(s))

	for i := 0; i < len(s); i++ {
		if s[i] != '\\' || i+1 >= len(s) {
			sb.WriteByte(s[i])
			continue
		}

		switch s[i+1] {
		case 'n':
			sb.WriteByte('\n')
		case 't':
			sb.WriteByte('\t')
		case 'r':
			sb.WriteByte('\r')
		case '"':
			sb.WriteByte('"')
		case '\\':
			sb.WriteByte('\\')
		default:
			sb.WriteByte(s[i])
			continue
		}
		i++
	}

	return sb.String()
}

// pareceEscapeLiteral diz se o conteudo veio com as quebras de linha
// escapadas: nenhuma quebra real, mas varios "\n" literais.
//
// E o suficiente para nao errar: um arquivo de uma unica linha com dois ou
// mais "\n" dentro de literais de string e raro; um arquivo inteiro colado
// numa linha com "\n" no lugar das quebras e o que o modelo produz quando
// erra o escape.
func pareceEscapeLiteral(conteudo string) bool {
	return !strings.Contains(conteudo, "\n") &&
		strings.Count(conteudo, `\n`) >= 2
}

// linhasParecidas descreve as linhas do arquivo mais próximas do trecho
// procurado, para o modelo poder repetir a edição com o texto exato.
func (s *Server) linhasParecidas(path, search string) string {
	content, err := s.fs.ReadFile(path)
	if err != nil {
		return ""
	}

	// Compara ignorando espaço: é onde o modelo erra.
	alvo := semEspacos(primeiraLinha(search))
	if alvo == "" {
		return ""
	}

	var achados []string
	for i, linha := range strings.Split(string(content), "\n") {
		if strings.Contains(semEspacos(linha), alvo) {
			achados = append(achados, fmt.Sprintf("  linha %d: %s", i+1, linha))
			if len(achados) >= 3 {
				break
			}
		}
	}
	if len(achados) == 0 {
		return "Nenhuma linha parecida. Leia o arquivo com read_file antes de editar."
	}

	return "Linhas parecidas (use o texto EXATO, com os mesmos espaços):\n" +
		strings.Join(achados, "\n")
}

func primeiraLinha(s string) string {
	if i := strings.IndexByte(s, '\n'); i >= 0 {
		return s[:i]
	}
	return s
}

func semEspacos(s string) string {
	return strings.Join(strings.Fields(s), "")
}

// pastasIgnoradas ficam fora da busca: não têm código-fonte do projeto e
// enchem o contexto do modelo com ruído. Em projetos Go, `build/` guarda o
// binário — nesta pasta, 23 MB dele.
var pastasIgnoradas = map[string]bool{
	".git": true, ".svn": true, ".hg": true,
	"node_modules": true, "vendor": true, "build": true, "dist": true,
	"target": true, "__pycache__": true, ".venv": true, "venv": true,
	".cache": true, ".next": true, ".idea": true, ".vscode": true,
}

// pareceBinario detecta binário do jeito que o próprio git detecta: um byte
// NUL no começo do arquivo. Casar um padrão dentro de um executável devolve
// lixo ilegível e ainda gasta a janela de contexto.
func pareceBinario(dados []byte) bool {
	limite := len(dados)
	if limite > 8000 {
		limite = 8000
	}
	for i := 0; i < limite; i++ {
		if dados[i] == 0 {
			return true
		}
	}
	return false
}

func (s *Server) deleteFile(req Request) Result {
	if strings.TrimSpace(req.Path) == "" {
		return Result{Tool: req.Tool, Status: "error", Error: "delete_file requer `path`"}
	}

	if err := s.fs.DeleteFile(req.Path); err != nil {
		return Result{Tool: req.Tool, Path: req.Path, Status: "error", Error: err.Error()}
	}

	return Result{
		Tool:    req.Tool,
		Path:    req.Path,
		Status:  "success",
		Content: fmt.Sprintf("Arquivo %s deletado com sucesso", req.Path),
	}
}

func (s *Server) searchFiles(req Request) Result {
	pattern := req.Pattern
	if pattern == "" {
		pattern = req.Search
	}
	if pattern == "" {
		return Result{Tool: req.Tool, Status: "error", Error: "search_files requer `pattern`"}
	}

	dir := req.Path
	if dir == "" {
		dir = "."
	}

	resolved, err := s.fs.Resolve(dir)
	if err != nil {
		return Result{Tool: req.Tool, Path: dir, Status: "error", Error: err.Error()}
	}

	var re *regexp.Regexp
	if req.Regex {
		re, err = regexp.Compile(pattern)
		if err != nil {
			return Result{Tool: req.Tool, Status: "error",
				Error: fmt.Sprintf("regex inválida: %v", err)}
		}
	}

	// Modelos pequenos usam search_files das duas maneiras: procurar TEXTO
	// dentro dos arquivos e procurar ARQUIVO por nome. Observado com o
	// lfm2.5:8b: ele pediu o padrao "*.go" esperando a lista de fontes, e a
	// busca por conteudo devolveu "nenhum resultado" -- ele seguiu adiante sem
	// nunca ver o codigo. Um padrao com curinga e um pedido de nome de arquivo.
	if strings.ContainsAny(pattern, "*?") && !req.Regex {
		return s.buscaPorNome(req, pattern, dir, resolved)
	}

	const limite = 100

	var matches []string
	// errLimite encerra a caminhada inteira ao atingir o limite. Antes usávamos
	// filepath.SkipDir, que só pula o resto do diretório atual: a busca
	// continuava varrendo o projeto e devolvia mais resultados do que o limite.
	errLimite := errors.New("limite atingido")

	walkErr := filepath.Walk(resolved, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return nil
		}

		if info.IsDir() {
			if path != resolved && pastasIgnoradas[info.Name()] {
				return filepath.SkipDir
			}
			return nil
		}

		if info.Size() > 2*1024*1024 {
			return nil
		}

		content, err := os.ReadFile(path)
		if err != nil || pareceBinario(content) {
			return nil
		}

		relPath, _ := filepath.Rel(resolved, path)
		for i, line := range strings.Split(string(content), "\n") {
			var found bool
			if req.Regex {
				found = re.MatchString(line)
			} else {
				found = strings.Contains(line, pattern)
			}
			if !found {
				continue
			}

			matches = append(matches, fmt.Sprintf("%s:%d: %s", relPath, i+1, strings.TrimSpace(line)))
			if len(matches) >= limite {
				return errLimite
			}
		}

		return nil
	})

	if len(matches) == 0 {
		return Result{
			Tool:    req.Tool,
			Path:    dir,
			Status:  "success",
			Content: fmt.Sprintf("Nenhum resultado encontrado para '%s' em %s", pattern, dir),
		}
	}

	content := fmt.Sprintf("📝 Encontrados %d resultado(s) para '%s':\n\n%s",
		len(matches), pattern, strings.Join(matches, "\n"))

	if errors.Is(walkErr, errLimite) {
		content += fmt.Sprintf("\n\n⚠️ Limite de %d resultados atingido. Use um padrão mais específico.", limite)
	}

	return Result{
		Tool:    req.Tool,
		Path:    dir,
		Status:  "success",
		Content: content,
	}
}

// buscaPorNome lista os arquivos cujo nome casa com o glob.
func (s *Server) buscaPorNome(req Request, pattern, dir, resolved string) Result {
	const limite = 200

	var achados []string
	errLimite := errors.New("limite atingido")

	walkErr := filepath.Walk(resolved, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return nil
		}
		if info.IsDir() {
			if path != resolved && pastasIgnoradas[info.Name()] {
				return filepath.SkipDir
			}
			return nil
		}

		casa, _ := filepath.Match(pattern, info.Name())
		if !casa {
			return nil
		}

		rel, _ := filepath.Rel(resolved, path)
		achados = append(achados, fmt.Sprintf("%s (%d bytes)", rel, info.Size()))
		if len(achados) >= limite {
			return errLimite
		}
		return nil
	})

	if len(achados) == 0 {
		return Result{
			Tool:   req.Tool,
			Path:   dir,
			Status: "success",
			Content: fmt.Sprintf("Nenhum arquivo com nome casando '%s' em %s. "+
				"Para procurar TEXTO dentro dos arquivos, use um padrão sem curinga.",
				pattern, dir),
		}
	}

	content := fmt.Sprintf("📄 %d arquivo(s) com nome casando '%s':\n\n%s",
		len(achados), pattern, strings.Join(achados, "\n"))
	if errors.Is(walkErr, errLimite) {
		content += fmt.Sprintf("\n\n⚠️ Limite de %d arquivos atingido.", limite)
	}

	return Result{Tool: req.Tool, Path: dir, Status: "success", Content: content}
}

func (s *Server) executeCommand(req Request) Result {
	command := req.Command
	if command == "" {
		return Result{Tool: req.Tool, Status: "error", Error: "execute_command requer `command`"}
	}

	// A autorização de execute_command é do usuário: toda chamada passa pela
	// tela de confirmação, que mostra o comando inteiro. Aqui recusamos só o
	// que é irreversível ou está fora do escopo de mexer num projeto.
	if motivo := comandoPerigoso(command); motivo != "" {
		return Result{
			Tool:   req.Tool,
			Status: "error",
			Error:  fmt.Sprintf("Recusado: o comando %s.", motivo),
		}
	}

	// Build e teste de projeto grande passam de um minuto.
	ctx, cancel := context.WithTimeout(context.Background(), 120*time.Second)
	defer cancel()

	// Executa através do shell, dentro da pasta de trabalho.
	//
	// Antes o comando era quebrado em campos e executado direto, sem shell:
	// "cd docs && ls" virava o comando "cd" com "&&" e "ls" como argumentos.
	// Sem shell não há &&, pipe, redirecionamento nem glob — justamente o que
	// se usa para trabalhar num projeto.
	cmd := exec.CommandContext(ctx, "sh", "-c", command)
	cmd.Dir = s.fs.Root()

	output, err := cmd.CombinedOutput()
	outputStr := string(output)

	if err != nil {
		if ctx.Err() == context.DeadlineExceeded {
			return Result{
				Tool:    req.Tool,
				Status:  "error",
				Error:   "Comando excedeu o timeout de 120 segundos",
				Content: outputStr,
			}
		}

		return Result{
			Tool:    req.Tool,
			Status:  "error",
			Error:   fmt.Sprintf("Comando falhou: %v", err),
			Content: outputStr,
		}
	}

	// O conteudo e a saida CRUA do comando, sem enfeite.
	//
	// Antes vinha com "✅ Comando executado com sucesso:" na frente. Isso e
	// decoracao de tela, mas quem le o Content e o modelo -- e ele copiou a
	// linha inteira, com o emoji, para dentro de um docs/project_improvements.md
	// que escreveu. O Status ja diz que deu certo; a decoracao pertence ao
	// renderizador, nao ao dado.
	if strings.TrimSpace(outputStr) == "" {
		outputStr = "(comando terminou sem saída)"
	}
	return Result{
		Tool:    req.Tool,
		Status:  "success",
		Content: outputStr,
	}
}

func (s *Server) fileInfo(req Request) Result {
	if strings.TrimSpace(req.Path) == "" {
		return Result{Tool: req.Tool, Status: "error", Error: "file_info requer `path`"}
	}

	info, absolute, err := s.fs.FileInfo(req.Path)
	if err != nil {
		return Result{Tool: req.Tool, Path: req.Path, Status: "error", Error: err.Error()}
	}

	var fileType string
	if info.IsDir() {
		fileType = "Diretório"
	} else {
		fileType = "Arquivo"
	}

	content := fmt.Sprintf(
		"📊 Informações de %s:\n"+
			"  Nome: %s\n"+
			"  Tipo: %s\n"+
			"  Tamanho: %d bytes\n"+
			"  Modificado: %s\n"+
			"  Permissões: %s\n"+
			"  Caminho: %s",
		req.Path,
		info.Name(),
		fileType,
		info.Size(),
		info.ModTime().Format("2006-01-02 15:04:05"),
		info.Mode().String(),
		absolute,
	)

	return Result{
		Tool:    req.Tool,
		Path:    req.Path,
		Status:  "success",
		Content: content,
	}
}

func (s *Server) moveFile(req Request) Result {
	sourcePath := req.Path
	destPath := req.DestPath

	if sourcePath == "" {
		return Result{Tool: req.Tool, Status: "error", Error: "move_file requer `path` (arquivo origem)"}
	}
	if destPath == "" {
		return Result{Tool: req.Tool, Status: "error", Error: "move_file requer `dest_path` (arquivo destino)"}
	}

	message, err := s.fs.MoveFile(sourcePath, destPath)
	if err != nil {
		return Result{
			Tool:   req.Tool,
			Path:   sourcePath,
			Status: "error",
			Error:  err.Error(),
		}
	}

	return Result{
		Tool:    req.Tool,
		Path:    sourcePath,
		Status:  "success",
		Content: message,
	}
}

// webFetch busca conteúdo de uma URL.
func (s *Server) webFetch(req Request) Result {
	if strings.TrimSpace(req.URL) == "" {
		return Result{
			Tool:   req.Tool,
			Status: "error",
			Error:  "web_fetch requer `url`",
		}
	}

	// Cria requisição HTTP
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodGet, req.URL, nil)
	if err != nil {
		return Result{
			Tool:   req.Tool,
			Status: "error",
			Error:  fmt.Sprintf("erro ao criar requisição: %v", err),
		}
	}

	// User-Agent para evitar bloqueios
	httpReq.Header.Set("User-Agent", "Mozilla/5.0 (compatible; AmazzoBot/1.0)")
	httpReq.Header.Set("Accept", "text/html,application/xhtml+xml,application/xml;q=0.9,*/*;q=0.8")

	// Faz a requisição
	client := &http.Client{
		Timeout: 30 * time.Second,
	}
	resp, err := client.Do(httpReq)
	if err != nil {
		return Result{
			Tool:   req.Tool,
			Status: "error",
			Error:  fmt.Sprintf("erro ao buscar URL: %v", err),
		}
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return Result{
			Tool:   req.Tool,
			Status: "error",
			Error:  fmt.Sprintf("HTTP %d: %s", resp.StatusCode, resp.Status),
		}
	}

	// Lê o conteúdo (limitado a 500KB para evitar overflow)
	body, err := io.ReadAll(io.LimitReader(resp.Body, 512*1024))
	if err != nil {
		return Result{
			Tool:   req.Tool,
			Status: "error",
			Error:  fmt.Sprintf("erro ao ler resposta: %v", err),
		}
	}

	// Sanitiza HTML removendo tags, scripts, etc
	cleanContent := sanitizeHTML(string(body))

	// Limita tamanho para não poluir a tela (máximo 15000 caracteres)
	if len(cleanContent) > 15000 {
		cleanContent = cleanContent[:15000] + "\n\n... [conteúdo truncado para caber na resposta - total original: " +
			fmt.Sprintf("%d caracteres]", len(cleanContent))
	}

	return Result{
		Tool:    req.Tool,
		Status:  "success",
		Content: fmt.Sprintf("📄 Conteúdo de: %s\n\n%s", req.URL, cleanContent),
	}
}

// sanitizeHTML remove tags HTML, scripts e limpa o conteúdo
func sanitizeHTML(html string) string {
	// Remove scripts
	scriptRe := regexp.MustCompile(`(?i)<script[^>]*>[\s\S]*?</script>`)
	html = scriptRe.ReplaceAllString(html, "")

	// Remove styles
	styleRe := regexp.MustCompile(`(?i)<style[^>]*>[\s\S]*?</style>`)
	html = styleRe.ReplaceAllString(html, "")

	// Remove comentários HTML
	commentRe := regexp.MustCompile(`<!--[\s\S]*?-->`)
	html = commentRe.ReplaceAllString(html, "")

	// Remove todas as tags HTML (mantém o conteúdo)
	tagRe := regexp.MustCompile(`<[^>]*>`)
	text := tagRe.ReplaceAllString(html, " ")

	// Remove múltiplos espaços
	spaceRe := regexp.MustCompile(`\s+`)
	text = spaceRe.ReplaceAllString(text, " ")

	// Remove linhas vazias excessivas
	lineRe := regexp.MustCompile(`\n\s*\n\s*\n`)
	text = lineRe.ReplaceAllString(text, "\n\n")

	// Decodifica entidades HTML comuns
	text = strings.ReplaceAll(text, "&nbsp;", " ")
	text = strings.ReplaceAll(text, "&amp;", "&")
	text = strings.ReplaceAll(text, "&lt;", "<")
	text = strings.ReplaceAll(text, "&gt;", ">")
	text = strings.ReplaceAll(text, "&quot;", "\"")
	text = strings.ReplaceAll(text, "&#39;", "'")

	return strings.TrimSpace(text)
}

// webSearch realiza uma busca na web (simulada por enquanto, pode integrar com API real).
func (s *Server) webSearch(req Request) Result {
	if strings.TrimSpace(req.Query) == "" {
		return Result{
			Tool:   req.Tool,
			Status: "error",
			Error:  "web_search requer `query`",
		}
	}

	// Por enquanto, retorna uma mensagem explicativa
	// Você pode integrar com APIs como DuckDuckGo, Google Custom Search, etc.
	return Result{
		Tool:   req.Tool,
		Status: "info",
		Content: fmt.Sprintf(`Busca web por: "%s"

⚠️  Web search direto ainda não implementado.

Alternativas:
1. Use web_fetch com URL específica:
   {"tool": "web_fetch", "url": "https://exemplo.com"}

2. Peça ao usuário para fornecer links específicos

3. Integração futura com:
   - DuckDuckGo API
   - Google Custom Search
   - SearXNG
   - Brave Search API

Para agora, recomende URLs conhecidas ou peça ao usuário os links.`, req.Query),
	}
}
