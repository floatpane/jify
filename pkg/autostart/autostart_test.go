//go:build darwin || linux

package autostart

import (
	"path/filepath"
	"runtime"
	"testing"
)

// isolate redirects the autostart location into a temp directory so the test
// never touches the real login items.
func isolate(t *testing.T) {
	t.Helper()
	dir := t.TempDir()
	t.Setenv("HOME", dir)
	if runtime.GOOS == "linux" {
		t.Setenv("XDG_CONFIG_HOME", filepath.Join(dir, ".config"))
	}
	t.Setenv("JIFY_SKIP_LAUNCHCTL", "1") // macOS: don't actually load into launchd
}

func TestEnableDisable(t *testing.T) {
	isolate(t)

	if on, err := IsEnabled(); err != nil || on {
		t.Fatalf("expected disabled initially (on=%v err=%v)", on, err)
	}

	if err := Enable(); err != nil {
		t.Fatalf("Enable: %v", err)
	}
	if on, err := IsEnabled(); err != nil || !on {
		t.Fatalf("expected enabled after Enable (on=%v err=%v)", on, err)
	}

	if err := Disable(); err != nil {
		t.Fatalf("Disable: %v", err)
	}
	if on, err := IsEnabled(); err != nil || on {
		t.Fatalf("expected disabled after Disable (on=%v err=%v)", on, err)
	}
}

func TestSync(t *testing.T) {
	isolate(t)

	if err := Sync(true); err != nil {
		t.Fatalf("Sync(true): %v", err)
	}
	if on, _ := IsEnabled(); !on {
		t.Fatal("Sync(true) should enable")
	}
	// Idempotent.
	if err := Sync(true); err != nil {
		t.Fatalf("Sync(true) again: %v", err)
	}

	if err := Sync(false); err != nil {
		t.Fatalf("Sync(false): %v", err)
	}
	if on, _ := IsEnabled(); on {
		t.Fatal("Sync(false) should disable")
	}
}
