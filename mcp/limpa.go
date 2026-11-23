package mcp

import (
	"regexp"
	"strings"
)

var ansiSaida = regexp.MustCompile(`\x1b\[[0-9;?]*[ -/]*[@-~]|\x1b\][^\x07\x1b]*(\x07|\x1b\\)|\x1b[@-Z\\-_]`)

// LimpaSaida tira da saída de um comando o que só serve a um terminal: cores e
// movimentos de cursor (ANSI) e barras de progresso que reescrevem a linha com
// \r. O modelo lia "\x1b[32m" como texto e gastava tokens com isso.
func LimpaSaida(s string) string {
	s = ansiSaida.ReplaceAllString(s, "")
	s = strings.ReplaceAll(s, "\r\n", "\n")
	if !strings.Contains(s, "\r") {
		return strings.ToValidUTF8(s, "�")
	}
	linhas := strings.Split(s, "\n")
	for i, l := range linhas {
		l = strings.TrimRight(l, "\r")
		if j := strings.LastIndexByte(l, '\r'); j >= 0 {
			l = l[j+1:]
		}
		linhas[i] = l
	}
	return strings.ToValidUTF8(strings.Join(linhas, "\n"), "�")
}
