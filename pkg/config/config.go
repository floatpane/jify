// Package config loads and persists jify's user configuration.
package config

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
)

// Hotkey describes a global keyboard shortcut (reserved for future use, e.g.
// opening the picker manually rather than via the trigger character).
type Hotkey struct {
	Modifier string `json:"modifier"`
	Key      string `json:"key"`
}

// Config is the user-editable configuration for jify.
type Config struct {
	// Trigger is the character that starts a suggestion session (default ":").
	Trigger string `json:"trigger"`
	// MaxSuggestions caps how many emojis the popup shows at once.
	MaxSuggestions int `json:"maxSuggestions"`
	// BlacklistedApps is a list of application bundle identifiers (macOS) or
	// process names where jify stays disabled, e.g. "com.apple.Terminal".
	BlacklistedApps []string `json:"blacklistedApps"`
	// Theme controls the popup appearance: "native", "dark" or "light".
	Theme string `json:"theme"`
	// Hotkey is reserved for a future manual-open shortcut.
	Hotkey Hotkey `json:"hotkey"`
}

// Default returns a Config populated with sensible defaults.
func Default() *Config {
	return &Config{
		Trigger:         ":",
		MaxSuggestions:  10,
		BlacklistedApps: []string{},
		Theme:           "native",
		Hotkey:          Hotkey{Modifier: "ctrl", Key: "space"},
	}
}

// Path returns the location of the config file (~/.config/jify/config.json on
// most platforms).
func Path() (string, error) {
	dir, err := os.UserConfigDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "jify", "config.json"), nil
}

// Load reads the config from disk, creating it with defaults if it does not yet
// exist. Missing individual fields fall back to their default values.
func Load() (*Config, error) {
	path, err := Path()
	if err != nil {
		return nil, err
	}

	data, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		cfg := Default()
		if err := cfg.Save(); err != nil {
			return nil, err
		}
		return cfg, nil
	}
	if err != nil {
		return nil, err
	}

	cfg := Default()
	if err := json.Unmarshal(data, cfg); err != nil {
		return nil, err
	}
	cfg.normalize()
	return cfg, nil
}

// Save writes the config to disk, creating the parent directory if needed.
func (c *Config) Save() error {
	path, err := Path()
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	data, err := json.MarshalIndent(c, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0o644)
}

func (c *Config) normalize() {
	if c.Trigger == "" {
		c.Trigger = ":"
	}
	if c.MaxSuggestions <= 0 {
		c.MaxSuggestions = 10
	}
	if c.Theme == "" {
		c.Theme = "native"
	}
	if c.BlacklistedApps == nil {
		c.BlacklistedApps = []string{}
	}
}

// TriggerRune returns the first rune of the configured trigger string.
func (c *Config) TriggerRune() rune {
	for _, r := range c.Trigger {
		return r
	}
	return ':'
}

// IsBlacklisted reports whether the given application identifier is blacklisted.
// Matching is case-insensitive and also succeeds on substring matches so that
// either a full bundle id ("com.apple.Terminal") or a plain name ("Terminal")
// can be listed.
func (c *Config) IsBlacklisted(appID string) bool {
	if appID == "" {
		return false
	}
	appID = strings.ToLower(appID)
	for _, entry := range c.BlacklistedApps {
		entry = strings.ToLower(strings.TrimSpace(entry))
		if entry == "" {
			continue
		}
		if appID == entry || strings.Contains(appID, entry) {
			return true
		}
	}
	return false
}
