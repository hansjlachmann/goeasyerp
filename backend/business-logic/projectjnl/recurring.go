package projectjnl

import (
	"github.com/hansjlachmann/openerp/backend/business-logic/tables"
	"github.com/hansjlachmann/openerp/backend/foundation/types"
	gtables "github.com/hansjlachmann/openerp/backend/generated/tables"
)

// updateRecurringLines (codeunit 203 Project Jnl.-Post Batch, recurring journal): after posting, each line of the
// journal moves on to its next date (Recurring Frequency); variable lines lose their quantity, total cost and total price
func (b *ProjectJnlPostBatch) updateRecurringLines(projectJnlLine *gtables.ProjectJournalLine) error {
	projectJnlLine2 := gtables.NewProjectJournalLine(b.src)
	projectJnlLine2.CopyFilters(projectJnlLine)
	var lines []*gtables.ProjectJournalLine
	for found := projectJnlLine2.FindSet(); found; found = projectJnlLine2.Next() {
		l := gtables.NewProjectJournalLine(b.src)
		l.Copy(projectJnlLine2)
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
		if l.Recurring_method == gtables.ProjectJournalLine_Recurring_method.Variable && l.No != "" {
			l.Quantity, l.Total_cost, l.Total_price = types.Decimal{}, types.Decimal{}, types.Decimal{}
		}
		err := l.ModifyOrError(false)
		if err != nil {
			return err
		}
	}
	return nil
}
