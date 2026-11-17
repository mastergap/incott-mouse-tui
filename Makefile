# Build helpers for Linux/macOS (and Windows with make). See docs/BUILDING.md.
BINARY  := incott
VERSION := $(shell git describe --tags --always --dirty 2>/dev/null || echo dev)
LDFLAGS := -s -w -X main.version=$(VERSION)
EXT     := $(shell go env GOEXE)

.PHONY: build test lint run clean doctor

build: doctor
	CGO_ENABLED=1 go build -trimpath -ldflags "$(LDFLAGS)" -o bin/$(BINARY)$(EXT) ./cmd/incott

test:
	go test ./... -race -cover

lint:
	golangci-lint run

run:
	go run ./cmd/incott --demo

clean:
	rm -rf bin dist coverage.out

doctor:
	@sh scripts/doctor.sh
