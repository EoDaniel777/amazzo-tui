package commands

import (
	"fmt"
	"strings"

	"amazzo/agents"
)

// handleAgentsCommand gerencia agentes especializados.
func (cm *Manager) handleAgentsCommand(args string) string {
	parts := strings.Fields(args)

	if len(parts) == 0 {
		return cm.listSpecializedAgents()
	}

	subcommand := parts[0]

	switch subcommand {
	case "list":
		return cm.listSpecializedAgents()
	case "create":
		if len(parts) < 2 {
			return "❌ Uso: /agentes create <nome> <expertise>\n\nExpertises disponíveis:\n" +
				"- bubble-plugins\n- go\n- rust\n- python\n- javascript\n- frontend\n- backend\n- devops"
		}
		return cm.createSpecializedAgent(parts[1:])
	case "delete":
		if len(parts) < 2 {
			return "❌ Uso: /agentes delete <nome>"
		}
		return cm.deleteSpecializedAgent(parts[1])
	case "info":
		if len(parts) < 2 {
			return "❌ Uso: /agentes info <nome>"
		}
		return cm.showAgentInfo(parts[1])
	default:
		return "❌ Subcomando desconhecido. Use: list, create, delete, info"
	}
}

// listSpecializedAgents lista todos os agentes especializados.
func (cm *Manager) listSpecializedAgents() string {
	regInterface := cm.deps.GetSpecializedAgentRegistry()
	if regInterface == nil {
		return "❌ Registry de agentes não disponível"
	}

	registry, ok := regInterface.(*agents.SpecializedAgentRegistry)
	if !ok {
		return "❌ Erro ao acessar registry"
	}

	agents := registry.ListAgents()
	if len(agents) == 0 {
		return `📋 Nenhum agente especializado criado ainda.

Use: /agentes create <nome> <expertise>

Exemplos:
  /agentes create bubble-dev bubble-plugins
  /agentes create go-backend go
  /agentes create rust-core rust`
	}

	var result strings.Builder
	result.WriteString("🤖 AGENTES ESPECIALIZADOS\n\n")

	for i, agent := range agents {
		status := "✅"
		if !agent.IsAvailable {
			status = "⏸️"
		}

		result.WriteString(fmt.Sprintf("%d. %s %s\n", i+1, status, agent.ID))
		result.WriteString(fmt.Sprintf("   Expertise: %s\n", agent.Expertise))
		result.WriteString(fmt.Sprintf("   Modelo: %s\n", agent.ModelName))

		if agent.CanSearchWeb {
			result.WriteString(fmt.Sprintf("   🌐 Web Search: Ativado (%d URLs)\n", len(agent.DocumentationURLs)))
		}

		result.WriteString("\n")
	}

	result.WriteString("\nComandos:\n")
	result.WriteString("  /agentes info <nome>   - Ver detalhes\n")
	result.WriteString("  /agentes create        - Criar novo\n")
	result.WriteString("  /agentes delete <nome> - Remover\n")

	return result.String()
}

// createSpecializedAgent cria um novo agente especializado.
func (cm *Manager) createSpecializedAgent(args []string) string {
	if len(args) < 2 {
		return "❌ Uso: /agentes create <nome> <expertise>"
	}

	name := args[0]
	expertiseStr := args[1]

	// Mapeia string para ExpertiseArea
	expertiseMap := map[string]agents.ExpertiseArea{
		"bubble-plugins": agents.ExpertiseBubblePlugins,
		"go":            agents.ExpertiseGo,
		"rust":          agents.ExpertiseRust,
		"python":        agents.ExpertisePython,
		"javascript":    agents.ExpertiseJavaScript,
		"frontend":      agents.ExpertiseFrontend,
		"backend":       agents.ExpertiseBackend,
		"devops":        agents.ExpertiseDevOps,
	}

	expertise, exists := expertiseMap[expertiseStr]
	if !exists {
		return fmt.Sprintf("❌ Expertise '%s' inválida.\n\nDisponíveis: %v",
			expertiseStr,
			getMapKeys(expertiseMap))
	}

	regInterface := cm.deps.GetSpecializedAgentRegistry()
	if regInterface == nil {
		return "❌ Registry de agentes não disponível"
	}

	registry, ok := regInterface.(*agents.SpecializedAgentRegistry)
	if !ok {
		return "❌ Erro ao acessar registry"
	}

	// Configuração baseada na expertise
	config := agents.SpecializedAgentConfig{
		Expertise:       expertise,
		ModelName:       "qwen2.5-coder-32b",
		EnableWebSearch: true,
		DocumentationURLs: getDefaultDocsForExpertise(expertise),
	}

	agent, err := registry.CreateAgent(name, config)
	if err != nil {
		return fmt.Sprintf("❌ Erro ao criar agente: %v", err)
	}

	return fmt.Sprintf(`✅ Agente criado com sucesso!

🤖 Nome: %s
🎯 Expertise: %s
🧠 Modelo: %s
🌐 Web Search: %v

O agente está pronto para uso no modo multi-agente.
Use /multiagente para ativar e o agente será selecionado automaticamente para tarefas relevantes.`,
		agent.ID,
		agent.Expertise,
		agent.ModelName,
		agent.CanSearchWeb)
}

