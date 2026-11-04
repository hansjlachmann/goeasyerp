package pages

import (
	"github.com/hansjlachmann/goeasyerp/backend/business-logic/reqwksh"
	"github.com/hansjlachmann/goeasyerp/backend/business-logic/tables"
	"github.com/hansjlachmann/goeasyerp/backend/foundation/pages"
	gtables "github.com/hansjlachmann/goeasyerp/backend/generated/tables"
)

// The requisition worksheet pages 291 Req. Worksheet and 296 Recurring Req. Worksheet: OnOpenPage
// opens a template and one of its worksheets (codeunit 330), a new line takes the type, method
// and order date of the line above (SetUpNewLine), the footer shows the line's description and
// vendor name.
func init() {
	for _, pageID := range []int{tables.ReqWorksheetPageID, tables.RecurringReqWorksheetPageID} {
		recurringJnl := pageID == tables.RecurringReqWorksheetPageID
		pages.OnOpenPage(pageID, func(rec *gtables.RequisitionLine, page *pages.Open) error {
			return reqwksh.OpenJournalPage(rec, page, recurringJnl)
		})
		pages.OnNewRecord(pageID, func(rec, xRec *gtables.RequisitionLine, belowXRec bool) {
			reqwksh.SetUpNewLine(rec, xRec)
		})
		pages.OnAfterGetRecord(pageID, func(rec *gtables.RequisitionLine, page *pages.Control) {
			description, buyFromVendorName := reqwksh.GetDescriptionAndRcptName(rec)
			page.SetValue("no_description", description)
			page.SetValue("buy_from_vendor_name", buyFromVendorName)
		})
	}
}
