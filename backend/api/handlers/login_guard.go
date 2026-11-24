package handlers

import (
	"strings"
	"sync"
	"time"

	"golang.org/x/crypto/bcrypt"
)

// Failed logins, so a password is not guessed — and so nobody can lock another user out (S20,
// S32). Three parts, kept in memory per server:
//   - per user ID and address: loginMaxFailures failures within loginFailureWindow lock that
//     address for that user ID for loginLockout (its correct password too) — whoever guesses
//     locks out only themselves;
//   - per user ID, from every address: after loginBrakeAfter failures each further attempt waits
//     a delay that doubles with every failure (1 s, 2 s, 4 s … loginBrakeMax); an attempt before
//     it is refused without a password check — guessing from many addresses becomes slow, and
//     nobody is shut out for long;
//   - an address the user ID logged in from before (within loginKnownAddressTTL) is not braked:
//     the user's own office keeps working while someone elsewhere guesses at the user ID.
//
// A successful login clears the address's failures and the user ID's brake. User IDs that do
// not exist count as well, so the answer does not tell which exist.
const (
	loginMaxFailures      = 10
	loginFailureWindow    = 15 * time.Minute
	loginLockout          = 15 * time.Minute
	loginBrakeAfter       = 10
	loginBrakeMax         = 60 * time.Second
	loginKnownAddressTTL  = 90 * 24 * time.Hour
	loginKnownPerUser     = 20    // known addresses kept per user ID
	loginGuardMaxEntries  = 10000 // entries per map at most (typed IDs and addresses count too)
	loginBrakeQuietPeriod = loginFailureWindow
)

// loginFailures: the failures of a user ID from one address
type loginFailures struct {
	count       int
	first       time.Time // the first failure of the window
	lockedUntil time.Time
}

// loginBrake: the failures of a user ID from every address
type loginBrake struct {
	count     int
	last      time.Time // the last failure
	notBefore time.Time // no attempt before
}

type loginGuard struct {
	mu        sync.Mutex
	failures  map[string]*loginFailures       // user ID + address
	brakes    map[string]*loginBrake          // user ID
	addresses map[string]map[string]time.Time // user ID → address → last successful login
	now       func() time.Time
}

func newLoginGuard() *loginGuard {
	return &loginGuard{
		failures:  map[string]*loginFailures{},
		brakes:    map[string]*loginBrake{},
		addresses: map[string]map[string]time.Time{},
		now:       time.Now,
	}
}

// logins is the server's login guard
var logins = newLoginGuard()

func loginKey(userID string) string { return strings.ToUpper(strings.TrimSpace(userID)) }

func loginAddressKey(userID, address string) string {
	return loginKey(userID) + "\x00" + address
}

// locked reports whether the user ID may not log in from the address now
func (g *loginGuard) locked(userID, address string) bool {
	g.mu.Lock()
	defer g.mu.Unlock()
	now := g.now()
	f := g.failures[loginAddressKey(userID, address)]
	if f != nil && now.Before(f.lockedUntil) {
		return true
	}
	if g.knownAddress(userID, address, now) {
		return false
	}
	b := g.brakes[loginKey(userID)]
	return b != nil && now.Before(b.notBefore)
}

// knownAddress: the user ID logged in from the address within loginKnownAddressTTL
func (g *loginGuard) knownAddress(userID, address string, now time.Time) bool {
	last, ok := g.addresses[loginKey(userID)][address]
	return ok && now.Sub(last) < loginKnownAddressTTL
}

// failed counts a failed login of the user ID from the address
func (g *loginGuard) failed(userID, address string) {
	g.mu.Lock()
	defer g.mu.Unlock()
	now := g.now()

	key := loginAddressKey(userID, address)
	f := g.failures[key]
	if f == nil || now.Sub(f.first) > loginFailureWindow && !now.Before(f.lockedUntil) {
		if f == nil && !g.room(len(g.failures), now) {
			return
		}
		f = &loginFailures{first: now}
		g.failures[key] = f
	}
	f.count++
	if f.count >= loginMaxFailures {
		f.lockedUntil = now.Add(loginLockout)
	}

	user := loginKey(userID)
	b := g.brakes[user]
	if b == nil || now.Sub(b.last) > loginBrakeQuietPeriod {
		if b == nil && !g.room(len(g.brakes), now) {
			return
		}
		b = &loginBrake{}
		g.brakes[user] = b
	}
	b.count++
	b.last = now
	if b.count > loginBrakeAfter {
		b.notBefore = now.Add(brakeDelay(b.count - loginBrakeAfter))
	}
}

// brakeDelay: the wait after the n-th failure beyond loginBrakeAfter (1 s, 2 s, 4 s …,
// loginBrakeMax at most)
func brakeDelay(n int) time.Duration {
	delay := time.Second
	for i := 1; i < n && delay < loginBrakeMax; i++ {
		delay *= 2
	}
	return min(delay, loginBrakeMax)
}

// succeeded clears the address's failures and the user ID's brake, and remembers the address
func (g *loginGuard) succeeded(userID, address string) {
	g.mu.Lock()
	defer g.mu.Unlock()
	now := g.now()
	user := loginKey(userID)
	delete(g.failures, loginAddressKey(userID, address))
	delete(g.brakes, user)

	known := g.addresses[user]
	if known == nil {
		if !g.room(len(g.addresses), now) {
			return
		}
		known = map[string]time.Time{}
		g.addresses[user] = known
	}
	if _, ok := known[address]; !ok && len(known) >= loginKnownPerUser {
		oldest := ""
		for a, t := range known {
			if oldest == "" || t.Before(known[oldest]) {
				oldest = a
			}
		}
		delete(known, oldest)
	}
	known[address] = now
}

// room: whether a map holding n entries may take another one, after dropping what is over
func (g *loginGuard) room(n int, now time.Time) bool {
	if n < loginGuardMaxEntries {
		return true
	}
	g.prune(now)
	return len(g.failures) < loginGuardMaxEntries && len(g.brakes) < loginGuardMaxEntries &&
		len(g.addresses) < loginGuardMaxEntries
}

// prune drops the entries whose window, lockout or brake is over and the addresses not used
// for loginKnownAddressTTL
func (g *loginGuard) prune(now time.Time) {
	for key, f := range g.failures {
		if now.Sub(f.first) > loginFailureWindow && !now.Before(f.lockedUntil) {
			delete(g.failures, key)
		}
	}
	for key, b := range g.brakes {
		if now.Sub(b.last) > loginBrakeQuietPeriod && !now.Before(b.notBefore) {
			delete(g.brakes, key)
		}
	}
	for user, known := range g.addresses {
		for address, last := range known {
			if now.Sub(last) >= loginKnownAddressTTL {
				delete(known, address)
			}
		}
		if len(known) == 0 {
			delete(g.addresses, user)
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
