package pages

import (
	"database/sql"
	"fmt"
	"testing"

	"github.com/hansjlachmann/goeasyerp/backend/business-logic/tables"
	"github.com/hansjlachmann/goeasyerp/backend/foundation/database"
	"github.com/hansjlachmann/goeasyerp/backend/foundation/pages"
	ftables "github.com/hansjlachmann/goeasyerp/backend/foundation/tables"
	"github.com/hansjlachmann/goeasyerp/backend/foundation/types"
	gtables "github.com/hansjlachmann/goeasyerp/backend/generated/tables"
)

// Page 397 Sales Invoice Statistics sums the posted invoice's lines: amounts, tax, cost and
// profit, quantity, parcels and weights, and the customer's credit limit used
func TestSalesInvoiceStatistics(t *testing.T) {
	db, err := sql.Open("sqlite3", ":memory:")
	if err != nil {
		t.Fatal(err)
	}
	db.SetMaxOpenConns(1)
	t.Cleanup(func() { _ = db.Close() })
	if err := tables.CreateCompanyTables(db, "c1", database.DBTypeSQLite); err != nil {
		t.Fatal(err)
	}
	src := ftables.DBSource(db, "c1", database.DBTypeSQLite)
	cust := gtables.NewCustomer(src)
	cust.No = "C1"
	cust.Credit_limit_lcy = types.NewDecimal(1000)
	if err := cust.InsertOrError(false); err != nil {
		t.Fatal(err)
	}
	header := gtables.NewSalesInvoiceHeader(src)
	header.No, header.Bill_to_customer_no = "INV1", "C1"
	if err := header.InsertOrError(false); err != nil {
		t.Fatal(err)
	}
	for i, l := range []struct{ qty, amount, inclTax, disc, cost, perParcel string }{
		{"3", "300", "324", "0", "60", "2"},
		{"2", "90", "97.2", "10", "20", "0"},
	} {
		line := gtables.NewSalesInvoiceLine(src)
		line.Document_no, line.Line_no = "INV1", (i+1)*10000
		line.Quantity, line.Amount, line.Amount_including_tax = types.MustDecimal(l.qty), types.MustDecimal(l.amount), types.MustDecimal(l.inclTax)
		line.Inv_discount_amount, line.Unit_cost_lcy, line.Units_per_parcel = types.MustDecimal(l.disc), types.MustDecimal(l.cost), types.MustDecimal(l.perParcel)
		line.Net_weight = types.MustDecimal("1.5")
		if err := line.InsertOrError(false); err != nil {
			t.Fatal(err)
		}
	}
	control, err := pages.RunAfterGetRecordControl(397, header)
	if err != nil {
		t.Fatal(err)
	}
	v := control.Values()
	want := map[string]string{
		"line_amount": "400", "inv_discount_amount": "10", "total": "390", "tax_amount": "31.2", "total_incl_tax": "421.2",
		"sales_lcy": "390", "cost_lcy": "220", "profit_lcy": "170", "profit_pct": "43.6",
		"quantity": "5", "parcels": "2", "net_weight": "7.5", "credit_limit_lcy": "1000",
	}
	for field, value := range want {
		got, _ := types.NewDecimalFromString(fmt.Sprint(v[field]))
		if !got.Equal(types.MustDecimal(value)) {
			t.Errorf("%s = %v, want %s", field, v[field], value)
		}
	}
}
