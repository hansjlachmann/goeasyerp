package migrations

import (
	fmigrations "github.com/hansjlachmann/goeasyerp/backend/foundation/migrations"
)

func init() {
	Register(&Migration035FixedAssetReclassInsurance{})
}

// Migration035FixedAssetReclassInsurance goes with the FA reclassification journal (FA Reclass.
// Journal Template/Batch/Line 5622–5624) and the insurance journal (Insurance Journal
// Template/Batch/Line 5633–5635, Insurance Register 5636). The READER role may read them, as all
// business tables; the tables come from the table sync.
type Migration035FixedAssetReclassInsurance struct{}

func (m *Migration035FixedAssetReclassInsurance) Version() int {
	return 35
}

func (m *Migration035FixedAssetReclassInsurance) Name() string {
	return "fixed-asset-reclass-insurance"
}

func (m *Migration035FixedAssetReclassInsurance) Description() string {
	return "READER reads the FA reclass. and insurance journals and the insurance registers"
}

func (m *Migration035FixedAssetReclassInsurance) Up(ctx *fmigrations.Context) error {
	return grantReaderRead(ctx, []string{"FA_RECLASS_JOURNAL_TEMPLATE", "FA_RECLASS_JOURNAL_BATCH", "FA_RECLASS_JOURNAL_LINE", "INSURANCE_JOURNAL_TEMPLATE", "INSURANCE_JOURNAL_BATCH", "INSURANCE_JOURNAL_LINE", "INSURANCE_REGISTER"})
}
