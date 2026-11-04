package migrations

import (
	fmigrations "github.com/hansjlachmann/goeasyerp/backend/foundation/migrations"
)

func init() {
	Register(&Migration016GLAccount{})
}

// Migration016GLAccount goes with the G/L accounts (table 15): the READER role may read them,
// as all business tables. The table and the Currency G/L account columns come from the sync.
type Migration016GLAccount struct{}

func (m *Migration016GLAccount) Version() int {
	return 16
}

func (m *Migration016GLAccount) Name() string {
	return "gl_account"
}

func (m *Migration016GLAccount) Description() string {
	return "READER reads the G/L accounts"
}

func (m *Migration016GLAccount) Up(ctx *fmigrations.Context) error {
	return grantReaderRead(ctx, []string{"G_L_ACCOUNT"})
}
