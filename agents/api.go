package agents

import (
	"context"
	"fmt"
)

// DefaultAPI implementa AgentAPI conectando com o sistema principal.
type DefaultAPI struct {
	callModelFunc   func(ctx context.Context, model string, prompt string) (string, error)
	executeToolFunc func(ctx context.Context, tool string, params map[string]interface{}) (interface{}, error)
	lockManager     *LockManager
	logFunc         func(event string)
}

// NewDefaultAPI cria uma nova instância da API.
func NewDefaultAPI(
	callModel func(ctx context.Context, model string, prompt string) (string, error),
	executeTool func(ctx context.Context, tool string, params map[string]interface{}) (interface{}, error),
	logFunc func(event string),
) *DefaultAPI {
	return &DefaultAPI{
		callModelFunc:   callModel,
		executeToolFunc: executeTool,
		lockManager:     NewLockManager(),
		logFunc:         logFunc,
	}
}

// CallModel chama um modelo de IA.
func (api *DefaultAPI) CallModel(ctx context.Context, model string, prompt string) (string, error) {
	if api.callModelFunc == nil {
		return "", fmt.Errorf("callModelFunc não configurado")
	}
	return api.callModelFunc(ctx, model, prompt)
}

// ExecuteTool executa uma ferramenta MCP.
func (api *DefaultAPI) ExecuteTool(ctx context.Context, tool string, params map[string]interface{}) (interface{}, error) {
	if api.executeToolFunc == nil {
		return nil, fmt.Errorf("executeToolFunc não configurado")
	}
	return api.executeToolFunc(ctx, tool, params)
}

// LockFile adquire lock em um arquivo.
func (api *DefaultAPI) LockFile(path string) error {
	return api.lockManager.Lock(path, "agent")
}

// UnlockFile libera lock de um arquivo.
func (api *DefaultAPI) UnlockFile(path string) error {
	return api.lockManager.Unlock(path, "agent")
}

// IsFileLocked verifica se um arquivo está locked.
func (api *DefaultAPI) IsFileLocked(path string) bool {
	return api.lockManager.IsLocked(path)
}

// GetFileContent lê conteúdo de um arquivo (via tool).
func (api *DefaultAPI) GetFileContent(path string) (string, error) {
	result, err := api.ExecuteTool(context.Background(), "read_file", map[string]interface{}{
		"path": path,
	})
	if err != nil {
		return "", err
	}

	if str, ok := result.(string); ok {
		return str, nil
	}

	return fmt.Sprintf("%v", result), nil
}

// LogEvent registra um evento.
func (api *DefaultAPI) LogEvent(event string) {
	if api.logFunc != nil {
		api.logFunc(event)
	}
}
