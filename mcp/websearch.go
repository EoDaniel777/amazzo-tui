package mcp

import (
	"context"
	"fmt"
	"html"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"sync"
	"time"
)

var (
	resultadoDDG = regexp.MustCompile(`(?s)<a[^>]*class="result__a"[^>]*href="([^"]+)"[^>]*>(.*?)</a>`)
	snippetDDG   = regexp.MustCompile(`(?s)class="result__snippet"[^>]*>(.*?)</a>`)
	tagsHTML     = regexp.MustCompile(`<[^>]*>`)
)

// pesquisaWeb consulta a versão HTML do DuckDuckGo: grátis, sem chave. Anúncios
// (links de duckduckgo.com/y.js) são descartados.
func pesquisaWeb(req Request) Result {
	q := strings.TrimSpace(req.Query)
	if q == "" {
		return Result{Tool: req.Tool, Status: "error", Error: "web_search requer `query`"}
	}
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	form := url.Values{"q": {q}, "kl": {"br-pt"}}
	hr, err := http.NewRequestWithContext(ctx, http.MethodPost, "https://html.duckduckgo.com/html/", strings.NewReader(form.Encode()))
	if err != nil {
		return Result{Tool: req.Tool, Status: "error", Error: err.Error()}
	}
	hr.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	hr.Header.Set("User-Agent", "Mozilla/5.0 (X11; Linux x86_64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/124.0 Safari/537.36")
	resp, err := http.DefaultClient.Do(hr)
	if err != nil {
		return Result{Tool: req.Tool, Status: "error", Error: "busca falhou: " + err.Error()}
	}
	defer resp.Body.Close()
	corpo, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if resp.StatusCode != http.StatusOK {
		return Result{Tool: req.Tool, Status: "error",
			Error: fmt.Sprintf("o buscador respondeu HTTP %d (pode ser limite de uso; tente de novo em instantes ou use web_fetch numa URL conhecida)", resp.StatusCode)}
	}
	itens := parseDDG(string(corpo), 8)
	if len(itens) == 0 {
		return Result{Tool: req.Tool, Status: "success",
			Content: fmt.Sprintf("Nenhum resultado para %q. Reformule a busca ou use web_fetch numa URL conhecida.", q)}
	}
	var sb strings.Builder
	fmt.Fprintf(&sb, "Resultados para %q (use web_fetch para abrir um deles):\n\n", q)
	for i, it := range itens {
		fmt.Fprintf(&sb, "%d. %s\n   %s\n", i+1, it[0], it[1])
		if it[2] != "" {
			fmt.Fprintf(&sb, "   %s\n", it[2])
		}
		sb.WriteString("\n")
	}
	return Result{Tool: req.Tool, Status: "success", Content: strings.TrimSpace(sb.String())}
}

func textoLimpo(s string) string {
	return strings.Join(strings.Fields(html.UnescapeString(tagsHTML.ReplaceAllString(s, ""))), " ")
}

// parseDDG devolve [título, url, trecho] de até n resultados orgânicos.
func parseDDG(pagina string, n int) [][3]string {
	var out [][3]string
	marcas := resultadoDDG.FindAllStringSubmatchIndex(pagina, -1)
	for k, m := range marcas {
		link := html.UnescapeString(pagina[m[2]:m[3]])
		titulo := textoLimpo(pagina[m[4]:m[5]])
		if strings.HasPrefix(link, "//") {
			link = "https:" + link
		}
		if u, err := url.Parse(link); err == nil {
			if strings.HasSuffix(u.Host, "duckduckgo.com") {
				if u.Path == "/y.js" {
					continue // anúncio
				}
				if real := u.Query().Get("uddg"); real != "" {
					link = real
				}
			}
		}
		fim := len(pagina)
		if k+1 < len(marcas) {
			fim = marcas[k+1][0]
		}
		trecho := ""
		if sm := snippetDDG.FindStringSubmatch(pagina[m[1]:fim]); sm != nil {
			trecho = textoLimpo(sm[1])
		}
		out = append(out, [3]string{titulo, link, trecho})
		if len(out) >= n {
			break
		}
	}
	return out
}

var githubBlob = regexp.MustCompile(`^https?://github\.com/([^/]+)/([^/]+)/blob/(.+)$`)

// urlDeConteudo troca o link da página de um arquivo do GitHub pelo conteúdo
// bruto: a página HTML do GitHub, sem JavaScript, não traz o código.
func urlDeConteudo(u string) string {
	if m := githubBlob.FindStringSubmatch(strings.TrimSpace(u)); m != nil {
		return "https://raw.githubusercontent.com/" + m[1] + "/" + m[2] + "/" + m[3]
	}
	return u
}

