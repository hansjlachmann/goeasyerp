package salesposting_test

import (
	"database/sql"
	"errors"
	"testing"
	"time"

	"github.com/hansjlachmann/goeasyerp/backend/business-logic/demodata"
	"github.com/hansjlachmann/goeasyerp/backend/business-logic/itemjnl"
	"github.com/hansjlachmann/goeasyerp/backend/business-logic/salesposting"
	"github.com/hansjlachmann/goeasyerp/backend/business-logic/tables"
	"github.com/hansjlachmann/goeasyerp/backend/foundation/database"
	apperrors "github.com/hansjlachmann/goeasyerp/backend/foundation/errors"
	ftables "github.com/hansjlachmann/goeasyerp/backend/foundation/tables"
	gtables "github.com/hansjlachmann/goeasyerp/backend/generated/tables"
)

// isInconsistent: the commit was refused by the balance check (the G/L entries named)
func isInconsistent(err error) bool {
	var appErr *apperrors.AppError
	return errors.As(err, &appErr) && (appErr.Code == apperrors.ErrGLNotBalanced || appErr.Code == apperrors.ErrInconsistent)
}

// offByOneCent makes the customer entry 0.01 more than the document: the G/L entries of the
// posting do not balance (a rounding difference)
func offByOneCent(genJnlLine *gtables.GenJournalLine) {
	genJnlLine.Amount = genJnlLine.Amount.Add(dec("0.01"))
	genJnlLine.Amount_lcy = genJnlLine.Amount_lcy.Add(dec("0.01"))
}

// A sales invoice whose G/L entries do not balance is refused when the posting commits: no
// posted invoice, no entries, the invoice still there and the number series unchanged
func TestSalesPostingUnbalancedIsRefused(t *testing.T) {
	db := demoDB(t)
	src := ftables.DBSource(db, "demo", database.DBTypeSQLite)
	invoice := newDocument(t, src, gtables.SalesHeader_Document_type.Invoice, "C00020")
	addLine(t, invoice, 10000, gtables.SalesLine_Type.Item, "1000", "2")
	glEntries := gtables.NewGLEntry(src).Count()
	custEntries := gtables.NewCustomerLedgerEntry(src).Count()
	postedInvoices := gtables.NewSalesInvoiceHeader(src).Count()
	salesSetup := gtables.GetSalesReceivablesSetup(src)
	noSeriesLine := gtables.NewNoSeriesLine(src)
	noSeriesLine.SetRange("series_code", salesSetup.Posted_invoice_nos)
	noSeriesLine.FindFirst()
	lastNoUsed := noSeriesLine.Last_no_used

	removeHook := salesposting.SetCustomerEntryHook(offByOneCent)
	defer removeHook()
	tx, err := ftables.BeginTransaction(db)
	must(t, err)
	txSrc := ftables.DBSource(tx, "demo", database.DBTypeSQLite)
	salesHeader := gtables.NewSalesHeader(txSrc)
	must(t, salesHeader.GetOrError(map[string]interface{}{"document_type": int(invoice.Document_type), "no": invoice.No}))
	err = salesposting.NewSalesPost(txSrc).Run(salesHeader)
	if err != nil {
		_ = ftables.Rollback(tx)
		t.Fatalf("the posting itself runs; the commit refuses it: %v", err)
	}
	err = ftables.Commit(tx)
	if !isInconsistent(err) {
		t.Fatalf("an unbalanced sales posting must be refused by the balance check, got %v", err)
	}

	if gtables.NewGLEntry(src).Count() != glEntries || gtables.NewCustomerLedgerEntry(src).Count() != custEntries ||
		gtables.NewSalesInvoiceHeader(src).Count() != postedInvoices {
		t.Error("entries or a posted invoice were saved")
	}
	if !reread(t, invoice) {
		t.Error("the invoice must still be there")
	}
	noSeriesLine.FindFirst()
	if noSeriesLine.Last_no_used != lastNoUsed {
		t.Errorf("the posted invoice series moved from %q to %q", lastNoUsed, noSeriesLine.Last_no_used)
	}
}

// Demo data posts many documents in one transaction: one posting that does not balance stops
// it, even when the postings after it balance, and nothing is saved
func TestDemoDataStopsOnUnbalancedPosting(t *testing.T) {
	removeHook := salesposting.SetCustomerEntryHook(func() func(*gtables.GenJournalLine) {
		first := true
		return func(genJnlLine *gtables.GenJournalLine) {
			if first {
				offByOneCent(genJnlLine)
				first = false
			}
		}
	}())
	defer removeHook()
	db := openDB(t)
	_, err := demodata.Create(db, "demo", database.DBTypeSQLite, demodata.Options{
		Today:           time.Date(2026, 10, 6, 0, 0, 0, 0, time.Local),
		ItemJnlPostLine: func(src ftables.Source) demodata.ItemJnlPoster { return itemjnl.NewItemJnlPostLine(src) },
		SalesPost:       func(src ftables.Source) demodata.SalesPoster { return salesposting.NewSalesPost(src) },
	})
	if !isInconsistent(err) {
		t.Fatalf("demo data with an unbalanced posting must be refused, got %v", err)
	}
	src := ftables.DBSource(db, "demo", database.DBTypeSQLite)
	if n := gtables.NewCustomer(src).Count(); n != 0 {
		t.Errorf("%d customers saved", n)
	}
	if n := gtables.NewGLEntry(src).Count(); n != 0 {
		t.Errorf("%d G/L entries saved", n)
	}
}

// openDB: an empty company
func openDB(t *testing.T) *sql.DB {
	t.Helper()
	db, err := sql.Open("sqlite3", ":memory:")
	must(t, err)
	db.SetMaxOpenConns(1)
	t.Cleanup(func() { _ = db.Close() })
	must(t, tables.CreateCompanyTables(db, "demo", database.DBTypeSQLite))
	return db
}
