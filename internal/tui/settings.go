package tui

import (
	"errors"
	"fmt"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/mastergap/incott-mouse-tui/internal/device"
	"github.com/mastergap/incott-mouse-tui/internal/protocol"
	"github.com/mastergap/incott-mouse-tui/internal/state"
)

// ctl is the kind of control used to edit a setting.
type ctl int

const (
	ctlCycle  ctl = iota // ◀ value ▶ over an ascending list of presets
	ctlSlider            // ◀ value ▶ plus a bar, over an integer range
	ctlToggle            // [x] On / [ ] Off
)

const (
	sliderBig   = 5
	sliderCells = 15
	labelWidth  = 18
)

// item describes one editable setting.
type item struct {
	group    string // header printed before the first item of a group
	label    string
	kind     device.Kind
	ctl      ctl
	options  []int // ascending presets (ctlCycle)
	bar      bool  // draw a level bar next to a ctlCycle control
	min, max int   // range (ctlSlider)
	format   func(int) string
}

func buildItems() []item {
	dpi := make([]int, len(protocol.DPIValues))
	copy(dpi, protocol.DPIValues[:])
	return []item{
		{group: "Performance", label: "DPI", kind: device.KindDPI, ctl: ctlCycle, bar: true, options: dpi, format: protocol.DPILabel},
		{label: "Polling rate", kind: device.KindPolling, ctl: ctlCycle, bar: true, options: protocol.PollingRatesAscending[:], format: protocol.PollingLabel},
		{label: "Lift-off distance", kind: device.KindLOD, ctl: ctlCycle, options: protocol.LODValues[:], format: protocol.LODLabel},
		{group: "Responsiveness", label: "Debounce", kind: device.KindDebounce, ctl: ctlSlider, min: 0, max: protocol.MaxDebounceMS, format: protocol.DebounceLabel},
		{label: "Motion sync", kind: device.KindMotionSync, ctl: ctlToggle, format: onOffLabel},
		{label: "Angle snapping", kind: device.KindAngleSnap, ctl: ctlToggle, format: onOffLabel},
		{label: "Ripple control", kind: device.KindRipple, ctl: ctlToggle, format: onOffLabel},
		{group: "Power & LED", label: "Sleep timer", kind: device.KindSleep, ctl: ctlCycle, options: protocol.SleepValues[:], format: protocol.SleepLabel},
		{label: "Receiver LED", kind: device.KindReceiverLED, ctl: ctlCycle, options: ledOptions(), format: func(v int) string { return protocol.LEDMode(v).String() }},
	}
}

func ledOptions() []int {
	out := make([]int, len(protocol.LEDModes))
	for i, m := range protocol.LEDModes {
		out[i] = int(m)
	}
	return out
}

func onOffLabel(v int) string { return protocol.OnOff(v != 0) }

type pendingVal struct{ value, seq int }

type picker struct{ cursor int }

// settings is the editor state: selection, values awaiting confirmation and the open picker.
type settings struct {
	items   []item
	cursor  int
	pending map[device.Kind]pendingVal // optimistic values not yet confirmed by the mouse
	seq     int
	picker  *picker
}

func newSettings() settings {
	return settings{items: buildItems(), pending: map[device.Kind]pendingVal{}}
}

// sync adapts the editable options to what the mouse supports.
func (s *settings) sync(st state.State) {
	for i := range s.items {
		if s.items[i].kind != device.KindLOD {
			continue
		}
		opts := make([]int, 0, len(protocol.LODValues))
		for _, v := range protocol.LODValues {
			if v == protocol.LOD07mm && st.LOD07Unsupported {
				continue
			}
			opts = append(opts, v)
		}
		s.items[i].options = opts
	}
}

// Messages.
type (
	applyDueMsg struct {
		kind device.Kind
		seq  int
	}
	applyResultMsg struct {
		kind  device.Kind
		value int
		err   error
	}
)

func (s *settings) selectable(i int, st state.State) bool {
	return s.items[i].kind != device.KindReceiverLED || !st.ReceiverLEDUnsupported
}

// moveCursor moves to the next selectable item in direction dir (±1), staying put at the ends.
func (s *settings) moveCursor(dir int, st state.State) {
	for i := s.cursor + dir; i >= 0 && i < len(s.items); i += dir {
		if s.selectable(i, st) {
			s.cursor = i
			return
		}
	}
}

