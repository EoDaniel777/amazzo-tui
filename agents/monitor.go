package agents

import (
	"sync"
	"time"
)

// AgentStatus representa o status de um agente.
type AgentStatus string

const (
	AgentStatusHealthy      AgentStatus = "healthy"
	AgentStatusSlow         AgentStatus = "slow"
	AgentStatusUnresponsive AgentStatus = "unresponsive"
	AgentStatusFailed       AgentStatus = "failed"
)

// AgentMetrics armazena métricas de um agente.
type AgentMetrics struct {
	TasksCompleted int
	TasksFailed    int
	TotalLatency   time.Duration
	AverageLatency time.Duration
	MinLatency     time.Duration
	MaxLatency     time.Duration
	TokensUsed     int
	LastActive     time.Time
	Status         AgentStatus
	ErrorCount     int
}

// AgentMonitor monitora a saúde e performance dos agentes.
type AgentMonitor struct {
	metrics            map[string]*AgentMetrics
	healthCheckInterval time.Duration
	slowThreshold      time.Duration
	unresponsiveTimeout time.Duration
	mu                 sync.RWMutex
}

// NewAgentMonitor cria um novo monitor de agentes.
func NewAgentMonitor(healthCheckInterval, slowThreshold, unresponsiveTimeout time.Duration) *AgentMonitor {
	return &AgentMonitor{
		metrics:             make(map[string]*AgentMetrics),
		healthCheckInterval: healthCheckInterval,
		slowThreshold:       slowThreshold,
		unresponsiveTimeout: unresponsiveTimeout,
	}
}

// RegisterAgent registra um novo agente para monitoramento.
func (am *AgentMonitor) RegisterAgent(agentID string) {
	am.mu.Lock()
	defer am.mu.Unlock()

	am.metrics[agentID] = &AgentMetrics{
		LastActive: time.Now(),
		Status:     AgentStatusHealthy,
	}
}

// RecordTaskStart registra o início de uma tarefa.
func (am *AgentMonitor) RecordTaskStart(agentID string) {
	am.mu.Lock()
	defer am.mu.Unlock()

	if metrics, exists := am.metrics[agentID]; exists {
		metrics.LastActive = time.Now()
	}
}

// RecordTaskComplete registra a conclusão bem-sucedida de uma tarefa.
func (am *AgentMonitor) RecordTaskComplete(agentID string, latency time.Duration, tokensUsed int) {
	am.mu.Lock()
	defer am.mu.Unlock()

	metrics, exists := am.metrics[agentID]
	if !exists {
		return
	}

	metrics.TasksCompleted++
	metrics.TotalLatency += latency
	metrics.TokensUsed += tokensUsed
	metrics.LastActive = time.Now()

	// Atualizar latências
	if metrics.MinLatency == 0 || latency < metrics.MinLatency {
		metrics.MinLatency = latency
	}
	if latency > metrics.MaxLatency {
		metrics.MaxLatency = latency
	}

	// Calcular média
	if metrics.TasksCompleted > 0 {
		metrics.AverageLatency = metrics.TotalLatency / time.Duration(metrics.TasksCompleted)
	}

	// Atualizar status baseado na latência
	am.updateStatus(agentID)
}

// RecordTaskFailed registra uma falha de tarefa.
func (am *AgentMonitor) RecordTaskFailed(agentID string, err error) {
	am.mu.Lock()
	defer am.mu.Unlock()

	metrics, exists := am.metrics[agentID]
	if !exists {
		return
	}

	metrics.TasksFailed++
	metrics.ErrorCount++
	metrics.LastActive = time.Now()

	// Atualizar status
	am.updateStatus(agentID)
}

