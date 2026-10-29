package agents

import (
	"context"
	"fmt"
	"sync"
	"time"
)

// Orchestrator é o "cérebro pensante" que gerencia e monitora todos os agentes.
// Ele garante que tarefas sejam completadas, faz retry de falhas, e redistribui trabalho.
type Orchestrator struct {
	// Lista de tarefas (TODO list)
	todoList []*Task
	todoMu   sync.RWMutex

	// Agentes disponíveis
	agents       []*Agent
	agentsMu     sync.RWMutex
	agentMetrics map[string]*AgentPerformance

	// Gerenciadores auxiliares
	manager      *Manager
	registry     *SpecializedAgentRegistry
	loadBalancer *LoadBalancer
	messageBus   *MessageBus
	dag          *DAG

	// Configurações
	checkInterval time.Duration // Intervalo de verificação (ex: 5s)
	maxRetries    int           // Tentativas por tarefa
	maxAgents     int           // Máximo de agentes simultâneos
	timeout       time.Duration // Timeout por tarefa

	// Controle de execução
	stopChan   chan bool
	pauseChan  chan bool
	resumeChan chan bool
	isRunning  bool
	isPaused   bool
	runningMu  sync.RWMutex

	// API e logging
	api AgentAPI

	// Métricas globais
	metrics *OrchestratorMetrics

	// Proteção contra deadlock
	lastProgressTime     time.Time
	lastCompletedCount   int
	cyclesWithoutProgress int
	maxCyclesWithoutProgress int // Máx ciclos sem progresso antes de parar
}

// AgentPerformance rastreia performance de cada agente.
type AgentPerformance struct {
	AgentID         string
	TasksCompleted  int
	TasksFailed     int
	TasksRetried    int
	AverageLatency  time.Duration
	LastActive      time.Time
	CurrentTask     *Task
	ConsecutiveFails int
}

// OrchestratorMetrics contém métricas globais do orquestrador.
type OrchestratorMetrics struct {
	TotalTasks       int
	CompletedTasks   int
	FailedTasks      int
	PendingTasks     int
	RunningTasks     int
	TotalRetries     int
	StartTime        time.Time
	LastUpdate       time.Time
	AgentsActive     int
	TasksPerMinute   float64
	mu               sync.RWMutex
}

// OrchestratorConfig configura o orquestrador.
type OrchestratorConfig struct {
	CheckInterval time.Duration
	MaxRetries    int
	MaxAgents     int
	Timeout       time.Duration
}

// NewOrchestrator cria um novo orquestrador.
func NewOrchestrator(api AgentAPI, config OrchestratorConfig) *Orchestrator {
	// Configurações padrão
	if config.CheckInterval == 0 {
		config.CheckInterval = 5 * time.Second
	}
	if config.MaxRetries == 0 {
		config.MaxRetries = 3
	}
	if config.MaxAgents == 0 {
		config.MaxAgents = 6
	}
	if config.Timeout == 0 {
		config.Timeout = 10 * time.Minute
	}

	manager := NewManager(api, config.MaxAgents)
	registry := NewSpecializedAgentRegistry("")
	loadBalancer := NewLoadBalancer()
	messageBus := NewMessageBus()
	dag := NewDAG()

	return &Orchestrator{
		todoList:     make([]*Task, 0),
		agents:       make([]*Agent, 0),
		agentMetrics: make(map[string]*AgentPerformance),

		manager:      manager,
		registry:     registry,
		loadBalancer: loadBalancer,
		messageBus:   messageBus,
		dag:          dag,

		checkInterval: config.CheckInterval,
		maxRetries:    config.MaxRetries,
		maxAgents:     config.MaxAgents,
		timeout:       config.Timeout,

		stopChan:   make(chan bool),
		pauseChan:  make(chan bool),
		resumeChan: make(chan bool),
		isRunning:  false,
		isPaused:   false,

		api: api,

		metrics: &OrchestratorMetrics{
			StartTime: time.Now(),
		},

		// Proteção contra deadlock
		lastProgressTime:         time.Now(),
		lastCompletedCount:       0,
		cyclesWithoutProgress:    0,
		maxCyclesWithoutProgress: 20, // Para após 20 ciclos sem progresso (20 * 5s = 100s)
	}
}

