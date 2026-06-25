//go:build linux

package native

/*
#cgo pkg-config: gtk+-3.0 x11 xtst
#include <stdlib.h>
#include "linux.h"
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

// Run starts the Linux (GTK + X11) backend, blocking on the GTK main loop.
//
// Selection uses the closing-trigger style: type the trigger, a name, then the
// trigger again (e.g. ":smile:") to insert the top suggestion. The popup updates
// live as you type. This avoids needing to consume keystrokes, which X11's
// XRecord cannot do. Requires X11 (works under XWayland for most apps).
func Run(cfg *config.Config, db *emoji.Database) error {
	activeConfig = cfg
	activeDB = db
	if len(iconPNG) > 0 {
		C.jifySetIcon(unsafe.Pointer(&iconPNG[0]), C.int(len(iconPNG)))
	}
	C.jifyRun()
	return nil
}
