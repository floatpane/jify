//go:build !darwin && !linux && !windows

package native

import (
	"fmt"
	"runtime"

	"github.com/floatpane/jify/pkg/config"
	"github.com/floatpane/jify/pkg/emoji"
)

// Run is not yet implemented on this platform. The macOS implementation lives
// in darwin.go / darwin.m; Linux and Windows backends can be added behind their
// own build tags following the same exported-callback pattern.
func Run(cfg *config.Config, db *emoji.Database) error {
	activeConfig = cfg
	activeDB = db
	return fmt.Errorf("jify: the native popup is not yet supported on %s", runtime.GOOS)
}
