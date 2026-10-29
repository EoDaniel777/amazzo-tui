package ui

import (
	"fmt"
	"os"
	"os/exec"
	"runtime"
	"strings"
)

// ZoomLevel representa o nível de zoom.
type ZoomLevel int

const (
	ZoomMin     ZoomLevel = -3
	ZoomDefault ZoomLevel = 0
	ZoomMax     ZoomLevel = 5
)

// ZoomManager gerencia o zoom da interface.
type ZoomManager struct {
	currentLevel ZoomLevel
	fontSizes    map[ZoomLevel]int
	terminal     string
}

// NewZoomManager cria um novo gerenciador de zoom.
func NewZoomManager() *ZoomManager {
	return &ZoomManager{
		currentLevel: ZoomDefault,
		fontSizes: map[ZoomLevel]int{
			-3: 8,
			-2: 10,
			-1: 11,
			0:  12, // Padrão
			1:  14,
			2:  16,
			3:  18,
			4:  20,
			5:  24,
		},
		terminal: detectTerminal(),
	}
}

// ZoomIn aumenta o zoom.
func (zm *ZoomManager) ZoomIn() (bool, string) {
	if zm.currentLevel >= ZoomMax {
		return false, "Zoom máximo atingido"
	}

	zm.currentLevel++
	return zm.applyZoom()
}

// ZoomOut diminui o zoom.
func (zm *ZoomManager) ZoomOut() (bool, string) {
	if zm.currentLevel <= ZoomMin {
		return false, "Zoom mínimo atingido"
	}

	zm.currentLevel--
	return zm.applyZoom()
}

// ResetZoom reseta o zoom para o padrão.
func (zm *ZoomManager) ResetZoom() (bool, string) {
	zm.currentLevel = ZoomDefault
	return zm.applyZoom()
}

// GetCurrentLevel retorna o nível de zoom atual.
func (zm *ZoomManager) GetCurrentLevel() ZoomLevel {
	return zm.currentLevel
}

// GetCurrentFontSize retorna o tamanho da fonte atual.
func (zm *ZoomManager) GetCurrentFontSize() int {
	return zm.fontSizes[zm.currentLevel]
}

// applyZoom aplica o zoom ao terminal (se suportado).
func (zm *ZoomManager) applyZoom() (bool, string) {
	fontSize := zm.fontSizes[zm.currentLevel]

	switch zm.terminal {
	case "kitty":
		return zm.applyKittyZoom(fontSize)
	case "alacritty":
		return zm.applyAlacrittyZoom(fontSize)
	case "tilix":
		return zm.applyTilixZoom(fontSize)
	default:
		// Fallback: Simular zoom com espaçamento de linhas
		return true, fmt.Sprintf("Zoom simulado: nível %d (fonte ~%dpt)", zm.currentLevel, fontSize)
	}
}

// applyKittyZoom aplica zoom no Kitty terminal.
func (zm *ZoomManager) applyKittyZoom(fontSize int) (bool, string) {
	// Kitty suporta mudança de fonte via escape sequences
	// https://sw.kovidgoyal.net/kitty/remote-control/
	cmd := exec.Command("kitty", "@", "set-font-size", fmt.Sprintf("%d", fontSize))
	if err := cmd.Run(); err != nil {
		return false, fmt.Sprintf("Falha ao aplicar zoom no Kitty: %v", err)
	}

	return true, fmt.Sprintf("Zoom aplicado: %dpt", fontSize)
}

// applyAlacrittyZoom aplica zoom no Alacritty (requer recarga de config).
func (zm *ZoomManager) applyAlacrittyZoom(fontSize int) (bool, string) {
	// Alacritty não suporta mudança dinâmica, apenas via config
	// Simulamos com escape sequences para padding
	return true, fmt.Sprintf("Zoom simulado: nível %d (Alacritty requer config)", zm.currentLevel)
}

// applyTilixZoom aplica zoom no Tilix.
func (zm *ZoomManager) applyTilixZoom(fontSize int) (bool, string) {
	// Tilix suporta Ctrl+Plus/Minus nativamente
	// Aqui apenas registramos a mudança
	return true, fmt.Sprintf("Zoom nível %d (use Ctrl++ no Tilix)", zm.currentLevel)
}

// detectTerminal detecta qual terminal está sendo usado.
func detectTerminal() string {
	// Verificar variáveis de ambiente
	if term := os.Getenv("TERM_PROGRAM"); term != "" {
		return strings.ToLower(term)
	}

	if term := os.Getenv("TERM"); term != "" {
		term = strings.ToLower(term)
		if strings.Contains(term, "kitty") {
			return "kitty"
		}
		if strings.Contains(term, "alacritty") {
			return "alacritty"
		}
		if strings.Contains(term, "tilix") {
			return "tilix"
		}
	}

	// Verificar processo pai
	if runtime.GOOS == "linux" {
		if ppid := os.Getppid(); ppid > 0 {
			if cmdline, err := os.ReadFile(fmt.Sprintf("/proc/%d/cmdline", ppid)); err == nil {
				cmd := string(cmdline)
				if strings.Contains(cmd, "kitty") {
					return "kitty"
				}
				if strings.Contains(cmd, "alacritty") {
					return "alacritty"
				}
				if strings.Contains(cmd, "tilix") {
					return "tilix"
				}
			}
		}
	}

	return "unknown"
}

// GetScalingFactor retorna o fator de escala para UI elements.
func (zm *ZoomManager) GetScalingFactor() float64 {
	// Zoom 0 = 1.0, cada nível = ±10%
	return 1.0 + (float64(zm.currentLevel) * 0.1)
}

// ScaleWidth escala uma largura baseado no zoom.
func (zm *ZoomManager) ScaleWidth(width int) int {
	scaled := float64(width) * zm.GetScalingFactor()
	return int(scaled)
}

// ScaleHeight escala uma altura baseado no zoom.
func (zm *ZoomManager) ScaleHeight(height int) int {
	scaled := float64(height) * zm.GetScalingFactor()
	return int(scaled)
}

// GetZoomIndicator retorna um indicador visual do zoom.
func (zm *ZoomManager) GetZoomIndicator() string {
	level := int(zm.currentLevel)

	if level == 0 {
		return "🔍 100%"
	}

	percentage := 100 + (level * 10)
	if level > 0 {
		return fmt.Sprintf("🔍 %d%% ↑", percentage)
	}

	return fmt.Sprintf("🔍 %d%% ↓", percentage)
}

// SupportsNativeZoom verifica se o terminal suporta zoom nativo.
func (zm *ZoomManager) SupportsNativeZoom() bool {
	return zm.terminal == "kitty" || zm.terminal == "tilix"
}

// GetTerminal retorna o nome do terminal detectado.
func (zm *ZoomManager) GetTerminal() string {
	return zm.terminal
}
