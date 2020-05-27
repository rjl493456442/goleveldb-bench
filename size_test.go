package bench

import (
	"testing"
)

func TestParseSize(t *testing.T) {
	tests := []struct {
		in string
		v  uint64
	}{
		{"82", 82},
		{"82b", 82},
		{"82kb", 82 * 1024},
		{"82mb", 82 * 1024 * 1024},
		{"82MB", 82 * 1024 * 1024},
		{"82gb", 82 * 1024 * 1024 * 1024},
		{"122GB", 122 * 1024 * 1024 * 1024},
	}
	for _, test := range tests {
		s, err := ParseSize(test.in)
		if err != nil {
			t.Errorf("%q: %v", test.in, err)
			continue
		}
		if s != test.v {
			t.Errorf("%q: got %d, want %d", test.in, s, test.v)
		}
	}
}