// Start inicia o orquestrador com uma requisição do usuário.
func (o *Orchestrator) Start(ctx context.Context, req CoordinatorRequest) error {
	o.runningMu.Lock()
	if o.isRunning {
		o.runningMu.Unlock()
		return fmt.Errorf("orquestrador já está em execução")
	}
	o.isRunning = true
	o.runningMu.Unlock()

	o.logEvent("🧠 ORQUESTRADOR INICIADO")
	o.logEvent(fmt.Sprintf("📝 Requisição: %s", req.UserInput))

	// 1. Criar plano de execução usando o coordenador
	o.logEvent("🤔 Pensando e criando plano de execução...")
	plan, err := o.manager.createExecutionPlan(ctx, req)
	if err != nil {
		return fmt.Errorf("falha ao criar plano: %w", err)
	}

	if len(plan) == 0 {
		o.logEvent("⚠️  Nenhuma tarefa identificada")
		return nil
	}

	// 2. Adiciona tarefas à TODO list
	o.addTasksToTodoList(plan)
	o.logEvent(fmt.Sprintf("✅ Plano criado: %d tarefas na TODO list", len(plan)))

	// 3. Cria agentes especializados baseado nas tarefas
	o.createSpecializedAgents(plan)
	o.logEvent(fmt.Sprintf("🤖 %d agentes especializados criados", len(o.agents)))

	// 4. Constrói o DAG de dependências
	if err := o.buildDAG(plan); err != nil {
		return fmt.Errorf("falha ao construir DAG: %w", err)
	}
	o.logEvent("🔗 DAG de dependências construído")

	// 5. Inicia loop de monitoramento contínuo
	o.logEvent("👀 Iniciando monitoramento contínuo...")
	return o.runMonitoringLoop(ctx)
}

// runMonitoringLoop é o loop principal que monitora e gerencia tudo.
func (o *Orchestrator) runMonitoringLoop(ctx context.Context) error {
	ticker := time.NewTicker(o.checkInterval)
	defer ticker.Stop()

	o.logEvent("🔄 Loop de monitoramento ativo")

	for {
		select {
		case <-ctx.Done():
			o.logEvent("🛑 Contexto cancelado, encerrando...")
			return ctx.Err()

		case <-o.stopChan:
			o.logEvent("🛑 Comando de parada recebido")
			return nil

		case <-o.pauseChan:
			o.logEvent("⏸️  Orquestrador pausado")
			o.isPaused = true
			<-o.resumeChan
			o.logEvent("▶️  Orquestrador retomado")
			o.isPaused = false

		case <-ticker.C:
			if o.isPaused {
				continue
			}

			// Verifica se ainda há tarefas pendentes
			if o.isComplete() {
				o.logEvent("🎉 TODAS AS TAREFAS CONCLUÍDAS!")
				o.printFinalReport()
				return nil
			}

			// Executa ciclo de verificação e ação
			if err := o.executeCycle(ctx); err != nil {
				o.logEvent(fmt.Sprintf("❌ Erro no ciclo: %v", err))
				// Não retorna erro, continua tentando
			}

			// Atualiza métricas
			o.updateMetrics()

			// Log de progresso
			o.logProgress()
		}
	}
}

// executeCycle executa um ciclo completo de verificação e ação.
func (o *Orchestrator) executeCycle(ctx context.Context) error {
	o.todoMu.Lock()
	defer o.todoMu.Unlock()

	// 0. Detecta deadlock (nenhum progresso em múltiplos ciclos)
	if err := o.detectDeadlock(); err != nil {
		return err
	}

	// 1. Verifica tarefas em execução
	o.checkRunningTasks()

	// 2. Identifica tarefas falhadas que precisam de retry
	o.retryFailedTasks(ctx)

	// 3. Identifica agentes inativos
	o.checkIdleAgents()

	// 4. Distribui novas tarefas para agentes disponíveis
	o.distributeNewTasks(ctx)

	// 5. Monitora saúde dos agentes
	o.monitorAgentHealth()

	// 6. Balanceia carga se necessário
	o.rebalanceLoad()

	return nil
}

