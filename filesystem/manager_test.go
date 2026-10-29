package filesystem

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// A mensagem de limite ensinava o caminho errado: sugeria a pasta PAI do
// destino, então o agente abria um nível acima do projeto que você pediu.
func TestErroSugereAPastaPedida(t *testing.T) {
	m, err := NewManager(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}

	projeto := t.TempDir() // uma pasta real, fora do workspace

	_, _, err = m.List(projeto)
	if err == nil {
		t.Fatal("deveria negar caminho fora da pasta")
	}
	if !strings.Contains(err.Error(), "/pasta "+projeto) {
		t.Errorf("deveria sugerir a própria pasta pedida: %v", err)
	}

	// Para um ARQUIVO, o que faz sentido abrir é a pasta que o contém.
	arquivo := filepath.Join(projeto, "main.go")
	if err := os.WriteFile(arquivo, []byte("package main"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := m.ReadFile(arquivo); err == nil {
		t.Fatal("deveria negar arquivo fora da pasta")
	} else if !strings.Contains(err.Error(), "/pasta "+projeto) {
		t.Errorf("para arquivo, deveria sugerir a pasta que o contém: %v", err)
	}
}
