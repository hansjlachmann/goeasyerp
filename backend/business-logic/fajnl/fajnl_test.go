package fajnl_test

import (
	"database/sql"
	"strings"
	"testing"
	"time"

	_ "github.com/mattn/go-sqlite3"

	"github.com/hansjlachmann/openerp/backend/business-logic/demodata"
	"github.com/hansjlachmann/openerp/backend/business-logic/fajnl"
	"github.com/hansjlachmann/openerp/backend/business-logic/tables"
	"github.com/hansjlachmann/openerp/backend/foundation/database"
	ftables "github.com/hansjlachmann/openerp/backend/foundation/tables"
	"github.com/hansjlachmann/openerp/backend/foundation/types"
	gtables "github.com/hansjlachmann/openerp/backend/generated/tables"
)

func must(t *testing.T, err error) {
	t.Helper()
	if err != nil {
		t.Fatal(err)
	}
}

func dec(s string) types.Decimal { return types.MustDecimal(s) }

func date(s string) types.Date {
	d, err := time.Parse("2006-01-02", s)
	if err != nil {
		panic(err)
	}
	return types.NewDateFromTime(d)
}

// demoDB: the demo company (fixed assets in the COMPANY book, posted to the G/L; a TAX book
// without G/L integration); FA0040 gets a TAX book
func demoDB(t *testing.T) *sql.DB {
	t.Helper()
	db, err := sql.Open("sqlite3", ":memory:")
	must(t, err)
	db.SetMaxOpenConns(1)
	t.Cleanup(func() { _ = db.Close() })
	must(t, tables.CreateCompanyTables(db, "demo", database.DBTypeSQLite))
	_, err = demodata.Create(db, "demo", database.DBTypeSQLite, demodata.Options{Today: time.Date(2026, 10, 6, 0, 0, 0, 0, time.Local)})
	must(t, err)
	src := ftables.DBSource(db, "demo", database.DBTypeSQLite)
	faDeprBook := gtables.NewFADepreciationBook(src)
	faDeprBook.Fa_no = "FA0040"
	faDeprBook.Depreciation_book_code = "TAX"
	must(t, faDeprBook.InsertOrError(true))
	must(t, faDeprBook.Validate("fa_posting_group", types.Code("EQUIPMENT")))
	must(t, faDeprBook.ModifyOrError(true))
	return db
}

// inTransaction: the demo company in a transaction; the returned function rolls it back (the
// database has one connection: roll back before the next transaction)
func inTransaction(t *testing.T, db *sql.DB) (ftables.Source, func()) {
	t.Helper()
	tx, err := ftables.BeginTransaction(db)
	must(t, err)
	return ftables.DBSource(tx, "demo", database.DBTypeSQLite), func() { _ = ftables.Rollback(tx) }
}

// journal: the demo's FA journal template ASSETS and its batch DEFAULT
func journal(t *testing.T, src ftables.Source) (*gtables.FAJournalTemplate, *gtables.FAJournalBatch) {
	t.Helper()
	tpl := gtables.NewFAJournalTemplate(src)
	must(t, tpl.GetOrError(types.Code("ASSETS")))
	batch := gtables.NewFAJournalBatch(src)
	must(t, batch.GetOrError(map[string]interface{}{"journal_template_name": "ASSETS", "name": "DEFAULT"}))
	return tpl, batch
}

// addLine: a journal line for FA0040 in the TAX book
func addLine(t *testing.T, batch *gtables.FAJournalBatch, lineNo int, postingType gtables.FAJournalLineFa_posting_type, postingDate, amount string) *gtables.FAJournalLine {
	t.Helper()
	line := gtables.NewFAJournalLine(batch)
	line.Journal_template_name = batch.Journal_template_name
	line.Journal_batch_name = batch.Name
	line.Line_no = lineNo
	line.Depreciation_book_code = "TAX"
	must(t, line.Validate("fa_no", types.Code("FA0040")))
	must(t, line.Validate("fa_posting_type", postingType))
	line.Fa_posting_date = date(postingDate)
	line.Document_no = "FA-TAX-1"
	must(t, line.Validate("amount", dec(amount)))
	must(t, line.InsertOrError(true))
	return line
}

