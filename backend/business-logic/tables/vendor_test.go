package tables

import (
	"testing"

	"github.com/hansjlachmann/goeasyerp/backend/foundation/database"
	apperrors "github.com/hansjlachmann/goeasyerp/backend/foundation/errors"
	ftables "github.com/hansjlachmann/goeasyerp/backend/foundation/tables"
	"github.com/hansjlachmann/goeasyerp/backend/foundation/types"
	gtables "github.com/hansjlachmann/goeasyerp/backend/generated/tables"
)

// The classic Vendor code: numbers from the Vendor Nos. series of the Purchases & Payables
// Setup, manual numbers only when the series allows them, Search Name, Post Code → City, the
// delete check with accounting periods and the delete of the vendor's comments, bank accounts
// and order addresses; an order address gets the vendor's name; Comment and Balance (LCY) as
// classic (-Sum: what is owed is positive)
func TestVendorTriggers(t *testing.T) {
	db := newTestDB(t)
	createTables(t, db, &gtables.Vendor{}, &gtables.VendorLedgerEntry{}, &gtables.PostCode{}, &gtables.AccountingPeriod{},
		&gtables.CommentLine{}, &gtables.OrderAddress{}, &gtables.VendorBankAccount{},
		&gtables.PurchasesPayablesSetup{}, &gtables.NoSeries{}, &gtables.NoSeriesLine{}, &gtables.NoSeriesRelationship{})
	src := ftables.DBSource(db, "c1", database.DBTypeSQLite)
	series := gtables.NewNoSeries(src)
	series.Code, series.Default_nos = "VEND", true
	line := gtables.NewNoSeriesLine(src)
	line.InitRecord()
	line.Series_code, line.Line_no, line.Starting_no = "VEND", 10000, "V00001"
	purchSetup := gtables.NewPurchasesPayablesSetup(src)
	purchSetup.InitRecord()
	purchSetup.Vendor_nos = "VEND"
	if !series.Insert(false) || !line.Insert(false) || !purchSetup.Insert(false) {
		t.Fatal("insert vendor number setup")
	}
	newVend := func() *gtables.Vendor {
		v := gtables.NewVendor(src)
		v.InitRecord()
		return v
	}

	// Numbers from the series; Invoice Disc. Code follows
	for _, want := range []types.Code{"V00001", "V00002"} {
		v := newVend()
		v.Name = "Nordisk Papir AS"
		if err := v.InsertOrError(true); err != nil || v.No != want || v.No_series != "VEND" || v.Invoice_disc_code != want {
			t.Fatalf("insert: no %q series %q disc %q, %v", v.No, v.No_series, v.Invoice_disc_code, err)
		}
	}
	v := newVend()
	if err := v.Validate("no", types.Code("X1")); err == nil || errCode(t, err) != apperrors.ErrNoSeriesManual {
		t.Errorf("manual number: %v", err)
	}
	_ = v.Validate("name", types.Text("Vestland Emballasje"))
	if v.Search_name != "VESTLAND EMBALLASJE" {
		t.Errorf("search name %q", v.Search_name)
	}
	postCode := gtables.NewPostCode(src)
	postCode.Code, postCode.City = "5003", "Bergen"
	if !postCode.Insert(false) {
		t.Fatal("insert post code")
	}
	if err := v.Validate("post_code", types.Code("5003")); err != nil || v.City != "Bergen" {
		t.Errorf("post code: city %q, %v", v.City, err)
	}

	// An order address gets the vendor's name; comments, bank accounts and order addresses
	stored := gtables.NewVendor(src)
	if err := stored.GetOrError(types.Code("V00001")); err != nil {
		t.Fatal(err)
	}
	orderAddr := gtables.NewOrderAddress(src)
	orderAddr.Vendor_no, orderAddr.Code = "V00001", "SALG"
	if err := orderAddr.InsertOrError(true); err != nil || orderAddr.Name != "Nordisk Papir AS" {
		t.Fatalf("order address name %q, %v", orderAddr.Name, err)
	}
	bankAcc := gtables.NewVendorBankAccount(src)
	bankAcc.Vendor_no, bankAcc.Code = "V00001", "DNB"
	comment := gtables.NewCommentLine(src)
	comment.Table_name, comment.No, comment.Line_no, comment.Comment = gtables.CommentLine_Table_name.Vendor, "V00001", 10000, "Note"
	if err := bankAcc.InsertOrError(true); err != nil {
		t.Fatal(err)
	}
	if err := comment.InsertOrError(true); err != nil {
		t.Fatal(err)
	}

	// An open invoice of 100 owed: Balance (LCY) 100 (-Sum); no delete while it is open
	entry := gtables.NewVendorLedgerEntry(src)
	entry.Entry_no, entry.Vendor_no, entry.Posting_date, entry.Open = 1, "V00001", types.NewDate(2025, 3, 1), true
	entry.Amount_lcy, entry.Remaining_amt_lcy = types.NewDecimal(-100), types.NewDecimal(-100)
	if err := entry.InsertOrError(false); err != nil {
		t.Fatal(err)
	}
	stored.CalcFields("balance_lcy", "comment")
	if !stored.Balance_lcy.Equal(types.NewDecimal(100)) || !stored.Comment {
		t.Errorf("balance %s, comment %v", stored.Balance_lcy, stored.Comment)
	}
	if stored.Delete(true) || errCode(t, stored.TriggerError()) != apperrors.ErrDeleteHasEntries {
		t.Fatalf("delete with an entry and no periods: %v", stored.TriggerError())
	}
	// 2025 closed: the entry still refuses the delete while it is open
	addPeriods(t, db, 2025, 2, types.NewDate(2025, 12, 1))
	if stored.Delete(true) || errCode(t, stored.TriggerError()) != apperrors.ErrDeleteOpenEntries {
		t.Fatalf("delete with an open entry: %v", stored.TriggerError())
	}
	entry.Open = false
	if err := entry.ModifyOrError(false); err != nil {
		t.Fatal(err)
	}
	if err := stored.DeleteOrError(true); err != nil {
		t.Fatalf("delete: %v", err)
	}
	if gtables.NewOrderAddress(src).Count() != 0 || gtables.NewVendorBankAccount(src).Count() != 0 || gtables.NewCommentLine(src).Count() != 0 {
		t.Error("the vendor's order addresses, bank accounts or comments are left")
	}
	if !entry.Get(1) || entry.Vendor_no != "" {
		t.Errorf("entry vendor no %q", entry.Vendor_no)
	}

	// AssistEdit without a dialog (no page can open): nothing changes
	v = newVend()
	if changed, err := AssistEditVendor(v); changed || err != nil || v.No != "" {
		t.Errorf("AssistEdit without a dialog: %v %v %q", changed, err, v.No)
	}
}
