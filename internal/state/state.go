// Package state defines the immutable-by-copy snapshot of the mouse that the device layer
// publishes and the UI renders.
package state

import "github.com/mastergap/incott-mouse-tui/internal/protocol"

// Opt is a value that may be unknown (not yet read, or unreadable).
type Opt[T any] struct {
	Val T
	OK  bool
}

// Some returns a known Opt.
func Some[T any](v T) Opt[T] { return Opt[T]{Val: v, OK: true} }

// Connection is the link state between the app and the mouse.
type Connection int

// Connection states.
const (
	Disconnected     Connection = iota // no device found; the client keeps searching
	Connecting                         // device found, reading its settings
	Connected                          // device ready
	PermissionDenied                   // device found but cannot be opened (OS permissions)
)

func (c Connection) String() string {
	switch c {
	case Disconnected:
		return "Not connected"
	case Connecting:
		return "Connecting"
	case Connected:
		return "Connected"
	case PermissionDenied:
		return "Permission denied"
	}
	return "Unknown"
}

// State is a snapshot of everything the app knows about the mouse. It contains no
// references, so a copy is safe to hand to another goroutine.
type State struct {
	Connection Connection
	Err        string // last connection error, empty when none

	Name      string // HID product string
	ProductID uint16 // protocol.ProductIDWireless / ProductIDCharging

	Battery  Opt[int] // percent
	Charging bool     // inferred from the battery byte; see protocol.Status

	DPI     Opt[int]
	Polling Opt[int] // Hz

	LOD              Opt[int] // tenths of mm
	LOD07Unsupported bool     // the mouse refused the 0.7 mm setting (it keeps reporting another value)
	MotionSync       Opt[bool]
	Ripple           Opt[bool]
	AngleSnap        Opt[bool]
	Debounce         Opt[int] // ms
	Sleep            Opt[int] // seconds

	ReceiverLED            Opt[protocol.LEDMode]
	ReceiverLEDUnsupported bool // the device never answered the receiver LED read
}

// Wired reports whether the mouse is connected through the charging cable PID.
func (s State) Wired() bool { return s.ProductID == protocol.ProductIDCharging }
