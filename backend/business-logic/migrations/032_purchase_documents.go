package migrations

import (
	fmigrations "github.com/hansjlachmann/goeasyerp/backend/foundation/migrations"
)

func init() {
	Register(&Migration032PurchaseDocuments{})
}

// Migration032PurchaseDocuments goes with the purchase documents: Purchase Header 38, Purchase Line 39, Vendor
// Invoice Disc. 24 and Purch. Comment Line 43. The READER role may read them, as all business
// tables; the tables come from the table sync.
type Migration032PurchaseDocuments struct{}

func (m *Migration032PurchaseDocuments) Version() int {
	return 32
}

func (m *Migration032PurchaseDocuments) Name() string {
	return "purchase-documents"
}

func (m *Migration032PurchaseDocuments) Description() string {
	return "READER reads the purchase documents, their lines and comments and the vendor invoice discounts"
}

func (m *Migration032PurchaseDocuments) Up(ctx *fmigrations.Context) error {
	return grantReaderRead(ctx, []string{"PURCHASE_HEADER", "PURCHASE_LINE", "VENDOR_INVOICE_DISC", "PURCH_COMMENT_LINE"})
}
