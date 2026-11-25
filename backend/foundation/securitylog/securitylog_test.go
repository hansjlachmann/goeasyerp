package securitylog

import (
	"strings"
	"sync"
	"testing"
	"time"
)

// capture starts the recorder with a writer that keeps what it is given
func capture(t *testing.T) func() []Event {
	t.Helper()
	var mu sync.Mutex
	var got []Event
	stop := Start(func(events []Event) error {
		mu.Lock()
		defer mu.Unlock()
		got = append(got, events...)
		return nil
	})
	t.Cleanup(stop)
	return func() []Event {
		Flush()
		mu.Lock()
		defer mu.Unlock()
		return append([]Event(nil), got...)
	}
}

// A flood of one kind from one user and address is written five times, then once more with the
// number held back when its minute is over; other users and addresses are not held back
func TestRecordThrottles(t *testing.T) {
	now := time.Date(2026, 11, 25, 12, 0, 0, 0, time.UTC)
	reset(func() time.Time { return now })
	t.Cleanup(Reset)
	events := capture(t)

	for i := 0; i < 12; i++ {
		Record(Event{Kind: LoginFailed, UserID: "admin", Address: "192.0.2.10", Details: "wrong password"})
	}
	Record(Event{Kind: LoginFailed, UserID: "ADMIN", Address: "192.0.2.11"})
	Record(Event{Kind: Login, UserID: "admin", Address: "192.0.2.10"})
	got := events()
	if len(got) != throttleLimit+2 {
		t.Fatalf("written %d events, want %d", len(got), throttleLimit+2)
	}

	now = now.Add(throttleWindow)
	got = events()
	if len(got) != throttleLimit+3 {
		t.Fatalf("after the minute %d events, want %d", len(got), throttleLimit+3)
	}
	more := got[len(got)-1]
	if more.Kind != LoginFailed || !strings.HasPrefix(more.Details, "7 more within a minute") {
		t.Errorf("the held-back event: %+v", more)
	}

	// the next minute starts a new window
	Record(Event{Kind: LoginFailed, UserID: "admin", Address: "192.0.2.10"})
	if got = events(); len(got) != throttleLimit+4 {
		t.Errorf("a new minute: %d events, want %d", len(got), throttleLimit+4)
	}
}

// Fields are cut to the table's lengths, and a full queue drops events instead of growing
func TestRecordLimits(t *testing.T) {
	reset(time.Now)
	t.Cleanup(Reset)
	Record(Event{Kind: AccessRefused, UserID: strings.Repeat("x", 80), Details: strings.Repeat("é", 300)})
	rec.mu.Lock()
	e := rec.queue[0]
	rec.mu.Unlock()
	if len([]rune(e.UserID)) != 50 || len([]rune(e.Details)) != 250 || e.Time.IsZero() {
		t.Errorf("not cut to size: %d/%d characters, time %v", len([]rune(e.UserID)), len([]rune(e.Details)), e.Time)
	}

	for i := 0; i < maxQueued+10; i++ {
		Record(Event{Kind: AccessRefused, UserID: "u", Address: "a" + strings.Repeat("1", i%60) + string(rune('a'+i%26)) + strings.Repeat("2", i/60%10)})
	}
	rec.mu.Lock()
	queued, dropped := len(rec.queue), rec.dropped
	rec.mu.Unlock()
	if queued != maxQueued || dropped == 0 {
		t.Errorf("queue %d (want %d), dropped %d", queued, maxQueued, dropped)
	}
}
