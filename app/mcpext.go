package app

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"amazzo/agents"
	"amazzo/mcp"
	"amazzo/mcpclient"

	tea "github.com/charmbracelet/bubbletea"
)

// Servidores MCP externos, adicionados pelo usuário ao Amazzo
// (`amazzo add mcp` ou /mcp add). Aparecem para o modelo como ferramentas "servidor_ferramenta".

type mcpReloadedMsg struct {
	status []mcpclient.Status
	quiet  bool
}

// mcpConfigPaths: só a configuração do próprio Amazzo (global e do projeto).
func mcpConfigPaths(workspace string) []string {
	return mcpclient.ConfigPaths(workspace)
}

func (m *Model) initMCP() {
	home, _ := os.UserHomeDir()
	m.mcpMgr = mcpclient.NewManager(m.workspaceRoot, mcpConfigPaths(m.workspaceRoot),
		filepath.Join(home, ".config", "amazzo", "mcp-estado.json"))
	mcp.SetExternal(mcpAdapter{m.mcpMgr, func() string { return m.workspaceRoot }})
}

// mcpAdapter liga o gerenciador ao executor de ferramentas, e acrescenta duas
// ferramentas próprias: o modelo consegue recarregar os servidores sozinho.
type mcpAdapter struct {
	mgr *mcpclient.Manager
	ws  func() string
}

func (a mcpAdapter) Has(name string) bool {
	switch name {
	case "mcp_reload", "mcp_list", "mcp_search", "mcp_call", "mcp_add", "mcp_remove":
		return true
	}
	return a.mgr.Has(name)
}

func (a mcpAdapter) Call(ctx context.Context, name string, args map[string]interface{}) (string, error) {
	switch name {
	case "mcp_reload":
		return resumoMCP(a.mgr.Reload(ctx), a.mgr), nil
	case "mcp_add":
		return a.add(ctx, args)
	case "mcp_remove":
		nome, _ := args["name"].(string)
		achou := false
		for _, p := range mcpclient.ConfigPaths(a.ws()) {
			if ok, _ := mcpclient.RemoveServer(p, nome); ok {
				achou = true
			}
		}
		if !achou {
			return "", fmt.Errorf("servidor %q não está na configuração do Amazzo", nome)
		}
		return "Removido. " + resumoMCP(a.mgr.Reload(ctx), a.mgr), nil
	case "mcp_list":
		return resumoMCP(a.mgr.Statuses(), a.mgr), nil
	case "mcp_search":
		q, _ := args["query"].(string)
		return buscaFerramentasMCP(a.mgr, q), nil
	case "mcp_call":
		alvo, _ := args["tool"].(string)
		inner, _ := args["arguments"].(map[string]interface{})
		if alvo == "" {
			return "", fmt.Errorf("mcp_call precisa de `tool` (nome completo, veja mcp_search)")
		}
		return a.mgr.Call(ctx, alvo, inner)
	}
	return a.mgr.Call(ctx, name, args)
}

// add grava um servidor na configuração do Amazzo e já reconecta, para o modelo
// poder usar as ferramentas na mesma conversa.
func (a mcpAdapter) add(ctx context.Context, args map[string]interface{}) (string, error) {
	cfg := mcpclient.ServerConfig{Env: map[string]string{}, Headers: map[string]string{}}
	cfg.Name, _ = args["name"].(string)
	if u, _ := args["url"].(string); u != "" {
		cfg.Remote, cfg.URL = true, u
	}
	switch c := args["command"].(type) {
	case string:
		if c != "" {
			cfg.Command = append(cfg.Command, c)
		}
	case []interface{}:
		for _, x := range c {
			cfg.Command = append(cfg.Command, fmt.Sprint(x))
		}
	}
	if xs, ok := args["args"].([]interface{}); ok && len(cfg.Command) > 0 {
		for _, x := range xs {
			cfg.Command = append(cfg.Command, fmt.Sprint(x))
		}
	}
	for k, dst := range map[string]map[string]string{"env": cfg.Env, "headers": cfg.Headers} {
		if mp, ok := args[k].(map[string]interface{}); ok {
			for ek, ev := range mp {
				dst[ek] = fmt.Sprint(ev)
			}
		}
	}
	if !cfg.Remote && len(cfg.Command) == 0 {
		if p, ok := mcpclient.Presets[strings.ToLower(cfg.Name)]; ok {
			cfg.Command = p.Command
		}
	}
	path := mcpclient.GlobalConfigPath()
	if esc, _ := args["scope"].(string); esc == "project" && a.ws() != "" {
		path = mcpclient.ProjectConfigPath(a.ws())
	}
	if err := mcpclient.AddServer(path, cfg); err != nil {
		return "", err
	}
	return fmt.Sprintf("Servidor %q gravado em %s.\n%s", cfg.Name, path, resumoMCP(a.mgr.Reload(ctx), a.mgr)), nil
}

