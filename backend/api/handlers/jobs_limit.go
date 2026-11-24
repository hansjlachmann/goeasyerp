package handlers

import (
	"strings"
	"sync"
)

// Limits on the routines and reports running at once (S33): a user with Execute on a heavy
// report (Detail Trial Balance, Inventory Valuation, Adjust Cost) could otherwise start dozens
// at once and keep the database and the processors busy for everybody.
//   - maxJobsPerUser per user: one more is refused (a run waiting for its user — a question,
//     a request page, a page opened with RunModal — counts while it waits, up to its timeout)
//   - maxRunningJobs on the server: more wait for a free slot, in the order they came
var (
	maxJobsPerUser = 4
	maxRunningJobs = 16
)

// jobLimiter counts the running jobs per user and holds the server-wide slots
type jobLimiter struct {
	mu      sync.Mutex
	perUser map[string]int
	slots   chan struct{}
}

var runningJobs = newJobLimiter(maxRunningJobs)

func newJobLimiter(slots int) *jobLimiter {
	return &jobLimiter{perUser: map[string]int{}, slots: make(chan struct{}, slots)}
}

// admit counts a new job of the user; false when the user already runs maxJobsPerUser
func (l *jobLimiter) admit(user string) bool {
	user = strings.ToUpper(user)
	l.mu.Lock()
	defer l.mu.Unlock()
	if l.perUser[user] >= maxJobsPerUser {
		return false
	}
	l.perUser[user]++
	return true
}

// start waits for a server-wide slot (the job's goroutine, before it runs)
func (l *jobLimiter) start() {
	l.slots <- struct{}{}
}

// done gives back the job's slot and its count for the user (once the job has run)
func (l *jobLimiter) done(user string) {
	<-l.slots
	l.release(user)
}

// release takes back a job admitted but not started (it could not be started)
func (l *jobLimiter) release(user string) {
	user = strings.ToUpper(user)
	l.mu.Lock()
	defer l.mu.Unlock()
	l.perUser[user]--
	if l.perUser[user] <= 0 {
		delete(l.perUser, user)
	}
}
