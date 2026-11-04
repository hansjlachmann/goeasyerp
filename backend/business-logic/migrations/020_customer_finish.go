package migrations

import (
	fmigrations "github.com/hansjlachmann/goeasyerp/backend/foundation/migrations"
)

func init() {
	Register(&Migration020CustomerFinish{})
}

// Migration020CustomerFinish goes with the tables that finish the customer: accounting periods
// (50), comment lines (97), ship-to addresses (222) and customer bank accounts (287). The
// READER role may read them, as all business tables; the tables come from the table sync.
type Migration020CustomerFinish struct{}

func (m *Migration020CustomerFinish) Version() int {
	return 20
}

func (m *Migration020CustomerFinish) Name() string {
	return "customer_finish"
}

func (m *Migration020CustomerFinish) Description() string {
	return "READER reads the accounting periods, comment lines, ship-to addresses and customer bank accounts"
}

func (m *Migration020CustomerFinish) Up(ctx *fmigrations.Context) error {
	return grantReaderRead(ctx, customerFinishTables)
}

var customerFinishTables = []string{"ACCOUNTING_PERIOD", "COMMENT_LINE", "SHIP_TO_ADDRESS", "CUSTOMER_BANK_ACCOUNT"}
