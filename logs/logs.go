package logs

import (
	"fmt"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"amazzo/filesystem"
)

// Saver lida com persistência de mensagens e respostas.
type Saver struct {
	fs *filesystem.Manager
}

// NewSaver cria um novo gerenciador de logs.
func NewSaver(fs *filesystem.Manager) *Saver {
	return &Saver{fs: fs}
}

// SaveConversation grava todo o histórico em um arquivo especificado.
func (s *Saver) SaveConversation(filename string, messages []string) (string, error) {
	if filename == "" {
		filename = fmt.Sprintf("chat_%s.log", time.Now().Format("20060102_150405"))
	}

	path := filepath.Clean(filename)
	if !strings.HasSuffix(path, ".log") && !strings.HasSuffix(path, ".txt") {
		path += ".log"
	}
	if filepath.Dir(path) == "." {
		path = filepath.Join("logs", path)
	}

	content := strings.Join(messages, "\n")
	return s.fs.WriteFile(path, []byte(content), false)
}

// SaveTempResponse salva a última resposta em arquivo temporário.
func (s *Saver) SaveTempResponse(content string) (string, error) {
	if strings.TrimSpace(content) == "" {
		return "", fmt.Errorf("nenhum conteúdo para salvar")
	}

	path := filepath.Join(".cache", "last_response.txt")
	return s.fs.WriteFile(path, []byte(content), false)
}

// stripANSI remove todos os códigos de escape ANSI de uma string.
func stripANSI(text string) string {
	// Regex para remover códigos ANSI (cores, estilos, etc)
	ansiRegex := regexp.MustCompile(`\x1b\[[0-9;]*[mGKHF]`)
	return ansiRegex.ReplaceAllString(text, "")
}

// SaveReadableConversation salva a conversa em formato legível (sem códigos ANSI).
// Esta função é ideal para exportar conversas para o usuário final.
func (s *Saver) SaveReadableConversation(filename string, messages []string) (string, error) {
	if filename == "" {
		filename = fmt.Sprintf("conversa_%s.txt", time.Now().Format("20060102_150405"))
	}

	path := filepath.Clean(filename)
	if !strings.HasSuffix(path, ".txt") && !strings.HasSuffix(path, ".md") {
		path += ".txt"
	}
	if filepath.Dir(path) == "." {
		path = filepath.Join("output", path)
	}

	// Remove códigos ANSI de todas as mensagens
	cleanMessages := make([]string, 0, len(messages))
	for _, msg := range messages {
		clean := stripANSI(msg)
		if strings.TrimSpace(clean) != "" {
			cleanMessages = append(cleanMessages, clean)
		}
	}

	// Adiciona cabeçalho com informações
	header := []string{
		"════════════════════════════════════════════════════════════════",
		"                    CONVERSA COM AMAZZO AGENT",
		fmt.Sprintf("                    Data: %s", time.Now().Format("02/01/2006 15:04:05")),
		"════════════════════════════════════════════════════════════════",
		"",
	}

	content := strings.Join(append(header, cleanMessages...), "\n")
	return s.fs.WriteFile(path, []byte(content), false)
}
