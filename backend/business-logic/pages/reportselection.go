package pages

import (
	"strconv"

	// Document-Print and PrintRecords: the Print functions of the document pages
	_ "github.com/hansjlachmann/openerp/backend/business-logic/docprint"
	"github.com/hansjlachmann/openerp/backend/business-logic/tables"
	"github.com/hansjlachmann/openerp/backend/foundation/pages"
	gtables "github.com/hansjlachmann/openerp/backend/generated/tables"
)

// Pages 306 Report Selection - Sales, 347 - Purchase, 524 - Reminder and 385 - Bank Acc.: the
// lines of the usage chosen above the list (the page's usage parameter, one of its header
// field's options — the first when none or another is given; SetUsageFilter), a new line with
// the next sequence (NewRecord)
func init() {
	for _, pageID := range []int{306, 347, 524, 385} {
		pages.OnOpenPage(pageID, func(rec *gtables.ReportSelections, page *pages.Open) error {
			return setUsageFilter(pageID, page)
		})
		pages.OnNewRecord(pageID, func(rec, xRec *gtables.ReportSelections, belowXRec bool) {
			tables.NewReportSelection(rec)
		})
	}
}

// setUsageFilter (SetUsageFilter): the page's usage parameter as the lines' filter
func setUsageFilter(pageID int, page *pages.Open) error {
	def, err := pages.GetRegistry().GetPage(pageID)
	if err != nil {
		return err
	}
	options := def.Page.HeaderFields[0].Options
	usage := page.Param("usage")
	known := false
	for _, option := range options {
		if option.Value == usage {
			known = true
		}
	}
	if !known {
		usage = options[0].Value
		page.SetParam("usage", usage)
	}
	value, err := strconv.Atoi(usage)
	if err != nil {
		return err
	}
	page.SetFilter("usage", value)
	return nil
}
