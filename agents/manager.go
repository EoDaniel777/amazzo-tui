package agents

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"sync"
	"time"
)

// Manager coordena múltiplos agentes trabalhando em paralelo.
type Manager struct {
	agents       []*Agent
	tasks        map[string]*Task
	lockManager  *LockManager
	api          AgentAPI
	mu           sync.RWMutex
	taskQueue    chan *Task
	maxAgents    int
	eventLog     []string
	eventLogMu   sync.Mutex
}

// NewManager cria um novo gerenciador de agentes.
func NewManager(api AgentAPI, maxAgents int) *Manager {
	if maxAgents <= 0 {
		maxAgents = 4
	}

	return &Manager{
		agents:      make([]*Agent, 0),
		tasks:       make(map[string]*Task),
		lockManager: NewLockManager(),
		api:         api,
		taskQueue:   make(chan *Task, 100),
		maxAgents:   maxAgents,
		eventLog:    make([]string, 0),
	}
}

// ExecuteWorkflow executa um workflow completo com múltiplos agentes.
func (m *Manager) ExecuteWorkflow(ctx context.Context, req CoordinatorRequest) (*WorkflowResult, error) {
	m.logEvent(fmt.Sprintf("🚀 Iniciando workflow: %s", req.UserInput))

	startTime := time.Now()

	// 1. Coordenador analisa a requisição e cria plano de tarefas
	m.logEvent("📋 Coordenador analisando requisição...")

	plan, err := m.createExecutionPlan(ctx, req)
	if err != nil {
		return nil, fmt.Errorf("falha ao criar plano: %w", err)
	}

	if len(plan) == 0 {
		return &WorkflowResult{
			Success:     true,
			Tasks:       []*Task{},
			Duration:    time.Since(startTime),
			FinalOutput: "Nenhuma tarefa necessária.",
		}, nil
	}

	// Valida que o plano tem tarefas reais de implementação, não só make_dir
	if !m.validatePlan(plan) {
		m.logEvent("⚠️  Plano tem apenas make_dir, solicitando correção...")
		return nil, fmt.Errorf("plano inválido: contém apenas criação de diretórios sem implementação de código")
	}

	m.logEvent(fmt.Sprintf("✅ Plano criado com %d tarefa(s)", len(plan)))

	// 2. Cria agentes especializados
	m.createAgents(plan)
	m.logEvent(fmt.Sprintf("👥 %d agente(s) criado(s)", len(m.agents)))

	// 3. Executa tarefas em paralelo respeitando dependências
	m.logEvent("⚙️  Executando tarefas...")

	if err := m.executeTasks(ctx, plan); err != nil {
		return m.buildResult(plan, startTime, false, err), err
	}

	// 4. Revisor valida todos os arquivos modificados
	if needsReview(plan) {
		m.logEvent("🔍 Revisor validando código...")
		if err := m.reviewChanges(ctx, plan); err != nil {
			m.logEvent(fmt.Sprintf("⚠️  Revisão encontrou problemas: %v", err))
		} else {
			m.logEvent("✅ Revisão concluída com sucesso")
		}
	}

	// 5. Executor roda testes se solicitado
	if needsTesting(plan) {
		m.logEvent("🧪 Executor rodando testes...")
		if err := m.runTests(ctx, plan); err != nil {
			m.logEvent(fmt.Sprintf("❌ Testes falharam: %v", err))
			return m.buildResult(plan, startTime, false, err), err
		}
		m.logEvent("✅ Testes passaram")
	}

	m.logEvent(fmt.Sprintf("🎉 Workflow concluído em %s", time.Since(startTime)))

	return m.buildResult(plan, startTime, true, nil), nil
}

// validatePlan verifica se o plano tem tarefas reais de implementação
func (m *Manager) validatePlan(tasks []*Task) bool {
	hasImplementation := false
	for _, task := range tasks {
		if task.Type == TaskWriteFile || task.Type == TaskEditFile {
			hasImplementation = true
			break
		}
	}
	return hasImplementation
}

