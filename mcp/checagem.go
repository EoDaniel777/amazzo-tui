package mcp

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

// comChecagem confere a sintaxe do arquivo recém-gravado e, se estiver
// quebrado, avisa NO RESULTADO da própria escrita. Visto em sessão real: o
// modelo gravou um .go sem o "package" e só descobriu minutos depois, quando o
// build falhou sem que ele soubesse qual edição tinha causado. Avisar na hora
// custa um gofmt (milissegundos) e aponta a causa.
func (s *Server) comChecagem(req Request, r Result) Result {
	if r.Status != "success" || strings.TrimSpace(req.Path) == "" {
		return r
	}
	abs, err := s.fs.Resolve(req.Path)
	if err != nil {
		return r
	}
	if aviso := checaSintaxe(abs); aviso != "" {
		r.Content += "\n⚠ ATENÇÃO: " + aviso + "\nCorrija agora (read_file para ver o estado atual) antes de seguir."
	}
	return r
}

// checaSintaxe devolve a descrição do erro de sintaxe, ou "" se está ok ou o
// tipo de arquivo não é verificável.
func checaSintaxe(abs string) string {
	switch strings.ToLower(filepath.Ext(abs)) {
	case ".go":
		bin, err := exec.LookPath("gofmt")
		if err != nil {
			return ""
		}
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		var stderr bytes.Buffer
		cmd := exec.CommandContext(ctx, bin, "-e", "-l", abs)
		cmd.Stderr = &stderr
		cmd.Stdout = nil
		if cmd.Run() == nil {
			return ""
		}
		msg := strings.TrimSpace(stderr.String())
		if msg == "" {
			return ""
		}
		linhas := strings.Split(msg, "\n")
		if len(linhas) > 6 {
			linhas = append(linhas[:6], "…")
		}
		return "o arquivo Go ficou com erro de sintaxe e não compila:\n" + strings.Join(linhas, "\n")
	case ".json":
		b, err := os.ReadFile(abs)
		if err != nil || json.Valid(b) {
			return ""
		}
		return "o arquivo JSON ficou inválido."
	}
	return ""
}
