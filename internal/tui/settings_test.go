package tui

import (
	"context"
	"errors"
	"strings"
	"sync"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/mastergap/incott-mouse-tui/internal/device"
	"github.com/mastergap/incott-mouse-tui/internal/protocol"
	"github.com/mastergap/incott-mouse-tui/internal/state"
)

// stub is a Backend that records writes and applies them to its state.
type stub struct {
	mu      sync.Mutex
	st      state.State
	applied []device.Setting
	err     error
	events  chan state.State
}

func newStub(st state.State) *stub { return &stub{st: st, events: make(chan state.State, 1)} }

func (s *stub) Events() <-chan state.State    { return s.events }
func (s *stub) State() state.State            { s.mu.Lock(); defer s.mu.Unlock(); return s.st }
func (s *stub) Refresh(context.Context) error { return s.err }
func (s *stub) Apply(_ context.Context, set device.Setting) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.err != nil {
		return s.err
	}
	s.applied = append(s.applied, set)
	switch set.Kind {
	case device.KindDPI:
		s.st.DPI = state.Some(set.Value)
	case device.KindPolling:
		s.st.Polling = state.Some(set.Value)
	case device.KindDebounce:
		s.st.Debounce = state.Some(set.Value)
	case device.KindMotionSync:
		s.st.MotionSync = state.Some(set.Value != 0)
	case device.KindSleep:
		s.st.Sleep = state.Some(set.Value)
	case device.KindReceiverLED:
		s.st.ReceiverLED = state.Some(protocol.LEDMode(set.Value))
	}
	return nil
}

func editModel(t *testing.T, b Backend) Model {
	t.Helper()
	m := New(b, nil)
	m.delay = 0
	m, _ = send(m, tea.WindowSizeMsg{Width: 80, Height: 24})
	m, _ = send(m, stateMsg(b.State()))
	return m
}

func key(s string) tea.KeyMsg {
	switch s {
	case "left":
		return tea.KeyMsg{Type: tea.KeyLeft}
	case "right":
		return tea.KeyMsg{Type: tea.KeyRight}
	case "up":
		return tea.KeyMsg{Type: tea.KeyUp}
	case "down":
		return tea.KeyMsg{Type: tea.KeyDown}
	case "enter":
		return tea.KeyMsg{Type: tea.KeyEnter}
	case "esc":
		return tea.KeyMsg{Type: tea.KeyEsc}
	case "space":
		return tea.KeyMsg{Type: tea.KeySpace, Runes: []rune{' '}}
	case "shift+left":
		return tea.KeyMsg{Type: tea.KeyShiftLeft}
	case "shift+right":
		return tea.KeyMsg{Type: tea.KeyShiftRight}
	}
	return tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(s)}
}

// run executes a command and returns its message; commands that sleep (tick timers for
// toast expiry) are abandoned after a short wait and yield nil.
func run(cmd tea.Cmd) tea.Msg {
	if cmd == nil {
		return nil
	}
	ch := make(chan tea.Msg, 1)
	go func() { ch <- cmd() }()
	select {
	case m := <-ch:
		return m
	case <-time.After(300 * time.Millisecond):
		return nil
	}
}

// settle feeds a command's message back into the model until the chain ends.
func settle(m Model, cmd tea.Cmd) Model {
	for i := 0; i < 6 && cmd != nil; i++ {
		msg := run(cmd)
		if msg == nil {
			return m
		}
		m, cmd = send(m, msg)
	}
	return m
}

func press(m Model, k string) (Model, tea.Cmd) { return send(m, key(k)) }

func TestCycleAppliesAndConfirms(t *testing.T) {
	b := newStub(connectedState()) // DPI 1600
	m := editModel(t, b)
	m, cmd := press(m, "right") // → 2400
	if p, ok := m.set.pending[device.KindDPI]; !ok || p.value != 2400 {
		t.Fatalf("pending = %+v", m.set.pending)
	}
	if !strings.Contains(m.View(), "…") {
		t.Error("pending value must be marked")
	}
	m = settle(m, cmd)
	if len(b.applied) != 1 || b.applied[0] != (device.Setting{Kind: device.KindDPI, Value: 2400}) {
		t.Fatalf("applied = %+v", b.applied)
	}
	if len(m.set.pending) != 0 || m.st.DPI.Val != 2400 {
		t.Errorf("not confirmed: pending=%v dpi=%d", m.set.pending, m.st.DPI.Val)
	}
	if !strings.Contains(m.toast.text, "DPI → 2400") || m.toast.err {
		t.Errorf("toast = %+v", m.toast)
	}
}