// Página já baixada e convertida, guardada por alguns minutos: ler a página em
// várias fatias (offset) não deve baixar tudo de novo a cada fatia.
var cachePaginas = struct {
	sync.Mutex
	m map[string]paginaEmCache
}{m: map[string]paginaEmCache{}}

type paginaEmCache struct {
	texto string
	em    time.Time
}

const (
	tamanhoDaFatia   = 15000
	limiteDoDownload = 4 << 20
)

// paginaWeb baixa a URL, converte HTML em markdown (outros tipos de texto vão
// como vieram) e devolve a fatia pedida, dizendo como continuar.
func paginaWeb(req Request) Result {
	texto, err := carregaPagina(req.URL)
	if err != nil {
		return Result{Tool: req.Tool, Status: "error", Error: err.Error()}
	}
	runas := []rune(texto)
	total := len(runas)
	ini := req.Offset
	if ini < 0 {
		ini = 0
	}
	if ini >= total && total > 0 {
		return Result{Tool: req.Tool, Status: "error",
			Error: fmt.Sprintf("offset %d passa do fim da página (%d caracteres)", ini, total)}
	}
	fim := ini + tamanhoDaFatia
	if fim >= total {
		fim = total
	} else {
		// Corta numa quebra de linha próxima, não no meio de uma frase.
		corte := string(runas[ini:fim])
		if i := strings.LastIndex(corte, "\n"); i > tamanhoDaFatia/2 {
			fim = ini + len([]rune(corte[:i]))
		}
	}
	corpo := string(runas[ini:fim])

	var rodape string
	if fim < total {
		rodape = fmt.Sprintf("\n\n[caracteres %d–%d de %d. Para continuar: web_fetch com url igual e offset=%d]", ini, fim, total, fim)
	} else if ini > 0 {
		rodape = fmt.Sprintf("\n\n[fim da página: caracteres %d–%d de %d]", ini, fim, total)
	}
	return Result{Tool: req.Tool, Status: "success", Content: "Conteúdo de " + req.URL + "\n\n" + corpo + rodape}
}

func carregaPagina(alvo string) (string, error) {
	cachePaginas.Lock()
	if p, ok := cachePaginas.m[alvo]; ok && time.Since(p.em) < 5*time.Minute {
		cachePaginas.Unlock()
		return p.texto, nil
	}
	cachePaginas.Unlock()

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	hr, err := http.NewRequestWithContext(ctx, http.MethodGet, alvo, nil)
	if err != nil {
		return "", fmt.Errorf("URL inválida: %v", err)
	}
	hr.Header.Set("User-Agent", "Mozilla/5.0 (compatible; AmazzoBot/1.0)")
	hr.Header.Set("Accept", "text/html,application/xhtml+xml,text/markdown,text/plain,application/json;q=0.9,*/*;q=0.5")
	resp, err := (&http.Client{Timeout: 30 * time.Second}).Do(hr)
	if err != nil {
		return "", fmt.Errorf("erro ao buscar URL: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode > 299 {
		return "", fmt.Errorf("HTTP %d ao abrir %s", resp.StatusCode, alvo)
	}
	bruto, err := io.ReadAll(io.LimitReader(resp.Body, limiteDoDownload))
	if err != nil {
		return "", fmt.Errorf("erro ao ler resposta: %v", err)
	}
	tipo := strings.ToLower(resp.Header.Get("Content-Type"))
	switch {
	case strings.Contains(tipo, "html") || (tipo == "" && strings.Contains(string(bruto[:min(len(bruto), 512)]), "<html")):
		base, _ := url.Parse(alvo)
		texto := htmlParaMarkdown(string(bruto), base)
		if strings.TrimSpace(texto) == "" {
			texto = "(a página não tem texto no HTML; provavelmente é montada por JavaScript, que o Amazzo não executa)"
		}
		return guardaPagina(alvo, texto), nil
	case strings.HasPrefix(tipo, "text/"), strings.Contains(tipo, "json"), strings.Contains(tipo, "xml"), strings.Contains(tipo, "yaml"):
		return guardaPagina(alvo, LimpaSaida(string(bruto))), nil
	}
	return "", fmt.Errorf("conteúdo %q não é texto (%d bytes); web_fetch lê páginas, JSON e texto", tipo, len(bruto))
}

func guardaPagina(alvo, texto string) string {
	cachePaginas.Lock()
	defer cachePaginas.Unlock()
	if len(cachePaginas.m) > 20 {
		cachePaginas.m = map[string]paginaEmCache{}
	}
	cachePaginas.m[alvo] = paginaEmCache{texto: texto, em: time.Now()}
	return texto
}
