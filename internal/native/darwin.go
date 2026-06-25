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
	"strings"

	"github.com/floatpane/jify/pkg/config"
	"github.com/floatpane/jify/pkg/emoji"
)

// Run starts the macOS event tap and popup, blocking on the AppKit run loop.
// It must be called from the main goroutine (see main.go's runtime.LockOSThread).
func Run(cfg *config.Config, db *emoji.Database) error {
	activeConfig = cfg
	activeDB = db
	C.jifyRun()
	return nil
}

// jifyQuery is called from Objective-C as the user types. It returns the matching
// emojis as a newline-separated list of "glyph\tshortcode" rows. The caller
// (C side) must free the returned string. An empty result returns an empty
// string.
//
//export jifyQuery
func jifyQuery(cq *C.char) *C.char {
	query := C.GoString(cq)
	results := activeDB.Search(query, activeConfig.MaxSuggestions)

	var b strings.Builder
	for i, e := range results {
		if i > 0 {
			b.WriteByte('\n')
		}
		b.WriteString(e.Char)
		b.WriteByte('\t')
		b.WriteString(e.Shortcode)
	}
	return C.CString(b.String())
}

// jifyIsBlacklisted reports (as 1/0) whether the frontmost application's bundle
// identifier is on the blacklist.
//
//export jifyIsBlacklisted
func jifyIsBlacklisted(cb *C.char) C.int {
	bundleID := C.GoString(cb)
	if activeConfig.IsBlacklisted(bundleID) {
		return 1
	}
	return 0
}

// jifyTriggerRune returns the configured trigger character as a Unicode code
// point for the event tap to match against.
//
//export jifyTriggerRune
func jifyTriggerRune() C.int {
	return C.int(activeConfig.TriggerRune())
}
