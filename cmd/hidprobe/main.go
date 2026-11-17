// Command hidprobe is a hardware diagnostic for the Incott HID protocol.
//
// It enumerates Incott HID interfaces, opens each candidate interface, issues every read
// command and prints the raw 9-byte responses next to their decoded values, then listens for
// interrupt (status) reports. Run it on each OS with a real mouse and paste the output into
// docs/protocol.md. It is read-only unless -send is given.
//
//	hidprobe                      # enumerate + probe every candidate interface
//	hidprobe -listen 10s          # listen longer for status reports (move the mouse, change DPI)
//	hidprobe -send "09 08 01"     # send a raw feature report, then re-read everything
package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"runtime"
	"strconv"
	"strings"
	"time"

	"github.com/karalabe/hid"

	"github.com/mastergap/incott-mouse-tui/internal/device"
	"github.com/mastergap/incott-mouse-tui/internal/protocol"
)

var version = "dev"

func main() {
	listen := flag.Duration("listen", 5*time.Second, "how long to listen for interrupt status reports per interface")
	send := flag.String("send", "", `raw hex feature report to send to the first responding interface, e.g. "09 08 01"`)
	clientFor := flag.Duration("client", 0, "instead of probing, run the real device.Client for this long and print every state change")
	all := flag.Bool("all", false, "also probe interfaces whose product string does not look like a mouse")
	flag.Parse()

	fmt.Printf("hidprobe %s — %s/%s, hid.Supported=%v\n\n", version, runtime.GOOS, runtime.GOARCH, hid.Supported())

	if *clientFor > 0 {
		runClient(*clientFor)
		return
	}

	infos, err := hid.Enumerate(protocol.VendorID, 0)
	if err != nil {
		fail("enumerate: %v", err)
	}
	if len(infos) == 0 {
		fmt.Printf("No HID interfaces with vendor ID 0x%04X found.\n", protocol.VendorID)
		fmt.Println("Check: dongle plugged in / mouse on cable, Linux udev permissions, macOS Input Monitoring.")
		os.Exit(2)
	}

	fmt.Println("== Enumerated interfaces ==")
	var candidates []hid.DeviceInfo
	for i, d := range infos {
		isMouse := protocol.IsMouseProduct(d.Product)
		known := protocol.IsKnownProductID(d.ProductID)
		fmt.Printf("[%d] pid=0x%04X usagePage=0x%04X usage=0x%04X iface=%d release=0x%04X\n"+
			"    product=%q manufacturer=%q serial=%q\n    path=%s\n    knownPID=%v mouseProduct=%v\n",
			i, d.ProductID, d.UsagePage, d.Usage, d.Interface, d.Release,
			d.Product, d.Manufacturer, d.Serial, d.Path, known, isMouse)
		if known && (isMouse || *all) {
			candidates = append(candidates, d)
		}
	}
	if len(candidates) == 0 {
		fmt.Println("\nNo candidate interfaces (known PID + mouse product). Use -all to probe everything.")
		os.Exit(2)
	}

	var responder *hid.DeviceInfo
	for i := range candidates {
		c := candidates[i]
		fmt.Printf("\n== Probing pid=0x%04X usagePage=0x%04X iface=%d path=%s ==\n", c.ProductID, c.UsagePage, c.Interface, c.Path)
		dev, err := c.Open()
		if err != nil {
			fmt.Printf("  open failed: %v\n", err)
			continue
		}
		got := probe(dev)
		if got && responder == nil {
			responder = &candidates[i]
		}
		if *listen > 0 {
			listenStatus(dev, *listen)
		}
		_ = dev.Close()
	}

	if *send != "" {
		if responder == nil {
			fail("-send: no interface answered the read commands; nothing to send to")
		}
		raw, err := parseHex(*send)
		if err != nil {
			fail("-send: %v", err)
		}
		fmt.Printf("\n== Sending feature report % X to iface=%d, then re-reading ==\n", raw, responder.Interface)
		dev, err := responder.Open()
		if err != nil {
			fail("open: %v", err)
		}
		n, err := dev.SendFeatureReport(raw)
		fmt.Printf("  SendFeatureReport: n=%d err=%v\n", n, err)
		time.Sleep(200 * time.Millisecond)
		probe(dev)
		_ = dev.Close()
	}
}

type readCase struct {
	name  string
	req   protocol.Request
	parse func(buf []byte) string
}

