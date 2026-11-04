package purchposting_test

import (
	"errors"
	"testing"

	"github.com/hansjlachmann/goeasyerp/backend/business-logic/purchposting"
	"github.com/hansjlachmann/goeasyerp/backend/foundation/database"
	apperrors "github.com/hansjlachmann/goeasyerp/backend/foundation/errors"
	ftables "github.com/hansjlachmann/goeasyerp/backend/foundation/tables"
	"github.com/hansjlachmann/goeasyerp/backend/foundation/types"
	gtables "github.com/hansjlachmann/goeasyerp/backend/generated/tables"
)

// A purchase invoice whose G/L entries do not balance (the vendor entry 0.01 off) is refused
// when the posting commits: no posted invoice, no entries, the invoice still there
func TestPurchPostingUnbalancedIsRefused(t *testing.T) {
	db := demoDB(t)
	src := ftables.DBSource(db, "demo", database.DBTypeSQLite)
	invoice := newDocument(t, src, gtables.PurchaseHeader_Document_type.Invoice, "V00010")
	addLine(t, invoice, 10000, gtables.PurchaseLine_Type.Item, "1000", "2")
	must(t, invoice.Validate("vendor_invoice_no", types.Code("V-1")))
	must(t, invoice.ModifyOrError(true))
	glEntries := gtables.NewGLEntry(src).Count()
	vendEntries := gtables.NewVendorLedgerEntry(src).Count()
	postedInvoices := gtables.NewPurchInvHeader(src).Count()

	removeHook := purchposting.SetVendorEntryHook(func(genJnlLine *gtables.GenJournalLine) {
		genJnlLine.Amount = genJnlLine.Amount.Add(dec("0.01"))
		genJnlLine.Amount_lcy = genJnlLine.Amount_lcy.Add(dec("0.01"))
	})
	defer removeHook()
	tx, err := ftables.BeginTransaction(db)
	must(t, err)
	txSrc := ftables.DBSource(tx, "demo", database.DBTypeSQLite)
	purchHeader := gtables.NewPurchaseHeader(txSrc)
	must(t, purchHeader.GetOrError(map[string]interface{}{"document_type": int(invoice.Document_type), "no": invoice.No}))
	err = purchposting.NewPurchPost(txSrc).Run(purchHeader)
	if err != nil {
		_ = ftables.Rollback(tx)
		t.Fatalf("the posting itself runs; the commit refuses it: %v", err)
	}
	err = ftables.Commit(tx)
	var appErr *apperrors.AppError
	if !errors.As(err, &appErr) || appErr.Code != apperrors.ErrGLNotBalanced && appErr.Code != apperrors.ErrInconsistent {
		t.Fatalf("an unbalanced purchase posting must be refused by the balance check, got %v", err)
	}
	if gtables.NewGLEntry(src).Count() != glEntries || gtables.NewVendorLedgerEntry(src).Count() != vendEntries ||
		gtables.NewPurchInvHeader(src).Count() != postedInvoices {
		t.Error("entries or a posted invoice were saved")
	}
	if !reread(t, invoice) {
		t.Error("the invoice must still be there")
	}
}
