package mcp

import (
	"net/url"
	"regexp"
	"strconv"
	"strings"

	"golang.org/x/net/html"
	"golang.org/x/net/html/atom"
)

// htmlParaMarkdown converte uma página HTML em markdown legível: títulos,
// parágrafos, listas, links (absolutos), código, citações e tabelas. Script,
// estilo e navegação ficam de fora. Se a página tem <main> ou <article>, só
// ele é convertido (o menu e o rodapé do site não interessam ao modelo).
func htmlParaMarkdown(fonte string, base *url.URL) string {
	doc, err := html.Parse(strings.NewReader(fonte))
	if err != nil {
		return sanitizeHTML(fonte)
	}
	raiz := doc
	for _, alvo := range []atom.Atom{atom.Main, atom.Article} {
		if n := achaTag(doc, alvo); n != nil {
			raiz = n
			break
		}
	}
	c := &conversor{base: base}
	c.visita(raiz)
	return c.resultado()
}

func achaTag(n *html.Node, a atom.Atom) *html.Node {
	if n.Type == html.ElementNode && n.DataAtom == a {
		return n
	}
	for ch := n.FirstChild; ch != nil; ch = ch.NextSibling {
		if r := achaTag(ch, a); r != nil {
			return r
		}
	}
	return nil
}

type conversor struct {
	sb      strings.Builder
	base    *url.URL
	emPre   bool
	listas  []*itemLista
	citacao int
}

type itemLista struct {
	ordenada bool
	n        int
}

var espacos = regexp.MustCompile(`[ \t\r\n\f]+`)

// bloco garante uma linha em branco antes do próximo conteúdo.
func (c *conversor) bloco() {
	s := c.sb.String()
	if s == "" {
		return
	}
	s = strings.TrimRight(s, " \t")
	c.sb.Reset()
	c.sb.WriteString(s)
	for !strings.HasSuffix(c.sb.String(), "\n\n") {
		c.sb.WriteString("\n")
	}
	c.prefixoDeCitacao()
}

func (c *conversor) linha() {
	s := strings.TrimRight(c.sb.String(), " \t")
	c.sb.Reset()
	c.sb.WriteString(s)
	if !strings.HasSuffix(s, "\n") {
		c.sb.WriteString("\n")
	}
	c.prefixoDeCitacao()
}

func (c *conversor) prefixoDeCitacao() {
	if c.citacao > 0 {
		c.sb.WriteString(strings.Repeat("> ", c.citacao))
	}
}

func (c *conversor) texto(n *html.Node) string {
	var sb strings.Builder
	var f func(*html.Node)
	f = func(x *html.Node) {
		if x.Type == html.TextNode {
			sb.WriteString(x.Data)
		}
		for ch := x.FirstChild; ch != nil; ch = ch.NextSibling {
			f(ch)
		}
	}
	f(n)
	return strings.TrimSpace(espacos.ReplaceAllString(sb.String(), " "))
}

func (c *conversor) absoluta(href string) string {
	href = strings.TrimSpace(href)
	if href == "" || strings.HasPrefix(href, "#") || strings.HasPrefix(strings.ToLower(href), "javascript:") {
		return ""
	}
	if c.base != nil {
		if u, err := c.base.Parse(href); err == nil {
			return u.String()
		}
	}
	return href
}

func (c *conversor) filhos(n *html.Node) {
	for ch := n.FirstChild; ch != nil; ch = ch.NextSibling {
		c.visita(ch)
	}
}

