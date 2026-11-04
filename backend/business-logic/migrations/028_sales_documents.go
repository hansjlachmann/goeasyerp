package migrations

import (
	fmigrations "github.com/hansjlachmann/goeasyerp/backend/foundation/migrations"
)

func init() {
	Register(&Migration028SalesDocuments{})
}

// Migration028SalesDocuments goes with the sales documents: Sales Header 36, Sales Line 37, Cust.
// Invoice Disc. 19 and Sales Comment Line 44. The READER role may read them, as all business
// tables; the tables come from the table sync.
type Migration028SalesDocuments struct{}

func (m *Migration028SalesDocuments) Version() int {
	return 28
}

func (m *Migration028SalesDocuments) Name() string {
	return "sales-documents"
}

func (m *Migration028SalesDocuments) Description() string {
	return "READER reads the sales documents, their lines and comments and the customer invoice discounts"
}

func (m *Migration028SalesDocuments) Up(ctx *fmigrations.Context) error {
	return grantReaderRead(ctx, []string{"SALES_HEADER", "SALES_LINE", "CUST_INVOICE_DISC", "SALES_COMMENT_LINE"})
}
