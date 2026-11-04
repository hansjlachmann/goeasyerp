package migrations

import (
	fmigrations "github.com/hansjlachmann/goeasyerp/backend/foundation/migrations"
)

func init() {
	Register(&Migration034FixedAssetJournal{})
}

// Migration034FixedAssetJournal goes with the fixed asset journals and depreciation: FA Posting
// Type Setup 5604, FA Journal Setup 5605, FA Allocation 5615, FA Register 5617, FA Journal
// Template/Batch/Line 5619–5621, Depreciation Table Header/Line 5642/5643. The READER role may read
// them, as all business tables; the tables come from the table sync (which runs after the
// migrations, so the books entered before get their FA Posting Type Setup lines on first use).
type Migration034FixedAssetJournal struct{}

func (m *Migration034FixedAssetJournal) Version() int {
	return 34
}

func (m *Migration034FixedAssetJournal) Name() string {
	return "fixed-asset-journal"
}

func (m *Migration034FixedAssetJournal) Description() string {
	return "READER reads the FA journals, their setups, the FA allocations, the depreciation tables and the FA registers"
}

func (m *Migration034FixedAssetJournal) Up(ctx *fmigrations.Context) error {
	return grantReaderRead(ctx, []string{"FA_POSTING_TYPE_SETUP", "FA_JOURNAL_SETUP", "FA_ALLOCATION", "DEPRECIATION_TABLE_HEADER", "DEPRECIATION_TABLE_LINE", "FA_REGISTER", "FA_JOURNAL_TEMPLATE", "FA_JOURNAL_BATCH", "FA_JOURNAL_LINE"})
}
