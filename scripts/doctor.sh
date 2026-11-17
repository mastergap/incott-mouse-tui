#!/bin/sh
# Verifies the local toolchain: Go, CGO and a C compiler.
set -eu
fail() { echo "doctor: $1" >&2; exit 1; }

command -v go >/dev/null 2>&1 || fail "Go not found. Install it from https://go.dev/dl/ (see docs/BUILDING.md)."
want=$(sed -n 's/^go \([0-9][0-9]*\.[0-9][0-9]*\).*/\1/p' go.mod | head -1)
have=$(go env GOVERSION | sed 's/^go//')
[ "$(go env CGO_ENABLED)" = "1" ] || fail "CGO is disabled; set CGO_ENABLED=1 (the HID library needs it)."
cc=$(go env CC)
command -v "$cc" >/dev/null 2>&1 || fail "C compiler '$cc' not found. Install build tools (see docs/BUILDING.md) or set CC."
echo "doctor: ok (go $have, needs >= $want; cc=$cc)"
