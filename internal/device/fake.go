package device

import (
	"errors"
	"sync"
	"time"

	"github.com/mastergap/incott-mouse-tui/internal/protocol"
)

// FakeMouse emulates an Incott mouse for tests and the --demo mode. It answers the real
// protocol (layouts taken from hardware captures, see docs/protocol.md), so the full client
// stack runs against it unchanged.
//
// Set commands are recognised by regenerating every valid set report with the protocol
// builders, so the fake stays correct if the wire encoding changes.
type FakeMouse struct {
	mu sync.Mutex

	// Device state (changed by set commands).
	Battery    int
	Charging   bool
	DPI        int
	Polling    int
	LOD        int // tenths of mm
	MotionSync bool
	Ripple     bool
	AngleSnap  bool
	Debounce   int
	Sleep      int
	LED        protocol.LEDMode

	// Behaviour switches.
	Name           string
	ProductID      uint16
	NoLOD07        bool          // ignore the 0.7 mm LOD command, like the hardware unit
	IgnoreSets     bool          // accept set commands but do not apply them (simulates a stuck device)
	LEDSupported   bool          // false: receiver LED reads are never answered (like the hardware unit)
	InterruptEvery time.Duration // period of unsolicited status reports (default 1s)

	snapDPI, snapPolling int // what the 0x89 response reports: a stale snapshot, like the hardware
	plugged              bool
	openErr              error
	sendErr              error
	pending              protocol.Report // response to the next GetFeature
	hasPending           bool
	gen                  int           // incremented on Unplug; transports of older generations fail
	wake                 chan struct{} // kicks ReadInterrupt
	lastIntr             time.Time
	setTable             map[protocol.Report]func(*FakeMouse)
}

// NewFakeMouse returns a plugged-in mouse with the settings captured from real hardware.
func NewFakeMouse() *FakeMouse {
	m := &FakeMouse{
		Battery: 87, DPI: 800, Polling: 125, LOD: protocol.LOD2mm, MotionSync: true,
		Debounce: 8, Sleep: 60, LED: protocol.LEDBatteryStatus,
		Name: "Wireless Gaming Mouse", ProductID: protocol.ProductIDWireless,
		LEDSupported: false, NoLOD07: true, InterruptEvery: time.Second,
		plugged: true, wake: make(chan struct{}, 1),
	}
	m.snapDPI, m.snapPolling = m.DPI, m.Polling
	m.buildSetTable()
	return m
}

func (m *FakeMouse) buildSetTable() {
	t := map[protocol.Report]func(*FakeMouse){}
	for _, v := range protocol.PollingRates {
		r, _ := protocol.SetPollingRate(v)
		t[r] = func(m *FakeMouse) { m.Polling = v }
	}
	for _, v := range protocol.DPIValues {
		r, _ := protocol.SetDPI(v)
		t[r] = func(m *FakeMouse) { m.DPI = v }
	}
	for _, v := range protocol.LODValues {
		r, _ := protocol.SetLOD(v)
		t[r] = func(m *FakeMouse) { m.LOD = v }
	}
	for _, on := range []bool{false, true} {
		t[protocol.SetMotionSync(on)] = func(m *FakeMouse) { m.MotionSync = on }
		t[protocol.SetRippleControl(on)] = func(m *FakeMouse) { m.Ripple = on }
		t[protocol.SetAngleSnapping(on)] = func(m *FakeMouse) { m.AngleSnap = on }
	}
	for v := 0; v <= protocol.MaxDebounceMS; v++ {
		r, _ := protocol.SetDebounce(v)
		t[r] = func(m *FakeMouse) { m.Debounce = v }
	}
	for v := 1; v <= protocol.MaxSleepSeconds; v++ {
		r, _ := protocol.SetSleep(v)
		t[r] = func(m *FakeMouse) { m.Sleep = v }
	}
	for _, v := range protocol.LEDModes {
		r, _ := protocol.SetReceiverLED(v)
		t[r] = func(m *FakeMouse) { m.LED = v }
	}
	m.setTable = t
}

// Connector returns a Connector that opens this fake mouse.
func (m *FakeMouse) Connector() Connector { return fakeConnector{m} }

// Plug connects or disconnects the mouse. Unplugging breaks open transports.
func (m *FakeMouse) Plug(plugged bool) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.plugged && !plugged {
		m.gen++
	}
	m.plugged = plugged
	m.kick()
}

// FailOpen makes Connect fail with err (e.g. ErrPermission); nil clears it.
func (m *FakeMouse) FailOpen(err error) {
	m.mu.Lock()
	m.openErr = err
	m.mu.Unlock()
}

// FailSend makes every feature-report send fail with err; nil clears it.
func (m *FakeMouse) FailSend(err error) {
	m.mu.Lock()
	m.sendErr = err
	m.mu.Unlock()
}

// Update changes device state under the mouse lock and pushes a status report, like the
// firmware does when the user presses a DPI button or the battery level changes.
func (m *FakeMouse) Update(f func(*FakeMouse)) {
	m.mu.Lock()
	f(m)
	m.lastIntr = time.Time{} // force an immediate report
	m.kick()
	m.mu.Unlock()
}

func (m *FakeMouse) kick() {
	select {
	case m.wake <- struct{}{}:
	default:
	}
}

type fakeConnector struct{ m *FakeMouse }