func resumoMCP(st []mcpclient.Status, mgr *mcpclient.Manager) string {
	if len(st) == 0 {
		return "Nenhum servidor MCP configurado. Adicione com /mcp add <nome>."
	}
	var sb strings.Builder
	tools := mgr.Tools()
	for _, s := range st {
		fmt.Fprintf(&sb, "%s: %s", s.Name, s.State)
		if s.Err != "" {
			fmt.Fprintf(&sb, " (%s)", s.Err)
		}
		sb.WriteString("\n")
		for _, t := range tools {
			if t.Server == s.Name {
				fmt.Fprintf(&sb, "  - %s\n", t.Name)
			}
		}
	}
	return strings.TrimSpace(sb.String())
}

// reloadMCP recarrega em segundo plano (subir um servidor leva segundos).
func (m *Model) reloadMCP(quiet bool) tea.Cmd {
	if m.mcpMgr == nil || m.mcpReloading {
		return nil
	}
	m.mcpReloading = true
	mgr := m.mcpMgr
	return func() tea.Msg {
		return mcpReloadedMsg{status: mgr.Reload(context.Background()), quiet: quiet}
	}
}

func (m *Model) onMCPReloaded(msg mcpReloadedMsg) {
	m.mcpReloading = false
	m.ide.mcpOn, m.ide.mcpTools = 0, 0
	for _, s := range msg.status {
		if s.State == "conectado" {
			m.ide.mcpOn++
			m.ide.mcpTools += s.Tools
		}
	}
	if msg.quiet && !hasMCPFailure(msg.status) {
		return
	}
	m.addMessage(m.mcpReport(msg.status))
	m.addMessage("")
}

func hasMCPFailure(st []mcpclient.Status) bool {
	for _, s := range st {
		if s.State == "falhou" {
			return true
		}
	}
	return false
}

func (m *Model) mcpReport(st []mcpclient.Status) string {
	if len(st) == 0 {
		return renderNotice("◆", "Nenhum servidor MCP configurado. Adicione com /mcp add <nome> (ex.: /mcp add context7).", colGraphite)
	}
	var sb strings.Builder
	sb.WriteString(renderNotice("◆", "Servidores MCP", colSignal))
	for _, s := range st {
		var mark string
		switch s.State {
		case "conectado":
			mark = renderNotice("●", fmt.Sprintf("%-14s %d ferramentas", s.Name, s.Tools), colEmber)
			if s.Trusted {
				mark += renderNotice("", "  confiável", colGraphite)
			}
		case "falhou":
			mark = renderNotice("✗", fmt.Sprintf("%-14s %s", s.Name, firstLine(s.Err)), colAlarm)
		default:
			mark = renderNotice("○", fmt.Sprintf("%-14s %s", s.Name, s.State), colGraphite)
		}
		sb.WriteString("\n  " + mark)
	}
	sb.WriteString("\n" + renderNotice("", "  /mcp recarregar · ligar|desligar|confiar|ferramentas <nome>", colGraphite))
	return sb.String()
}

func firstLine(s string) string {
	if i := strings.IndexByte(s, '\n'); i >= 0 {
		s = s[:i]
	}
	if len([]rune(s)) > 90 {
		s = string([]rune(s)[:89]) + "…"
	}
	return s
}

