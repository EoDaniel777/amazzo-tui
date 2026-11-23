package mcp

import "testing"

func TestParseDDGPulaAnuncioEDesembrulhaLink(t *testing.T) {
	pg := `<a rel="nofollow" class="result__a" href="https://duckduckgo.com/y.js?ad_domain=x.com">Anuncio</a>
<a class="result__snippet" href="#">patrocinado</a>
<a rel="nofollow" class="result__a" href="//duckduckgo.com/l/?uddg=https%3A%2F%2Fgithub.com%2Fcharmbracelet%2Fbubbletea&amp;rut=1">Bubble <b>Tea</b></a>
<a class="result__snippet" href="#">Um framework &amp; TUI</a>`
	r := parseDDG(pg, 5)
	if len(r) != 1 {
		t.Fatalf("esperava 1 resultado orgânico, veio %d: %v", len(r), r)
	}
	if r[0][0] != "Bubble Tea" || r[0][1] != "https://github.com/charmbracelet/bubbletea" || r[0][2] != "Um framework & TUI" {
		t.Fatalf("%v", r[0])
	}
}

func TestUrlDeConteudoGithub(t *testing.T) {
	got := urlDeConteudo("https://github.com/a/b/blob/main/x/y.go")
	if got != "https://raw.githubusercontent.com/a/b/main/x/y.go" {
		t.Fatal(got)
	}
	if urlDeConteudo("https://exemplo.com/blob/x") != "https://exemplo.com/blob/x" {
		t.Fatal("alterou URL comum")
	}
}