// createExecutionPlan usa o coordenador para criar um plano de tarefas.
func (m *Manager) createExecutionPlan(ctx context.Context, req CoordinatorRequest) ([]*Task, error) {
	prompt := fmt.Sprintf(`Você é um Coordenador que divide tarefas para agentes especializados.

REQUISIÇÃO: %s
WORKSPACE: %s

REGRAS CRÍTICAS:
1. SEMPRE crie tarefas de write_file para IMPLEMENTAR código, não apenas make_dir
2. Cada arquivo .go, .js, .py, etc PRECISA de uma tarefa write_file COMPLETA
3. NÃO crie pastas sem propósito - só crie diretórios se os arquivos dentro deles já estiverem planejados
4. FOQUE em IMPLEMENTAÇÃO DE CÓDIGO, não em estrutura de pastas

TIPOS DE TAREFAS:
- write_file: Criar arquivo com código COMPLETO (USE ISTO PARA IMPLEMENTAR!)
- edit_file: Modificar arquivo existente
- read_file: Ler arquivo para análise
- make_dir: Criar diretório (USE SOMENTE se necessário antes de write_file)

FORMATO JSON:
[
  {
    "id": "task-1",
    "type": "make_dir",
    "description": "Criar diretório src",
    "priority": 10,
    "file_path": "src",
    "dependencies": []
  },
  {
    "id": "task-2",
    "type": "write_file",
    "description": "Implementar servidor HTTP em Go com endpoints /api/users",
    "model_name": "qwen2.5-coder-32b",
    "priority": 9,
    "file_path": "src/main.go",
    "dependencies": ["task-1"]
  }
]

EXEMPLO DE PLANO RUIM (NÃO FAÇA):
[{"type":"make_dir","file_path":"src"},{"type":"make_dir","file_path":"tests"}]

EXEMPLO DE PLANO BOM (FAÇA):
[{"type":"make_dir","file_path":"src"},{"type":"write_file","file_path":"src/main.go","description":"Implementar servidor HTTP completo"}]

IMPORTANTE: Priorize tarefas de write_file! A requisição pede IMPLEMENTAÇÃO, não estrutura de pastas!

RESPONDA APENAS JSON, SEM EXPLICAÇÕES.`, req.UserInput, req.WorkspaceDir)

	response, err := m.api.CallModel(ctx, "qwen", prompt)
	if err != nil {
		return nil, fmt.Errorf("erro ao chamar coordenador: %w", err)
	}

	// Extrai JSON da resposta
	jsonStr := extractJSON(response)
	if jsonStr == "" {
		return nil, fmt.Errorf("coordenador não retornou JSON válido: %s", response)
	}

	var tasks []*Task
	if err := json.Unmarshal([]byte(jsonStr), &tasks); err != nil {
		return nil, fmt.Errorf("erro ao parsear plano: %w", err)
	}

	// Preenche campos adicionais
	now := time.Now()
	for _, task := range tasks {
		task.Status = StatusPending
		task.CreatedAt = now
		m.tasks[task.ID] = task
	}

	return tasks, nil
}

// createAgents cria agentes especializados com base nas tarefas.
func (m *Manager) createAgents(tasks []*Task) {
	m.mu.Lock()
	defer m.mu.Unlock()

	roleCount := make(map[AgentRole]int)

	// Analisa quais roles são necessários
	for _, task := range tasks {
		var role AgentRole
		switch task.Type {
		case TaskWriteFile, TaskEditFile:
			role = RoleCoder
		case TaskReviewCode, TaskValidate:
			role = RoleReviewer
		case TaskExecuteTests:
			role = RoleExecutor
		default:
			role = RoleCoder
		}
		roleCount[role]++
	}

	// Cria agentes (máximo 2 por role ou maxAgents total)
	agentID := 0
	for role, count := range roleCount {
		numAgents := count
		if numAgents > 2 {
			numAgents = 2
		}
		if len(m.agents)+numAgents > m.maxAgents {
			numAgents = m.maxAgents - len(m.agents)
		}

		for i := 0; i < numAgents && len(m.agents) < m.maxAgents; i++ {
			agentID++
			agent := &Agent{
				ID:          fmt.Sprintf("%s-%d", role, agentID),
				Role:        role,
				ModelName:   "qwen2.5-coder-32b", // Modelo padrão
				IsAvailable: true,
			}
			m.agents = append(m.agents, agent)
		}
	}
}

// executeTasks executa todas as tarefas respeitando dependências.
func (m *Manager) executeTasks(ctx context.Context, tasks []*Task) error {
	var wg sync.WaitGroup
	errChan := make(chan error, len(tasks))

	// Goroutine pool de workers
	workerCount := len(m.agents)
	if workerCount == 0 {
		workerCount = 1
	}

	for i := 0; i < workerCount; i++ {
		wg.Add(1)
		go func(workerID int) {
			defer wg.Done()

			for {
				select {
				case <-ctx.Done():
					return
				default:
					task := m.getNextReadyTask(tasks)
					if task == nil {
						// Verifica se todas completaram ou falharam
						if m.allTasksDone(tasks) {
							return
						}
						time.Sleep(100 * time.Millisecond)
						continue
					}

					// Executa tarefa
					if err := m.executeTask(ctx, task); err != nil {
						errChan <- err
						return
					}
				}
			}
		}(i)
	}

	wg.Wait()
	close(errChan)

	// Verifica erros
	for err := range errChan {
		if err != nil {
			return err
		}
	}

	return nil
}

// getNextReadyTask retorna a próxima tarefa pronta para executar.
func (m *Manager) getNextReadyTask(tasks []*Task) *Task {
	m.mu.Lock()
	defer m.mu.Unlock()

	for _, task := range tasks {
		if task.Status != StatusPending {
			continue
		}

		// Verifica dependências
		ready := true
		for _, depID := range task.Dependencies {
			depTask, exists := m.tasks[depID]
			if !exists || depTask.Status != StatusCompleted {
				ready = false
				break
			}
		}

		if ready {
			task.Status = StatusRunning
			return task
		}
	}

	return nil
}