// handleMCPCommand trata "/mcp ...". Devolve false se não for esse comando.
func (m *Model) handleMCPCommand(input string) bool {
	campos := strings.Fields(input)
	if len(campos) == 0 || strings.ToLower(campos[0]) != "/mcp" {
		return false
	}
	if m.mcpMgr == nil {
		m.initMCP()
	}
	sub := ""
	if len(campos) > 1 {
		sub = strings.ToLower(campos[1])
	}
	nome := ""
	if len(campos) > 2 {
		nome = campos[2]
	}

	switch sub {
	case "", "status", "lista":
		m.addMessage(m.mcpReport(m.mcpMgr.Statuses()))
		m.addMessage("")
	case "add", "adicionar", "remover", "remove":
		m.mcpAddRemove(sub, campos[2:])
	case "recarregar", "reload", "reiniciar", "source":
		m.addMessage(renderNotice("↻", "Recarregando servidores MCP (relê a configuração e reinicia todos)…", colGraphite))
		m.pendingCmd = tea.Batch(m.pendingCmd, m.reloadMCP(false))
	case "ligar", "desligar", "confiar", "desconfiar":
		if nome == "" {
			m.addMessage(renderNotice("!", "Informe o servidor: /mcp "+sub+" <nome>", colAct))
			break
		}
		var err error
		switch sub {
		case "ligar":
			err = m.mcpMgr.SetEnabled(context.Background(), nome, true)
		case "desligar":
			err = m.mcpMgr.SetEnabled(context.Background(), nome, false)
		case "confiar":
			err = m.mcpMgr.SetTrusted(nome, true)
		default:
			err = m.mcpMgr.SetTrusted(nome, false)
		}
		if err != nil {
			m.addMessage(renderNotice("✗", err.Error(), colAlarm))
		} else {
			m.addMessage(m.mcpReport(m.mcpMgr.Statuses()))
		}
		m.addMessage("")
	case "ferramentas", "tools":
		var linhas []string
		for _, t := range m.mcpMgr.Tools() {
			if nome == "" || strings.EqualFold(t.Server, nome) {
				ro := ""
				if t.ReadOnly {
					ro = " (leitura)"
				}
				linhas = append(linhas, fmt.Sprintf("  %s%s — %s", t.Name, ro, firstLine(t.Description)))
			}
		}
		sort.Strings(linhas)
		if len(linhas) == 0 {
			linhas = []string{"  (nenhuma ferramenta)"}
		}
		m.addMessage(strings.Join(linhas, "\n"))
		m.addMessage("")
	default:
		m.addMessage(renderNotice("!", "Uso: /mcp [add|remover|recarregar|ligar|desligar|confiar|desconfiar|ferramentas] [servidor]", colAct))
	}
	m.viewport.GotoBottom()
	return true
}

// mcpAddRemove trata "/mcp add ..." e "/mcp remover <nome>" e recarrega.
func (m *Model) mcpAddRemove(sub string, args []string) {
	if sub == "add" || sub == "adicionar" {
		cfg, projeto, err := mcpclient.ParseAddArgs(args)
		if err == nil {
			path := mcpclient.GlobalConfigPath()
			if projeto {
				path = mcpclient.ProjectConfigPath(m.workspaceRoot)
			}
			if err = mcpclient.AddServer(path, cfg); err == nil {
				m.addMessage(renderNotice("✓", fmt.Sprintf("Servidor %q adicionado em %s. Conectando…", cfg.Name, path), colEmber))
				m.pendingCmd = tea.Batch(m.pendingCmd, m.reloadMCP(false))
			}
		}
		if err != nil {
			m.addMessage(renderNotice("✗", err.Error(), colAlarm))
		}
	} else if len(args) == 0 {
		m.addMessage(renderNotice("!", "Informe o servidor: /mcp remover <nome>", colAct))
	} else {
		achou := false
		for _, p := range mcpclient.ConfigPaths(m.workspaceRoot) {
			if ok, _ := mcpclient.RemoveServer(p, args[0]); ok {
				achou = true
			}
		}
		if achou {
			m.addMessage(renderNotice("✓", fmt.Sprintf("Servidor %q removido.", args[0]), colEmber))
			m.pendingCmd = tea.Batch(m.pendingCmd, m.reloadMCP(false))
		} else {
			m.addMessage(renderNotice("✗", fmt.Sprintf("Servidor %q não encontrado na configuração do Amazzo.", args[0]), colAlarm))
		}
	}
	m.addMessage("")
}

