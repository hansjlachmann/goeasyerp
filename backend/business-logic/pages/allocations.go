package pages

import (
	"github.com/hansjlachmann/goeasyerp/backend/foundation/pages"
	"github.com/hansjlachmann/goeasyerp/backend/foundation/types"
	gtables "github.com/hansjlachmann/goeasyerp/backend/generated/tables"
)

// Page 284 Allocations: the footer shows the allocated amount up to the cursor line and the
// total of the journal line's allocations (UpdateAllocationAmount)
func init() {
	pages.OnAfterGetRecord(284, func(rec *gtables.GenJnlAllocation, page *pages.Control) {
		genJnlAlloc := gtables.NewGenJnlAllocation(rec)
		genJnlAlloc.SetRange("journal_template_name", rec.Journal_template_name)
		genJnlAlloc.SetRange("journal_batch_name", rec.Journal_batch_name)
		genJnlAlloc.SetRange("journal_line_no", rec.Journal_line_no)
		var total types.Decimal
		if genJnlAlloc.CalcSums("amount") == nil {
			total = genJnlAlloc.Amount
		}
		var upTo types.Decimal
		genJnlAlloc.SetRange("line_no", 0, rec.Line_no)
		if genJnlAlloc.CalcSums("amount") == nil {
			upTo = genJnlAlloc.Amount
		}
		page.SetValue("allocation_amount", upTo)
		page.SetValue("total_allocation_amount", total)
	})
}
