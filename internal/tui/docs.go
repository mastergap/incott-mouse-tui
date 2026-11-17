package tui

import (
	"strings"
)

// docEntry explains one setting (or panel item). It is the single source for the in-app help
// and for the "Settings explained" section of README.md (kept in sync by a test).
type docEntry struct {
	Group   string // heading shown before the first entry of a group
	Name    string
	Values  string // the choices this mouse offers
	Purpose string // what it is for
	Effect  string // how it changes the mouse's behaviour
	Tip     string // when to change it / trade-offs
}

var settingDocs = []docEntry{
	{
		Group: "Performance", Name: "DPI", Values: "400, 800, 1600, 2400, 3200, 6400",
		Purpose: "Sensor sensitivity: how many counts the sensor reports for each inch you move the mouse.",
		Effect:  "Higher DPI moves the cursor farther for the same hand movement; lower DPI moves it less.",
		Tip:     "Low values (400–1600) suit precise aiming and gaming; high values suit large or multi-monitor desktops. Combine with your game's or OS sensitivity: only the overall result matters.",
	},
	{
		Name: "Polling rate", Values: "125, 250, 500, 1000, 2000, 4000, 8000 Hz",
		Purpose: "How many times per second the mouse reports its position and clicks to the computer.",
		Effect:  "125 Hz is one report every 8 ms, 1000 Hz every 1 ms, 8000 Hz every 0.125 ms. A higher rate means lower latency and a smoother cursor path, especially at high speed or on high-refresh-rate screens.",
		Tip:     "Higher rates cost battery on a wireless mouse and use more CPU (8000 Hz can stress some games or USB hubs; use a port directly on the computer). 1000 Hz is a good default; use lower rates to save battery.",
	},
	{
		Name: "Lift-off distance", Values: "1 mm, 2 mm (0.7 mm on mice that support it)",
		Purpose: "How far the mouse can be raised from the surface before the sensor stops tracking.",
		Effect:  "A lower value stops tracking almost as soon as you lift the mouse, so the cursor does not drift while you reposition it. A higher value keeps tracking while slightly lifted, which can move the cursor unintentionally.",
		Tip:     "Pick the lowest value that still tracks reliably on your mousepad. Players who lift the mouse often (low sensitivity) usually prefer the lowest. Some mice ignore 0.7 mm; the app then tells you and keeps the previous value.",
	},
	{
		Group: "Responsiveness", Name: "Debounce", Values: "0–30 ms",
		Purpose: "Filters the electrical noise of a button switch so one physical click is not registered as two.",
		Effect:  "After a click the mouse ignores further switch changes for this time. A higher value blocks accidental double clicks; a lower value makes rapid clicking feel more immediate.",
		Tip:     "Start around 2–8 ms. Raise it if you see unwanted double clicks (a sign of a worn switch); lower it for fast-clicking games. Too high can swallow quick intentional double clicks.",
	},
	{
		Name: "Motion sync", Values: "On / Off",
		Purpose: "Aligns the sensor's data with the moments the mouse reports to the computer.",
		Effect:  "On: reports are evenly spaced, so movement looks consistent and smooth. Off: the newest sensor data is sent as soon as possible, with slightly less regular spacing.",
		Tip:     "On adds a very small delay that depends on the polling rate and is negligible at 1000 Hz and above. Many players leave it on; turn it off only if you prefer the absolute minimum latency.",
	},
	{
		Name: "Angle snapping", Values: "On / Off",
		Purpose: "Straightens small deviations in your hand movement.",
		Effect:  "On: the cursor path is corrected toward straight horizontal or vertical lines. Off: the cursor follows your hand exactly.",
		Tip:     "Useful for drawing or dragging in straight lines. Leave it off for gaming and general use, because it makes natural, slightly curved movements less accurate.",
	},
	{
		Name: "Ripple control", Values: "On / Off",
		Purpose: "Sensor-side smoothing that reduces tiny cursor wobble caused by noise on some surfaces or at high DPI.",
		Effect:  "On: the sensor filters small irregularities, giving a steadier path. Off: raw sensor movement is reported unfiltered.",
		Tip:     "Leave it off for the most direct 1:1 tracking. Try it on if the cursor shakes or jitters at high DPI. The exact filtering is defined by the mouse firmware.",
	},
	{
		Group: "Power & LED", Name: "Sleep timer", Values: "10 s to 15 min",
		Purpose: "How long the mouse can sit idle before it goes to sleep to save battery.",
		Effect:  "A shorter time saves more battery. A longer time keeps the mouse ready, so it reacts instantly after a pause. A sleeping mouse wakes with a click or movement and may need a moment to reconnect.",
		Tip:     "Use a short time (10–60 s) to maximise battery life; use a longer time if the wake-up delay annoys you. If values show \"—\" after a long idle, wake the mouse and press r.",
	},
	{
		Name: "Receiver LED", Values: "Connect & polling rate / Battery status / Battery warning",
		Purpose: "What the status light on the wireless receiver (dongle) shows.",
		Effect:  "Connect & polling rate: signals the link and the current polling rate. Battery status: shows the battery level. Battery warning: lights up only when the battery is low.",
		Tip:     "Only available on mice that answer this setting; otherwise it is greyed out as not supported.",
	},
	{
		Group: "Status panel (top)", Name: "Battery and connection", Values: "read-only",
		Purpose: "Shows what the mouse itself reports right now.",
		Effect:  "The battery bar is green above 40%, amber from 15 to 40% and red below 15%; ⚡ marks charging (inferred from the battery data). The badge shows connected, connecting, not connected or permission denied, and whether the mouse is wireless or on its charging cable.",
		Tip:     "After you change a setting the panel updates once the mouse confirms it, so it always shows the real state. Press r to re-read everything.",
	},
}

// settingsMarkdown renders settingDocs as the README section.
func settingsMarkdown() string {
	var b strings.Builder
	for _, d := range settingDocs {
		if d.Group != "" {
			b.WriteString("\n#### " + d.Group + "\n")
		}
		b.WriteString("\n**" + d.Name + "**")
		if d.Values != "" {
			b.WriteString(" — " + d.Values)
		}
		b.WriteString("\n\n")
		b.WriteString("- *Purpose:* " + d.Purpose + "\n")
		b.WriteString("- *Effect:* " + d.Effect + "\n")
		b.WriteString("- *Tip:* " + d.Tip + "\n")
	}
	return strings.TrimLeft(b.String(), "\n")
}