// ensureCursor keeps the cursor on a selectable item (e.g. after the LED becomes unsupported).
func (s *settings) ensureCursor(st state.State) {
	if s.selectable(s.cursor, st) {
		return
	}
	s.moveCursor(-1, st)
}

func (s *settings) reset() {
	s.pending = map[device.Kind]pendingVal{}
	s.picker = nil
}

// current returns the value to display: the pending value if any, else the device value.
func (s *settings) current(it item, st state.State) (v int, known, pending bool) {
	if p, ok := s.pending[it.kind]; ok {
		return p.value, true, true
	}
	v, known = device.Value(st, it.kind)
	return v, known, false
}

// step computes the next value for a cycle/slider item. changed=false at the ends.
func step(it item, cur int, known bool, dir int, big bool) (int, bool) {
	switch it.ctl {
	case ctlSlider:
		n := 1
		if big {
			n = sliderBig
		}
		if !known {
			return it.min, true
		}
		v := min(max(cur+dir*n, it.min), it.max)
		return v, v != cur
	case ctlCycle:
		if !known {
			return it.options[0], true
		}
		if dir > 0 {
			for _, o := range it.options {
				if o > cur {
					return o, true
				}
			}
		} else {
			for i := len(it.options) - 1; i >= 0; i-- {
				if it.options[i] < cur {
					return it.options[i], true
				}
			}
		}
	}
	return cur, false
}

// propose records v as pending for the item and schedules the write: toggles and picker
// choices are written at once, repeated cycling/sliding is debounced so holding a key
// sends only the final value.
func (s *settings) propose(it item, v int, delay time.Duration) tea.Cmd {
	s.seq++
	p := pendingVal{value: v, seq: s.seq}
	s.pending[it.kind] = p
	due := applyDueMsg{kind: it.kind, seq: p.seq}
	if delay <= 0 {
		return func() tea.Msg { return due }
	}
	return tea.Tick(delay, func(time.Time) tea.Msg { return due })
}

// onKey handles a key for the editor. It returns whether the key was consumed.
func (s *settings) onKey(key string, st state.State, delay time.Duration) (tea.Cmd, bool) {
	if s.picker != nil {
		return s.pickerKey(key, delay), true
	}
	it := s.items[s.cursor]
	cur, known, _ := s.current(it, st)

	switch key {
	case "up", "k":
		s.moveCursor(-1, st)
		return nil, true
	case "down", "j":
		s.moveCursor(1, st)
		return nil, true
	case "left", "h", "right", "l", "shift+left", "shift+right", "pgup", "pgdown":
		dir := 1
		if key == "left" || key == "h" || key == "shift+left" || key == "pgdown" {
			dir = -1
		}
		if it.ctl == ctlToggle {
			return s.propose(it, b2i(dir > 0), 0), true // ← off, → on
		}
		big := key == "shift+left" || key == "shift+right" || key == "pgup" || key == "pgdown"
		if v, changed := step(it, cur, known, dir, big); changed {
			return s.propose(it, v, delay), true
		}
		return nil, true
	case " ", "enter":
		switch it.ctl {
		case ctlToggle:
			return s.propose(it, b2i(!known || cur == 0), 0), true
		case ctlCycle:
			c := 0
			for i, o := range it.options {
				if known && o == cur {
					c = i
				}
			}
			s.picker = &picker{cursor: c}
			return nil, true
		}
	}
	return nil, false
}

func b2i(b bool) int {
	if b {
		return 1
	}
	return 0
}

func (s *settings) pickerKey(key string, delay time.Duration) tea.Cmd {
	it := s.items[s.cursor]
	p := s.picker
	switch key {
	case "up", "k":
		p.cursor = max(p.cursor-1, 0)
	case "down", "j":
		p.cursor = min(p.cursor+1, len(it.options)-1)
	case "home", "g":
		p.cursor = 0
	case "end", "G":
		p.cursor = len(it.options) - 1
	case "esc", "q":
		s.picker = nil
	case "enter", " ":
		v := it.options[p.cursor]
		s.picker = nil
		return s.propose(it, v, 0)
	}
	return nil
}

// ---- Mouse -----------------------------------------------------------------------------

// Columns (relative to the panel content) where a row's control starts, and offsets inside it.
const (
	ctlCol        = 2 + labelWidth    // selection prefix + label
	valueCell     = 7                 // width of the value text before the ▶ on bar controls
	sliderRightAt = 2 + valueCell + 1 // "◀ " + value cell + " " → the ▶
	sliderBarAt   = sliderRightAt + 1 + 2
)