// toolSchemas junta as ferramentas internas com as dos servidores MCP.
func (m *Model) toolSchemas() []agents.Tool {
	base := agents.DefaultTools()
	if m.mcpMgr == nil {
		return base
	}
	base = append(base, mcpAddTool(), mcpRemoveTool())
	ext := m.mcpMgr.Tools()
	if len(ext) == 0 && len(m.mcpMgr.Statuses()) == 0 {
		return base
	}
	base = append(base, agents.Tool{Type: agents.ToolTypeFunction, Function: agents.Function{
		Name: "mcp_reload",
		Description: "Relê a configuração e reinicia todos os servidores MCP desta sessão. " +
			"Use quando uma ferramenta de servidor falhar, sumir, ou o usuário disser que atualizou/recompilou um servidor.",
		Parameters: map[string]interface{}{"type": "object", "properties": map[string]interface{}{}},
	}})
	// Servidores pequenos entram inteiros; os grandes ficam atrás de
	// mcp_search/mcp_call, que custam poucos tokens.
	porServidor := map[string][]mcpclient.Tool{}
	var nomes []string
	for _, t := range ext {
		if _, ok := porServidor[t.Server]; !ok {
			nomes = append(nomes, t.Server)
		}
		porServidor[t.Server] = append(porServidor[t.Server], t)
	}
	sort.SliceStable(nomes, func(i, j int) bool { return len(porServidor[nomes[i]]) < len(porServidor[nomes[j]]) })

	gasto, adiados := 0, false
	for _, nome := range nomes {
		var tools []agents.Tool
		for _, t := range porServidor[nome] {
			tools = append(tools, agents.Tool{Type: agents.ToolTypeFunction, Function: agents.Function{
				Name:        t.Name,
				Description: cortaTexto("["+t.Server+"] "+t.Description, 220),
				Parameters:  esquemaEnxuto(t.Schema),
			}})
		}
		b, _ := json.Marshal(tools)
		custo := estimateTokens(string(b))
		if gasto+custo > orcamentoSchemasExternos {
			adiados = true
			continue
		}
		gasto += custo
		base = append(base, tools...)
	}
	if adiados {
		base = append(base,
			agents.Tool{Type: agents.ToolTypeFunction, Function: agents.Function{
				Name: "mcp_search",
				Description: "Procura ferramentas de servidores MCP que não estão na lista (servidores grandes). " +
					"Devolve nome e parâmetros; depois use mcp_call.",
				Parameters: map[string]interface{}{"type": "object", "properties": map[string]interface{}{
					"query": map[string]interface{}{"type": "string", "description": "palavras-chave, ex.: apk manifest"}}},
			}},
			agents.Tool{Type: agents.ToolTypeFunction, Function: agents.Function{
				Name:        "mcp_call",
				Description: "Chama uma ferramenta de servidor MCP achada com mcp_search.",
				Parameters: map[string]interface{}{"type": "object", "properties": map[string]interface{}{
					"tool":      map[string]interface{}{"type": "string", "description": "nome completo da ferramenta"},
					"arguments": map[string]interface{}{"type": "object", "description": "argumentos da ferramenta"}},
					"required": []string{"tool"}},
			}})
	}
	return base
}

func mcpAddTool() agents.Tool {
	str := func(d string) map[string]interface{} { return map[string]interface{}{"type": "string", "description": d} }
	obj := func(d string) map[string]interface{} { return map[string]interface{}{"type": "object", "description": d} }
	return agents.Tool{Type: agents.ToolTypeFunction, Function: agents.Function{
		Name: "mcp_add",
		Description: "Adiciona um servidor MCP à configuração do Amazzo e conecta na hora; as ferramentas ficam disponíveis em seguida. " +
			"Use quando o usuário pedir. Para um projeto que já define servidores MCP (ex.: .mcp.json, opencode.json), leia o arquivo e adicione cada um com scope=project. " +
			"Nunca copie configs de outras ferramentas sem o usuário pedir.",
		Parameters: map[string]interface{}{"type": "object", "properties": map[string]interface{}{
			"name":    str("nome do servidor (ex.: context7)"),
			"command": str("executável, para servidor local (stdio)"),
			"args":    map[string]interface{}{"type": "array", "items": map[string]interface{}{"type": "string"}, "description": "argumentos do comando"},
			"url":     str("endereço, para servidor remoto (HTTP)"),
			"env":     obj("variáveis de ambiente"),
			"headers": obj("cabeçalhos HTTP"),
			"scope":   str("project (só esta pasta) ou global (padrão)"),
		}, "required": []string{"name"}},
	}}
}

func mcpRemoveTool() agents.Tool {
	return agents.Tool{Type: agents.ToolTypeFunction, Function: agents.Function{
		Name:        "mcp_remove",
		Description: "Remove um servidor MCP da configuração do Amazzo.",
		Parameters: map[string]interface{}{"type": "object", "properties": map[string]interface{}{
			"name": map[string]interface{}{"type": "string"}}, "required": []string{"name"}},
	}}
}

