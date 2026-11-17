package protocol

import "strconv"

// ---- Presets and their wire encodings -------------------------------------------------

// DPIValues are the selectable DPI presets, indexed by their wire value.
var DPIValues = [...]int{400, 800, 1600, 2400, 3200, 6400}

// PollingRates are the selectable polling rates (Hz), indexed by their wire value.
var PollingRates = [...]int{1000, 500, 250, 125, 8000, 4000, 2000}

// PollingRatesAscending lists polling rates in the order a UI should present them.
var PollingRatesAscending = [...]int{125, 250, 500, 1000, 2000, 4000, 8000}

// LOD (lift-off distance) is expressed in tenths of a millimetre.
const (
	LOD07mm = 7
	LOD1mm  = 10
	LOD2mm  = 20
)

// LODValues lists the selectable lift-off distances in ascending order (tenths of mm).
var LODValues = [...]int{LOD07mm, LOD1mm, LOD2mm}

// LEDMode is the receiver LED mode.
type LEDMode int

// Receiver LED modes (wire value = constant value).
const (
	LEDConnectAndRate LEDMode = iota
	LEDBatteryStatus
	LEDBatteryWarning
)

// LEDModes lists all receiver LED modes in wire order.
var LEDModes = [...]LEDMode{LEDConnectAndRate, LEDBatteryStatus, LEDBatteryWarning}

// Limits.
const (
	MaxDebounceMS   = 30
	MaxSleepSeconds = 900
)

// SleepValues are the selectable sleep timeouts in seconds.
var SleepValues = [...]int{10, 20, 30, 60, 120, 180, 240, 300, 360, 420, 480, 540, 600, 660, 720, 780, 840, 900}

func indexOf(vals []int, v int) (int, bool) {
	for i, x := range vals {
		if x == v {
			return i, true
		}
	}
	return 0, false
}

// ---- Labels ----------------------------------------------------------------------------

// DPILabel formats a DPI value, e.g. "1600".
func DPILabel(dpi int) string { return strconv.Itoa(dpi) }

// PollingLabel formats a polling rate, e.g. "8000 Hz".
func PollingLabel(hz int) string { return strconv.Itoa(hz) + " Hz" }

// LODLabel formats a lift-off distance given in tenths of mm: "0.7 mm", "1 mm", "2 mm".
func LODLabel(tenths int) string {
	switch tenths {
	case LOD07mm:
		return "0.7 mm"
	case LOD1mm:
		return "1 mm"
	case LOD2mm:
		return "2 mm"
	}
	return "—"
}

// DebounceLabel formats a debounce time, e.g. "4 ms".
func DebounceLabel(ms int) string { return strconv.Itoa(ms) + " ms" }

// SleepLabel formats a sleep timeout: "30 sec", "5 min". Non-minute values above a
// minute (not selectable, but possible on the wire) are shown in seconds.
func SleepLabel(seconds int) string {
	if seconds >= 60 && seconds%60 == 0 {
		return strconv.Itoa(seconds/60) + " min"
	}
	return strconv.Itoa(seconds) + " sec"
}

// String returns the human-readable name of the LED mode.
func (m LEDMode) String() string {
	switch m {
	case LEDConnectAndRate:
		return "Connect & polling rate"
	case LEDBatteryStatus:
		return "Battery status"
	case LEDBatteryWarning:
		return "Battery warning"
	}
	return "—"
}

// OnOff formats a boolean toggle.
func OnOff(v bool) string {
	if v {
		return "On"
	}
	return "Off"
}
