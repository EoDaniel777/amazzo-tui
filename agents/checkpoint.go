package agents

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"
)

// Checkpoint representa um snapshot do estado da workflow.
type Checkpoint struct {
	WorkflowID     string            `json:"workflow_id"`
	CompletedTasks []string          `json:"completed_tasks"`
	PendingTasks   []string          `json:"pending_tasks"`
	FailedTasks    []string          `json:"failed_tasks"`
	AgentStates    map[string]string `json:"agent_states"` // agentID -> status
	Timestamp      time.Time         `json:"timestamp"`
	Version        string            `json:"version"`
}

// CheckpointManager gerencia salvamento e recuperação de checkpoints.
type CheckpointManager struct {
	checkpointsDir string
	mu             sync.RWMutex
}

// NewCheckpointManager cria um novo gerenciador de checkpoints.
func NewCheckpointManager(baseDir string) (*CheckpointManager, error) {
	checkpointsDir := filepath.Join(baseDir, "checkpoints")
	if err := os.MkdirAll(checkpointsDir, 0755); err != nil {
		return nil, fmt.Errorf("falha ao criar diretório de checkpoints: %w", err)
	}

	return &CheckpointManager{
		checkpointsDir: checkpointsDir,
	}, nil
}

// Save salva um checkpoint no disco.
func (cm *CheckpointManager) Save(checkpoint Checkpoint) error {
	cm.mu.Lock()
	defer cm.mu.Unlock()

	checkpoint.Timestamp = time.Now()
	checkpoint.Version = "2.0"

	filename := fmt.Sprintf("%s_%s.json", checkpoint.WorkflowID, checkpoint.Timestamp.Format("20060102_150405"))
	filepath := filepath.Join(cm.checkpointsDir, filename)

	data, err := json.MarshalIndent(checkpoint, "", "  ")
	if err != nil {
		return fmt.Errorf("falha ao serializar checkpoint: %w", err)
	}

	if err := os.WriteFile(filepath, data, 0644); err != nil {
		return fmt.Errorf("falha ao salvar checkpoint: %w", err)
	}

	return nil
}

// Load carrega um checkpoint do disco.
func (cm *CheckpointManager) Load(workflowID string) (*Checkpoint, error) {
	cm.mu.RLock()
	defer cm.mu.RUnlock()

	// Encontrar checkpoint mais recente para este workflow
	files, err := os.ReadDir(cm.checkpointsDir)
	if err != nil {
		return nil, fmt.Errorf("falha ao ler diretório de checkpoints: %w", err)
	}

	var latestFile string
	var latestTime time.Time

	prefix := workflowID + "_"
	for _, file := range files {
		if file.IsDir() {
			continue
		}

		name := file.Name()
		if len(name) > len(prefix) && name[:len(prefix)] == prefix {
			info, err := file.Info()
			if err != nil {
				continue
			}

			if info.ModTime().After(latestTime) {
				latestTime = info.ModTime()
				latestFile = name
			}
		}
	}

	if latestFile == "" {
		return nil, fmt.Errorf("nenhum checkpoint encontrado para workflow %s", workflowID)
	}

	// Carregar checkpoint
	data, err := os.ReadFile(filepath.Join(cm.checkpointsDir, latestFile))
	if err != nil {
		return nil, fmt.Errorf("falha ao ler checkpoint: %w", err)
	}

	var checkpoint Checkpoint
	if err := json.Unmarshal(data, &checkpoint); err != nil {
		return nil, fmt.Errorf("falha ao deserializar checkpoint: %w", err)
	}

	return &checkpoint, nil
}

// List lista todos os checkpoints disponíveis.
func (cm *CheckpointManager) List() ([]Checkpoint, error) {
	cm.mu.RLock()
	defer cm.mu.RUnlock()

	files, err := os.ReadDir(cm.checkpointsDir)
	if err != nil {
		return nil, fmt.Errorf("falha ao ler diretório de checkpoints: %w", err)
	}

	var checkpoints []Checkpoint
	for _, file := range files {
		if file.IsDir() || filepath.Ext(file.Name()) != ".json" {
			continue
		}

		data, err := os.ReadFile(filepath.Join(cm.checkpointsDir, file.Name()))
		if err != nil {
			continue
		}

		var checkpoint Checkpoint
		if err := json.Unmarshal(data, &checkpoint); err != nil {
			continue
		}

		checkpoints = append(checkpoints, checkpoint)
	}

	return checkpoints, nil
}

// Delete remove um checkpoint específico.
func (cm *CheckpointManager) Delete(workflowID string) error {
	cm.mu.Lock()
	defer cm.mu.Unlock()

	files, err := os.ReadDir(cm.checkpointsDir)
	if err != nil {
		return fmt.Errorf("falha ao ler diretório de checkpoints: %w", err)
	}

	prefix := workflowID + "_"
	deleted := 0

	for _, file := range files {
		if file.IsDir() {
			continue
		}

		name := file.Name()
		if len(name) > len(prefix) && name[:len(prefix)] == prefix {
			if err := os.Remove(filepath.Join(cm.checkpointsDir, name)); err != nil {
				return fmt.Errorf("falha ao deletar checkpoint: %w", err)
			}
			deleted++
		}
	}

	if deleted == 0 {
		return fmt.Errorf("nenhum checkpoint encontrado para workflow %s", workflowID)
	}

	return nil
}

// CleanOld remove checkpoints mais antigos que a duração especificada.
func (cm *CheckpointManager) CleanOld(maxAge time.Duration) error {
	cm.mu.Lock()
	defer cm.mu.Unlock()

	files, err := os.ReadDir(cm.checkpointsDir)
	if err != nil {
		return fmt.Errorf("falha ao ler diretório de checkpoints: %w", err)
	}

	cutoff := time.Now().Add(-maxAge)
	deleted := 0

	for _, file := range files {
		if file.IsDir() || filepath.Ext(file.Name()) != ".json" {
			continue
		}

		info, err := file.Info()
		if err != nil {
			continue
		}

		if info.ModTime().Before(cutoff) {
			if err := os.Remove(filepath.Join(cm.checkpointsDir, file.Name())); err != nil {
				continue
			}
			deleted++
		}
	}

	return nil
}
