package config

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

// withTempHome points the OS config dir at a temporary directory so tests don't
// touch the real user config.
func withTempHome(t *testing.T) {
	t.Helper()
	dir := t.TempDir()
	t.Setenv("HOME", dir)
	// Linux honours XDG_CONFIG_HOME for os.UserConfigDir.
	if runtime.GOOS == "linux" {
		t.Setenv("XDG_CONFIG_HOME", filepath.Join(dir, ".config"))
	}
}

func TestLoadCreatesDefault(t *testing.T) {
	withTempHome(t)

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if cfg.Trigger != ":" {
		t.Errorf("default trigger = %q, want \":\"", cfg.Trigger)
	}
	if cfg.MaxSuggestions != 10 {
		t.Errorf("default maxSuggestions = %d, want 10", cfg.MaxSuggestions)
	}
	if !cfg.Autostart {
		t.Error("default autostart should be true")
	}

	path, err := Path()
	if err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(path); err != nil {
		t.Errorf("Load did not create config file: %v", err)
	}
}

func TestSaveRoundTrip(t *testing.T) {
	withTempHome(t)

	cfg := Default()
	cfg.Trigger = ";"
	cfg.MaxSuggestions = 5
	cfg.Autostart = false
	cfg.BlacklistedApps = []string{"com.apple.Terminal"}
	if err := cfg.Save(); err != nil {
		t.Fatalf("Save: %v", err)
	}

	got, err := Load()
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if got.Trigger != ";" || got.MaxSuggestions != 5 || got.Autostart {
		t.Errorf("round trip mismatch: %+v", got)
	}
	if len(got.BlacklistedApps) != 1 || got.BlacklistedApps[0] != "com.apple.Terminal" {
		t.Errorf("blacklist round trip mismatch: %v", got.BlacklistedApps)
	}
}

func TestNormalizeFillsMissing(t *testing.T) {
	c := &Config{} // everything zero
	c.normalize()
	if c.Trigger != ":" {
		t.Errorf("trigger = %q, want \":\"", c.Trigger)
	}
	if c.MaxSuggestions != 10 {
		t.Errorf("maxSuggestions = %d, want 10", c.MaxSuggestions)
	}
	if c.Theme != "native" {
		t.Errorf("theme = %q, want native", c.Theme)
	}
	if c.BlacklistedApps == nil {
		t.Error("blacklistedApps should be non-nil after normalize")
	}
}

func TestTriggerRune(t *testing.T) {
	cases := map[string]rune{":": ':', ";": ';', "": ':', "/x": '/'}
	for in, want := range cases {
		c := &Config{Trigger: in}
		if got := c.TriggerRune(); got != want {
			t.Errorf("TriggerRune(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestIsBlacklisted(t *testing.T) {
	c := &Config{BlacklistedApps: []string{"com.apple.Terminal", "1Password"}}
	cases := map[string]bool{
		"com.apple.Terminal":          true,
		"COM.APPLE.TERMINAL":          true, // case-insensitive
		"/Applications/1Password.app": true, // substring
		"com.apple.Safari":            false,
		"":                            false,
	}
	for in, want := range cases {
		if got := c.IsBlacklisted(in); got != want {
			t.Errorf("IsBlacklisted(%q) = %v, want %v", in, got, want)
		}
	}
}
