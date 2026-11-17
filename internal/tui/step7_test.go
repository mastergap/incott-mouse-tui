package tui

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/mastergap/incott-mouse-tui/internal/device"
	"github.com/mastergap/incott-mouse-tui/internal/state"
)

func click(x, y int) tea.MouseMsg {
	return tea.MouseMsg{X: x, Y: y, Button: tea.MouseButtonLeft, Action: tea.MouseActionPress}
}

// at returns the screen position of column rel of the control of the item with the given kind.
func at(t *testing.T, m Model, kind device.Kind, rel int) (x, y int) {
	t.Helper()
	top, inner := m.bodyGeometry()
	lines, focus, items := m.set.view(m.st)
	start := windowStart(len(lines), focus, inner)
	for li, idx := range items {
		if idx >= 0 && m.set.items[idx].kind == kind {
			return 2 + ctlCol + rel, top + li - start
		}
	}
	t.Fatalf("item %v not visible", kind)
	return
}

func clickItem(t *testing.T, m Model, kind device.Kind, rel int) (Model, tea.Cmd) {
	t.Helper()
	x, y := at(t, m, kind, rel)
	return send(m, click(x, y))
}

func TestHelpOverlay(t *testing.T) {
	m := editModel(t, newStub(connectedState()))
	m, _ = press(m, "?")
	if !m.help {
		t.Fatal("? must open help")
	}
	out := m.View()
	assertFits(t, out, 80)
	for _, want := range []string{"Help", "MOVE", "CHANGE", "MOUSE", "GENERAL", "esc close help", "scroll", "INCOTT", "%"} {
		if !strings.Contains(out, want) {
			t.Errorf("help view missing %q:\n%s", want, out)
		}
	}
	if n := len(strings.Split(out, "\n")); n != 24 {
		t.Errorf("help view is %d lines", n)
	}
	// Editing keys are blocked behind the overlay.
	if m2, cmd := press(m, "right"); cmd != nil || len(m2.set.pending) != 0 {
		t.Error("editing must be blocked behind the help overlay")
	}
	// q closes help instead of quitting.
	m, cmd := press(m, "q")
	if m.help || (cmd != nil && run(cmd) == tea.Quit()) {
		t.Error("q must close help, not quit")
	}
	for _, k := range []string{"esc", "?"} {
		m, _ = press(m, "?")
		m, _ = press(m, k)
		if m.help {
			t.Errorf("%s must close help", k)
		}
	}
	m, _ = press(m, "?")
	if _, cmd := send(m, tea.KeyMsg{Type: tea.KeyCtrlC}); cmd == nil || run(cmd) != tea.Quit() {
		t.Error("ctrl+c must still quit")
	}
}

func TestHelpScrolls(t *testing.T) {
	m := editModel(t, newStub(connectedState()))
	m, _ = press(m, "?")
	first := m.View()
	if strings.Contains(first, "Receiver LED") {
		t.Fatal("end of the help must not be visible before scrolling")
	}
	m, _ = press(m, "down")
	if m.helpScroll != 1 {
		t.Errorf("scroll = %d", m.helpScroll)
	}
	m, _ = press(m, "up")
	m, _ = press(m, "up") // clamped at the top
	if m.helpScroll != 0 {
		t.Errorf("scroll = %d", m.helpScroll)
	}
	m, _ = send(m, tea.KeyMsg{Type: tea.KeyPgDown})
	if m.helpScroll != m.helpPage() {
		t.Errorf("page down scrolled %d, want %d", m.helpScroll, m.helpPage())
	}
	m, _ = press(m, "G") // end
	last := m.View()
	assertFits(t, last, 80)
	if !strings.Contains(last, "Battery and connection") || !strings.Contains(last, "100%") {
		t.Errorf("end of help:\n%s", last)
	}
	m, _ = press(m, "down") // clamped at the bottom
	max := m.helpScroll
	m, _ = press(m, "down")
	if m.helpScroll != max {
		t.Error("scroll must clamp at the end")
	}
	m, _ = send(m, tea.MouseMsg{Button: tea.MouseButtonWheelUp, Action: tea.MouseActionPress})
	if m.helpScroll != max-3 {
		t.Errorf("wheel up: %d", m.helpScroll)
	}
	m, _ = press(m, "g")
	if m.helpScroll != 0 {
		t.Error("g must go to the top")
	}
	m, _ = press(m, "esc")
	m, _ = press(m, "?")
	if m.helpScroll != 0 {
		t.Error("help must reopen at the top")
	}
}

func TestHelpDocumentsEverySetting(t *testing.T) {
	text := strings.Join(helpLines(76), "\n")
	for _, it := range buildItems() {
		if !strings.Contains(text, it.label) {
			t.Errorf("help does not explain %q", it.label)
		}
	}
	for _, d := range settingDocs {
		if d.Purpose == "" || d.Effect == "" || d.Tip == "" {
			t.Errorf("%s: incomplete entry", d.Name)
		}
	}
	for _, line := range helpLines(60) {
		if w := lipglossWidth(line); w > 60 {
			t.Errorf("help line wider than the panel (%d): %q", w, line)
		}
	}
}

