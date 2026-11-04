package migrations

import (
	fmigrations "github.com/hansjlachmann/goeasyerp/backend/foundation/migrations"
)

func init() {
	Register(&Migration041Intrastat{})
}

// Migration041Intrastat goes with Intrastat: Transaction Type 258, Transport Method 259, Tariff
// Number 260, Entry/Exit Point 282, Area 284, Transaction Specification 285 and the Intrastat
// Jnl. Template/Batch/Line 261-263. The READER role may read them, as all business tables; the
// tables come from the table sync.
type Migration041Intrastat struct{}

func (m *Migration041Intrastat) Version() int {
	return 41
}

func (m *Migration041Intrastat) Name() string {
	return "intrastat"
}

func (m *Migration041Intrastat) Description() string {
	return "READER reads the Intrastat codes and journals"
}

func (m *Migration041Intrastat) Up(ctx *fmigrations.Context) error {
	return grantReaderRead(ctx, []string{"TRANSACTION_TYPE", "TRANSPORT_METHOD", "TARIFF_NUMBER", "ENTRY_EXIT_POINT", "AREA",
		"TRANSACTION_SPECIFICATION", "INTRASTAT_JNL_TEMPLATE", "INTRASTAT_JNL_BATCH", "INTRASTAT_JNL_LINE"})
}