func (c *conversor) visita(n *html.Node) {
	switch n.Type {
	case html.TextNode:
		if c.emPre {
			c.sb.WriteString(n.Data)
			return
		}
		t := espacos.ReplaceAllString(n.Data, " ")
		cur := c.sb.String()
		if strings.TrimSpace(t) == "" {
			if cur != "" && !strings.HasSuffix(cur, " ") && !strings.HasSuffix(cur, "\n") && !strings.HasSuffix(cur, "> ") {
				c.sb.WriteString(" ")
			}
			return
		}
		if strings.HasSuffix(cur, " ") || strings.HasSuffix(cur, "\n") || cur == "" {
			t = strings.TrimLeft(t, " ")
		}
		c.sb.WriteString(t)
		return
	case html.DocumentNode:
		c.filhos(n)
		return
	case html.ElementNode:
	default:
		return
	}

	switch n.DataAtom {
	case atom.Script, atom.Style, atom.Noscript, atom.Svg, atom.Iframe, atom.Template,
		atom.Head, atom.Nav, atom.Footer, atom.Form, atom.Button, atom.Select, atom.Canvas:
		return
	case atom.H1, atom.H2, atom.H3, atom.H4, atom.H5, atom.H6:
		nivel := int(n.Data[1] - '0')
		t := c.texto(n)
		if t == "" {
			return
		}
		c.bloco()
		c.sb.WriteString(strings.Repeat("#", nivel) + " " + t)
		c.bloco()
	case atom.P, atom.Div, atom.Section, atom.Article, atom.Main, atom.Header, atom.Aside, atom.Figure, atom.Figcaption, atom.Dl, atom.Details, atom.Summary:
		c.bloco()
		c.filhos(n)
		c.bloco()
	case atom.Br:
		c.linha()
	case atom.Hr:
		c.bloco()
		c.sb.WriteString("---")
		c.bloco()
	case atom.Ul, atom.Ol:
		c.bloco()
		c.listas = append(c.listas, &itemLista{ordenada: n.DataAtom == atom.Ol})
		c.filhos(n)
		c.listas = c.listas[:len(c.listas)-1]
		c.bloco()
	case atom.Li:
		c.linha()
		prof := len(c.listas) - 1
		marca := "- "
		if prof >= 0 {
			if l := c.listas[prof]; l.ordenada {
				l.n++
				marca = strconv.Itoa(l.n) + ". "
			}
		} else {
			prof = 0
		}
		c.sb.WriteString(strings.Repeat("  ", prof) + marca)
		c.filhos(n)
		c.linha()
	case atom.Dt, atom.Dd:
		c.linha()
		c.filhos(n)
		c.linha()
	case atom.Pre:
		c.bloco()
		c.sb.WriteString("```\n")
		ant := c.emPre
		c.emPre = true
		c.filhos(n)
		c.emPre = ant
		s := strings.TrimRight(c.sb.String(), "\n")
		c.sb.Reset()
		c.sb.WriteString(s + "\n```")
		c.bloco()
	case atom.Code, atom.Kbd, atom.Samp, atom.Tt:
		if c.emPre {
			c.filhos(n)
			return
		}
		t := c.texto(n)
		if t != "" {
			c.sb.WriteString("`" + t + "`")
		}
	case atom.Strong, atom.B:
		t := c.texto(n)
		if t != "" {
			c.sb.WriteString("**" + t + "**")
		}
	case atom.Em, atom.I:
		t := c.texto(n)
		if t != "" {
			c.sb.WriteString("*" + t + "*")
		}
	case atom.A:
		t := c.texto(n)
		href := ""
		for _, a := range n.Attr {
			if a.Key == "href" {
				href = c.absoluta(a.Val)
			}
		}
		switch {
		case t == "":
		case href == "":
			c.sb.WriteString(t)
		default:
			c.sb.WriteString("[" + t + "](" + href + ")")
		}
	case atom.Img:
		for _, a := range n.Attr {
			if a.Key == "alt" && strings.TrimSpace(a.Val) != "" {
				c.sb.WriteString("[imagem: " + strings.TrimSpace(a.Val) + "]")
			}
		}
	case atom.Blockquote:
		c.bloco()
		c.citacao++
		c.sb.WriteString("> ")
		c.filhos(n)
		c.citacao--
		c.bloco()
	case atom.Table:
		c.bloco()
		c.tabela(n)
		c.bloco()
	default:
		c.filhos(n)
	}
}

// tabela escreve linhas "| a | b |", com a linha de separação após o cabeçalho.
func (c *conversor) tabela(t *html.Node) {
	var linhas [][]string
	var f func(*html.Node)
	f = func(n *html.Node) {
		if n.Type == html.ElementNode && n.DataAtom == atom.Tr {
			var cel []string
			for ch := n.FirstChild; ch != nil; ch = ch.NextSibling {
				if ch.Type == html.ElementNode && (ch.DataAtom == atom.Td || ch.DataAtom == atom.Th) {
					cel = append(cel, strings.ReplaceAll(c.texto(ch), "|", "\\|"))
				}
			}
			if len(cel) > 0 {
				linhas = append(linhas, cel)
			}
			return
		}
		for ch := n.FirstChild; ch != nil; ch = ch.NextSibling {
			f(ch)
		}
	}
	f(t)
	for i, l := range linhas {
		c.sb.WriteString("| " + strings.Join(l, " | ") + " |\n")
		if i == 0 && len(linhas) > 1 {
			sep := make([]string, len(l))
			for j := range sep {
				sep[j] = "---"
			}
			c.sb.WriteString("| " + strings.Join(sep, " | ") + " |\n")
		}
	}
}

var linhasEmBranco = regexp.MustCompile(`\n{3,}`)

func (c *conversor) resultado() string {
	s := c.sb.String()
	linhas := strings.Split(s, "\n")
	for i, l := range linhas {
		linhas[i] = strings.TrimRight(l, " \t")
	}
	s = strings.Join(linhas, "\n")
	s = linhasEmBranco.ReplaceAllString(s, "\n\n")
	return strings.TrimSpace(s)
}
