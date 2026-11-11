package middleware

import (
	"io"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/gofiber/fiber/v2"
	fcodeunits "github.com/hansjlachmann/goeasyerp/backend/foundation/codeunits"
	"github.com/hansjlachmann/goeasyerp/backend/foundation/session"
)

// A request whose code asks a question (Confirm in a table trigger) is answered with 409 and the
// question; sent again with the answers, Confirm takes them in order and the request does its work
func TestUserContextQuestions(t *testing.T) {
	app := fiber.New()
	app.Use(func(c *fiber.Ctx) error {
		c.Locals("session", session.NewSession(nil, "alpha", nil))
		return c.Next()
	})
	app.Use(UserContext())
	app.Post("/x", func(c *fiber.Ctx) error {
		first := fcodeunits.Confirm("First?")
		second := fcodeunits.Confirm("Second?")
		return c.SendString(map[bool]string{true: "Y", false: "N"}[first] + map[bool]string{true: "Y", false: "N"}[second])
	})
	send := func(answers string) (int, string, string) {
		t.Helper()
		req := httptest.NewRequest("POST", "/x", nil)
		if answers != "" {
			req.Header.Set(AnswersHeader, answers)
		}
		resp, err := app.Test(req)
		if err != nil {
			t.Fatal(err)
		}
		body, _ := io.ReadAll(resp.Body)
		question, _ := url.QueryUnescape(resp.Header.Get(QuestionHeader))
		return resp.StatusCode, question, string(body)
	}
	if status, question, _ := send(""); status != 409 || question != "First?" {
		t.Errorf("no answers: %d %q", status, question)
	}
	if status, question, _ := send("1"); status != 409 || question != "Second?" {
		t.Errorf("one answer: %d %q", status, question)
	}
	if status, question, body := send("1,0"); status != 200 || question != "" || body != "YN" {
		t.Errorf("both answers: %d %q %q", status, question, body)
	}
	if got := parseAnswers(" 1, 0,,1" + strings.Repeat(",1", 30)); len(got) != maxAnswers || !got[0] || got[1] || !got[2] {
		t.Errorf("parseAnswers: %v", got)
	}
}
