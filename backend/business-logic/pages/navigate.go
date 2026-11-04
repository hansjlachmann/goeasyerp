package pages

import (
	"github.com/hansjlachmann/goeasyerp/backend/business-logic/navigate"
	"github.com/hansjlachmann/goeasyerp/backend/foundation/pages"
	gtables "github.com/hansjlachmann/goeasyerp/backend/generated/tables"
)

// Page 344 Navigate: its header fields Document No. and Posting Date are the page parameters
// document_no and posting_date (filters); its lines (a temporary Document Entry) are the
// tables with entries in those filters (FindRecords)
const navigatePageID = 344

func init() {
	pages.OnFindRecords(navigatePageID, func(rec *gtables.DocumentEntry, page *pages.Open) error {
		return navigate.FindRecords(rec, page.Param("document_no"), page.Param("posting_date"))
	})
}