func TestCycleEndsDoNothing(t *testing.T) {
	st := connectedState()
	st.DPI = state.Some(6400)
	m := editModel(t, newStub(st))
	if _, cmd := press(m, "right"); cmd != nil {
		t.Error("right at the last preset must not write")
	}
	st.DPI = state.Some(400)
	m = editModel(t, newStub(st))
	if _, cmd := press(m, "left"); cmd != nil {
		t.Error("left at the first preset must not write")
	}
}

func TestCycleFromValueNotInList(t *testing.T) {
	st := connectedState()
	st.DPI = state.Some(1000) // between presets
	m := editModel(t, newStub(st))
	m, _ = press(m, "right")
	right := m.set.pending[device.KindDPI].value
	m.set.reset()
	m, _ = press(m, "left")
	if left := m.set.pending[device.KindDPI].value; right != 1600 || left != 800 {
		t.Errorf("right → %d, left → %d", right, left)
	}
}

func TestUnknownValueStartsAtFirstOption(t *testing.T) {
	st := connectedState()
	st.DPI = state.Opt[int]{}
	m := editModel(t, newStub(st))
	m, _ = press(m, "right")
	if m.set.pending[device.KindDPI].value != 400 {
		t.Errorf("pending = %+v", m.set.pending)
	}
}

func TestNavigationAndGroups(t *testing.T) {
	m := editModel(t, newStub(connectedState()))
	want := []device.Kind{device.KindDPI, device.KindPolling, device.KindLOD, device.KindDebounce,
		device.KindMotionSync, device.KindAngleSnap, device.KindRipple, device.KindSleep, device.KindReceiverLED}
	for i, k := range want {
		if got := m.set.items[m.set.cursor].kind; got != k {
			t.Fatalf("step %d: on %v, want %v", i, got, k)
		}
		m, _ = press(m, "down")
	}
	if m.set.items[m.set.cursor].kind != device.KindReceiverLED {
		t.Error("cursor must stay on the last item")
	}
	m, _ = press(m, "up")
	if m.set.items[m.set.cursor].kind != device.KindSleep {
		t.Error("up")
	}
}

func TestUnsupportedLEDIsNotSelectable(t *testing.T) {
	st := connectedState()
	st.ReceiverLEDUnsupported, st.ReceiverLED = true, state.Opt[protocol.LEDMode]{}
	m := editModel(t, newStub(st))
	for i := 0; i < 12; i++ {
		m, _ = press(m, "down")
	}
	if m.set.items[m.set.cursor].kind != device.KindSleep {
		t.Errorf("cursor on %v", m.set.items[m.set.cursor].kind)
	}
	if !strings.Contains(m.View(), "not supported by this mouse") {
		t.Error("row must say it is unsupported")
	}
}

func TestSliderStepsClampAndDebounce(t *testing.T) {
	st := connectedState() // debounce 4
	b := newStub(st)
	m := editModel(t, b)
	m, _ = press(m, "down")
	m, _ = press(m, "down")
	m, _ = press(m, "down") // → Debounce
	if m.set.items[m.set.cursor].kind != device.KindDebounce {
		t.Fatal("cursor not on debounce")
	}
	// Three quick presses produce three due messages; only the last one writes.
	m, c1 := press(m, "right")
	m, c2 := press(m, "right")
	m, c3 := press(m, "shift+right")
	if got := m.set.pending[device.KindDebounce].value; got != 4+1+1+5 {
		t.Fatalf("pending = %d", got)
	}
	var writes int
	for _, c := range []tea.Cmd{c1, c2, c3} {
		_, cmd := send(m, run(c))
		if cmd != nil {
			writes++
		}
	}
	if writes != 1 {
		t.Errorf("%d writes, want 1 (stale timers must be ignored)", writes)
	}
	// Clamp at the ends.
	m.set.pending[device.KindDebounce] = pendingVal{value: 29, seq: 99}
	m, _ = press(m, "shift+right")
	if m.set.pending[device.KindDebounce].value != 30 {
		t.Errorf("max clamp: %d", m.set.pending[device.KindDebounce].value)
	}
	if _, cmd := press(m, "right"); cmd != nil {
		t.Error("right at max must not write")
	}
}

