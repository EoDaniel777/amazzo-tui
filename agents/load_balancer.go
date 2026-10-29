package agents

import (
	"sync"
)

// LoadBalancer distribui tarefas entre agentes de forma equilibrada.
type LoadBalancer struct {
	agentLoads      map[string]int            // agentID -> número de tarefas ativas
	agentCapabilities map[string][]string     // agentID -> tipos de tarefa que pode executar
	mu              sync.RWMutex
}

// NewLoadBalancer cria um novo load balancer.
func NewLoadBalancer() *LoadBalancer {
	return &LoadBalancer{
		agentLoads:        make(map[string]int),
		agentCapabilities: make(map[string][]string),
	}
}

// RegisterAgent registra um agente no load balancer.
func (lb *LoadBalancer) RegisterAgent(agentID string, capabilities []string) {
	lb.mu.Lock()
	defer lb.mu.Unlock()

	lb.agentLoads[agentID] = 0
	lb.agentCapabilities[agentID] = capabilities
}

// UnregisterAgent remove um agente do load balancer.
func (lb *LoadBalancer) UnregisterAgent(agentID string) {
	lb.mu.Lock()
	defer lb.mu.Unlock()

	delete(lb.agentLoads, agentID)
	delete(lb.agentCapabilities, agentID)
}

// AssignTask escolhe o melhor agente para uma tarefa.
func (lb *LoadBalancer) AssignTask(task Task) string {
	lb.mu.RLock()
	defer lb.mu.RUnlock()

	var bestAgent string
	minLoad := int(^uint(0) >> 1) // Max int

	// Filtrar agentes capazes
	capableAgents := lb.getCapableAgents(string(task.Type))

	// Escolher o com menos carga
	for _, agentID := range capableAgents {
		if load := lb.agentLoads[agentID]; load < minLoad {
			minLoad = load
			bestAgent = agentID
		}
	}

	return bestAgent
}

// getCapableAgents retorna agentes que podem executar um tipo de tarefa.
func (lb *LoadBalancer) getCapableAgents(taskType string) []string {
	var capable []string

	for agentID, capabilities := range lb.agentCapabilities {
		// Se não tem capabilities definidas, pode executar qualquer tarefa
		if len(capabilities) == 0 {
			capable = append(capable, agentID)
			continue
		}

		// Verificar se tem a capability necessária
		for _, cap := range capabilities {
			if cap == taskType || cap == "*" {
				capable = append(capable, agentID)
				break
			}
		}
	}

	return capable
}

// IncrementLoad incrementa a carga de um agente.
func (lb *LoadBalancer) IncrementLoad(agentID string) {
	lb.mu.Lock()
	defer lb.mu.Unlock()

	lb.agentLoads[agentID]++
}

// DecrementLoad decrementa a carga de um agente.
func (lb *LoadBalancer) DecrementLoad(agentID string) {
	lb.mu.Lock()
	defer lb.mu.Unlock()

	if lb.agentLoads[agentID] > 0 {
		lb.agentLoads[agentID]--
	}
}

// GetLoad retorna a carga atual de um agente.
func (lb *LoadBalancer) GetLoad(agentID string) int {
	lb.mu.RLock()
	defer lb.mu.RUnlock()

	return lb.agentLoads[agentID]
}

// GetAllLoads retorna a carga de todos os agentes.
func (lb *LoadBalancer) GetAllLoads() map[string]int {
	lb.mu.RLock()
	defer lb.mu.RUnlock()

	loads := make(map[string]int)
	for id, load := range lb.agentLoads {
		loads[id] = load
	}

	return loads
}

// IsBalanced verifica se a carga está balanceada (diferença < 2).
func (lb *LoadBalancer) IsBalanced() bool {
	lb.mu.RLock()
	defer lb.mu.RUnlock()

	if len(lb.agentLoads) == 0 {
		return true
	}

	minLoad := int(^uint(0) >> 1)
	maxLoad := 0

	for _, load := range lb.agentLoads {
		if load < minLoad {
			minLoad = load
		}
		if load > maxLoad {
			maxLoad = load
		}
	}

	return (maxLoad - minLoad) <= 2
}
