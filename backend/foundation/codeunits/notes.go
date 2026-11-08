package codeunits

import (
	"sync"
	"time"
)

// currentNotes holds the notes a request collects for the user (per goroutine): StartNotes
// begins collecting, Notify adds one, TakeNotes returns them
var currentNotes sync.Map

type notes struct {
	texts []string
}

// StartNotes begins collecting the notes of the current goroutine (an API request): the API
// sends them with its answer, and the browser shows them
func StartNotes() {
	currentNotes.Store(getGoroutineID(), &notes{})
}

// TakeNotes returns the notes collected since StartNotes and stops collecting
func TakeNotes() []string {
	v, ok := currentNotes.LoadAndDelete(getGoroutineID())
	if !ok {
		return nil
	}
	return v.(*notes).texts
}

// Notify tells the user something without stopping the code (classic MESSAGE in a trigger):
// in a codeunit with a dialog it is shown at once, in an API request it comes with the answer;
// elsewhere (a scheduled run) it is dropped. Pass a translated text.
func Notify(text string) {
	if text == "" {
		return
	}
	dialog := GetCurrentDialog()
	if dialog != nil {
		dialog.Notify(text)
		return
	}
	v, ok := currentNotes.Load(getGoroutineID())
	if !ok {
		return
	}
	n := v.(*notes)
	for _, t := range n.texts {
		if t == text {
			return // once per request
		}
	}
	n.texts = append(n.texts, text)
}

// Notify shows a note to the user (an info message, the code goes on)
func (d *Dialog) Notify(text string) {
	d.mu.Lock()
	defer d.mu.Unlock()

	if d.closed {
		return
	}

	select {
	case d.events <- ProgressEvent{
		JobID:     d.jobID,
		EventType: "notify",
		Message:   text,
		Timestamp: time.Now().UnixMilli(),
	}:
	case <-d.ctx.Done():
	}
}
