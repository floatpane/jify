// Package emoji provides a searchable database of shortcode emojis.
package emoji

import (
	_ "embed"
	"encoding/json"
	"sort"
	"strings"
)

// emojiJSON is the full shortcode emoji dataset, generated from GitHub's gemoji
// database by `go run ./tools/emojigen` (see scripts/gen-emoji.sh).
//
//go:embed emoji.json
var emojiJSON []byte

// emojiData parses the embedded dataset. The data is generated and validated, so
// a parse failure is a build-time programmer error.
func emojiData() []Emoji {
	var out []Emoji
	if err := json.Unmarshal(emojiJSON, &out); err != nil {
		panic("emoji: invalid embedded dataset: " + err.Error())
	}
	return out
}

// Emoji represents a single emoji together with its searchable metadata.
type Emoji struct {
	// Char is the actual emoji glyph, e.g. "😀".
	Char string `json:"char"`
	// Shortcode is the canonical name typed after the trigger, e.g. "grinning".
	Shortcode string `json:"shortcode"`
	// Keywords are additional terms that should match this emoji.
	Keywords []string `json:"keywords"`
	// Category groups related emojis (purely informational for now).
	Category string `json:"category"`
}

// Database holds all available emojis and supports fuzzy-ish search.
type Database struct {
	emojis []Emoji
}

// NewDatabase creates and populates the emoji database.
func NewDatabase() *Database {
	return &Database{emojis: emojiData()}
}

// Search finds emojis matching the query, ordered by relevance, returning at
// most limit results. An empty query returns nil.
func (db *Database) Search(query string, limit int) []Emoji {
	query = strings.ToLower(strings.TrimSpace(query))
	if query == "" {
		return nil
	}

	type scored struct {
		emoji Emoji
		score int
		order int
	}

	var matches []scored
	for i, e := range db.emojis {
		score := scoreEmoji(e, query)
		if score > 0 {
			matches = append(matches, scored{emoji: e, score: score, order: i})
		}
	}

	sort.SliceStable(matches, func(i, j int) bool {
		if matches[i].score != matches[j].score {
			return matches[i].score > matches[j].score
		}
		// Stable tie-break: shorter shortcodes first, then original order.
		li, lj := len(matches[i].emoji.Shortcode), len(matches[j].emoji.Shortcode)
		if li != lj {
			return li < lj
		}
		return matches[i].order < matches[j].order
	})

	if limit <= 0 || limit > len(matches) {
		limit = len(matches)
	}
	out := make([]Emoji, 0, limit)
	for i := 0; i < limit; i++ {
		out = append(out, matches[i].emoji)
	}
	return out
}

// scoreEmoji ranks how well an emoji matches the query. Higher is better; 0
// means no match.
func scoreEmoji(e Emoji, query string) int {
	sc := strings.ToLower(e.Shortcode)
	best := 0

	switch {
	case sc == query:
		best = 100
	case strings.HasPrefix(sc, query):
		best = 70
	case strings.Contains(sc, query):
		best = 40
	}

	for _, kw := range e.Keywords {
		kw = strings.ToLower(kw)
		switch {
		case kw == query && best < 60:
			best = 60
		case strings.HasPrefix(kw, query) && best < 35:
			best = 35
		case strings.Contains(kw, query) && best < 20:
			best = 20
		}
	}
	return best
}

// GetAll returns every emoji in the database.
func (db *Database) GetAll() []Emoji {
	return db.emojis
}
