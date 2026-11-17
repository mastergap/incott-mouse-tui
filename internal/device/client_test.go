package device

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/mastergap/incott-mouse-tui/internal/protocol"
	"github.com/mastergap/incott-mouse-tui/internal/state"
)

var fast = Options{
	RetryInterval: 10 * time.Millisecond,
	LostInterval:  10 * time.Millisecond,
	PollInterval:  time.Millisecond,
	PollAttempts:  5,
	ReadSlice:     5 * time.Millisecond,
	ConfirmWithin: 200 * time.Millisecond,
}

type harness struct {
	t      *testing.T
	mouse  *FakeMouse
	client *Client
	ctx    context.Context
	cancel context.CancelFunc
	done   chan struct{}
}

func start(t *testing.T, m *FakeMouse) *harness {
	t.Helper()
	ctx, cancel := context.WithCancel(context.Background())
	c := New(m.Connector(), fast)
	h := &harness{t: t, mouse: m, client: c, ctx: ctx, cancel: cancel, done: make(chan struct{})}
	go func() { c.Run(ctx); close(h.done) }()
	t.Cleanup(func() { cancel(); <-h.done })
	return h
}

// waitFor blocks until a published state satisfies pred.
func (h *harness) waitFor(what string, pred func(state.State) bool) state.State {
	h.t.Helper()
	if s := h.client.State(); pred(s) {
		return s
	}
	timeout := time.After(3 * time.Second)
	for {
		select {
		case s, ok := <-h.client.Events():
			if !ok {
				h.t.Fatalf("events closed while waiting for %s", what)
			}
			if pred(s) {
				return s
			}
		case <-timeout:
			h.t.Fatalf("timeout waiting for %s; last state %+v", what, h.client.State())
		}
	}
}

func (h *harness) connected() state.State {
	return h.waitFor("connected", func(s state.State) bool { return s.Connection == state.Connected })
}

func (h *harness) apply(s Setting) error {
	h.t.Helper()
	ctx, cancel := context.WithTimeout(h.ctx, 3*time.Second)
	defer cancel()
	return h.client.Apply(ctx, s)
}

func TestInitialRead(t *testing.T) {
	h := start(t, NewFakeMouse())
	s := h.connected()
	want := state.State{
		Connection: state.Connected, Name: "Wireless Gaming Mouse", ProductID: protocol.ProductIDWireless,
		Battery: state.Some(87), DPI: state.Some(800), Polling: state.Some(125),
		LOD: state.Some(protocol.LOD2mm), MotionSync: state.Some(true),
		Ripple: state.Some(false), AngleSnap: state.Some(false),
		Debounce: state.Some(8), Sleep: state.Some(60),
		ReceiverLEDUnsupported: true, // the capture unit does not answer 0x88
	}
	if s != want {
		t.Errorf("got  %+v\nwant %+v", s, want)
	}
	if s.Wired() {
		t.Error("wireless PID must not be wired")
	}
}

func TestReceiverLEDSupported(t *testing.T) {
	m := NewFakeMouse()
	m.LEDSupported = true
	h := start(t, m)
	s := h.connected()
	if s.ReceiverLEDUnsupported || !s.ReceiverLED.OK || s.ReceiverLED.Val != protocol.LEDBatteryStatus {
		t.Fatalf("led: %+v", s)
	}
	if err := h.apply(ReceiverLED(protocol.LEDBatteryWarning)); err != nil {
		t.Fatal(err)
	}
	if got := h.client.State().ReceiverLED.Val; got != protocol.LEDBatteryWarning {
		t.Errorf("led = %v", got)
	}
}

