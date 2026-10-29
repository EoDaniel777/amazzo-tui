package cmd

import (
	"fmt"
	"os"
	"path/filepath"

	"amazzo/app"
	"amazzo/credentials"

	"github.com/charmbracelet/bubbletea"
	"github.com/joho/godotenv"
	"github.com/spf13/cobra"
)

var tuiCmd = &cobra.Command{
	Use:   "tui",
	Short: "Iniciar a interface TUI do Amazzo",
	Long:  `Iniciar a interface de usuário baseada em terminal do Amazzo com recursos avançados de manipulação de documentos.`,
	Run: func(cmd *cobra.Command, args []string) {
		// Obtém o token de várias fontes
		hfToken, tokenStatus, err := GetToken()
		if err != nil {
			fmt.Printf("\n❌ Erro: %v\n", err)
			os.Exit(1)
		}

		cwd, err := os.Getwd()
		if err != nil {
			fmt.Printf("Erro ao obter diretório atual: %v\n", err)
			os.Exit(1)
		}

		// Inicializa o modelo da aplicação com o token e workspace
		model, err := app.New(hfToken, cwd, tokenStatus)
		if err != nil {
			fmt.Printf("Erro ao inicializar aplicação: %v\n", err)
			os.Exit(1)
		}

		p := tea.NewProgram(
			model,
			tea.WithAltScreen(),
			tea.WithMouseAllMotion(),
		)

		if _, err := p.Run(); err != nil {
			fmt.Printf("Erro ao executar o programa: %v\n", err)
			os.Exit(1)
		}
	},
}

// GetToken obtém o token de várias fontes (em ordem de prioridade)
func GetToken() (string, string, error) {
	// 1. Tenta variável de ambiente (para compatibilidade/CI/CD)
	if token := os.Getenv("HF_TOKEN"); token != "" {
		return token, "✓ Token carregado da variável de ambiente HF_TOKEN", nil
	}

	// 2. Tenta carregar de .env files
	homeDir, _ := os.UserHomeDir()
	envPaths := []string{
		".env",
		filepath.Join(homeDir, ".config", "amazzo", ".env"),
		filepath.Join(homeDir, ".env"),
	}

	for _, path := range envPaths {
		if _, err := os.Stat(path); err == nil {
			if err := godotenv.Load(path); err == nil {
				if token := os.Getenv("HF_TOKEN"); token != "" {
					return token, fmt.Sprintf("✓ Token carregado de: %s", path), nil
				}
			}
		}
	}

	// 3. Tenta carregar das credenciais criptografadas
	credManager, err := credentials.NewManager()
	if err != nil {
		return "", "", fmt.Errorf("erro ao inicializar gerenciador de credenciais: %w", err)
	}

	if credManager.HasToken() {
		token, err := credManager.LoadToken()
		if err != nil {
			// Se falhou ao carregar, avisamos mas continuamos para pedir um novo
			fmt.Printf("⚠️  Erro ao carregar token salvo: %v\n", err)
		} else if token != "" {
			return token, "✓ Token carregado das credenciais salvas (criptografado)", nil
		}
	}

	// 4. Sem token: segue em frente.
	//
	// O token so e necessario para os modelos do Hugging Face. Quem usa
	// apenas modelos locais (Ollama) nao tem token nenhum, e pedir um no
	// arranque travava a abertura -- inclusive pelo lancador grafico, onde
	// nao ha onde digitar. Quando um modelo remoto for escolhido sem token,
	// o erro explica o que fazer; /key configura quando for o caso.
	return "", "🏠 Modo local (Ollama). Use /key para habilitar os modelos remotos.", nil
}

func init() {
	rootCmd.AddCommand(tuiCmd)
}
