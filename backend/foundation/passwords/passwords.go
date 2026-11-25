// Package passwords holds the rules a new password must meet (S38): at least MinLength
// characters, not one of the commonly used passwords, not containing the user ID.
package passwords

import (
	_ "embed"
	"strings"
	"unicode/utf8"
)

// MinLength is the shortest password accepted
const MinLength = 10

//go:embed common.txt
var commonList string

var common = func() map[string]bool {
	m := map[string]bool{}
	for _, line := range strings.Split(commonList, "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		m[strings.ToLower(line)] = true
	}
	return m
}()

// Problem is why a password is refused
type Problem int

const (
	OK Problem = iota
	TooShort
	TooCommon
	HasUserID
)

// Check reports whether password may be the password of userID
func Check(password, userID string) Problem {
	if utf8.RuneCountInString(password) < MinLength {
		return TooShort
	}
	lower := strings.ToLower(password)
	if common[lower] || oneCharacter(lower) {
		return TooCommon
	}
	id := strings.ToLower(strings.TrimSpace(userID))
	if utf8.RuneCountInString(id) >= 3 && strings.Contains(lower, id) {
		return HasUserID
	}
	return OK
}

// oneCharacter: the same character throughout (aaaaaaaaaa)
func oneCharacter(s string) bool {
	first, _ := utf8.DecodeRuneInString(s)
	return strings.Trim(s, string(first)) == ""
}
