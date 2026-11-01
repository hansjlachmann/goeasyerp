package migrations

import (
	fmigrations "github.com/hansjlachmann/openerp/backend/foundation/migrations"
)

func init() {
	Register(&Migration036RemindersFinanceCharges{})
}

// Migration036RemindersFinanceCharges goes with reminders and finance charge memos: Reminder
// Header/Line 295/296, Issued Reminder Header/Line 297/298, Reminder Comment Line 299,
// Reminder/Fin. Charge Entry 300, Finance Charge Text 301, Finance Charge Memo Header/Line
// 302/303, Issued Fin. Charge Memo Header/Line 304/305, Fin. Charge Comment Line 306. The READER
// role may read them, as all business tables; the tables come from the table sync.
type Migration036RemindersFinanceCharges struct{}

func (m *Migration036RemindersFinanceCharges) Version() int {
	return 36
}

func (m *Migration036RemindersFinanceCharges) Name() string {
	return "reminders-finance-charges"
}

func (m *Migration036RemindersFinanceCharges) Description() string {
	return "READER reads the reminders, finance charge memos and their entries"
}

func (m *Migration036RemindersFinanceCharges) Up(ctx *fmigrations.Context) error {
	return grantReaderRead(ctx, []string{"REMINDER_HEADER", "REMINDER_LINE", "ISSUED_REMINDER_HEADER", "ISSUED_REMINDER_LINE", "REMINDER_COMMENT_LINE",
		"REMINDER_FIN_CHARGE_ENTRY", "FINANCE_CHARGE_TEXT", "FINANCE_CHARGE_MEMO_HEADER", "FINANCE_CHARGE_MEMO_LINE", "ISSUED_FIN_CHARGE_MEMO_HEADER",
		"ISSUED_FIN_CHARGE_MEMO_LINE", "FIN_CHARGE_COMMENT_LINE"})
}
