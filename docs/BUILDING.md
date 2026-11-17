# Building locally

The project is Go. The HID library uses CGO, so you need **Go** and a **C compiler** for your OS.
Builds are native (no cross-compiling): build on the OS you target.

Check your toolchain at any time: `make doctor` (Linux/macOS).

## Prerequisites

### macOS
```sh
brew install go
xcode-select --install     # C compiler (Command Line Tools)
```

### Linux (Debian/Ubuntu)
```sh
sudo apt install golang build-essential
# Fedora: sudo dnf install golang gcc
# Arch:   sudo pacman -S go base-devel
```
Install the udev rule so the mouse is accessible without root:
```sh
sudo cp packaging/linux/99-incott.rules /etc/udev/rules.d/
sudo udevadm control --reload && sudo udevadm trigger
```
(Replug the mouse afterwards.)

### Windows
```powershell
winget install GoLang.Go
winget install BrechtSanders.WinLibs.POSIX.UCRT   # or MSYS2 mingw-w64 (gcc)
# if gcc is not on PATH:  $env:CC = "C:\path\to\gcc.exe"
```

## Build

| OS | Command | Output |
|---|---|---|
| Linux / macOS | `make build` | `bin/incott` |
| Windows (PowerShell) | `scripts\build.ps1` | `bin\incott.exe` |
| Any (with [Task](https://taskfile.dev)) | `task build` | `bin/incott[.exe]` |
| Any | `CGO_ENABLED=1 go build -o bin/incott ./cmd/incott` | `bin/incott` |

The version is taken from `git describe` (falls back to `dev`). Check with `bin/incott --version`.

## Test, lint, run
```sh
make test        # go test ./... -race -cover
make lint        # needs golangci-lint (https://golangci-lint.run)
make run         # TUI against a simulated mouse (no hardware needed); in demo mode: u = unplug/plug, x = permission error
```

## macOS note
macOS may ask to allow **Input Monitoring** for your terminal app the first time the app opens the mouse.

## Troubleshooting
- `C compiler not found` / `cgo: C compiler "gcc" not found`: install the compiler above or set `CC`.
- `CGO_ENABLED=0`: set `CGO_ENABLED=1`.
- Linux: the HID library bundles libusb, so no `-dev` packages are needed; only `gcc`. If the mouse cannot be opened, install the udev rule above and replug.

## CI and GitHub builds
- `.github/workflows/ci.yml` runs on every push to `main` and every pull request: `go vet`, `go test -race`
  (Ubuntu, macOS arm64/amd64, Windows) and lint (`gofmt`, `go mod tidy`, golangci-lint). It does **not** build binaries.
- `.github/workflows/build.yml` is **manual**: GitHub → Actions → *Build* → *Run workflow*, pick the branch/tag
  and optionally a version string. It builds natively on all four runners and uploads the binaries as artifacts.

## Run-time options
```
incott [--demo] [--theme auto|dark|light] [--debug] [--log FILE] [--version]
```
- `--theme` (or env `INCOTT_THEME`): force light or dark colours when detection is wrong. `NO_COLOR=1` disables colours.
- `--debug`: write a debug log (default `<user cache dir>/incott/incott.log`, or `--log FILE`). Nothing is
  ever printed to the terminal while the UI runs.
- A terminal that does not answer the background-colour query delays startup by ~5 s (Bubble Tea issues it at import time).

## Updating screen snapshots
UI layouts are covered by golden files in `internal/tui/testdata`. After an intentional UI change:
`go test ./internal/tui -update`, then review the diff.
