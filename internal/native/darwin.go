//go:build darwin

package native

/*
#cgo CFLAGS: -x objective-c -fobjc-arc
#cgo LDFLAGS: -framework Cocoa -framework Carbon -framework ApplicationServices
#include <stdlib.h>
#include "darwin.h"
*/
import "C"

import (
	_ "embed"
	"unsafe"

	"github.com/floatpane/jify/pkg/config"
	"github.com/floatpane/jify/pkg/emoji"
)

//go:embed jify_icon.png
var iconPNG []byte

// Run starts the macOS event tap and popup, blocking on the AppKit run loop.
// It must be called from the main goroutine (see main.go's runtime.LockOSThread).
func Run(cfg *config.Config, db *emoji.Database) error {
	activeConfig = cfg
	activeDB = db
	if len(iconPNG) > 0 {
		C.jifySetAppIcon(unsafe.Pointer(&iconPNG[0]), C.int(len(iconPNG)))
	}
	C.jifyRun()
	return nil
}
