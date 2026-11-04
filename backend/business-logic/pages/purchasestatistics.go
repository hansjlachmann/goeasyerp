package pages

import (
	"github.com/hansjlachmann/goeasyerp/backend/business-logic/purchpost"
	"github.com/hansjlachmann/goeasyerp/backend/foundation/pages"
	gtables "github.com/hansjlachmann/goeasyerp/backend/generated/tables"
)

// Page 161 Purchase Statistics: the document's totals (Purch.-Post SumPurchLines) — amount
// before and after the invoice discount, tax, total incl. tax, the purchase in LCY, quantity,
// parcels, weights, volume — and the pay-to vendor's balance (OnAfterGetRecord)
func init() {
	pages.OnAfterGetRecord(161, func(rec *gtables.PurchaseHeader, page *pages.Control) {
		totals, err := purchpost.SumPurchLines(rec, purchpost.General)
		if err != nil {
			return
		}
		totalPurchLine := totals.TotalPurchLine
		totalPurchLineLCY := totals.TotalPurchLineLCY
		page.SetValue("line_amount", totalPurchLine.Amount.Add(totalPurchLine.Inv_discount_amount))
		page.SetValue("inv_discount_amount", totalPurchLine.Inv_discount_amount)
		page.SetValue("total", totalPurchLine.Amount)
		page.SetValue("tax_amount", totals.TaxAmount)
		page.SetValue("total_incl_tax", totalPurchLine.Amount_including_tax)
		page.SetValue("purchase_lcy", totalPurchLineLCY.Amount)
		page.SetValue("quantity", totalPurchLine.Quantity)
		page.SetValue("parcels", totalPurchLine.Units_per_parcel)
		page.SetValue("net_weight", totalPurchLine.Net_weight)
		page.SetValue("gross_weight", totalPurchLine.Gross_weight)
		page.SetValue("volume", totalPurchLine.Unit_volume)

		vend := gtables.NewVendor(rec)
		if vend.Get(rec.Pay_to_vendor_no) {
			vend.CalcFields("balance_lcy")
		} else {
			vend.Init()
		}
		page.SetValue("balance_lcy", vend.Balance_lcy)
	})
}
