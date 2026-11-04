package pages

import (
	"github.com/hansjlachmann/goeasyerp/backend/business-logic/fareclass"
	"github.com/hansjlachmann/goeasyerp/backend/business-logic/tables"
	"github.com/hansjlachmann/goeasyerp/backend/foundation/pages"
	gtables "github.com/hansjlachmann/goeasyerp/backend/generated/tables"
)

// The FA reclass. journal page 5636: OnOpenPage opens a template and one of its batches
// (codeunit 5646); a new line takes the FA posting date of the line above (SetUpNewLine); the
// footer shows the descriptions of both fixed assets.
const faReclassJournalPageID = 5636

func init() {
	pages.OnOpenPage(faReclassJournalPageID, func(rec *gtables.FAReclassJournalLine, page *pages.Open) error {
		return fareclass.OpenJournalPage(rec, page)
	})
	pages.OnNewRecord(faReclassJournalPageID, func(rec, xRec *gtables.FAReclassJournalLine, belowXRec bool) {
		tables.SetUpNewFAReclassJnlLine(rec, xRec)
	})
	pages.OnAfterGetRecord(faReclassJournalPageID, func(rec *gtables.FAReclassJournalLine, page *pages.Control) {
		faDescription, newFADescription := fareclass.GetFAS(rec)
		page.SetValue("fa_description", faDescription)
		page.SetValue("new_fa_description", newFADescription)
	})
}