func postBatch(t *testing.T, src ftables.Source, batch *gtables.FAJournalBatch) (fajnl.PostBatchResult, error) {
	t.Helper()
	line := gtables.NewFAJournalLine(src)
	line.Journal_template_name = batch.Journal_template_name
	line.Journal_batch_name = batch.Name
	return fajnl.NewFAJnlPostBatch(src).Run(line)
}

// faDeprBook: FA0040's TAX book with its FlowFields
func faDeprBook(t *testing.T, src ftables.Source) *gtables.FADepreciationBook {
	t.Helper()
	b := gtables.NewFADepreciationBook(src)
	must(t, b.GetOrError(map[string]interface{}{"fa_no": "FA0040", "depreciation_book_code": "TAX"}))
	b.CalcFields("acquisition_cost", "depreciation", "write_down", "salvage_value", "book_value", "depreciable_basis", "maintenance")
	return b
}

func wantDec(t *testing.T, what string, got types.Decimal, want string) {
	t.Helper()
	if !got.Equal(dec(want)) {
		t.Errorf("%s = %s, want %s", what, got, want)
	}
}

// An acquisition with a salvage value, a depreciation, a write-down and a maintenance posted in
// the TAX book: the FA and maintenance ledger entries, the book's amounts and dates, the FA
// register, the posted lines deleted; the G/L is not touched
func TestPostFAJournal(t *testing.T) {
	src, rollback := inTransaction(t, demoDB(t))
	defer rollback()
	_, batch := journal(t, src)
	glEntries := gtables.NewGLEntry(src).Count()

	acq := addLine(t, batch, 10000, gtables.FAJournalLine_Fa_posting_type.AcquisitionCost, "2026-01-02", "100000")
	acq.Salvage_value = dec("-10000")
	must(t, acq.ModifyOrError(true))
	if acq.Description != "Pallet racking" || acq.Fa_posting_group != "EQUIPMENT" || acq.Debit_amount.IsZero() {
		t.Errorf("line from the asset: %q %q debit %s", acq.Description, acq.Fa_posting_group, acq.Debit_amount)
	}
	addLine(t, batch, 20000, gtables.FAJournalLine_Fa_posting_type.Depreciation, "2026-06-30", "-9000")
	addLine(t, batch, 30000, gtables.FAJournalLine_Fa_posting_type.WriteDown, "2026-06-30", "-5000")
	maint := addLine(t, batch, 40000, gtables.FAJournalLine_Fa_posting_type.Maintenance, "2026-03-15", "2500")
	must(t, maint.Validate("maintenance_code", types.Code("SERVICE")))
	must(t, maint.ModifyOrError(true))

	result, err := postBatch(t, src, batch)
	must(t, err)
	if result.RegisterNo == 0 {
		t.Fatal("nothing posted")
	}
	b := faDeprBook(t, src)
	wantDec(t, "Acquisition Cost", b.Acquisition_cost, "100000")
	wantDec(t, "Depreciation", b.Depreciation, "-9000")
	wantDec(t, "Write-Down", b.Write_down, "-5000")
	wantDec(t, "Salvage Value", b.Salvage_value, "-10000")
	wantDec(t, "Book Value", b.Book_value, "86000") // write-down is part of the book value
	wantDec(t, "Depreciable Basis", b.Depreciable_basis, "90000")
	wantDec(t, "Maintenance", b.Maintenance, "2500")
	if !b.Acquisition_date.Equal(date("2026-01-02")) || !b.Last_depreciation_date.Equal(date("2026-06-30")) ||
		!b.Last_write_down_date.Equal(date("2026-06-30")) || !b.Last_salvage_value_date.Equal(date("2026-01-02")) ||
		!b.Last_maintenance_date.Equal(date("2026-03-15")) {
		t.Errorf("book dates: acq %s depr %s write-down %s salvage %s maint %s", b.Acquisition_date, b.Last_depreciation_date,
			b.Last_write_down_date, b.Last_salvage_value_date, b.Last_maintenance_date)
	}

	faReg := gtables.NewFARegister(src)
	must(t, faReg.GetOrError(result.RegisterNo))
	if faReg.Journal_type != gtables.FARegister_Journal_type.FixedAsset || faReg.Source_code != "FAJNL" || faReg.To_entry_no-faReg.From_entry_no != 3 ||
		faReg.From_maintenance_entry_no == 0 || faReg.From_maintenance_entry_no != faReg.To_maintenance_entry_no {
		t.Errorf("register: type %v entries %d..%d maintenance %d..%d", faReg.Journal_type, faReg.From_entry_no, faReg.To_entry_no,
			faReg.From_maintenance_entry_no, faReg.To_maintenance_entry_no)
	}
	salvage := gtables.NewFALedgerEntry(src)
	must(t, salvage.GetOrError(faReg.From_entry_no+1))
	if salvage.Fa_posting_type != gtables.FALedgerEntry_Fa_posting_type.SalvageValue || !salvage.Automatic_entry || salvage.Part_of_book_value {
		t.Errorf("salvage entry: type %v automatic %v book value %v", salvage.Fa_posting_type, salvage.Automatic_entry, salvage.Part_of_book_value)
	}
	if gtables.NewGLEntry(src).Count() != glEntries {
		t.Error("the FA journal posted to the G/L")
	}
	lines := gtables.NewFAJournalLine(src)
	lines.SetRange("journal_batch_name", batch.Name)
	lines.SetFilter("fa_no", "<>''")
	if lines.Count() != 0 {
		t.Error("the posted lines are still there")
	}
}

