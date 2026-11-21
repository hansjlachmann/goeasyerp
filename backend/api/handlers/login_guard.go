package handlers

import (
	"strings"
	"sync"
	"time"

	"golang.org/x/crypto/bcrypt"
)

// Failed logins per user ID, so a password is not guessed from many addresses (the login rate
// limit counts per address): after loginMaxFailures failures within loginFailureWindow the user
// ID is refused for loginLockout, its correct password too; a successful login clears the
// count. Kept in memory per server; user IDs that do not exist count as well, so the answer
// does not tell which exist.
const (
	loginMaxFailures   = 10
	loginFailureWindow = 15 * time.Minute
	loginLockout       = 15 * time.Minute
	loginGuardMaxUsers = 10000 // user IDs tracked at most (typed IDs that do not exist count too)
)

type loginFailures struct {
	count       int
	first       time.Time // the first failure of the window
	lockedUntil time.Time
}

type loginGuard struct {
	mu       sync.Mutex
	failures map[string]*loginFailures
	now      func() time.Time
}

func newLoginGuard() *loginGuard {
	return &loginGuard{failures: map[string]*loginFailures{}, now: time.Now}
}

// logins is the server's login guard
var logins = newLoginGuard()

func loginKey(userID string) string { return strings.ToUpper(strings.TrimSpace(userID)) }

// locked reports whether the user ID may not log in now
func (g *loginGuard) locked(userID string) bool {
	g.mu.Lock()
	defer g.mu.Unlock()
	f := g.failures[loginKey(userID)]
	return f != nil && g.now().Before(f.lockedUntil)
}

// failed counts a failed login of the user ID
func (g *loginGuard) failed(userID string) {
	g.mu.Lock()
	defer g.mu.Unlock()
	key, now := loginKey(userID), g.now()
	f := g.failures[key]
	if f == nil || now.Sub(f.first) > loginFailureWindow && !now.Before(f.lockedUntil) {
		if f == nil && len(g.failures) >= loginGuardMaxUsers {
			g.prune(now)
			if len(g.failures) >= loginGuardMaxUsers {
				return
			}
		}
		f = &loginFailures{first: now}
		g.failures[key] = f
	}
	f.count++
	if f.count >= loginMaxFailures {
		f.lockedUntil = now.Add(loginLockout)
	}
}

// succeeded clears the user ID's failures
func (g *loginGuard) succeeded(userID string) {
	g.mu.Lock()
	defer g.mu.Unlock()
	delete(g.failures, loginKey(userID))
}

// prune drops the entries whose window and lockout are over
func (g *loginGuard) prune(now time.Time) {
	for key, f := range g.failures {
		if now.Sub(f.first) > loginFailureWindow && !now.Before(f.lockedUntil) {
			delete(g.failures, key)
		}
	}
}

// dummyHash is a bcrypt hash of a random password, checked for user IDs that do not exist
var dummyHash = sync.OnceValue(func() []byte {
	hash, err := bcrypt.GenerateFromPassword([]byte(time.Now().String()), bcrypt.DefaultCost)
	if err != nil {
		return nil
	}
	return hash
})

// checkDummyPassword takes as long as checking a real password (the result is always no)
func checkDummyPassword(password string) {
	_ = bcrypt.CompareHashAndPassword(dummyHash(), []byte(password))
}
