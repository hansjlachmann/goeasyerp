package migrations

import (
	fmigrations "github.com/hansjlachmann/goeasyerp/backend/foundation/migrations"
)

func init() {
	Register(&Migration017GLEntry{})
}

// Migration017GLEntry goes with the G/L entries (table 17) and the source and reason codes
// (230, 231): the READER role may read them, as all business tables. The tables come from the
// table sync.
type Migration017GLEntry struct{}

func (m *Migration017GLEntry) Version() int {
	return 17
}

func (m *Migration017GLEntry) Name() string {
	return "gl_entry"
}

func (m *Migration017GLEntry) Description() string {
	return "READER reads the G/L entries, source codes and reason codes"
}

func (m *Migration017GLEntry) Up(ctx *fmigrations.Context) error {
	return grantReaderRead(ctx, []string{"G_L_ENTRY", "SOURCE_CODE", "REASON_CODE"})
}
