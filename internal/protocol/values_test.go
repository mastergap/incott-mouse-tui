package protocol

import "testing"

func TestIsMouseProduct(t *testing.T) {
	tests := []struct {
		product string
		want    bool
	}{
		{"", true},
		{"incott 8K wireless mouse", true},
		{"Incott G23 Wireless", true},
		{"G24 Pro", true},
		{"Ghero Series Mouse", true},
		{"Zero 29 Wireless", true},
		{"Zero 39 Pro", true},
		{"Incott Keyboard", false},
		{"Some Other Device", false},
		{"MOUSE", true},
		{"g23v2", true},
	}
	for _, tt := range tests {
		if got := IsMouseProduct(tt.product); got != tt.want {
			t.Errorf("IsMouseProduct(%q) = %v, want %v", tt.product, got, tt.want)
		}
	}
}

func TestIsKnownProductID(t *testing.T) {
	if !IsKnownProductID(0x522C) || !IsKnownProductID(0x622C) || IsKnownProductID(0x1234) {
		t.Error("product id check")
	}
}

func TestLabels(t *testing.T) {
	tests := []struct{ got, want string }{
		{DPILabel(1600), "1600"},
		{PollingLabel(8000), "8000 Hz"},
		{LODLabel(LOD07mm), "0.7 mm"},
		{LODLabel(LOD1mm), "1 mm"},
		{LODLabel(LOD2mm), "2 mm"},
		{LODLabel(0), "—"},
		{DebounceLabel(4), "4 ms"},
		{SleepLabel(10), "10 sec"},
		{SleepLabel(30), "30 sec"},
		{SleepLabel(60), "1 min"},
		{SleepLabel(900), "15 min"},
		{SleepLabel(90), "90 sec"},
		{LEDConnectAndRate.String(), "Connect & polling rate"},
		{LEDBatteryStatus.String(), "Battery status"},
		{LEDBatteryWarning.String(), "Battery warning"},
		{LEDMode(9).String(), "—"},
		{OnOff(true), "On"},
		{OnOff(false), "Off"},
	}
	for _, tt := range tests {
		if tt.got != tt.want {
			t.Errorf("got %q, want %q", tt.got, tt.want)
		}
	}
}

func TestSleepValuesAscendingAndInRange(t *testing.T) {
	prev := 0
	for _, v := range SleepValues {
		if v <= prev || v > MaxSleepSeconds {
			t.Errorf("bad sleep value %d after %d", v, prev)
		}
		prev = v
	}
}
