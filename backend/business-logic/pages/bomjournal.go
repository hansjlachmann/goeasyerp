package pages

import (
	"github.com/hansjlachmann/goeasyerp/backend/business-logic/bomjnl"
	"github.com/hansjlachmann/goeasyerp/backend/business-logic/tables"
	"github.com/hansjlachmann/goeasyerp/backend/foundation/pages"
	gtables "github.com/hansjlachmann/goeasyerp/backend/generated/tables"
)

// The BOM journal pages 109 BOM Journal and 287 Recurring BOM Jnl.: OnOpenPage opens a template and one of its batches
// (BOMJnlManagement); a new line gets its values from the line above (SetUpNewLine); the footer
// shows the line's item description.
const bomJournalPageID = 109

func init() {
	for _, pageID := range []int{bomJournalPageID, tables.RecurringBOMJnlPageID} {
		pages.OnOpenPage(pageID, func(rec *gtables.BOMJournalLine, page *pages.Open) error {
			return bomjnl.OpenJournalPage(rec, page, pageID == tables.RecurringBOMJnlPageID)
		})
		pages.OnNewRecord(pageID, func(rec, xRec *gtables.BOMJournalLine, belowXRec bool) {
			_ = tables.SetUpNewBOMJnlLine(rec, xRec)
		})
		pages.OnAfterGetRecord(pageID, func(rec *gtables.BOMJournalLine, page *pages.Control) {
			page.SetValue("item_description", bomjnl.GetItem(rec))
		})
	}

	// Page 265 BOM Journal Batches: a new batch takes the template's numbering and reason code
	// (SetupNewBatch)
	pages.OnNewRecord(265, func(rec, xRec *gtables.BOMJournalBatch, belowXRec bool) {
		_ = tables.SetupNewBOMJnlBatch(rec)
	})
}
