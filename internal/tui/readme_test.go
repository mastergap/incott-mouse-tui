package tui

import (
	"os"
	"strings"
	"testing"

	"github.com/charmbracelet/x/ansi"
)

func ansiWidth(s string) int { return ansi.StringWidth(s) }

const (
	readmePath  = "../../README.md"
	markerStart = "<!-- settings:start -->"
	markerEnd   = "<!-- settings:end -->"
)

// The README section is generated from settingDocs (the same data as the in-app help).
// Run `go test ./internal/tui -update` after editing docs.go.
func TestReadmeSettingsInSync(t *testing.T) {
	b, err := os.ReadFile(readmePath)
	if err != nil {
		t.Fatal(err)
	}
	readme := string(b)
	i, j := strings.Index(readme, markerStart), strings.Index(readme, markerEnd)
	if i < 0 || j < i {
		t.Fatalf("README.md must contain %s … %s", markerStart, markerEnd)
	}
	want := markerStart + "\n" + settingsMarkdown() + "\n" + markerEnd
	if got := readme[i : j+len(markerEnd)]; got != want {
		if *update {
			out := readme[:i] + want + readme[j+len(markerEnd):]
			if err := os.WriteFile(readmePath, []byte(out), 0o644); err != nil {
				t.Fatal(err)
			}
			return
		}
		t.Error("README settings section is out of date: run go test ./internal/tui -update")
	}
}
