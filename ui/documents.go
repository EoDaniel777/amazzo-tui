package ui

import (
	"strings"

	"github.com/charmbracelet/lipgloss"
)

type Document struct {
	Name string
	Type string
	Size string
	Date string
}

func (d Document) Title() string       { return d.Name }
func (d Document) Description() string { return d.Type + " • " + d.Size + " • " + d.Date }
func (d Document) FilterValue() string { return d.Name }

var sampleDocuments = []Document{
	{Name: "relatorio-q1.pdf", Type: "PDF", Size: "2.4 MB", Date: "2023-04-15"},
	{Name: "contrato-cliente.docx", Type: "DOCX", Size: "0.8 MB", Date: "2023-04-10"},
	{Name: "apresentacao.pptx", Type: "PPTX", Size: "5.2 MB", Date: "2023-04-05"},
	{Name: "dados-analise.xlsx", Type: "XLSX", Size: "1.1 MB", Date: "2023-04-01"},
	{Name: "manual-usUARIO.pdf", Type: "PDF", Size: "3.7 MB", Date: "2023-03-28"},
}

func RenderDocumentsList(width, height int) string {
	title := SubtitleStyle.Render("Documentos Recentes")
	
	// Cabeçalho da tabela
	header := lipgloss.JoinHorizontal(lipgloss.Left,
		lipgloss.NewStyle().Width(width/4).Render("Nome"),
		lipgloss.NewStyle().Width(width/4).Render("Tipo"),
		lipgloss.NewStyle().Width(width/4).Render("Tamanho"),
		lipgloss.NewStyle().Width(width/4).Render("Data"),
	)
	
	underline := strings.Repeat("─", width)
	
	var rows []string
	rows = append(rows, title, header, underline)
	
	// Linhas de dados
	for _, doc := range sampleDocuments {
		row := lipgloss.JoinHorizontal(lipgloss.Left,
			lipgloss.NewStyle().Width(width/4).Render(doc.Name),
			lipgloss.NewStyle().Width(width/4).Render(doc.Type),
			lipgloss.NewStyle().Width(width/4).Render(doc.Size),
			lipgloss.NewStyle().Width(width/4).Render(doc.Date),
		)
		rows = append(rows, row)
	}
	
	content := lipgloss.JoinVertical(lipgloss.Left, rows...)
	
	return ContentStyle.Width(width - 4).Render(content)
}