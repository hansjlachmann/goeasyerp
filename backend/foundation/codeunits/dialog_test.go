package codeunits

import (
	"testing"
	"time"
)

// Cancel answers an input dialog with nothing (the browser sends {}): RequestInput gives nil,
// so a caller stops — the Job Queue report runner started the job with empty values instead
func TestRequestInputCancel(t *testing.T) {
	d := OpenDialogWithID("", "request-input-cancel-test")
	SetCurrentDialog(d)
	defer ClearCurrentDialog()
	answer := func(values map[string]string) {
		go func() {
			ev := <-d.Events()
			for !d.RespondToInput(ev.ConfirmID, values) {
				time.Sleep(time.Millisecond)
			}
		}()
	}
	fields := []InputField{{Name: "code", Label: "Code", Type: "text", Required: true}}

	answer(map[string]string{})
	if got := RequestInput("Filter", fields); got != nil {
		t.Errorf("Cancel gave %v, want nil", got)
	}
	answer(map[string]string{"code": "ABC"})
	if got := RequestInput("Filter", fields); got == nil || got["code"] != "ABC" {
		t.Errorf("OK gave %v", got)
	}
	answer(map[string]string{"_ok": "1"}) // OK on a dialog without fields
	if got := RequestInput("Confirm", nil); got == nil {
		t.Error("OK without fields gave nil")
	}
}
