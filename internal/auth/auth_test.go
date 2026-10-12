package auth

import (
	"strings"
	"testing"
)

func TestValidUsernameLengthCountsCharacters(t *testing.T) {
	if !validUsernameLength(strings.Repeat("é", 25)) {
		t.Fatal("25 accented characters should be accepted (50 bytes)")
	}
	if validUsernameLength(strings.Repeat("é", 26)) {
		t.Fatal("26 characters should be rejected")
	}
	if validUsernameLength("ab") {
		t.Fatal("two characters should be rejected")
	}
	if !validUsernameLength("abc") {
		t.Fatal("three characters should be accepted")
	}
}
