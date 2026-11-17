package protocol

import (
	"errors"
	"testing"
)

func TestSetReportsGolden(t *testing.T) {
	mustR := func(r Report, err error) Report {
		t.Helper()
		if err != nil {
			t.Fatal(err)
		}
		return r
	}
	tests := []struct {
		name string
		got  Report
		want Report
	}{
		{"hz 1000", mustR(SetPollingRate(1000)), Report{0x09, 0x01, 0x00}},
		{"hz 500", mustR(SetPollingRate(500)), Report{0x09, 0x01, 0x01}},
		{"hz 250", mustR(SetPollingRate(250)), Report{0x09, 0x01, 0x02}},
		{"hz 125", mustR(SetPollingRate(125)), Report{0x09, 0x01, 0x03}},
		{"hz 8000", mustR(SetPollingRate(8000)), Report{0x09, 0x01, 0x04}},
		{"hz 4000", mustR(SetPollingRate(4000)), Report{0x09, 0x01, 0x05}},
		{"hz 2000", mustR(SetPollingRate(2000)), Report{0x09, 0x01, 0x06}},
		{"dpi 400", mustR(SetDPI(400)), Report{0x09, 0x03, 0x06, 0x00}},
		{"dpi 800", mustR(SetDPI(800)), Report{0x09, 0x03, 0x06, 0x01}},
		{"dpi 1600", mustR(SetDPI(1600)), Report{0x09, 0x03, 0x06, 0x02}},
		{"dpi 2400", mustR(SetDPI(2400)), Report{0x09, 0x03, 0x06, 0x03}},
		{"dpi 3200", mustR(SetDPI(3200)), Report{0x09, 0x03, 0x06, 0x04}},
		{"dpi 6400", mustR(SetDPI(6400)), Report{0x09, 0x03, 0x06, 0x05}},
		{"lod 1mm", mustR(SetLOD(LOD1mm)), Report{0x09, 0x04, 0x01, 0x00}},
		{"lod 2mm", mustR(SetLOD(LOD2mm)), Report{0x09, 0x04, 0x01, 0x01}},
		{"lod 0.7mm", mustR(SetLOD(LOD07mm)), Report{0x09, 0x04, 0x01, 0x02}},
		{"ripple on", SetRippleControl(true), Report{0x09, 0x04, 0x02, 0x01}},
		{"ripple off", SetRippleControl(false), Report{0x09, 0x04, 0x02, 0x00}},
		{"angle on", SetAngleSnapping(true), Report{0x09, 0x04, 0x03, 0x01}},
		{"angle off", SetAngleSnapping(false), Report{0x09, 0x04, 0x03, 0x00}},
		{"motion on", SetMotionSync(true), Report{0x09, 0x04, 0x04, 0x01}},
		{"motion off", SetMotionSync(false), Report{0x09, 0x04, 0x04, 0x00}},
		{"debounce 0", mustR(SetDebounce(0)), Report{0x09, 0x05, 0x01, 0x00}},
		{"debounce 30", mustR(SetDebounce(30)), Report{0x09, 0x05, 0x01, 30}},
		{"sleep 10s", mustR(SetSleep(10)), Report{0x09, 0x05, 0x03, 0x0A, 0x00}},
		{"sleep 300s", mustR(SetSleep(300)), Report{0x09, 0x05, 0x03, 0x2C, 0x01}},
		{"sleep 900s", mustR(SetSleep(900)), Report{0x09, 0x05, 0x03, 0x84, 0x03}},
		{"led connect", mustR(SetReceiverLED(LEDConnectAndRate)), Report{0x09, 0x08, 0x00}},
		{"led battery", mustR(SetReceiverLED(LEDBatteryStatus)), Report{0x09, 0x08, 0x01}},
		{"led warning", mustR(SetReceiverLED(LEDBatteryWarning)), Report{0x09, 0x08, 0x02}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.got != tt.want {
				t.Errorf("got % X, want % X", tt.got, tt.want)
			}
		})
	}
}

