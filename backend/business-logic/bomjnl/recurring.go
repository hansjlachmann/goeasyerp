package bomjnl

import (
	"github.com/hansjlachmann/openerp/backend/business-logic/tables"
	"github.com/hansjlachmann/openerp/backend/foundation/types"
	gtables "github.com/hansjlachmann/openerp/backend/generated/tables"
)

// updateRecurringLines (codeunit 33 BOM Jnl.-Post Batch, recurring journal): after posting, each line of the
// journal moves on to its next date (Recurring Frequency); variable lines lose their quantity and total cost
func (b *BOMJnlPostBatch) updateRecurringLines(bomJnlLine *gtables.BOMJournalLine) error {
	bomJnlLine2 := gtables.NewBOMJournalLine(b.src)
	bomJnlLine2.CopyFilters(bomJnlLine)
	var lines []*gtables.BOMJournalLine
	for found := bomJnlLine2.FindSet(); found; found = bomJnlLine2.Next() {
		l := gtables.NewBOMJournalLine(b.src)
		l.Copy(bomJnlLine2)
		lines = append(lines, l)
	}
	for _, l := range lines {
		if !l.Posting_date.IsZero() {
			nextDate, err := tables.NextRecurringDate(l.Posting_date, l.Recurring_frequency)
			if err != nil {
				return err
			}
			err = l.Validate("posting_date", nextDate)
			if err != nil {
				return err
			}
		}
		if l.Recurring_method == gtables.BOMJournalLine_Recurring_method.Variable && l.Item_no != "" {
			l.Quantity, l.Total_cost = types.Decimal{}, types.Decimal{}
		}
		err := l.ModifyOrError(false)
		if err != nil {
			return err
		}
	}
	return nil
}
