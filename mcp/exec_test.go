package mcp

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"syscall"
	"testing"
	"time"

	"amazzo/filesystem"
)

func servidorExec(t *testing.T) (*Server, string) {
	t.Helper()
	dir := t.TempDir()
	fsm, err := filesystem.NewManager(dir)
	if err != nil {
		t.Fatal(err)
	}
	return NewServer(fsm), dir
}

func TestExecutaBinarioLocalSemPontoBarra(t *testing.T) {
	s, dir := servidorExec(t)
	os.WriteFile(filepath.Join(dir, "meubin"), []byte("#!/bin/sh\necho ola-local\n"), 0o755)

	res := s.executeCommand(Request{Tool: "execute_command", Command: "meubin"})
	if res.Status != "success" || !strings.Contains(res.Content, "ola-local") {
		t.Fatalf("esperava rodar ./meubin: %+v", res)
	}
}

func TestErroTrazStderr(t *testing.T) {
	s, _ := servidorExec(t)
	res := s.executeCommand(Request{Tool: "execute_command", Command: "naoexistexyz"})
	if res.Status != "error" || !strings.Contains(res.Content, "naoexistexyz") {
		t.Fatalf("erro deveria carregar a saída do shell: %+v", res)
	}
}

func TestProgramaLongoVaiParaSegundoPlano(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}
	s, _ := servidorExec(t)
	inicio := time.Now()
	res := s.executeCommand(Request{Tool: "execute_command", Command: "echo iniciou; sleep 60"})
	if res.Status != "success" || !strings.Contains(res.Content, "segundo plano") || !strings.Contains(res.Content, "iniciou") {
		t.Fatalf("esperava segundo plano com a saída parcial: %+v", res)
	}
	if time.Since(inicio) > 30*time.Second {
		t.Fatalf("demorou demais: %s", time.Since(inicio))
	}
}

func TestBackgroundVoltaRapidoEViveForaDoShell(t *testing.T) {
	dir := t.TempDir()
	fs, err := filesystem.NewManager(dir)
	if err != nil {
		t.Fatal(err)
	}
	srv := NewServer(fs)

	inicio := time.Now()
	res, _ := srv.Handle(context.Background(), []Request{{Tool: "execute_command",
		Command: "echo subiu; sleep 60", Background: true}})
	if d := time.Since(inicio); d > 6*time.Second {
		t.Fatalf("background demorou %v", d)
	}
	if len(res) != 1 || res[0].Status != "success" || !strings.Contains(res[0].Content, "segundo plano") ||
		!strings.Contains(res[0].Content, "subiu") {
		t.Fatalf("resultado inesperado: %+v", res)
	}
	// "kill -- -pid" precisa derrubar o shell E o sleep.
	var pid int
	fmt.Sscanf(res[0].Content[strings.Index(res[0].Content, "pid ")+4:], "%d", &pid)
	if pid == 0 {
		t.Fatal("sem pid na resposta")
	}
	_ = syscall.Kill(-pid, syscall.SIGTERM)
	time.Sleep(300 * time.Millisecond)
	if err := syscall.Kill(pid, 0); err == nil {
		t.Error("o processo devia ter morrido com o grupo")
	}
}

func TestContextoCanceladoMataOComando(t *testing.T) {
	dir := t.TempDir()
	fs, _ := filesystem.NewManager(dir)
	srv := NewServer(fs)
	ctx, cancel := context.WithCancel(context.Background())
	go func() { time.Sleep(300 * time.Millisecond); cancel() }()
	inicio := time.Now()
	res, _ := srv.Handle(ctx, []Request{{Tool: "execute_command", Command: "go version && sleep 60"}})
	if time.Since(inicio) > 5*time.Second {
		t.Fatal("o cancelamento não interrompeu o comando")
	}
	if len(res) == 0 || res[0].Status != "error" {
		t.Fatalf("esperava erro de interrupção: %+v", res)
	}
}
