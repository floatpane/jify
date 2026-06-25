//go:build darwin || linux

package native

/*
#include <stdlib.h>
*/
import "C"

import "strings"

// This file holds the cgo callbacks shared by the macOS (Objective-C) and
// Linux (GTK/X11) native layers. The C side calls them via "_cgo_export.h".

// jifyQuery is called from C as the user types. It returns the matching emojis
// as a newline-separated list of "glyph\tshortcode" rows. The caller (C side)
// must free the returned string. An empty result returns an empty string.
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

// jifyIsBlacklisted reports (as 1/0) whether the given application identifier is
// on the blacklist.
//
//export jifyIsBlacklisted
func jifyIsBlacklisted(cb *C.char) C.int {
	if activeConfig.IsBlacklisted(C.GoString(cb)) {
		return 1
	}
	return 0
}

// jifyTriggerRune returns the configured trigger character as a Unicode code
// point for the native layer to match against.
//
//export jifyTriggerRune
func jifyTriggerRune() C.int {
	return C.int(activeConfig.TriggerRune())
}
