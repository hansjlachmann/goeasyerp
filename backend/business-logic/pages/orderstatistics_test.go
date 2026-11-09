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

func orderStatisticsSource(t *testing.T) ftables.Source {
	t.Helper()
	db, err := sql.Open("sqlite3", ":memory:")
	if err != nil {
		t.Fatal(err)
	}
	db.SetMaxOpenConns(1)
	t.Cleanup(func() { _ = db.Close() })
	if err := tables.CreateCompanyTables(db, "c1", database.DBTypeSQLite); err != nil {
		t.Fatal(err)
	}
	return ftables.DBSource(db, "c1", database.DBTypeSQLite)
}

func checkValues(t *testing.T, v map[string]interface{}, want map[string]string) {
	t.Helper()
	for field, value := range want {
		got, _ := types.NewDecimalFromString(fmt.Sprint(v[field]))
		if !got.Equal(types.MustDecimal(value)) {
			t.Errorf("%s = %v, want %s", field, v[field], value)
		}
	}
}

// Page 402 Sales Order Statistics totals the order's lines three ways: their whole quantity
// (General), what is to be invoiced (Invoicing) and what is to be shipped (Shipping)
func TestSalesOrderStatistics(t *testing.T) {
	src := orderStatisticsSource(t)
	cust := gtables.NewCustomer(src)
	cust.No = "C1"
	cust.Credit_limit_lcy = types.NewDecimal(1000)
	if err := cust.InsertOrError(false); err != nil {
		t.Fatal(err)
	}
	header := gtables.NewSalesHeader(src)
	header.Document_type, header.No, header.Bill_to_customer_no = gtables.SalesHeader_Document_type.Order, "SO1", "C1"
	if err := header.InsertOrError(false); err != nil {
		t.Fatal(err)
	}
	line := gtables.NewSalesLine(src)
	line.Document_type, line.Document_no, line.Line_no = gtables.SalesLine_Document_type.Order, "SO1", 10000
	line.Quantity, line.Qty_to_invoice, line.Qty_to_ship = types.NewDecimal(10), types.NewDecimal(4), types.NewDecimal(6)
	line.Amount, line.Amount_including_tax, line.Unit_cost_lcy = types.NewDecimal(1000), types.NewDecimal(1100), types.NewDecimal(50)
	if err := line.InsertOrError(false); err != nil {
		t.Fatal(err)
	}
	control, err := pages.RunAfterGetRecordControl(402, header)
	if err != nil {
		t.Fatal(err)
	}
	checkValues(t, control.Values(), map[string]string{
		"general_quantity": "10", "general_total": "1000", "general_tax_amount": "100", "general_cost_lcy": "500",
		"invoicing_quantity": "4", "invoicing_total": "400", "invoicing_total_incl_tax": "440", "invoicing_profit_lcy": "200",
		"shipping_quantity": "6", "shipping_total": "600", "shipping_sales_lcy": "600",
		"credit_limit_lcy": "1000",
	})
}

// Page 403 Purchase Order Statistics: the same for a purchase order (Receiving)
func TestPurchaseOrderStatistics(t *testing.T) {
	src := orderStatisticsSource(t)
	header := gtables.NewPurchaseHeader(src)
	header.Document_type, header.No = gtables.PurchaseHeader_Document_type.Order, "PO1"
	if err := header.InsertOrError(false); err != nil {
		t.Fatal(err)
	}
	line := gtables.NewPurchaseLine(src)
	line.Document_type, line.Document_no, line.Line_no = gtables.PurchaseLine_Document_type.Order, "PO1", 10000
	line.Quantity, line.Qty_to_invoice, line.Qty_to_receive = types.NewDecimal(10), types.NewDecimal(2), types.NewDecimal(5)
	line.Amount, line.Amount_including_tax = types.NewDecimal(500), types.NewDecimal(500)
	if err := line.InsertOrError(false); err != nil {
		t.Fatal(err)
	}
	control, err := pages.RunAfterGetRecordControl(403, header)
	if err != nil {
		t.Fatal(err)
	}
	checkValues(t, control.Values(), map[string]string{
		"general_quantity": "10", "general_total": "500",
		"invoicing_quantity": "2", "invoicing_total": "100", "invoicing_purchase_lcy": "100",
		"receiving_quantity": "5", "receiving_total_incl_tax": "250",
	})
}
