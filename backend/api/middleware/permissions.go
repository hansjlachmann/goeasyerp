package middleware

import (
	"encoding/json"
	"net/url"
	"strings"

	"github.com/gofiber/fiber/v2"
	apitypes "github.com/hansjlachmann/goeasyerp/backend/api/types"
	fcodeunits "github.com/hansjlachmann/goeasyerp/backend/foundation/codeunits"
	apperrors "github.com/hansjlachmann/goeasyerp/backend/foundation/errors"
	"github.com/hansjlachmann/goeasyerp/backend/foundation/session"
)

// getSessionFromLocals retrieves the session from c.Locals, falling back to global session
func getSessionFromLocals(c *fiber.Ctx) *session.Session {
	if sess, ok := c.Locals("session").(*session.Session); ok {
		return sess
	}
	return session.GetCurrent()
}

// PermissionCheck returns middleware that enforces table-level permissions.
// Maps HTTP method + path to a permission operation and checks against the session.
func PermissionCheck() fiber.Handler {
	return func(c *fiber.Ctx) error {
		sess := getSessionFromLocals(c)
		if sess == nil {
			// No session — let downstream handlers deal with it
			return c.Next()
		}

		// SUPER users bypass all permission checks
		if sess.IsSuper {
			return c.Next()
		}

		tableName := c.Params("table")
		if tableName == "" {
			return c.Next()
		}

		op := mapRouteToOperation(c)
		if op == "" {
			// Route doesn't map to a permission operation (e.g. unknown path)
			return c.Next()
		}

		if sess.HasPermission(tableName, op) {
			return c.Next()
		}

		language := sess.GetLanguage()
		opName := string(op)
		appErr := apperrors.PermissionDenied(opName, tableName)
		return c.Status(403).JSON(apitypes.NewErrorResponse(appErr.Message(language)))
	}
}

// mapRouteToOperation maps the HTTP method and path to a permission operation.
func mapRouteToOperation(c *fiber.Ctx) session.PermissionOperation {
	method := c.Method()
	path := c.Path()

	switch method {
	case "GET":
		return session.PermRead
	case "POST":
		// Initializing a new record is the first step of inserting one
		// /insert-many: a list paste inserts records
		if strings.HasSuffix(path, "/insert") || strings.HasSuffix(path, "/init") || strings.HasSuffix(path, "/insert-many") {
			return session.PermInsert
		}
		// /copy: a list copy reads records
		if strings.HasSuffix(path, "/validate") || strings.HasSuffix(path, "/copy") {
			return session.PermRead
		}
		return ""
	case "PUT":
		return session.PermModify
	case "DELETE":
		// removing a picture changes its record
		if strings.Contains(path, "/blob/") {
			return session.PermModify
		}
		return session.PermDelete
	default:
		return ""
	}
}

// UserContext gives the request's code the user: table triggers and codeunits read the user
// (system.UserId), the company, the language and the user's permissions (CanExecute) from the
// goroutine context while the request runs
func UserContext() fiber.Handler {
	return func(c *fiber.Ctx) error {
		sess := getSessionFromLocals(c)
		if sess == nil {
			return c.Next()
		}
		fcodeunits.SetCurrentContext(sess.GetUserID(), sess.GetUserName(), sess.GetCompany(), sess.GetLanguage())
		fcodeunits.SetCurrentAccess(sess)
		defer fcodeunits.ClearCurrentContext()
		fcodeunits.StartNotes()
		fcodeunits.StartQuestions(parseAnswers(c.Get(AnswersHeader)))
		err := c.Next()
		question := fcodeunits.TakeQuestion()
		setNotesHeader(c, fcodeunits.TakeNotes())
		if question != "" {
			// nothing was saved (ftables.Commit refused): the browser asks the user and sends the
			// request again with the answers
			c.Set(QuestionHeader, url.QueryEscape(question))
			return c.Status(fiber.StatusConflict).JSON(fiber.Map{"success": false, "error": question, "question": question})
		}
		return err
	}
}

// QuestionHeader carries the question a request's code asked (classic CONFIRM in a table
// trigger, fcodeunits.Confirm), URL-encoded, with status 409: nothing was saved, the browser
// asks the user and sends the request again with AnswersHeader
const QuestionHeader = "X-App-Question"

// AnswersHeader carries the user's answers to the questions a request asked so far, in the
// order they were asked: "1" Yes, "0" No, separated by commas
const AnswersHeader = "X-App-Answers"

// maxAnswers: a request asks at most this many questions (the browser stops sending it again)
const maxAnswers = 20

func parseAnswers(header string) []bool {
	var answers []bool
	for _, a := range strings.Split(header, ",") {
		a = strings.TrimSpace(a)
		if a == "" {
			continue
		}
		if len(answers) == maxAnswers {
			break
		}
		answers = append(answers, a == "1")
	}
	return answers
}

// NotesHeader carries the notes of a request (fcodeunits.Notify, e.g. a number series reaching
// its Warning No.): a JSON array of texts, URL-encoded (header values are ASCII); the browser
// shows each
const NotesHeader = "X-App-Notes"

func setNotesHeader(c *fiber.Ctx, notes []string) {
	if len(notes) == 0 {
		return
	}
	data, err := json.Marshal(notes)
	if err != nil {
		return
	}
	c.Set(NotesHeader, url.QueryEscape(string(data)))
}