func (c fakeConnector) Connect() (Transport, Info, error) {
	m := c.m
	m.mu.Lock()
	defer m.mu.Unlock()
	if !m.plugged {
		return nil, Info{}, ErrNotFound
	}
	if m.openErr != nil {
		return nil, Info{}, m.openErr
	}
	m.snapDPI, m.snapPolling = m.DPI, m.Polling // the snapshot is taken when the device (re)connects
	return &fakeTransport{m: m, gen: m.gen}, Info{Name: m.Name, ProductID: m.ProductID}, nil
}

var errFakeGone = errors.New("fake: device unplugged")

type fakeTransport struct {
	m      *FakeMouse
	gen    int
	closed bool
}

func (t *fakeTransport) alive() error {
	if t.closed || t.gen != t.m.gen || !t.m.plugged {
		return errFakeGone
	}
	return nil
}

func (t *fakeTransport) Close() error {
	t.m.mu.Lock()
	t.closed = true
	t.m.mu.Unlock()
	return nil
}

func (t *fakeTransport) SendFeature(r []byte) error {
	m := t.m
	m.mu.Lock()
	defer m.mu.Unlock()
	if err := t.alive(); err != nil {
		return err
	}
	if m.sendErr != nil {
		return m.sendErr
	}
	var rep protocol.Report
	copy(rep[:], r)
	if set, ok := m.setTable[rep]; ok {
		if !m.IgnoreSets {
			prev := m.LOD
			set(m)
			if m.NoLOD07 && m.LOD == protocol.LOD07mm {
				m.LOD = prev
			}
			m.lastIntr = time.Time{} // the firmware pushes a status report after a change
			m.kick()
		}
		return nil
	}
	if resp, ok := m.respond(rep); ok {
		m.pending, m.hasPending = resp, true
	}
	return nil
}

func (t *fakeTransport) GetFeature(b []byte) (int, error) {
	m := t.m
	m.mu.Lock()
	defer m.mu.Unlock()
	if err := t.alive(); err != nil {
		return 0, err
	}
	if len(b) > 0 {
		b[0] = protocol.ReportID
	}
	if m.hasPending {
		return copy(b, m.pending[:]), nil
	}
	return 0, nil // nothing staged: the buffer keeps only the report ID
}

func (t *fakeTransport) ReadInterrupt(b []byte, timeoutMS int) (int, error) {
	m := t.m
	deadline := time.Now().Add(time.Duration(timeoutMS) * time.Millisecond)
	for {
		m.mu.Lock()
		if err := t.alive(); err != nil {
			m.mu.Unlock()
			return 0, err
		}
		if time.Since(m.lastIntr) >= m.InterruptEvery {
			m.lastIntr = time.Now()
			n := copy(b, m.interruptReport())
			m.mu.Unlock()
			return n, nil
		}
		m.mu.Unlock()
		remaining := time.Until(deadline)
		if remaining <= 0 {
			return 0, nil
		}
		select {
		case <-m.wake:
		case <-time.After(remaining):
			return 0, nil
		}
	}
}

func idx(vals []int, v int) byte {
	for i, x := range vals {
		if x == v {
			return byte(i)
		}
	}
	return 0
}

func (m *FakeMouse) batteryByte() byte {
	if m.Charging {
		return byte(m.Battery + 128)
	}
	return byte(m.Battery)
}

// interruptReport: 09 <battery> <dpiIdx<<4|rateIdx> 08 00 0F 0F <lod|motion> ... (33 bytes, as on macOS).
func (m *FakeMouse) interruptReport() []byte {
	out := make([]byte, 33)
	out[0], out[1] = protocol.ReportID, m.batteryByte()
	out[2] = idx(protocol.DPIValues[:], m.DPI)<<4 | idx(protocol.PollingRates[:], m.Polling)
	out[3], out[5], out[6], out[7] = 0x08, 0x0F, 0x0F, m.lodMotionByte()
	return out
}

func (m *FakeMouse) lodMotionByte() byte {
	var lod byte
	switch m.LOD {
	case protocol.LOD2mm:
		lod = 1
	case protocol.LOD07mm:
		lod = 2
	}
	b := lod << 4
	if m.MotionSync {
		b |= 1
	}
	return b
}

func boolB(v bool) byte {
	if v {
		return 1
	}
	return 0
}

// respond builds the response to a read request, mirroring the captured layouts.
func (m *FakeMouse) respond(req protocol.Report) (protocol.Report, bool) {
	tail := func(r protocol.Report) protocol.Report { r[5], r[6], r[7] = 0x0F, 0x0F, m.lodMotionByte(); return r }
	switch req {
	case protocol.ReadStatus.Report:
		return protocol.Report{9, 0x89, m.batteryByte(), idx(protocol.DPIValues[:], m.snapDPI),
			idx(protocol.PollingRates[:], m.snapPolling), 0x08, 0x00, 0x01, 0x5A}, true
	case protocol.ReadLODMotionSync.Report:
		return tail(protocol.Report{9, 0x84, 0x00, 0x08}), true
	case protocol.ReadRippleControl.Report:
		return tail(protocol.Report{9, 0x84, 0x02, boolB(m.Ripple)}), true
	case protocol.ReadAngleSnapping.Report:
		return tail(protocol.Report{9, 0x84, 0x03, boolB(m.AngleSnap)}), true
	case protocol.ReadDebounce.Report:
		return tail(protocol.Report{9, 0x85, 0x01, byte(m.Debounce)}), true
	case protocol.ReadSleep.Report:
		return tail(protocol.Report{9, 0x85, 0x03, byte(m.Sleep & 0xFF), byte(m.Sleep >> 8)}), true
	case protocol.ReadReceiverLED.Report:
		if m.LEDSupported {
			return protocol.Report{9, 0x88, byte(m.LED)}, true
		}
	}
	return protocol.Report{}, false
}
