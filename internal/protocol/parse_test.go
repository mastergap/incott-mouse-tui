package protocol

import (
	"errors"
	"testing"
)

func rep(b ...byte) []byte {
	out := make([]byte, ReportLen)
	copy(out, b)
	return out
}

func TestParseInterruptStatus(t *testing.T) {
	tests := []struct {
		name    string
		buf     []byte
		want    Status
		wantErr bool
	}{
		{"normal", rep(9, 80, 0x10), Status{80, false, 800, 1000}, false},
		{"charging bit 137 -> 9%", rep(9, 137, 0x10), Status{9, true, 800, 1000}, false},
		{"8000Hz + 6400dpi", rep(9, 50, 0x54), Status{50, false, 6400, 8000}, false},
		{"100% not charging", rep(9, 100, 0x00), Status{100, false, 400, 1000}, false},
		{"228 = 100% charging", rep(9, 228, 0x26), Status{100, true, 1600, 2000}, false},
		{"battery out of range", rep(9, 229, 0x10), Status{}, true},
		{"dpi preset out of range", rep(9, 60, 0x60), Status{}, true},
		{"hz preset out of range", rep(9, 60, 0x17), Status{}, true},
		{"too short", []byte{9}, Status{}, true},
		{"wrong report id", rep(1, 60, 0x10), Status{}, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ParseInterruptStatus(tt.buf)
			if (err != nil) != tt.wantErr {
				t.Fatalf("err = %v, wantErr %v", err, tt.wantErr)
			}
			if got != tt.want {
				t.Errorf("got %+v, want %+v", got, tt.want)
			}
		})
	}
}

func TestParseLODMotionSync(t *testing.T) {
	tests := []struct {
		b       byte
		lod     int
		motion  bool
		wantErr bool
	}{
		{0x00, LOD1mm, false, false},
		{0x01, LOD1mm, true, false},
		{0x10, LOD2mm, false, false},
		{0x11, LOD2mm, true, false},
		{0x20, LOD07mm, false, false},
		{0x21, LOD07mm, true, false},
		{0x22, LOD07mm, false, false},
		{0xF0, 0, false, true},
	}
	for _, tt := range tests {
		lod, ms, err := ParseLODMotionSync(rep(9, 0x84, 0, 0, 0, 0, 0, tt.b))
		if (err != nil) != tt.wantErr || lod != tt.lod || ms != tt.motion {
			t.Errorf("0x%02X: got (%d,%v,%v), want (%d,%v,wantErr=%v)", tt.b, lod, ms, err, tt.lod, tt.motion, tt.wantErr)
		}
	}
	if _, _, err := ParseLODMotionSync([]byte{9, 0x84}); !errors.Is(err, ErrShortReport) {
		t.Errorf("short: %v", err)
	}
}

func TestParseToggles(t *testing.T) {
	for _, parse := range []func([]byte) (bool, error){ParseRippleControl, ParseAngleSnapping} {
		if v, err := parse(rep(9, 0x84, 2, 1)); err != nil || !v {
			t.Errorf("on: %v %v", v, err)
		}
		if v, err := parse(rep(9, 0x84, 2, 0)); err != nil || v {
			t.Errorf("off: %v %v", v, err)
		}
		if v, err := parse(rep(9, 0x84, 2, 7)); err != nil || v {
			t.Errorf("non-1 is off: %v %v", v, err)
		}
		if _, err := parse([]byte{9}); err == nil {
			t.Error("short must fail")
		}
	}
}

func TestParseDebounce(t *testing.T) {
	for _, v := range []int{0, 1, 8, 30} {
		got, err := ParseDebounce(rep(9, 0x85, 1, byte(v)))
		if err != nil || got != v {
			t.Errorf("%d: got %d, %v", v, got, err)
		}
	}
	for _, v := range []byte{31, 0xFF} {
		if _, err := ParseDebounce(rep(9, 0x85, 1, v)); !errors.Is(err, ErrUnexpected) {
			t.Errorf("%d: err = %v", v, err)
		}
	}
}

func TestParseSleep(t *testing.T) {
	tests := []struct {
		lo, hi  byte
		want    int
		wantErr bool
	}{
		{0x0A, 0x00, 10, false},
		{0x3C, 0x00, 60, false},
		{0x2C, 0x01, 300, false},
		{0x84, 0x03, 900, false},
		{0x00, 0x00, 0, true},
		{0x85, 0x03, 0, true},
		{0xFF, 0xFF, 0, true},
	}
	for _, tt := range tests {
		got, err := ParseSleep(rep(9, 0x85, 3, tt.lo, tt.hi))
		if (err != nil) != tt.wantErr || got != tt.want {
			t.Errorf("(%02X,%02X): got %d, %v", tt.lo, tt.hi, got, err)
		}
	}
}

