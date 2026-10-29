package agents

import (
	"sync"
	"time"
)

// RateLimiter controla a taxa de requisições por agente.
type RateLimiter struct {
	tokensPerAgent     map[string]int
	lastReset          map[string]time.Time
	maxTokensPerMinute int
	maxTokensPerHour   int
	mu                 sync.RWMutex
}

// NewRateLimiter cria um novo rate limiter.
func NewRateLimiter(maxPerMinute, maxPerHour int) *RateLimiter {
	return &RateLimiter{
		tokensPerAgent:     make(map[string]int),
		lastReset:          make(map[string]time.Time),
		maxTokensPerMinute: maxPerMinute,
		maxTokensPerHour:   maxPerHour,
	}
}

// CanExecute verifica se um agente pode fazer uma requisição.
func (rl *RateLimiter) CanExecute(agentID string, estimatedTokens int) bool {
	rl.mu.Lock()
	defer rl.mu.Unlock()

	now := time.Now()

	// Inicializar se necessário
	if _, exists := rl.lastReset[agentID]; !exists {
		rl.lastReset[agentID] = now
		rl.tokensPerAgent[agentID] = 0
	}

	// Reset se passou 1 minuto
	if now.Sub(rl.lastReset[agentID]) >= time.Minute {
		rl.tokensPerAgent[agentID] = 0
		rl.lastReset[agentID] = now
	}

	// Verificar limite
	if rl.tokensPerAgent[agentID]+estimatedTokens > rl.maxTokensPerMinute {
		return false
	}

	return true
}

// RecordExecution registra uma execução de agente.
func (rl *RateLimiter) RecordExecution(agentID string, tokensUsed int) {
	rl.mu.Lock()
	defer rl.mu.Unlock()

	rl.tokensPerAgent[agentID] += tokensUsed
}

// GetRemaining retorna quantos tokens um agente ainda pode usar.
func (rl *RateLimiter) GetRemaining(agentID string) int {
	rl.mu.RLock()
	defer rl.mu.RUnlock()

	used := rl.tokensPerAgent[agentID]
	remaining := rl.maxTokensPerMinute - used

	if remaining < 0 {
		return 0
	}

	return remaining
}

// GetUsage retorna o uso atual de um agente.
func (rl *RateLimiter) GetUsage(agentID string) (used, limit int) {
	rl.mu.RLock()
	defer rl.mu.RUnlock()

	return rl.tokensPerAgent[agentID], rl.maxTokensPerMinute
}

// GetAllUsage retorna o uso de todos os agentes.
func (rl *RateLimiter) GetAllUsage() map[string]int {
	rl.mu.RLock()
	defer rl.mu.RUnlock()

	usage := make(map[string]int)
	for id, tokens := range rl.tokensPerAgent {
		usage[id] = tokens
	}

	return usage
}

// Reset limpa os contadores de um agente.
func (rl *RateLimiter) Reset(agentID string) {
	rl.mu.Lock()
	defer rl.mu.Unlock()

	rl.tokensPerAgent[agentID] = 0
	rl.lastReset[agentID] = time.Now()
}

// ResetAll limpa os contadores de todos os agentes.
func (rl *RateLimiter) ResetAll() {
	rl.mu.Lock()
	defer rl.mu.Unlock()

	now := time.Now()
	for agentID := range rl.tokensPerAgent {
		rl.tokensPerAgent[agentID] = 0
		rl.lastReset[agentID] = now
	}
}

// WaitTime retorna quanto tempo esperar até poder executar.
func (rl *RateLimiter) WaitTime(agentID string) time.Duration {
	rl.mu.RLock()
	defer rl.mu.RUnlock()

	lastReset, exists := rl.lastReset[agentID]
	if !exists {
		return 0
	}

	elapsed := time.Since(lastReset)
	if elapsed >= time.Minute {
		return 0
	}

	return time.Minute - elapsed
}
