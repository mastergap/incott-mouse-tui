package device

import (
	"reflect"
	"testing"

	"github.com/mastergap/incott-mouse-tui/internal/protocol"
)

func TestSelectCandidates(t *testing.T) {
	const pid = protocol.ProductIDWireless
	mac := []Candidate{ // captured on macOS
		{"kbd", pid, "Wireless Gaming Mouse", 0x0001, 0x0006, 0},
		{"mouse", pid, "Wireless Gaming Mouse", 0x0001, 0x0002, 1},
		{"v0", pid, "Wireless Gaming Mouse", 0xFF00, 0x0001, 1},
		{"cfg", pid, "Wireless Gaming Mouse", 0xFF05, 0x0001, 2},
	}
	got := SelectCandidates(mac)
	if len(got) != 2 || got[0].Path != "cfg" || got[1].Path != "v0" {
		t.Fatalf("mac: %+v", got)
	}

	linux := []Candidate{ // libusb backend: no usage pages
		{"i0", pid, "Wireless Gaming Mouse", 0, 0, 0},
		{"i1", pid, "Wireless Gaming Mouse", 0, 0, 1},
		{"i2", pid, "Wireless Gaming Mouse", 0, 0, 2},
	}
	got = SelectCandidates(linux)
	if len(got) != 1 || got[0].Path != "i2" {
		t.Fatalf("linux: %+v", got)
	}
}

func TestSelectCandidatesFilters(t *testing.T) {
	cands := []Candidate{
		{"wrongpid", 0x1234, "Mouse", 0xFF05, 1, 2},
		{"keyboard", protocol.ProductIDWireless, "Incott Keyboard", 0xFF05, 1, 2},
		{"charging", protocol.ProductIDCharging, "G23 Mouse", 0xFF05, 1, 2},
		{"noname", protocol.ProductIDWireless, "", 0xFF05, 1, 2},
	}
	got := SelectCandidates(cands)
	var paths []string
	for _, c := range got {
		paths = append(paths, c.Path)
	}
	if !reflect.DeepEqual(paths, []string{"charging", "noname"}) {
		t.Fatalf("paths = %v", paths)
	}
	if len(SelectCandidates(nil)) != 0 {
		t.Error("nil input")
	}
}
