package migrations

import (
	fmigrations "github.com/hansjlachmann/goeasyerp/backend/foundation/migrations"
)

func init() {
	Register(&Migration031SalesPosting{})
}

// Migration031SalesPosting goes with sales posting: the posted documents — Sales Shipment
// Header/Line 110/111, Sales Invoice Header/Line 112/113, Sales Cr.Memo Header/Line 114/115.
// The READER role may read them, as all business tables; the tables come from the table sync.
type Migration031SalesPosting struct{}

func (m *Migration031SalesPosting) Version() int {
	return 31
}

func (m *Migration031SalesPosting) Name() string {
	return "sales-posting"
}

func (m *Migration031SalesPosting) Description() string {
	return "READER reads the posted sales shipments, invoices and credit memos"
}

func (m *Migration031SalesPosting) Up(ctx *fmigrations.Context) error {
	return grantReaderRead(ctx, []string{"SALES_SHIPMENT_HEADER", "SALES_SHIPMENT_LINE", "SALES_INVOICE_HEADER", "SALES_INVOICE_LINE", "SALES_CR_MEMO_HEADER", "SALES_CR_MEMO_LINE"})
}
