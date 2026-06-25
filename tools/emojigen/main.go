// Command emojigen converts GitHub's gemoji database (db/emoji.json) into
// jify's emoji dataset (pkg/emoji/emoji.json).
//
// One entry is produced per shortcode alias so every alias is a first-class,
// searchable result. Tags, the description, and sibling aliases become keywords.
//
// Usage: go run ./tools/emojigen <gemoji.json> <out.json>
package main

import (
	"encoding/json"
	"fmt"
	"os"
	"sort"
	"strings"
)

type gemoji struct {
	Emoji       string   `json:"emoji"`
	Description string   `json:"description"`
	Category    string   `json:"category"`
	Aliases     []string `json:"aliases"`
	Tags        []string `json:"tags"`
}

type emoji struct {
	Char      string   `json:"char"`
	Shortcode string   `json:"shortcode"`
	Keywords  []string `json:"keywords,omitempty"`
	Category  string   `json:"category,omitempty"`
}

func main() {
	if len(os.Args) != 3 {
		fmt.Fprintln(os.Stderr, "usage: emojigen <gemoji.json> <out.json>")
		os.Exit(2)
	}

	raw, err := os.ReadFile(os.Args[1])
	if err != nil {
		fmt.Fprintln(os.Stderr, "read:", err)
		os.Exit(1)
	}
	var src []gemoji
	if err := json.Unmarshal(raw, &src); err != nil {
		fmt.Fprintln(os.Stderr, "parse:", err)
		os.Exit(1)
	}

	var out []emoji
	for _, g := range src {
		if g.Emoji == "" || len(g.Aliases) == 0 {
			continue
		}
		for i, alias := range g.Aliases {
			out = append(out, emoji{
				Char:      g.Emoji,
				Shortcode: alias,
				Keywords:  keywords(g, i),
				Category:  g.Category,
			})
		}
	}

	data, err := json.Marshal(out)
	if err != nil {
		fmt.Fprintln(os.Stderr, "marshal:", err)
		os.Exit(1)
	}
	if err := os.WriteFile(os.Args[2], append(data, '\n'), 0o644); err != nil {
		fmt.Fprintln(os.Stderr, "write:", err)
		os.Exit(1)
	}
	fmt.Printf("wrote %d emoji entries from %d glyphs\n", len(out), len(src))
}

// keywords builds the searchable keyword set: tags, sibling aliases, and the
// individual words of the description, de-duplicated.
func keywords(g gemoji, selfAlias int) []string {
	seen := map[string]bool{}
	var kw []string
	add := func(s string) {
		s = strings.ToLower(strings.TrimSpace(s))
		if s == "" || seen[s] {
			return
		}
		seen[s] = true
		kw = append(kw, s)
	}

	for _, t := range g.Tags {
		add(t)
	}
	for j, a := range g.Aliases {
		if j != selfAlias {
			add(a)
		}
	}
	for _, w := range strings.Fields(g.Description) {
		add(w)
	}
	sort.Strings(kw)
	return kw
}