// deleteSpecializedAgent remove um agente.
func (cm *Manager) deleteSpecializedAgent(name string) string {
	regInterface := cm.deps.GetSpecializedAgentRegistry()
	if regInterface == nil {
		return "❌ Registry de agentes não disponível"
	}

	registry, ok := regInterface.(*agents.SpecializedAgentRegistry)
	if !ok {
		return "❌ Erro ao acessar registry"
	}

	if err := registry.DeleteAgent(name); err != nil {
		return fmt.Sprintf("❌ Erro ao deletar agente: %v", err)
	}

	return fmt.Sprintf("✅ Agente '%s' removido com sucesso", name)
}

// showAgentInfo mostra detalhes de um agente.
func (cm *Manager) showAgentInfo(name string) string {
	regInterface := cm.deps.GetSpecializedAgentRegistry()
	if regInterface == nil {
		return "❌ Registry de agentes não disponível"
	}

	registry, ok := regInterface.(*agents.SpecializedAgentRegistry)
	if !ok {
		return "❌ Erro ao acessar registry"
	}

	agent, exists := registry.GetAgent(name)
	if !exists {
		return fmt.Sprintf("❌ Agente '%s' não encontrado", name)
	}

	var result strings.Builder
	result.WriteString(fmt.Sprintf("🤖 AGENTE: %s\n\n", agent.ID))
	result.WriteString(fmt.Sprintf("🎯 Expertise: %s\n", agent.Expertise))
	result.WriteString(fmt.Sprintf("🧠 Modelo: %s\n", agent.ModelName))
	result.WriteString(fmt.Sprintf("📊 Status: %s\n", getAgentStatus(agent)))
	result.WriteString(fmt.Sprintf("🌐 Web Search: %v\n\n", agent.CanSearchWeb))

	if len(agent.DocumentationURLs) > 0 {
		result.WriteString("📚 URLs de Documentação:\n")
		for _, url := range agent.DocumentationURLs {
			result.WriteString(fmt.Sprintf("  - %s\n", url))
		}
		result.WriteString("\n")
	}

	result.WriteString("💡 System Prompt (preview):\n")
	preview := agent.SystemPrompt
	if len(preview) > 300 {
		preview = preview[:300] + "..."
	}
	result.WriteString(preview)

	return result.String()
}

// Helper functions

func getMapKeys(m map[string]agents.ExpertiseArea) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	return keys
}

func getDefaultDocsForExpertise(expertise agents.ExpertiseArea) []string {
	docsMap := map[agents.ExpertiseArea][]string{
		agents.ExpertiseBubblePlugins: {
			"https://manual.bubble.io/",
			"https://bubble.io/plugin_editor_documentation",
		},
		agents.ExpertiseGo: {
			"https://go.dev/doc/",
			"https://pkg.go.dev/std",
		},
		agents.ExpertiseRust: {
			"https://doc.rust-lang.org/",
			"https://docs.rs/",
		},
		agents.ExpertisePython: {
			"https://docs.python.org/3/",
			"https://pypi.org/",
		},
		agents.ExpertiseJavaScript: {
			"https://developer.mozilla.org/en-US/docs/Web/JavaScript",
			"https://www.typescriptlang.org/docs/",
		},
	}

	urls, exists := docsMap[expertise]
	if !exists {
		return []string{}
	}
	return urls
}

func getAgentStatus(agent *agents.SpecializedAgent) string {
	if agent.IsAvailable {
		return "✅ Disponível"
	}
	return "⏸️ Ocupado"
}
