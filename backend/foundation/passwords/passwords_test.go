package passwords

import "testing"

func TestCheck(t *testing.T) {
	cases := []struct {
		password, userID string
		want             Problem
	}{
		{"short", "HANS", TooShort},
		{"ninechars", "HANS", TooShort},
		{"blå-hav-søt", "HANS", OK}, // characters, not bytes
		{"Password123", "HANS", TooCommon},
		{"QWERTYUIOP", "HANS", TooCommon},
		{"zzzzzzzzzzzz", "HANS", TooCommon},
		{"hans-was-here-2026", "HANS", HasUserID},
		{"hans-was-here-2026", "AL", OK}, // a user ID that short is not checked
		{"correct horse battery", "HANS", OK},
		{"7f3c9a1e2b4d", "ADMIN", OK},
	}
	for _, c := range cases {
		if got := Check(c.password, c.userID); got != c.want {
			t.Errorf("Check(%q, %q) = %d, want %d", c.password, c.userID, got, c.want)
		}
	}
}

// Every listed password is long enough to need the list (shorter ones are refused anyway)
func TestCommonListEntriesAreLongEnough(t *testing.T) {
	if len(common) < 100 {
		t.Fatalf("only %d common passwords loaded", len(common))
	}
	for p := range common {
		if len([]rune(p)) < MinLength {
			t.Errorf("%q is shorter than %d characters: drop it from common.txt", p, MinLength)
		}
	}
}
