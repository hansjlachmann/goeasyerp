package fajnl_test

import (
	"database/sql"
	"strconv"
	"testing"
	"time"

	"github.com/hansjlachmann/openerp/backend/business-logic/demodata"
	"github.com/hansjlachmann/openerp/backend/business-logic/genjnl"
	"github.com/hansjlachmann/openerp/backend/business-logic/glcheck"
	"github.com/hansjlachmann/openerp/backend/business-logic/tables"
	"github.com/hansjlachmann/openerp/backend/foundation/database"
	ftables "github.com/hansjlachmann/openerp/backend/foundation/tables"
	"github.com/hansjlachmann/openerp/backend/foundation/types"
	gtables "github.com/hansjlachmann/openerp/backend/generated/tables"
)

// The demo company with its fixed assets posted (Options.GenJnlPostLine): every FA entry has its
// G/L entry (but the Net disposal's proceeds), the acquisitions are paid, FA0040 is depreciated monthly as codeunit 5610 calculates
// it, the forklift FA0030 is sold last month (depreciated up to the sale, disposal date, gain or
// loss), the fixed asset accounts match the FA ledger and the G/L passes the balance check
func TestDemoFixedAssetsPosted(t *testing.T) {
	db, err := sql.Open("sqlite3", ":memory:")
	must(t, err)
	db.SetMaxOpenConns(1)
	t.Cleanup(func() { _ = db.Close() })
	must(t, tables.CreateCompanyTables(db, "demo", database.DBTypeSQLite))
	today := time.Date(2026, 10, 6, 0, 0, 0, 0, time.Local)
	counts, err := demodata.Create(db, "demo", database.DBTypeSQLite, demodata.Options{Today: today,
		GenJnlPostLine: func(src ftables.Source) demodata.GenJnlPoster { return genjnl.NewGenJnlPostLine(src) }})
	must(t, err)
	src := ftables.DBSource(db, "demo", database.DBTypeSQLite)
	if counts.FALedgerEntries < 100 || counts.MaintenanceLedgerEntries < 3 {
		t.Errorf("%d FA entries, %d maintenance entries", counts.FALedgerEntries, counts.MaintenanceLedgerEntries)
	}
	// (the COMPANY book disposes Net: the proceeds entry itself has no G/L entry)
	entries := gtables.NewFALedgerEntry(src)
	entries.SetRange("g_l_entry_no", 0)
	entries.SetFilter("fa_posting_type", "<>"+strconv.Itoa(int(gtables.FALedgerEntry_Fa_posting_type.ProceedsOnDisposal)))
	for found := entries.FindSet(); found; found = entries.Next() {
		t.Errorf("FA entry %d without a G/L entry: %s %v %v %s", entries.Entry_no, entries.Fa_no, entries.Fa_posting_category, entries.Fa_posting_type, entries.Amount)
	}

	// FA0040: acquired 34 months before this month (1 December 2023) for 148,000 over 10 years —
	// 34 monthly depreciations of 1,233.33, December 2023 to September 2026
	b := gtables.NewFADepreciationBook(src)
	must(t, b.GetOrError(map[string]interface{}{"fa_no": "FA0040", "depreciation_book_code": "COMPANY"}))
	b.CalcFields("acquisition_cost", "depreciation")
	wantDec(t, "FA0040 acquisition cost", b.Acquisition_cost, "148000")
	wantDec(t, "FA0040 depreciation", b.Depreciation, dec("-1233.33").Mul(types.NewDecimalFromInt(34)).String())
	if !b.Last_depreciation_date.Equal(date("2026-09-30")) {
		t.Errorf("FA0040 last depreciation date %s", b.Last_depreciation_date)
	}

	// FA0030 sold on 15 September 2026, depreciated up to that day
	sold := gtables.NewFADepreciationBook(src)
	must(t, sold.GetOrError(map[string]interface{}{"fa_no": "FA0030", "depreciation_book_code": "COMPANY"}))
	sold.CalcFields("proceeds_on_disposal", "gain_loss", "book_value")
	if !sold.Disposal_date.Equal(date("2026-09-15")) || !sold.Last_depreciation_date.Equal(date("2026-09-15")) {
		t.Errorf("FA0030 disposal %s, last depreciation %s", sold.Disposal_date, sold.Last_depreciation_date)
	}
	wantDec(t, "FA0030 proceeds", sold.Proceeds_on_disposal, "-150000")
	if sold.Gain_loss.IsZero() {
		t.Error("FA0030 sale without gain or loss")
	}

	// the acquisition invoices are paid
	vendLedgEntry := gtables.NewVendorLedgerEntry(src)
	vendLedgEntry.SetRange("document_no", types.Code("FA-FA0040"))
	if !vendLedgEntry.FindFirst() || vendLedgEntry.Open {
		t.Error("the acquisition of FA0040 is not paid")
	}

	// the machinery account holds the equipment's acquisition cost (the forklift sold: FA0040 only)
	glAcc := gtables.NewGLAccount(src)
	must(t, glAcc.GetOrError(types.Code("1230")))
	glAcc.CalcFields("balance")
	wantDec(t, "G/L 1230", glAcc.Balance, "148000")

	result := glcheck.Check(src, nil)
	if !result.Balanced() {
		t.Errorf("G/L balance check: %+v", result)
	}
}
