// Package autostart registers (or unregisters) jify to launch automatically at
// login, using the native mechanism for each OS:
//
//   - macOS:   a LaunchAgent plist in ~/Library/LaunchAgents
//   - Windows: an HKCU ...\CurrentVersion\Run registry value
//   - Linux:   a .desktop file in ~/.config/autostart
package autostart

import "os"

const (
	// label is the reverse-DNS identifier used for the macOS LaunchAgent and as
	// the Windows registry value / Linux desktop file name.
	label   = "com.floatpane.jify"
	appName = "jify"
)

// exePath returns the absolute path to the running jify binary.
func exePath() (string, error) {
	p, err := os.Executable()
	if err != nil {
		return "", err
	}
	return p, nil
}

// Sync makes the OS autostart state match want, enabling or disabling as needed.
func Sync(want bool) error {
	on, err := IsEnabled()
	if err != nil {
		return err
	}
	switch {
	case want && !on:
		return Enable()
	case !want && on:
		return Disable()
	}
	return nil
}
