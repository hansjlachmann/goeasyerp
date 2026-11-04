package migrations

import (
	fmigrations "github.com/hansjlachmann/goeasyerp/backend/foundation/migrations"
)

func init() {
	Register(&Migration051ProjectJournalReconcile{})
}

// Migration051ProjectJournalReconcile goes with page 376 Project Journal Reconcile: the READER
// role may read its temporary table 278 Project Journal Quantity, as all business tables; the
// table comes from the table sync.
type Migration051ProjectJournalReconcile struct{}

func (m *Migration051ProjectJournalReconcile) Version() int {
	return 51
}

func (m *Migration051ProjectJournalReconcile) Name() string {
	return "project_journal_reconcile"
}

func (m *Migration051ProjectJournalReconcile) Description() string {
	return "READER reads the project journal reconciliation"
}

func (m *Migration051ProjectJournalReconcile) Up(ctx *fmigrations.Context) error {
	return grantReaderRead(ctx, []string{"PROJECT_JOURNAL_QUANTITY"})
}
