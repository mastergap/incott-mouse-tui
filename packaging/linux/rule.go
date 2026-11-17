// Package linux embeds the udev rule that lets a normal user open the Incott mouse on Linux.
// It is the single source of the rule: `incott --print-udev-rule` prints it, and tests keep the
// copies in README.md and docs/BUILDING.md identical.
package linux

import _ "embed"

// RulesFileName is the file name to install under /etc/udev/rules.d/.
const RulesFileName = "99-incott.rules"

// UdevRule is the content of 99-incott.rules.
//
//go:embed 99-incott.rules
var UdevRule string
