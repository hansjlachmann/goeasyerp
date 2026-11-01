package purchposting_test

import (
	"database/sql"
	"testing"
	"time"

	"github.com/hansjlachmann/openerp/backend/business-logic/demodata"
	"github.com/hansjlachmann/openerp/backend/business-logic/itemjnl"
	"github.com/hansjlachmann/openerp/backend/business-logic/purchposting"
	"github.com/hansjlachmann/openerp/backend/business-logic/tables"
	"github.com/hansjlachmann/openerp/backend/foundation/database"
	ftables "github.com/hansjlachmann/openerp/backend/foundation/tables"
	gtables "github.com/hansjlachmann/openerp/backend/generated/tables"
)

// The demo purchase documents: posted through codeunit 90 — receipts, invoices, a credit memo —
// and open quotes, orders and an invoice; the G/L stays balanced
func TestDemoPurchases(t *testing.T) {
	db, err := sql.Open("sqlite3", ":memory:")
	must(t, err)
	db.SetMaxOpenConns(1)
	t.Cleanup(func() { _ = db.Close() })
	must(t, tables.CreateCompanyTables(db, "demo", database.DBTypeSQLite))
	var percents []int
	phases := map[demodata.Phase]bool{}
	counts, err := demodata.Create(db, "demo", database.DBTypeSQLite, demodata.Options{
		Progress: func(pct int, phase demodata.Phase) {
			percents = append(percents, pct)
			phases[phase] = true
		},
		Today:           time.Date(2026, 10, 6, 0, 0, 0, 0, time.Local),
		ItemJnlPostLine: func(src ftables.Source) demodata.ItemJnlPoster { return itemjnl.NewItemJnlPostLine(src) },
		PurchPost:       func(src ftables.Source) demodata.PurchPoster { return purchposting.NewPurchPost(src) },
	})
	must(t, err)
	if counts.PostedPurchInvoices < 9 || counts.PostedPurchReceipts < 9 || counts.PostedPurchCrMemos != 1 ||
		counts.PurchaseQuotes != 2 || counts.PurchaseOrders != 3 || counts.PurchaseInvoices != 1 {
		t.Errorf("counts %+v", counts)
	}
	for i, pct := range percents {
		if pct > 100 || i > 0 && pct < percents[i-1] {
			t.Fatalf("progress %v", percents)
		}
	}
	if !phases[demodata.PhasePurchases] || percents[len(percents)-1] != 100 {
		t.Errorf("progress ends at %d, phases %v", percents[len(percents)-1], phases)
	}
	src := ftables.DBSource(db, "demo", database.DBTypeSQLite)
	if n := gtables.NewPurchInvHeader(src).Count(); n != counts.PostedPurchInvoices {
		t.Errorf("posted invoices %d, counted %d", n, counts.PostedPurchInvoices)
	}
	if n := gtables.NewPurchRcptHeader(src).Count(); n != counts.PostedPurchReceipts {
		t.Errorf("posted receipts %d, counted %d", n, counts.PostedPurchReceipts)
	}
	if n := gtables.NewPurchaseHeader(src).Count(); n != counts.PurchaseQuotes+counts.PurchaseOrders+counts.PurchaseInvoices {
		t.Errorf("open documents %d", n)
	}
	glEntry := gtables.NewGLEntry(src)
	must(t, glEntry.CalcSums("amount"))
	if !glEntry.Amount.Round(2).IsZero() { // SQLite sums decimals as floating point
		t.Errorf("G/L balance %s", glEntry.Amount)
	}
	crMemo := gtables.NewPurchCrMemoHdr(src)
	if !crMemo.FindFirst() || crMemo.Applies_to_doc_no == "" {
		t.Errorf("credit memo applied to %q", crMemo.Applies_to_doc_no)
	}
	// the order received in part is still open with what is left
	purchLine := gtables.NewPurchaseLine(src)
	purchLine.SetRange("document_type", gtables.PurchaseLine_Document_type.Order)
	purchLine.SetFilter("quantity_received", "<>0")
	if purchLine.Count() == 0 {
		t.Error("no open order received in part")
	}
	// the vendor entries of the invoices are open, the credit memo closed against its invoice
	vendLedgEntry := gtables.NewVendorLedgerEntry(src)
	vendLedgEntry.SetRange("document_type", gtables.VendorLedgerEntry_Document_type.CreditMemo)
	if !vendLedgEntry.FindFirst() || vendLedgEntry.Open {
		t.Error("the credit memo's vendor entry is not applied")
	}
}