// click handles a left click on the control of item i at content column x (relative to the
// panel content, 0 = first column inside the border). The row is selected first.
func (s *settings) click(i, x int, st state.State, delay time.Duration) tea.Cmd {
	if !s.selectable(i, st) {
		return nil
	}
	s.cursor = i
	it := s.items[i]
	rel := x - ctlCol
	if rel < 0 {
		return nil // label area: select only
	}
	cur, known, _ := s.current(it, st)
	switch it.ctl {
	case ctlToggle:
		return s.propose(it, b2i(!known || cur == 0), 0)
	case ctlSlider:
		switch {
		case rel == 0:
			return s.keyStep(it, cur, known, -1, delay)
		case rel == sliderRightAt:
			return s.keyStep(it, cur, known, 1, delay)
		case rel >= sliderBarAt && rel < sliderBarAt+sliderCells:
			cell := rel - sliderBarAt
			v := it.min + (cell*(it.max-it.min)+sliderCells/2)/(sliderCells-1)
			v = min(max(v, it.min), it.max)
			if known && v == cur {
				return nil
			}
			return s.propose(it, v, delay)
		}
	case ctlCycle:
		valW := 1
		if known {
			valW = lipgloss.Width(it.format(cur))
		}
		rightAt := 2 + valW + 1
		if it.bar {
			rightAt = sliderRightAt
			if rel >= sliderBarAt && rel < sliderBarAt+sliderCells {
				n := len(it.options)
				v := it.options[((rel-sliderBarAt)*(n-1)+sliderCells/2)/(sliderCells-1)]
				if known && v == cur {
					return nil
				}
				return s.propose(it, v, delay)
			}
		}
		switch {
		case rel == 0:
			return s.keyStep(it, cur, known, -1, delay)
		case rel == rightAt:
			return s.keyStep(it, cur, known, 1, delay)
		case rel >= 2 && rel < 2+valW:
			cmd, _ := s.onKey("enter", st, delay) // open the picker
			return cmd
		}
	}
	return nil
}

func (s *settings) keyStep(it item, cur int, known bool, dir int, delay time.Duration) tea.Cmd {
	if v, changed := step(it, cur, known, dir, false); changed {
		return s.propose(it, v, delay)
	}
	return nil
}

// wheel changes the selected value (up = next/on, down = previous/off) or moves the picker.
func (s *settings) wheel(up bool, st state.State, delay time.Duration) tea.Cmd {
	if s.picker != nil {
		if up {
			return s.pickerKey("up", delay)
		}
		return s.pickerKey("down", delay)
	}
	key := "left"
	if up {
		key = "right"
	}
	cmd, _ := s.onKey(key, st, delay)
	return cmd
}

// ---- Rendering -------------------------------------------------------------------------

// view returns the editor body lines (including the picker when open), the line to keep
// visible, and for every line the index of the item (or picker option) on it, -1 if none.
func (s *settings) view(st state.State) (lines []string, focus int, at []int) {
	if s.picker != nil {
		return s.pickerView(st)
	}
	for i, it := range s.items {
		if it.group != "" {
			if i > 0 {
				lines, at = append(lines, ""), append(at, -1)
			}
			lines, at = append(lines, styleDim.Bold(true).Render(strings.ToUpper(it.group))), append(at, -1)
		}
		if i == s.cursor {
			focus = len(lines)
		}
		lines, at = append(lines, s.row(i, it, st)), append(at, i)
	}
	return lines, focus, at
}

func (s *settings) row(i int, it item, st state.State) string {
	selected := i == s.cursor
	if !s.selectable(i, st) {
		return "  " + padTo(styleDim.Render(it.label), labelWidth) + styleDim.Render("not supported by this mouse")
	}
	prefix, label := "  ", styleValue.Render(it.label)
	if selected {
		prefix, label = styleKey.Render("▸ "), styleTitle.Render(it.label)
	}
	cur, known, pending := s.current(it, st)
	out := prefix + padTo(label, labelWidth) + s.control(it, cur, known, selected)
	if pending {
		out += styleDim.Render(" …")
	}
	return out
}

