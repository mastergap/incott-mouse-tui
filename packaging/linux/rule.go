// Package linux embeds the udev rule that lets a normal user open the Incott mouse on Linux.
// It is the single source of the rule: `incott --print-udev-rule` prints it, and tests keep the
// copies in README.md and docs/BUILDING.md identical.
package linux

import (
	_ "embed"
	"strings"
)

// RulesFileName is the file name to install under /etc/udev/rules.d/.
const RulesFileName = "99-incott.rules"

//go:embed 99-incott.rules
var embedded string

// UdevRule is the content of 99-incott.rules with LF line endings (udev files must not contain
// CR; a Windows checkout with autocrlf would otherwise embed CRLF).
var UdevRule = strings.ReplaceAll(embedded, "\r\n", "\n")
