package migrations

import (
	fmigrations "github.com/hansjlachmann/goeasyerp/backend/foundation/migrations"
)

func init() {
	Register(&Migration043DateCompression{})
}

// Migration043DateCompression goes with date compression: Date Compr. Register 87. The READER
// role may read it, as all business tables; the table comes from the table sync.
type Migration043DateCompression struct{}

func (m *Migration043DateCompression) Version() int {
	return 43
}

func (m *Migration043DateCompression) Name() string {
	return "date_compression"
}

func (m *Migration043DateCompression) Description() string {
	return "READER reads the date compression registers"
}

func (m *Migration043DateCompression) Up(ctx *fmigrations.Context) error {
	return grantReaderRead(ctx, []string{"DATE_COMPR_REGISTER"})
}
