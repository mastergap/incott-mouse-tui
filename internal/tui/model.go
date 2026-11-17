package tui

import (
	"context"
	"runtime"
	"strconv"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"

	"github.com/mastergap/incott-mouse-tui/internal/device"
	"github.com/mastergap/incott-mouse-tui/internal/protocol"
	"github.com/mastergap/incott-mouse-tui/internal/state"
)

// Layout thresholds.
const (
	minWidth      = 60
	minHeight     = 12
	compactBelowH = 24 // status panel collapses to one line below this height
)

// Timing.
const (
	applyDelay  = 150 * time.Millisecond // debounce for cycling/sliding
	applyLimit  = 5 * time.Second
	toastLength = 3 * time.Second
)

// Backend is what the UI needs from the device layer. *device.Client implements it.
type Backend interface {
	Events() <-chan state.State
	State() state.State
	Apply(ctx context.Context, s device.Setting) error
	Refresh(ctx context.Context) error
}

// DemoHooks lets the model drive a fake mouse in --demo mode (nil in normal operation).
type DemoHooks struct {
	TogglePlug       func()
	TogglePermission func()
}

// Model is the root Bubble Tea model.
type Model struct {
	backend Backend
	demo    *DemoHooks
	goos    string
	delay   time.Duration

	st            state.State
	set           settings
	toast         toast
	help          bool
	helpScroll    int
	frame         int // spinner frame while connecting
	width, height int
}

// New creates the root model.
func New(b Backend, demo *DemoHooks) Model {
	m := Model{backend: b, demo: demo, goos: runtime.GOOS, delay: applyDelay, st: b.State(), set: newSettings()}
	m.set.sync(m.st)
	return m
}

type (
	stateMsg   state.State
	stoppedMsg struct{}
	refreshMsg struct{ err error }
	spinMsg    struct{}
)

const spinInterval = 100 * time.Millisecond

func spinTick() tea.Cmd { return tea.Tick(spinInterval, func(time.Time) tea.Msg { return spinMsg{} }) }

func waitState(b Backend) tea.Cmd {
	return func() tea.Msg {
		s, ok := <-b.Events()
		if !ok {
			return stoppedMsg{}
		}
		return stateMsg(s)
	}
}

func applyCmd(b Backend, s device.Setting) tea.Cmd {
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), applyLimit)
		defer cancel()
		return applyResultMsg{kind: s.Kind, value: s.Value, err: b.Apply(ctx, s)}
	}
}

// Init starts listening for device state.
func (m Model) Init() tea.Cmd { return waitState(m.backend) }

// Update handles messages.
func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width, m.height = msg.Width, msg.Height

	case stateMsg:
		return m.onState(state.State(msg))

	case spinMsg:
		if m.st.Connection == state.Connecting {
			m.frame++
			return m, spinTick()
		}

	case stoppedMsg:
		return m, tea.Quit

	case applyDueMsg:
		if p, ok := m.set.pending[msg.kind]; ok && p.seq == msg.seq {
			return m, applyCmd(m.backend, device.Setting{Kind: msg.kind, Value: p.value})
		}

	case applyResultMsg:
		return m.onApplyResult(msg)

	case refreshMsg:
		m.st = m.backend.State()
		if msg.err != nil {
			return m.withToast(describeError(0, msg.err), true)
		}
		return m.withToast("Settings refreshed", false)

	case toastExpireMsg:
		if msg.id == m.toast.id {
			m.toast = toast{id: m.toast.id}
		}

	case tea.KeyMsg:
		return m.onKey(msg)

	case tea.MouseMsg:
		return m.onMouse(msg)
	}
	return m, nil
}

// onState adopts a new device state and announces connection changes.
func (m Model) onState(next state.State) (tea.Model, tea.Cmd) {
	prev := m.st.Connection
	m.st = next
	if next.Connection != state.Connected {
		m.set.reset() // nothing can be edited without a mouse; drop unconfirmed values
	}
	m.set.sync(next)
	m.set.ensureCursor(next)

	cmds := []tea.Cmd{waitState(m.backend)}
	if next.Connection == state.Connecting && prev != state.Connecting {
		m.frame = 0
		cmds = append(cmds, spinTick())
	}
	switch {
	case next.Connection == state.Connected && prev != state.Connected:
		name := next.Name
		if name == "" {
			name = "mouse"
		}
		var c tea.Cmd
		nm, c := m.withToast("Connected to "+name, false)
		m, cmds = nm.(Model), append(cmds, c)
	case prev == state.Connected && next.Connection != state.Connected:
		var c tea.Cmd
		nm, c := m.withToast("Mouse disconnected", true)
		m, cmds = nm.(Model), append(cmds, c)
	}
	return m, tea.Batch(cmds...)
}

