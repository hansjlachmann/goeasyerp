package pages

import (
	"github.com/hansjlachmann/openerp/backend/business-logic/setupchecklist"
	"github.com/hansjlachmann/openerp/backend/business-logic/tables"
	"github.com/hansjlachmann/openerp/backend/foundation/pages"
	gtables "github.com/hansjlachmann/openerp/backend/generated/tables"
)

// Page 531 Setup Checklist makes the checklist again when it opens (UpdateChecklist) and shows
// each table's records in the company (No. of Records); on page 532 Setup Checklist Comment
// Sheet the first comment of a table gets the work date (SetUpNewLine)
func init() {
	pages.OnOpenPage(531, func(rec *gtables.SetupChecklistLine, page *pages.Open) error {
		return setupchecklist.UpdateChecklist(rec)
	})
	pages.OnAfterGetRecord(531, func(rec *gtables.SetupChecklistLine, page *pages.Control) {
		page.SetValue("no_of_records", setupchecklist.NoOfRecords(rec, rec.GetCompany(), rec.Table_id))
	})
	pages.OnNewRecord(532, func(rec, xRec *gtables.SetupChecklistComment, belowXRec bool) {
		tables.SetUpNewSetupChecklistComment(rec)
	})
}
