package cmd

import (
	"fmt"
	"os"
	"sort"

	"github.com/spf13/cobra"

	"amazzo/mcpclient"
)

const addUso = `Exemplos:
  amazzo add mcp context7
  amazzo add mcp meu-servidor https://exemplo.com/mcp --header "Authorization=Bearer X"
  amazzo add mcp meu-servidor --env CHAVE=valor -- npx -y pacote-mcp
Use --projeto para gravar em .amazzo/mcp.json em vez do arquivo global.`

func runAddMCP(args []string) error {
	cfg, projeto, err := mcpclient.ParseAddArgs(args)
	if err != nil {
		return err
	}
	path := mcpclient.GlobalConfigPath()
	if projeto {
		wd, _ := os.Getwd()
		path = mcpclient.ProjectConfigPath(wd)
	}
	if err := mcpclient.AddServer(path, cfg); err != nil {
		return err
	}
	fmt.Printf("✓ Servidor MCP %q adicionado em %s\n  Abra a TUI (ou use /mcp recarregar) para conectar.\n", cfg.Name, path)
	return nil
}

func init() {
	addCmd := &cobra.Command{Use: "add", Short: "Adiciona recursos ao Amazzo"}
	addMcp := &cobra.Command{
		Use:                "mcp <nome> [url | -- comando args…]",
		Short:              "Adiciona um servidor MCP à configuração do Amazzo",
		Long:               addUso,
		DisableFlagParsing: true,
		RunE: func(c *cobra.Command, args []string) error {
			if len(args) == 1 && (args[0] == "-h" || args[0] == "--help") {
				return c.Help()
			}
			return runAddMCP(args)
		},
	}
	addCmd.AddCommand(addMcp)

	mcpCmd := &cobra.Command{Use: "mcp", Short: "Gerencia os servidores MCP do Amazzo"}
	mcpCmd.AddCommand(&cobra.Command{
		Use: "add <nome> ...", Short: "Mesmo que `amazzo add mcp`", Long: addUso,
		DisableFlagParsing: true,
		RunE:               func(c *cobra.Command, args []string) error { return runAddMCP(args) },
	})
	mcpCmd.AddCommand(&cobra.Command{
		Use: "list", Short: "Lista os servidores configurados",
		RunE: func(c *cobra.Command, args []string) error {
			wd, _ := os.Getwd()
			cfgs := mcpclient.LoadConfigs(mcpclient.ConfigPaths(wd))
			if len(cfgs) == 0 {
				fmt.Println("Nenhum servidor MCP configurado. Use: amazzo add mcp <nome>")
				return nil
			}
			sort.Slice(cfgs, func(i, j int) bool { return cfgs[i].Name < cfgs[j].Name })
			for _, s := range cfgs {
				alvo := s.URL
				if !s.Remote {
					alvo = fmt.Sprint(s.Command)
				}
				fmt.Printf("%-18s %s\n", s.Name, alvo)
			}
			return nil
		},
	})
	mcpCmd.AddCommand(&cobra.Command{
		Use: "remove <nome>", Aliases: []string{"rm"}, Short: "Remove um servidor", Args: cobra.ExactArgs(1),
		RunE: func(c *cobra.Command, args []string) error {
			wd, _ := os.Getwd()
			achou := false
			for _, p := range mcpclient.ConfigPaths(wd) {
				ok, err := mcpclient.RemoveServer(p, args[0])
				if err != nil {
					return err
				}
				achou = achou || ok
			}
			if !achou {
				return fmt.Errorf("servidor %q não encontrado", args[0])
			}
			fmt.Printf("✓ Servidor MCP %q removido\n", args[0])
			return nil
		},
	})
	rootCmd.AddCommand(addCmd, mcpCmd)
}
