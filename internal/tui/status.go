package tui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"

	"github.com/mastergap/incott-mouse-tui/internal/protocol"
	"github.com/mastergap/incott-mouse-tui/internal/state"
)

const unknown = "—"

// statusPanel renders the always-visible status panel: connection, battery and every setting
// as currently reported by the mouse. compact collapses it to one bordered line.
func statusPanel(st state.State, width int, compact bool) string {
	title := styleTitle.Render("INCOTT")
	if st.Name != "" {
		title += styleDim.Render(" · ") + styleValue.Render(st.Name)
	}
	badge := connectionBadge(st)

	if compact {
		return box(title, badge, []string{compactLine(st)}, width, styleBorder)
	}

	inner := width - 4
	c1 := inner * 40 / 100
	c2 := inner * 30 / 100
	c3 := inner - c1 - c2

	row := func(a, b, c string) string {
		return padTo(a, c1) + padTo(b, c2) + padTo(c, c3)
	}
	body := []string{
		row(kv("Battery", batteryCell(st), 9), kv("DPI", val(st.DPI, protocol.DPILabel), 10), kv("Motion sync", onOff(st.MotionSync), 12)),
		row(kv("Polling", val(st.Polling, protocol.PollingLabel), 9), kv("LOD", val(st.LOD, protocol.LODLabel), 10), kv("Angle snap", onOff(st.AngleSnap), 12)),
		row(kv("Sleep", val(st.Sleep, protocol.SleepLabel), 9), kv("Debounce", val(st.Debounce, protocol.DebounceLabel), 10), kv("Ripple", onOff(st.Ripple), 12)),
		kv("Rcv LED", ledCell(st), 9),
	}
	return box(title, badge, body, width, styleBorder)
}

func compactLine(st state.State) string {
	parts := []string{
		batteryText(st),
		val(st.DPI, func(v int) string { return protocol.DPILabel(v) + " DPI" }),
		val(st.Polling, protocol.PollingLabel),
		val(st.LOD, protocol.LODLabel),
		val(st.Debounce, protocol.DebounceLabel),
		val(st.Sleep, protocol.SleepLabel),
	}
	return strings.Join(parts, styleDim.Render(" · "))
}

func connectionBadge(st state.State) string {
	switch st.Connection {
	case state.Connected:
		kind := "wireless"
		if st.Wired() {
			kind = "wired"
		}
		return styleOK.Render("● Connected") + styleDim.Render(" ("+kind+")")
	case state.Connecting:
		return styleWarn.Render("◐ Connecting…")
	case state.PermissionDenied:
		return styleBad.Render("⚠ Permission denied")
	}
	return styleDim.Render("○ Not connected")
}

func kv(label, value string, labelW int) string {
	return padTo(styleLabel.Render(label), labelW+1) + value
}

func padTo(s string, w int) string {
	s = ansi.Truncate(s, w, "…")
	if pad := w - lipgloss.Width(s); pad > 0 {
		s += strings.Repeat(" ", pad)
	}
	return s
}

// val formats a known value, or an em dash for unknown.
func val[T any](o state.Opt[T], f func(T) string) string {
	if !o.OK {
		return styleDim.Render(unknown)
	}
	return styleValue.Render(f(o.Val))
}

func onOff(o state.Opt[bool]) string {
	if !o.OK {
		return styleDim.Render(unknown)
	}
	if o.Val {
		return styleOK.Render("On")
	}
	return styleDim.Render("Off")
}

func ledCell(st state.State) string {
	if st.ReceiverLEDUnsupported {
		return styleDim.Render("not supported")
	}
	return val(st.ReceiverLED, func(m protocol.LEDMode) string { return m.String() })
}

// batteryStyle picks the colour by level: green > 40, amber 15–40, red < 15.
func batteryStyle(pct int) lipgloss.Style {
	switch {
	case pct > 40:
		return styleOK
	case pct >= 15:
		return styleWarn
	}
	return styleBad
}

func batteryText(st state.State) string {
	if !st.Battery.OK {
		return styleDim.Render(unknown)
	}
	s := batteryStyle(st.Battery.Val).Render(fmt.Sprintf("%d%%", st.Battery.Val))
	if st.Charging {
		s += " " + styleWarn.Render("⚡")
	}
	return s
}

func batteryCell(st state.State) string {
	if !st.Battery.OK {
		return styleDim.Render(strings.Repeat("░", barWidth) + " " + unknown)
	}
	pct := st.Battery.Val
	filled := (pct*barWidth + 50) / 100
	if pct > 0 && filled == 0 {
		filled = 1
	}
	bar := batteryStyle(pct).Render(strings.Repeat("█", filled)) + styleDim.Render(strings.Repeat("░", barWidth-filled))
	return bar + " " + batteryText(st)
}

const barWidth = 10
