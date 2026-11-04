package postingpreview_test

import (
	"database/sql"
	"errors"
	"fmt"
	"testing"
	"time"

	_ "github.com/mattn/go-sqlite3"

	"github.com/hansjlachmann/goeasyerp/backend/business-logic/bomjnl"
	"github.com/hansjlachmann/goeasyerp/backend/business-logic/demodata"
	"github.com/hansjlachmann/goeasyerp/backend/business-logic/genjnl"
	"github.com/hansjlachmann/goeasyerp/backend/business-logic/itemjnl"
	"github.com/hansjlachmann/goeasyerp/backend/business-logic/postingpreview"
	"github.com/hansjlachmann/goeasyerp/backend/business-logic/salesposting"
	"github.com/hansjlachmann/goeasyerp/backend/business-logic/tables"
	fcodeunits "github.com/hansjlachmann/goeasyerp/backend/foundation/codeunits"
	"github.com/hansjlachmann/goeasyerp/backend/foundation/database"
	apperrors "github.com/hansjlachmann/goeasyerp/backend/foundation/errors"
	ftables "github.com/hansjlachmann/goeasyerp/backend/foundation/tables"
	"github.com/hansjlachmann/goeasyerp/backend/foundation/types"
	gtables "github.com/hansjlachmann/goeasyerp/backend/generated/tables"
)

func must(t *testing.T, err error) {
	t.Helper()
	if err != nil {
		t.Fatal(err)
	}
}

func demoDB(t *testing.T) *sql.DB {
	t.Helper()
	db, err := sql.Open("sqlite3", ":memory:")
	must(t, err)
	db.SetMaxOpenConns(1)
	t.Cleanup(func() { _ = db.Close() })
	must(t, tables.CreateCompanyTables(db, "demo", database.DBTypeSQLite))
	_, err = demodata.Create(db, "demo", database.DBTypeSQLite, demodata.Options{
		Today:           time.Date(2026, 10, 6, 0, 0, 0, 0, time.Local),
		ItemJnlPostLine: func(src ftables.Source) demodata.ItemJnlPoster { return itemjnl.NewItemJnlPostLine(src) },
	})
	must(t, err)
	return db
}

// A sales invoice previewed: the preview holds the G/L, customer, tax and item entries the
// posting writes; nothing is saved (no entries, the invoice is there, the series unchanged);
// the preview is the user's only
func TestPreviewSalesInvoice(t *testing.T) {
	db := demoDB(t)
	src := ftables.DBSource(db, "demo", database.DBTypeSQLite)
	invoice := gtables.NewSalesHeader(src)
	invoice.Document_type = gtables.SalesHeader_Document_type.Invoice
	must(t, invoice.InsertOrError(true))
	must(t, invoice.Validate("sell_to_customer_no", types.Code("C00020")))
	must(t, invoice.ModifyOrError(true))
	salesLine := gtables.NewSalesLine(src)
	salesLine.Document_type = gtables.SalesLine_Document_type.Invoice
	salesLine.Document_no = invoice.No
	salesLine.Line_no = 10000
	must(t, salesLine.Validate("type", gtables.SalesLine_Type.Item))
	must(t, salesLine.Validate("no", types.Code("1000")))
	must(t, salesLine.Validate("quantity", types.MustDecimal("2")))
	must(t, salesLine.InsertOrError(true))
	glEntries := gtables.NewGLEntry(src).Count()

	preview, err := postingpreview.Run(db, "demo", database.DBTypeSQLite, "ADMIN", func(txSrc ftables.Source) error {
		salesHeader := gtables.NewSalesHeader(txSrc)
		err := salesHeader.GetOrError(map[string]interface{}{"document_type": int(invoice.Document_type), "no": invoice.No})
		if err != nil {
			return err
		}
		return salesposting.NewSalesPost(txSrc).Run(salesHeader)
	})
	must(t, err)
	if preview.Inconsistency != nil {
		t.Fatalf("a balanced posting: %v", preview.Inconsistency)
	}
	found := map[string]int{}
	for _, entries := range preview.Tables {
		found[entries.Table.Name] = len(entries.Rows)
	}
	if found[gtables.GLEntryTableName] == 0 || found["Customer_ledger_entry"] != 1 || found[gtables.ItemLedgerEntryTableName] != 1 || found[gtables.TaxEntryTableName] == 0 {
		t.Errorf("preview tables %v", found)
	}
	total := types.Decimal{}
	for _, row := range preview.Rows(gtables.GLEntryTableName) {
		total = total.Add(types.MustDecimal(fmt.Sprint(row["amount"])))
	}
	if !total.IsZero() {
		t.Errorf("previewed G/L entries add up to %s", total)
	}

	if gtables.NewGLEntry(src).Count() != glEntries {
		t.Error("a preview saved G/L entries")
	}
	again := gtables.NewSalesHeader(src)
	if !again.Get(map[string]interface{}{"document_type": int(invoice.Document_type), "no": invoice.No}) {
		t.Error("the invoice is gone after a preview")
	}

	if _, ok := postingpreview.Get(preview.ID, "ADMIN", "demo"); !ok {
		t.Error("the user's preview is kept")
	}
	if _, ok := postingpreview.Get(preview.ID, "OTHER", "demo"); ok {
		t.Error("another user must not get the preview")
	}
	if _, ok := postingpreview.Get(preview.ID, "ADMIN", "other"); ok {
		t.Error("another company must not get the preview")
	}
}