// executeTask executa uma tarefa específica.
func (m *Manager) executeTask(ctx context.Context, task *Task) error {
	m.logEvent(fmt.Sprintf("▶️  Executando: %s (%s)", task.ID, task.Description))

	// Implementação simplificada - delega para a API
	params := map[string]interface{}{
		"description": task.Description,
		"model":       task.ModelName,
		"file_path":   task.FilePath,
		"type":        task.Type,
	}

	result, err := m.api.ExecuteTool(ctx, string(task.Type), params)
	if err != nil {
		task.Status = StatusFailed
		task.Error = err
		m.logEvent(fmt.Sprintf("❌ Falhou: %s - %v", task.ID, err))
		return err
	}

	task.Status = StatusCompleted
	task.Result = result
	now := time.Now()
	task.CompletedAt = &now
	m.logEvent(fmt.Sprintf("✅ Concluído: %s", task.ID))

	return nil
}

// allTasksDone verifica se todas as tarefas terminaram.
func (m *Manager) allTasksDone(tasks []*Task) bool {
	for _, task := range tasks {
		if task.Status == StatusPending || task.Status == StatusRunning {
			return false
		}
	}
	return true
}

// reviewChanges usa agente revisor para validar mudanças.
func (m *Manager) reviewChanges(ctx context.Context, tasks []*Task) error {
	// Coleta arquivos modificados
	var modifiedFiles []string
	for _, task := range tasks {
		if task.FilePath != "" && (task.Type == TaskWriteFile || task.Type == TaskEditFile) {
			modifiedFiles = append(modifiedFiles, task.FilePath)
		}
	}

	if len(modifiedFiles) == 0 {
		return nil
	}

	// TODO: Implementar lógica de revisão real
	m.logEvent(fmt.Sprintf("Revisando arquivos: %v", modifiedFiles))
	return nil
}

// runTests executa testes no projeto.
func (m *Manager) runTests(ctx context.Context, tasks []*Task) error {
	// TODO: Implementar execução de testes
	m.logEvent("Executando testes...")
	return nil
}

// buildResult constrói o resultado final do workflow.
func (m *Manager) buildResult(tasks []*Task, startTime time.Time, success bool, err error) *WorkflowResult {
	result := &WorkflowResult{
		Success:    success,
		Tasks:      tasks,
		Duration:   time.Since(startTime),
		AgentsUsed: make([]string, 0),
	}

	// Coleta IDs dos agentes usados
	agentIDs := make(map[string]bool)
	for _, agent := range m.agents {
		agentIDs[agent.ID] = true
	}
	for id := range agentIDs {
		result.AgentsUsed = append(result.AgentsUsed, id)
	}

	// Monta output final
	var output strings.Builder
	output.WriteString(fmt.Sprintf("📊 Workflow Executado em %s\n\n", result.Duration))
	output.WriteString(fmt.Sprintf("Tarefas: %d total\n", len(tasks)))

	completed := 0
	for _, task := range tasks {
		if task.Status == StatusCompleted {
			completed++
		}
	}
	output.WriteString(fmt.Sprintf("  ✅ Concluídas: %d\n", completed))
	output.WriteString(fmt.Sprintf("  ❌ Falhadas: %d\n\n", len(tasks)-completed))

	output.WriteString(fmt.Sprintf("Agentes: %d utilizados\n", len(result.AgentsUsed)))
	for _, agentID := range result.AgentsUsed {
		output.WriteString(fmt.Sprintf("  - %s\n", agentID))
	}

	if err != nil {
		output.WriteString(fmt.Sprintf("\n❌ Erro: %v\n", err))
		result.Errors = []error{err}
	}

	result.FinalOutput = output.String()
	return result
}

// logEvent registra um evento no log.
func (m *Manager) logEvent(event string) {
	m.eventLogMu.Lock()
	defer m.eventLogMu.Unlock()

	timestamp := time.Now().Format("15:04:05")
	m.eventLog = append(m.eventLog, fmt.Sprintf("[%s] %s", timestamp, event))
	m.api.LogEvent(event)
}

// GetEventLog retorna o log de eventos.
func (m *Manager) GetEventLog() []string {
	m.eventLogMu.Lock()
	defer m.eventLogMu.Unlock()

	logCopy := make([]string, len(m.eventLog))
	copy(logCopy, m.eventLog)
	return logCopy
}

// Helper functions

func needsReview(tasks []*Task) bool {
	for _, task := range tasks {
		if task.Type == TaskWriteFile || task.Type == TaskEditFile {
			return true
		}
	}
	return false
}

func needsTesting(tasks []*Task) bool {
	for _, task := range tasks {
		if task.Type == TaskExecuteTests {
			return true
		}
	}
	return false
}

func extractJSON(text string) string {
	// Procura por array JSON
	start := strings.Index(text, "[")
	end := strings.LastIndex(text, "]")

	if start >= 0 && end > start {
		return text[start : end+1]
	}

	// Procura por objeto JSON
	start = strings.Index(text, "{")
	end = strings.LastIndex(text, "}")

	if start >= 0 && end > start {
		return text[start : end+1]
	}

	return ""
}
