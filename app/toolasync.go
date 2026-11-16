package app

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"amazzo/mcp"

	tea "github.com/charmbracelet/bubbletea"
)

// toolsDoneMsg chega quando um lote de ferramentas termina de rodar.
//
// As ferramentas rodam fora do laço principal: um "go build" de 30 s (ou um
// servidor que demora) não pode congelar a tela. Enquanto isso o campo de
// digitação, a barra de status e o Esc continuam funcionando.
type toolsDoneMsg struct {
	requests []mcp.Request
	results  []mcp.Result
	pre      []mcp.Result // resultados já prontos (abrir pasta, operações leves)
	err      error
}

// startTools dispara o lote em segundo plano e devolve o comando do Bubble Tea.
func (m *Model) startTools(requests []mcp.Request, pre []mcp.Result) tea.Cmd {
	// set_workspace muda o alcance do agente, não um arquivo: é decisão do
	// app, e é instantâneo.
	requests, abriuPasta := m.aplicaAberturaDePasta(requests)
	pre = append(pre, abriuPasta...)

	if len(requests) == 0 {
		return func() tea.Msg { return toolsDoneMsg{pre: pre} }
	}

	// O teto comporta a soma dos passos de um lote (build + teste).
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	m.toolsCancel = cancel
	m.toolsRunning = true
	m.toolsStart = time.Now()
	m.toolsLabel = rotuloDoLote(requests)
	m.loading = true

	srv := m.mcpServer
	return tea.Batch(m.spinner.Tick, func() tea.Msg {
		defer cancel()
		res, err := srv.Handle(ctx, requests)
		return toolsDoneMsg{requests: requests, results: res, pre: pre, err: err}
	})
}

// finishTools mostra os resultados e devolve o turno ao modelo.
func (m *Model) finishTools(msg toolsDoneMsg) tea.Cmd {
	m.toolsRunning = false
	m.toolsCancel = nil

	// Esc durante a execução: o turno já foi encerrado, só registra.
	cancelado := !m.loading && m.toolsInterrupted
	m.toolsInterrupted = false

	if msg.err != nil && !errors.Is(msg.err, context.Canceled) {
		m.addMessage(renderNotice("✗", fmt.Sprintf("erro ao executar ferramentas: %v", msg.err), colAlarm))
		m.addMessage("")
	}

	for i, res := range msg.results {
		m.renderToolResult(res)
		m.recordIfCommand(msg.requests, i, res)
	}

	todos := append(append([]mcp.Result{}, msg.pre...), msg.results...)
	if cancelado {
		// Mantém o histórico coerente: o modelo vê o que rodou antes do Esc.
		if len(todos) > 0 {
			m.registraResultadosDeFerramenta(todos)
		}
		return nil
	}
	if len(todos) == 0 {
		m.toolRuns = 0
		m.loading = false
		return nil
	}

	m.loading = true
	m.addMessage(renderNotice("↻", fmt.Sprintf("ciclo %d/%d — devolvendo o resultado ao modelo", m.toolRuns, 15), colGraphite))
	return tea.Batch(m.spinner.Tick, m.continuaAposFerramentas(todos))
}

// interrompeFerramentas cancela o lote em andamento (Esc).
func (m *Model) interrompeFerramentas() bool {
	if !m.toolsRunning || m.toolsCancel == nil {
		return false
	}
	m.toolsCancel()
	m.toolsInterrupted = true
	m.loading = false
	m.addMessage(renderNotice("■", "Execução interrompida.", colGraphite))
	m.addMessage("")
	return true
}

// rotuloDoLote resume o que está rodando, para a linha de progresso.
func rotuloDoLote(reqs []mcp.Request) string {
	if len(reqs) == 0 {
		return ""
	}
	r := reqs[0]
	var alvo string
	switch {
	case r.Command != "":
		alvo = r.Command
	case r.Path != "":
		alvo = r.Path
	}
	rotulo := strings.ToLower(r.Tool)
	if alvo != "" {
		rotulo += " " + alvo
	}
	if len(reqs) > 1 {
		rotulo += fmt.Sprintf(" (+%d)", len(reqs)-1)
	}
	if len([]rune(rotulo)) > 60 {
		rotulo = string([]rune(rotulo)[:59]) + "…"
	}
	return rotulo
}

// palavrasDeEspera são os verbos que giram enquanto o modelo trabalha, no
// estilo do Claude Code: dizem que há vida sem fingir que sabem a etapa.
var palavrasDeEspera = []string{
	"pensando", "assimilando", "carregando", "refletindo", "analisando",
	"ruminando", "conectando ideias", "decifrando", "processando",
	"mastigando", "cogitando", "organizando", "destrinchando", "calculando",
	"investigando", "montando o quebra-cabeça", "ponderando", "digerindo",
}

// palavraDeEspera escolhe o verbo da vez. Troca a cada 3 s e a ordem muda de
// um turno para outro (semente = início do turno), então não vira um relógio.
func palavraDeEspera(inicio time.Time, agora time.Time) string {
	n := len(palavrasDeEspera)
	passo := int(agora.Sub(inicio) / (3 * time.Second))
	semente := int(inicio.UnixNano()/1e6) % n
	if semente < 0 {
		semente = -semente
	}
	// 7 é primo com 18: percorre todos os verbos sem repetir antes da hora.
	return palavrasDeEspera[(semente+passo*7)%n] + "…"
}

// toolsPreview é a linha de progresso enquanto as ferramentas rodam.
func (m *Model) toolsPreview(width int) string {
	return entry{
		rail:  colAct,
		label: m.spinner.View() + " executando",
		meta:  formatElapsed(time.Since(m.toolsStart)),
		body:  m.toolsLabel + "\nesc interrompe",
		width: width,
	}.render()
}

// executeToolsAsync separa o que pode rodar já do que precisa de permissão.
// Comandos de shell nunca rodam dentro do laço principal: se houver pedido
// sujeito a confirmação, os comandos "livres" esperam junto com ele.
func (m *Model) executeToolsAsync(requests []mcp.Request) (tea.Cmd, bool) {
	if len(requests) == 0 {
		return nil, false
	}

	immediate, privileged := m.splitRequests(requests)

	if len(privileged) > 0 {
		var leves []mcp.Request
		for _, r := range immediate {
			if ehComando(r.Tool) {
				privileged = append(privileged, r)
			} else {
				leves = append(leves, r)
			}
		}
		m.pendingRequests = append([]mcp.Request(nil), privileged...)
		m.awaitingApproval = true
		m.pendingResults = nil
		if len(leves) > 0 {
			m.pendingResults = append(m.pendingResults, m.runMCPRequests(leves)...)
		}

		m.addMessage(m.theme.HelpStyle.Render("⚠️ Operações MCP aguardando confirmação:"))
		m.addMessage(m.describePendingRequests(privileged))
		m.addMessage(m.theme.HelpStyle.Render("Digite 'permitir' para continuar ou 'negar' para cancelar."))
		m.addMessage("")
		return nil, true
	}

	if len(immediate) == 0 {
		return nil, false
	}
	return m.startTools(immediate, nil), false
}

func ehComando(tool string) bool {
	switch strings.ToLower(tool) {
	case "execute_command", "exec", "run":
		return true
	}
	return false
}