func TestAllPresetsEncode(t *testing.T) {
	for _, v := range DPIValues {
		if _, err := SetDPI(v); err != nil {
			t.Errorf("dpi %d: %v", v, err)
		}
	}
	for _, v := range PollingRatesAscending {
		if _, err := SetPollingRate(v); err != nil {
			t.Errorf("hz %d: %v", v, err)
		}
	}
	for _, v := range LODValues {
		if _, err := SetLOD(v); err != nil {
			t.Errorf("lod %d: %v", v, err)
		}
	}
	for _, v := range SleepValues {
		if _, err := SetSleep(v); err != nil {
			t.Errorf("sleep %d: %v", v, err)
		}
	}
	if len(PollingRatesAscending) != len(PollingRates) {
		t.Error("polling rate lists differ in length")
	}
}

func TestSetInvalid(t *testing.T) {
	errs := []error{
		func() error { _, e := SetPollingRate(999); return e }(),
		func() error { _, e := SetDPI(1000); return e }(),
		func() error { _, e := SetLOD(5); return e }(),
		func() error { _, e := SetDebounce(-1); return e }(),
		func() error { _, e := SetDebounce(31); return e }(),
		func() error { _, e := SetSleep(0); return e }(),
		func() error { _, e := SetSleep(901); return e }(),
		func() error { _, e := SetReceiverLED(3); return e }(),
		func() error { _, e := SetReceiverLED(-1); return e }(),
	}
	for i, err := range errs {
		if !errors.Is(err, ErrInvalidValue) {
			t.Errorf("case %d: err = %v, want ErrInvalidValue", i, err)
		}
	}
}

func TestReadRequestsGolden(t *testing.T) {
	tests := []struct {
		name string
		req  Request
		want Report
	}{
		{"status", ReadStatus, Report{0x09, 0x89}},
		{"lod+motion", ReadLODMotionSync, Report{0x09, 0x84, 0x00}},
		{"ripple", ReadRippleControl, Report{0x09, 0x84, 0x02}},
		{"angle", ReadAngleSnapping, Report{0x09, 0x84, 0x03}},
		{"debounce", ReadDebounce, Report{0x09, 0x85, 0x01}},
		{"sleep", ReadSleep, Report{0x09, 0x85, 0x03}},
		{"led", ReadReceiverLED, Report{0x09, 0x88, 0x00}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.req.Report != tt.want {
				t.Errorf("got % X, want % X", tt.req.Report, tt.want)
			}
		})
	}
}

func TestRequestMatches(t *testing.T) {
	tests := []struct {
		name string
		req  Request
		buf  []byte
		want bool
	}{
		{"echo cmd+sub", ReadDebounce, []byte{9, 0x85, 0x01, 4, 0, 0, 0, 0, 0}, true},
		{"wrong sub", ReadDebounce, []byte{9, 0x85, 0x03, 4, 0, 0, 0, 0, 0}, false},
		{"wrong cmd", ReadDebounce, []byte{9, 0x84, 0x01, 4, 0, 0, 0, 0, 0}, false},
		{"wrong report id", ReadDebounce, []byte{1, 0x85, 0x01, 4, 0, 0, 0, 0, 0}, false},
		{"short", ReadDebounce, []byte{9, 0x85, 0x01}, false},
		{"no-sub request ignores byte 2", ReadLODMotionSync, []byte{9, 0x84, 0x7F, 0, 0, 0, 0, 0x21, 0}, true},
		{"status echo", ReadStatus, []byte{9, 0x89, 0x57, 1, 3, 8, 0, 1, 0x5A}, true},
		{"status interrupt is not an echo", ReadStatus, []byte{9, 0x57, 0x13, 8, 0, 0x0F, 0x0F, 0x11, 0}, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.req.Matches(tt.buf); got != tt.want {
				t.Errorf("Matches = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestReportLayout(t *testing.T) {
	r, _ := SetDPI(800)
	if len(r) != ReportLen || r[0] != ReportID {
		t.Fatalf("bad layout % X", r)
	}
}
