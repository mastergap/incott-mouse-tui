package protocol

// ---- Set commands ----------------------------------------------------------------------

// SetPollingRate builds the report that sets the polling rate (125–8000 Hz).
func SetPollingRate(hz int) (Report, error) {
	i, ok := indexOf(PollingRates[:], hz)
	if !ok {
		return Report{}, invalid("polling rate", hz)
	}
	return setReport(cmdPolling, byte(i)), nil
}

// SetDPI builds the report that selects a DPI preset.
func SetDPI(dpi int) (Report, error) {
	i, ok := indexOf(DPIValues[:], dpi)
	if !ok {
		return Report{}, invalid("dpi", dpi)
	}
	return setReport(cmdDPI, dpiSelector, byte(i)), nil
}

// SetLOD builds the report that sets the lift-off distance (tenths of mm: 7, 10 or 20).
func SetLOD(tenths int) (Report, error) {
	var b byte
	switch tenths {
	case LOD1mm:
		b = 0x00
	case LOD2mm:
		b = 0x01
	case LOD07mm:
		b = 0x02
	default:
		return Report{}, invalid("lod", tenths)
	}
	return setReport(cmdToggle, subLOD, b), nil
}

// SetRippleControl builds the report that toggles ripple control.
func SetRippleControl(on bool) Report { return setReport(cmdToggle, subRipple, boolByte(on)) }

// SetAngleSnapping builds the report that toggles angle snapping.
func SetAngleSnapping(on bool) Report { return setReport(cmdToggle, subAngle, boolByte(on)) }

// SetMotionSync builds the report that toggles motion sync.
func SetMotionSync(on bool) Report { return setReport(cmdToggle, subMotion, boolByte(on)) }

// SetDebounce builds the report that sets the button debounce time (0–30 ms).
func SetDebounce(ms int) (Report, error) {
	if ms < 0 || ms > MaxDebounceMS {
		return Report{}, invalid("debounce", ms)
	}
	return setReport(cmdTiming, subDebounce, byte(ms)), nil
}

// SetSleep builds the report that sets the sleep timeout (1–900 seconds, little endian).
func SetSleep(seconds int) (Report, error) {
	if seconds < 1 || seconds > MaxSleepSeconds {
		return Report{}, invalid("sleep", seconds)
	}
	return setReport(cmdTiming, subSleep, byte(seconds&0xFF), byte(seconds>>8)), nil
}

// SetReceiverLED builds the report that sets the receiver LED mode.
func SetReceiverLED(m LEDMode) (Report, error) {
	if m < LEDConnectAndRate || m > LEDBatteryWarning {
		return Report{}, invalid("receiver led", int(m))
	}
	return setReport(cmdLED, byte(m)), nil
}

// ---- Read requests ---------------------------------------------------------------------

// Read requests. Each response is decoded by the matching Parse* function.
var (
	ReadStatus        = readRequest(cmdStatus, 0)
	ReadLODMotionSync = readRequest(cmdToggle, 0)
	ReadRippleControl = readRequest(cmdToggle, subRipple)
	ReadAngleSnapping = readRequest(cmdToggle, subAngle)
	ReadDebounce      = readRequest(cmdTiming, subDebounce)
	ReadSleep         = readRequest(cmdTiming, subSleep)
	ReadReceiverLED   = readRequest(cmdLED, 0)
)
