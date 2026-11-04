package fajnl

import (
	"github.com/hansjlachmann/goeasyerp/backend/business-logic/tables"
	"github.com/hansjlachmann/goeasyerp/backend/foundation/types"
	gtables "github.com/hansjlachmann/goeasyerp/backend/generated/tables"
)

// updateRecurringLines (codeunit 5633 FA Jnl.-Post Batch, recurring journal): after posting, each line of the
// journal moves on to its next date (Recurring Frequency); lines that are not fixed lose their amounts (ZeroAmounts)
func (b *FAJnlPostBatch) updateRecurringLines(faJnlLine *gtables.FAJournalLine) error {
	faJnlLine2 := gtables.NewFAJournalLine(b.src)
	faJnlLine2.CopyFilters(faJnlLine)
	var lines []*gtables.FAJournalLine
	for found := faJnlLine2.FindSet(); found; found = faJnlLine2.Next() {
		l := gtables.NewFAJournalLine(b.src)
		l.Copy(faJnlLine2)
		lines = append(lines, l)
	}
	for _, l := range lines {
		if !l.Fa_posting_date.IsZero() {
			nextDate, err := tables.NextRecurringDate(l.Fa_posting_date, l.Recurring_frequency)
			if err != nil {
				return err
			}
			err = l.Validate("fa_posting_date", nextDate)
			if err != nil {
				return err
			}
		}
		if l.Recurring_method != gtables.FAJournalLine_Recurring_method.FFixed {
			l.Amount, l.Debit_amount, l.Credit_amount, l.Salvage_value = types.Decimal{}, types.Decimal{}, types.Decimal{}, types.Decimal{}
		}
		err := l.ModifyOrError(false)
		if err != nil {
			return err
		}
	}
	return nil
}
