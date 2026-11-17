// Package tui implements the terminal UI (Bubble Tea).
package tui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
)

// Palette: one accent colour, semantic status colours, adaptive to light/dark terminals.
// Colours degrade automatically (truecolor → 256 → 16 → none; NO_COLOR is honoured by the
// colour profile detection).
var (
	colAccent = lipgloss.AdaptiveColor{Light: "#5B3FD9", Dark: "#9D8CFF"}
	colOK     = lipgloss.AdaptiveColor{Light: "#1A7F37", Dark: "#3FB950"}
	colWarn   = lipgloss.AdaptiveColor{Light: "#9A6700", Dark: "#D29922"}
	colBad    = lipgloss.AdaptiveColor{Light: "#CF222E", Dark: "#F85149"}
	colDim    = lipgloss.AdaptiveColor{Light: "#6E7781", Dark: "#7D8590"}
	colText   = lipgloss.AdaptiveColor{Light: "#1F2328", Dark: "#E6EDF3"}
	colBorder = lipgloss.AdaptiveColor{Light: "#AFB8C1", Dark: "#484F58"}
)

var (
	styleTitle  = lipgloss.NewStyle().Bold(true).Foreground(colAccent)
	styleLabel  = lipgloss.NewStyle().Foreground(colDim)
	styleValue  = lipgloss.NewStyle().Foreground(colText)
	styleDim    = lipgloss.NewStyle().Foreground(colDim)
	styleOK     = lipgloss.NewStyle().Foreground(colOK)
	styleWarn   = lipgloss.NewStyle().Foreground(colWarn)
	styleBad    = lipgloss.NewStyle().Foreground(colBad)
	styleBorder = lipgloss.NewStyle().Foreground(colBorder)
	styleKey    = lipgloss.NewStyle().Foreground(colAccent).Bold(true)
)

// SetTheme selects the colour scheme: "dark", "light" or "auto" (default, detect from the
// terminal). Use it when detection picks the wrong scheme. It does not avoid the background
// colour query itself: Bubble Tea v1 issues that in a package init(), before main runs.
// Real terminals answer at once; a terminal that never answers (some multiplexers, bare ptys)
// delays startup by about 5 seconds. NO_COLOR is honoured automatically.
func SetTheme(mode string) error {
	switch strings.ToLower(mode) {
	case "", "auto":
	case "dark":
		lipgloss.SetHasDarkBackground(true)
	case "light":
		lipgloss.SetHasDarkBackground(false)
	default:
		return fmt.Errorf("unknown theme %q (use auto, dark or light)", mode)
	}
	return nil
}