func TestToggleKeys(t *testing.T) {
	b := newStub(connectedState()) // motion sync on, angle off
	m := editModel(t, b)
	for i := 0; i < 4; i++ {
		m, _ = press(m, "down")
	}
	if m.set.items[m.set.cursor].kind != device.KindMotionSync {
		t.Fatal("cursor not on motion sync")
	}
	m, cmd := press(m, "space") // on → off
	m = settle(m, cmd)
	if len(b.applied) != 1 || b.applied[0].Value != 0 {
		t.Fatalf("space: %+v", b.applied)
	}
	m, cmd = press(m, "right") // → on
	m = settle(m, cmd)
	m, cmd = press(m, "left") // → off
	m = settle(m, cmd)
	if got := []int{b.applied[1].Value, b.applied[2].Value}; got[0] != 1 || got[1] != 0 {
		t.Errorf("right/left: %v", got)
	}
	if m.st.MotionSync.Val {
		t.Error("state must show off")
	}
}

func TestPickerFlow(t *testing.T) {
	b := newStub(connectedState()) // sleep 300 s (index 7)
	m := editModel(t, b)
	for i := 0; i < 7; i++ {
		m, _ = press(m, "down")
	}
	if m.set.items[m.set.cursor].kind != device.KindSleep {
		t.Fatal("cursor not on sleep")
	}
	m, _ = press(m, "enter")
	if m.set.picker == nil || m.set.picker.cursor != 7 {
		t.Fatalf("picker = %+v", m.set.picker)
	}
	view := m.View()
	if !strings.Contains(view, "5 min") || !strings.Contains(view, "Pick a value") {
		t.Errorf("picker view:\n%s", view)
	}
	m, _ = press(m, "esc")
	if m.set.picker != nil || len(b.applied) != 0 {
		t.Error("esc must cancel without writing")
	}
	m, _ = press(m, "enter")
	m, _ = press(m, "down") // 6 min
	m, cmd := press(m, "enter")
	m = settle(m, cmd)
	if len(b.applied) != 1 || b.applied[0] != (device.Setting{Kind: device.KindSleep, Value: 360}) {
		t.Fatalf("applied = %+v", b.applied)
	}
	if m.set.picker != nil {
		t.Error("picker must close")
	}
}

func TestPickerKeysDoNotLeak(t *testing.T) {
	m := editModel(t, newStub(connectedState()))
	m, _ = press(m, "enter") // picker on DPI
	if _, cmd := press(m, "q"); cmd != nil && run(cmd) == tea.Quit() {
		t.Error("q inside the picker must close it, not quit")
	}
}

func TestErrorRollsBackAndToasts(t *testing.T) {
	b := newStub(connectedState())
	b.err = device.ErrMismatch
	m := editModel(t, b)
	m, cmd := press(m, "right")
	m = settle(m, cmd)
	if len(m.set.pending) != 0 || m.st.DPI.Val != 1600 {
		t.Errorf("rollback failed: pending=%v dpi=%d", m.set.pending, m.st.DPI.Val)
	}
	if !m.toast.err || !strings.Contains(m.toast.text, "different value") {
		t.Errorf("toast = %+v", m.toast)
	}
	if !strings.Contains(m.View(), "1600") {
		t.Error("view must show the device value again")
	}
}

func TestNewerPendingSurvivesOlderResult(t *testing.T) {
	m := editModel(t, newStub(connectedState()))
	m, _ = press(m, "right") // pending 2400
	m, _ = press(m, "right") // pending 3200
	m, _ = send(m, applyResultMsg{kind: device.KindDPI, value: 2400})
	if p := m.set.pending[device.KindDPI]; p.value != 3200 {
		t.Errorf("newer pending dropped: %+v", p)
	}
}

func TestEditingDisabledWhenDisconnected(t *testing.T) {
	m := editModel(t, newStub(state.State{}))
	for _, k := range []string{"right", "left", "enter", "space", "down"} {
		var cmd tea.Cmd
		m, cmd = press(m, k)
		if cmd != nil || len(m.set.pending) != 0 || m.set.picker != nil {
			t.Fatalf("%s edited while disconnected", k)
		}
	}
	if !strings.Contains(m.View(), "No Incott mouse found") {
		t.Error("hint expected")
	}
}

func TestDisconnectDropsPendingAndPicker(t *testing.T) {
	m := editModel(t, newStub(connectedState()))
	m, _ = press(m, "right")
	m, _ = send(m, stateMsg(state.State{}))
	if len(m.set.pending) != 0 || m.set.picker != nil {
		t.Error("pending/picker must be cleared")
	}
}

func TestToastExpiry(t *testing.T) {
	m := editModel(t, newStub(connectedState()))
	m, _ = m.withToastModel("hello", false)
	id := m.toast.id
	m, _ = send(m, toastExpireMsg{id: id - 1})
	if m.toast.text == "" {
		t.Error("stale expiry must not clear a newer toast")
	}
	m, _ = send(m, toastExpireMsg{id: id})
	if m.toast.text != "" {
		t.Error("toast must expire")
	}
}

