package linkid

import (
	"regexp"
	"slices"
	"strings"
	"testing"
)

func TestNew(t *testing.T) {
	seen := map[string]bool{}
	for range 1000 {
		id := New()
		words := strings.Split(id, "-")
		if len(words) != 3 || !slices.Contains(adjectives, words[0]) ||
			!slices.Contains(nouns, words[1]) || !slices.Contains(verbs, words[2]) {
			t.Fatalf("New() = %q, want adjective-noun-verb", id)
		}
		seen[id] = true
	}
	if len(seen) < 990 {
		t.Errorf("1000 ids had only %d distinct values", len(seen))
	}
}

// TestWords keeps the lists easy to maintain and the ids easy to read.
func TestWords(t *testing.T) {
	valid := regexp.MustCompile(`^[a-z]{3,8}$`)
	all := map[string]string{}
	for name, words := range map[string][]string{"adjectives": adjectives, "nouns": nouns, "verbs": verbs} {
		if !slices.IsSorted(words) {
			t.Errorf("%s are not sorted", name)
		}
		for _, w := range words {
			if !valid.MatchString(w) {
				t.Errorf("%s: %q is not 3 to 8 lowercase letters", name, w)
			}
			if other, ok := all[w]; ok {
				t.Errorf("%q is in both %s and %s", w, other, name)
			}
			all[w] = name
		}
	}
	// Fewer ids would make collisions, and guessing links, more likely.
	if n := len(adjectives) * len(nouns) * len(verbs); n < 7_000_000 {
		t.Errorf("the lists give %d ids, want at least 7 million", n)
	}
}
