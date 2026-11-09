package pages

import (
	"github.com/hansjlachmann/goeasyerp/backend/business-logic/salespost"
	"github.com/hansjlachmann/goeasyerp/backend/foundation/pages"
	"github.com/hansjlachmann/goeasyerp/backend/foundation/system"
	"github.com/hansjlachmann/goeasyerp/backend/foundation/types"
	gtables "github.com/hansjlachmann/goeasyerp/backend/generated/tables"
)

// salesOrderStatisticsTabs are the quantities page 402 totals, one section each (classic
// form 402's tabs General, Invoicing, Shipping)
var salesOrderStatisticsTabs = []struct {
	prefix  string
	qtyType salespost.QtyType
}{
	{"general", salespost.General},
	{"invoicing", salespost.Invoicing},
	{"shipping", salespost.Shipping},
}

// Page 402 Sales Order Statistics: the order's totals (Sales-Post SumSalesLines) of all its
// lines' quantity, of what is to be invoiced and of what is to be shipped — amount before and
// after the invoice discount, tax, total incl. tax, sales, cost and profit in LCY, quantity,
// parcels, weights, volume — and the bill-to customer's balance against its credit limit
// (OnAfterGetRecord)
func init() {
	pages.OnAfterGetRecord(402, func(rec *gtables.SalesHeader, page *pages.Control) {
		for _, tab := range salesOrderStatisticsTabs {
			totals, err := salespost.SumSalesLines(rec, tab.qtyType)
			if err != nil {
				return
			}
			totalSalesLine := totals.TotalSalesLine
			totalSalesLineLCY := totals.TotalSalesLineLCY
			page.SetValue(tab.prefix+"_line_amount", totalSalesLine.Amount.Add(totalSalesLine.Inv_discount_amount))
			page.SetValue(tab.prefix+"_inv_discount_amount", totalSalesLine.Inv_discount_amount)
			page.SetValue(tab.prefix+"_total", totalSalesLine.Amount)
			page.SetValue(tab.prefix+"_tax_amount", totals.TaxAmount)
			page.SetValue(tab.prefix+"_total_incl_tax", totalSalesLine.Amount_including_tax)
			page.SetValue(tab.prefix+"_sales_lcy", totalSalesLineLCY.Amount)
			page.SetValue(tab.prefix+"_cost_lcy", totalSalesLineLCY.Unit_cost_lcy)
			page.SetValue(tab.prefix+"_profit_lcy", totals.ProfitLCY)
			page.SetValue(tab.prefix+"_profit_pct", totals.ProfitPct)
			page.SetValue(tab.prefix+"_quantity", totalSalesLine.Quantity)
			page.SetValue(tab.prefix+"_parcels", totalSalesLine.Units_per_parcel)
			page.SetValue(tab.prefix+"_net_weight", totalSalesLine.Net_weight)
			page.SetValue(tab.prefix+"_gross_weight", totalSalesLine.Gross_weight)
			page.SetValue(tab.prefix+"_volume", totalSalesLine.Unit_volume)
		}

		cust := gtables.NewCustomer(rec)
		if cust.Get(rec.Bill_to_customer_no) {
			cust.CalcFields("balance_lcy")
		} else {
			cust.Init()
		}
		creditLimitExpendedPct := types.Decimal{}
		if !cust.Credit_limit_lcy.IsZero() {
			creditLimitExpendedPct = system.Round(cust.Balance_lcy.Div(cust.Credit_limit_lcy).Mul(types.NewDecimalFromInt(100)), types.MustDecimal("0.01"))
		}
		page.SetValue("balance_lcy", cust.Balance_lcy)
		page.SetValue("credit_limit_lcy", cust.Credit_limit_lcy)
		page.SetValue("credit_limit_expended_pct", creditLimitExpendedPct)
	})
}
