package migrations

import (
	fmigrations "github.com/hansjlachmann/goeasyerp/backend/foundation/migrations"
)

func init() {
	Register(&Migration033PostedPurchaseDocuments{})
}

// Migration033PostedPurchaseDocuments goes with the purchase posting: Purch. Rcpt. Header 120
// and Line 121, Purch. Inv. Header 122 and Line 123, Purch. Cr. Memo Hdr. 124 and Line 125. The
// READER role may read them, as all business tables; the tables come from the table sync.
type Migration033PostedPurchaseDocuments struct{}

func (m *Migration033PostedPurchaseDocuments) Version() int {
	return 33
}

func (m *Migration033PostedPurchaseDocuments) Name() string {
	return "posted-purchase-documents"
}

func (m *Migration033PostedPurchaseDocuments) Description() string {
	return "READER reads the posted purchase receipts, invoices and credit memos and their lines"
}

func (m *Migration033PostedPurchaseDocuments) Up(ctx *fmigrations.Context) error {
	return grantReaderRead(ctx, []string{"PURCH_RCPT_HEADER", "PURCH_RCPT_LINE", "PURCH_INV_HEADER", "PURCH_INV_LINE", "PURCH_CR_MEMO_HDR", "PURCH_CR_MEMO_LINE"})
}
