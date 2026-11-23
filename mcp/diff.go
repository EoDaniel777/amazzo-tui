package mcp

import (
	"fmt"
	"strings"
)

// contaLinhas conta as linhas de um texto (a última sem "\n" também conta).
func contaLinhas(s string) int {
	if s == "" {
		return 0
	}
	n := strings.Count(s, "\n")
	if !strings.HasSuffix(s, "\n") {
		n++
	}
	return n
}

// trechoNumerado devolve as linhas [de, ate] (1-based) com numeração.
func trechoNumerado(texto string, de, ate int) string {
	linhas := strings.Split(texto, "\n")
	if de < 1 {
		de = 1
	}
	if ate > len(linhas) {
		ate = len(linhas)
	}
	if ate-de > 40 {
		ate = de + 40
	}
	var sb strings.Builder
	for i := de; i <= ate; i++ {
		sb.WriteString(fmt.Sprintf("%4d| %s\n", i, linhas[i-1]))
	}
	return strings.TrimRight(sb.String(), "\n")
}

// diffDeLinhas mostra o que mudou entre dois textos: tira o prefixo e o
// sufixo de linhas iguais e marca o miolo com "-" e "+". Para arquivo novo
// (antigo vazio) são só linhas "+". Passando de max linhas, corta com "…".
func diffDeLinhas(antigo, novo string, max int) string {
	a := splitLinhas(antigo)
	b := splitLinhas(novo)

	ini := 0
	for ini < len(a) && ini < len(b) && a[ini] == b[ini] {
		ini++
	}
	fa, fb := len(a), len(b)
	for fa > ini && fb > ini && a[fa-1] == b[fb-1] {
		fa--
		fb--
	}

	var out []string
	for _, l := range a[ini:fa] {
		out = append(out, "-"+l)
	}
	for _, l := range b[ini:fb] {
		out = append(out, "+"+l)
	}
	if len(out) > max {
		resto := len(out) - max
		out = append(out[:max], fmt.Sprintf("… +%d linhas", resto))
	}
	return strings.Join(out, "\n")
}

func splitLinhas(s string) []string {
	if s == "" {
		return nil
	}
	return strings.Split(strings.TrimSuffix(s, "\n"), "\n")
}
