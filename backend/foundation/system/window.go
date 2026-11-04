package system

import (
	"regexp"
	"strconv"
	"strings"

	"github.com/hansjlachmann/goeasyerp/backend/foundation/codeunits"
)

// Window is the progress window of a codeunit, opened with a template of text fields #1####
// and progress bars @2@@@@ — the same template text as in classic code:
//
//	w := system.OpenWindow("Posting customers\nCustomer #1##########\n@2@@@@@@@@@@@@@")
//	defer w.Close()
//	w.Update(1, cust.No)        // a text field shows the value (Format)
//	w.Update(2, i*10000/total)  // a progress bar takes 0 … 10000
//
// It shows in the codeunit's dialog: the template text with the values filled in and the bar
// last updated. Without a dialog (a scheduled run) it does nothing.
type Window struct {
	template string
	values   map[int]string
	progress int
	closed   bool
}

var windowField = regexp.MustCompile(`#(\d+)#*|@(\d+)@*`)

// OpenWindow opens a progress window with a template (pass a translated text)
func OpenWindow(template string, values ...any) *Window {
	w := &Window{template: template, values: map[int]string{}}
	for i, v := range values {
		w.values[i+1] = Format(v)
	}
	w.show()
	return w
}

// Update sets a field of the window: a text field shows the value, a progress bar takes a
// number from 0 to 10000
func (w *Window) Update(field int, value any) {
	if w.closed {
		return
	}
	if w.isBar(field) {
		n, _ := strconv.Atoi(Format(value))
		w.progress = min(max(n, 0), 10000)
	} else {
		w.values[field] = Format(value)
	}
	w.show()
}

// Close closes the window: later updates are ignored (the codeunit's dialog closes when the
// codeunit ends)
func (w *Window) Close() { w.closed = true }

// Text returns the template with the values filled in and the progress bars left out (empty
// lines dropped)
func (w *Window) Text() string {
	text := windowField.ReplaceAllStringFunc(w.template, func(m string) string {
		if m[0] == '@' {
			return ""
		}
		n, _ := strconv.Atoi(strings.Trim(m, "#"))
		return w.values[n]
	})
	var lines []string
	for _, line := range strings.Split(text, "\n") {
		if line = strings.TrimRight(line, " "); line != "" {
			lines = append(lines, line)
		}
	}
	return strings.Join(lines, "\n")
}

func (w *Window) isBar(field int) bool {
	for _, m := range windowField.FindAllStringSubmatch(w.template, -1) {
		if m[2] != "" {
			if n, _ := strconv.Atoi(m[2]); n == field {
				return true
			}
		}
	}
	return false
}

// show sends the window to the codeunit's dialog: the text and the bar in percent
func (w *Window) show() {
	if d := codeunits.GetCurrentDialog(); d != nil {
		d.UpdateWithMessage(1, w.progress/100, w.Text())
	}
}
