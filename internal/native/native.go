// Package native bridges jify's Go core (config + emoji search) to the
// platform-specific keyboard hook and popup UI.
package native

import (
	"github.com/floatpane/jify/pkg/config"
	"github.com/floatpane/jify/pkg/emoji"
)

// Shared state used by the platform layers via the exported cgo callbacks.
var (
	activeConfig *config.Config
	activeDB     *emoji.Database
)