// textoDosSchemasExternos mede o que as ferramentas externas custam no prompt.
func (m *Model) textoDosSchemasExternos() string {
	if m.mcpMgr == nil {
		return ""
	}
	ext := m.toolSchemas()[len(agents.DefaultTools()):]
	b, _ := json.Marshal(ext)
	return string(b)
}

func cortaTexto(s string, max int) string {
	s = strings.Join(strings.Fields(s), " ")
	r := []rune(s)
	if len(r) > max {
		return string(r[:max-1]) + "…"
	}
	return s
}

// esquemaEnxuto garante um schema válido e corta descrições longas: cada
// token de schema é um token a menos de contexto num modelo de 12k.
func esquemaEnxuto(s map[string]interface{}) map[string]interface{} {
	if s == nil {
		return map[string]interface{}{"type": "object", "properties": map[string]interface{}{}}
	}
	out := map[string]interface{}{}
	for k, v := range s {
		switch x := v.(type) {
		case map[string]interface{}:
			out[k] = esquemaEnxuto(x)
		case string:
			if k == "description" {
				x = cortaTexto(x, 120)
			}
			out[k] = x
		default:
			out[k] = v
		}
	}
	if _, ok := out["type"]; !ok && out["properties"] != nil {
		out["type"] = "object"
	}
	if t, _ := out["type"].(string); t == "object" && out["properties"] == nil {
		out["properties"] = map[string]interface{}{}
	}
	return out
}

// motivoExterno devolve por que uma ferramenta externa pede permissão.
func (m *Model) motivoExterno(req mcp.Request) string {
	if m.mcpMgr == nil {
		return ""
	}
	alvo := req.Tool
	if alvo == "mcp_add" {
		cmd, _ := req.Args["command"].(string)
		if u, _ := req.Args["url"].(string); u != "" {
			cmd = u
		}
		return fmt.Sprintf("adiciona o servidor MCP %v (%s), que será executado/conectado", req.Args["name"], cmd)
	}
	if alvo == "mcp_remove" {
		return ""
	}
	if alvo == "mcp_call" {
		alvo, _ = req.Args["tool"].(string)
	}
	if pede, motivo := m.mcpMgr.NeedsApproval(alvo); pede {
		return motivo + " (use /mcp confiar <servidor> para não perguntar mais)"
	}
	return ""
}

func descreveExterna(req mcp.Request) string {
	b, _ := json.Marshal(req.Args)
	return req.Tool + " " + cortaTexto(string(b), 160)
}

// buscaFerramentasMCP procura nas ferramentas dos servidores que ficaram de
// fora do prompt e devolve nome, descrição e parâmetros.
func buscaFerramentasMCP(mgr *mcpclient.Manager, query string) string {
	termos := strings.Fields(strings.ToLower(query))
	type achado struct {
		pontos int
		texto  string
	}
	var achados []achado
	for _, t := range mgr.Tools() {
		alvo := strings.ToLower(t.Name + " " + t.Description)
		pontos := 0
		for _, termo := range termos {
			if strings.Contains(strings.ToLower(t.Name), termo) {
				pontos += 3
			} else if strings.Contains(alvo, termo) {
				pontos++
			}
		}
		if len(termos) > 0 && pontos == 0 {
			continue
		}
		var params []string
		if props, ok := t.Schema["properties"].(map[string]interface{}); ok {
			for k := range props {
				params = append(params, k)
			}
			sort.Strings(params)
		}
		achados = append(achados, achado{pontos, fmt.Sprintf("- %s(%s): %s", t.Name,
			strings.Join(params, ", "), cortaTexto(t.Description, 160))})
	}
	sort.SliceStable(achados, func(i, j int) bool { return achados[i].pontos > achados[j].pontos })
	if len(achados) == 0 {
		return "Nenhuma ferramenta encontrada. Tente outra palavra, ou mcp_list para ver os servidores."
	}
	if len(achados) > 12 {
		achados = achados[:12]
	}
	var sb strings.Builder
	sb.WriteString("Use mcp_call com tool=<nome> e arguments={...}.\n")
	for _, a := range achados {
		sb.WriteString(a.texto + "\n")
	}
	return sb.String()
}

// orcamentoSchemasExternos é quanto do contexto as ferramentas de servidores
// MCP podem ocupar. Um servidor grande pode expor 200+ ferramentas: mandar todas
// estouraria a janela de 12k de um modelo local.
const orcamentoSchemasExternos = 1500
