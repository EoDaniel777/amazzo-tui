package credentials

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"path/filepath"
)

// Manager gerencia credenciais criptografadas
type Manager struct {
	credPath string
	key      []byte
}

// NewManager cria um novo gerenciador de credenciais
func NewManager() (*Manager, error) {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return nil, fmt.Errorf("erro ao obter diretório home: %w", err)
	}

	configDir := filepath.Join(homeDir, ".config", "amazzo")
	if err := os.MkdirAll(configDir, 0700); err != nil {
		return nil, fmt.Errorf("erro ao criar diretório de config: %w", err)
	}

	credPath := filepath.Join(configDir, "credentials")

	// Gera chave de criptografia baseada no hostname + username
	// (identificador único da máquina + usuário)
	hostname, _ := os.Hostname()
	username := os.Getenv("USER")
	seed := fmt.Sprintf("%s:%s:amazzo-secret", hostname, username)
	hash := sha256.Sum256([]byte(seed))
	key := hash[:]

	return &Manager{
		credPath: credPath,
		key:      key,
	}, nil
}

// SaveToken salva o token de forma criptografada
func (m *Manager) SaveToken(token string) error {
	block, err := aes.NewCipher(m.key)
	if err != nil {
		return fmt.Errorf("erro ao criar cipher: %w", err)
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return fmt.Errorf("erro ao criar GCM: %w", err)
	}

	nonce := make([]byte, gcm.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return fmt.Errorf("erro ao gerar nonce: %w", err)
	}

	ciphertext := gcm.Seal(nonce, nonce, []byte(token), nil)
	encoded := hex.EncodeToString(ciphertext)

	if err := os.WriteFile(m.credPath, []byte(encoded), 0600); err != nil {
		return fmt.Errorf("erro ao salvar credenciais: %w", err)
	}

	return nil
}

// LoadToken carrega e descriptografa o token
func (m *Manager) LoadToken() (string, error) {
	data, err := os.ReadFile(m.credPath)
	if err != nil {
		if os.IsNotExist(err) {
			return "", nil // Token não existe ainda
		}
		return "", fmt.Errorf("erro ao ler credenciais: %w", err)
	}

	ciphertext, err := hex.DecodeString(string(data))
	if err != nil {
		return "", fmt.Errorf("erro ao decodificar credenciais: %w", err)
	}

	block, err := aes.NewCipher(m.key)
	if err != nil {
		return "", fmt.Errorf("erro ao criar cipher: %w", err)
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", fmt.Errorf("erro ao criar GCM: %w", err)
	}

	nonceSize := gcm.NonceSize()
	if len(ciphertext) < nonceSize {
		return "", fmt.Errorf("ciphertext inválido")
	}

	nonce, ciphertext := ciphertext[:nonceSize], ciphertext[nonceSize:]
	plaintext, err := gcm.Open(nil, nonce, ciphertext, nil)
	if err != nil {
		return "", fmt.Errorf("erro ao descriptografar: %w", err)
	}

	return string(plaintext), nil
}

// DeleteToken remove o token salvo
func (m *Manager) DeleteToken() error {
	if err := os.Remove(m.credPath); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("erro ao deletar credenciais: %w", err)
	}
	return nil
}

// HasToken verifica se existe um token salvo
func (m *Manager) HasToken() bool {
	_, err := os.Stat(m.credPath)
	return err == nil
}
