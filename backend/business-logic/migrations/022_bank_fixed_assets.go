package migrations

import (
	fmigrations "github.com/hansjlachmann/goeasyerp/backend/foundation/migrations"
)

func init() {
	Register(&Migration022BankFixedAssets{})
}

// Migration022BankFixedAssets goes with the bank accounts (tables 270–277: bank accounts, their
// ledger and check ledger entries, posting groups) and the fixed assets (5600–5640: fixed
// assets, FA setup, posting groups, classes, subclasses, locations, depreciation books, FA and
// maintenance ledger entries, maintenance, main asset components, insurance). The READER role
// may read them, as all business tables; the tables come from the table sync.
type Migration022BankFixedAssets struct{}

func (m *Migration022BankFixedAssets) Version() int {
	return 22
}

func (m *Migration022BankFixedAssets) Name() string {
	return "bank_fixed_assets"
}

func (m *Migration022BankFixedAssets) Description() string {
	return "READER reads the bank accounts, fixed assets, insurance and their ledger entries"
}

func (m *Migration022BankFixedAssets) Up(ctx *fmigrations.Context) error {
	return grantReaderRead(ctx, bankFixedAssetTables)
}

var bankFixedAssetTables = []string{
	"BANK_ACCOUNT", "BANK_ACCOUNT_LEDGER_ENTRY", "CHECK_LEDGER_ENTRY", "BANK_ACCOUNT_POSTING_GROUP",
	"FIXED_ASSET", "FA_LEDGER_ENTRY", "FA_SETUP", "FA_POSTING_GROUP", "FA_CLASS", "FA_SUBCLASS", "FA_LOCATION",
	"DEPRECIATION_BOOK", "FA_DEPRECIATION_BOOK", "MAINTENANCE_LEDGER_ENTRY", "MAINTENANCE", "MAINTENANCE_REGISTRATION",
	"MAIN_ASSET_COMPONENT", "INSURANCE", "INSURANCE_TYPE", "INS_COVERAGE_LEDGER_ENTRY",
}
