package linux

import (
	"fmt"
	"os"
	"strings"
	"testing"

	"github.com/mastergap/incott-mouse-tui/internal/protocol"
)

func TestRuleMatchesProtocolIDs(t *testing.T) {
	want := []string{
		fmt.Sprintf(`ATTRS{idVendor}=="%04x"`, protocol.VendorID),
		fmt.Sprintf(`ATTRS{idProduct}=="%04x|%04x"`, protocol.ProductIDWireless, protocol.ProductIDCharging),
		`TAG+="uaccess"`,
	}
	for _, w := range want {
		if !strings.Contains(UdevRule, w) {
			t.Errorf("rule does not contain %s:\n%s", w, UdevRule)
		}
	}
}

// Both docs show the rule inside a `sudo tee ... <<'EOF'` block; it must equal the real file.
func TestDocsContainExactRule(t *testing.T) {
	for _, path := range []string{"../../README.md", "../../docs/BUILDING.md"} {
		b, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		doc := strings.ReplaceAll(string(b), "\r\n", "\n")
		block := "<<'EOF'\n" + strings.TrimRight(UdevRule, "\n") + "\nEOF\n"
		if !strings.Contains(doc, block) {
			t.Errorf("%s must contain this exact block (copy of 99-incott.rules):\n%s", path, block)
		}
	}
}