func cases() []readCase {
	return []readCase{
		{"status", protocol.ReadStatus, func(b []byte) string {
			s, err := protocol.ParseStatusResponse(b)
			if err != nil {
				return "ERR " + err.Error()
			}
			return fmt.Sprintf("battery=%d%% charging(inferred)=%v dpi=%d polling=%dHz", s.Battery, s.Charging, s.DPI, s.Polling)
		}},
		{"lod+motionsync", protocol.ReadLODMotionSync, func(b []byte) string {
			l, m, err := protocol.ParseLODMotionSync(b)
			if err != nil {
				return "ERR " + err.Error()
			}
			return fmt.Sprintf("lod=%s motionSync=%v", protocol.LODLabel(l), m)
		}},
		{"ripple", protocol.ReadRippleControl, func(b []byte) string { v, err := protocol.ParseRippleControl(b); return res(v, err) }},
		{"angleSnap", protocol.ReadAngleSnapping, func(b []byte) string { v, err := protocol.ParseAngleSnapping(b); return res(v, err) }},
		{"debounce", protocol.ReadDebounce, func(b []byte) string { v, err := protocol.ParseDebounce(b); return res(v, err) }},
		{"sleep", protocol.ReadSleep, func(b []byte) string { v, err := protocol.ParseSleep(b); return res(v, err) }},
		{"receiverLED", protocol.ReadReceiverLED, func(b []byte) string {
			v, err := protocol.ParseReceiverLED(b)
			if err != nil {
				return "ERR " + err.Error()
			}
			return fmt.Sprintf("%d (%s)", int(v), v)
		}},
	}
}

func res[T any](v T, err error) string {
	if err != nil {
		return "ERR " + err.Error()
	}
	return fmt.Sprint(v)
}

// probe runs every read command and reports whether at least one was answered.
func probe(dev hid.Device) bool {
	any := false
	for _, c := range cases() {
		buf, tries, err := readSetting(dev, c.req)
		switch {
		case err != nil:
			fmt.Printf("  %-15s request % X -> %v\n", c.name, c.req.Report[:], err)
		default:
			any = true
			fmt.Printf("  %-15s request % X -> response % X (poll %d)  %s\n", c.name, c.req.Report[:], buf[:protocol.ReportLen], tries, c.parse(buf))
		}
	}
	return any
}

// readSetting mirrors the vendor flow: send the request as a feature report, then poll
// GetFeatureReport every 50 ms (up to 15 times) until a matching response arrives.
func readSetting(dev hid.Device, req protocol.Request) ([]byte, int, error) {
	if _, err := dev.SendFeatureReport(req.Report[:]); err != nil {
		return nil, 0, fmt.Errorf("send failed: %w", err)
	}
	var lastErr error
	for i := 1; i <= 15; i++ {
		time.Sleep(50 * time.Millisecond)
		buf := make([]byte, 64)
		buf[0] = protocol.ReportID
		if _, err := dev.GetFeatureReport(buf); err != nil {
			lastErr = err
			continue
		}
		if req.Matches(buf) {
			return buf, i, nil
		}
	}
	if lastErr != nil {
		return nil, 0, fmt.Errorf("no response (last GetFeatureReport error: %w)", lastErr)
	}
	return nil, 0, fmt.Errorf("no matching response")
}

func listenStatus(dev hid.Device, d time.Duration) {
	fmt.Printf("  -- listening %s for interrupt reports (move the mouse / change DPI) --\n", d)
	deadline := time.Now().Add(d)
	n := 0
	for time.Now().Before(deadline) {
		buf := make([]byte, 64)
		got, err := dev.ReadTimeout(buf, 250)
		if err != nil {
			fmt.Printf("  read error: %v\n", err)
			return
		}
		if got == 0 {
			continue
		}
		n++
		line := fmt.Sprintf("  interrupt (%d bytes): % X", got, buf[:got])
		if got >= protocol.ReportLen && protocol.IsInterruptStatus(buf[:got]) {
			if s, err := protocol.ParseInterruptStatus(buf[:got]); err == nil {
				line += fmt.Sprintf("  -> battery=%d%% charging(inferred)=%v dpi=%d polling=%dHz", s.Battery, s.Charging, s.DPI, s.Polling)
			}
		}
		fmt.Println(line)
	}
	if n == 0 {
		fmt.Println("  (no interrupt reports received)")
	}
}

func parseHex(s string) ([]byte, error) {
	fields := strings.Fields(strings.NewReplacer(",", " ", "0x", "").Replace(s))
	out := make([]byte, 0, len(fields))
	for _, f := range fields {
		v, err := strconv.ParseUint(f, 16, 8)
		if err != nil {
			return nil, fmt.Errorf("bad hex byte %q", f)
		}
		out = append(out, byte(v))
	}
	if len(out) == 0 {
		return nil, fmt.Errorf("empty")
	}
	return out, nil
}

func fail(format string, args ...any) {
	fmt.Fprintf(os.Stderr, "hidprobe: "+format+"\n", args...)
	os.Exit(1)
}

// runClient exercises the production Client (read-only) and prints each published state.
func runClient(d time.Duration) {
	ctx, cancel := context.WithTimeout(context.Background(), d)
	defer cancel()
	c := device.New(device.HIDConnector{}, device.Options{})
	go c.Run(ctx)
	for s := range c.Events() {
		fmt.Printf("%s %+v\n", time.Now().Format("15:04:05.000"), s)
	}
}
