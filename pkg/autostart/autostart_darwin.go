//go:build darwin

package autostart

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
)

func plistPath() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, "Library", "LaunchAgents", label+".plist"), nil
}

const plistTemplate = `<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0">
<dict>
	<key>Label</key>
	<string>%s</string>
	<key>ProgramArguments</key>
	<array>
		<string>%s</string>
	</array>
	<key>RunAtLoad</key>
	<true/>
	<key>ProcessType</key>
	<string>Interactive</string>
</dict>
</plist>
`

// Enable writes the LaunchAgent plist and loads it so jify starts at login.
func Enable() error {
	exe, err := exePath()
	if err != nil {
		return err
	}
	path, err := plistPath()
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	content := fmt.Sprintf(plistTemplate, label, exe)
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		return err
	}
	if skipLaunchctl() {
		return nil
	}
	// Reload so the change takes effect immediately; ignore unload errors (it
	// may not be loaded yet).
	_ = exec.Command("launchctl", "unload", path).Run()
	return exec.Command("launchctl", "load", "-w", path).Run()
}

// skipLaunchctl lets tests register the plist without actually loading it into
// launchd (which would try to spawn the agent).
func skipLaunchctl() bool {
	return os.Getenv("JIFY_SKIP_LAUNCHCTL") == "1"
}

// Disable unloads and removes the LaunchAgent plist.
func Disable() error {
	path, err := plistPath()
	if err != nil {
		return err
	}
	if !skipLaunchctl() {
		_ = exec.Command("launchctl", "unload", "-w", path).Run()
	}
	if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
		return err
	}
	return nil
}

// IsEnabled reports whether the LaunchAgent plist exists.
func IsEnabled() (bool, error) {
	path, err := plistPath()
	if err != nil {
		return false, err
	}
	_, err = os.Stat(path)
	if os.IsNotExist(err) {
		return false, nil
	}
	return err == nil, err
}
