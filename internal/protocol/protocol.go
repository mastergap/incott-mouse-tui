// Package protocol implements the Incott mouse HID protocol as pure functions:
// building 9-byte feature reports, parsing responses and mapping setting values.
// It performs no I/O and is fully unit-tested.
//
// Wire format (reverse engineered from the vendor WebHID driver): every report is
// 9 bytes with report ID 0x09. A "read" command is the matching "set" command
// with the high bit (0x80) set. There is no checksum.
package protocol

import (
	"errors"
	"fmt"
	"strings"
)

// Device identification.
const (
	VendorID          uint16 = 0x093A
	ProductIDWireless uint16 = 0x522C
	ProductIDCharging uint16 = 0x622C

	// UsagePageVendor is the vendor-defined usage page of the configuration interface.
	UsagePageVendor uint16 = 0xFF05
	// UsagePageVendorMin is the lowest usage page accepted when UsagePageVendor is absent.
	UsagePageVendorMin uint16 = 0xFF00
)

// Report layout.
const (
	ReportID  byte = 0x09
	ReportLen      = 9
	readFlag  byte = 0x80
)

// Command bytes (set direction).
const (
	cmdPolling  byte = 0x01
	cmdDPI      byte = 0x03
	cmdToggle   byte = 0x04 // LOD / ripple / angle snapping / motion sync, selected by sub-command
	cmdTiming   byte = 0x05 // debounce / sleep, selected by sub-command
	cmdLED      byte = 0x08
	cmdStatus   byte = 0x09 // only exists as read: 0x89
	subLOD      byte = 0x01
	subRipple   byte = 0x02
	subAngle    byte = 0x03
	subMotion   byte = 0x04
	subDebounce byte = 0x01
	subSleep    byte = 0x03
	dpiSelector byte = 0x06 // second byte of the DPI command
)

// Report is a 9-byte HID report including the report ID.
type Report [ReportLen]byte

// Request describes a read command and how to recognise its response.
type Request struct {
	Report Report
	cmd    byte // expected response command byte (request command with the read flag)
	sub    byte // expected response sub-command byte; 0 = not checked
}

// Matches reports whether buf is the response to this request.
// A response echoes the command byte: buf[0]==0x09, buf[1]==cmd and, if the request has a
// sub-command, buf[2]==sub.
func (r Request) Matches(buf []byte) bool {
	if len(buf) < ReportLen || buf[0] != ReportID || buf[1] != r.cmd {
		return false
	}
	return r.sub == 0 || buf[2] == r.sub
}

// Validation errors.
var (
	ErrInvalidValue = errors.New("invalid setting value")
	ErrShortReport  = errors.New("report too short")
	ErrUnexpected   = errors.New("unexpected report")
)

func invalid(name string, v any) error {
	return fmt.Errorf("%w: %s=%v", ErrInvalidValue, name, v)
}

func setReport(cmd byte, args ...byte) Report {
	r := Report{ReportID, cmd}
	copy(r[2:], args)
	return r
}

func readRequest(cmd, sub byte) Request {
	return Request{
		Report: setReport(cmd|readFlag, sub),
		cmd:    cmd | readFlag,
		sub:    sub,
	}
}

func boolByte(v bool) byte {
	if v {
		return 0x01
	}
	return 0x00
}

// mouseKeywords are lowercase fragments of the HID product string of known mice.
// Incott keyboards share the vendor ID, so the product string is used to tell them apart.
var mouseKeywords = []string{"mouse", "g23", "g24", "ghero", "zero 29", "zero 39"}

// IsMouseProduct reports whether a HID product string looks like a supported mouse.
// An empty string is accepted because some firmwares do not report one.
func IsMouseProduct(product string) bool {
	if product == "" {
		return true
	}
	p := strings.ToLower(product)
	for _, kw := range mouseKeywords {
		if strings.Contains(p, kw) {
			return true
		}
	}
	return false
}

// IsKnownProductID reports whether pid is one of the supported product IDs.
func IsKnownProductID(pid uint16) bool {
	return pid == ProductIDWireless || pid == ProductIDCharging
}
