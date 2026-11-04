package migrations

import (
	fmigrations "github.com/hansjlachmann/goeasyerp/backend/foundation/migrations"
)

func init() {
	Register(&Migration023GeneralJournal{})
}

// Migration023GeneralJournal goes with the general journals and posting (tables 80 Gen. Journal
// Template, 81 Gen. Journal Line, 232 Gen. Journal Batch, 45 G/L Register, 242 Source Code
// Setup, 91 User Setup). The READER role may read them, as all business tables; the tables come
// from the table sync.
type Migration023GeneralJournal struct{}

func (m *Migration023GeneralJournal) Version() int {
	return 23
}

func (m *Migration023GeneralJournal) Name() string {
	return "general_journal"
}

func (m *Migration023GeneralJournal) Description() string {
	return "READER reads the general journals, G/L registers, source code setup and user setup"
}

func (m *Migration023GeneralJournal) Up(ctx *fmigrations.Context) error {
	return grantReaderRead(ctx, generalJournalTables)
}

var generalJournalTables = []string{
	"GEN_JOURNAL_TEMPLATE", "GEN_JOURNAL_BATCH", "GEN_JOURNAL_LINE", "G_L_REGISTER", "SOURCE_CODE_SETUP", "USER_SETUP",
}
