package tui

import (
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"
)

// box draws a rounded panel exactly `width` columns wide with a title on the top border
// (left) and an optional right-aligned tag. Body lines are truncated or padded to fit.
func box(title, right string, body []string, width int, border lipgloss.Style) string {
	if width < 10 {
		width = 10
	}
	inner := width - 4 // "│ " + content + " │"

	left := " " + title + " "
	tag := ""
	if right != "" {
		tag = " " + right + " "
	}
	fill := width - 4 - lipgloss.Width(left) - lipgloss.Width(tag) // ╭─ … ─╮ corners + one dash each side
	if fill < 0 {
		left = ansi.Truncate(left, max(0, lipgloss.Width(left)+fill), "…")
		fill = 0
	}
	var b strings.Builder
	b.WriteString(border.Render("╭─") + left + border.Render(strings.Repeat("─", fill)) + tag + border.Render("─╮"))
	for _, line := range body {
		line = ansi.Truncate(line, inner, "…")
		pad := inner - lipgloss.Width(line)
		b.WriteString("\n" + border.Render("│") + " " + line + strings.Repeat(" ", pad) + " " + border.Render("│"))
	}
	b.WriteString("\n" + border.Render("╰"+strings.Repeat("─", width-2)+"╯"))
	return b.String()
}