// detectDeadlock detecta se o sistema está travado sem progresso.
func (o *Orchestrator) detectDeadlock() error {
	// Conta tarefas completadas atualmente
	currentCompleted := 0
	for _, task := range o.todoList {
		if task.Status == StatusCompleted {
			currentCompleted++
		}
	}

	// Verifica se houve progresso
	if currentCompleted > o.lastCompletedCount {
		// Houve progresso! Reseta contadores
		o.lastCompletedCount = currentCompleted
		o.lastProgressTime = time.Now()
		o.cyclesWithoutProgress = 0
		return nil
	}

	// Não houve progresso neste ciclo
	o.cyclesWithoutProgress++

	// Se excedeu limite de ciclos sem progresso
	if o.cyclesWithoutProgress >= o.maxCyclesWithoutProgress {
		// Verifica se há tarefas pendentes/running
		hasPendingOrRunning := false
		for _, task := range o.todoList {
			if task.Status == StatusPending || task.Status == StatusRunning {
				hasPendingOrRunning = true
				break
			}
		}

		if hasPendingOrRunning {
			o.logEvent(fmt.Sprintf("🔴 DEADLOCK DETECTADO: %d ciclos (%v) sem progresso",
				o.cyclesWithoutProgress, time.Since(o.lastProgressTime)))
			o.logEvent("   Possíveis causas:")
			o.logEvent("   - Dependências circulares")
			o.logEvent("   - Todos os agentes travados")
			o.logEvent("   - Tarefas impossíveis de completar")
			return fmt.Errorf("deadlock detectado: %d ciclos sem progresso", o.cyclesWithoutProgress)
		}
	}

	// Se passou muito tempo sem progresso (mas não excedeu ciclos), avisa
	if o.cyclesWithoutProgress > 0 && o.cyclesWithoutProgress%5 == 0 {
		o.logEvent(fmt.Sprintf("⚠️  Sem progresso há %d ciclos (%v)",
			o.cyclesWithoutProgress, time.Since(o.lastProgressTime)))
	}

	return nil
}

// checkRunningTasks verifica o status de tarefas em execução.
func (o *Orchestrator) checkRunningTasks() {
	for _, task := range o.todoList {
		if task.Status != StatusRunning {
			continue
		}

		// Verifica se a tarefa está travada (muito tempo em running)
		if time.Since(task.CreatedAt) > o.timeout {
			o.logEvent(fmt.Sprintf("⏱️  Tarefa %s expirou timeout, marcando como falhada", task.ID))
			task.Status = StatusFailed
			task.Error = fmt.Errorf("timeout após %v", o.timeout)

			// Atualiza métricas do agente
			o.updateAgentMetrics(task, false)
		}
	}
}

// retryFailedTasks tenta novamente tarefas que falharam.
func (o *Orchestrator) retryFailedTasks(ctx context.Context) {
	for _, task := range o.todoList {
		if task.Status != StatusFailed {
			continue
		}

		// Verifica se já excedeu max retries
		retryCount := o.getRetryCount(task)
		if retryCount >= o.maxRetries {
			o.logEvent(fmt.Sprintf("❌ Tarefa %s excedeu máximo de retries (%d)", task.ID, o.maxRetries))
			continue
		}

		// Marca para retry
		o.logEvent(fmt.Sprintf("🔄 Retry %d/%d para tarefa %s", retryCount+1, o.maxRetries, task.ID))
		task.Status = StatusPending
		task.Error = nil
		o.metrics.TotalRetries++

		// Incrementa contador de retry no contexto
		if task.Context == nil {
			task.Context = make(map[string]interface{})
		}
		task.Context["retry_count"] = retryCount + 1
	}
}

