package migrations

import (
	fmigrations "github.com/hansjlachmann/goeasyerp/backend/foundation/migrations"
)

func init() {
	Register(&Migration044GLSetupConsolidation{})
}

// Migration044GLSetupConsolidation goes with the General Ledger setup and consolidation step:
// User Time Register 51, Setup Checklist Line 344, Setup Checklist Comment 345, Application
// Area Line 346 and Business Unit 220. The READER role may read them, as all business tables;
// the tables come from the table sync.
type Migration044GLSetupConsolidation struct{}

func (m *Migration044GLSetupConsolidation) Version() int {
	return 44
}

func (m *Migration044GLSetupConsolidation) Name() string {
	return "gl_setup_consolidation"
}

func (m *Migration044GLSetupConsolidation) Description() string {
	return "READER reads the user time registers, the setup checklist and the business units"
}

func (m *Migration044GLSetupConsolidation) Up(ctx *fmigrations.Context) error {
	return grantReaderRead(ctx, []string{"USER_TIME_REGISTER", "SETUP_CHECKLIST_LINE", "SETUP_CHECKLIST_COMMENT", "APPLICATION_AREA_LINE", "BUSINESS_UNIT"})
}
