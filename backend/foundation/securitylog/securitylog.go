// Package securitylog records security events (S48): logins and failed logins, lockouts, log
// outs, refused requests, password changes, the first user and the actions only SUPER users may
// take. Record never blocks the request and never fails it: events wait in a queue that a
// background writer (Start) puts into the Security Event table in batches. A flood of one kind —
// a password-guessing run, a script probing what it may do — is throttled: at most
// throttleLimit events per kind, user and address within throttleWindow, then one event saying
// how many more there were, so the log cannot be used to fill the database.
package securitylog

import (
	"fmt"
	"log"
	"strings"
	"sync"
	"time"
)

// Kind is what happened; the numbers are the options of the table's Event field (Security
// Event 3), in the same order
type Kind int

const (
	Login                Kind = iota // a user logged in
	LoginFailed                      // wrong user ID or password, or the user is not active
	LoginLocked                      // too many failed logins: the address is locked for the user ID
	LoginRefused                     // a login while the address is locked or braked
	Logout                           // a user logged out
	CompanyRefused                   // a login or company switch to a company the user has no access to
	AccessRefused                    // a request refused for want of permission (403)
	CrossSiteRefused                 // a change sent from another web site (S26)
	PasswordChanged                  // users changed their own password
	PasswordChangeFailed             // a password change with a wrong current password
	FirstUser                        // the first user was made with the setup token
	SetupTokenRefused                // a first-user request with a wrong setup token
	Administration                   // an action only SUPER users may take (apps, companies)
)

// kindNames are the option values of the table (and the log lines)
var kindNames = []string{
	"Login", "Login Failed", "Login Locked", "Login Refused", "Logout", "Company Refused",
	"Access Refused", "Cross-Site Refused", "Password Changed", "Password Change Failed",
	"First User", "Setup Token Refused", "Administration",
}

// Names are the kinds' names in option order (a test checks them against the table)
func Names() []string { return append([]string(nil), kindNames...) }

func (k Kind) String() string {
	if int(k) >= 0 && int(k) < len(kindNames) {
		return kindNames[k]
	}
	return fmt.Sprintf("Kind(%d)", int(k))
}

// Event is one security event
type Event struct {
	Time    time.Time
	Kind    Kind
	UserID  string // the user, or the user ID typed at a failed login
	Address string // the client's address (blank: not from a request)
	Company string
	Details string
}

// Writer stores a batch of events (the Security Event table, set by the API server)
type Writer func(events []Event) error

const (
	throttleWindow = time.Minute
	throttleLimit  = 5
	maxQueued      = 1000  // events waiting to be written; more are dropped (and counted)
	maxWindows     = 10000 // throttle windows kept at once
	flushInterval  = time.Second
)

type window struct {
	start      time.Time
	count      int
	suppressed int
	last       Event
}

type recorder struct {
	mu      sync.Mutex
	writer  Writer
	queue   []Event
	windows map[string]*window
	dropped int
	now     func() time.Time
	stop    chan struct{}
	done    chan struct{}
}

var rec = &recorder{windows: map[string]*window{}, now: time.Now}

// Start sets the writer and starts writing the queue every second; the returned function writes
// what is left and stops. Without Start, Record only prints the event to the log.
func Start(w Writer) (stop func()) {
	rec.mu.Lock()
	rec.writer = w
	rec.stop = make(chan struct{})
	rec.done = make(chan struct{})
	stopCh, doneCh := rec.stop, rec.done
	rec.mu.Unlock()
	go func() {
		defer close(doneCh)
		ticker := time.NewTicker(flushInterval)
		defer ticker.Stop()
		for {
			select {
			case <-ticker.C:
				Flush()
			case <-stopCh:
				Flush()
				return
			}
		}
	}()
	var once sync.Once
	return func() {
		once.Do(func() {
			close(stopCh)
			<-doneCh
			rec.mu.Lock()
			rec.writer = nil
			rec.mu.Unlock()
		})
	}
}

// Record notes a security event (never blocks or fails); the time is now when not set
func Record(e Event) {
	r := rec
	r.mu.Lock()
	defer r.mu.Unlock()
	now := r.now()
	if e.Time.IsZero() {
		e.Time = now
	}
	e.UserID = clip(e.UserID, 50)
	e.Address = clip(e.Address, 64)
	e.Company = clip(e.Company, 100)
	e.Details = clip(e.Details, 250)

	key := fmt.Sprintf("%d|%s|%s", e.Kind, strings.ToUpper(e.UserID), e.Address)
	w := r.windows[key]
	if w != nil && now.Sub(w.start) >= throttleWindow {
		r.closeWindow(w)
		delete(r.windows, key)
		w = nil
	}
	if w == nil {
		if len(r.windows) >= maxWindows {
			r.pruneWindows(now)
		}
		if len(r.windows) < maxWindows {
			w = &window{start: now}
			r.windows[key] = w
		}
	}
	if w != nil {
		w.count++
		w.last = e
		if w.count > throttleLimit {
			w.suppressed++
			return
		}
	}
	log.Printf("security: %s user=%q address=%s company=%q %s", e.Kind, e.UserID, e.Address, e.Company, e.Details)
	r.enqueue(e)
}

// Flush writes the queued events now (and the "N more" events of throttle windows that ended)
func Flush() {
	r := rec
	r.mu.Lock()
	now := r.now()
	r.pruneWindows(now)
	events := r.queue
	r.queue = nil
	dropped := r.dropped
	r.dropped = 0
	writer := r.writer
	r.mu.Unlock()

	if dropped > 0 {
		log.Printf("security: %d event(s) not recorded (queue full)", dropped)
	}
	if len(events) == 0 || writer == nil {
		return
	}
	err := writer(events)
	if err != nil {
		log.Printf("security: writing %d event(s): %v", len(events), err)
	}
}

// enqueue adds an event to the queue (r.mu held)
func (r *recorder) enqueue(e Event) {
	if len(r.queue) >= maxQueued {
		r.dropped++
		return
	}
	r.queue = append(r.queue, e)
}

// closeWindow queues the "N more" event of a window that held back events (r.mu held)
func (r *recorder) closeWindow(w *window) {
	if w.suppressed == 0 {
		return
	}
	e := w.last
	e.Time = w.start.Add(throttleWindow)
	e.Details = clip(fmt.Sprintf("%d more within a minute (last: %s)", w.suppressed, e.Details), 250)
	log.Printf("security: %s user=%q address=%s %s", e.Kind, e.UserID, e.Address, e.Details)
	r.enqueue(e)
}

// pruneWindows closes and drops the windows that have ended (r.mu held)
func (r *recorder) pruneWindows(now time.Time) {
	for key, w := range r.windows {
		if now.Sub(w.start) >= throttleWindow {
			r.closeWindow(w)
			delete(r.windows, key)
		}
	}
}

// clip cuts s to n characters
func clip(s string, n int) string {
	s = strings.TrimSpace(s)
	runes := []rune(s)
	if len(runes) <= n {
		return s
	}
	return string(runes[:n])
}

// reset empties the recorder (tests)
func reset(now func() time.Time) {
	rec.mu.Lock()
	defer rec.mu.Unlock()
	rec.queue = nil
	rec.windows = map[string]*window{}
	rec.dropped = 0
	rec.now = now
}

// Reset forgets the queue and the throttle windows (tests in other packages)
func Reset() { reset(time.Now) }