func (s *settings) control(it item, cur int, known, selected bool) string {
	text := styleDim.Render(unknown)
	if known {
		text = styleValue.Render(it.format(cur))
		if selected {
			text = styleTitle.Render(it.format(cur))
		}
	}
	switch it.ctl {
	case ctlToggle:
		box := "[ ]"
		if known && cur != 0 {
			box = styleOK.Render("[x]")
		}
		return box + " " + text
	case ctlSlider:
		left, right := arrows(known, cur > it.min, cur < it.max)
		cells := 0
		if known && it.max > it.min {
			cells = (cur - it.min) * sliderCells / (it.max - it.min)
		}
		bar := styleKey.Render(strings.Repeat("█", cells)) + styleDim.Render(strings.Repeat("░", sliderCells-cells))
		return left + " " + padTo(text, valueCell) + " " + right + "  " + bar
	default:
		idx := -1
		for i, o := range it.options {
			if known && o == cur {
				idx = i
			}
		}
		canLeft, canRight := idx != 0, idx != len(it.options)-1
		if idx < 0 {
			canLeft, canRight = true, true
		}
		left, right := arrows(true, canLeft, canRight)
		if !it.bar {
			return left + " " + text + " " + right
		}
		cells := 0
		if idx >= 0 && len(it.options) > 1 {
			cells = max((idx*sliderCells+(len(it.options)-1)/2)/(len(it.options)-1), 1)
		}
		bar := styleKey.Render(strings.Repeat("█", cells)) + styleDim.Render(strings.Repeat("░", sliderCells-cells))
		return left + " " + padTo(text, valueCell) + " " + right + "  " + bar
	}
}

func arrows(known, canLeft, canRight bool) (l, r string) {
	l, r = styleDim.Render("◀"), styleDim.Render("▶")
	if !known {
		return
	}
	if canLeft {
		l = styleKey.Render("◀")
	}
	if canRight {
		r = styleKey.Render("▶")
	}
	return
}

func (s *settings) pickerView(st state.State) ([]string, int, []int) {
	it := s.items[s.cursor]
	cur, known, _ := s.current(it, st)
	lines := []string{styleTitle.Render(it.label), ""}
	at := []int{-1, -1}
	focus := 0
	for i, o := range it.options {
		mark, cursor := " ", "  "
		if known && o == cur {
			mark = styleOK.Render("●")
		}
		label := styleValue.Render(it.format(o))
		if i == s.picker.cursor {
			cursor, label = styleKey.Render("▸ "), styleTitle.Render(it.format(o))
			focus = len(lines)
		}
		lines, at = append(lines, cursor+mark+" "+label), append(at, i)
	}
	return lines, focus, at
}

// windowStart returns the first visible line when showing h of n lines with focus visible.
func windowStart(n, focus, h int) int {
	if n <= h {
		return 0
	}
	return min(max(focus-h/2, 0), n-h)
}

// window returns at most h lines containing focus, keeping it roughly centred.
func window(lines []string, focus, h int) []string {
	if h <= 0 {
		return nil
	}
	start := windowStart(len(lines), focus, h)
	return lines[start:min(start+h, len(lines))]
}

// keyHints returns the footer key hints for the current selection.
func (s *settings) keyHints() [][2]string {
	if s.picker != nil {
		return [][2]string{{"↑↓", "select"}, {"enter", "apply"}, {"esc", "cancel"}}
	}
	nav := [2]string{"↑↓", "select"}
	switch s.items[s.cursor].ctl {
	case ctlToggle:
		return [][2]string{nav, {"←→", "off/on"}, {"space", "toggle"}}
	case ctlSlider:
		return [][2]string{nav, {"←→", "±1"}, {"⇧←→", "±5"}}
	}
	return [][2]string{nav, {"←→", "change"}, {"enter", "pick list"}}
}

// ---- Toasts ----------------------------------------------------------------------------

type toast struct {
	id   int
	text string
	err  bool
}

type toastExpireMsg struct{ id int }

func (t toast) render() string {
	if t.text == "" {
		return ""
	}
	if t.err {
		return styleBad.Render("✕ " + t.text)
	}
	return styleOK.Render("✓ " + t.text)
}

// describeError turns a device error into a short user-facing sentence.
func describeError(k device.Kind, err error) string {
	switch {
	case errors.Is(err, device.ErrUnsupported):
		return fmt.Sprintf("%s is not supported by this mouse", k)
	case errors.Is(err, device.ErrNotConnected):
		return "No mouse connected"
	case errors.Is(err, device.ErrConnectionLost):
		return "Connection to the mouse was lost"
	case errors.Is(err, device.ErrMismatch):
		return fmt.Sprintf("%s: the mouse kept a different value", k)
	}
	return fmt.Sprintf("%s: %v", k, err)
}
