package tasks

import (
	"fmt"
	"strings"
	"sync"
)

// TaskStatus representa o status de uma tarefa
type TaskStatus string

const (
	StatusPending    TaskStatus = "pending"
	StatusInProgress TaskStatus = "in_progress"
	StatusCompleted  TaskStatus = "completed"
	StatusFailed     TaskStatus = "failed"
)

// Task representa uma tarefa individual
type Task struct {
	ID          string
	Description string
	Status      TaskStatus
	Error       string
}

// Tracker rastreia tarefas e garante conclusão
type Tracker struct {
	tasks  []*Task
	mu     sync.RWMutex
	active bool
}

// NewTracker cria um novo rastreador de tarefas
func NewTracker() *Tracker {
	return &Tracker{
		tasks:  make([]*Task, 0),
		active: false,
	}
}

// AddTask adiciona uma nova tarefa
func (t *Tracker) AddTask(description string) string {
	t.mu.Lock()
	defer t.mu.Unlock()

	id := fmt.Sprintf("task-%d", len(t.tasks)+1)
	task := &Task{
		ID:          id,
		Description: description,
		Status:      StatusPending,
	}

	t.tasks = append(t.tasks, task)
	t.active = true

	return id
}

// AddTasks adiciona múltiplas tarefas de uma vez
func (t *Tracker) AddTasks(descriptions []string) {
	for _, desc := range descriptions {
		t.AddTask(desc)
	}
}

// SetInProgress marca uma tarefa como em progresso
func (t *Tracker) SetInProgress(id string) error {
	t.mu.Lock()
	defer t.mu.Unlock()

	task := t.findTask(id)
	if task == nil {
		return fmt.Errorf("tarefa não encontrada: %s", id)
	}

	task.Status = StatusInProgress
	return nil
}

// SetCompleted marca uma tarefa como completada
func (t *Tracker) SetCompleted(id string) error {
	t.mu.Lock()
	defer t.mu.Unlock()

	task := t.findTask(id)
	if task == nil {
		return fmt.Errorf("tarefa não encontrada: %s", id)
	}

	task.Status = StatusCompleted

	// Desativa automaticamente se todas as tarefas foram concluídas ou falharam
	t.checkAndDeactivate()

	return nil
}

// SetFailed marca uma tarefa como falhada
func (t *Tracker) SetFailed(id string, err string) error {
	t.mu.Lock()
	defer t.mu.Unlock()

	task := t.findTask(id)
	if task == nil {
		return fmt.Errorf("tarefa não encontrada: %s", id)
	}

	task.Status = StatusFailed
	task.Error = err

	// Desativa automaticamente se todas as tarefas foram concluídas ou falharam
	t.checkAndDeactivate()

	return nil
}

// GetNextPending retorna a próxima tarefa pendente
func (t *Tracker) GetNextPending() *Task {
	t.mu.RLock()
	defer t.mu.RUnlock()

	for _, task := range t.tasks {
		if task.Status == StatusPending {
			return task
		}
	}

	return nil
}

// GetAllTasks retorna todas as tarefas
func (t *Tracker) GetAllTasks() []*Task {
	t.mu.RLock()
	defer t.mu.RUnlock()

	// Retorna uma cópia
	tasksCopy := make([]*Task, len(t.tasks))
	copy(tasksCopy, t.tasks)
	return tasksCopy
}

// IsComplete verifica se todas as tarefas foram completadas
func (t *Tracker) IsComplete() bool {
	t.mu.RLock()
	defer t.mu.RUnlock()

	if len(t.tasks) == 0 {
		return true
	}

	for _, task := range t.tasks {
		if task.Status != StatusCompleted {
			return false
		}
	}

	return true
}

// GetProgress retorna o progresso atual (completadas/total)
func (t *Tracker) GetProgress() (completed, total int) {
	t.mu.RLock()
	defer t.mu.RUnlock()

	total = len(t.tasks)
	for _, task := range t.tasks {
		if task.Status == StatusCompleted {
			completed++
		}
	}

	return
}

// GetSummary retorna um resumo formatado das tarefas
func (t *Tracker) GetSummary() string {
	t.mu.RLock()
	defer t.mu.RUnlock()

	if len(t.tasks) == 0 {
		return "Nenhuma tarefa ativa"
	}

	var builder strings.Builder
	builder.WriteString("┌─ 📋 Tarefas:\n")

	for _, task := range t.tasks {
		icon := "⏳"
		switch task.Status {
		case StatusCompleted:
			icon = "✅"
		case StatusInProgress:
			icon = "🔄"
		case StatusFailed:
			icon = "❌"
		}

		builder.WriteString(fmt.Sprintf("│  %s %s\n", icon, task.Description))
		if task.Error != "" {
			builder.WriteString(fmt.Sprintf("│     └─ Erro: %s\n", task.Error))
		}
	}

	completed, total := t.GetProgress()
	builder.WriteString(fmt.Sprintf("│\n│  Progresso: %d/%d completadas\n", completed, total))
	builder.WriteString("└─")

	return builder.String()
}

// Clear limpa todas as tarefas
func (t *Tracker) Clear() {
	t.mu.Lock()
	defer t.mu.Unlock()

	t.tasks = make([]*Task, 0)
	t.active = false
}

// IsActive verifica se o tracker está ativo
func (t *Tracker) IsActive() bool {
	t.mu.RLock()
	defer t.mu.RUnlock()

	return t.active
}

// checkAndDeactivate verifica se todas as tarefas estão em estado final e desativa o tracker
// IMPORTANTE: Esta função assume que o lock já foi adquirido pelo caller
func (t *Tracker) checkAndDeactivate() {
	if len(t.tasks) == 0 {
		t.active = false
		return
	}

	// Verifica se há alguma tarefa pendente ou em progresso
	for _, task := range t.tasks {
		if task.Status == StatusPending || task.Status == StatusInProgress {
			// Ainda há tarefas ativas
			return
		}
	}

	// Todas as tarefas estão em estado final (completed ou failed)
	t.active = false
}

// findTask busca uma tarefa por ID (sem lock, uso interno)
func (t *Tracker) findTask(id string) *Task {
	for _, task := range t.tasks {
		if task.ID == id {
			return task
		}
	}
	return nil
}

// ParseUserRequest analisa a requisição do usuário e extrai tarefas
func ParseUserRequest(request string) []string {
	request = strings.ToLower(request)
	tasks := make([]string, 0)

	// Padrões comuns de requisições
	if strings.Contains(request, "crie") || strings.Contains(request, "criar") {
		if strings.Contains(request, "projeto") {
			tasks = append(tasks, "Criar estrutura do projeto")
			tasks = append(tasks, "Implementar arquivos principais")
			tasks = append(tasks, "Adicionar configurações")
		} else if strings.Contains(request, "api") || strings.Contains(request, "servidor") {
			tasks = append(tasks, "Criar servidor")
			tasks = append(tasks, "Implementar endpoints")
			tasks = append(tasks, "Adicionar tratamento de erros")
		}
	}

	if strings.Contains(request, "implemente") || strings.Contains(request, "implementar") {
		tasks = append(tasks, "Implementar funcionalidade solicitada")
		tasks = append(tasks, "Adicionar validações")
	}

	if strings.Contains(request, "teste") || strings.Contains(request, "test") {
		tasks = append(tasks, "Escrever testes")
		tasks = append(tasks, "Executar testes")
	}

	// Se não detectou nenhuma tarefa específica, cria uma genérica
	if len(tasks) == 0 {
		tasks = append(tasks, "Processar requisição")
	}

	return tasks
}
