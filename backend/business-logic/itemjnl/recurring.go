package itemjnl

import (
	"github.com/hansjlachmann/goeasyerp/backend/business-logic/tables"
	"github.com/hansjlachmann/goeasyerp/backend/foundation/types"
	gtables "github.com/hansjlachmann/goeasyerp/backend/generated/tables"
)

// updateRecurringLines (codeunit 23 Item Jnl.-Post Batch, recurring journal): after posting, each line of the
// journal moves on to its next date (Recurring Frequency); variable lines lose their quantity and amount
func (b *ItemJnlPostBatch) updateRecurringLines(itemJnlLine *gtables.ItemJournalLine) error {
	itemJnlLine2 := gtables.NewItemJournalLine(b.src)
	itemJnlLine2.CopyFilters(itemJnlLine)
	var lines []*gtables.ItemJournalLine
	for found := itemJnlLine2.FindSet(); found; found = itemJnlLine2.Next() {
		l := gtables.NewItemJournalLine(b.src)
		l.Copy(itemJnlLine2)
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
		if l.Recurring_method == gtables.ItemJournalLine_Recurring_method.Variable && l.Item_no != "" {
			l.Quantity, l.Invoiced_quantity, l.Amount = types.Decimal{}, types.Decimal{}, types.Decimal{}
		}
		err := l.ModifyOrError(false)
		if err != nil {
			return err
		}
	}
	return nil
}
