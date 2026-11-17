package tui

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/muesli/termenv"

	"github.com/mastergap/incott-mouse-tui/internal/device"
	"github.com/mastergap/incott-mouse-tui/internal/protocol"
	"github.com/mastergap/incott-mouse-tui/internal/state"
)

func init() { lipgloss.SetColorProfile(termenv.Ascii) } // plain text: stable assertions

func connectedState() state.State {
	return state.State{
		Connection: state.Connected, Name: "G23 V2", ProductID: protocol.ProductIDWireless,
		Battery: state.Some(84), DPI: state.Some(1600), Polling: state.Some(8000),
		LOD: state.Some(protocol.LOD07mm), MotionSync: state.Some(true),
		Ripple: state.Some(true), AngleSnap: state.Some(false),
		Debounce: state.Some(4), Sleep: state.Some(300),
		ReceiverLED: state.Some(protocol.LEDBatteryStatus),
	}
}

func assertFits(t *testing.T, out string, w int) {
	t.Helper()
	for i, line := range strings.Split(out, "\n") {
		if got := lipgloss.Width(line); got > w {
			t.Errorf("line %d is %d wide (> %d): %q", i, got, w, line)
		}
	}
}

func TestStatusPanelConnected(t *testing.T) {
	for _, w := range []int{60, 80, 120} {
		out := statusPanel(connectedState(), w, false)
		assertFits(t, out, w)
		for _, line := range strings.Split(out, "\n") {
			if lipgloss.Width(line) != w {
				t.Errorf("w=%d: line not full width (%d): %q", w, lipgloss.Width(line), line)
			}
		}
		if w >= 80 {
			for _, want := range []string{"INCOTT", "G23 V2", "● Connected (wireless)", "84%", "1600", "8000 Hz",
				"0.7 mm", "4 ms", "5 min", "On", "Off", "Battery status"} {
				if !strings.Contains(out, want) {
					t.Errorf("w=%d: missing %q in\n%s", w, want, out)
				}
			}
		}
	}
}

func TestStatusPanelStates(t *testing.T) {
	tests := []struct {
		name string
		st   state.State
		want string
	}{
		{"disconnected", state.State{}, "○ Not connected"},
		{"connecting", state.State{Connection: state.Connecting}, "◐ Connecting"},
		{"permission", state.State{Connection: state.PermissionDenied}, "⚠ Permission denied"},
	}
	for _, tt := range tests {
		out := statusPanel(tt.st, 80, false)
		if !strings.Contains(out, tt.want) {
			t.Errorf("%s: missing %q in\n%s", tt.name, tt.want, out)
		}
		if !strings.Contains(out, "—") {
			t.Errorf("%s: unknown values must show an em dash", tt.name)
		}
	}
}

func TestStatusPanelBadges(t *testing.T) {
	s := connectedState()
	s.ProductID = protocol.ProductIDCharging
	s.Charging = true
	out := statusPanel(s, 80, false)
	if !strings.Contains(out, "(wired)") || !strings.Contains(out, "⚡") {
		t.Errorf("wired/charging not shown:\n%s", out)
	}
	s.ReceiverLEDUnsupported, s.ReceiverLED = true, state.Opt[protocol.LEDMode]{}
	if out := statusPanel(s, 80, false); !strings.Contains(out, "not supported") {
		t.Errorf("unsupported LED not shown:\n%s", out)
	}
}

func TestStatusPanelCompact(t *testing.T) {
	out := statusPanel(connectedState(), 80, true)
	if n := len(strings.Split(out, "\n")); n != 3 {
		t.Errorf("compact panel has %d lines, want 3", n)
	}
	assertFits(t, out, 80)
	if !strings.Contains(out, "84%") || !strings.Contains(out, "1600 DPI") {
		t.Errorf("compact content:\n%s", out)
	}
}

func TestBatteryBar(t *testing.T) {
	for _, pct := range []int{0, 1, 14, 15, 40, 41, 100} {
		s := state.State{Battery: state.Some(pct)}
		cell := batteryCell(s)
		if got := strings.Count(cell, "█") + strings.Count(cell, "░"); got != barWidth {
			t.Errorf("%d%%: bar has %d cells", pct, got)
		}
	}
	if got := strings.Count(batteryCell(state.State{Battery: state.Some(100)}), "█"); got != barWidth {
		t.Errorf("100%% should fill the bar, got %d", got)
	}
	if got := strings.Count(batteryCell(state.State{Battery: state.Some(1)}), "█"); got != 1 {
		t.Errorf("1%% should show one block, got %d", got)
	}
}

