package app

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
)

func TestDetectaComoRodarOProjeto(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "go.mod"), []byte("module x\n"), 0o644)
	os.MkdirAll(filepath.Join(dir, "cmd", "app"), 0o755)
	os.WriteFile(filepath.Join(dir, "cmd", "app", "main.go"), []byte("package main\n"), 0o644)
	os.WriteFile(filepath.Join(dir, "Makefile"), []byte("build:\n\tgo build\nrun:\n\tgo run .\n"), 0o644)
	os.WriteFile(filepath.Join(dir, "meubin"), []byte(strings.Repeat("x", 200*1024)), 0o755)
	os.WriteFile(filepath.Join(dir, "script.sh"), []byte("#!/bin/sh\n"), 0o755)

	var cmds []string
	for _, c := range detectRunCommands(dir) {
		cmds = append(cmds, c.Cmd)
	}
	joined := strings.Join(cmds, "|")
	for _, quero := range []string{"./meubin", "go run ./cmd/app", "make run"} {
		if !strings.Contains(joined, quero) {
			t.Errorf("faltou %q em %v", quero, cmds)
		}
	}
	if strings.Contains(joined, "script.sh") {
		t.Errorf("script pequeno não é binário: %v", cmds)
	}
}

func TestBangRodaComandoSemModelo(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "oi"), []byte("#!/bin/sh\necho ola-do-projeto\n"), 0o755)
	m, err := New("", dir, "")
	if err != nil {
		t.Fatal(err)
	}
	m.Update(tea.WindowSizeMsg{Width: 140, Height: 36})
	m.showSplash = false
	digita(m, "!oi")
	_, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	if cmd == nil || !m.loading {
		t.Fatal("!oi deveria disparar a execução")
	}
	// Executa o tea.Cmd (pode vir dentro de um Batch).
	var done *shellDoneMsg
	var walk func(c tea.Cmd)
	walk = func(c tea.Cmd) {
		if c == nil || done != nil {
			return
		}
		switch r := c().(type) {
		case shellDoneMsg:
			done = &r
		case tea.BatchMsg:
			for _, x := range r {
				walk(x)
			}
		}
	}
	walk(cmd)
	if done == nil {
		t.Fatal("não recebi shellDoneMsg")
	}
	m.Update(*done)
	if m.loading {
		t.Fatal("deveria terminar")
	}
	if !strings.Contains(m.ide.lastOutput, "ola-do-projeto") || m.ide.lastExit != "0" {
		t.Fatalf("painel: saída=%q exit=%q", m.ide.lastOutput, m.ide.lastExit)
	}
}
