package codeunits

import (
	"testing"

	fcodeunits "github.com/hansjlachmann/goeasyerp/backend/foundation/codeunits"
)

// A dialog field's label is a message key when one exists, else shown as written — a plain
// label came out as "messages.Code"
func TestResolveFieldsLabels(t *testing.T) {
	fields := resolveFields([]fcodeunits.InputField{
		{Name: "code", Label: "Code", Type: "text"},
		{Name: "period", Label: "PERIOD_CODE", Type: "text"},
		{Name: "salesperson", Label: "Salesperson Code", Type: "text"},
		{Name: "date", Label: "CU_REPORT_DATE", Type: "date", Default: "today"},
	}, "en-US")
	if fields[0].Label != "Code" {
		t.Errorf("a plain label: %q", fields[0].Label)
	}
	if fields[1].Label != "Code" {
		t.Errorf("a message key: %q", fields[1].Label)
	}
	if fields[2].Label != "Salesperson Code" {
		t.Errorf("a plain label with a space: %q", fields[2].Label)
	}
	if fields[3].Label == "CU_REPORT_DATE" || fields[3].Label == "messages.CU_REPORT_DATE" || len(fields[3].Default) != 10 {
		t.Errorf("the date field: %+v", fields[3])
	}
}
