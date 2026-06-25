//go:build !darwin && !linux && !windows

package autostart

import (
	"fmt"
	"runtime"
)

// Enable is unsupported on this platform.
func Enable() error { return fmt.Errorf("autostart not supported on %s", runtime.GOOS) }

// Disable is unsupported on this platform.
func Disable() error { return fmt.Errorf("autostart not supported on %s", runtime.GOOS) }

// IsEnabled always returns false on unsupported platforms.
func IsEnabled() (bool, error) { return false, nil }