func TestParseReceiverLED(t *testing.T) {
	for _, m := range LEDModes {
		got, err := ParseReceiverLED(rep(9, 0x88, byte(m)))
		if err != nil || got != m {
			t.Errorf("%v: got %v, %v", m, got, err)
		}
	}
	if _, err := ParseReceiverLED(rep(9, 0x88, 3)); err == nil {
		t.Error("mode 3 must fail")
	}
}

// Round trip: what we write is what the matching read decodes (the device echoes the value
// at the documented offsets).
func TestSetThenParseRoundTrip(t *testing.T) {
	for _, s := range SleepValues {
		set, _ := SetSleep(s)
		resp := rep(9, 0x85, 3, set[3], set[4])
		if got, err := ParseSleep(resp); err != nil || got != s {
			t.Errorf("sleep %d: got %d, %v", s, got, err)
		}
	}
	for ms := 0; ms <= MaxDebounceMS; ms++ {
		set, _ := SetDebounce(ms)
		if got, err := ParseDebounce(rep(9, 0x85, 1, set[3])); err != nil || got != ms {
			t.Errorf("debounce %d: got %d, %v", ms, got, err)
		}
	}
	for _, m := range LEDModes {
		set, _ := SetReceiverLED(m)
		if got, err := ParseReceiverLED(rep(9, 0x88, set[2])); err != nil || got != m {
			t.Errorf("led %v: got %v, %v", m, got, err)
		}
	}
}

func TestIsInterruptStatus(t *testing.T) {
	if !IsInterruptStatus(rep(9, 80, 0x10)) {
		t.Error("normal status")
	}
	if IsInterruptStatus(rep(9, 0, 0x10)) {
		t.Error("zero battery byte")
	}
	if IsInterruptStatus([]byte{9, 80}) {
		t.Error("short")
	}
	if IsInterruptStatus(rep(2, 80, 0x10)) {
		t.Error("wrong id")
	}
}

func TestParseStatusResponse(t *testing.T) {
	tests := []struct {
		name string
		buf  []byte
		want Status
		err  error
	}{
		{"hardware capture", rep(9, 0x89, 0x57, 0x01, 0x03, 0x08, 0x00, 0x01, 0x5A), Status{87, false, 800, 125}, nil},
		{"charging", rep(9, 0x89, 0xE4, 0x05, 0x04), Status{100, true, 6400, 8000}, nil},
		{"placeholder", rep(9, 0x89, 0, 0), Status{}, ErrNotReady},
		{"not a status echo", rep(9, 0x84, 0x57, 0x01, 0x03), Status{}, ErrUnexpected},
		{"bad index", rep(9, 0x89, 0x57, 0x09, 0x03), Status{}, ErrUnexpected},
		{"short", []byte{9, 0x89}, Status{}, ErrShortReport},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ParseStatusResponse(tt.buf)
			if !errors.Is(err, tt.err) || got != tt.want {
				t.Errorf("got %+v, %v; want %+v, %v", got, err, tt.want, tt.err)
			}
		})
	}
}

// Real captures from a "Wireless Gaming Mouse" (VID 093A PID 522C, usage page FF05, iface 2)
// on macOS arm64. The status interrupt and the 0x89 response must decode identically.
func TestHardwareCaptures(t *testing.T) {
	interrupt := rep(9, 0x57, 0x13, 0x08, 0x00, 0x0F, 0x0F, 0x11)
	response := rep(9, 0x89, 0x57, 0x01, 0x03, 0x08, 0x00, 0x01, 0x5A)
	a, err1 := ParseInterruptStatus(interrupt)
	b, err2 := ParseStatusResponse(response)
	want := Status{Battery: 87, DPI: 800, Polling: 125}
	if err1 != nil || err2 != nil || a != want || b != want {
		t.Fatalf("interrupt=%+v,%v response=%+v,%v want %+v", a, err1, b, err2, want)
	}
	if !ReadStatus.Matches(response) {
		t.Error("ReadStatus must match the 0x89 response")
	}
	if lod, ms, err := ParseLODMotionSync(rep(9, 0x84, 0x00, 0x08, 0x00, 0x0F, 0x0F, 0x11)); err != nil || lod != LOD2mm || !ms {
		t.Errorf("lod capture: %d %v %v", lod, ms, err)
	}
	if v, err := ParseDebounce(rep(9, 0x85, 0x01, 0x08, 0x00, 0x0F, 0x0F, 0x11)); err != nil || v != 8 {
		t.Errorf("debounce capture: %d %v", v, err)
	}
	if v, err := ParseSleep(rep(9, 0x85, 0x03, 0x3C, 0x00, 0x0F, 0x0F, 0x11)); err != nil || v != 60 {
		t.Errorf("sleep capture: %d %v", v, err)
	}
	if v, err := ParseRippleControl(rep(9, 0x84, 0x02, 0x00)); err != nil || v {
		t.Errorf("ripple capture: %v %v", v, err)
	}
}
