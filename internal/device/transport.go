// Package device owns the connection to the mouse: discovery, the HID transport, the
// settings read/write flow and reconnection. All device I/O happens on a single goroutine
// (Client.Run), so there is no locking around the handle.
package device

import (
	"errors"
	"sort"

	"github.com/mastergap/incott-mouse-tui/internal/protocol"
)

// Transport is the minimal HID handle the client needs. *hid.Device satisfies it through
// an adapter (hid.go); tests use FakeMouse.
type Transport interface {
	// SendFeature sends a feature report (first byte = report ID).
	SendFeature(report []byte) error
	// GetFeature fills buf (buf[0] = report ID) with a feature report and returns the length.
	GetFeature(buf []byte) (int, error)
	// ReadInterrupt reads an input report, waiting at most timeoutMS. It returns (0, nil)
	// when nothing arrived in time.
	ReadInterrupt(buf []byte, timeoutMS int) (int, error)
	Close() error
}

// Info describes the device a Transport is connected to.
type Info struct {
	Name      string
	ProductID uint16
}

// Connector finds and opens the mouse.
type Connector interface {
	// Connect returns ErrNotFound when no mouse is present and ErrPermission when a mouse is
	// present but cannot be opened. Other errors are treated like ErrNotFound with a message.
	Connect() (Transport, Info, error)
}

// Errors.
var (
	ErrNotFound       = errors.New("mouse not found")
	ErrPermission     = errors.New("cannot open mouse (permission denied or in use)")
	ErrNotConnected   = errors.New("mouse not connected")
	ErrConnectionLost = errors.New("connection to mouse lost")
	ErrUnsupported    = errors.New("setting not supported by this mouse")
	ErrMismatch       = errors.New("mouse reports a different value than requested")
	ErrTimeout        = errors.New("no response from mouse")
	ErrStopped        = errors.New("client stopped")
)

// vendorInterface is the USB interface number of the configuration collection (observed on
// macOS). Used only when the OS does not report usage pages (Linux/libusb).
const vendorInterface = 2

// Candidate is a HID interface found during enumeration.
type Candidate struct {
	Path      string
	ProductID uint16
	Product   string
	UsagePage uint16
	Usage     uint16
	Interface int
}

// SelectCandidates filters and orders interfaces worth trying, best first:
//  1. usage page 0xFF05 (the configuration collection; the only one delivering status interrupts),
//  2. other vendor usage pages >= 0xFF00 (feature reports work, interrupts do not),
//  3. interfaces with no usage page information (Linux) at the vendor interface number.
//
// Only known product IDs whose product string looks like a mouse are considered.
func SelectCandidates(all []Candidate) []Candidate {
	type ranked struct {
		c    Candidate
		rank int
	}
	var rs []ranked
	for _, c := range all {
		if !protocol.IsKnownProductID(c.ProductID) || !protocol.IsMouseProduct(c.Product) {
			continue
		}
		switch {
		case c.UsagePage == protocol.UsagePageVendor:
			rs = append(rs, ranked{c, 0})
		case c.UsagePage >= protocol.UsagePageVendorMin:
			rs = append(rs, ranked{c, 1})
		case c.UsagePage == 0 && c.Interface == vendorInterface:
			rs = append(rs, ranked{c, 2})
		}
	}
	sort.SliceStable(rs, func(i, j int) bool { return rs[i].rank < rs[j].rank })
	out := make([]Candidate, len(rs))
	for i, r := range rs {
		out[i] = r.c
	}
	return out
}
