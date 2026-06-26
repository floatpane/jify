//go:build linux && cgo

package native

/*
#cgo pkg-config: gtk+-3.0 x11 xtst libevdev xkbcommon gtk-layer-shell-0
#include <stdlib.h>
#include "linux.h"

// Forward declarations for both backends (only one will be called at runtime)
void jifyRunX11(void);
void jifyRunWayland(void);

// jifySetIcon is defined in linux_x11.c and shared by both backends
*/
import "C"

import (
	_ "embed"
	"os"
	"strings"
	"unsafe"

	"github.com/floatpane/jify/pkg/config"
	"github.com/floatpane/jify/pkg/emoji"
)

//go:embed jify_icon.png
var iconPNG []byte

// isWaylandSession reports whether we're running under a Wayland compositor.
func isWaylandSession() bool {
	sessionType := os.Getenv("XDG_SESSION_TYPE")
	waylandDisplay := os.Getenv("WAYLAND_DISPLAY")
	return strings.EqualFold(sessionType, "wayland") || waylandDisplay != ""
}

// Run starts the Linux backend, choosing between X11 (GTK + XRecord) and
// Wayland (GTK + libevdev/uinput) based on the current session type.
//
// Selection uses the closing-trigger style: type the trigger, a name, then the
// trigger again (e.g. ":smile:") to insert the top suggestion. The popup updates
// live as you type. This avoids needing to consume keystrokes, which X11's
// XRecord cannot do.
//
// X11: global key capture via XRecord, text injection via XTest.
//
// Wayland/Hyprland: global key capture reads /dev/input/event* via libevdev
// (requires the user to be in the 'input' group) and translates keycodes with
// xkbcommon; text injection uses `wtype` (the virtual-keyboard protocol). The
// popup is drawn through XWayland so it can be positioned at the cursor.
func Run(cfg *config.Config, db *emoji.Database) error {
	activeConfig = cfg
	activeDB = db
	if len(iconPNG) > 0 {
		C.jifySetIcon(unsafe.Pointer(&iconPNG[0]), C.int(len(iconPNG)))
	}
	if isWaylandSession() {
		C.jifyRunWayland()
	} else {
		C.jifyRunX11()
	}
	return nil
}