// bodyGeometry returns the screen row where the body content starts and its height.
func (m Model) bodyGeometry() (top, inner int) {
	statusH := lipgloss.Height(statusPanel(m.st, m.width, m.height < compactBelowH))
	return statusH + 1, max(m.height-statusH-1-2, 1) // border row, then content
}

// helpContent returns the wrapped help lines and the visible height of the panel.
func (m Model) helpContent() (lines []string, inner int) {
	_, inner = m.bodyGeometry()
	return helpLines(m.width - 4), inner
}

func (m Model) helpPage() int {
	_, inner := m.helpContent()
	return max(inner-1, 1)
}

// scrollHelp moves the help view by n lines, clamped to the content.
func (m *Model) scrollHelp(n int) {
	lines, inner := m.helpContent()
	m.helpScroll = min(max(m.helpScroll+n, 0), max(len(lines)-inner, 0))
}

func (m Model) onMouse(msg tea.MouseMsg) (tea.Model, tea.Cmd) {
	if m.width == 0 || msg.Action != tea.MouseActionPress {
		return m, nil
	}
	if m.help {
		switch msg.Button {
		case tea.MouseButtonLeft:
			m.help = false
		case tea.MouseButtonWheelUp:
			m.scrollHelp(-3)
		case tea.MouseButtonWheelDown:
			m.scrollHelp(3)
		}
		return m, nil
	}
	if m.st.Connection != state.Connected {
		return m, nil
	}
	switch msg.Button {
	case tea.MouseButtonWheelUp:
		return m, m.set.wheel(true, m.st, m.delay)
	case tea.MouseButtonWheelDown:
		return m, m.set.wheel(false, m.st, m.delay)
	case tea.MouseButtonLeft:
		top, inner := m.bodyGeometry()
		row := msg.Y - top
		if row < 0 || row >= inner {
			return m, nil
		}
		lines, focus, at := m.set.view(m.st)
		li := windowStart(len(lines), focus, inner) + row
		if li >= len(at) || at[li] < 0 {
			return m, nil
		}
		if m.set.picker != nil {
			m.set.picker.cursor = at[li]
			cmd, _ := m.set.onKey("enter", m.st, m.delay)
			return m, cmd
		}
		return m, m.set.click(at[li], msg.X-2, m.st, m.delay) // 2 = border + padding
	}
	return m, nil
}

func (m Model) onApplyResult(r applyResultMsg) (tea.Model, tea.Cmd) {
	// The backend publishes the new state before Apply returns; adopt it now so the value
	// does not flicker back while the event travels through the stream.
	m.st = m.backend.State()
	if p, ok := m.set.pending[r.kind]; ok && p.value == r.value {
		delete(m.set.pending, r.kind) // confirmed, or rolled back to the device value on error
	}
	if r.err != nil {
		if r.kind == device.KindLOD && r.value == protocol.LOD07mm && m.st.LOD07Unsupported {
			m.set.sync(m.st)
			return m.withToast("This mouse does not support 0.7 mm — staying at "+protocol.LODLabel(m.st.LOD.Val), true)
		}
		return m.withToast(describeError(r.kind, r.err), true)
	}
	label := r.kind.String()
	for _, it := range m.set.items {
		if it.kind == r.kind {
			return m.withToast(label+" → "+it.format(r.value), false)
		}
	}
	return m, nil
}

func (m Model) withToast(text string, isErr bool) (tea.Model, tea.Cmd) {
	id := m.toast.id + 1
	m.toast = toast{id: id, text: text, err: isErr}
	return m, tea.Tick(toastLength, func(time.Time) tea.Msg { return toastExpireMsg{id: id} })
}

