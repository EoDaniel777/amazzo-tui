package agents

import (
	"fmt"
	"strings"
	"time"
)

// WorkflowTemplate representa um template de workflow reutilizável.
type WorkflowTemplate struct {
	Name        string
	Description string
	Tasks       []TaskTemplate
	Variables   map[string]string // Variáveis para substituição
}

// TaskTemplate representa um template de tarefa.
type TaskTemplate struct {
	Type       string
	Path       string
	Content    string
	DependsOn  []string
	Priority   TaskPriority
	Timeout    time.Duration
	Params     map[string]interface{}
}

// WorkflowTemplates contém templates pré-definidos.
var WorkflowTemplates = map[string]WorkflowTemplate{
	"crud_api": {
		Name:        "CRUD API Completa",
		Description: "Cria uma API REST completa com CRUD em Go",
		Tasks: []TaskTemplate{
			{
				Type:     "write_file",
				Path:     "{{project}}/models/{{entity}}.go",
				Priority: PriorityHigh,
				Timeout:  5 * time.Minute,
			},
			{
				Type:      "write_file",
				Path:      "{{project}}/handlers/{{entity}}_handler.go",
				DependsOn: []string{"models"},
				Priority:  PriorityNormal,
				Timeout:   5 * time.Minute,
			},
			{
				Type:      "write_file",
				Path:      "{{project}}/routes/{{entity}}_routes.go",
				DependsOn: []string{"handlers"},
				Priority:  PriorityNormal,
				Timeout:   5 * time.Minute,
			},
			{
				Type:      "write_file",
				Path:      "{{project}}/tests/{{entity}}_test.go",
				DependsOn: []string{"routes"},
				Priority:  PriorityLow,
				Timeout:   5 * time.Minute,
			},
			{
				Type:      "review_code",
				DependsOn: []string{"models", "handlers", "routes"},
				Priority:  PriorityHigh,
				Timeout:   3 * time.Minute,
			},
			{
				Type:      "execute_tests",
				DependsOn: []string{"review_code", "tests"},
				Priority:  PriorityHigh,
				Timeout:   10 * time.Minute,
			},
		},
		Variables: map[string]string{
			"project": "api",
			"entity":  "user",
		},
	},

	"refactoring": {
		Name:        "Refatoração de Código",
		Description: "Analisa e refatora código existente",
		Tasks: []TaskTemplate{
			{
				Type:     "analyze_code",
				Path:     "{{target_path}}",
				Priority: PriorityHigh,
				Timeout:  5 * time.Minute,
			},
			{
				Type:      "create_plan",
				DependsOn: []string{"analyze_code"},
				Priority:  PriorityHigh,
				Timeout:   3 * time.Minute,
			},
			{
				Type:      "execute_refactor",
				DependsOn: []string{"create_plan"},
				Priority:  PriorityNormal,
				Timeout:   10 * time.Minute,
			},
			{
				Type:      "run_tests",
				DependsOn: []string{"execute_refactor"},
				Priority:  PriorityHigh,
				Timeout:   10 * time.Minute,
			},
		},
		Variables: map[string]string{
			"target_path": ".",
		},
	},

	"microservice": {
		Name:        "Microserviço Completo",
		Description: "Cria um microserviço com Docker, testes e CI/CD",
		Tasks: []TaskTemplate{
			{
				Type:     "write_file",
				Path:     "{{project}}/main.go",
				Priority: PriorityHigh,
				Timeout:  5 * time.Minute,
			},
			{
				Type:      "write_file",
				Path:      "{{project}}/Dockerfile",
				DependsOn: []string{"main.go"},
				Priority:  PriorityNormal,
				Timeout:   3 * time.Minute,
			},
			{
				Type:      "write_file",
				Path:      "{{project}}/docker-compose.yml",
				DependsOn: []string{"Dockerfile"},
				Priority:  PriorityNormal,
				Timeout:   3 * time.Minute,
			},
			{
				Type:      "write_file",
				Path:      "{{project}}/.github/workflows/ci.yml",
				DependsOn: []string{"docker-compose.yml"},
				Priority:  PriorityLow,
				Timeout:   3 * time.Minute,
			},
			{
				Type:      "write_file",
				Path:      "{{project}}/README.md",
				DependsOn: []string{"main.go"},
				Priority:  PriorityLow,
				Timeout:   3 * time.Minute,
			},
			{
				Type:      "review_code",
				DependsOn: []string{"main.go", "Dockerfile"},
				Priority:  PriorityHigh,
				Timeout:   5 * time.Minute,
			},
		},
		Variables: map[string]string{
			"project": "microservice",
		},
	},

	"testing_suite": {
		Name:        "Suite de Testes",
		Description: "Cria suite completa de testes unitários e integração",
		Tasks: []TaskTemplate{
			{
				Type:     "analyze_code",
				Path:     "{{target_path}}",
				Priority: PriorityHigh,
				Timeout:  5 * time.Minute,
			},
			{
				Type:      "write_file",
				Path:      "{{target_path}}/tests/unit_test.go",
				DependsOn: []string{"analyze_code"},
				Priority:  PriorityHigh,
				Timeout:   10 * time.Minute,
			},
			{
				Type:      "write_file",
				Path:      "{{target_path}}/tests/integration_test.go",
				DependsOn: []string{"unit_test.go"},
				Priority:  PriorityNormal,
				Timeout:   10 * time.Minute,
			},
			{
				Type:      "write_file",
				Path:      "{{target_path}}/tests/benchmark_test.go",
				DependsOn: []string{"integration_test.go"},
				Priority:  PriorityLow,
				Timeout:   5 * time.Minute,
			},
			{
				Type:      "execute_tests",
				DependsOn: []string{"unit_test.go", "integration_test.go"},
				Priority:  PriorityHigh,
				Timeout:   15 * time.Minute,
			},
		},
		Variables: map[string]string{
			"target_path": ".",
		},
	},

	"documentation": {
		Name:        "Documentação Completa",
		Description: "Gera documentação completa do projeto",
		Tasks: []TaskTemplate{
			{
				Type:     "analyze_code",
				Path:     "{{project_path}}",
				Priority: PriorityHigh,
				Timeout:  5 * time.Minute,
			},
			{
				Type:      "write_file",
				Path:      "{{project_path}}/README.md",
				DependsOn: []string{"analyze_code"},
				Priority:  PriorityHigh,
				Timeout:   10 * time.Minute,
			},
			{
				Type:      "write_file",
				Path:      "{{project_path}}/docs/API.md",
				DependsOn: []string{"README.md"},
				Priority:  PriorityNormal,
				Timeout:   10 * time.Minute,
			},
			{
				Type:      "write_file",
				Path:      "{{project_path}}/docs/ARCHITECTURE.md",
				DependsOn: []string{"API.md"},
				Priority:  PriorityNormal,
				Timeout:   10 * time.Minute,
			},
			{
				Type:      "write_file",
				Path:      "{{project_path}}/CONTRIBUTING.md",
				DependsOn: []string{"ARCHITECTURE.md"},
				Priority:  PriorityLow,
				Timeout:   5 * time.Minute,
			},
		},
		Variables: map[string]string{
			"project_path": ".",
		},
	},
}

