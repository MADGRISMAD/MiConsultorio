package api

import (
	"strings"
	"testing"
	"unicode/utf8"
)

// A value cut for the AI answers must never be longer than the limit the screens and validators enforce when it is saved.
func TestTruncateRunesNeverExceedsTheLimit(t *testing.T) {
	long := strings.Repeat("ñandú ", 100)
	for _, n := range []int{0, 1, 2, 30, 60, 120, 200} {
		if got := truncateRunes(long, n); utf8.RuneCountInString(got) > n {
			t.Fatalf("n=%d got %d runes", n, utf8.RuneCountInString(got))
		}
	}
	if got := truncateRunes("corto", 10); got != "corto" {
		t.Fatalf("short text is kept: %q", got)
	}
	if got := truncateRunes(long, 20); !strings.HasSuffix(got, "…") {
		t.Fatalf("a cut text says so: %q", got)
	}
}
