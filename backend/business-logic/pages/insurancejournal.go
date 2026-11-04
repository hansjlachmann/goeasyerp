package pages

import (
	"github.com/hansjlachmann/goeasyerp/backend/business-logic/insurancejnl"
	"github.com/hansjlachmann/goeasyerp/backend/business-logic/tables"
	"github.com/hansjlachmann/goeasyerp/backend/foundation/pages"
	gtables "github.com/hansjlachmann/goeasyerp/backend/generated/tables"
)

// The insurance journal page 5654: OnOpenPage opens a template and one of its batches (codeunit
// 5656); a new line gets its values from the line above (SetUpNewLine); the footer shows the
// descriptions of the line's insurance and fixed asset.
const insuranceJournalPageID = 5654

func init() {
	pages.OnOpenPage(insuranceJournalPageID, func(rec *gtables.InsuranceJournalLine, page *pages.Open) error {
		return insurancejnl.OpenJournalPage(rec, page)
	})
	pages.OnNewRecord(insuranceJournalPageID, func(rec, xRec *gtables.InsuranceJournalLine, belowXRec bool) {
		_ = tables.SetUpNewInsuranceJnlLine(rec, xRec)
	})
	pages.OnAfterGetRecord(insuranceJournalPageID, func(rec *gtables.InsuranceJournalLine, page *pages.Control) {
		insuranceDescription, faDescription := insurancejnl.GetDescriptions(rec)
		page.SetValue("insurance_description", insuranceDescription)
		page.SetValue("fa_description_footer", faDescription)
	})
	// Page 5655 Insurance Journal Batches: a new batch takes the template's numbering and reason
	// code (SetupNewBatch)
	pages.OnNewRecord(5655, func(rec, xRec *gtables.InsuranceJournalBatch, belowXRec bool) {
		_ = tables.SetupNewInsuranceJnlBatch(rec)
	})
}
