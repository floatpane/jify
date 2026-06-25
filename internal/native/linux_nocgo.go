//go:build linux && !cgo

package native

import (
	"fmt"

	"github.com/floatpane/jify/pkg/config"
	"github.com/floatpane/jify/pkg/emoji"
)

// Run requires cgo on Linux (the backend uses GTK3 and X11). Build with
// CGO_ENABLED=1 and the gtk+-3.0 / libx11 / libxtst development packages
// installed.
func Run(cfg *config.Config, db *emoji.Database) error {
	activeConfig = cfg
	activeDB = db
	return fmt.Errorf("jify: the Linux backend requires cgo; rebuild with CGO_ENABLED=1")
}
