package migrations

import (
	fmigrations "github.com/hansjlachmann/goeasyerp/backend/foundation/migrations"
)

func init() {
	Register(&Migration027Navigate{})
}

// Migration027Navigate goes with page 344 Navigate: its lines are Document Entry (table 265),
// a temporary table the list reads through the table API, so READER needs read on it like on
// every business table; the table comes from the table sync.
type Migration027Navigate struct{}

func (m *Migration027Navigate) Version() int {
	return 27
}

func (m *Migration027Navigate) Name() string {
	return "navigate"
}

func (m *Migration027Navigate) Description() string {
	return "READER reads Document Entry, the lines of Navigate"
}

func (m *Migration027Navigate) Up(ctx *fmigrations.Context) error {
	return grantReaderRead(ctx, []string{"DOCUMENT_ENTRY"})
}