// distributeNewTasks distribui tarefas pendentes para agentes disponíveis.
func (o *Orchestrator) distributeNewTasks(ctx context.Context) {
	// Pega tarefas prontas do DAG (dependências satisfeitas)
	readyTasks := o.dag.GetReadyTasks(o.getCompletedTaskIDsMap())

	for _, taskID := range readyTasks {
		task := o.getTaskByID(taskID)
		if task == nil || task.Status != StatusPending {
			continue
		}

		// Encontra agente disponível
		agent := o.findAvailableAgent(task)
		if agent == nil {
			// Sem agentes disponíveis no momento
			continue
		}

		// Atribui tarefa ao agente
		o.assignTaskToAgent(ctx, task, agent)
	}
}

// assignTaskToAgent atribui uma tarefa a um agente específico.
func (o *Orchestrator) assignTaskToAgent(ctx context.Context, task *Task, agent *Agent) {
	o.logEvent(fmt.Sprintf("📤 Atribuindo tarefa %s ao agente %s", task.ID, agent.ID))

	task.Status = StatusRunning
	agent.CurrentTask = task
	agent.IsAvailable = false

	// Executa tarefa em goroutine separada
	go func() {
		startTime := time.Now()

		// Executa a tarefa
		err := o.manager.executeTask(ctx, task)

		duration := time.Since(startTime)

		// Atualiza status
		o.todoMu.Lock()
		if err != nil {
			task.Status = StatusFailed
			task.Error = err
			o.logEvent(fmt.Sprintf("❌ Tarefa %s falhou: %v", task.ID, err))
			o.updateAgentMetrics(task, false)
		} else {
			task.Status = StatusCompleted
			now := time.Now()
			task.CompletedAt = &now
			o.logEvent(fmt.Sprintf("✅ Tarefa %s concluída em %v", task.ID, duration))
			o.updateAgentMetrics(task, true)
		}

		// Libera agente
		agent.IsAvailable = true
		agent.CurrentTask = nil
		o.todoMu.Unlock()
	}()
}

// findAvailableAgent encontra o melhor agente disponível para uma tarefa.
func (o *Orchestrator) findAvailableAgent(task *Task) *Agent {
	o.agentsMu.RLock()
	defer o.agentsMu.RUnlock()

	// Primeiro tenta usar o registry para agente especializado
	if o.registry != nil {
		specializedAgent := o.registry.GetBestAgentForTask(task)
		if specializedAgent != nil && specializedAgent.IsAvailable {
			return specializedAgent.Agent
		}
	}

	// Caso contrário, pega qualquer agente disponível com role apropriado
	for _, agent := range o.agents {
		if !agent.IsAvailable {
			continue
		}

		// Verifica se o role é compatível
		if o.isAgentCompatible(agent, task) {
			return agent
		}
	}

	return nil
}

// isAgentCompatible verifica se um agente é compatível com uma tarefa.
func (o *Orchestrator) isAgentCompatible(agent *Agent, task *Task) bool {
	switch task.Type {
	case TaskWriteFile, TaskEditFile, TaskSearchCode:
		return agent.Role == RoleCoder || agent.Role == RoleCoordinator
	case TaskReviewCode, TaskValidate:
		return agent.Role == RoleReviewer
	case TaskExecuteTests:
		return agent.Role == RoleExecutor
	case TaskCreateDocs:
		return agent.Role == RoleDocumenter
	default:
		return true
	}
}

// checkIdleAgents verifica agentes que estão inativos.
func (o *Orchestrator) checkIdleAgents() {
	o.agentsMu.RLock()
	defer o.agentsMu.RUnlock()

	for _, agent := range o.agents {
		if !agent.IsAvailable {
			continue
		}

		metrics := o.agentMetrics[agent.ID]
		if metrics == nil {
			continue
		}

		// Se agente está inativo há muito tempo
		if time.Since(metrics.LastActive) > 2*o.checkInterval {
			o.logEvent(fmt.Sprintf("💤 Agente %s inativo há %v", agent.ID, time.Since(metrics.LastActive)))
		}
	}
}

