package utils

import (
	"strings"
	"testing"
	"unicode/utf8"
)

func TestTruncateRunesNeverSplitsCharacters(t *testing.T) {
	// 24 ASCII bytes plus one two-byte character: byte-based slicing at 25 would split it.
	nick := strings.Repeat("a", 24) + "ç"
	got := TruncateRunes(nick, 25)
	if !utf8.ValidString(got) {
		t.Fatalf("TruncateRunes produced invalid UTF-8: %q", got)
	}
	if got != nick {
		t.Fatalf("25 characters fit exactly; got %q", got)
	}

	short := TruncateRunes(strings.Repeat("ç", 30), 10)
	if utf8.RuneCountInString(short) != 10 || !utf8.ValidString(short) {
		t.Fatalf("want 10 valid characters, got %q (%d runes)", short, utf8.RuneCountInString(short))
	}
}

func TestTruncateRunesEdgeCases(t *testing.T) {
	if got := TruncateRunes("abc", 0); got != "" {
		t.Fatalf("max 0 should be empty, got %q", got)
	}
	if got := TruncateRunes("abc", 10); got != "abc" {
		t.Fatalf("short input should be unchanged, got %q", got)
	}
	if got := TruncateRunes("", 5); got != "" {
		t.Fatalf("empty input should stay empty, got %q", got)
	}
}

func TestSanitizeNicknameCountsCharacters(t *testing.T) {
	got := SanitizeNickname("  " + strings.Repeat("ç", 30) + "  ")
	if !utf8.ValidString(got) || utf8.RuneCountInString(got) != 25 {
		t.Fatalf("want 25 valid characters, got %q (%d runes)", got, utf8.RuneCountInString(got))
	}
}
