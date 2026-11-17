package device

import (
	"fmt"

	"github.com/mastergap/incott-mouse-tui/internal/protocol"
	"github.com/mastergap/incott-mouse-tui/internal/state"
)

// Kind identifies a configurable setting.
type Kind int

// Setting kinds.
const (
	KindDPI Kind = iota + 1
	KindPolling
	KindLOD
	KindMotionSync
	KindRipple
	KindAngleSnap
	KindDebounce
	KindSleep
	KindReceiverLED
)

func (k Kind) String() string {
	switch k {
	case KindDPI:
		return "DPI"
	case KindPolling:
		return "Polling rate"
	case KindLOD:
		return "Lift-off distance"
	case KindMotionSync:
		return "Motion sync"
	case KindRipple:
		return "Ripple control"
	case KindAngleSnap:
		return "Angle snapping"
	case KindDebounce:
		return "Debounce"
	case KindSleep:
		return "Sleep timer"
	case KindReceiverLED:
		return "Receiver LED"
	}
	return "Setting"
}

// Setting is a requested change. Value units: DPI as-is, Polling in Hz, LOD in tenths of
// mm, Debounce in ms, Sleep in seconds, toggles 0/1, ReceiverLED as protocol.LEDMode.
type Setting struct {
	Kind  Kind
	Value int
}

// Constructors.
func DPI(v int) Setting             { return Setting{KindDPI, v} }
func Polling(hz int) Setting        { return Setting{KindPolling, hz} }
func LOD(tenths int) Setting        { return Setting{KindLOD, tenths} }
func MotionSync(on bool) Setting    { return Setting{KindMotionSync, b2i(on)} }
func RippleControl(on bool) Setting { return Setting{KindRipple, b2i(on)} }
func AngleSnapping(on bool) Setting { return Setting{KindAngleSnap, b2i(on)} }
func Debounce(ms int) Setting       { return Setting{KindDebounce, ms} }
func Sleep(seconds int) Setting     { return Setting{KindSleep, seconds} }
func ReceiverLED(m protocol.LEDMode) Setting {
	return Setting{KindReceiverLED, int(m)}
}

func (s Setting) String() string { return fmt.Sprintf("%s=%d", s.Kind, s.Value) }

func b2i(b bool) int {
	if b {
		return 1
	}
	return 0
}

// report builds the set report for the setting.
func (s Setting) report() (protocol.Report, error) {
	switch s.Kind {
	case KindDPI:
		return protocol.SetDPI(s.Value)
	case KindPolling:
		return protocol.SetPollingRate(s.Value)
	case KindLOD:
		return protocol.SetLOD(s.Value)
	case KindMotionSync:
		return protocol.SetMotionSync(s.Value != 0), nil
	case KindRipple:
		return protocol.SetRippleControl(s.Value != 0), nil
	case KindAngleSnap:
		return protocol.SetAngleSnapping(s.Value != 0), nil
	case KindDebounce:
		return protocol.SetDebounce(s.Value)
	case KindSleep:
		return protocol.SetSleep(s.Value)
	case KindReceiverLED:
		return protocol.SetReceiverLED(protocol.LEDMode(s.Value))
	}
	return protocol.Report{}, fmt.Errorf("%w: unknown setting kind %d", protocol.ErrInvalidValue, s.Kind)
}

// request returns the read request that reports the setting's current value.
func (k Kind) request() protocol.Request {
	switch k {
	case KindDPI, KindPolling:
		return protocol.ReadStatus
	case KindLOD, KindMotionSync:
		return protocol.ReadLODMotionSync
	case KindRipple:
		return protocol.ReadRippleControl
	case KindAngleSnap:
		return protocol.ReadAngleSnapping
	case KindDebounce:
		return protocol.ReadDebounce
	case KindSleep:
		return protocol.ReadSleep
	}
	return protocol.ReadReceiverLED
}

// store parses a response for kind k into st.
func store(st *state.State, k Kind, buf []byte) error {
	switch k {
	case KindDPI, KindPolling:
		s, err := protocol.ParseStatusResponse(buf)
		if err != nil {
			return err
		}
		applyStatus(st, s)
	case KindLOD, KindMotionSync:
		lod, ms, err := protocol.ParseLODMotionSync(buf)
		if err != nil {
			return err
		}
		st.LOD, st.MotionSync = state.Some(lod), state.Some(ms)
	case KindRipple:
		v, err := protocol.ParseRippleControl(buf)
		if err != nil {
			return err
		}
		st.Ripple = state.Some(v)
	case KindAngleSnap:
		v, err := protocol.ParseAngleSnapping(buf)
		if err != nil {
			return err
		}
		st.AngleSnap = state.Some(v)
	case KindDebounce:
		v, err := protocol.ParseDebounce(buf)
		if err != nil {
			return err
		}
		st.Debounce = state.Some(v)
	case KindSleep:
		v, err := protocol.ParseSleep(buf)
		if err != nil {
			return err
		}
		st.Sleep = state.Some(v)
	case KindReceiverLED:
		v, err := protocol.ParseReceiverLED(buf)
		if err != nil {
			return err
		}
		st.ReceiverLED = state.Some(v)
	}
	return nil
}

func applyStatus(st *state.State, s protocol.Status) {
	st.Battery = state.Some(s.Battery)
	st.Charging = s.Charging
	st.DPI = state.Some(s.DPI)
	st.Polling = state.Some(s.Polling)
}

// Value returns the current value of setting kind k in st, and whether it is known.
func Value(st state.State, k Kind) (int, bool) { return value(&st, k) }

// value reads the current value of kind k from st.
func value(st *state.State, k Kind) (int, bool) {
	switch k {
	case KindDPI:
		return st.DPI.Val, st.DPI.OK
	case KindPolling:
		return st.Polling.Val, st.Polling.OK
	case KindLOD:
		return st.LOD.Val, st.LOD.OK
	case KindMotionSync:
		return b2i(st.MotionSync.Val), st.MotionSync.OK
	case KindRipple:
		return b2i(st.Ripple.Val), st.Ripple.OK
	case KindAngleSnap:
		return b2i(st.AngleSnap.Val), st.AngleSnap.OK
	case KindDebounce:
		return st.Debounce.Val, st.Debounce.OK
	case KindSleep:
		return st.Sleep.Val, st.Sleep.OK
	case KindReceiverLED:
		return int(st.ReceiverLED.Val), st.ReceiverLED.OK
	}
	return 0, false
}

// assume records v as the value of kind k without reading it back (optimistic).
func assume(st *state.State, k Kind, v int) {
	switch k {
	case KindDPI:
		st.DPI = state.Some(v)
	case KindPolling:
		st.Polling = state.Some(v)
	case KindLOD:
		st.LOD = state.Some(v)
	case KindMotionSync:
		st.MotionSync = state.Some(v != 0)
	case KindRipple:
		st.Ripple = state.Some(v != 0)
	case KindAngleSnap:
		st.AngleSnap = state.Some(v != 0)
	case KindDebounce:
		st.Debounce = state.Some(v)
	case KindSleep:
		st.Sleep = state.Some(v)
	case KindReceiverLED:
		st.ReceiverLED = state.Some(protocol.LEDMode(v))
	}
}
