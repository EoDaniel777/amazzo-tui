package app

import (
	"crypto/sha256"
	"encoding/hex"
	"sync"
	"time"
)

// CacheEntry representa uma entrada no cache.
type CacheEntry struct {
	Response  string
	Timestamp time.Time
	Model     string
}

// ResponseCache gerencia o cache de respostas da API.
type ResponseCache struct {
	cache    map[string]CacheEntry
	mu       sync.RWMutex
	maxAge   time.Duration
	maxSize  int
	hits     int
	misses   int
}

// NewResponseCache cria um novo cache de respostas.
func NewResponseCache(maxAge time.Duration, maxSize int) *ResponseCache {
	return &ResponseCache{
		cache:   make(map[string]CacheEntry),
		maxAge:  maxAge,
		maxSize: maxSize,
	}
}

// Get busca uma resposta no cache.
func (c *ResponseCache) Get(prompt, model string) (string, bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	key := c.generateKey(prompt, model)
	entry, exists := c.cache[key]

	if !exists {
		c.misses++
		return "", false
	}

	// Verificar se o cache expirou
	if time.Since(entry.Timestamp) > c.maxAge {
		c.misses++
		return "", false
	}

	c.hits++
	return entry.Response, true
}

// Set armazena uma resposta no cache.
func (c *ResponseCache) Set(prompt, model, response string) {
	c.mu.Lock()
	defer c.mu.Unlock()

	// Se o cache está cheio, remover entradas antigas
	if len(c.cache) >= c.maxSize {
		c.evictOldest()
	}

	key := c.generateKey(prompt, model)
	c.cache[key] = CacheEntry{
		Response:  response,
		Timestamp: time.Now(),
		Model:     model,
	}
}

// Clear limpa todo o cache.
func (c *ResponseCache) Clear() {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.cache = make(map[string]CacheEntry)
	c.hits = 0
	c.misses = 0
}

// Stats retorna estatísticas do cache.
func (c *ResponseCache) Stats() (hits, misses int, hitRate float64) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	total := c.hits + c.misses
	if total > 0 {
		hitRate = float64(c.hits) / float64(total) * 100
	}

	return c.hits, c.misses, hitRate
}

// generateKey gera uma chave única para o cache.
func (c *ResponseCache) generateKey(prompt, model string) string {
	hash := sha256.Sum256([]byte(prompt + model))
	return hex.EncodeToString(hash[:])
}

// evictOldest remove as entradas mais antigas do cache.
func (c *ResponseCache) evictOldest() {
	var oldestKey string
	var oldestTime time.Time

	for key, entry := range c.cache {
		if oldestTime.IsZero() || entry.Timestamp.Before(oldestTime) {
			oldestTime = entry.Timestamp
			oldestKey = key
		}
	}

	if oldestKey != "" {
		delete(c.cache, oldestKey)
	}
}
