package app

import (
	"regexp"
	"strings"
	"unicode"
	"unicode/utf8"

	xansi "github.com/charmbracelet/x/ansi"
)

// Texto que vem de fora (saída de comando, resposta de servidor MCP, texto do
// modelo) pode trazer o que quebra a tela: tabs (largura que o terminal e o
// lipgloss calculam diferente), \r de barra de progresso, sequências ANSI de
// cor/cursor do próprio comando, caracteres de controle e UTF-8 inválido.
// Tudo passa por aqui antes de ir para a tela.

var ansiRe = regexp.MustCompile(`\x1b\[[0-9;?]*[ -/]*[@-~]|\x1b\][^\x07\x1b]*(\x07|\x1b\\)|\x1b[@-Z\\-_]`)

// limpaTexto devolve texto seguro para desenhar: sem ANSI, sem controle, com
// tabs expandidos e \r resolvido (vale o último trecho da linha, como no
// terminal).
func limpaTexto(s string) string {
	if s == "" {
		return s
	}
	if !utf8.ValidString(s) {
		s = strings.ToValidUTF8(s, "�")
	}
	s = ansiRe.ReplaceAllString(s, "")
	s = strings.ReplaceAll(s, "\r\n", "\n")

	linhas := strings.Split(s, "\n")
	for i, l := range linhas {
		if j := strings.LastIndexByte(l, '\r'); j >= 0 {
			if resto := l[j+1:]; resto != "" {
				l = resto
			} else {
				l = strings.TrimRight(l, "\r")
				if k := strings.LastIndexByte(l, '\r'); k >= 0 {
					l = l[k+1:]
				}
			}
		}
		linhas[i] = expandeTabs(strings.Map(func(r rune) rune {
			switch {
			case r == '\t':
				return r
			case r == 0x7f, r < 0x20, r >= 0x80 && r < 0xa0:
				return -1
			case r == 0x200b, r == 0x200e, r == 0x200f, r == 0x2028, r == 0x2029, r == 0xfeff:
				return -1
			case unicode.Is(unicode.Cf, r) && r != 0x200d:
				return -1
			}
			return r
		}, l), 4)
	}
	return strings.Join(linhas, "\n")
}

// expandeTabs troca cada tab pelo espaço até a próxima coluna múltipla de n.
func expandeTabs(l string, n int) string {
	if !strings.Contains(l, "\t") {
		return l
	}
	var b strings.Builder
	col := 0
	for _, r := range l {
		if r == '\t' {
			p := n - col%n
			b.WriteString(strings.Repeat(" ", p))
			col += p
			continue
		}
		b.WriteRune(r)
		col += xansi.StringWidth(string(r))
	}
	return b.String()
}

// cortaLinhas limita cada linha a `largura` colunas (com reticências). Cortar
// o bloco inteiro de uma vez só afetava a primeira linha: as outras vazavam
// pela borda da caixa.
func cortaLinhas(linhas []string, largura int) []string {
	out := make([]string, len(linhas))
	for i, l := range linhas {
		out[i] = xansi.Truncate(l, largura, "…")
	}
	return out
}
