package reports_test

import (
	"testing"

	_ "github.com/hansjlachmann/goeasyerp/backend/business-logic/pages"
	"github.com/hansjlachmann/goeasyerp/backend/business-logic/tables"
	"github.com/hansjlachmann/goeasyerp/backend/foundation/pages"
	"github.com/hansjlachmann/goeasyerp/backend/foundation/types"
	gtables "github.com/hansjlachmann/goeasyerp/backend/generated/tables"
)

// A purchase line takes the vendor's quantity discount on the item for its quantity (Item Purch.
// Qty. Disc.: the highest minimum quantity up to the line's base quantity) as Line Discount %
func TestPurchaseQuantityDiscount(t *testing.T) {
	src := listCompany(t)
	purchHeader := gtables.NewPurchaseHeader(src)
	purchHeader.Document_type = gtables.PurchaseHeader_Document_type.Order
	mustOK(t, purchHeader.InsertOrError(true))
	mustOK(t, purchHeader.Validate("buy_from_vendor_no", types.Code("V00010")))
	mustOK(t, purchHeader.ModifyOrError(true))

	purchLine := gtables.NewPurchaseLine(src)
	purchLine.Document_type = gtables.PurchaseLine_Document_type.Order
	purchLine.Document_no = purchHeader.No
	purchLine.Line_no = 10000
	mustOK(t, purchLine.Validate("type", gtables.PurchaseLine_Type.Item))
	mustOK(t, purchLine.Validate("no", types.Code("1000")))
	for _, c := range []struct{ qty, disc int64 }{{10, 0}, {200, 3}, {499, 3}, {500, 5}, {1000, 5}} {
		mustOK(t, purchLine.Validate("quantity", types.NewDecimalFromInt(c.qty)))
		if !purchLine.Quantity_disc_percent.Equal(types.NewDecimalFromInt(c.disc)) || !purchLine.Line_discount_percent.Equal(types.NewDecimalFromInt(c.disc)) {
			t.Errorf("quantity %d: Quantity Disc. %% %s, Line Discount %% %s, want %d", c.qty, purchLine.Quantity_disc_percent, purchLine.Line_discount_percent, c.disc)
		}
	}
}

// A reminder in a currency with a Currency for Reminder Level line takes that fee; a finance
// charge memo the Currency for Fin. Charge Terms fee
func TestAdditionalFeeByCurrency(t *testing.T) {
	src := listCompany(t)
	reminderHeader := gtables.NewReminderHeader(src)
	mustOK(t, reminderHeader.InsertOrError(true))
	mustOK(t, reminderHeader.Validate("customer_no", types.Code("C00010")))
	reminderHeader.Currency_code = "EUR"
	reminderHeader.Reminder_terms_code = "DOMESTIC"
	reminderHeader.Reminder_level = 2
	mustOK(t, reminderHeader.ModifyOrError(false))
	mustOK(t, tables.InsertReminderLines(reminderHeader))
	reminderLine := gtables.NewReminderLine(src)
	reminderLine.SetRange("reminder_no", reminderHeader.No)
	reminderLine.SetRange("type", gtables.ReminderLine_Type.GLAccount)
	if !reminderLine.FindFirst() || !reminderLine.Amount.Equal(types.NewDecimalFromInt(6)) {
		t.Errorf("reminder fee line: %s, want 6 (EUR)", reminderLine.Amount)
	}

	finChrgMemoHeader := gtables.NewFinanceChargeMemoHeader(src)
	mustOK(t, finChrgMemoHeader.InsertOrError(true))
	mustOK(t, finChrgMemoHeader.Validate("customer_no", types.Code("C00010")))
	finChrgMemoHeader.Currency_code = "EUR"
	finChrgMemoHeader.Fin_charge_terms_code = "STD"
	mustOK(t, finChrgMemoHeader.ModifyOrError(false))
	mustOK(t, tables.InsertFinChrgMemoLines(finChrgMemoHeader))
	finChrgMemoLine := gtables.NewFinanceChargeMemoLine(src)
	finChrgMemoLine.SetRange("finance_charge_memo_no", finChrgMemoHeader.No)
	finChrgMemoLine.SetRange("type", gtables.FinanceChargeMemoLine_Type.GLAccount)
	if !finChrgMemoLine.FindFirst() || !finChrgMemoLine.Amount.Equal(types.NewDecimalFromInt(6)) {
		t.Errorf("finance charge memo fee line: %s, want 6 (EUR)", finChrgMemoLine.Amount)
	}
}

// Page 514: a cell of the discount overview is the Cust./Item Discount; typing a value changes,
// inserts or (0) deletes it
func TestCustItemDiscountOverview(t *testing.T) {
	src := listCompany(t)
	group := gtables.NewCustomerDiscountGroup(src)
	mustOK(t, group.GetOrError(types.Code("WHOLESALE")))
	itemGroup := gtables.NewItemDiscountGroup(src)
	itemGroup.SetRange("code", "FOOD")
	if !itemGroup.FindFirst() {
		t.Fatal("no item discount group FOOD")
	}
	disc := func() (types.Decimal, bool) {
		custItemDisc := gtables.NewCustItemDiscount(src)
		ok := custItemDisc.Get(map[string]interface{}{"customer_disc_group": "WHOLESALE", "item_discount_group": "FOOD"})
		return custItemDisc.Discount_percent, ok
	}
	column := map[string]string{"item_discount_group": "FOOD"}
	mustOK(t, pages.RunMatrixValidate(514, group, column, types.NewDecimalFromInt(4)))
	if v, ok := disc(); !ok || !v.Equal(types.NewDecimalFromInt(4)) {
		t.Errorf("after 4: %s %v", v, ok)
	}
	mustOK(t, pages.RunMatrixValidate(514, group, column, types.NewDecimalFromInt(7)))
	if v, _ := disc(); !v.Equal(types.NewDecimalFromInt(7)) {
		t.Errorf("after 7: %s", v)
	}
	mustOK(t, pages.RunMatrixValidate(514, group, column, types.Decimal{}))
	if _, ok := disc(); ok {
		t.Error("discount not deleted by 0")
	}

	all := gtables.NewCustomerDiscountGroup(src)
	columns, values, err := pages.RunMatrixCalculate(514, all, all)
	mustOK(t, err)
	paper := -1
	for i, c := range columns {
		if c.Name == "PAPER" {
			paper = i
		}
	}
	if paper < 0 || values["WHOLESALE"][paper] != "10" {
		t.Errorf("WHOLESALE × PAPER: columns %v, values %v", columns, values["WHOLESALE"])
	}
}
