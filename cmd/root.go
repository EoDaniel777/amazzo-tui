package cmd

import (
	"fmt"
	"os"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/spf13/cobra"

	"amazzo/app"
)

var rootCmd = &cobra.Command{
	Use:   "amazzo",
	Short: "Amazzo é um agente de terminal para manipulação de documentos",
	Long: `Amazzo é um poderoso agente de terminal que permite manipular documentos,
processar informações e automatizar tarefas através de uma interface de terminal elegante.`,
}

func Execute() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
}

// GetTUIProgram retorna um programa TUI para execução direta
func GetTUIProgram(hfToken, workspace, tokenStatus string) (*tea.Program, error) {
	model, err := GetTUIModel(hfToken, workspace, tokenStatus)
	if err != nil {
		return nil, err
	}
	return tea.NewProgram(model), nil
}

// GetTUIModel retorna o modelo TUI
func GetTUIModel(hfToken, workspace, tokenStatus string) (tea.Model, error) {
	return app.New(hfToken, workspace, tokenStatus)
}