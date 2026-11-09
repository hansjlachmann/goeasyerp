package reports_test

import (
	"testing"
	"time"

	"github.com/hansjlachmann/goeasyerp/backend/business-logic/salestax"
	ftables "github.com/hansjlachmann/goeasyerp/backend/foundation/tables"
	"github.com/hansjlachmann/goeasyerp/backend/foundation/types"
	gtables "github.com/hansjlachmann/goeasyerp/backend/generated/tables"
)

// Recalculate Sales Tax: three lines whose tax is rounded line by line; afterwards the lines' tax
// is the tax of their total
func TestRecalculateSalesTax(t *testing.T) {
	src := documentsCompany(t)
	day := types.NewDateFromTime(time.Date(2026, 10, 6, 0, 0, 0, 0, time.Local))
	order := gtables.NewSalesHeader(src)
	order.Document_type = gtables.SalesHeader_Document_type.Order
	mustOK(t, order.InsertOrError(true))
	mustOK(t, order.Validate("sell_to_customer_no", types.Code("C00010")))
	mustOK(t, order.Validate("posting_date", day))
	mustOK(t, order.ModifyOrError(true))
	for i := 1; i <= 3; i++ {
		line := gtables.NewSalesLine(src)
		line.Document_type = gtables.SalesLine_Document_type.Order
		line.Document_no = order.No
		line.Line_no = i * 10000
		mustOK(t, line.Validate("type", gtables.SalesLine_Type.Item))
		mustOK(t, line.Validate("no", types.Code("1000")))
		mustOK(t, line.Validate("quantity", types.NewDecimalFromInt(1)))
		mustOK(t, line.Validate("unit_price", types.MustDecimal("3.33")))
		mustOK(t, line.InsertOrError(true))
	}
	lineTax := func() (types.Decimal, types.Decimal, *gtables.SalesLine) {
		var tax, base types.Decimal
		line := gtables.NewSalesLine(src)
		line.SetRange("document_type", int(order.Document_type))
		line.SetRange("document_no", order.No)
		for found := line.FindSet(); found; found = line.Next() {
			tax = tax.Add(line.Amount_including_tax.Sub(line.Amount))
			base = base.Add(line.Tax_base_amount)
		}
		line.FindFirst()
		return tax, base, line
	}
	_, base, first := lineTax()
	if !first.Tax_liable || first.Tax_area_code == "" {
		t.Skipf("the demo customer's lines carry no sales tax (area %q)", first.Tax_area_code)
	}
	want, err := salestax.New(src).CalculateTax(first.Tax_area_code, first.Tax_group_code, true, day, base, types.NewDecimalFromInt(3), order.Currency_factor)
	mustOK(t, err)

	mustOK(t, inTransaction(t, src, func(txSrc ftables.Source) error {
		salesHeader := gtables.NewSalesHeader(txSrc)
		mustOK(t, salesHeader.GetOrError(map[string]interface{}{"document_type": 1, "no": order.No}))
		return salestax.New(txSrc).RecalcSalesLines(salesHeader)
	}))
	tax, _, _ := lineTax()
	if !tax.Equal(want) {
		t.Errorf("tax of the lines %s after recalculating, want %s (the tax of the total %s)", tax, want, base)
	}
}