func TestHelpWorksWhenDisconnected(t *testing.T) {
	m := editModel(t, newStub(state.State{}))
	m, _ = press(m, "?")
	if !strings.Contains(m.View(), "GENERAL") {
		t.Error("help must open without a mouse")
	}
}

func TestMouseWheelChangesValue(t *testing.T) {
	m := editModel(t, newStub(connectedState())) // DPI 1600 selected
	m, _ = send(m, tea.MouseMsg{Button: tea.MouseButtonWheelUp, Action: tea.MouseActionPress})
	if m.set.pending[device.KindDPI].value != 2400 {
		t.Errorf("wheel up: %+v", m.set.pending)
	}
	m, _ = send(m, tea.MouseMsg{Button: tea.MouseButtonWheelDown, Action: tea.MouseActionPress})
	m, _ = send(m, tea.MouseMsg{Button: tea.MouseButtonWheelDown, Action: tea.MouseActionPress})
	if m.set.pending[device.KindDPI].value != 800 {
		t.Errorf("wheel down x2: %+v", m.set.pending)
	}
}

func TestMouseWheelMovesPicker(t *testing.T) {
	m := editModel(t, newStub(connectedState()))
	m, _ = press(m, "enter") // picker on DPI, cursor on 1600 (idx 2)
	m, _ = send(m, tea.MouseMsg{Button: tea.MouseButtonWheelDown, Action: tea.MouseActionPress})
	if m.set.picker.cursor != 3 {
		t.Errorf("picker cursor = %d", m.set.picker.cursor)
	}
	m, _ = send(m, tea.MouseMsg{Button: tea.MouseButtonWheelUp, Action: tea.MouseActionPress})
	if m.set.picker.cursor != 2 {
		t.Errorf("picker cursor = %d", m.set.picker.cursor)
	}
}

func TestClickSelectsRow(t *testing.T) {
	m := editModel(t, newStub(connectedState()))
	x, y := at(t, m, device.KindRipple, -ctlCol+3) // inside the label
	m, cmd := send(m, click(x, y))
	if m.set.items[m.set.cursor].kind != device.KindRipple || cmd != nil || len(m.set.pending) != 0 {
		t.Errorf("label click must only select; cursor on %v, pending %v", m.set.items[m.set.cursor].kind, m.set.pending)
	}
}

func TestClickCycleArrowsAndValue(t *testing.T) {
	m := editModel(t, newStub(connectedState()))          // DPI 1600: "◀ 1600 ▶"
	m, _ = clickItem(t, m, device.KindDPI, sliderRightAt) // ▶
	if m.set.pending[device.KindDPI].value != 2400 {
		t.Errorf("right arrow: %+v", m.set.pending)
	}
	m.set.reset()
	m, _ = clickItem(t, m, device.KindDPI, 0) // ◀
	if m.set.pending[device.KindDPI].value != 800 {
		t.Errorf("left arrow: %+v", m.set.pending)
	}
	m.set.reset()
	m, _ = clickItem(t, m, device.KindDPI, 3) // value → picker
	if m.set.picker == nil {
		t.Error("clicking the value must open the picker")
	}
}

func TestClickToggle(t *testing.T) {
	m := editModel(t, newStub(connectedState())) // motion sync on
	m, cmd := clickItem(t, m, device.KindMotionSync, 1)
	if p := m.set.pending[device.KindMotionSync]; p.value != 0 || cmd == nil {
		t.Errorf("toggle: %+v", m.set.pending)
	}
}

func TestClickSlider(t *testing.T) {
	m := editModel(t, newStub(connectedState())) // debounce 4
	m, _ = clickItem(t, m, device.KindDebounce, sliderRightAt)
	if m.set.pending[device.KindDebounce].value != 5 {
		t.Errorf("slider ▶: %+v", m.set.pending)
	}
	m.set.reset()
	m, _ = clickItem(t, m, device.KindDebounce, 0)
	if m.set.pending[device.KindDebounce].value != 3 {
		t.Errorf("slider ◀: %+v", m.set.pending)
	}
	m.set.reset()
	m, _ = clickItem(t, m, device.KindDebounce, sliderBarAt+sliderCells-1) // right end of the bar
	if m.set.pending[device.KindDebounce].value != 30 {
		t.Errorf("bar end: %+v", m.set.pending)
	}
	m.set.reset()
	m, _ = clickItem(t, m, device.KindDebounce, sliderBarAt) // left end
	if m.set.pending[device.KindDebounce].value != 0 {
		t.Errorf("bar start: %+v", m.set.pending)
	}
	m.set.reset()
	m, _ = clickItem(t, m, device.KindDebounce, sliderBarAt+7) // middle ≈ 15
	if v := m.set.pending[device.KindDebounce].value; v < 13 || v > 17 {
		t.Errorf("bar middle: %d", v)
	}
}

