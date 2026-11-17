# incott

A cross-platform (Windows, macOS, Linux) **terminal UI** to view and configure Incott mice
(Ghero, G23, G24, G23V2, Zero 29, Zero 39) over HID. No vendor software needed.
It is a rewrite of the Windows-only [IncottHIDApp](https://github.com/romkazor/IncottHIDApp) for the terminal, without the tray icon and auto-boost.

```
╭─ INCOTT · G23 V2 ─────────────────────────────────── ● Connected (wireless) ─╮
│ Battery   ████████░░ 84%      DPI        1600       Motion sync  On          │
│ Polling   8000 Hz             LOD        0.7 mm     Angle snap   Off         │
│ Sleep     5 min               Debounce   4 ms       Ripple       On          │
│ Rcv LED   Battery status                                                     │
╰──────────────────────────────────────────────────────────────────────────────╯
╭─ Settings ───────────────────────────────────────────────────────────────────╮
│ PERFORMANCE                                                                  │
│ ▸ DPI               ◀ 1600    ▶  ██████░░░░░░░░░                             │
│   Polling rate      ◀ 8000 Hz ▶  ███████████████                             │
│   Lift-off distance ◀ 0.7 mm ▶                                               │
│                                                                              │
│ RESPONSIVENESS                                                               │
│   Debounce          ◀ 4 ms    ▶  ██░░░░░░░░░░░░░                             │
│   Motion sync       [x] On                                                   │
│   Angle snapping    [ ] Off                                                  │
│   Ripple control    [x] On                                                   │
│                                                                              │
│ POWER & LED                                                                  │
│   Sleep timer       ◀ 5 min ▶                                                │
│   Receiver LED      ◀ Battery status ▶                                       │
│                                                                              │
╰──────────────────────────────────────────────────────────────────────────────╯
 ↑↓ select · ←→ change · enter pick list · ? help · r refresh · q quit
```

The top panel always shows what the mouse reports (connection, battery, every setting); the
panel below edits them. Changes are confirmed by reading the value back from the mouse.

## Features
- DPI (400–6400), polling rate (125–8000 Hz), lift-off distance (0.7/1/2 mm), debounce (0–30 ms),
  sleep timer (10 s–15 min), motion sync, angle snapping, ripple control, receiver LED mode
  (only on mice that support it).
- Keyboard and mouse control, help overlay (`?`), light/dark themes, `NO_COLOR` support.
- `--demo` runs against a simulated mouse, so you can try the UI without hardware.

## Settings explained
The same text is available in the app: press `?` and scroll.

<!-- settings:start -->
#### Performance

**DPI** — 400, 800, 1600, 2400, 3200, 6400

- *Purpose:* Sensor sensitivity: how many counts the sensor reports for each inch you move the mouse.
- *Effect:* Higher DPI moves the cursor farther for the same hand movement; lower DPI moves it less.
- *Tip:* Low values (400–1600) suit precise aiming and gaming; high values suit large or multi-monitor desktops. Combine with your game's or OS sensitivity: only the overall result matters.

**Polling rate** — 125, 250, 500, 1000, 2000, 4000, 8000 Hz

- *Purpose:* How many times per second the mouse reports its position and clicks to the computer.
- *Effect:* 125 Hz is one report every 8 ms, 1000 Hz every 1 ms, 8000 Hz every 0.125 ms. A higher rate means lower latency and a smoother cursor path, especially at high speed or on high-refresh-rate screens.
- *Tip:* Higher rates cost battery on a wireless mouse and use more CPU (8000 Hz can stress some games or USB hubs; use a port directly on the computer). 1000 Hz is a good default; use lower rates to save battery.

**Lift-off distance** — 1 mm, 2 mm (0.7 mm on mice that support it)

- *Purpose:* How far the mouse can be raised from the surface before the sensor stops tracking.
- *Effect:* A lower value stops tracking almost as soon as you lift the mouse, so the cursor does not drift while you reposition it. A higher value keeps tracking while slightly lifted, which can move the cursor unintentionally.
- *Tip:* Pick the lowest value that still tracks reliably on your mousepad. Players who lift the mouse often (low sensitivity) usually prefer the lowest. Some mice ignore 0.7 mm; the app then tells you and keeps the previous value.

#### Responsiveness

**Debounce** — 0–30 ms

- *Purpose:* Filters the electrical noise of a button switch so one physical click is not registered as two.
- *Effect:* After a click the mouse ignores further switch changes for this time. A higher value blocks accidental double clicks; a lower value makes rapid clicking feel more immediate.
- *Tip:* Start around 2–8 ms. Raise it if you see unwanted double clicks (a sign of a worn switch); lower it for fast-clicking games. Too high can swallow quick intentional double clicks.

**Motion sync** — On / Off

- *Purpose:* Aligns the sensor's data with the moments the mouse reports to the computer.
- *Effect:* On: reports are evenly spaced, so movement looks consistent and smooth. Off: the newest sensor data is sent as soon as possible, with slightly less regular spacing.
- *Tip:* On adds a very small delay that depends on the polling rate and is negligible at 1000 Hz and above. Many players leave it on; turn it off only if you prefer the absolute minimum latency.

**Angle snapping** — On / Off

- *Purpose:* Straightens small deviations in your hand movement.
- *Effect:* On: the cursor path is corrected toward straight horizontal or vertical lines. Off: the cursor follows your hand exactly.
- *Tip:* Useful for drawing or dragging in straight lines. Leave it off for gaming and general use, because it makes natural, slightly curved movements less accurate.

**Ripple control** — On / Off

- *Purpose:* Sensor-side smoothing that reduces tiny cursor wobble caused by noise on some surfaces or at high DPI.
- *Effect:* On: the sensor filters small irregularities, giving a steadier path. Off: raw sensor movement is reported unfiltered.
- *Tip:* Leave it off for the most direct 1:1 tracking. Try it on if the cursor shakes or jitters at high DPI. The exact filtering is defined by the mouse firmware.

#### Power & LED

**Sleep timer** — 10 s to 15 min

- *Purpose:* How long the mouse can sit idle before it goes to sleep to save battery.
- *Effect:* A shorter time saves more battery. A longer time keeps the mouse ready, so it reacts instantly after a pause. A sleeping mouse wakes with a click or movement and may need a moment to reconnect.
- *Tip:* Use a short time (10–60 s) to maximise battery life; use a longer time if the wake-up delay annoys you. If values show "—" after a long idle, wake the mouse and press r.

**Receiver LED** — Connect & polling rate / Battery status / Battery warning

- *Purpose:* What the status light on the wireless receiver (dongle) shows.
- *Effect:* Connect & polling rate: signals the link and the current polling rate. Battery status: shows the battery level. Battery warning: lights up only when the battery is low.
- *Tip:* Only available on mice that answer this setting; otherwise it is greyed out as not supported.

#### Status panel (top)

**Battery and connection** — read-only

- *Purpose:* Shows what the mouse itself reports right now.
- *Effect:* The battery bar is green above 40%, amber from 15 to 40% and red below 15%; ⚡ marks charging (inferred from the battery data). The badge shows connected, connecting, not connected or permission denied, and whether the mouse is wireless or on its charging cable.
- *Tip:* After you change a setting the panel updates once the mouse confirms it, so it always shows the real state. Press r to re-read everything.

<!-- settings:end -->

## Install
Download the archive for your OS from the [Releases](../../releases) page, check it against
`SHA256SUMS`, unpack it and run `incott`.

- **Linux:** install the udev rule once, then replug the mouse:
  `sudo cp 99-incott.rules /etc/udev/rules.d/ && sudo udevadm control --reload && sudo udevadm trigger`
- **macOS:** the binary is not signed. If Gatekeeper blocks it: `xattr -d com.apple.quarantine ./incott`.
  Allow your terminal under *System Settings → Privacy & Security → Input Monitoring* if the mouse cannot be opened.
- **Windows:** nothing to install. Close vendor software or browser tabs that hold the mouse (WebHID).

## Usage
```
incott [--demo] [--theme auto|dark|light] [--debug] [--log FILE] [--version]
```
Press `?` in the app for all keys. Basics: `↑↓` select, `←→` change, `enter` pick list, `r` re-read, `q` quit.

## Build from source
See [docs/BUILDING.md](docs/BUILDING.md). In short: Go + a C compiler, then `make build`
(or `scripts\build.ps1` on Windows).

## Releasing (maintainers)
Releases are built by hand: GitHub → *Actions* → **Release** → *Run workflow*, enter a tag such as `v0.1.0`.
It tests and builds natively on Linux (amd64, arm64), macOS (arm64, amd64) and Windows (amd64), packages
the archives, writes `SHA256SUMS` and creates a **draft** release to review and publish. The **Build** workflow
produces plain binaries as workflow artifacts without creating a release. Regular CI (vet, tests, lint) runs on every push and pull request.

## Credits
Protocol knowledge derived from [IncottHIDApp](https://github.com/romkazor/IncottHIDApp) (MIT, by romkazor), the original Windows-only tray app, and confirmed on real hardware;
see [docs/protocol.md](docs/protocol.md). Licensed under the MIT license.