func newModel(t *testing.T, demo *DemoHooks) Model {
	t.Helper()
	return New(device.New(device.NewFakeMouse().Connector(), device.Options{}), demo)
}

func send(m Model, msg tea.Msg) (Model, tea.Cmd) {
	nm, cmd := m.Update(msg)
	return nm.(Model), cmd
}

func TestViewLayout(t *testing.T) {
	for _, size := range [][2]int{{80, 24}, {120, 40}, {60, 20}, {100, 12}} {
		m := newModel(t, nil)
		m, _ = send(m, tea.WindowSizeMsg{Width: size[0], Height: size[1]})
		m, _ = send(m, stateMsg(connectedState()))
		out := m.View()
		lines := strings.Split(out, "\n")
		if len(lines) > size[1] {
			t.Errorf("%v: %d lines exceed height", size, len(lines))
		}
		assertFits(t, out, size[0])
		if !strings.Contains(out, "INCOTT") || !strings.Contains(out, "q quit") {
			t.Errorf("%v: missing status or footer:\n%s", size, out)
		}
	}
}

func TestViewFillsHeightWhenNormal(t *testing.T) {
	m := newModel(t, nil)
	m, _ = send(m, tea.WindowSizeMsg{Width: 80, Height: 30})
	if n := len(strings.Split(m.View(), "\n")); n != 30 {
		t.Errorf("view is %d lines, want 30", n)
	}
}

func TestViewTooSmall(t *testing.T) {
	m := newModel(t, nil)
	m, _ = send(m, tea.WindowSizeMsg{Width: 40, Height: 10})
	if out := m.View(); !strings.Contains(out, "too small") {
		t.Errorf("got %q", out)
	}
	m2 := newModel(t, nil)
	if m2.View() != "" {
		t.Error("view before first size message must be empty")
	}
}

func TestKeys(t *testing.T) {
	m := newModel(t, nil)
	for _, k := range []tea.KeyMsg{{Type: tea.KeyRunes, Runes: []rune{'q'}}, {Type: tea.KeyCtrlC}} {
		_, cmd := send(m, k)
		if cmd == nil || cmd() != tea.Quit() {
			t.Errorf("key %v must quit", k)
		}
	}
	if _, cmd := send(m, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'r'}}); cmd == nil {
		t.Error("r must return a refresh command")
	}
	// Demo keys do nothing without hooks.
	if _, cmd := send(m, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'u'}}); cmd != nil {
		t.Error("u without demo must be a no-op")
	}
}

func TestDemoKeys(t *testing.T) {
	var plug, perm int
	m := newModel(t, &DemoHooks{TogglePlug: func() { plug++ }, TogglePermission: func() { perm++ }})
	send(m, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'u'}})
	send(m, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'x'}})
	if plug != 1 || perm != 1 {
		t.Errorf("plug=%d perm=%d", plug, perm)
	}
	m, _ = send(m, tea.WindowSizeMsg{Width: 100, Height: 24})
	if !strings.Contains(m.View(), "demo: unplug/plug") {
		t.Error("demo footer missing")
	}
}

func TestStateMsgUpdatesAndRelistens(t *testing.T) {
	m := newModel(t, nil)
	m, cmd := send(m, stateMsg(connectedState()))
	if m.st.DPI.Val != 1600 || cmd == nil {
		t.Errorf("state not applied or listener not re-armed: %+v", m.st)
	}
	if _, cmd := send(m, stoppedMsg{}); cmd == nil || cmd() != tea.Quit() {
		t.Error("stopped client must quit the UI")
	}
}

func TestHints(t *testing.T) {
	join := func(st state.State, goos string) string { return strings.Join(hint(st, goos, 0), "\n") }
	if s := join(state.State{}, "linux"); !strings.Contains(s, "No Incott mouse found") {
		t.Error(s)
	}
	perm := state.State{Connection: state.PermissionDenied, Err: "boom"}
	for goos, want := range map[string]string{"linux": "udev", "darwin": "Input Monitoring", "windows": "Close other apps"} {
		if s := join(perm, goos); !strings.Contains(s, want) || !strings.Contains(s, "boom") {
			t.Errorf("%s: %s", goos, s)
		}
	}
	if s := join(state.State{Connection: state.Connecting}, "linux"); !strings.Contains(s, "Reading") {
		t.Error(s)
	}
	if hint(connectedState(), "linux", 0) != nil {
		t.Error("no hint when connected")
	}
}

func TestBoxTinyWidthDoesNotPanic(t *testing.T) {
	assertFits(t, box("a very long title indeed", "tag", []string{"x"}, 12, styleBorder), 12)
}
