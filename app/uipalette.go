package app

import (
	"strings"

	"amazzo/agents"

	"github.com/charmbracelet/lipgloss"
)

// uiPalette é o conjunto de cores da interface nova (chat, explorador, painel,
// barra de status e popup). Cada tema escolhido em /temas aponta para uma.
type uiPalette struct {
	Light bool

	Bg, Panel, Line, Fg, Dim string // fundo do chat, fundo dos painéis, bordas, texto, texto apagado
	User, Model              string // voz do usuário e do modelo
	Red, Yellow, Orange      string // erro, aviso/ação, caminhos e comandos
	Green                    string // arquivo novo, sucesso
	Bar                      string // fundo da barra de status e da seleção
}

var currentUIPalette = "github-dark"

// paletteKeyFor traduz a chave do tema para a paleta. O tema "default" antigo
// vira o GitHub Dark.
func paletteKeyFor(themeKey string) string {
	k := strings.ToLower(themeKey)
	if _, ok := uiPalettes[k]; ok {
		return k
	}
	return "github-dark"
}

// applyUIPalette troca as cores globais da interface.
func applyUIPalette(themeKey string) {
	key := paletteKeyFor(themeKey)
	p := uiPalettes[key]
	currentUIPalette = key

	ghBg, ghPanel, ghLine = lipgloss.Color(p.Bg), lipgloss.Color(p.Panel), lipgloss.Color(p.Line)
	ghFg, ghDim = lipgloss.Color(p.Fg), lipgloss.Color(p.Dim)
	ghBlue, ghGreen = lipgloss.Color(p.Model), lipgloss.Color(p.Green)
	ghRed, ghYellow = lipgloss.Color(p.Red), lipgloss.Color(p.Yellow)
	ghSel = lipgloss.Color(p.Bar)

	colEmber, colSignal = lipgloss.Color(p.User), lipgloss.Color(p.Model)
	colThought, colGraphite = lipgloss.Color(p.Dim), lipgloss.Color(p.Dim)
	colAct, colAlarm, colPath = lipgloss.Color(p.Yellow), lipgloss.Color(p.Red), lipgloss.Color(p.Orange)
}

func paletteIsLight() bool { return uiPalettes[currentUIPalette].Light }

// modelPurpose resume para que serve um modelo, pelo nome.
func modelPurpose(cfg agents.ModelConfig) string {
	if cfg.Provider == providerAmazzo {
		switch cfg.Name {
		case "amazzo":
			return "nuvem · modelo padrão do amazzo.tech"
		case "autom-tico":
			return "nuvem · o servidor escolhe o modelo"
		}
		if strings.Contains(cfg.DisplayName, "só texto") {
			return "nuvem · só texto"
		}
		return "nuvem · texto e imagem"
	}
	name := cfg.DisplayName
	n := strings.ToLower(name)
	var tags []string
	switch {
	case strings.Contains(n, "coder") || strings.Contains(n, "code"):
		tags = append(tags, "código")
	case strings.Contains(n, "lfm"):
		tags = append(tags, "leve e rápido")
	case strings.Contains(n, "qwen3"):
		tags = append(tags, "uso geral")
	case strings.Contains(n, "deepseek") || strings.Contains(n, "r1"):
		tags = append(tags, "raciocínio")
	case strings.Contains(n, "gpt") || strings.Contains(n, "kimi"):
		tags = append(tags, "uso geral")
	}
	if strings.Contains(n, "abl") {
		tags = append(tags, "sem restrições")
	}
	for _, s := range []string{"1.7b", "3b", "4b", "7b", "8b", "30b", "120b", "480b"} {
		if strings.Contains(n, s) {
			tags = append(tags, strings.ToUpper(s))
			break
		}
	}
	if strings.Contains(n, "ollama") || strings.Contains(n, "agent") || strings.Contains(n, "latest") {
		tags = append(tags, "local")
	}
	return strings.Join(tags, " · ")
}
