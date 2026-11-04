package pages

import (
	"github.com/hansjlachmann/goeasyerp/backend/business-logic/salespost"
	"github.com/hansjlachmann/goeasyerp/backend/foundation/pages"
	"github.com/hansjlachmann/goeasyerp/backend/foundation/system"
	"github.com/hansjlachmann/goeasyerp/backend/foundation/types"
	gtables "github.com/hansjlachmann/goeasyerp/backend/generated/tables"
)

// Page 160 Sales Statistics: the document's totals (Sales-Post SumSalesLines) — amount before
// and after the invoice discount, tax, total incl. tax, sales, cost and profit in LCY, quantity,
// parcels, weights, volume — and the bill-to customer's balance against its credit limit
// (OnAfterGetRecord)
func init() {
	pages.OnAfterGetRecord(160, func(rec *gtables.SalesHeader, page *pages.Control) {
		totals, err := salespost.SumSalesLines(rec, salespost.General)
		if err != nil {
			return
		}
		totalSalesLine := totals.TotalSalesLine
		totalSalesLineLCY := totals.TotalSalesLineLCY
		page.SetValue("line_amount", totalSalesLine.Amount.Add(totalSalesLine.Inv_discount_amount))
		page.SetValue("inv_discount_amount", totalSalesLine.Inv_discount_amount)
		page.SetValue("total", totalSalesLine.Amount)
		page.SetValue("tax_amount", totals.TaxAmount)
		page.SetValue("total_incl_tax", totalSalesLine.Amount_including_tax)
		page.SetValue("sales_lcy", totalSalesLineLCY.Amount)
		page.SetValue("cost_lcy", totalSalesLineLCY.Unit_cost_lcy)
		page.SetValue("profit_lcy", totals.ProfitLCY)
		page.SetValue("profit_pct", totals.ProfitPct)
		page.SetValue("quantity", totalSalesLine.Quantity)
		page.SetValue("parcels", totalSalesLine.Units_per_parcel)
		page.SetValue("net_weight", totalSalesLine.Net_weight)
		page.SetValue("gross_weight", totalSalesLine.Gross_weight)
		page.SetValue("volume", totalSalesLine.Unit_volume)

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
