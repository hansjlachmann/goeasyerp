package migrations

import (
	fmigrations "github.com/hansjlachmann/goeasyerp/backend/foundation/migrations"
)

func init() {
	Register(&Migration024SalesTax{})
}

// Migration024SalesTax goes with the sales tax (tables 252 General Posting Setup, 254 Tax Entry,
// 319 Tax Area Line, 320 Tax Jurisdiction, 322 Tax Detail, 325 Tax Posting Setup). The READER
// role may read them, as all business tables; the tables come from the table sync.
type Migration024SalesTax struct{}

func (m *Migration024SalesTax) Version() int {
	return 24
}

func (m *Migration024SalesTax) Name() string {
	return "sales_tax"
}

func (m *Migration024SalesTax) Description() string {
	return "READER reads the sales tax setup, the general posting setup and the tax entries"
}

func (m *Migration024SalesTax) Up(ctx *fmigrations.Context) error {
	return grantReaderRead(ctx, salesTaxTables)
}

var salesTaxTables = []string{
	"GENERAL_POSTING_SETUP", "TAX_ENTRY", "TAX_AREA_LINE", "TAX_JURISDICTION", "TAX_DETAIL", "TAX_POSTING_SETUP",
}