// monitorAgentHealth monitora a saúde dos agentes.
func (o *Orchestrator) monitorAgentHealth() {
	o.agentsMu.RLock()
	defer o.agentsMu.RUnlock()

	for _, agent := range o.agents {
		metrics := o.agentMetrics[agent.ID]
		if metrics == nil {
			continue
		}

		// Se agente está falhando muito
		if metrics.ConsecutiveFails >= 3 {
			o.logEvent(fmt.Sprintf("🔴 Agente %s com %d falhas consecutivas - considere reiniciar",
				agent.ID, metrics.ConsecutiveFails))
		}

		// Se agente está muito lento
		if metrics.AverageLatency > o.timeout/2 {
			o.logEvent(fmt.Sprintf("🐌 Agente %s com latência alta: %v", agent.ID, metrics.AverageLatency))
		}
	}
}

// rebalanceLoad rebalanceia a carga entre agentes.
func (o *Orchestrator) rebalanceLoad() {
	if o.loadBalancer == nil {
		return
	}

	// Verifica se está balanceado
	isBalanced := o.loadBalancer.IsBalanced()
	if !isBalanced {
		o.logEvent("⚖️  Desbalanceamento detectado, redistribuindo tarefas...")
		// TODO: Implementar lógica de redistribuição
	}
}

// updateAgentMetrics atualiza métricas de um agente após executar uma tarefa.
func (o *Orchestrator) updateAgentMetrics(task *Task, success bool) {
	if task.CompletedAt == nil {
		return
	}

	agentID := o.findAgentIDForTask(task)
	if agentID == "" {
		return
	}

	metrics, exists := o.agentMetrics[agentID]
	if !exists {
		metrics = &AgentPerformance{
			AgentID: agentID,
		}
		o.agentMetrics[agentID] = metrics
	}

	metrics.LastActive = time.Now()

	if success {
		metrics.TasksCompleted++
		metrics.ConsecutiveFails = 0
	} else {
		metrics.TasksFailed++
		metrics.ConsecutiveFails++
	}

	// Atualiza latência média
	duration := task.CompletedAt.Sub(task.CreatedAt)
	if metrics.AverageLatency == 0 {
		metrics.AverageLatency = duration
	} else {
		metrics.AverageLatency = (metrics.AverageLatency + duration) / 2
	}
}

// findAgentIDForTask encontra o ID do agente que executou uma tarefa.
func (o *Orchestrator) findAgentIDForTask(task *Task) string {
	o.agentsMu.RLock()
	defer o.agentsMu.RUnlock()

	for _, agent := range o.agents {
		if agent.CurrentTask != nil && agent.CurrentTask.ID == task.ID {
			return agent.ID
		}
	}

	return ""
}

// updateMetrics atualiza as métricas globais do orquestrador.
func (o *Orchestrator) updateMetrics() {
	o.metrics.mu.Lock()
	defer o.metrics.mu.Unlock()

	o.todoMu.RLock()
	defer o.todoMu.RUnlock()

	completed := 0
	failed := 0
	pending := 0
	running := 0

	for _, task := range o.todoList {
		switch task.Status {
		case StatusCompleted:
			completed++
		case StatusFailed:
			failed++
		case StatusPending, StatusWaiting:
			pending++
		case StatusRunning:
			running++
		}
	}

	o.metrics.TotalTasks = len(o.todoList)
	o.metrics.CompletedTasks = completed
	o.metrics.FailedTasks = failed
	o.metrics.PendingTasks = pending
	o.metrics.RunningTasks = running
	o.metrics.LastUpdate = time.Now()

	// Calcula tarefas por minuto
	elapsed := time.Since(o.metrics.StartTime).Minutes()
	if elapsed > 0 {
		o.metrics.TasksPerMinute = float64(completed) / elapsed
	}

	// Conta agentes ativos
	o.agentsMu.RLock()
	activeAgents := 0
	for _, agent := range o.agents {
		if !agent.IsAvailable {
			activeAgents++
		}
	}
	o.agentsMu.RUnlock()
	o.metrics.AgentsActive = activeAgents
}

