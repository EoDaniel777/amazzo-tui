package ui

import (
	"strings"

	"github.com/charmbracelet/lipgloss"
)

type MenuItem struct {
	ID          string
	Label       string
	Description string
	IsActive    bool
}

var MainMenuItems = []MenuItem{
	{ID: "documents", Label: "Documentos", Description: "Gerenciar documentos", IsActive: true},
	{ID: "agents", Label: "Agentes", Description: "Configurar agentes", IsActive: false},
	{ID: "tasks", Label: "Tarefas", Description: "Visualizar tarefas", IsActive: false},
	{ID: "settings", Label: "Configurações", Description: "Preferências do sistema", IsActive: false},
}

func RenderMenu(selectedIndex int, width int) string {
	var menuItems []string
	
	for i, item := range MainMenuItems {
		var style lipgloss.Style
		if i == selectedIndex {
			style = ActiveButtonStyle
		} else {
			style = ButtonStyle
		}
		
		menuItem := style.Render(item.Label)
		menuItems = append(menuItems, menuItem)
	}
	
	menuRow := strings.Join(menuItems, "")
	centeredMenu := lipgloss.PlaceHorizontal(width, lipgloss.Center, menuRow)
	
	return centeredMenu
}