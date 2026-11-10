package pages

import (
	"github.com/hansjlachmann/goeasyerp/backend/business-logic/navigate"
	fcodeunits "github.com/hansjlachmann/goeasyerp/backend/foundation/codeunits"
	"github.com/hansjlachmann/goeasyerp/backend/foundation/i18n"
	"github.com/hansjlachmann/goeasyerp/backend/foundation/pages"
	gtables "github.com/hansjlachmann/goeasyerp/backend/generated/tables"
)

// Page 344 Navigate: its header fields Document No. and Posting Date are the page parameters
// document_no and posting_date (filters); its lines (a temporary Document Entry) are the
// tables with entries in those filters (FindRecords). With both blank, the external search
// (FindPush → FindExtRecords): the header fields External Document No., Business Contact Type
// (Vendor, Customer) and Business Contact No. find the documents by the vendor's or customer's
// ledger entries.
const navigatePageID = 344

func init() {
	pages.OnFindRecords(navigatePageID, func(rec *gtables.DocumentEntry, page *pages.Open) error {
		docNoFilter := page.Param("document_no")
		postingDateFilter := page.Param("posting_date")
		extDocNo := page.Param("ext_document_no")
		contactNo := page.Param("contact_no")
		if docNoFilter == "" && postingDateFilter == "" && (extDocNo != "" || contactNo != "") {
			found, err := navigate.FindExtRecords(rec, page.Param("contact_type"), contactNo, extDocNo)
			if err != nil {
				return err
			}
			if !found {
				fcodeunits.Notify(i18n.GetInstance().Message("NAVIGATE_NO_EXT_RECORDS", fcodeunits.CurrentLanguage()))
			}
			return nil
		}
		return navigate.FindRecords(rec, docNoFilter, postingDateFilter)
	})
}