// A posting whose G/L entries do not balance is previewed with the balance check's error; an
// error of the posting itself is returned
func TestPreviewUnbalancedAndFailing(t *testing.T) {
	db := demoDB(t)
	preview, err := postingpreview.Run(db, "demo", database.DBTypeSQLite, "ADMIN", func(src ftables.Source) error {
		line := gtables.NewGenJournalLine(src)
		line.Journal_template_name = "GENERAL"
		line.Journal_batch_name = "DEFAULT"
		line.Line_no = 10000
		line.Posting_date = types.MustDate("2026-10-15")
		line.Document_no = "PREVIEW1"
		line.Account_no = "1210"
		line.Amount = types.MustDecimal("100")
		line.Amount_lcy = types.MustDecimal("100")
		return genjnl.NewGenJnlPostLine(src).RunWithoutCheck(line)
	})
	must(t, err)
	var appErr *apperrors.AppError
	if !errors.As(preview.Inconsistency, &appErr) || appErr.Code != apperrors.ErrGLNotBalanced || appErr.Details == nil {
		t.Errorf("unbalanced preview: %v", preview.Inconsistency)
	}
	if len(preview.Rows(gtables.GLEntryTableName)) != 1 {
		t.Errorf("the unbalanced entry is previewed: %d", len(preview.Rows(gtables.GLEntryTableName)))
	}
	if _, err := preview.Result(); err == nil {
		t.Error("the result of an unbalanced preview is the balance check's error")
	}

	_, err = postingpreview.Run(db, "demo", database.DBTypeSQLite, "ADMIN", func(src ftables.Source) error {
		return errors.New("the posting failed")
	})
	if err == nil || err.Error() != "the posting failed" {
		t.Errorf("a failing posting: %v", err)
	}
}

// Preview Posting on the item and BOM journals: the batch is posted in the preview — item and
// BOM ledger entries collected —, the journal lines stay and no entry is saved
func TestPreviewItemAndBOMJournals(t *testing.T) {
	db := demoDB(t)
	src := ftables.DBSource(db, "demo", database.DBTypeSQLite)
	itemEntries := gtables.NewItemLedgerEntry(src).Count()

	itemJnlLine := gtables.NewItemJournalLine(src)
	itemJnlLine.InitRecord()
	itemJnlLine.Journal_template_name, itemJnlLine.Journal_batch_name, itemJnlLine.Line_no = "ITEM", "DEFAULT", 10000
	must(t, itemJnlLine.Validate("posting_date", types.NewDate(2026, 10, 6)))
	itemJnlLine.Document_no = "T00001" // the batch's series
	must(t, itemJnlLine.Validate("entry_type", gtables.ItemJournalLine_Entry_type.PositiveAdjmt))
	must(t, itemJnlLine.Validate("item_no", types.Code("1000")))
	must(t, itemJnlLine.Validate("location_code", types.Code("MAIN")))
	must(t, itemJnlLine.Validate("quantity", types.MustDecimal("4")))
	must(t, itemJnlLine.InsertOrError(true))
	checkPreview(t, itemjnl.ItemJnlPostPreview, itemJnlLine, gtables.ItemLedgerEntryTableName)

	bomJnlLine := gtables.NewBOMJournalLine(src)
	bomJnlLine.InitRecord()
	bomJnlLine.Journal_template_name, bomJnlLine.Journal_batch_name, bomJnlLine.Line_no = "BOM", "DEFAULT", 10000
	must(t, bomJnlLine.Validate("posting_date", types.NewDate(2026, 10, 6)))
	bomJnlLine.Document_no = "B00001"
	must(t, bomJnlLine.Validate("item_no", types.Code("2100")))
	must(t, bomJnlLine.Validate("location_code", types.Code("MAIN")))
	must(t, bomJnlLine.Validate("quantity", types.MustDecimal("1")))
	must(t, bomJnlLine.InsertOrError(true))
	checkPreview(t, bomjnl.BOMJnlPostPreview, bomJnlLine, gtables.BOMLedgerEntryTableName)

	if n := gtables.NewItemLedgerEntry(src).Count(); n != itemEntries {
		t.Errorf("%d item ledger entries saved by the previews", n-itemEntries)
	}
	if !itemJnlLine.Get(map[string]interface{}{"journal_template_name": "ITEM", "journal_batch_name": "DEFAULT", "line_no": 10000}) ||
		!bomJnlLine.Get(map[string]interface{}{"journal_template_name": "BOM", "journal_batch_name": "DEFAULT", "line_no": 10000}) {
		t.Error("a previewed journal line is gone")
	}
}

// checkPreview runs a journal's Preview Posting and finds the table's entries in the preview
func checkPreview[T any](t *testing.T, preview func(T) (fcodeunits.Result, error), line T, table string) {
	t.Helper()
	result, err := preview(line)
	must(t, err)
	if result.OpenPage == nil || result.OpenPage.Params["preview"] == "" {
		t.Fatalf("%s: no Posting Preview opened: %+v", table, result)
	}
	p, ok := postingpreview.Get(result.OpenPage.Params["preview"], fcodeunits.CurrentUserID(), "demo")
	if !ok {
		t.Fatalf("%s: preview not kept", table)
	}
	for _, entries := range p.Tables {
		if entries.Table.Name == table && len(entries.Rows) > 0 {
			return
		}
	}
	t.Errorf("the preview has no %s entries", table)
}
