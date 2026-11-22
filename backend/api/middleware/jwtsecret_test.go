package middleware

import (
	"strings"
	"testing"
)

func TestCheckJWTSecret(t *testing.T) {
	long := strings.Repeat("a1", 32)
	cases := []struct {
		secret  string
		server  bool
		refused bool
		warned  bool
	}{
		{"", false, false, false}, // development: a random key
		{"", true, true, false},
		{"change-me-in-production", false, true, false},
		{"replace-this-with-a-long-random-text", true, true, false},
		{"short", true, true, false},
		{"0123456789abcdef0123", true, false, true},
		{long, true, false, false},
	}
	for _, tc := range cases {
		warning, err := checkJWTSecret(tc.secret, tc.server)
		if (err != nil) != tc.refused {
			t.Errorf("%q server=%v: refused %v, want %v (%v)", tc.secret, tc.server, err != nil, tc.refused, err)
		}
		if (warning != "") != tc.warned {
			t.Errorf("%q server=%v: warning %q", tc.secret, tc.server, warning)
		}
	}
}