// logProgress loga o progresso atual.
func (o *Orchestrator) logProgress() {
	o.metrics.mu.RLock()
	defer o.metrics.mu.RUnlock()

	total := o.metrics.TotalTasks
	completed := o.metrics.CompletedTasks
	running := o.metrics.RunningTasks
	failed := o.metrics.FailedTasks

	if total == 0 {
		return
	}

	progress := float64(completed) / float64(total) * 100
	o.logEvent(fmt.Sprintf("📊 Progresso: %.1f%% (%d/%d) | Executando: %d | Falhadas: %d | Agentes ativos: %d | Taxa: %.1f tarefas/min",
		progress, completed, total, running, failed, o.metrics.AgentsActive, o.metrics.TasksPerMinute))
}

// printFinalReport imprime relatório final.
func (o *Orchestrator) printFinalReport() {
	o.metrics.mu.RLock()
	defer o.metrics.mu.RUnlock()

	duration := time.Since(o.metrics.StartTime)

	separator := "============================================================"
	o.logEvent(separator)
	o.logEvent("📈 RELATÓRIO FINAL DO ORQUESTRADOR")
	o.logEvent(separator)
	o.logEvent(fmt.Sprintf("⏱️  Tempo total: %v", duration))
	o.logEvent(fmt.Sprintf("📝 Total de tarefas: %d", o.metrics.TotalTasks))
	o.logEvent(fmt.Sprintf("✅ Concluídas: %d", o.metrics.CompletedTasks))
	o.logEvent(fmt.Sprintf("❌ Falhadas: %d", o.metrics.FailedTasks))
	o.logEvent(fmt.Sprintf("🔄 Total de retries: %d", o.metrics.TotalRetries))
	o.logEvent(fmt.Sprintf("🤖 Agentes utilizados: %d", len(o.agents)))
	o.logEvent(fmt.Sprintf("⚡ Taxa média: %.1f tarefas/min", o.metrics.TasksPerMinute))
	o.logEvent("")

	// Performance por agente
	o.logEvent("👥 PERFORMANCE DOS AGENTES:")
	for agentID, metrics := range o.agentMetrics {
		o.logEvent(fmt.Sprintf("  %s:", agentID))
		o.logEvent(fmt.Sprintf("    ✅ Concluídas: %d", metrics.TasksCompleted))
		o.logEvent(fmt.Sprintf("    ❌ Falhadas: %d", metrics.TasksFailed))
		o.logEvent(fmt.Sprintf("    🔄 Retries: %d", metrics.TasksRetried))
		o.logEvent(fmt.Sprintf("    ⏱️  Latência média: %v", metrics.AverageLatency))
	}
	o.logEvent(separator)
}

// Helper functions

func (o *Orchestrator) addTasksToTodoList(tasks []*Task) {
	o.todoMu.Lock()
	defer o.todoMu.Unlock()
	o.todoList = append(o.todoList, tasks...)
}

func (o *Orchestrator) createSpecializedAgents(tasks []*Task) {
	o.agentsMu.Lock()
	defer o.agentsMu.Unlock()

	// Conta tipos de tarefas necessárias
	roleCount := make(map[AgentRole]int)
	for _, task := range tasks {
		role := o.determineRoleForTask(task)
		roleCount[role]++
	}

	// Cria agentes (máximo 2 por role)
	agentID := 0
	for role, count := range roleCount {
		numAgents := min(count, 2)
		if len(o.agents)+numAgents > o.maxAgents {
			numAgents = o.maxAgents - len(o.agents)
		}

		for i := 0; i < numAgents && len(o.agents) < o.maxAgents; i++ {
			agentID++
			agent := &Agent{
				ID:          fmt.Sprintf("%s-%d", role, agentID),
				Role:        role,
				ModelName:   "qwen2.5-coder-32b",
				IsAvailable: true,
			}
			o.agents = append(o.agents, agent)

			// Inicializa métricas
			o.agentMetrics[agent.ID] = &AgentPerformance{
				AgentID:    agent.ID,
				LastActive: time.Now(),
			}
		}
	}
}