func (m Model) onKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	key := msg.String()

	if m.help { // the help page swallows keys: scroll, or close
		switch key {
		case "ctrl+c":
			return m, tea.Quit
		case "?", "esc", "q", "enter":
			m.help = false
		case "up", "k":
			m.scrollHelp(-1)
		case "down", "j":
			m.scrollHelp(1)
		case "pgup", "b":
			m.scrollHelp(-m.helpPage())
		case "pgdown", "f", " ":
			m.scrollHelp(m.helpPage())
		case "home", "g":
			m.helpScroll = 0
		case "end", "G":
			m.scrollHelp(1 << 20)
		}
		return m, nil
	}
	if key == "?" {
		m.help, m.helpScroll = true, 0
		return m, nil
	}

	// Editor first, but only while there is a mouse to edit.
	if m.st.Connection == state.Connected {
		if cmd, handled := m.set.onKey(key, m.st, m.delay); handled {
			return m, cmd
		}
	}

	switch key {
	case "q", "ctrl+c":
		return m, tea.Quit
	case "r":
		b := m.backend
		return m, func() tea.Msg {
			ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			defer cancel()
			return refreshMsg{err: b.Refresh(ctx)}
		}
	case "u":
		if m.demo != nil {
			m.demo.TogglePlug()
		}
	case "x":
		if m.demo != nil {
			m.demo.TogglePermission()
		}
	}
	return m, nil
}

// View renders the screen.
func (m Model) View() string {
	if m.width == 0 {
		return ""
	}
	if m.width < minWidth || m.height < minHeight {
		return styleWarn.Render("Terminal too small") + "\n" +
			styleDim.Render("need at least 60x12, have "+strconv.Itoa(m.width)+"x"+strconv.Itoa(m.height))
	}

	status := statusPanel(m.st, m.width, m.height < compactBelowH)
	bodyH := m.height - lipgloss.Height(status) - 1 // footer line
	return lipgloss.JoinVertical(lipgloss.Left, status, m.bodyPanel(bodyH), m.footer())
}

// bodyPanel is the settings editor, or guidance when no mouse is usable.
func (m Model) bodyPanel(height int) string {
	inner := max(height-2, 1)
	var lines []string
	title := "Settings"
	switch {
	case m.help:
		all := helpLines(m.width - 4)
		start := min(max(m.helpScroll, 0), max(len(all)-inner, 0))
		lines = all[start:min(start+inner, len(all))]
		title = "Help"
		if len(all) > inner {
			title += " · " + strconv.Itoa((start+inner)*100/len(all)) + "%"
		}
	case m.st.Connection == state.Connected:
		var focus int
		lines, focus, _ = m.set.view(m.st)
		lines = window(lines, focus, inner)
		if m.set.picker != nil {
			title = "Pick a value"
		}
	default:
		lines = hint(m.st, m.goos, m.frame)
	}
	for len(lines) < inner {
		lines = append(lines, "")
	}
	return box(styleTitle.Render(title), "", lines[:inner], m.width, styleBorder)
}

// footer shows contextual key hints on the left and the latest toast on the right.
// On narrow terminals contextual hints are dropped from the end until the global keys fit.
func (m Model) footer() string {
	var ctx [][2]string
	if m.st.Connection == state.Connected && !m.help {
		ctx = m.set.keyHints()
	}
	var global [][2]string
	switch {
	case m.help:
		global = append(global, [2]string{"↑↓ PgUp PgDn", "scroll"}, [2]string{"esc", "close help"})
	case m.set.picker == nil:
		global = append(global, [2]string{"?", "help"}, [2]string{"r", "refresh"}, [2]string{"q", "quit"})
	}
	if m.demo != nil {
		global = append(global, [2]string{"u", "demo: unplug/plug"}, [2]string{"x", "demo: permission error"})
	}
	right := m.toast.render()
	reserve := 0
	if right != "" {
		reserve = lipgloss.Width(right) + 3
	}
	left := ""
	for n := len(ctx); n >= 0; n-- {
		left = joinHints(append(append([][2]string{}, ctx[:n]...), global...))
		if lipgloss.Width(left)+reserve <= m.width {
			break
		}
	}
	left = ansi.Truncate(left, m.width, "…")
	if right == "" {
		return left
	}
	gap := m.width - lipgloss.Width(left) - lipgloss.Width(right) - 1
	if gap < 2 {
		return " " + right // not enough room for both: the message wins
	}
	return left + strings.Repeat(" ", gap) + right
}

func joinHints(items [][2]string) string {
	parts := make([]string, len(items))
	for i, it := range items {
		parts[i] = styleKey.Render(it[0]) + " " + styleDim.Render(it[1])
	}
	return " " + strings.Join(parts, styleDim.Render(" · "))
}
