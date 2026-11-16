package app

import (
	"fmt"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
)

// Relógios da barra de status:
//   - conversa: tempo desde a primeira mensagem da conversa;
//   - tarefa: tempo que o modelo passou trabalhando desde o último pedido,
//     sem contar a espera por uma autorização sua.

type clockTickMsg time.Time

func clockCmd() tea.Cmd {
	return tea.Tick(time.Second, func(t time.Time) tea.Msg { return clockTickMsg(t) })
}

// clockState vive dentro de ideState.
type clockState struct {
	convStart time.Time
	taskBusy  time.Duration
	lastTick  time.Time
}

// startTask marca o início de um pedido novo.
func (m *Model) startTask() {
	now := time.Now()
	if m.ide.clock.convStart.IsZero() {
		m.ide.clock.convStart = now
	}
	m.ide.clock.taskBusy = 0
	m.ide.clock.lastTick = now
}

// resetClock zera os dois relógios (nova conversa).
func (m *Model) resetClock() { m.ide.clock = clockState{} }

// onClockTick acumula o tempo de trabalho enquanto o modelo está ocupado.
func (m *Model) onClockTick(t time.Time) {
	c := &m.ide.clock
	if m.loading && !c.lastTick.IsZero() {
		c.taskBusy += t.Sub(c.lastTick)
	}
	c.lastTick = t
}

// taskElapsed é o tempo da tarefa, incluindo o trecho desde o último tick.
func (m *Model) taskElapsed() time.Duration {
	c := m.ide.clock
	d := c.taskBusy
	if m.loading && !c.lastTick.IsZero() {
		d += time.Since(c.lastTick)
	}
	return d
}

func (m *Model) clockSegments() []string {
	c := m.ide.clock
	if c.convStart.IsZero() {
		return nil
	}
	segs := []string{"conversa " + clockFmt(time.Since(c.convStart))}
	if t := m.taskElapsed(); m.loading {
		segs = append(segs, "tarefa "+clockFmt(t)+" ●")
	} else if t > 0 {
		segs = append(segs, "última tarefa "+clockFmt(t))
	}
	return segs
}

// clockFmt formata como 45s, 12m05s ou 1h02m.
func clockFmt(d time.Duration) string {
	s := int(d.Seconds())
	switch {
	case s < 60:
		return fmt.Sprintf("%ds", s)
	case s < 3600:
		return fmt.Sprintf("%dm%02ds", s/60, s%60)
	default:
		return fmt.Sprintf("%dh%02dm", s/3600, (s%3600)/60)
	}
}

// Texto colado muito grande: o que passa de pasteLimit caracteres vira início
// + fim. Cada 4 caracteres custam cerca de 1 token, e o contexto é pequeno.
const (
	pasteLimit = 6000
	pasteHead  = 4000
	pasteTail  = 1500
)

// limitPaste devolve o texto a enviar ao modelo e um aviso se houve corte.
func limitPaste(input string) (string, string) {
	r := []rune(input)
	if len(r) <= pasteLimit {
		return input, ""
	}
	cortados := len(r) - pasteHead - pasteTail
	out := string(r[:pasteHead]) +
		fmt.Sprintf("\n\n[... %d caracteres omitidos do meio do texto ...]\n\n", cortados) +
		string(r[len(r)-pasteTail:])
	aviso := fmt.Sprintf("Texto grande (%d caracteres): enviei o começo e o fim, omiti %d do meio. "+
		"Use @arquivo para o modelo ler o conteúdo completo sob demanda.", len(r), cortados)
	return out, aviso
}

var _ = strings.TrimSpace
