package tui

import (
	"strings"

	"github.com/charmbracelet/x/ansi"
)

// helpLines builds the scrollable help page for a panel `width` columns wide (content width):
// keys and gestures first, then an explanation of every setting.
func helpLines(width int) []string {
	row := func(keys, what string) string {
		return "  " + styleKey.Render(padTo(keys, 22)) + styleValue.Render(what)
	}
	head := func(s string) string { return styleDim.Bold(true).Render(strings.ToUpper(s)) }
	lines := []string{
		head("Move"),
		row("↑ ↓   j k", "select a setting"),
		head("Change"),
		row("← →   h l", "prev / next value; off / on"),
		row("⇧← ⇧→  PgDn PgUp", "debounce slider ±5"),
		row("space  enter", "toggle on/off"),
		row("enter", "open the pick list"),
		head("Mouse"),
		row("click row", "select it"),
		row("click ◀ ▶ value bar", "step · pick list · set level"),
		row("wheel", "change selected value"),
		head("General"),
		row("↓  PgDn", "scroll: what each setting does"),
		row("r", "re-read from the mouse"),
		row("?", "this help (↑↓ PgUp PgDn, esc)"),
		row("q  ctrl+c", "quit"),
		"",
		styleTitle.Render("WHAT EACH SETTING DOES"),
	}
	w := max(width, 20)
	wrap := func(indent, label, text string) {
		first := indent + styleLabel.Render(label) + " "
		pad := strings.Repeat(" ", ansi.StringWidth(indent+label)+1)
		for i, l := range strings.Split(ansi.Wordwrap(text, w-ansi.StringWidth(pad), ""), "\n") {
			if i == 0 {
				lines = append(lines, first+styleValue.Render(l))
			} else {
				lines = append(lines, pad+styleValue.Render(l))
			}
		}
	}
	for _, d := range settingDocs {
		if d.Group != "" {
			lines = append(lines, "", head(d.Group))
		}
		title := styleTitle.Render(d.Name)
		if d.Values != "" {
			title += styleDim.Render("  " + d.Values)
		}
		lines = append(lines, ansi.Truncate(title, w, "…"))
		wrap("  ", "Purpose:", d.Purpose)
		wrap("  ", "Effect: ", d.Effect)
		wrap("  ", "Tip:    ", d.Tip)
	}
	return lines
}
