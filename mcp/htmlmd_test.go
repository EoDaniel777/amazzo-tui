package mcp

import (
	"net/url"
	"strings"
	"testing"
)

func TestHtmlParaMarkdown(t *testing.T) {
	pagina := `<html><head><title>x</title><style>.a{}</style></head><body>
<nav><a href="/menu">Menu</a></nav>
<main><h1>Título</h1><p>Texto com <strong>negrito</strong>, <em>ênfase</em> e <a href="/doc/x">link</a> e <code>go build</code>.</p>
<ul><li>um</li><li>dois<ul><li>filho</li></ul></li></ul><ol><li>a</li><li>b</li></ol>
<pre><code>func main() {
	println("oi")
}</code></pre>
<table><tr><th>Nome</th><th>Valor</th></tr><tr><td>a</td><td>1</td></tr></table>
<blockquote>citado</blockquote><script>alert(1)</script></main><footer>rodapé</footer></body></html>`
	base, _ := url.Parse("https://ex.com/a/b")
	md := htmlParaMarkdown(pagina, base)
	for _, quer := range []string{
		"# Título", "**negrito**", "*ênfase*", "[link](https://ex.com/doc/x)", "`go build`",
		"- um", "  - filho", "1. a", "2. b", "```\nfunc main() {", "| Nome | Valor |", "| --- | --- |", "| a | 1 |", "> citado",
	} {
		if !strings.Contains(md, quer) {
			t.Errorf("faltou %q em:\n%s", quer, md)
		}
	}
	for _, nao := range []string{"Menu", "rodapé", "alert", "<"} {
		if strings.Contains(md, nao) {
			t.Errorf("não devia conter %q em:\n%s", nao, md)
		}
	}
}

func TestPaginaWebEmFatias(t *testing.T) {
	grande := strings.Repeat("linha de texto numero um\n", 2000) // ~50 mil
	guardaPagina("http://teste/grande", grande)
	r := paginaWeb(Request{Tool: "web_fetch", URL: "http://teste/grande"})
	if r.Status != "success" || !strings.Contains(r.Content, "offset=") {
		t.Fatalf("sem indicação de continuação: %.200s", r.Content)
	}
	var off int
	i := strings.Index(r.Content, "offset=")
	for _, c := range r.Content[i+7:] {
		if c < '0' || c > '9' {
			break
		}
		off = off*10 + int(c-'0')
	}
	r2 := paginaWeb(Request{Tool: "web_fetch", URL: "http://teste/grande", Offset: off})
	if r2.Status != "success" || strings.Contains(r2.Content, "Conteúdo de http://teste/grande\n\nlinha de texto numero um\n[") {
		t.Fatalf("segunda fatia ruim")
	}
	fim := paginaWeb(Request{Tool: "web_fetch", URL: "http://teste/grande", Offset: len(grande) + 5})
	if fim.Status != "error" {
		t.Fatal("offset além do fim devia dar erro")
	}
	ult := paginaWeb(Request{Tool: "web_fetch", URL: "http://teste/grande", Offset: len(grande) - 100})
	if !strings.Contains(ult.Content, "fim da página") {
		t.Fatalf("última fatia sem aviso de fim: %s", ult.Content)
	}
}