// updateStatus atualiza o status de saúde do agente.
func (am *AgentMonitor) updateStatus(agentID string) {
	metrics := am.metrics[agentID]

	// Verificar se está unresponsive
	if time.Since(metrics.LastActive) > am.unresponsiveTimeout {
		metrics.Status = AgentStatusUnresponsive
		return
	}

	// Verificar taxa de falha
	totalTasks := metrics.TasksCompleted + metrics.TasksFailed
	if totalTasks > 0 {
		failureRate := float64(metrics.TasksFailed) / float64(totalTasks)
		if failureRate > 0.5 { // Mais de 50% de falha
			metrics.Status = AgentStatusFailed
			return
		}
	}

	// Verificar latência
	if metrics.AverageLatency > am.slowThreshold {
		metrics.Status = AgentStatusSlow
		return
	}

	metrics.Status = AgentStatusHealthy
}

// GetMetrics retorna as métricas de um agente.
func (am *AgentMonitor) GetMetrics(agentID string) *AgentMetrics {
	am.mu.RLock()
	defer am.mu.RUnlock()

	metrics, exists := am.metrics[agentID]
	if !exists {
		return nil
	}

	// Retornar cópia
	copy := *metrics
	return &copy
}

// GetAllMetrics retorna as métricas de todos os agentes.
func (am *AgentMonitor) GetAllMetrics() map[string]*AgentMetrics {
	am.mu.RLock()
	defer am.mu.RUnlock()

	result := make(map[string]*AgentMetrics)
	for id, metrics := range am.metrics {
		copy := *metrics
		result[id] = &copy
	}

	return result
}

// GetHealthStatus retorna o status de saúde de todos os agentes.
func (am *AgentMonitor) GetHealthStatus() map[string]AgentStatus {
	am.mu.RLock()
	defer am.mu.RUnlock()

	// Atualizar status de todos antes de retornar
	for agentID := range am.metrics {
		am.updateStatus(agentID)
	}

	status := make(map[string]AgentStatus)
	for id, metrics := range am.metrics {
		status[id] = metrics.Status
	}

	return status
}

// GetHealthyAgents retorna IDs dos agentes saudáveis.
func (am *AgentMonitor) GetHealthyAgents() []string {
	am.mu.RLock()
	defer am.mu.RUnlock()

	var healthy []string
	for id, metrics := range am.metrics {
		if metrics.Status == AgentStatusHealthy {
			healthy = append(healthy, id)
		}
	}

	return healthy
}

// GetUnhealthyAgents retorna IDs dos agentes com problemas.
func (am *AgentMonitor) GetUnhealthyAgents() []string {
	am.mu.RLock()
	defer am.mu.RUnlock()

	var unhealthy []string
	for id, metrics := range am.metrics {
		if metrics.Status != AgentStatusHealthy {
			unhealthy = append(unhealthy, id)
		}
	}

	return unhealthy
}

// Reset limpa as métricas de um agente.
func (am *AgentMonitor) Reset(agentID string) {
	am.mu.Lock()
	defer am.mu.Unlock()

	if _, exists := am.metrics[agentID]; exists {
		am.metrics[agentID] = &AgentMetrics{
			LastActive: time.Now(),
			Status:     AgentStatusHealthy,
		}
	}
}

// GetSummary retorna um resumo das métricas agregadas.
func (am *AgentMonitor) GetSummary() map[string]interface{} {
	am.mu.RLock()
	defer am.mu.RUnlock()

	totalAgents := len(am.metrics)
	totalCompleted := 0
	totalFailed := 0
	totalTokens := 0
	healthyCount := 0

	for _, metrics := range am.metrics {
		totalCompleted += metrics.TasksCompleted
		totalFailed += metrics.TasksFailed
		totalTokens += metrics.TokensUsed
		if metrics.Status == AgentStatusHealthy {
			healthyCount++
		}
	}

	successRate := 0.0
	totalTasks := totalCompleted + totalFailed
	if totalTasks > 0 {
		successRate = float64(totalCompleted) / float64(totalTasks) * 100
	}

	return map[string]interface{}{
		"total_agents":      totalAgents,
		"healthy_agents":    healthyCount,
		"total_completed":   totalCompleted,
		"total_failed":      totalFailed,
		"success_rate":      successRate,
		"total_tokens_used": totalTokens,
	}
}
