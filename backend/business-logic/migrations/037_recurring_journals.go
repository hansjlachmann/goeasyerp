package migrations

import (
	fmigrations "github.com/hansjlachmann/openerp/backend/foundation/migrations"
)

func init() {
	Register(&Migration037RecurringJournals{})
}

// Migration037RecurringJournals goes with recurring journals: Gen. Jnl. Allocation 221. The
// READER role may read it, as all business tables; the table comes from the table sync.
type Migration037RecurringJournals struct{}

func (m *Migration037RecurringJournals) Version() int {
	return 37
}

func (m *Migration037RecurringJournals) Name() string {
	return "recurring-journals"
}

func (m *Migration037RecurringJournals) Description() string {
	return "READER reads the general journal allocations"
}

func (m *Migration037RecurringJournals) Up(ctx *fmigrations.Context) error {
	return grantReaderRead(ctx, []string{"GEN_JNL_ALLOCATION"})
}
