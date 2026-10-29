package app

import (
	"amazzo/agents"
	"amazzo/themes"
	"fmt"

	"github.com/76creates/stickers/flexbox"
	"github.com/76creates/stickers/table"
	"github.com/charmbracelet/lipgloss"
)

// LayoutManager gerencia layouts flexíveis usando stickers
type LayoutManager struct {
	flexboxInstance *flexbox.FlexBox
	width           int
	height          int
}

// NewLayoutManager cria um novo gerenciador de layout
func NewLayoutManager(width, height int) *LayoutManager {
	return &LayoutManager{
		flexboxInstance: flexbox.New(width, height),
		width:           width,
		height:          height,
	}
}

// CreateMainLayout cria o layout principal do Amazzo
func (lm *LayoutManager) CreateMainLayout() *flexbox.FlexBox {
	// Configura FlexBox
	lm.flexboxInstance.SetWidth(lm.width)
	lm.flexboxInstance.SetHeight(lm.height)

	return lm.flexboxInstance
}

// CreateHeaderRow cria a linha do header com informações do modelo e sessão
func CreateHeaderRow(fb *flexbox.FlexBox, modelInfo, sessionInfo string, theme themes.ThemeConfig, width int) *flexbox.Row {
	leftWidth := width / 2
	rightWidth := width - leftWidth

	// Célula esquerda (info do modelo)
	leftCell := flexbox.NewCell(1, 1).
		SetContent(theme.InfoStyle.Render(modelInfo)).
		SetStyle(lipgloss.NewStyle().
			Width(leftWidth).
			Padding(0, 1).
			BorderStyle(lipgloss.RoundedBorder()).
			BorderForeground(lipgloss.Color("8")))

	// Célula direita (info da sessão)
	rightCell := flexbox.NewCell(1, 1).
		SetContent(theme.HelpStyle.Render(sessionInfo)).
		SetStyle(lipgloss.NewStyle().
			Width(rightWidth).
			Align(lipgloss.Right).
			Padding(0, 1).
			BorderStyle(lipgloss.RoundedBorder()).
			BorderForeground(lipgloss.Color("8")))

	// Cria row com as duas células
	return fb.NewRow().AddCells(leftCell, rightCell)
}

// CreateOutputRow cria a linha do output (viewport)
func CreateOutputRow(fb *flexbox.FlexBox, content string, theme themes.ThemeConfig, width, height int) *flexbox.Row {
	cell := flexbox.NewCell(1, height).
		SetContent(content).
		SetStyle(lipgloss.NewStyle().
			Width(width).
			Height(height).
			BorderStyle(lipgloss.DoubleBorder()).
			BorderForeground(lipgloss.Color("8")).
			Padding(0, 1))

	return fb.NewRow().AddCells(cell)
}

// CreateInputRow cria a linha de input (textarea)
func CreateInputRow(fb *flexbox.FlexBox, content string, theme themes.ThemeConfig, width int) *flexbox.Row {
	cell := flexbox.NewCell(1, 3).
		SetContent(content).
		SetStyle(lipgloss.NewStyle().
			Width(width).
			BorderStyle(lipgloss.RoundedBorder()).
			BorderForeground(lipgloss.Color("8")).
			Padding(0, 1))

	return fb.NewRow().AddCells(cell)
}

// CreateShortcutsRow cria a linha de atalhos
func CreateShortcutsRow(fb *flexbox.FlexBox, shortcuts string, theme themes.ThemeConfig, width int) *flexbox.Row {
	cell := flexbox.NewCell(1, 1).
		SetContent(theme.HelpStyle.Render(shortcuts)).
		SetStyle(lipgloss.NewStyle().
			Width(width).
			BorderStyle(lipgloss.RoundedBorder()).
			BorderForeground(lipgloss.Color("8")).
			Padding(0, 1))

	return fb.NewRow().AddCells(cell)
}

// ModelsTable cria uma tabela com lista de modelos usando stickers.Table
func ModelsTable(models []agents.ModelConfig, currentIndex int, theme themes.ThemeConfig) *table.Table {
	headers := []string{"#", "Nome", "Status"}
	tbl := table.NewTable(0, 0, headers)

	// Define larguras das colunas (ratio)
	tbl.SetRatio([]int{5, 60, 10})

	// Adiciona linhas
	rows := make([][]any, 0, len(models))
	for i, model := range models {
		status := "  "
		if i == currentIndex {
			status = "►"
		}

		row := []any{
			fmt.Sprintf("%d", i+1),
			model.DisplayName,
			status,
		}

		rows = append(rows, row)
	}

	tbl.AddRows(rows)

	return tbl
}

// AgentsTable cria uma tabela com lista de agentes especializados
func AgentsTable(agents []AgentInfo, theme themes.ThemeConfig) *table.Table {
	headers := []string{"Nome", "Expertise", "Status"}
	tbl := table.NewTable(0, 0, headers)

	// Define larguras
	tbl.SetRatio([]int{25, 30, 15})

	// Adiciona linhas
	rows := make([][]any, 0, len(agents))
	for _, agent := range agents {
		statusIcon := "✅"
		if !agent.Available {
			statusIcon = "⏸️"
		}

		row := []any{
			agent.Name,
			agent.Expertise,
			statusIcon,
		}

		rows = append(rows, row)
	}

	tbl.AddRows(rows)

	return tbl
}

// AgentInfo representa informações de um agente para tabela
type AgentInfo struct {
	Name      string
	Expertise string
	Available bool
}
