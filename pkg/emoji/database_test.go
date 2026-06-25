package emoji

import "testing"

func TestSearchExactShortcodeRanksFirst(t *testing.T) {
	db := NewDatabase()
	got := db.Search("fire", 5)
	if len(got) == 0 {
		t.Fatal("expected results for 'fire'")
	}
	if got[0].Shortcode != "fire" || got[0].Char != "🔥" {
		t.Fatalf("expected 🔥 fire first, got %s %s", got[0].Char, got[0].Shortcode)
	}
}

func TestSearchKeywordMatch(t *testing.T) {
	db := NewDatabase()
	got := db.Search("lol", 10)
	found := false
	for _, e := range got {
		if e.Shortcode == "joy" || e.Shortcode == "rofl" {
			found = true
		}
	}
	if !found {
		t.Fatalf("expected a laughing emoji for keyword 'lol', got %v", got)
	}
}

func TestSearchEmptyReturnsNil(t *testing.T) {
	db := NewDatabase()
	if got := db.Search("   ", 5); got != nil {
		t.Fatalf("expected nil for empty query, got %v", got)
	}
}

func TestSearchRespectsLimit(t *testing.T) {
	db := NewDatabase()
	got := db.Search("a", 3)
	if len(got) > 3 {
		t.Fatalf("expected at most 3 results, got %d", len(got))
	}
}

func TestPrefixBeatsContains(t *testing.T) {
	db := NewDatabase()
	got := db.Search("heart", 10)
	if len(got) == 0 || got[0].Shortcode != "heart" {
		t.Fatalf("expected 'heart' to rank first, got %v", got)
	}
}
