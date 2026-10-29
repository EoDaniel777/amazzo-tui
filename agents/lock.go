package agents

import (
	"fmt"
	"sync"
	"time"
)

// FileLock representa um lock em um arquivo específico.
type FileLock struct {
	FilePath  string
	AgentID   string
	LockedAt  time.Time
	ExpiresAt time.Time
}

// LockManager gerencia locks de arquivos para evitar conflitos entre agentes.
type LockManager struct {
	locks      map[string]*FileLock
	mu         sync.RWMutex
	defaultTTL time.Duration
}

// NewLockManager cria um novo gerenciador de locks.
func NewLockManager() *LockManager {
	return &LockManager{
		locks:      make(map[string]*FileLock),
		defaultTTL: 5 * time.Minute,
	}
}

// Lock tenta adquirir um lock em um arquivo.
func (lm *LockManager) Lock(filePath, agentID string) error {
	lm.mu.Lock()
	defer lm.mu.Unlock()

	// Limpa locks expirados
	lm.cleanExpiredLocks()

	// Verifica se já existe lock
	if lock, exists := lm.locks[filePath]; exists {
		if lock.AgentID == agentID {
			// Mesmo agente, renova o lock
			lock.ExpiresAt = time.Now().Add(lm.defaultTTL)
			return nil
		}
		return fmt.Errorf("arquivo %s está locked pelo agente %s até %s",
			filePath, lock.AgentID, lock.ExpiresAt.Format("15:04:05"))
	}

	// Cria novo lock
	lm.locks[filePath] = &FileLock{
		FilePath:  filePath,
		AgentID:   agentID,
		LockedAt:  time.Now(),
		ExpiresAt: time.Now().Add(lm.defaultTTL),
	}

	return nil
}

// Unlock libera um lock.
func (lm *LockManager) Unlock(filePath, agentID string) error {
	lm.mu.Lock()
	defer lm.mu.Unlock()

	lock, exists := lm.locks[filePath]
	if !exists {
		return fmt.Errorf("nenhum lock encontrado para %s", filePath)
	}

	if lock.AgentID != agentID {
		return fmt.Errorf("apenas o agente %s pode desbloquear %s", lock.AgentID, filePath)
	}

	delete(lm.locks, filePath)
	return nil
}

// IsLocked verifica se um arquivo está locked.
func (lm *LockManager) IsLocked(filePath string) bool {
	lm.mu.RLock()
	defer lm.mu.RUnlock()

	lm.cleanExpiredLocks()

	_, exists := lm.locks[filePath]
	return exists
}

// GetLockInfo retorna informações sobre o lock de um arquivo.
func (lm *LockManager) GetLockInfo(filePath string) (*FileLock, error) {
	lm.mu.RLock()
	defer lm.mu.RUnlock()

	lock, exists := lm.locks[filePath]
	if !exists {
		return nil, fmt.Errorf("arquivo %s não está locked", filePath)
	}

	return lock, nil
}

// cleanExpiredLocks remove locks que expiraram.
func (lm *LockManager) cleanExpiredLocks() {
	now := time.Now()
	for path, lock := range lm.locks {
		if now.After(lock.ExpiresAt) {
			delete(lm.locks, path)
		}
	}
}

// WaitForUnlock aguarda até que um arquivo seja desbloqueado.
func (lm *LockManager) WaitForUnlock(filePath string, timeout time.Duration) error {
	deadline := time.Now().Add(timeout)

	for {
		if !lm.IsLocked(filePath) {
			return nil
		}

		if time.Now().After(deadline) {
			lock, _ := lm.GetLockInfo(filePath)
			if lock != nil {
				return fmt.Errorf("timeout esperando unlock de %s (locked por %s)",
					filePath, lock.AgentID)
			}
			return fmt.Errorf("timeout esperando unlock de %s", filePath)
		}

		time.Sleep(500 * time.Millisecond)
	}
}

// GetAllLocks retorna todos os locks ativos.
func (lm *LockManager) GetAllLocks() map[string]*FileLock {
	lm.mu.RLock()
	defer lm.mu.RUnlock()

	lm.cleanExpiredLocks()

	// Cria cópia para evitar race conditions
	result := make(map[string]*FileLock)
	for k, v := range lm.locks {
		lockCopy := *v
		result[k] = &lockCopy
	}

	return result
}

// ClearAll remove todos os locks (use com cuidado).
func (lm *LockManager) ClearAll() {
	lm.mu.Lock()
	defer lm.mu.Unlock()

	lm.locks = make(map[string]*FileLock)
}
