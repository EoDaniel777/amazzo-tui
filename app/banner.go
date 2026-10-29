package app

import (
	"amazzo/themes"
)

// getBanner retorna o banner apropriado baseado na largura disponível.
func getBanner(width int, theme themes.ThemeConfig) string {
	// Banner completo para telas largas (>= 80 caracteres para margem de segurança)
	if width >= 80 {
		return theme.LogoStyle.Render(`╔══════════════════════════════════════════════════════════════════╗
║       █████╗ ███╗   ███╗ █████╗ ███████╗███████╗ ██████╗         ║
║      ██╔══██╗████╗ ████║██╔══██╗╚══███╔╝╚══███╔╝██╔═══██╗        ║
║      ███████║██╔████╔██║███████║  ███╔╝   ███╔╝ ██║   ██║        ║
║      ██╔══██║██║╚██╔╝██║██╔══██║ ███╔╝   ███╔╝  ██║   ██║        ║
║      ██║  ██║██║ ╚═╝ ██║██║  ██║███████╗███████╗╚██████╔╝        ║
║      ╚═╝  ╚═╝╚═╝     ╚═╝╚═╝  ╚═╝╚══════╝╚══════╝ ╚═════╝         ║
║            • Multi-model Command Suite •                         ║
╚══════════════════════════════════════════════════════════════════╝`)
	}

	// Banner médio para telas medianas (>= 60 caracteres)
	if width >= 60 {
		return theme.LogoStyle.Render(`╔════════════════════════════════════════════════╗
║       ╔═╗╔╦╗╔═╗╔═╗╔═╗╔═╗  ╔═╗╔═╗╔═╗╔╗╔╔╦╗      ║
║       ╠═╣║║║╠═╣╔═╝╔═╝║ ║  ╠═╣║ ╦║╣ ║║║ ║       ║
║       ╩ ╩╩ ╩╩ ╩╚═╝╚═╝╚═╝  ╩ ╩╚═╝╚═╝╝╚╝ ╩       ║
║        • Multi-model AI Assistant •            ║
╚════════════════════════════════════════════════╝`)
	}

	// Banner compacto para telas pequenas (>= 40 caracteres)
	if width >= 40 {
		return theme.LogoStyle.Render(`╔═══════════════════════════════╗
║      AMAZZO AGENT v2.0        ║
║   Multi-model AI Assistant    ║
╚═══════════════════════════════╝`)
	}

	// Banner mínimo para telas muito pequenas
	return theme.LogoStyle.Render(`┌─────────────────────┐
│  AMAZZO AGENT v2.0  │
└─────────────────────┘`)
}

// getCompactLogo retorna um logo inline quando o banner está oculto.
func getCompactLogo(theme themes.ThemeConfig) string {
	return theme.LogoStyle.Render("⚡ AMAZZO") + " • "
}