func (m Model) withToastModel(text string, isErr bool) (Model, tea.Cmd) {
	nm, cmd := m.withToast(text, isErr)
	return nm.(Model), cmd
}

func TestViewShowsEditor(t *testing.T) {
	for _, size := range [][2]int{{80, 24}, {60, 16}, {120, 40}, {60, 12}} {
		m := New(newStub(connectedState()), nil)
		m, _ = send(m, tea.WindowSizeMsg{Width: size[0], Height: size[1]})
		m, _ = send(m, stateMsg(connectedState()))
		out := m.View()
		assertFits(t, out, size[0])
		if n := len(strings.Split(out, "\n")); n > size[1] {
			t.Errorf("%v: %d lines", size, n)
		}
		if !strings.Contains(out, "▸ DPI") {
			t.Errorf("%v: cursor row missing:\n%s", size, out)
		}
		if size[1] >= 24 {
			for _, want := range []string{"PERFORMANCE", "RESPONSIVENESS", "POWER & LED", "◀ 1600    ▶", "Polling rate",
				"[x] On", "[ ] Off", "████", "Sleep timer", "Receiver LED"} {
				if !strings.Contains(out, want) {
					t.Errorf("%v: missing %q in\n%s", size, want, out)
				}
			}
		}
	}
}

func TestSmallHeightScrollsToCursor(t *testing.T) {
	m := New(newStub(connectedState()), nil)
	m, _ = send(m, tea.WindowSizeMsg{Width: 70, Height: 12})
	m, _ = send(m, stateMsg(connectedState()))
	for i := 0; i < 8; i++ {
		m, _ = press(m, "down")
	}
	if out := m.View(); !strings.Contains(out, "▸ Receiver LED") {
		t.Errorf("cursor scrolled out of view:\n%s", out)
	}
}

func TestFooterHintsFollowSelection(t *testing.T) {
	m := editModel(t, newStub(connectedState()))
	if f := m.footer(); !strings.Contains(f, "pick list") {
		t.Errorf("cycle footer: %s", f)
	}
	for i := 0; i < 3; i++ {
		m, _ = press(m, "down")
	}
	if f := m.footer(); !strings.Contains(f, "±5") {
		t.Errorf("slider footer: %s", f)
	}
	m, _ = press(m, "down")
	if f := m.footer(); !strings.Contains(f, "toggle") {
		t.Errorf("toggle footer: %s", f)
	}
}

func TestDescribeError(t *testing.T) {
	tests := []struct {
		err  error
		want string
	}{
		{device.ErrUnsupported, "not supported"},
		{device.ErrConnectionLost, "lost"},
		{device.ErrNotConnected, "No mouse connected"},
		{device.ErrMismatch, "different value"},
		{errors.New("boom"), "boom"},
	}
	for _, tt := range tests {
		if got := describeError(device.KindDPI, tt.err); !strings.Contains(got, tt.want) {
			t.Errorf("%v → %q", tt.err, got)
		}
	}
}

// End to end against the protocol-accurate fake mouse and the real client.
func TestEditAgainstFakeMouse(t *testing.T) {
	mouse := device.NewFakeMouse()
	client := device.New(mouse.Connector(), device.Options{
		RetryInterval: 10 * time.Millisecond, LostInterval: 10 * time.Millisecond,
		PollInterval: time.Millisecond, PollAttempts: 5, ReadSlice: 5 * time.Millisecond,
	})
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() { client.Run(ctx); close(done) }()
	t.Cleanup(func() { cancel(); <-done })

	deadline := time.Now().Add(3 * time.Second)
	for client.State().Connection != state.Connected {
		if time.Now().After(deadline) {
			t.Fatal("client never connected")
		}
		time.Sleep(5 * time.Millisecond)
	}

	m := New(client, nil)
	m.delay = 0
	m, _ = send(m, tea.WindowSizeMsg{Width: 80, Height: 24})
	m, _ = send(m, stateMsg(client.State()))

	m, cmd := press(m, "right") // DPI 800 → 1600
	m = settle(m, cmd)
	if m.st.DPI.Val != 1600 || client.State().DPI.Val != 1600 {
		t.Fatalf("dpi = %d / %d", m.st.DPI.Val, client.State().DPI.Val)
	}
	var devDPI int
	mouse.Update(func(f *device.FakeMouse) { devDPI = f.DPI })
	if devDPI != 1600 {
		t.Errorf("the mouse itself holds %d", devDPI)
	}
}

