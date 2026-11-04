package pages

import (
	"github.com/hansjlachmann/goeasyerp/backend/business-logic/resjnl"
	"github.com/hansjlachmann/goeasyerp/backend/business-logic/tables"
	"github.com/hansjlachmann/goeasyerp/backend/foundation/pages"
	gtables "github.com/hansjlachmann/goeasyerp/backend/generated/tables"
)

// The resource journal pages 207 Resource Journal and 290 Recurring Resource Jnl.: OnOpenPage opens a template and one of its
// batches (ResJnlManagement); a new line gets its values from the line above (SetUpNewLine);
// the footer shows the line's resource name.
const resJournalPageID = 207

func init() {
	for _, pageID := range []int{resJournalPageID, tables.RecurringResJnlPageID} {
		pages.OnOpenPage(pageID, func(rec *gtables.ResJournalLine, page *pages.Open) error {
			return resjnl.OpenJournalPage(rec, page, pageID == tables.RecurringResJnlPageID)
		})
		pages.OnNewRecord(pageID, func(rec, xRec *gtables.ResJournalLine, belowXRec bool) {
			_ = tables.SetUpNewResJnlLine(rec, xRec)
		})
		pages.OnAfterGetRecord(pageID, func(rec *gtables.ResJournalLine, page *pages.Control) {
			page.SetValue("resource_name", resjnl.GetRes(rec))
		})
	}

	// Page 272 Resource Jnl. Batches: a new batch takes the template's numbering and reason code
	// (SetupNewBatch)
	pages.OnNewRecord(272, func(rec, xRec *gtables.ResJournalBatch, belowXRec bool) {
		_ = tables.SetupNewResJnlBatch(rec)
	})
}
