package main

import (
	"fmt"
	"os"

	"amazzo/cmd"
)

func main() {
	// Se chamado sem argumentos ou com tui, inicia a interface TUI
	if len(os.Args) == 1 || (len(os.Args) > 1 && os.Args[1] == "tui") {
		// Obter token e status
		hfToken, tokenStatus, err := cmd.GetToken()
		if err != nil {
			fmt.Printf("Erro ao obter token: %v\n", err)
			os.Exit(1)
		}

		// Obter o diretório de trabalho atual
		workspace, err := os.Getwd()
		if err != nil {
			fmt.Printf("Erro ao obter diretório atual: %v\n", err)
			os.Exit(1)
		}

		// Iniciar o programa com o token, workspace e status
		p, err := cmd.GetTUIProgram(hfToken, workspace, tokenStatus)
		if err != nil {
			fmt.Printf("Erro ao iniciar TUI: %v\n", err)
			os.Exit(1)
		}

		if _, err := p.Run(); err != nil {
			// Verificar se é erro de TTY (terminal não suportado)
			if err.Error() == "could not open a new TTY: open /dev/tty: no such device or address" {
				fmt.Println("❌ Erro: Não é possível iniciar a interface TUI neste ambiente.")
				fmt.Println("💡 Dica: Execute em um terminal compatível com TUI (como o terminal local).")
				fmt.Println("   Ou use os comandos CLI normais: amazzo [comando]")
				os.Exit(1)
			} else {
				fmt.Printf("Erro ao rodar TUI: %v\n", err)
				os.Exit(1)
			}
		}
		return
	}

	// Executar comandos CLI normais
	cmd.Execute()
}