func TestClickInPickerAppliesOption(t *testing.T) {
	b := newStub(connectedState())
	m := editModel(t, b)
	m, _ = press(m, "enter") // DPI picker
	top, _ := m.bodyGeometry()
	// Options start two lines below the title; pick the first (400).
	m, cmd := send(m, click(5, top+2))
	m = settle(m, cmd)
	if len(b.applied) != 1 || b.applied[0].Value != 400 || m.set.picker != nil {
		t.Errorf("applied %+v, picker %v", b.applied, m.set.picker)
	}
}

func TestClicksIgnoredWhenNotApplicable(t *testing.T) {
	// Outside the body, on headers, on release, while disconnected.
	m := editModel(t, newStub(connectedState()))
	top, _ := m.bodyGeometry()
	for _, msg := range []tea.MouseMsg{
		click(10, 0),          // status panel
		click(10, top),        // PERFORMANCE header
		click(10, m.height-1), // footer
		{X: 30, Y: top + 1, Button: tea.MouseButtonLeft, Action: tea.MouseActionRelease},
		{X: 30, Y: top + 1, Button: tea.MouseButtonLeft, Action: tea.MouseActionMotion},
	} {
		if m2, cmd := send(m, msg); cmd != nil || len(m2.set.pending) != 0 || m2.set.cursor != m.set.cursor {
			t.Errorf("%+v changed something", msg)
		}
	}
	d := editModel(t, newStub(state.State{}))
	if d2, cmd := send(d, click(30, 10)); cmd != nil || len(d2.set.pending) != 0 {
		t.Error("click while disconnected")
	}
	if d2, cmd := send(d, tea.MouseMsg{Button: tea.MouseButtonWheelUp, Action: tea.MouseActionPress}); cmd != nil || len(d2.set.pending) != 0 {
		t.Error("wheel while disconnected")
	}
}

func TestClickClosesHelp(t *testing.T) {
	m := editModel(t, newStub(connectedState()))
	m, _ = press(m, "?")
	m, _ = send(m, click(10, 10))
	if m.help {
		t.Error("click must close help")
	}
}

func TestClickWorksWithScrolledList(t *testing.T) {
	m := New(newStub(connectedState()), nil)
	m.delay = 0
	m, _ = send(m, tea.WindowSizeMsg{Width: 80, Height: 12})
	m, _ = send(m, stateMsg(connectedState()))
	for i := 0; i < 8; i++ {
		m, _ = press(m, "down") // scroll to Receiver LED
	}
	m, _ = clickItem(t, m, device.KindSleep, 0) // ◀ on a row that is visible only after scrolling
	if m.set.items[m.set.cursor].kind != device.KindSleep || m.set.pending[device.KindSleep].value != 240 {
		t.Errorf("cursor on %v, pending %+v", m.set.items[m.set.cursor].kind, m.set.pending)
	}
}

func TestConnectionToastsAndSpinner(t *testing.T) {
	m := New(newStub(state.State{}), nil)
	m, _ = send(m, tea.WindowSizeMsg{Width: 80, Height: 24})

	nm, cmd := m.Update(stateMsg(state.State{Connection: state.Connecting, Name: "G23"}))
	m = nm.(Model)
	if cmd == nil {
		t.Fatal("connecting must start the spinner")
	}
	if !strings.Contains(m.View(), spinnerFrames[0]) {
		t.Errorf("spinner frame missing:\n%s", m.View())
	}
	m, cmd = send(m, spinMsg{})
	if m.frame != 1 || cmd == nil {
		t.Errorf("spin: frame=%d cmd=%v", m.frame, cmd)
	}

	m, _ = send(m, stateMsg(connectedState()))
	if !strings.Contains(m.toast.text, "Connected to G23 V2") || m.toast.err {
		t.Errorf("connect toast = %+v", m.toast)
	}
	if m2, cmd := send(m, spinMsg{}); cmd != nil || m2.frame != m.frame {
		t.Error("spinner must stop once connected")
	}

	m, _ = send(m, stateMsg(state.State{Err: "gone"}))
	if !m.toast.err || !strings.Contains(m.toast.text, "disconnected") {
		t.Errorf("disconnect toast = %+v", m.toast)
	}
	// Repeated disconnected snapshots must not re-toast.
	id := m.toast.id
	m, _ = send(m, stateMsg(state.State{Err: "still gone"}))
	if m.toast.id != id {
		t.Error("toast spam on unchanged connection state")
	}
}

func TestRefreshWhileDisconnectedToasts(t *testing.T) {
	m := editModel(t, newStub(state.State{}))
	m, _ = send(m, refreshMsg{err: device.ErrNotConnected})
	if !m.toast.err || !strings.Contains(m.toast.text, "No mouse connected") {
		t.Errorf("toast = %+v", m.toast)
	}
	m, _ = send(m, refreshMsg{})
	if m.toast.err || !strings.Contains(m.toast.text, "refreshed") {
		t.Errorf("toast = %+v", m.toast)
	}
}

func lipglossWidth(s string) int { return ansiWidth(s) }
