package handlers

import (
	"fmt"
	"testing"
	"time"
)

// Failed logins lock the address they come from for that user ID only (S20, S32)
func TestLoginGuardLocksTheAddress(t *testing.T) {
	now := time.Date(2026, 11, 21, 12, 0, 0, 0, time.UTC)
	g := newLoginGuard()
	g.now = func() time.Time { return now }
	const attacker, office = "203.0.113.7", "198.51.100.20"

	for i := 1; i < loginMaxFailures; i++ {
		g.failed("kari", attacker)
	}
	if g.locked("KARI", attacker) {
		t.Fatalf("locked after %d failures", loginMaxFailures-1)
	}
	g.failed(" Kari ", attacker)
	if !g.locked("kari", attacker) {
		t.Fatalf("the address is not locked after %d failures (user ID in any case)", loginMaxFailures)
	}
	if g.locked("OLA", attacker) {
		t.Error("another user ID is locked from that address")
	}
	// the brake waits loginBrakeAfter failures; a lock from one address does not reach another
	if g.locked("KARI", office) {
		t.Error("the user is locked from another address too")
	}
	now = now.Add(loginLockout + time.Second)
	if g.locked("KARI", attacker) {
		t.Error("still locked after the lockout")
	}

	// failures spread over more than the window do not lock
	g.succeeded("KARI", attacker)
	for i := 0; i < 2*loginMaxFailures; i++ {
		g.failed("KARI", attacker)
		now = now.Add(loginFailureWindow / 5)
	}
	if g.locked("KARI", attacker) {
		t.Error("locked by failures spread over several windows")
	}

	// a successful login clears the count
	for i := 1; i < loginMaxFailures; i++ {
		g.failed("OLA", office)
	}
	g.succeeded("OLA", office)
	g.failed("OLA", office)
	if g.locked("OLA", office) {
		t.Error("the failures before a successful login still counted")
	}
}

// Guessing from many addresses is braked per user ID: each failure beyond loginBrakeAfter
// doubles the wait, capped — and the user's own known address is not braked
func TestLoginGuardBrakesTheUserFromEverywhere(t *testing.T) {
	now := time.Date(2026, 11, 21, 12, 0, 0, 0, time.UTC)
	g := newLoginGuard()
	g.now = func() time.Time { return now }
	const office = "198.51.100.20"
	g.succeeded("ADMIN", office) // the administrator logged in from the office before

	// loginBrakeAfter failures, each from a new address: no brake yet
	for i := 0; i < loginBrakeAfter; i++ {
		g.failed("ADMIN", fmt.Sprintf("203.0.113.%d", i))
	}
	if g.locked("ADMIN", "192.0.2.1") {
		t.Fatal("braked before loginBrakeAfter failures")
	}

	// the next failure brakes every unknown address for 1 s, then 2 s, 4 s …
	g.failed("ADMIN", "192.0.2.1")
	if !g.locked("ADMIN", "192.0.2.2") {
		t.Error("an unknown address is not braked after the failures")
	}
	if g.locked("ADMIN", office) {
		t.Error("the administrator's own office is braked by others' failures")
	}
	now = now.Add(time.Second)
	if g.locked("ADMIN", "192.0.2.2") {
		t.Error("still braked after the first delay (1 s)")
	}
	g.failed("ADMIN", "192.0.2.2")
	now = now.Add(time.Second)
	if !g.locked("ADMIN", "192.0.2.3") {
		t.Error("the second delay is not longer (2 s)")
	}
	now = now.Add(time.Second)
	if g.locked("ADMIN", "192.0.2.3") {
		t.Error("still braked after the second delay")
	}

	// the delay doubles up to loginBrakeMax
	for n, want := range map[int]time.Duration{1: time.Second, 2: 2 * time.Second, 3: 4 * time.Second, 6: 32 * time.Second, 7: loginBrakeMax, 40: loginBrakeMax} {
		if got := brakeDelay(n); got != want {
			t.Errorf("brakeDelay(%d) = %v, want %v", n, got, want)
		}
	}

	// the right password from the office clears the brake
	g.succeeded("ADMIN", office)
	if g.locked("ADMIN", "192.0.2.9") {
		t.Error("the brake stayed after a successful login")
	}

	// the brake ends once the guessing stops for loginBrakeQuietPeriod
	for i := 0; i <= loginBrakeAfter; i++ {
		g.failed("KARI", fmt.Sprintf("203.0.113.%d", i))
	}
	now = now.Add(loginBrakeQuietPeriod + time.Minute)
	g.failed("KARI", "192.0.2.50")
	if g.locked("KARI", "192.0.2.51") {
		t.Error("an old run of failures still brakes after the quiet period")
	}
}

// The guard keeps bounded memory: entries past their time are dropped to make room, and a
// user ID remembers at most loginKnownPerUser addresses
func TestLoginGuardStaysBounded(t *testing.T) {
	now := time.Date(2026, 11, 21, 12, 0, 0, 0, time.UTC)
	g := newLoginGuard()
	g.now = func() time.Time { return now }
	for i := 0; i < loginKnownPerUser+5; i++ {
		g.succeeded("KARI", fmt.Sprintf("198.51.100.%d", i))
		now = now.Add(time.Minute)
	}
	if n := len(g.addresses["KARI"]); n != loginKnownPerUser {
		t.Errorf("%d known addresses, want %d", n, loginKnownPerUser)
	}
	if _, ok := g.addresses["KARI"]["198.51.100.0"]; ok {
		t.Error("the oldest address was not the one dropped")
	}
	g.failed("OLA", "203.0.113.1")
	now = now.Add(loginKnownAddressTTL)
	g.prune(now)
	if len(g.failures) != 0 || len(g.brakes) != 0 || len(g.addresses) != 0 {
		t.Errorf("prune left %d failures, %d brakes, %d users with addresses", len(g.failures), len(g.brakes), len(g.addresses))
	}
}
