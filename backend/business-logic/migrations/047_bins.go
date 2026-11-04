package migrations

import (
	fmigrations "github.com/hansjlachmann/goeasyerp/backend/foundation/migrations"
)

func init() {
	Register(&Migration047Bins{})
}

// Migration047Bins goes with bins: Bin 5400. The READER role may read it, as all business
// tables; the table comes from the table sync.
type Migration047Bins struct{}

func (m *Migration047Bins) Version() int {
	return 47
}

func (m *Migration047Bins) Name() string {
	return "bins"
}

func (m *Migration047Bins) Description() string {
	return "READER reads the bins"
}

func (m *Migration047Bins) Up(ctx *fmigrations.Context) error {
	return grantReaderRead(ctx, []string{"BIN"})
}
