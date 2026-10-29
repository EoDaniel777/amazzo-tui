package ui

import (
	"github.com/charmbracelet/lipgloss"
)

// ModernTabs representa um sistema de abas moderno
type ModernTabs struct {
	tabs        []string
	activeIndex int
}

// NewModernTabs cria um novo sistema de abas
func NewModernTabs(tabs []string) *ModernTabs {
	return &ModernTabs{
		tabs:        tabs,
		activeIndex: 0,
	}
}

// SetActiveIndex define o índice ativo
func (mt *ModernTabs) SetActiveIndex(index int) {
	if index >= 0 && index < len(mt.tabs) {
		mt.activeIndex = index
	}
}

// GetActiveIndex retorna o índice ativo
func (mt *ModernTabs) GetActiveIndex() int {
	return mt.activeIndex
}

// GetActiveTab retorna a aba ativa
func (mt *ModernTabs) GetActiveTab() string {
	if mt.activeIndex >= 0 && mt.activeIndex < len(mt.tabs) {
		return mt.tabs[mt.activeIndex]
	}
	return ""
}

// Next seleciona a próxima aba
func (mt *ModernTabs) Next() {
	mt.activeIndex = (mt.activeIndex + 1) % len(mt.tabs)
}

// Prev seleciona a aba anterior
func (mt *ModernTabs) Prev() {
	mt.activeIndex--
	if mt.activeIndex < 0 {
		mt.activeIndex = len(mt.tabs) - 1
	}
}

// Render renderiza as abas
func (mt *ModernTabs) Render(width int) string {
	var tabElements []string

	for i, tab := range mt.tabs {
		var style lipgloss.Style
		if i == mt.activeIndex {
			style = ActiveTabStyle
		} else {
			style = InactiveTabStyle
		}
		tabElements = append(tabElements, style.Render(tab))
	}

	tabsRow := lipgloss.JoinHorizontal(lipgloss.Top, tabElements...)
	
	// Centraliza as abas
	tabsContainer := lipgloss.NewStyle().
		Width(width).
		Align(lipgloss.Center).
		Render(tabsRow)

	return tabsContainer
}

// RenderVertical renderiza as abas verticalmente (para layouts laterais)
func (mt *ModernTabs) RenderVertical() string {
	var tabElements []string

	for i, tab := range mt.tabs {
		var style lipgloss.Style
		if i == mt.activeIndex {
			style = ActiveTabStyle
		} else {
			style = InactiveTabStyle
		}
		tabElements = append(tabElements, style.Render(tab))
	}

	return lipgloss.JoinVertical(lipgloss.Left, tabElements...)
}