package app

import (
	"fmt"
	"path/filepath"
	"strings"
	"time"

	"github.com/charmbracelet/lipgloss"
)

func (m *Model) View() string {
	// Splash screen
	if m.showSplash {
		if m.ultraMode {
			banner := m.ultimateStyles.RenderAwesomeBanner(m.width)
			elapsed := time.Since(m.splashStartTime)
			remaining := 3*time.Second - elapsed
			if remaining < 0 {
				remaining = 0
			}
			loadingMsg := fmt.Sprintf("Carregando sistemas de IA... %.1fs", remaining.Seconds())
			loader := m.ultimateStyles.RenderPulsingLoader(m.spinner.View())
			return lipgloss.JoinVertical(lipgloss.Center, "", banner, "", loader, "",
				m.ultimateStyles.RenderSeparator(m.width, m.ultimateStyles.NeonCyan),
				lipgloss.NewStyle().Foreground(m.ultimateStyles.NeonGreen).Render(loadingMsg))
		}

		banner := getBanner(m.width, m.theme)
		elapsed := time.Since(m.splashStartTime)
		remaining := 3*time.Second - elapsed
		if remaining < 0 {
			remaining = 0
		}
		loadingMsg := fmt.Sprintf("\n\n%s Carregando... %.1fs\n\n",
			m.theme.InfoStyle.Render("⚡"), remaining.Seconds())
		return lipgloss.JoinVertical(lipgloss.Center, "", "", banner, m.theme.HelpStyle.Render(loadingMsg))
	}

	// ========== LAYOUT CALCULATION ==========
	// Simple, clean calculations for a chat interface
	contentWidth := m.width - 2 // Minimal margin
	if contentWidth < 60 {
		contentWidth = 60
	}

	// Calculate heights for each section
	headerHeight := 1                         // Single line header
	footerHeight := 1                         // Single line footer
	inputBoxHeight := m.textarea.Height() + 6 // Input area: title + \n + content + padding(2) + border(2)

	// Task card height (if visible)
	taskHeight := 0
	var taskContent string
	if m.taskView != nil && m.taskView.IsVisible() {
		taskContent = m.taskView.Render()
		if taskContent != "" {
			taskHeight = lipgloss.Height(taskContent) + 6 // content + padding(4) + border(2)
		}
	}

	// Loading / streaming: montado aqui para que a altura seja medida de
	// verdade -- a previa do streaming cresce enquanto o modelo escreve.
	var loadingBlock string
	if m.loading {
		// A prévia já diz a fase e o tempo decorrido; um spinner com
		// "processando sua requisição" só duplicaria o ruído — e é linguagem
		// de API, não de modelo local.
		loadingBlock = m.streamingPreview()
		if loadingBlock == "" {
			loadingBlock = m.modernStyles.RenderLoadingSpinner(m.spinner.View())
		}
	}

	loadingHeight := 0
	if loadingBlock != "" {
		loadingHeight = lipgloss.Height(loadingBlock)
	}

	// Agent creator height (if active)
	agentCreatorHeight := 0
	var agentCreatorContent string
	if m.agentCreator != nil && m.agentCreator.IsActive() {
		agentCreatorContent = m.agentCreator.View()
		agentCreatorHeight = lipgloss.Height(agentCreatorContent) + 2
	}

	// Autocomplete height
	autocompleteHeight := 0
	if m.fileAutocomplete && len(m.fileAutocompleteList) > 0 {
		autocompleteHeight = 15 // Fixed height for autocomplete
	}

	// Viewport gets remaining space
	viewportHeight := m.height - headerHeight - footerHeight - inputBoxHeight -
		taskHeight - loadingHeight - agentCreatorHeight - autocompleteHeight - 5 // -5 for safety margin

	if viewportHeight < 10 {
		viewportHeight = 10
	}

	// Update viewport size if changed
	if m.viewport.Height != viewportHeight || m.viewport.Width != contentWidth {
		m.viewport.Width = contentWidth
		m.viewport.Height = viewportHeight
	}

	// ========== HEADER ==========
	workspace := filepath.Base(m.workspaceRoot)
	if workspace == "." || workspace == string(filepath.Separator) || workspace == "" {
		workspace = m.workspaceRoot
	}

	tokensDisplay := fmt.Sprintf("%d", m.totalTokens)
	if m.totalTokens >= 1000000 {
		tokensDisplay = fmt.Sprintf("%.1fM", float64(m.totalTokens)/1000000)
	} else if m.totalTokens >= 1000 {
		tokensDisplay = fmt.Sprintf("%.1fk", float64(m.totalTokens)/1000)
	}

	var header string
	if m.ultraMode {
		header = m.ultimateStyles.RenderAwesomeStatusBar(
			fmt.Sprintf("⚡ %s", availableModels[m.currentModel].DisplayName),
			fmt.Sprintf("📁 %s", workspace),
			fmt.Sprintf("🎯 %s", tokensDisplay),
			fmt.Sprintf("🔧 %d", m.toolCallsTotal))
	} else {
		meta := fmt.Sprintf("%s tokens · %d ferramentas", tokensDisplay, m.toolCallsTotal)
		if st := m.lastOllamaStats; st.GenTokens > 0 {
			meta = fmt.Sprintf("%.0f tok/s · %s", st.TokensPerSecond(), meta)
		}
		// Fora do fim da conversa, dizer onde você está evita a impressão de
		// que a resposta acabou ali.
		if !m.viewport.AtBottom() {
			meta = fmt.Sprintf("▲ %.0f%% · %s", m.viewport.ScrollPercent()*100, meta)
		}
		header = renderStatusBar(availableModels[m.currentModel].DisplayName,
			workspace, meta, m.width)
	}

	// ========== CHAT VIEWPORT ==========
	chatArea := m.viewport.View()

	// ========== TASK VIEW ==========
	var taskBlock string
	if taskContent != "" {
		taskBlock = m.modernStyles.RenderCard(taskContent, contentWidth)
	}

	// ========== INPUT AREA ==========
	var inputBlock string
	if m.agentCreator != nil && m.agentCreator.IsActive() {
		inputBlock = m.modernStyles.RenderCard(agentCreatorContent, contentWidth)
	} else {
		if m.ultraMode {
			inputBlock = m.ultimateStyles.RenderAwesomeInput(m.textarea.View(), contentWidth)
		} else {
			inputBlock = renderInputFrame(m.textarea.View(), contentWidth, !m.loading)
		}
	}

	// ========== AUTOCOMPLETE ==========
	var autocompleteBlock string
	if m.fileAutocomplete && len(m.fileAutocompleteList) > 0 {
		var items []string
		maxShow := 10
		if len(m.fileAutocompleteList) > maxShow {
			items = m.fileAutocompleteList[:maxShow]
		} else {
			items = m.fileAutocompleteList
		}

		var lines []string
		lines = append(lines, m.theme.InfoStyle.Render(fmt.Sprintf("📁 Files (%d):", len(m.fileAutocompleteList))))
		for i, file := range items {
			if i == m.fileAutocompleteIdx {
				lines = append(lines, m.theme.LogoStyle.Render(fmt.Sprintf(" ▶ %s", file)))
			} else {
				lines = append(lines, fmt.Sprintf("   %s", file))
			}
		}
		if len(m.fileAutocompleteList) > maxShow {
			lines = append(lines, m.theme.HelpStyle.Render(fmt.Sprintf("   ... +%d more", len(m.fileAutocompleteList)-maxShow)))
		}
		lines = append(lines, "")
		lines = append(lines, m.theme.HelpStyle.Render("↑/↓: Navigate • Enter/Tab: Select • Esc: Cancel"))

		autocompleteBlock = lipgloss.NewStyle().
			Width(contentWidth).
			BorderStyle(lipgloss.RoundedBorder()).
			BorderForeground(lipgloss.Color("10")).
			Padding(0, 1).
			Render(strings.Join(lines, "\n"))
	}

	// ========== FOOTER ==========
	shortcuts := []string{
		"enter enviar",
		"esc interromper",
		"↑↓ histórico",
		"pgup/pgdn rolar",
		"@ arquivos",
		"ctrl+r raciocínio",
		"ctrl+t tarefas",
		"ctrl+l limpar",
		"ctrl+c sair",
	}
	footer := renderFooter(shortcuts, m.width)

	// ========== ASSEMBLE UI ==========
	var sections []string

	sections = append(sections, header)
	sections = append(sections, chatArea)

	if taskBlock != "" {
		sections = append(sections, taskBlock)
	}

	if loadingBlock != "" {
		sections = append(sections, loadingBlock)
	}

	sections = append(sections, inputBlock)

	if autocompleteBlock != "" {
		sections = append(sections, autocompleteBlock)
	}

	sections = append(sections, footer)

	return lipgloss.JoinVertical(lipgloss.Left, sections...)
}
