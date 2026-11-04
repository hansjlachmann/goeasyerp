package migrations

import (
	fmigrations "github.com/hansjlachmann/goeasyerp/backend/foundation/migrations"
)

func init() {
	Register(&Migration021Vendor{})
}

// Migration021Vendor goes with the vendors (table 23) and the tables around them: vendor ledger
// entries (25), vendor posting groups (93), order addresses (224), vendor bank accounts (288)
// and the Purchases & Payables Setup (312). The READER role may read them, as all business
// tables; the tables come from the table sync.
type Migration021Vendor struct{}

func (m *Migration021Vendor) Version() int {
	return 21
}

func (m *Migration021Vendor) Name() string {
	return "vendor"
}

func (m *Migration021Vendor) Description() string {
	return "READER reads the vendors, their ledger entries, posting groups, order addresses, bank accounts and the purchases setup"
}

func (m *Migration021Vendor) Up(ctx *fmigrations.Context) error {
	return grantReaderRead(ctx, vendorTables)
}

var vendorTables = []string{"VENDOR", "VENDOR_LEDGER_ENTRY", "VENDOR_POSTING_GROUP", "ORDER_ADDRESS", "VENDOR_BANK_ACCOUNT", "PURCHASES_PAYABLES_SETUP"}
