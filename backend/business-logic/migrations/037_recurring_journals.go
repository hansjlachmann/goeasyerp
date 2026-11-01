package migrations

import (
	fmigrations "github.com/hansjlachmann/openerp/backend/foundation/migrations"
)

func init() {
	Register(&Migration037RecurringJournals{})
}

// Migration037RecurringJournals goes with recurring journals, bank account reconciliation and
// exchange rate adjustment: Gen. Jnl. Allocation 221, Bank Acc. Reconciliation/Line 273/274,
// Bank Account Statement/Line 275/276. The READER role may read them, as all business tables;
// the tables come from the table sync.
type Migration037RecurringJournals struct{}

func (m *Migration037RecurringJournals) Version() int {
	return 37
}

func (m *Migration037RecurringJournals) Name() string {
	return "recurring-bank-reconciliation"
}

func (m *Migration037RecurringJournals) Description() string {
	return "READER reads the journal allocations, bank reconciliations and statements"
}

func (m *Migration037RecurringJournals) Up(ctx *fmigrations.Context) error {
	return grantReaderRead(ctx, []string{"GEN_JNL_ALLOCATION", "BANK_ACC_RECONCILIATION", "BANK_ACC_RECONCILIATION_LINE",
		"BANK_ACCOUNT_STATEMENT", "BANK_ACCOUNT_STATEMENT_LINE"})
}
