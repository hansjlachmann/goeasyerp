package handlers

import (
	"testing"
	"time"
)

// Failed logins lock a user ID for a while, whichever address they come from (S20)
func TestLoginGuard(t *testing.T) {
	now := time.Date(2026, 11, 21, 12, 0, 0, 0, time.UTC)
	g := newLoginGuard()
	g.now = func() time.Time { return now }

	for i := 1; i < loginMaxFailures; i++ {
		g.failed("kari")
	}
	if g.locked("KARI") {
		t.Fatalf("locked after %d failures", loginMaxFailures-1)
	}
	g.failed(" Kari ")
	if !g.locked("kari") {
		t.Fatalf("not locked after %d failures (user ID in any case)", loginMaxFailures)
	}
	if g.locked("OLA") {
		t.Error("another user ID is locked")
	}
	now = now.Add(loginLockout + time.Second)
	if g.locked("KARI") {
		t.Error("still locked after the lockout")
	}

	// failures spread over more than the window do not lock
	g.succeeded("KARI")
	for i := 0; i < 2*loginMaxFailures; i++ {
		g.failed("KARI")
		now = now.Add(loginFailureWindow / 5)
	}
	if g.locked("KARI") {
		t.Error("locked by failures spread over several windows")
	}

	// a successful login clears the count
	for i := 1; i < loginMaxFailures; i++ {
		g.failed("OLA")
	}
	g.succeeded("OLA")
	g.failed("OLA")
	if g.locked("OLA") {
		t.Error("the failures before a successful login still counted")
	}
}