func TestApplyEachSetting(t *testing.T) {
	m := NewFakeMouse()
	m.LEDSupported = true
	m.NoLOD07 = false
	h := start(t, m)
	h.connected()

	tests := []struct {
		set   Setting
		check func(s state.State) bool
	}{
		{DPI(3200), func(s state.State) bool { return s.DPI.Val == 3200 }},
		{Polling(8000), func(s state.State) bool { return s.Polling.Val == 8000 }},
		{LOD(protocol.LOD07mm), func(s state.State) bool { return s.LOD.Val == protocol.LOD07mm }},
		{LOD(protocol.LOD1mm), func(s state.State) bool { return s.LOD.Val == protocol.LOD1mm }},
		{MotionSync(false), func(s state.State) bool { return s.MotionSync.OK && !s.MotionSync.Val }},
		{RippleControl(true), func(s state.State) bool { return s.Ripple.Val }},
		{AngleSnapping(true), func(s state.State) bool { return s.AngleSnap.Val }},
		{Debounce(30), func(s state.State) bool { return s.Debounce.Val == 30 }},
		{Sleep(900), func(s state.State) bool { return s.Sleep.Val == 900 }},
		{ReceiverLED(protocol.LEDConnectAndRate), func(s state.State) bool { return s.ReceiverLED.Val == protocol.LEDConnectAndRate }},
	}
	for _, tt := range tests {
		t.Run(tt.set.String(), func(t *testing.T) {
			if err := h.apply(tt.set); err != nil {
				t.Fatal(err)
			}
			if s := h.client.State(); !tt.check(s) {
				t.Errorf("state not updated: %+v", s)
			}
		})
	}

	// The device itself, not only our state, must hold the values.
	type dev struct {
		DPI, Polling, LOD, Debounce, Sleep int
		MotionSync, Ripple, AngleSnap      bool
		LED                                protocol.LEDMode
	}
	var d dev
	m.Update(func(m *FakeMouse) {
		d = dev{m.DPI, m.Polling, m.LOD, m.Debounce, m.Sleep, m.MotionSync, m.Ripple, m.AngleSnap, m.LED}
	})
	want := dev{3200, 8000, protocol.LOD1mm, 30, 900, false, true, true, protocol.LEDConnectAndRate}
	if d != want {
		t.Errorf("device state wrong: %+v, want %+v", d, want)
	}
}

func TestApplyDoesNotChangeOtherSettings(t *testing.T) {
	h := start(t, NewFakeMouse())
	before := h.connected()
	if err := h.apply(DPI(6400)); err != nil {
		t.Fatal(err)
	}
	after := h.client.State()
	after.DPI = before.DPI
	if after != before {
		t.Errorf("side effects:\nbefore %+v\nafter  %+v", before, after)
	}
}

func TestApplyInvalidValueDoesNotTouchDevice(t *testing.T) {
	m := NewFakeMouse()
	h := start(t, m)
	h.connected()
	for _, s := range []Setting{DPI(999), Polling(1), LOD(3), Debounce(31), Sleep(0), {Kind: 99}} {
		if err := h.apply(s); !errors.Is(err, protocol.ErrInvalidValue) {
			t.Errorf("%v: err = %v", s, err)
		}
	}
	if got := h.client.State().DPI.Val; got != 800 {
		t.Errorf("dpi changed to %d", got)
	}
}

func TestApplyUnsupportedLED(t *testing.T) {
	h := start(t, NewFakeMouse())
	h.connected()
	if err := h.apply(ReceiverLED(protocol.LEDBatteryStatus)); !errors.Is(err, ErrUnsupported) {
		t.Errorf("err = %v", err)
	}
}

func TestApplyMismatch(t *testing.T) {
	m := NewFakeMouse()
	h := start(t, m)
	h.connected()
	m.Update(func(m *FakeMouse) { m.IgnoreSets = true })
	err := h.apply(DPI(1600))
	if !errors.Is(err, ErrMismatch) {
		t.Fatalf("err = %v", err)
	}
	if got := h.client.State().DPI.Val; got != 800 {
		t.Errorf("state must show the device value 800, got %d", got)
	}
}

func TestInterruptUpdatesState(t *testing.T) {
	m := NewFakeMouse()
	h := start(t, m)
	h.connected()
	m.Update(func(m *FakeMouse) { m.Battery, m.Charging, m.DPI, m.Polling = 42, true, 1600, 4000 })
	s := h.waitFor("interrupt update", func(s state.State) bool { return s.Battery.Val == 42 })
	if !s.Charging || s.DPI.Val != 1600 || s.Polling.Val != 4000 {
		t.Errorf("state: %+v", s)
	}
}

func TestRefreshRereadsSettings(t *testing.T) {
	m := NewFakeMouse()
	h := start(t, m)
	h.connected()
	m.Update(func(m *FakeMouse) { m.Debounce = 3 }) // interrupts do not carry debounce
	if got := h.client.State().Debounce.Val; got != 8 {
		t.Fatalf("precondition: %d", got)
	}
	if err := h.client.Refresh(h.ctx); err != nil {
		t.Fatal(err)
	}
	if got := h.client.State().Debounce.Val; got != 3 {
		t.Errorf("debounce = %d after refresh", got)
	}
}

func TestStartsDisconnectedThenConnects(t *testing.T) {
	m := NewFakeMouse()
	m.Plug(false)
	h := start(t, m)
	s := h.waitFor("not found", func(s state.State) bool { return s.Err != "" })
	if s.Connection != state.Disconnected {
		t.Errorf("connection = %v", s.Connection)
	}
	if err := h.apply(DPI(400)); !errors.Is(err, ErrNotConnected) {
		t.Errorf("apply while disconnected: %v", err)
	}
	m.Plug(true)
	h.connected()
}

