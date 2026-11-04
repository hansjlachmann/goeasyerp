package pages

import (
	"github.com/hansjlachmann/goeasyerp/backend/business-logic/projectjnl"
	"github.com/hansjlachmann/goeasyerp/backend/business-logic/tables"
	"github.com/hansjlachmann/goeasyerp/backend/foundation/pages"
	gtables "github.com/hansjlachmann/goeasyerp/backend/generated/tables"
)

// The project journal pages 201 Project Journal and 289 Recurring Project Jnl.: OnOpenPage opens a template and one of its
// batches (ProjectJnlManagement); a new line gets its values from the line above (SetUpNewLine);
// the footer shows the line's project description and account name.
const projectJournalPageID = 201

func init() {
	for _, pageID := range []int{projectJournalPageID, tables.RecurringProjectJnlPageID} {
		pages.OnOpenPage(pageID, func(rec *gtables.ProjectJournalLine, page *pages.Open) error {
			return projectjnl.OpenJournalPage(rec, page, pageID == tables.RecurringProjectJnlPageID)
		})
		pages.OnNewRecord(pageID, func(rec, xRec *gtables.ProjectJournalLine, belowXRec bool) {
			_ = tables.SetUpNewProjectJnlLine(rec, xRec)
		})
		pages.OnAfterGetRecord(pageID, func(rec *gtables.ProjectJournalLine, page *pages.Control) {
			projectDescription, accName := projectjnl.GetNames(rec)
			page.SetValue("project_description", projectDescription)
			page.SetValue("account_name", accName)
		})
	}

	// Page 276 Project Jnl. Batches: a new batch takes the template's numbering and reason code
	// (SetupNewBatch)
	pages.OnNewRecord(276, func(rec, xRec *gtables.ProjectJournalBatch, belowXRec bool) {
		_ = tables.SetupNewProjectJnlBatch(rec)
	})
}
