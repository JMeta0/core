package rand

import (
	"strings"
	"testing"
)

func TestStringWithCharset(t *testing.T) {
	s := String(32)

	if len(s) != 32 {
		t.Fatalf("expected a string of length 32, got %d", len(s))
	}

	for _, c := range s {
		if !strings.ContainsRune(CharsetAll, c) {
			t.Fatalf("unexpected character %q in %q", c, s)
		}
	}

	if String(0) != "" {
		t.Fatalf("expected an empty string for length 0")
	}

	if StringWithCharset(8, "") != "" {
		t.Fatalf("expected an empty string for an empty charset")
	}

	if s == String(32) {
		t.Fatalf("two subsequent random strings are identical")
	}

	s = StringAlphanumeric(18)
	if len(s) != 18 {
		t.Fatalf("expected a string of length 18, got %d", len(s))
	}

	for _, c := range s {
		if !strings.ContainsRune(CharsetAlphanumeric, c) {
			t.Fatalf("unexpected character %q in %q", c, s)
		}
	}
}
