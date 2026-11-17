package tui

import (
	"flag"
	"os"
	"path/filepath"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/muesli/termenv"

	"github.com/mastergap/incott-mouse-tui/internal/protocol"
	"github.com/mastergap/incott-mouse-tui/internal/state"
)

var update = flag.Bool("update", false, "rewrite golden files")

func golden(t *testing.T, name, got string) {
	t.Helper()
	got = strings.TrimRight(got, "\n") + "\n"
	path := filepath.Join("testdata", name+".golden")
	if *update {
		if err := os.WriteFile(path, []byte(got), 0o644); err != nil {
			t.Fatal(err)
		}
		return
	}
	want, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("%v (run: go test ./internal/tui -update)", err)
	}
	if strings.ReplaceAll(string(want), "\r\n", "\n") != got {
		t.Errorf("%s differs from golden.\n--- got ---\n%s--- want ---\n%s", name, got, want)
	}
}

func shot(t *testing.T, w, h int, st state.State, keys ...string) string {
	t.Helper()
	m := New(newStub(st), &DemoHooks{TogglePlug: func() {}, TogglePermission: func() {}})
	m.demo = nil
	m.goos = "linux"
	m, _ = send(m, tea.WindowSizeMsg{Width: w, Height: h})
	m, _ = send(m, stateMsg(st))
	m.toast = toast{} // announcement toasts are covered elsewhere
	for _, k := range keys {
		m, _ = press(m, k)
	}
	return m.View()
}

func TestGoldenScreens(t *testing.T) {
	conn := connectedState()
	charging := connectedState()
	charging.Charging, charging.ProductID, charging.Battery = true, protocol.ProductIDCharging, state.Some(9)
	noLED := connectedState()
	noLED.ReceiverLEDUnsupported, noLED.ReceiverLED = true, state.Opt[protocol.LEDMode]{}

	tests := []struct {
		name string
		w, h int
		st   state.State
		keys []string
	}{
		{"connected_80x24", 80, 24, conn, nil},
		{"connected_120x40", 120, 40, conn, nil},
		{"connected_60x20", 60, 20, conn, nil},
		{"connected_compact_100x14", 100, 14, conn, []string{"down", "down", "down"}},
		{"charging_low_battery", 80, 24, charging, nil},
		{"no_receiver_led", 80, 24, noLED, nil},
		{"picker_sleep", 80, 24, conn, []string{"j", "j", "j", "j", "j", "j", "j", "enter"}},
		{"help", 80, 24, conn, []string{"?"}},
		{"disconnected", 80, 24, state.State{}, nil},
		{"connecting", 80, 24, state.State{Connection: state.Connecting, Name: "G23 V2"}, nil},
		{"permission_linux", 80, 24, state.State{Connection: state.PermissionDenied, Err: "cannot open mouse"}, nil},
		{"too_small", 40, 10, conn, nil},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) { golden(t, tt.name, shot(t, tt.w, tt.h, tt.st, tt.keys...)) })
	}
}

// Resizing at any time must never produce lines wider or taller than the terminal.
func TestResizeSweep(t *testing.T) {
	m := New(newStub(connectedState()), nil)
	m, _ = send(m, stateMsg(connectedState()))
	for h := 1; h <= 45; h += 4 {
		for w := 1; w <= 130; w += 7 {
			m, _ = send(m, tea.WindowSizeMsg{Width: w, Height: h})
			out := m.View()
			assertFits(t, out, max(w, 60)) // the too-small notice may exceed tiny widths but is never taller
			if lines := len(strings.Split(out, "\n")); w >= minWidth && h >= minHeight && lines > h {
				t.Fatalf("%dx%d: %d lines", w, h, lines)
			}
		}
	}
}

func TestNoColorProducesNoEscapes(t *testing.T) {
	defer lipgloss.SetColorProfile(termenv.Ascii)
	lipgloss.SetColorProfile(termenv.TrueColor)
	if !strings.Contains(statusPanel(connectedState(), 80, false), "\x1b[") {
		t.Fatal("precondition: truecolor output must contain escapes")
	}
	lipgloss.SetColorProfile(termenv.Ascii) // what termenv selects when NO_COLOR is set
	if strings.Contains(statusPanel(connectedState(), 80, false), "\x1b[") {
		t.Error("ascii profile must not emit escape codes")
	}
}

// Meaning must never rely on colour alone: every state carries an icon or text.
func TestStatesAreDistinguishableWithoutColor(t *testing.T) {
	seen := map[string]state.Connection{}
	for _, c := range []state.Connection{state.Disconnected, state.Connecting, state.Connected, state.PermissionDenied} {
		b := connectionBadge(state.State{Connection: c})
		if prev, dup := seen[b]; dup {
			t.Errorf("%v and %v render the same badge %q", prev, c, b)
		}
		seen[b] = c
	}
}

func TestSetTheme(t *testing.T) {
	for _, ok := range []string{"", "auto", "dark", "LIGHT"} {
		if err := SetTheme(ok); err != nil {
			t.Errorf("%q: %v", ok, err)
		}
	}
	if err := SetTheme("neon"); err == nil {
		t.Error("invalid theme must error")
	}
}
