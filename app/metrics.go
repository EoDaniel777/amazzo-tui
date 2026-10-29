package app

import (
	"sync"
	"time"
)

// Metrics armazena métricas de performance da aplicação.
type Metrics struct {
	TotalRequests      int
	SuccessfulRequests int
	FailedRequests     int
	TotalLatency       time.Duration
	MinLatency         time.Duration
	MaxLatency         time.Duration
	TotalTokens        int
	CacheHits          int
	CacheMisses        int
	mu                 sync.RWMutex
}

// NewMetrics cria uma nova instância de métricas.
func NewMetrics() *Metrics {
	return &Metrics{}
}

// RecordRequest registra uma requisição.
func (m *Metrics) RecordRequest(latency time.Duration, success bool, tokens int) {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.TotalRequests++
	if success {
		m.SuccessfulRequests++
	} else {
		m.FailedRequests++
	}

	m.TotalLatency += latency
	m.TotalTokens += tokens

	if m.MinLatency == 0 || latency < m.MinLatency {
		m.MinLatency = latency
	}

	if latency > m.MaxLatency {
		m.MaxLatency = latency
	}
}

// RecordCacheHit registra um hit no cache.
func (m *Metrics) RecordCacheHit() {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.CacheHits++
}

// RecordCacheMiss registra um miss no cache.
func (m *Metrics) RecordCacheMiss() {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.CacheMisses++
}

// GetStats retorna estatísticas consolidadas.
func (m *Metrics) GetStats() MetricsStats {
	m.mu.RLock()
	defer m.mu.RUnlock()

	var avgLatency time.Duration
	if m.SuccessfulRequests > 0 {
		avgLatency = m.TotalLatency / time.Duration(m.SuccessfulRequests)
	}

	var successRate float64
	if m.TotalRequests > 0 {
		successRate = float64(m.SuccessfulRequests) / float64(m.TotalRequests) * 100
	}

	var cacheHitRate float64
	totalCacheAccess := m.CacheHits + m.CacheMisses
	if totalCacheAccess > 0 {
		cacheHitRate = float64(m.CacheHits) / float64(totalCacheAccess) * 100
	}

	var tokensPerSecond float64
	if m.TotalLatency > 0 {
		tokensPerSecond = float64(m.TotalTokens) / m.TotalLatency.Seconds()
	}

	return MetricsStats{
		TotalRequests:      m.TotalRequests,
		SuccessfulRequests: m.SuccessfulRequests,
		FailedRequests:     m.FailedRequests,
		AverageLatency:     avgLatency,
		MinLatency:         m.MinLatency,
		MaxLatency:         m.MaxLatency,
		SuccessRate:        successRate,
		TotalTokens:        m.TotalTokens,
		TokensPerSecond:    tokensPerSecond,
		CacheHitRate:       cacheHitRate,
	}
}

// MetricsStats representa estatísticas consolidadas.
type MetricsStats struct {
	TotalRequests      int
	SuccessfulRequests int
	FailedRequests     int
	AverageLatency     time.Duration
	MinLatency         time.Duration
	MaxLatency         time.Duration
	SuccessRate        float64
	TotalTokens        int
	TokensPerSecond    float64
	CacheHitRate       float64
}

// Reset limpa todas as métricas.
func (m *Metrics) Reset() {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.TotalRequests = 0
	m.SuccessfulRequests = 0
	m.FailedRequests = 0
	m.TotalLatency = 0
	m.MinLatency = 0
	m.MaxLatency = 0
	m.TotalTokens = 0
	m.CacheHits = 0
	m.CacheMisses = 0
}
