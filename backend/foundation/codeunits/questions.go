package codeunits

import (
	"sync"

	ftables "github.com/hansjlachmann/goeasyerp/backend/foundation/tables"
)

// Questions in an API request (classic CONFIRM in a table trigger): a request has no dialog to
// wait on, so it is answered in rounds. The first time code asks, Confirm records the question
// and answers No; the request's transaction cannot commit (ftables.Commit refuses — nothing is
// saved) and the API answers with the question (409, middleware.QuestionHeader). The browser asks
// the user and sends the request again with the answers so far (middleware.AnswersHeader): the
// code runs again from the start and Confirm takes the answers in the order it asks, until it
// has asked everything — then the request does its work.

// currentQuestions holds the questions state of the current goroutine (an API request)
var currentQuestions sync.Map

type questions struct {
	answers  []bool
	next     int
	question string // the first question not answered yet
}

func init() {
	ftables.SetCommitGuard(func() error {
		question := PendingQuestion()
		if question == "" {
			return nil
		}
		return &QuestionError{Question: question}
	})
}

// QuestionError is the refused commit of a request with a question not answered yet
type QuestionError struct {
	Question string
}

func (e *QuestionError) Error() string { return e.Question }

// StartQuestions lets the code of the current goroutine (an API request) ask questions, with the
// answers the user gave to the questions asked before
func StartQuestions(answers []bool) {
	currentQuestions.Store(getGoroutineID(), &questions{answers: answers})
}

// TakeQuestion returns the question the request asked and has no answer yet ("" when none) and
// stops the questions of the goroutine
func TakeQuestion() string {
	v, ok := currentQuestions.LoadAndDelete(getGoroutineID())
	if !ok {
		return ""
	}
	return v.(*questions).question
}

// PendingQuestion is the question the current goroutine asked without an answer yet ("" when none)
func PendingQuestion() string {
	v, ok := currentQuestions.Load(getGoroutineID())
	if !ok {
		return ""
	}
	return v.(*questions).question
}

// CanAskQuestions reports whether the current goroutine is an API request whose code can ask
// questions (Confirm)
func CanAskQuestions() bool {
	_, ok := currentQuestions.Load(getGoroutineID())
	return ok
}

// requestConfirm answers a question in an API request: the user's answer when the request
// brought one, else No — recording the question (the first one) for the browser to ask
func requestConfirm(message string) (bool, bool) {
	v, ok := currentQuestions.Load(getGoroutineID())
	if !ok {
		return false, false
	}
	q := v.(*questions)
	if q.question == "" && q.next < len(q.answers) {
		answer := q.answers[q.next]
		q.next++
		return answer, true
	}
	if q.question == "" {
		q.question = message
	}
	return false, true
}
