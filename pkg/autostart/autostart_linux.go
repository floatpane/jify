//go:build linux

package autostart

import (
	"fmt"
	"os"
	"path/filepath"
)

func desktopPath() (string, error) {
	dir, err := os.UserConfigDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "autostart", appName+".desktop"), nil
}

const desktopTemplate = `[Desktop Entry]
Type=Application
Name=jify
Comment=Native shortcode emoji picker
Exec=%s
Terminal=false
X-GNOME-Autostart-enabled=true
`

// Enable writes a freedesktop autostart .desktop entry.
func Enable() error {
	exe, err := exePath()
	if err != nil {
		return err
	}
	path, err := desktopPath()
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	content := fmt.Sprintf(desktopTemplate, exe)
	return os.WriteFile(path, []byte(content), 0o644)
}

// Disable removes the autostart .desktop entry.
func Disable() error {
	path, err := desktopPath()
	if err != nil {
		return err
	}
	if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
		return err
	}
	return nil
}

// IsEnabled reports whether the autostart .desktop entry exists.
func IsEnabled() (bool, error) {
	path, err := desktopPath()
	if err != nil {
		return false, err
	}
	_, err = os.Stat(path)
	if os.IsNotExist(err) {
		return false, nil
	}
	return err == nil, err
}
