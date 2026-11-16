package mcp

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestEditFileRecusaTrechoRepetido(t *testing.T) {
	s, dir := servidorExec(t)
	os.WriteFile(filepath.Join(dir, "d.txt"), []byte("a\na\nb\n"), 0o644)
	res := s.editFile(Request{Tool: "edit_file", Path: "d.txt", Search: "a", Replace: "x"})
	if res.Status != "error" || !strings.Contains(res.Error, "2 vezes") {
		t.Fatalf("deveria recusar trecho repetido: %+v", res)
	}
	b, _ := os.ReadFile(filepath.Join(dir, "d.txt"))
	if string(b) != "a\na\nb\n" {
		t.Fatalf("arquivo não pode mudar: %q", b)
	}
	// Com contexto suficiente, vira único e funciona.
	res = s.editFile(Request{Tool: "edit_file", Path: "d.txt", Search: "a\nb", Replace: "x\nb"})
	if res.Status != "success" {
		t.Fatalf("trecho único deveria editar: %+v", res)
	}
}

func TestReadFilePorIntervalo(t *testing.T) {
	s, dir := servidorExec(t)
	os.WriteFile(filepath.Join(dir, "l.txt"), []byte("1\n2\n3\n4\n5"), 0o644)
	res := s.readFile(Request{Tool: "read_file", Path: "l.txt", StartLine: 2, EndLine: 3})
	if !strings.Contains(res.Content, "[linhas 2-3 de 5]") || !strings.Contains(res.Content, "2\n3") || strings.Contains(res.Content, "4") {
		t.Fatalf("intervalo errado: %q", res.Content)
	}
	if res := s.readFile(Request{Tool: "read_file", Path: "l.txt", StartLine: 99}); res.Status != "error" {
		t.Fatalf("start_line além do fim deveria dar erro: %+v", res)
	}
}

func TestWriteFileDiferenciaCriadoDeSobrescrito(t *testing.T) {
	s, _ := servidorExec(t)
	r1 := s.writeFile(Request{Tool: "write_file", Path: "n.txt", Content: "x"}, false)
	r2 := s.writeFile(Request{Tool: "write_file", Path: "n.txt", Content: "y"}, false)
	if !strings.Contains(r1.Content, "criado") || !strings.Contains(r2.Content, "sobrescrito") {
		t.Fatalf("mensagens: %q | %q", r1.Content, r2.Content)
	}
}
