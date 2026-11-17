package logging

import (
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestSetupWritesDebugLog(t *testing.T) {
	path := filepath.Join(t.TempDir(), "sub", "x.log")
	c, err := Setup(path, true)
	if err != nil {
		t.Fatal(err)
	}
	slog.Debug("hello", "k", 1)
	_ = c.Close()
	b, err := os.ReadFile(path)
	if err != nil || !strings.Contains(string(b), "hello") {
		t.Fatalf("log = %q, %v", b, err)
	}
}

func TestSetupDisabledCreatesNothing(t *testing.T) {
	path := filepath.Join(t.TempDir(), "x.log")
	c, _ := Setup(path, false)
	slog.Info("nope")
	_ = c.Close()
	if _, err := os.Stat(path); err == nil {
		t.Error("file must not exist when logging is off")
	}
}

func TestSetupBadPath(t *testing.T) {
	f := filepath.Join(t.TempDir(), "file")
	_ = os.WriteFile(f, nil, 0o600)
	if _, err := Setup(filepath.Join(f, "x.log"), true); err == nil {
		t.Error("expected error")
	}
	if DefaultPath() == "" {
		t.Error("default path")
	}
}
