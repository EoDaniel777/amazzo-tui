package ui

import (
	"fmt"
	"strings"
	"time"

	"amazzo/tasks"
	"amazzo/themes"
)

// TaskView renderiza tarefas em tempo real
type TaskView struct {
	tracker   *tasks.Tracker
	theme     themes.ThemeConfig
	startTime time.Time
	visible   bool
}

// NewTaskView cria uma nova view de tarefas
func NewTaskView(tracker *tasks.Tracker, theme themes.ThemeConfig) *TaskView {
	return &TaskView{
		tracker:   tracker,
		theme:     theme,
		startTime: time.Now(),
		visible:   true,
	}
}

// Render renderiza as tarefas com design moderno estilo Tododo
func (tv *TaskView) Render() string {
	if !tv.visible || !tv.tracker.IsActive() {
		return ""
	}

	completed, total := tv.tracker.GetProgress()
	if total == 0 {
		return ""
	}

	var builder strings.Builder

	// Cabeçalho compacto com progresso
	elapsed := time.Since(tv.startTime)
	percentage := float64(completed) / float64(total) * 100

	// Barra de progresso visual (estilo Gonzo)
	progressBar := tv.renderProgressBar(completed, total, 20)

	header := fmt.Sprintf("📋 Tarefas: %d/%d (%.0f%%) • %s\n%s",
		completed, total, percentage, formatDuration(elapsed), progressBar)
	builder.WriteString(tv.theme.InfoStyle.Render(header))
	builder.WriteString("\n\n")

	// Lista de tarefas compacta (estilo Tododo)
	allTasks := tv.tracker.GetAllTasks()
	for _, task := range allTasks {
		icon := tv.getTaskIcon(task.Status)

		// Linha compacta sem prefixos desnecessários
		taskLine := fmt.Sprintf("%s %s", icon, task.Description)

		switch task.Status {
		case tasks.StatusCompleted:
			builder.WriteString(tv.theme.InfoStyle.Render(taskLine))
		case tasks.StatusInProgress:
			// Destaque para tarefa em progresso
			builder.WriteString(tv.theme.LogoStyle.Render("▶ " + icon + " " + task.Description))
		case tasks.StatusFailed:
			builder.WriteString(tv.theme.ErrorStyle.Render(taskLine))
		default:
			builder.WriteString(tv.theme.HelpStyle.Render(taskLine))
		}

		builder.WriteString("\n")
	}

	return builder.String()
}

// renderProgressBar cria uma barra de progresso visual
func (tv *TaskView) renderProgressBar(completed, total, width int) string {
	if total == 0 {
		return ""
	}

	filled := int(float64(completed) / float64(total) * float64(width))
	empty := width - filled

	bar := strings.Repeat("█", filled) + strings.Repeat("░", empty)
	return bar
}

// getTaskIcon retorna o ícone para cada status (estilo moderno)
func (tv *TaskView) getTaskIcon(status tasks.TaskStatus) string {
	switch status {
	case tasks.StatusCompleted:
		return "✅" // Check mark (verde)
	case tasks.StatusInProgress:
		return "🔄" // Arrows rotating (progresso)
	case tasks.StatusFailed:
		return "❌" // X (vermelho)
	default:
		return "⏳" // Hourglass (pendente)
	}
}

// Toggle alterna visibilidade
func (tv *TaskView) Toggle() {
	tv.visible = !tv.visible
}

// Show mostra a view
func (tv *TaskView) Show() {
	tv.visible = true
}

// Hide esconde a view
func (tv *TaskView) Hide() {
	tv.visible = false
}

// IsVisible verifica se está visível
func (tv *TaskView) IsVisible() bool {
	return tv.visible
}

// Reset reseta o timer
func (tv *TaskView) Reset() {
	tv.startTime = time.Now()
}

// formatDuration formata duração de forma legível
func formatDuration(d time.Duration) string {
	if d < time.Second {
		return "< 1s"
	}

	seconds := int(d.Seconds())

	if seconds < 60 {
		return fmt.Sprintf("%ds", seconds)
	}

	minutes := seconds / 60
	remainingSeconds := seconds % 60

	if minutes < 60 {
		return fmt.Sprintf("%dm %ds", minutes, remainingSeconds)
	}

	hours := minutes / 60
	remainingMinutes := minutes % 60

	return fmt.Sprintf("%dh %dm", hours, remainingMinutes)
}
