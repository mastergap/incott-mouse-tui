package tui

import "github.com/mastergap/incott-mouse-tui/internal/state"

var spinnerFrames = []string{"⠋", "⠙", "⠹", "⠸", "⠼", "⠴", "⠦", "⠧", "⠇", "⠏"}

// hint returns guidance lines shown in the body when the mouse is not usable.
// goos is runtime.GOOS (a parameter so every platform is testable).
func hint(st state.State, goos string, frame int) []string {
	switch st.Connection {
	case state.Disconnected:
		lines := []string{
			styleValue.Render("No Incott mouse found."),
			"",
			styleDim.Render("Plug in the wireless receiver or connect the charging cable."),
			styleDim.Render("The app keeps searching and connects automatically."),
		}
		if st.Err != "" && st.Err != "mouse not found" {
			lines = append(lines, "", styleDim.Render("Last error: "+st.Err))
		}
		return lines
	case state.PermissionDenied:
		lines := []string{styleBad.Render("The mouse was found but cannot be opened."), ""}
		switch goos {
		case "linux":
			lines = append(lines,
				styleDim.Render("Install the udev rule, then unplug and replug the mouse:"),
				styleValue.Render("  incott --print-udev-rule | sudo tee /etc/udev/rules.d/99-incott.rules"),
				styleValue.Render("  sudo udevadm control --reload-rules && sudo udevadm trigger"))
		case "darwin":
			lines = append(lines,
				styleDim.Render("Allow your terminal app under System Settings →"),
				styleDim.Render("Privacy & Security → Input Monitoring, then restart it."))
		default:
			lines = append(lines,
				styleDim.Render("Close other apps that use the mouse (vendor software, browsers"),
				styleDim.Render("with WebHID tabs) and try again."))
		}
		if st.Err != "" {
			lines = append(lines, "", styleDim.Render("Details: "+st.Err))
		}
		return lines
	case state.Connecting:
		return []string{styleWarn.Render(spinnerFrames[frame%len(spinnerFrames)] + " Reading settings from the mouse…")}
	}
	return nil
}
