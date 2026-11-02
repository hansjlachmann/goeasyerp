package pages

import (
	"github.com/hansjlachmann/openerp/backend/business-logic/fajnl"
	"github.com/hansjlachmann/openerp/backend/business-logic/tables"
	"github.com/hansjlachmann/openerp/backend/foundation/pages"
	ftables "github.com/hansjlachmann/openerp/backend/foundation/tables"
	gtables "github.com/hansjlachmann/openerp/backend/generated/tables"
)

// The FA journal pages 5629 Fixed Asset Journal and 5634 Recurring Fixed Asset Journal: OnOpenPage opens a template and one of its batches
// (FAJnlManagement); a new line gets its values from the line above (SetUpNewLine); the footer
// shows the line's fixed asset description.
const faJournalPageID = 5629

func init() {
	for _, pageID := range []int{faJournalPageID, tables.RecurringFAJnlPageID} {
		pages.OnOpenPage(pageID, func(rec *gtables.FAJournalLine, page *pages.Open) error {
			err := ensureAllFAPostingTypeSetups(rec)
			if err != nil {
				return err
			}
			return fajnl.OpenJournalPage(rec, page, pageID == tables.RecurringFAJnlPageID)
		})
		pages.OnNewRecord(pageID, func(rec, xRec *gtables.FAJournalLine, belowXRec bool) {
			_ = tables.SetUpNewFAJnlLine(rec, xRec)
		})
		pages.OnAfterGetRecord(pageID, func(rec *gtables.FAJournalLine, page *pages.Control) {
			page.SetValue("fa_description", fajnl.GetFADescription(rec))
		})
	}
	// Page 5611 Depreciation Book List: the books entered before FA Posting Type Setup existed
	// get their lines (Depreciation Book OnInsert creates them for new books)
	pages.OnOpenPage(5611, func(rec *gtables.DepreciationBook, page *pages.Open) error {
		return ensureAllFAPostingTypeSetups(rec)
	})
	// Page 5660 Depreciation Table Lines: a new line's period no. follows the last (NewRecord)
	pages.OnNewRecord(5660, func(rec, xRec *gtables.DepreciationTableLine, belowXRec bool) {
		tables.NewDeprTableLine(rec)
	})

	// Page 5633 FA Journal Batches: a new batch takes the template's numbering and reason code
	// (SetupNewBatch)
	pages.OnNewRecord(5633, func(rec, xRec *gtables.FAJournalBatch, belowXRec bool) {
		_ = tables.SetupNewFAJnlBatch(rec)
	})
}

// ensureAllFAPostingTypeSetups: every depreciation book has its FA Posting Type Setup lines
func ensureAllFAPostingTypeSetups(src ftables.Source) error {
	deprBook := gtables.NewDepreciationBook(src)
	for found := deprBook.FindSet(); found; found = deprBook.Next() {
		err := tables.EnsureFAPostingTypeSetups(src, deprBook.Code)
		if err != nil {
			return err
		}
	}
	return nil
}
