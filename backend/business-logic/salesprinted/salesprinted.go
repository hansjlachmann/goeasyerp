// Package salesprinted is codeunit 313 Sales-Printed: a sales document a report printed (not a
// preview) counts one more print in No. Printed.
package salesprinted

import (
	gtables "github.com/hansjlachmann/goeasyerp/backend/generated/tables"
)

// Run (OnRun): the document read again, No. Printed raised by one and saved (no triggers)
func Run(rec *gtables.SalesHeader) error {
	salesHeader := gtables.NewSalesHeader(rec)
	err := salesHeader.GetOrError(map[string]interface{}{"document_type": int(rec.Document_type), "no": rec.No})
	if err != nil {
		return err
	}
	salesHeader.No_printed = salesHeader.No_printed + 1
	return salesHeader.ModifyOrError(false)
}