// InstantiateTemplate cria tarefas a partir de um template.
func InstantiateTemplate(templateName string, variables map[string]string) ([]Task, error) {
	template, exists := WorkflowTemplates[templateName]
	if !exists {
		return nil, fmt.Errorf("template %s não encontrado", templateName)
	}

	// Mesclar variáveis do template com as fornecidas
	mergedVars := make(map[string]string)
	for k, v := range template.Variables {
		mergedVars[k] = v
	}
	for k, v := range variables {
		mergedVars[k] = v
	}

	// Instanciar tarefas
	var tasks []Task
	for i, taskTemplate := range template.Tasks {
		task := Task{
			ID:       fmt.Sprintf("%s_task_%d", templateName, i),
			Type:     TaskType(taskTemplate.Type),
			Priority: int(taskTemplate.Priority),
			Context:  make(map[string]interface{}),
		}

		// Substituir variáveis no path
		if taskTemplate.Path != "" {
			task.FilePath = replaceVariables(taskTemplate.Path, mergedVars)
			task.Context["path"] = task.FilePath
		}

		// Copiar outros parâmetros
		if taskTemplate.Params != nil {
			for k, v := range taskTemplate.Params {
				task.Context[k] = v
			}
		}

		tasks = append(tasks, task)
	}

	return tasks, nil
}

// replaceVariables substitui variáveis no formato {{var}} por seus valores.
func replaceVariables(text string, vars map[string]string) string {
	result := text
	for key, value := range vars {
		placeholder := fmt.Sprintf("{{%s}}", key)
		result = strings.ReplaceAll(result, placeholder, value)
	}
	return result
}

// ListTemplates retorna lista de templates disponíveis.
func ListTemplates() []string {
	var names []string
	for name := range WorkflowTemplates {
		names = append(names, name)
	}
	return names
}

// GetTemplateInfo retorna informações sobre um template.
func GetTemplateInfo(templateName string) (string, error) {
	template, exists := WorkflowTemplates[templateName]
	if !exists {
		return "", fmt.Errorf("template %s não encontrado", templateName)
	}

	info := fmt.Sprintf("Nome: %s\n", template.Name)
	info += fmt.Sprintf("Descrição: %s\n", template.Description)
	info += fmt.Sprintf("Tarefas: %d\n", len(template.Tasks))
	info += "Variáveis:\n"
	for k, v := range template.Variables {
		info += fmt.Sprintf("  - %s: %s\n", k, v)
	}

	return info, nil
}
