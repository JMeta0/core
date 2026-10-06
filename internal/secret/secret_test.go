package secret

import "testing"

func TestEqual(t *testing.T) {
	tests := []struct {
		a, b string
		want bool
	}{
		{"", "", true},
		{"abc", "abc", true},
		{"abc", "abd", false},
		{"abc", "abcd", false},
		{"abcd", "abc", false},
		{"\x00\x01\x02", "\x00\x01\x02", true},
	}

	for _, test := range tests {
		if got := Equal(test.a, test.b); got != test.want {
			t.Errorf("Equal(%q, %q) = %v, want %v", test.a, test.b, got, test.want)
		}
	}
}
