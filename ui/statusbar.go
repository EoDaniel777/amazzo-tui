package ui

import (
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/lipgloss"
)

// ModernStatusBar representa uma barra de status moderna
type ModernStatusBar struct {
	leftText   string
	rightText  string
	centerText string
	width      int
	height     int
	showTime   bool
	timeFormat string
}

// NewModernStatusBar cria uma nova barra de status
func NewModernStatusBar() *ModernStatusBar {
	return &ModernStatusBar{
		leftText:   "Amazzo Terminal",
		rightText:  "Pronto",
		centerText: "",
		width:      80,
		height:     1,
		showTime:   true,
		timeFormat: "15:04:05",
	}
}

// SetLeftText define o texto à esquerda
func (msb *ModernStatusBar) SetLeftText(text string) {
	msb.leftText = text
}

// SetRightText define o texto à direita
func (msb *ModernStatusBar) SetRightText(text string) {
	msb.rightText = text
}

// SetCenterText define o texto central
func (msb *ModernStatusBar) SetCenterText(text string) {
	msb.centerText = text
}

// SetShowTime define se mostra o horário
func (msb *ModernStatusBar) SetShowTime(show bool) {
	msb.showTime = show
}

// SetSize define o tamanho
func (msb *ModernStatusBar) SetSize(width, height int) {
	msb.width = width
	msb.height = height
}

// Render renderiza a barra de status
func (msb *ModernStatusBar) Render() string {
	var timeStr string
	if msb.showTime {
		timeStr = time.Now().Format(msb.timeFormat)
	}

	left := StatusBarStyle.Render(msb.leftText)
	right := StatusBarStyle.Render(msb.rightText)
	
	if msb.centerText != "" {
		right = StatusBarStyle.Render(fmt.Sprintf("%s | %s", msb.centerText, timeStr))
	} else if msb.showTime {
		right = StatusBarStyle.Render(timeStr)
	}

	// Calcular o espaço disponível para centralizar
	totalLength := len(msb.leftText) + len(msb.centerText) + len(timeStr) + 4 // +4 for separators
	availableSpace := msb.width - totalLength
	padding := strings.Repeat(" ", max(0, availableSpace/2))

	// Juntar os elementos
	var parts []string
	parts = append(parts, left)
	
	if padding != "" {
		parts = append(parts, padding)
	}
	
	if msb.centerText != "" {
		parts = append(parts, StatusBarStyle.Render(msb.centerText))
		if padding != "" {
			parts = append(parts, padding)
		}
	}
	
	if msb.showTime {
		parts = append(parts, right)
	}

	return lipgloss.JoinHorizontal(lipgloss.Top, parts...)
}

// RenderSimple renderiza uma barra de status simples
func (msb *ModernStatusBar) RenderSimple() string {
	content := fmt.Sprintf("%s | %s", msb.leftText, msb.rightText)
	
	if msb.showTime {
		content = fmt.Sprintf("%s | %s", content, time.Now().Format(msb.timeFormat))
	}

	return StatusBarStyle.Width(msb.width).Render(content)
}

// RenderWithProgress renderiza com barra de progresso
func (msb *ModernStatusBar) RenderWithProgress(progress int) string {
	// Criar uma barra de progresso
	barWidth := 20
	filled := (progress * barWidth) / 100
	empty := barWidth - filled
	
	progressBar := strings.Repeat("█", filled) + strings.Repeat("░", empty)
	progressText := fmt.Sprintf("[%s] %d%%", progressBar, progress)
	
	content := fmt.Sprintf("%s | %s | %s", msb.leftText, progressText, msb.rightText)
	
	if msb.showTime {
		content = fmt.Sprintf("%s | %s", content, time.Now().Format(msb.timeFormat))
	}
	
	return StatusBarStyle.Width(msb.width).Render(content)
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}