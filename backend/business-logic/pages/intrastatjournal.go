package pages

import (
	"github.com/hansjlachmann/goeasyerp/backend/business-logic/intrastatjnl"
	"github.com/hansjlachmann/goeasyerp/backend/foundation/pages"
	gtables "github.com/hansjlachmann/goeasyerp/backend/generated/tables"
)

// The Intrastat journal page 311: OnOpenPage opens a template and one of its batches (codeunit
// 350); the footer shows the statistical value up to the line and of the whole batch.
func init() {
	pages.OnOpenPage(311, func(rec *gtables.IntrastatJnlLine, page *pages.Open) error {
		return intrastatjnl.OpenJournalPage(rec, page)
	})
	pages.OnAfterGetRecord(311, func(rec *gtables.IntrastatJnlLine, page *pages.Control) {
		statisticalValue, totalStatisticalValue := intrastatjnl.CalcStatisticalValue(rec)
		page.SetValue("statistical_value_to_line", statisticalValue)
		page.SetValue("total_statistical_value", totalStatisticalValue)
	})
}
