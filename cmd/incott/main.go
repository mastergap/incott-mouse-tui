// Command incott is a cross-platform terminal UI to view and configure Incott mice.
package main

import (
	"context"
	"flag"
	"fmt"
	"os"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/mastergap/incott-mouse-tui/internal/device"
	"github.com/mastergap/incott-mouse-tui/internal/logging"
	"github.com/mastergap/incott-mouse-tui/internal/tui"
)

// version is injected at build time: -ldflags "-X main.version=v1.2.3".
var version = "dev"

func main() {
	showVersion := flag.Bool("version", false, "print version and exit")
	demo := flag.Bool("demo", false, "run against a simulated mouse (no hardware needed)")
	theme := flag.String("theme", os.Getenv("INCOTT_THEME"), "colour theme: auto, dark or light (env INCOTT_THEME)")
	debug := flag.Bool("debug", false, "write a debug log")
	logPath := flag.String("log", "", "debug log file (default: user cache dir, with --debug)")
	flag.Parse()

	if *showVersion {
		fmt.Println("incott", version)
		return
	}
	if err := tui.SetTheme(*theme); err != nil {
		fmt.Fprintln(os.Stderr, "incott:", err)
		os.Exit(2)
	}
	closer, err := logging.Setup(*logPath, *debug)
	if err != nil {
		fmt.Fprintln(os.Stderr, "incott: log:", err)
		os.Exit(1)
	}
	if err := run(*demo); err != nil {
		_ = closer.Close()
		fmt.Fprintln(os.Stderr, "incott:", err)
		os.Exit(1)
	}
	_ = closer.Close()
}

func run(demo bool) error {
	var (
		conn  device.Connector = device.HIDConnector{}
		hooks *tui.DemoHooks
	)
	if demo {
		mouse := device.NewFakeMouse()
		mouse.Name = "Incott G23 V2 (demo)"
		mouse.LEDSupported = true
		conn, hooks = mouse.Connector(), demoHooks(mouse)
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	client := device.New(conn, device.Options{})
	done := make(chan struct{})
	go func() { client.Run(ctx); close(done) }()

	_, err := tea.NewProgram(tui.New(client, hooks), tea.WithAltScreen(), tea.WithMouseCellMotion(), tea.WithContext(ctx)).Run()
	cancel()
	<-done
	return err
}

// demoHooks lets the demo UI simulate unplugging and a permission failure.
func demoHooks(m *device.FakeMouse) *tui.DemoHooks {
	plugged, denied := true, false
	return &tui.DemoHooks{
		TogglePlug: func() { plugged = !plugged; m.Plug(plugged) },
		TogglePermission: func() {
			denied = !denied
			if denied {
				m.FailOpen(device.ErrPermission)
				// Replug: the open connection breaks and the client's next attempt fails.
				m.Plug(false)
				m.Plug(true)
			} else {
				m.FailOpen(nil)
			}
		},
	}
}
