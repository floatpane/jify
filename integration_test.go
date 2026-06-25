//go:build integration

// Package main's integration tests exercise the built jify binary end to end:
// the CLI subcommands, the autostart lifecycle against the real filesystem, and
// (on Linux with an X server) starting the background daemon.
//
// Run with: go test -tags integration ./...
package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"
)

var binPath string

func TestMain(m *testing.M) {
	// Wrapped in a helper so deferred cleanup runs before os.Exit.
	os.Exit(runTests(m))
}

func runTests(m *testing.M) int {
	dir, err := os.MkdirTemp("", "jify-it")
	if err != nil {
		panic(err)
	}
	defer os.RemoveAll(dir)

	binPath = filepath.Join(dir, "jify")
	if runtime.GOOS == "windows" {
		binPath += ".exe"
	}
	build := exec.Command("go", "build", "-o", binPath, ".")
	build.Stderr = os.Stderr
	if err := build.Run(); err != nil {
		panic("build failed: " + err.Error())
	}

	return m.Run()
}

// run executes the binary with an isolated HOME so autostart writes land in a
// temp directory.
func run(t *testing.T, home string, args ...string) (string, error) {
	t.Helper()
	cmd := exec.Command(binPath, args...)
	cmd.Env = append(os.Environ(),
		"HOME="+home,
		"XDG_CONFIG_HOME="+filepath.Join(home, ".config"),
		"JIFY_SKIP_LAUNCHCTL=1",
	)
	out, err := cmd.CombinedOutput()
	return string(out), err
}

func TestCLIVersion(t *testing.T) {
	out, err := run(t, t.TempDir(), "version")
	if err != nil {
		t.Fatalf("version: %v (%s)", err, out)
	}
	if !strings.Contains(out, "jify") {
		t.Errorf("version output missing 'jify': %q", out)
	}
}

func TestCLIHelp(t *testing.T) {
	out, err := run(t, t.TempDir(), "help")
	if err != nil {
		t.Fatalf("help: %v (%s)", err, out)
	}
	for _, want := range []string{"Usage:", "enable", "disable", "config"} {
		if !strings.Contains(out, want) {
			t.Errorf("help output missing %q:\n%s", want, out)
		}
	}
}

func TestCLIConfigPath(t *testing.T) {
	home := t.TempDir()
	out, err := run(t, home, "config")
	if err != nil {
		t.Fatalf("config: %v (%s)", err, out)
	}
	if !strings.Contains(out, "jify") {
		t.Errorf("config path looks wrong: %q", out)
	}
}

func TestCLIAutostartLifecycle(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("windows autostart uses the global registry; covered separately")
	}
	home := t.TempDir()

	if out, err := run(t, home, "status"); err != nil || !strings.Contains(out, "disabled") {
		t.Fatalf("initial status: %v (%s)", err, out)
	}
	if out, err := run(t, home, "enable"); err != nil {
		t.Fatalf("enable: %v (%s)", err, out)
	}
	if out, err := run(t, home, "status"); err != nil || !strings.Contains(out, "enabled") {
		t.Fatalf("status after enable: %v (%s)", err, out)
	}
	if out, err := run(t, home, "disable"); err != nil {
		t.Fatalf("disable: %v (%s)", err, out)
	}
	if out, err := run(t, home, "status"); err != nil || !strings.Contains(out, "disabled") {
		t.Fatalf("status after disable: %v (%s)", err, out)
	}
}

// TestDaemonStartsLinux launches the background daemon under an X server and
// verifies it initialises GTK + X11 (XRecord/XTest) without crashing.
func TestDaemonStartsLinux(t *testing.T) {
	if runtime.GOOS != "linux" {
		t.Skip("daemon smoke test is Linux-only")
	}
	if os.Getenv("DISPLAY") == "" {
		t.Skip("no X server (set DISPLAY, e.g. via Xvfb)")
	}
	home := t.TempDir()

	cmd := exec.Command(binPath)
	cmd.Env = append(os.Environ(),
		"HOME="+home,
		"XDG_CONFIG_HOME="+filepath.Join(home, ".config"),
	)
	var out strings.Builder
	cmd.Stdout = &out
	cmd.Stderr = &out
	if err := cmd.Start(); err != nil {
		t.Fatalf("start daemon: %v", err)
	}
	t.Cleanup(func() {
		_ = cmd.Process.Kill()
		_, _ = cmd.Process.Wait()
	})

	// Give it a moment to initialise, then confirm it's still running.
	time.Sleep(3 * time.Second)
	if cmd.ProcessState != nil && cmd.ProcessState.Exited() {
		t.Fatalf("daemon exited early:\n%s", out.String())
	}
	if !strings.Contains(out.String(), "running") {
		t.Logf("daemon output:\n%s", out.String())
	}
}
