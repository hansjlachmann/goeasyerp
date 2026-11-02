package migrations

import (
	fmigrations "github.com/hansjlachmann/openerp/backend/foundation/migrations"
)

func init() {
	Register(&Migration038TaxStatements{})
}

// Migration038TaxStatements goes with tax statements: Tax Statement Template 255, Line 256 and
// Name 257. The READER role may read them, as all business tables; the tables come from the
// table sync.
type Migration038TaxStatements struct{}

func (m *Migration038TaxStatements) Version() int {
	return 38
}

func (m *Migration038TaxStatements) Name() string {
	return "tax-statements"
}

func (m *Migration038TaxStatements) Description() string {
	return "READER reads the tax statement templates, names and lines"
}

func (m *Migration038TaxStatements) Up(ctx *fmigrations.Context) error {
	return grantReaderRead(ctx, []string{"TAX_STATEMENT_TEMPLATE", "TAX_STATEMENT_LINE", "TAX_STATEMENT_NAME"})
}
