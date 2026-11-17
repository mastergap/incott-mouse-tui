# Incott HID protocol — findings

Sources: the original Windows app (`IncottHIDApp`), and hardware probing with `cmd/hidprobe`.
Implementation: `internal/protocol` (pure functions, golden tests including real captures).

## Interface selection
- VID `0x093A`, PID `0x522C` (wireless) / `0x622C` (charging). Product string varies by unit
  (e.g. `"Wireless Gaming Mouse"`, manufacturer `"PIXART"`), so product keywords are a weak filter.
- The mouse exposes 3 USB interfaces. Configuration lives on the vendor collection **usage page `0xFF05`,
  usage `0x0001`, USB interface 2** (macOS capture). Interface 0 = keyboard (0x0001/0x0006), interface 1 = mouse,
  consumer, system and vendor `0xFF00` collections.
- Feature-report reads/writes worked on every collection of the same device on macOS, but **interrupt status
  reports only arrive on the 0xFF05 interface** → always open `0xFF05`.
- First run on macOS: opening the non-vendor collections failed once and succeeded on later runs (likely the
  Input Monitoring permission prompt). The UI must surface open errors clearly.

## Reports (9 bytes, ID `0x09`, feature reports; read command = set command | 0x80)
See the table in the plan / `internal/protocol/build.go`. Findings from hardware (macOS arm64, PID 522C):

| Read | Request | Real response | Decoded |
|---|---|---|---|
| status | `09 89` | `09 89 57 01 03 08 00 01 5A` | **byte2 battery 0x57=87%, byte3 DPI idx 1=800, byte4 rate idx 3=125 Hz** |
| LOD+motion sync | `09 84 00` | `09 84 00 08 00 0F 0F 11 00` | byte7 `0x11` → LOD idx 1 = 2 mm, motion sync on |
| ripple | `09 84 02` | `09 84 02 00 00 0F 0F 11 00` | byte3 = 0 → off |
| angle snapping | `09 84 03` | `09 84 03 00 00 0F 0F 11 00` | byte3 = 0 → off |
| debounce | `09 85 01` | `09 85 01 08 00 0F 0F 11 00` | byte3 = 8 ms |
| sleep | `09 85 03` | `09 85 03 3C 00 0F 0F 11 00` | LE u16 = 60 s |
| receiver LED | `09 88 00` | **no response** | not supported on this unit |

Interrupt status report (every ~2 s; 33 bytes on macOS, only the first 9 matter):
`09 57 13 08 00 0F 0F 11 00 …` → byte1 battery 87%, byte2 `0x13` = DPI idx 1 (800) | rate idx 3 (125 Hz).

### DPI / polling: `0x89` is stale, interrupts are the truth (hardware trace, macOS)
After `09 03 06 02` (DPI 1600) the mouse pushed `09 4E 23 08 00 1F 1F 11` ~390 ms later (DPI idx 2), while
repeated `09 89` reads kept returning DPI idx 1 (800) for 4+ seconds. Reads of every other setting reflect
writes immediately. So: DPI, polling rate and battery are taken from interrupt reports only; a write is
confirmed by waiting (≤2 s) for an interrupt carrying the new value, and a refresh keeps the live values.
Bytes 5–6 of the interrupt change too (`0F 0F` → `1F 1F` at DPI 1600): meaning unknown.

### LOD 0.7 mm is ignored by this unit (hardware trace, macOS)
`09 04 01 00` (1 mm) and `09 04 01 01` (2 mm) take effect (read-back byte7 `01` / `11`). `09 04 01 02` (0.7 mm)
is accepted without error but the mouse keeps reporting 2 mm (`11`) for at least 1 s. The app treats this as
"0.7 mm not supported by this mouse": it shows a message and drops the option for the session. Other models
of the family may support it. Also: a read 50 ms after a write may return an empty `09 00 00 …` report
(device busy); the client keeps polling until the echo arrives.

### Corrections to the old app
- The old code parsed the `0x89` *response* with the *interrupt* layout (battery in byte 1). The layouts differ
  (response echoes `0x89` in byte 1). It went unnoticed because the first interrupt report overwrote the wrong values.
  `ParseStatusResponse` and `ParseInterruptStatus` are now separate functions.
- Trailing bytes `0F 0F 11` appear in every report; byte 7 mirrors LOD|motion sync and may be a global state block.
  Byte 8 of the status response (`5A`) is unknown (checksum? firmware flag?).

## Open items (need hardware / other OS)
1. **Receiver LED**: no answer to `09 88` on the tested unit. Treat as optional: if the read times out, show the
   setting as unsupported (hidden/disabled) instead of "unknown". The 0/1/2 naming order is still unverified
   (needs a unit that responds; do not guess by writing).
2. **Charging bit**: `battery > 100` ⇒ charging is inferred from the old code only. Verify by plugging the cable
   (the device may re-enumerate as PID `0x622C`) and reading the status.
3. **Wired mode**: probe with the cable connected (PID 0x622C) to confirm the same interface layout.
4. **Windows**: confirm usage page `0xFF05` / interface number; capture output of `hidprobe`.
5. **Linux**: `karalabe/hid` uses a bundled **libusb** backend there (no hidraw, no libudev), which
   - reports `UsagePage`/`Usage` as 0 → select the interface by number (2) when usage page is 0;
   - needs access to `/dev/bus/usb/*` (rule in `packaging/linux/99-incott.rules`) and detaches the kernel driver
     from that interface while open. Validate with `hidprobe` on Linux. If problematic, swap the `Transport`
     implementation for a hidraw-based library (`sstallion/go-hid`) — the interface boundary exists for this.

## How to contribute a capture
```sh
go build -o bin/hidprobe ./cmd/hidprobe && bin/hidprobe -listen 10s > capture-<os>.txt
bin/hidprobe -send "09 01 03"   # optional: raw write (sets 125 Hz), then re-reads everything
```
