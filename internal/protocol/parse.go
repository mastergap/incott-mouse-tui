package protocol

import (
	"errors"
	"fmt"
)

// Status is the device status: battery, DPI and polling rate.
type Status struct {
	Battery  int  // percent, 0–100
	Charging bool // INFERRED: the device sets +128 on the battery byte; unverified on hardware
	DPI      int
	Polling  int // Hz
}

// ErrNotReady is returned when the device answered a status request with its empty placeholder.
var ErrNotReady = errors.New("status not ready")

func checkLen(buf []byte) error {
	if len(buf) < ReportLen {
		return fmt.Errorf("%w: %d bytes", ErrShortReport, len(buf))
	}
	if buf[0] != ReportID {
		return fmt.Errorf("%w: report id 0x%02X", ErrUnexpected, buf[0])
	}
	return nil
}

// IsInterruptStatus reports whether buf, read from the interrupt endpoint, is a status
// report. Interrupt reports are not command echoes: byte 1 is the battery.
// Captured example (hardware; 33 bytes on macOS, first 9 significant):
//
//	09 57 13 08 00 0F 0F 11 00 ...   battery 0x57=87%, 0x13 = DPI idx 1 (800) | rate idx 3 (125 Hz)
func IsInterruptStatus(buf []byte) bool {
	return len(buf) >= ReportLen && buf[0] == ReportID && buf[1] > 0
}

// ParseInterruptStatus decodes an interrupt status report: buf[1] battery (+128 while
// charging), buf[2] high nibble DPI index, low nibble polling index.
func ParseInterruptStatus(buf []byte) (Status, error) {
	if err := checkLen(buf); err != nil {
		return Status{}, err
	}
	return decodeStatus(buf[1], buf[2]>>4, buf[2]&0x0F)
}

// ParseStatusResponse decodes the feature-report response to ReadStatus. Unlike the interrupt
// report it echoes the command: buf[1]==0x89, buf[2] battery, buf[3] DPI index, buf[4]
// polling index. Captured example:
//
//	09 89 57 01 03 08 00 01 5A   battery 87%, DPI idx 1 (800), rate idx 3 (125 Hz)
//
// An all-zero payload (09 89 00 00 ...) is the firmware's "no data yet" placeholder and
// yields ErrNotReady.
func ParseStatusResponse(buf []byte) (Status, error) {
	if err := checkLen(buf); err != nil {
		return Status{}, err
	}
	if buf[1] != cmdStatus|readFlag {
		return Status{}, fmt.Errorf("%w: command 0x%02X", ErrUnexpected, buf[1])
	}
	if buf[2] == 0 && buf[3] == 0 {
		return Status{}, ErrNotReady
	}
	return decodeStatus(buf[2], buf[3], buf[4])
}

func decodeStatus(batteryByte, dpiIdx, hzIdx byte) (Status, error) {
	bat := int(batteryByte)
	charging := false
	if bat > 100 {
		bat -= 128
		charging = true
	}
	if bat < 0 || bat > 100 {
		return Status{}, fmt.Errorf("%w: battery byte 0x%02X", ErrUnexpected, batteryByte)
	}
	if int(dpiIdx) >= len(DPIValues) || int(hzIdx) >= len(PollingRates) {
		return Status{}, fmt.Errorf("%w: dpi idx %d, rate idx %d", ErrUnexpected, dpiIdx, hzIdx)
	}
	return Status{Battery: bat, Charging: charging, DPI: DPIValues[dpiIdx], Polling: PollingRates[hzIdx]}, nil
}

// ParseLODMotionSync decodes the response to ReadLODMotionSync: buf[7] high nibble is the
// LOD (tenths of mm), low nibble is motion sync.
func ParseLODMotionSync(buf []byte) (lodTenths int, motionSync bool, err error) {
	if err = checkLen(buf); err != nil {
		return 0, false, err
	}
	switch buf[7] >> 4 {
	case 0x0:
		lodTenths = LOD1mm
	case 0x1:
		lodTenths = LOD2mm
	case 0x2:
		lodTenths = LOD07mm
	default:
		return 0, false, fmt.Errorf("%w: lod nibble 0x%X", ErrUnexpected, buf[7]>>4)
	}
	return lodTenths, buf[7]&0x0F == 0x01, nil
}

// ParseRippleControl decodes the response to ReadRippleControl.
func ParseRippleControl(buf []byte) (bool, error) { return parseToggle(buf) }

// ParseAngleSnapping decodes the response to ReadAngleSnapping.
func ParseAngleSnapping(buf []byte) (bool, error) { return parseToggle(buf) }

func parseToggle(buf []byte) (bool, error) {
	if err := checkLen(buf); err != nil {
		return false, err
	}
	return buf[3] == 0x01, nil
}

// ParseDebounce decodes the response to ReadDebounce (0–30 ms).
func ParseDebounce(buf []byte) (int, error) {
	if err := checkLen(buf); err != nil {
		return 0, err
	}
	if buf[3] > MaxDebounceMS {
		return 0, fmt.Errorf("%w: debounce %d", ErrUnexpected, buf[3])
	}
	return int(buf[3]), nil
}

// ParseSleep decodes the response to ReadSleep: little-endian seconds in buf[3:5], 1–900.
func ParseSleep(buf []byte) (int, error) {
	if err := checkLen(buf); err != nil {
		return 0, err
	}
	v := int(buf[3]) | int(buf[4])<<8
	if v < 1 || v > MaxSleepSeconds {
		return 0, fmt.Errorf("%w: sleep %d", ErrUnexpected, v)
	}
	return v, nil
}

// ParseReceiverLED decodes the response to ReadReceiverLED (mode in buf[2]).
func ParseReceiverLED(buf []byte) (LEDMode, error) {
	if err := checkLen(buf); err != nil {
		return 0, err
	}
	if buf[2] > byte(LEDBatteryWarning) {
		return 0, fmt.Errorf("%w: led mode %d", ErrUnexpected, buf[2])
	}
	return LEDMode(buf[2]), nil
}
