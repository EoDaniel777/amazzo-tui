package agents

import (
	"fmt"
	"time"
)

// TaskPriority representa a prioridade de uma tarefa.
type TaskPriority int

const (
	PriorityLow    TaskPriority = 0
	PriorityNormal TaskPriority = 5
	PriorityHigh   TaskPriority = 10
)

// TaskDependency representa as dependências de uma tarefa.
type TaskDependency struct {
	TaskID    string
	DependsOn []string      // IDs das tarefas que devem completar primeiro
	Priority  TaskPriority
	Timeout   time.Duration
}

// DAG (Directed Acyclic Graph) representa o grafo de dependências.
type DAG struct {
	nodes map[string]*DAGNode
	edges map[string][]string
}

// DAGNode representa um nó no grafo.
type DAGNode struct {
	TaskID       string
	Dependencies []string
	Priority     TaskPriority
	Level        int // Nível de profundidade no grafo (para paralelização)
}

// NewDAG cria um novo grafo de dependências.
func NewDAG() *DAG {
	return &DAG{
		nodes: make(map[string]*DAGNode),
		edges: make(map[string][]string),
	}
}

// AddTask adiciona uma tarefa ao grafo.
func (d *DAG) AddTask(dep TaskDependency) error {
	if _, exists := d.nodes[dep.TaskID]; exists {
		return fmt.Errorf("tarefa %s já existe no grafo", dep.TaskID)
	}

	d.nodes[dep.TaskID] = &DAGNode{
		TaskID:       dep.TaskID,
		Dependencies: dep.DependsOn,
		Priority:     dep.Priority,
		Level:        -1,
	}

	// Adicionar arestas
	for _, depID := range dep.DependsOn {
		d.edges[depID] = append(d.edges[depID], dep.TaskID)
	}

	return nil
}

// DetectCycles detecta ciclos no grafo (deadlocks).
func (d *DAG) DetectCycles() error {
	visited := make(map[string]bool)
	recStack := make(map[string]bool)

	for nodeID := range d.nodes {
		if !visited[nodeID] {
			if d.hasCycle(nodeID, visited, recStack) {
				return fmt.Errorf("ciclo detectado no grafo de dependências (deadlock potencial)")
			}
		}
	}

	return nil
}

// hasCycle é uma função auxiliar para detecção de ciclos.
func (d *DAG) hasCycle(nodeID string, visited, recStack map[string]bool) bool {
	visited[nodeID] = true
	recStack[nodeID] = true

	for _, neighbor := range d.edges[nodeID] {
		if !visited[neighbor] {
			if d.hasCycle(neighbor, visited, recStack) {
				return true
			}
		} else if recStack[neighbor] {
			return true
		}
	}

	recStack[nodeID] = false
	return false
}

// TopologicalSort retorna a ordem de execução das tarefas.
func (d *DAG) TopologicalSort() ([][]string, error) {
	if err := d.DetectCycles(); err != nil {
		return nil, err
	}

	// Calcular níveis de profundidade
	d.calculateLevels()

	// Agrupar tarefas por nível (podem ser executadas em paralelo)
	maxLevel := 0
	for _, node := range d.nodes {
		if node.Level > maxLevel {
			maxLevel = node.Level
		}
	}

	levels := make([][]string, maxLevel+1)
	for _, node := range d.nodes {
		levels[node.Level] = append(levels[node.Level], node.TaskID)
	}

	// Ordenar cada nível por prioridade
	for i := range levels {
		d.sortByPriority(levels[i])
	}

	return levels, nil
}

// calculateLevels calcula o nível de profundidade de cada nó.
func (d *DAG) calculateLevels() {
	// Encontrar nós raiz (sem dependências)
	for _, node := range d.nodes {
		if len(node.Dependencies) == 0 {
			node.Level = 0
		}
	}

	// Propagar níveis
	changed := true
	for changed {
		changed = false
		for _, node := range d.nodes {
			if node.Level == -1 {
				maxDepLevel := -1
				allDepsCalculated := true

				for _, depID := range node.Dependencies {
					depNode := d.nodes[depID]
					if depNode.Level == -1 {
						allDepsCalculated = false
						break
					}
					if depNode.Level > maxDepLevel {
						maxDepLevel = depNode.Level
					}
				}

				if allDepsCalculated && maxDepLevel != -1 {
					node.Level = maxDepLevel + 1
					changed = true
				}
			}
		}
	}
}

// sortByPriority ordena tarefas por prioridade (maior primeiro).
func (d *DAG) sortByPriority(taskIDs []string) {
	// Bubble sort simples (suficiente para listas pequenas)
	n := len(taskIDs)
	for i := 0; i < n-1; i++ {
		for j := 0; j < n-i-1; j++ {
			if d.nodes[taskIDs[j]].Priority < d.nodes[taskIDs[j+1]].Priority {
				taskIDs[j], taskIDs[j+1] = taskIDs[j+1], taskIDs[j]
			}
		}
	}
}

// GetReadyTasks retorna tarefas prontas para execução (todas dependências completas).
func (d *DAG) GetReadyTasks(completed map[string]bool) []string {
	var ready []string

	for taskID, node := range d.nodes {
		if completed[taskID] {
			continue
		}

		allDepsComplete := true
		for _, depID := range node.Dependencies {
			if !completed[depID] {
				allDepsComplete = false
				break
			}
		}

		if allDepsComplete {
			ready = append(ready, taskID)
		}
	}

	d.sortByPriority(ready)
	return ready
}