func lodOptions(m Model) []int {
	for _, it := range m.set.items {
		if it.kind == device.KindLOD {
			return it.options
		}
	}
	return nil
}

func TestLOD07RefusalToastAndOptionRemoved(t *testing.T) {
	st := connectedState() // LOD 0.7 in the fixture; use 2 mm for the scenario
	st.LOD = state.Some(protocol.LOD2mm)
	m := editModel(t, newStub(st))
	if got := lodOptions(m); len(got) != 3 {
		t.Fatalf("options before: %v", got)
	}
	refused := st
	refused.LOD07Unsupported = true
	b := m.backend.(*stub)
	b.mu.Lock()
	b.st = refused
	b.mu.Unlock()
	m, _ = send(m, applyResultMsg{kind: device.KindLOD, value: protocol.LOD07mm, err: device.ErrMismatch})
	if !m.toast.err || !strings.Contains(m.toast.text, "does not support 0.7 mm") || !strings.Contains(m.toast.text, "2 mm") {
		t.Errorf("toast = %+v", m.toast)
	}
	if got := lodOptions(m); len(got) != 2 || got[0] != protocol.LOD1mm {
		t.Errorf("options after: %v", got)
	}
	// Cycling from 1 mm no longer offers 0.7 mm to the left.
	m.st.LOD = state.Some(protocol.LOD1mm)
	for i := 0; i < 2; i++ {
		m, _ = press(m, "down")
	}
	if m.set.items[m.set.cursor].kind != device.KindLOD {
		t.Fatal("cursor not on LOD")
	}
	if _, cmd := press(m, "left"); cmd != nil {
		t.Error("left from 1 mm must be a no-op when 0.7 mm is unsupported")
	}
}

func TestBarsOnDPIAndPolling(t *testing.T) {
	st := connectedState()
	for _, tt := range []struct {
		dpi, hz int
		cells   int // filled cells on the DPI bar
	}{{400, 125, 1}, {800, 250, 3}, {6400, 8000, sliderCells}} {
		st.DPI, st.Polling = state.Some(tt.dpi), state.Some(tt.hz)
		m := editModel(t, newStub(st))
		lines, _, _ := m.set.view(m.st)
		out := strings.Join(lines, "\n")
		var dpiLine, hzLine string
		for _, l := range lines {
			if strings.Contains(l, "DPI") {
				dpiLine = l
			}
			if strings.Contains(l, "Polling rate") {
				hzLine = l
			}
		}
		if got := strings.Count(dpiLine, "█"); got != tt.cells {
			t.Errorf("dpi %d: %d filled cells, want %d\n%s", tt.dpi, got, tt.cells, out)
		}
		if strings.Count(dpiLine, "█")+strings.Count(dpiLine, "░") != sliderCells ||
			strings.Count(hzLine, "█")+strings.Count(hzLine, "░") != sliderCells {
			t.Errorf("bars must be %d cells:\n%s", sliderCells, out)
		}
	}
	// Polling top value fills the bar; lowest shows one cell.
	st.Polling = state.Some(8000)
	m := editModel(t, newStub(st))
	lines, _, _ := m.set.view(m.st)
	if got := strings.Count(lines[2], "█"); got != sliderCells {
		t.Errorf("8000 Hz: %d cells\n%s", got, lines[2])
	}
}

func TestClickBarSetsPreset(t *testing.T) {
	m := editModel(t, newStub(connectedState())) // DPI 1600
	m, _ = clickItem(t, m, device.KindDPI, sliderBarAt+sliderCells-1)
	if m.set.pending[device.KindDPI].value != 6400 {
		t.Errorf("bar end: %+v", m.set.pending)
	}
	m.set.reset()
	m, _ = clickItem(t, m, device.KindDPI, sliderBarAt)
	if m.set.pending[device.KindDPI].value != 400 {
		t.Errorf("bar start: %+v", m.set.pending)
	}
	m.set.reset()
	m, _ = clickItem(t, m, device.KindDPI, sliderRightAt) // ▶ at the aligned position
	if m.set.pending[device.KindDPI].value != 2400 {
		t.Errorf("▶: %+v", m.set.pending)
	}
	m.set.reset()
	m, _ = clickItem(t, m, device.KindPolling, sliderBarAt) // 8000 Hz current; bar start = 125
	if m.set.pending[device.KindPolling].value != 125 {
		t.Errorf("polling bar start: %+v", m.set.pending)
	}
}
