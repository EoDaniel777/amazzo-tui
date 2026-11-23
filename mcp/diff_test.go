package mcp

import "testing"

func TestDiffDeLinhas(t *testing.T) {
	if got := diffDeLinhas("", "a\nb\n", 10); got != "+a\n+b" {
		t.Fatalf("novo: %q", got)
	}
	if got := diffDeLinhas("a\nb\nc\n", "a\nX\nc\n", 10); got != "-b\n+X" {
		t.Fatalf("edicao: %q", got)
	}
	if got := diffDeLinhas("", "1\n2\n3\n4\n", 2); got != "+1\n+2\n… +2 linhas" {
		t.Fatalf("corte: %q", got)
	}
}
