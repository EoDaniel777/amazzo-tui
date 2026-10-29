package agents

import (
	"bufio"
	"fmt"
	"os"
	"strings"
)

// AgentConfig configuração de um agente personalizado
type AgentConfig struct {
	Name          string
	Description   string
	Languages     []string
	Frameworks    []string
	DocURLs       []string
	Expertise     string
	SystemPrompt  string
}

// AgentBuilder constrói agentes especializados de forma interativa
type AgentBuilder struct {
	config AgentConfig
}

// NewAgentBuilder cria um novo construtor de agentes
func NewAgentBuilder() *AgentBuilder {
	return &AgentBuilder{
		config: AgentConfig{
			Languages:  make([]string, 0),
			Frameworks: make([]string, 0),
			DocURLs:    make([]string, 0),
		},
	}
}

// BuildInteractive constrói um agente de forma interativa
func (b *AgentBuilder) BuildInteractive() (*AgentConfig, error) {
	reader := bufio.NewReader(os.Stdin)

	fmt.Println("┌─ 🤖 Criar Agente Especializado")
	fmt.Println("│")

	// Nome do agente
	fmt.Print("│  Nome do agente: ")
	name, _ := reader.ReadString('\n')
	b.config.Name = strings.TrimSpace(name)

	// Descrição
	fmt.Print("│  Descrição (o que ele faz): ")
	desc, _ := reader.ReadString('\n')
	b.config.Description = strings.TrimSpace(desc)

	// Linguagens
	fmt.Print("│  Linguagens (separadas por vírgula): ")
	langs, _ := reader.ReadString('\n')
	b.config.Languages = b.parseList(langs)

	// Frameworks
	fmt.Print("│  Frameworks/libs (separados por vírgula): ")
	frameworks, _ := reader.ReadString('\n')
	b.config.Frameworks = b.parseList(frameworks)

	// URLs de documentação
	fmt.Println("│")
	fmt.Println("│  URLs de documentação (pressione Enter sem digitar para pular):")
	for {
		fmt.Print("│    URL: ")
		url, _ := reader.ReadString('\n')
		url = strings.TrimSpace(url)
		if url == "" {
			break
		}
		b.config.DocURLs = append(b.config.DocURLs, url)
	}

	// Área de expertise
	fmt.Print("│  Área de expertise: ")
	expertise, _ := reader.ReadString('\n')
	b.config.Expertise = strings.TrimSpace(expertise)

	// Gera prompt do sistema
	b.generateSystemPrompt()

	fmt.Println("│")
	fmt.Println("✅ Agente criado com sucesso!")
	fmt.Println("└─")

	return &b.config, nil
}

// parseList parseia uma string separada por vírgulas em slice
func (b *AgentBuilder) parseList(input string) []string {
	input = strings.TrimSpace(input)
	if input == "" {
		return []string{}
	}

	parts := strings.Split(input, ",")
	result := make([]string, 0, len(parts))

	for _, part := range parts {
		trimmed := strings.TrimSpace(part)
		if trimmed != "" {
			result = append(result, trimmed)
		}
	}

	return result
}

// generateSystemPrompt gera o prompt do sistema para o agente
func (b *AgentBuilder) generateSystemPrompt() {
	var builder strings.Builder

	builder.WriteString(fmt.Sprintf("Você é %s, um agente especializado.\n\n", b.config.Name))
	builder.WriteString(fmt.Sprintf("FUNÇÃO: %s\n\n", b.config.Description))

	if len(b.config.Languages) > 0 {
		builder.WriteString(fmt.Sprintf("LINGUAGENS: %s\n", strings.Join(b.config.Languages, ", ")))
	}

	if len(b.config.Frameworks) > 0 {
		builder.WriteString(fmt.Sprintf("FRAMEWORKS: %s\n", strings.Join(b.config.Frameworks, ", ")))
	}

	if b.config.Expertise != "" {
		builder.WriteString(fmt.Sprintf("EXPERTISE: %s\n", b.config.Expertise))
	}

	builder.WriteString("\nREGRAS:\n")
	builder.WriteString("1. SEMPRE implemente código funcional e completo\n")
	builder.WriteString("2. Use as melhores práticas das tecnologias listadas\n")
	builder.WriteString("3. NÃO crie documentação a menos que solicitado\n")
	builder.WriteString("4. FOQUE em código prático e direto\n")
	builder.WriteString("5. Responda em Português do Brasil\n")

	if len(b.config.DocURLs) > 0 {
		builder.WriteString("\nDOCUMENTAÇÃO DE REFERÊNCIA:\n")
		for _, url := range b.config.DocURLs {
			builder.WriteString(fmt.Sprintf("- %s\n", url))
		}
	}

	b.config.SystemPrompt = builder.String()
}

// GetConfig retorna a configuração do agente
func (b *AgentBuilder) GetConfig() *AgentConfig {
	return &b.config
}

// SaveToRegistry salva o agente no registro
// TODO: Implementar integração com SpecializedAgentRegistry
func (b *AgentBuilder) SaveToRegistry(registry *SpecializedAgentRegistry) error {
	// Por enquanto, apenas retorna sucesso
	// O usuário precisa adicionar manualmente em specialized.go
	return nil
}
