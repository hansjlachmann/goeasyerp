package handlers

import (
	"fmt"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gofiber/fiber/v2"

	fcodeunits "github.com/hansjlachmann/goeasyerp/backend/foundation/codeunits"
	"github.com/hansjlachmann/goeasyerp/backend/foundation/session"
)

// jobOwnerApp serves the job endpoints to user in company ("" user: no session)
func jobOwnerApp(user, company string) *fiber.App {
	app := fiber.New()
	app.Use(func(c *fiber.Ctx) error {
		if user != "" {
			c.Locals("session", &session.Session{UserID: user, Company: company, Language: "en-US"})
		} else {
			c.Locals("session", (*session.Session)(nil))
		}
		return c.Next()
	})
	h := &JobsHandler{}
	app.Get("/api/jobs/:id/events", h.GetJobEvents)
	app.Post("/api/jobs/:id/confirm", h.RespondToConfirm)
	app.Post("/api/jobs/:id/cancel", h.CancelJob)
	return app
}

// Only the user who started a job, in its company, follows, answers or cancels it: a job ID
// read elsewhere (the request log) gives nobody else access (S24)
func TestJobEndpointsBoundToTheirUser(t *testing.T) {
	d := fcodeunits.OpenDialogWithID("", "owned-job-test")
	d.SetOwner("HANS", "TEST")
	t.Cleanup(func() { fcodeunits.GetJobRegistry().Remove("owned-job-test") })
	status := func(app *fiber.App, method, path string) int {
		t.Helper()
		req := httptest.NewRequest(method, path, strings.NewReader(`{"confirm_id":"x","response":true}`))
		req.Header.Set("Content-Type", "application/json")
		resp, err := app.Test(req, 2000)
		if err != nil {
			t.Fatal(err)
		}
		return resp.StatusCode
	}
	for _, c := range []struct {
		name, user, company string
		want                int
	}{
		{"no session", "", "", 401},
		{"another user", "KARI", "TEST", 404},
		{"the user in another company", "HANS", "OTHER", 404},
	} {
		app := jobOwnerApp(c.user, c.company)
		for _, ep := range [][2]string{{"GET", "/api/jobs/owned-job-test/events"}, {"POST", "/api/jobs/owned-job-test/confirm"}, {"POST", "/api/jobs/owned-job-test/cancel"}} {
			if got := status(app, ep[0], ep[1]); got != c.want {
				t.Errorf("%s: %s %s = %d, want %d", c.name, ep[0], ep[1], got, c.want)
			}
		}
	}
	if d.Context().Err() != nil {
		t.Fatal("another user cancelled the job")
	}
	if got := status(jobOwnerApp("hans", "TEST"), "POST", "/api/jobs/owned-job-test/cancel"); got != 200 || d.Context().Err() == nil {
		t.Errorf("the job's user cancelling: %d, cancelled %v", got, d.Context().Err() != nil)
	}
}

// Input dialog answers may hold numbers (number inputs): they reach the codeunit as text
func TestInputAnswerWithNumbers(t *testing.T) {
	d := fcodeunits.OpenDialogWithID("", "input-number-test")
	d.SetOwner("HANS", "TEST")
	answer := make(chan map[string]string, 1)
	go func() {
		answer <- d.RequestInput("Options", []fcodeunits.InputField{{Name: "days", Type: "number"}})
	}()
	ev := <-d.Events()

	app := jobOwnerApp("HANS", "TEST")
	out := postJSON(t, app, "/api/jobs/input-number-test/confirm",
		fmt.Sprintf(`{"confirm_id":%q,"values":{"days":30,"filter":"J001","flag":true,"empty":null}}`, ev.ConfirmID))
	if out["success"] != true {
		t.Fatalf("answer refused: %v", out)
	}
	values := <-answer
	if values["days"] != "30" || values["filter"] != "J001" || values["flag"] != "true" || values["empty"] != "" {
		t.Errorf("values %v", values)
	}
}
