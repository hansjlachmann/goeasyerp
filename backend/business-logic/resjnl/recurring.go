package resjnl

import (
	"github.com/hansjlachmann/goeasyerp/backend/business-logic/tables"
	"github.com/hansjlachmann/goeasyerp/backend/foundation/types"
	gtables "github.com/hansjlachmann/goeasyerp/backend/generated/tables"
)

// updateRecurringLines (codeunit 213 Res. Jnl.-Post Batch, recurring journal): after posting, each line of the
// journal moves on to its next date (Recurring Frequency); variable lines lose their quantity, total cost and total price
func (b *ResJnlPostBatch) updateRecurringLines(resJnlLine *gtables.ResJournalLine) error {
	resJnlLine2 := gtables.NewResJournalLine(b.src)
	resJnlLine2.CopyFilters(resJnlLine)
	var lines []*gtables.ResJournalLine
	for found := resJnlLine2.FindSet(); found; found = resJnlLine2.Next() {
		l := gtables.NewResJournalLine(b.src)
		l.Copy(resJnlLine2)
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
		if l.Recurring_method == gtables.ResJournalLine_Recurring_method.Variable && l.Resource_no != "" {
			l.Quantity, l.Total_cost, l.Total_price = types.Decimal{}, types.Decimal{}, types.Decimal{}
		}
		err := l.ModifyOrError(false)
		if err != nil {
			return err
		}
	}
	return nil
}
