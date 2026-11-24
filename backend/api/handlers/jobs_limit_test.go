package handlers

import (
	"database/sql"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/hansjlachmann/goeasyerp/backend/business-logic/codeunits"
	fcodeunits "github.com/hansjlachmann/goeasyerp/backend/foundation/codeunits"
	"github.com/hansjlachmann/goeasyerp/backend/foundation/database"
	"github.com/hansjlachmann/goeasyerp/backend/foundation/session"
)

// The limiter: per user a count, refused beyond maxJobsPerUser; server-wide slots
func TestJobLimiter(t *testing.T) {
	old := maxJobsPerUser
	t.Cleanup(func() { maxJobsPerUser = old })
	maxJobsPerUser = 2

	l := newJobLimiter(1)
	if !l.admit("anna") || !l.admit("ANNA") {
		t.Fatal("the first two jobs must be admitted")
	}
	if l.admit("Anna") {
		t.Error("a third job of the same user (any letter case) must be refused")
	}
	if !l.admit("bob") {
		t.Error("another user is counted on his own")
	}
	l.release("bob")

	// one server-wide slot: the second job waits until the first is done
	l.start()
	started := make(chan struct{})
	go func() { l.start(); close(started) }()
	select {
	case <-started:
		t.Fatal("the second job started without a free slot")
	case <-time.After(50 * time.Millisecond):
	}
	l.done("anna")
	select {
	case <-started:
	case <-time.After(time.Second):
		t.Fatal("the waiting job did not start when the slot was freed")
	}
	if !l.admit("anna") {
		t.Error("a finished job gives its count back")
	}
}

// waitCodeunit runs until it is let go (a long report)
type waitCodeunit struct{}

var (
	waitRelease = make(chan struct{})
	waitRunning atomic.Int32
)

func (waitCodeunit) ID() int             { return 99995 }
func (waitCodeunit) Name() string        { return "wait-test" }
func (waitCodeunit) SourceTable() string { return "" }
func (waitCodeunit) UsesProgress() bool  { return true }
func (waitCodeunit) Run(record interface{}) (fcodeunits.Result, error) {
	waitRunning.Add(1)
	defer waitRunning.Add(-1)
	<-waitRelease
	return fcodeunits.Result{Success: true, Message: "done"}, nil
}

// /api/jobs/start refuses a user's run beyond maxJobsPerUser with a translated 429, and the
// user may start again when one has ended
func TestStartJobRefusesTooManyRuns(t *testing.T) {
	old := maxJobsPerUser
	t.Cleanup(func() { maxJobsPerUser = old })
	maxJobsPerUser = 2

	db, err := sql.Open("sqlite3", "file:"+t.Name()+"?mode=memory&cache=shared")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close() })
	codeunits.Register(99995, "wait-test", func(database.Executor, string, database.DBType) fcodeunits.Codeunit {
		return waitCodeunit{}
	})
	app := fiber.New()
	app.Use(func(c *fiber.Ctx) error {
		c.Locals("session", &session.Session{UserID: "LIMITTEST", IsSuper: true, HasRoles: true, Company: "TEST", Language: "en-US"})
		return c.Next()
	})
	app.Post("/api/jobs/start", NewJobsHandlerWithDBType(db, database.DBTypeSQLite).StartJob)

	start := func() (int, string) {
		req := httptest.NewRequest("POST", "/api/jobs/start", strings.NewReader(`{"codeunit_id":99995,"record":{}}`))
		req.Header.Set("Content-Type", "application/json")
		resp, err := app.Test(req, -1)
		if err != nil {
			t.Fatal(err)
		}
		body := make([]byte, 4096)
		n, _ := resp.Body.Read(body)
		return resp.StatusCode, string(body[:n])
	}
	for i := 0; i < 2; i++ {
		if status, body := start(); status != 200 {
			t.Fatalf("run %d: %d %s", i+1, status, body)
		}
	}
	status, body := start()
	if status != fiber.StatusTooManyRequests || !strings.Contains(body, "already have 2 routines or reports running") {
		t.Errorf("third run: %d %s, want 429 with the message", status, body)
	}

	// let the runs end: the user may start again
	close(waitRelease)
	deadline := time.Now().Add(5 * time.Second)
	for waitRunning.Load() > 0 && time.Now().Before(deadline) {
		time.Sleep(10 * time.Millisecond)
	}
	time.Sleep(50 * time.Millisecond) // the counts are given back after Run returns
	if status, body := start(); status != 200 {
		t.Errorf("after the runs ended: %d %s", status, body)
	}
}