func (o *Orchestrator) determineRoleForTask(task *Task) AgentRole {
	switch task.Type {
	case TaskWriteFile, TaskEditFile, TaskSearchCode:
		return RoleCoder
	case TaskReviewCode, TaskValidate:
		return RoleReviewer
	case TaskExecuteTests:
		return RoleExecutor
	case TaskCreateDocs:
		return RoleDocumenter
	default:
		return RoleCoder
	}
}

func (o *Orchestrator) buildDAG(tasks []*Task) error {
	for _, task := range tasks {
		priority := PriorityNormal
		if task.Priority >= 8 {
			priority = PriorityHigh
		} else if task.Priority <= 3 {
			priority = PriorityLow
		}

		dep := TaskDependency{
			TaskID:    task.ID,
			DependsOn: task.Dependencies,
			Priority:  priority,
			Timeout:   o.timeout,
		}

		if err := o.dag.AddTask(dep); err != nil {
			return err
		}
	}

	// Verifica ciclos
	if err := o.dag.DetectCycles(); err != nil {
		return fmt.Errorf("dependências cíclicas detectadas no DAG: %w", err)
	}

	return nil
}

func (o *Orchestrator) isComplete() bool {
	o.todoMu.RLock()
	defer o.todoMu.RUnlock()

	for _, task := range o.todoList {
		if task.Status == StatusPending || task.Status == StatusRunning || task.Status == StatusWaiting {
			return false
		}
	}

	return true
}

func (o *Orchestrator) getRetryCount(task *Task) int {
	if task.Context == nil {
		return 0
	}
	count, ok := task.Context["retry_count"].(int)
	if !ok {
		return 0
	}
	return count
}

func (o *Orchestrator) getCompletedTaskIDs() []string {
	o.todoMu.RLock()
	defer o.todoMu.RUnlock()

	var completed []string
	for _, task := range o.todoList {
		if task.Status == StatusCompleted {
			completed = append(completed, task.ID)
		}
	}
	return completed
}

func (o *Orchestrator) getCompletedTaskIDsMap() map[string]bool {
	o.todoMu.RLock()
	defer o.todoMu.RUnlock()

	completed := make(map[string]bool)
	for _, task := range o.todoList {
		if task.Status == StatusCompleted {
			completed[task.ID] = true
		}
	}
	return completed
}

func (o *Orchestrator) getTaskByID(id string) *Task {
	for _, task := range o.todoList {
		if task.ID == id {
			return task
		}
	}
	return nil
}

func (o *Orchestrator) logEvent(msg string) {
	timestamp := time.Now().Format("15:04:05")
	logMsg := fmt.Sprintf("[%s] %s", timestamp, msg)
	o.api.LogEvent(logMsg)
}

// Controle externo

// Stop para o orquestrador.
func (o *Orchestrator) Stop() {
	o.stopChan <- true
}

// Pause pausa o orquestrador.
func (o *Orchestrator) Pause() {
	o.pauseChan <- true
}

// Resume retoma o orquestrador.
func (o *Orchestrator) Resume() {
	o.resumeChan <- true
}

// GetMetrics retorna métricas atuais.
func (o *Orchestrator) GetMetrics() *OrchestratorMetrics {
	o.metrics.mu.RLock()
	defer o.metrics.mu.RUnlock()

	// Retorna cópia
	return &OrchestratorMetrics{
		TotalTasks:     o.metrics.TotalTasks,
		CompletedTasks: o.metrics.CompletedTasks,
		FailedTasks:    o.metrics.FailedTasks,
		PendingTasks:   o.metrics.PendingTasks,
		RunningTasks:   o.metrics.RunningTasks,
		TotalRetries:   o.metrics.TotalRetries,
		StartTime:      o.metrics.StartTime,
		LastUpdate:     o.metrics.LastUpdate,
		AgentsActive:   o.metrics.AgentsActive,
		TasksPerMinute: o.metrics.TasksPerMinute,
	}
}

// GetTodoList retorna cópia da TODO list.
func (o *Orchestrator) GetTodoList() []*Task {
	o.todoMu.RLock()
	defer o.todoMu.RUnlock()

	tasks := make([]*Task, len(o.todoList))
	copy(tasks, o.todoList)
	return tasks
}

// Helper function
func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
