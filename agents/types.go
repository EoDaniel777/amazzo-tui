package agents

import (
	"context"
	"time"
)

// AgentRole define o papel de um agente no sistema.
type AgentRole string

const (
	RoleCoordinator AgentRole = "coordinator" // Distribui tarefas
	RoleCoder       AgentRole = "coder"       // Escreve código
	RoleReviewer    AgentRole = "reviewer"    // Revisa e valida
	RoleExecutor    AgentRole = "executor"    // Executa comandos/testes
	RoleDocumenter  AgentRole = "documenter"  // Cria documentação
)

// TaskType define o tipo de tarefa a ser executada.
type TaskType string

const (
	TaskReadFile     TaskType = "read_file"
	TaskWriteFile    TaskType = "write_file"
	TaskEditFile     TaskType = "edit_file"
	TaskSearchCode   TaskType = "search_code"
	TaskReviewCode   TaskType = "review_code"
	TaskExecuteTests TaskType = "execute_tests"
	TaskValidate     TaskType = "validate"
	TaskCreateDocs   TaskType = "create_docs"
	TaskMakeDir      TaskType = "make_dir"
)

// Message representa uma mensagem no formato da API OpenAI/HF/Ollama.
//
// ToolName acompanha as mensagens de role "tool": e como o Ollama identifica
// de qual ferramenta veio o resultado ao montar o template de chat. Sem ele o
// resultado chega ao modelo como texto solto, e ele parafraseia em vez de ler
// (medimos 22 de 30 entradas de um list_dir, com agents/ e mcp/ omitidos).
type Message struct {
	Role       string     `json:"role"`
	Content    string     `json:"content"`
	ToolCalls  []ToolCall `json:"tool_calls,omitempty"`
	ToolCallID string     `json:"tool_call_id,omitempty"`
	ToolName   string     `json:"tool_name,omitempty"`
}

// Tool define a ferramenta que o modelo pode chamar.
type Tool struct {
	Type     ToolType `json:"type"`
	Function Function `json:"function"`
}

type ToolType string

const ToolTypeFunction ToolType = "function"

type Function struct {
	Name        string                 `json:"name"`
	Description string                 `json:"description"`
	Parameters  map[string]interface{} `json:"parameters"`
}

// ToolCall representa uma chamada de ferramenta feita pelo modelo.
type ToolCall struct {
	ID       string           `json:"id"`
	Function ToolCallFunction `json:"function"`
}

type ToolCallFunction struct {
	Name      string                 `json:"name"`
	Arguments map[string]interface{} `json:"arguments"`
}

// ModelConfig representa a configuração de um modelo LLM.
type ModelConfig struct {
	Name        string
	DisplayName string
	Endpoint    string
}

// Task representa uma tarefa a ser executada por um agente.
type Task struct {
	ID           string                 `json:"id"`
	Type         TaskType               `json:"type"`
	Description  string                 `json:"description"`
	ModelName    string                 `json:"model_name"`   // Qual modelo usar
	Priority     int                    `json:"priority"`     // 1-10
	Dependencies []string               `json:"dependencies"` // IDs de tarefas que devem completar antes
	Context      map[string]interface{} `json:"context"`      // Dados adicionais
	FilePath     string                 `json:"file_path,omitempty"`
	Status       TaskStatus             `json:"status"`
	Result       interface{}            `json:"result,omitempty"`
	Error        error                  `json:"error,omitempty"`
	CreatedAt    time.Time              `json:"created_at"`
	CompletedAt  *time.Time             `json:"completed_at,omitempty"`
}

// TaskStatus indica o estado de uma tarefa.
type TaskStatus string

const (
	StatusPending   TaskStatus = "pending"
	StatusRunning   TaskStatus = "running"
	StatusCompleted TaskStatus = "completed"
	StatusFailed    TaskStatus = "failed"
	StatusWaiting   TaskStatus = "waiting" // Aguardando dependências
	StatusCancelled TaskStatus = "cancelled"
)

// Agent representa um agente de IA com role específico.
type Agent struct {
	ID           string
	Role         AgentRole
	ModelName    string
	CurrentTask  *Task
	TasksHandled int
	IsAvailable  bool
}

// WorkflowResult contém o resultado de uma execução multi-agente.
type WorkflowResult struct {
	Success     bool
	Tasks       []*Task
	Duration    time.Duration
	AgentsUsed  []string
	FinalOutput string
	Errors      []error
}

// CoordinatorRequest é a requisição inicial para o coordenador.
type CoordinatorRequest struct {
	UserInput    string
	WorkspaceDir string
	MaxAgents    int
	Timeout      time.Duration
}

// AgentAPI define a interface que os agentes usam para se comunicar com o sistema.
type AgentAPI interface {
	CallModel(ctx context.Context, model string, prompt string) (string, error)
	ExecuteTool(ctx context.Context, tool string, params map[string]interface{}) (interface{}, error)
	LockFile(path string) error
	UnlockFile(path string) error
	IsFileLocked(path string) bool
	GetFileContent(path string) (string, error)
	LogEvent(event string)
}