// The checks: a posting type the book posts to the G/L belongs in the general journal; the first
// entry must be an acquisition cost (also for a disposal); depreciation must not become positive;
// a cancelling line takes the entry it cancels off the asset
func TestFAJournalChecks(t *testing.T) {
	db := demoDB(t)
	// each case in a transaction of its own (the posting codeunit rolls a refused posting back)
	expectError := func(text string, lines func(batch *gtables.FAJournalBatch)) {
		t.Helper()
		src, rollback := inTransaction(t, db)
		defer rollback()
		_, batch := journal(t, src)
		lines(batch)
		_, err := postBatch(t, src, batch)
		if err == nil || !strings.Contains(err.Error(), text) {
			t.Errorf("want an error with %q, got %v", text, err)
		}
	}
	expectError("must be posted in the general journal", func(batch *gtables.FAJournalBatch) {
		line := addLine(t, batch, 10000, gtables.FAJournalLine_Fa_posting_type.AcquisitionCost, "2026-01-02", "1000")
		line.Depreciation_book_code = "COMPANY"
		must(t, line.ModifyOrError(true))
	})
	expectError("first entry", func(batch *gtables.FAJournalBatch) {
		addLine(t, batch, 10000, gtables.FAJournalLine_Fa_posting_type.WriteDown, "2026-01-02", "-1000")
	})
	expectError("must be an acquisition cost", func(batch *gtables.FAJournalBatch) {
		// the disposal of an asset not acquired
		addLine(t, batch, 10000, gtables.FAJournalLine_Fa_posting_type.Disposal, "2026-01-02", "-1000")
	})
	expectError("must not be positive", func(batch *gtables.FAJournalBatch) {
		addLine(t, batch, 10000, gtables.FAJournalLine_Fa_posting_type.AcquisitionCost, "2026-01-02", "1000")
		addLine(t, batch, 20000, gtables.FAJournalLine_Fa_posting_type.Depreciation, "2026-02-01", "500")
	})

	// cancel an acquisition: the line with the opposite amount and the entry's number
	src, rollback := inTransaction(t, db)
	defer rollback()
	_, batch := journal(t, src)
	addLine(t, batch, 10000, gtables.FAJournalLine_Fa_posting_type.AcquisitionCost, "2026-01-02", "1000")
	result, err := postBatch(t, src, batch)
	must(t, err)
	faReg := gtables.NewFARegister(src)
	must(t, faReg.GetOrError(result.RegisterNo))
	cancel := addLine(t, batch, 20000, gtables.FAJournalLine_Fa_posting_type.AcquisitionCost, "2026-01-02", "-1000")
	cancel.Fa_error_entry_no = faReg.From_entry_no
	must(t, cancel.ModifyOrError(true))
	_, err = postBatch(t, src, batch)
	must(t, err)
	original := gtables.NewFALedgerEntry(src)
	must(t, original.GetOrError(faReg.From_entry_no))
	if original.Fa_no != "" || original.Canceled_from_fa_no != "FA0040" {
		t.Errorf("cancelled entry: FA No. %q, canceled from %q", original.Fa_no, original.Canceled_from_fa_no)
	}
	wantDec(t, "acquisition cost after the cancellation", faDeprBook(t, src).Acquisition_cost, "0")
}