func TestReconnectAfterUnplug(t *testing.T) {
	m := NewFakeMouse()
	h := start(t, m)
	h.connected()
	m.Plug(false)
	h.waitFor("disconnected", func(s state.State) bool { return s.Connection == state.Disconnected })
	m.Update(func(m *FakeMouse) { m.DPI = 2400 })
	m.Plug(true)
	s := h.connected()
	if s.DPI.Val != 2400 {
		t.Errorf("settings must be re-read after reconnect, dpi = %d", s.DPI.Val)
	}
}

func TestPermissionDenied(t *testing.T) {
	m := NewFakeMouse()
	m.FailOpen(ErrPermission)
	h := start(t, m)
	s := h.waitFor("permission denied", func(s state.State) bool { return s.Connection == state.PermissionDenied })
	if s.Err == "" {
		t.Error("expected error text")
	}
	m.FailOpen(nil)
	h.connected()
}

func TestSendFailureDuringApplyDisconnects(t *testing.T) {
	m := NewFakeMouse()
	h := start(t, m)
	h.connected()
	m.FailSend(errors.New("boom"))
	if err := h.apply(DPI(1600)); !errors.Is(err, ErrConnectionLost) {
		t.Fatalf("err = %v", err)
	}
	h.waitFor("disconnected", func(s state.State) bool { return s.Connection != state.Connected })
	m.FailSend(nil)
	h.connected()
}

func TestStopClosesEventsAndFailsApply(t *testing.T) {
	h := start(t, NewFakeMouse())
	h.connected()
	h.cancel()
	<-h.done
	if err := h.client.Apply(context.Background(), DPI(800)); !errors.Is(err, ErrStopped) {
		t.Errorf("err = %v", err)
	}
	for range h.client.Events() { // must terminate: channel closed
	}
}

func TestEventsKeepOnlyLatest(t *testing.T) {
	m := NewFakeMouse()
	h := start(t, m)
	h.connected()
	for i := 0; i < 5; i++ { // never read events: publisher must not block
		if err := h.apply(Debounce(i)); err != nil {
			t.Fatal(err)
		}
	}
	s := <-h.client.Events()
	if !s.Debounce.OK {
		t.Error("expected a snapshot")
	}
}

// Regression for the hardware trace: the 0x89 response keeps the pre-change DPI/polling while
// the interrupt report carries the new value. Apply must confirm from the interrupt.
func TestDPIAndPollingConfirmedFromInterruptNotStaleStatus(t *testing.T) {
	m := NewFakeMouse()
	h := start(t, m)
	h.connected()
	if err := h.apply(DPI(1600)); err != nil {
		t.Fatalf("dpi: %v", err)
	}
	if err := h.apply(Polling(1000)); err != nil {
		t.Fatalf("polling: %v", err)
	}
	if s := h.client.State(); s.DPI.Val != 1600 || s.Polling.Val != 1000 {
		t.Errorf("state %d / %d", s.DPI.Val, s.Polling.Val)
	}
}

func TestRefreshKeepsLiveDPIAndPolling(t *testing.T) {
	m := NewFakeMouse()
	h := start(t, m)
	h.connected()
	if err := h.apply(DPI(3200)); err != nil {
		t.Fatal(err)
	}
	if err := h.client.Refresh(h.ctx); err != nil { // 0x89 would say 800
		t.Fatal(err)
	}
	if got := h.client.State().DPI.Val; got != 3200 {
		t.Errorf("refresh reverted DPI to the stale %d", got)
	}
}

func TestApplySameValueIsNotAMismatch(t *testing.T) {
	h := start(t, NewFakeMouse())
	h.connected()
	m := h.mouse
	m.Update(func(m *FakeMouse) { m.InterruptEvery = time.Hour }) // no reports: a no-op change pushes nothing
	if err := h.apply(DPI(800)); err != nil {                     // already 800
		t.Errorf("err = %v", err)
	}
}

// Hardware: the mouse ignores 0.7 mm and keeps reporting 2 mm.
func TestLOD07RefusedIsFlagged(t *testing.T) {
	h := start(t, NewFakeMouse())
	if s := h.connected(); s.LOD07Unsupported {
		t.Fatal("must start unflagged")
	}
	err := h.apply(LOD(protocol.LOD07mm))
	if !errors.Is(err, ErrMismatch) {
		t.Fatalf("err = %v", err)
	}
	s := h.client.State()
	if !s.LOD07Unsupported || s.LOD.Val != protocol.LOD2mm {
		t.Errorf("state: flag=%v lod=%d", s.LOD07Unsupported, s.LOD.Val)
	}
	if err := h.apply(LOD(protocol.LOD1mm)); err != nil { // other values still work
		t.Fatal(err)
	}
	if err := h.client.Refresh(h.ctx); err != nil || !h.client.State().LOD07Unsupported {
		t.Errorf("flag must survive a refresh: %v", err)
	}
}
