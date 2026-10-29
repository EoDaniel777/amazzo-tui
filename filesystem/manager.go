package filesystem

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
)

// Manager centraliza operações de arquivo dentro do workspace do agente.
type Manager struct {
	root string
}

// NewManager cria um gerenciador garantindo que o caminho root exista.
func NewManager(root string) (*Manager, error) {
	if root == "" {
		return nil, errors.New("workspace root não pode ser vazio")
	}

	abs, err := filepath.Abs(root)
	if err != nil {
		return nil, fmt.Errorf("não foi possível resolver caminho absoluto: %w", err)
	}

	info, err := os.Stat(abs)
	if err != nil {
		return nil, fmt.Errorf("não foi possível acessar workspace: %w", err)
	}
	if !info.IsDir() {
		return nil, fmt.Errorf("workspace precisa ser diretório: %s", abs)
	}

	return &Manager{root: abs}, nil
}

// Root retorna o diretório raiz seguro.
func (m *Manager) Root() string {
	return m.root
}

// Resolve garante que um caminho esteja dentro do workspace.
func (m *Manager) Resolve(path string) (string, error) {
	if path == "" {
		return "", errors.New("caminho não pode ser vazio")
	}

	clean := filepath.Clean(path)
	if filepath.IsAbs(clean) {
		// Permitir caminhos absolutos desde que fiquem dentro da raiz.
		rel, err := filepath.Rel(m.root, clean)
		if err != nil {
			return "", fmt.Errorf("falha ao resolver caminho: %w", err)
		}
		if strings.HasPrefix(rel, "..") {
			// Sugere a própria pasta pedida, não a pai: sugerir filepath.Dir
			// fazia o agente abrir um nível acima do projeto solicitado.
			sugestao := clean
			if info, err := os.Stat(clean); err != nil || !info.IsDir() {
				sugestao = filepath.Dir(clean)
			}
			return "", fmt.Errorf(
				"%s está fora da pasta de trabalho (%s). Use /pasta %s para trabalhar lá",
				path, m.root, sugestao)
		}
		return clean, nil
	}

	joined := filepath.Join(m.root, clean)
	return m.Resolve(joined)
}

// ReadFile lê um arquivo relativo ao workspace.
func (m *Manager) ReadFile(path string) ([]byte, error) {
	absolute, err := m.Resolve(path)
	if err != nil {
		return nil, err
	}
	return os.ReadFile(absolute)
}

// WriteFile cria/atualiza um arquivo garantindo diretórios intermediários.
func (m *Manager) WriteFile(path string, content []byte, append bool) (string, error) {
	absolute, err := m.Resolve(path)
	if err != nil {
		return "", err
	}

	if err := os.MkdirAll(filepath.Dir(absolute), 0o755); err != nil {
		return "", fmt.Errorf("falha ao criar diretórios: %w", err)
	}

	flag := os.O_CREATE | os.O_WRONLY
	if append {
		flag |= os.O_APPEND
	} else {
		flag |= os.O_TRUNC
	}

	file, err := os.OpenFile(absolute, flag, 0o644)
	if err != nil {
		return "", fmt.Errorf("falha ao abrir arquivo: %w", err)
	}
	defer file.Close()

	if _, err := file.Write(content); err != nil {
		return "", fmt.Errorf("falha ao escrever arquivo: %w", err)
	}

	return absolute, nil
}

// Mkdir garante que o diretório informado exista.
func (m *Manager) Mkdir(path string) (string, error) {
	absolute, err := m.Resolve(path)
	if err != nil {
		return "", err
	}
	if err := os.MkdirAll(absolute, 0o755); err != nil {
		return "", fmt.Errorf("falha ao criar diretório: %w", err)
	}
	return absolute, nil
}

// List retorna entradas de diretório relativas ao workspace.
func (m *Manager) List(path string) ([]fs.DirEntry, string, error) {
	absolute, err := m.Resolve(path)
	if err != nil {
		return nil, "", err
	}

	entries, err := os.ReadDir(absolute)
	if err != nil {
		return nil, "", fmt.Errorf("falha ao listar diretório: %w", err)
	}

	return entries, absolute, nil
}

// DeleteFile remove um arquivo do workspace.
func (m *Manager) DeleteFile(path string) error {
	absolute, err := m.Resolve(path)
	if err != nil {
		return err
	}

	info, err := os.Stat(absolute)
	if err != nil {
		return fmt.Errorf("falha ao verificar arquivo: %w", err)
	}

	if info.IsDir() {
		return fmt.Errorf("use delete_dir para remover diretórios")
	}

	if err := os.Remove(absolute); err != nil {
		return fmt.Errorf("falha ao deletar arquivo: %w", err)
	}

	return nil
}

// EditFile realiza search & replace em um arquivo.
func (m *Manager) EditFile(path, search, replace string) (string, error) {
	absolute, err := m.Resolve(path)
	if err != nil {
		return "", err
	}

	content, err := os.ReadFile(absolute)
	if err != nil {
		return "", fmt.Errorf("falha ao ler arquivo: %w", err)
	}

	originalContent := string(content)
	modifiedContent := strings.ReplaceAll(originalContent, search, replace)

	if originalContent == modifiedContent {
		return "", fmt.Errorf("padrão '%s' não encontrado no arquivo", search)
	}

	if err := os.WriteFile(absolute, []byte(modifiedContent), 0o644); err != nil {
		return "", fmt.Errorf("falha ao escrever arquivo: %w", err)
	}

	occurrences := strings.Count(originalContent, search)
	return fmt.Sprintf("%d ocorrência(s) substituída(s)", occurrences), nil
}

// FileInfo retorna informações sobre um arquivo.
func (m *Manager) FileInfo(path string) (os.FileInfo, string, error) {
	absolute, err := m.Resolve(path)
	if err != nil {
		return nil, "", err
	}

	info, err := os.Stat(absolute)
	if err != nil {
		return nil, "", fmt.Errorf("falha ao obter informações: %w", err)
	}

	return info, absolute, nil
}

// MoveFile move um arquivo de um local para outro dentro do workspace.
func (m *Manager) MoveFile(sourcePath, destPath string) (string, error) {
	// Resolve paths
	absoluteSource, err := m.Resolve(sourcePath)
	if err != nil {
		return "", fmt.Errorf("caminho origem inválido: %w", err)
	}

	absoluteDest, err := m.Resolve(destPath)
	if err != nil {
		return "", fmt.Errorf("caminho destino inválido: %w", err)
	}

	// Verifica se source existe
	sourceInfo, err := os.Stat(absoluteSource)
	if err != nil {
		return "", fmt.Errorf("arquivo origem não encontrado: %w", err)
	}

	// Não permite mover diretórios
	if sourceInfo.IsDir() {
		return "", fmt.Errorf("não é possível mover diretórios, apenas arquivos")
	}

	// Cria diretório destino se necessário
	destDir := filepath.Dir(absoluteDest)
	if err := os.MkdirAll(destDir, 0755); err != nil {
		return "", fmt.Errorf("falha ao criar diretório destino: %w", err)
	}

	// Move o arquivo
	if err := os.Rename(absoluteSource, absoluteDest); err != nil {
		return "", fmt.Errorf("falha ao mover arquivo: %w", err)
	}

	relativeDest, _ := filepath.Rel(m.root, absoluteDest)
	return fmt.Sprintf("Arquivo movido de %s para %s com sucesso", sourcePath, relativeDest), nil
}
