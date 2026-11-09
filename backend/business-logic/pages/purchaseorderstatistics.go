package pages

import (
	"github.com/hansjlachmann/goeasyerp/backend/business-logic/purchpost"
	"github.com/hansjlachmann/goeasyerp/backend/foundation/pages"
	gtables "github.com/hansjlachmann/goeasyerp/backend/generated/tables"
)

// purchOrderStatisticsTabs are the quantities page 403 totals, one section each (classic
// form 403's tabs General, Invoicing, Shipping — what is to be received)
var purchOrderStatisticsTabs = []struct {
	prefix  string
	qtyType purchpost.QtyType
}{
	{"general", purchpost.General},
	{"invoicing", purchpost.Invoicing},
	{"receiving", purchpost.Receiving},
}

// Page 403 Purchase Order Statistics: the order's totals (Purch.-Post SumPurchLines) of all its
// lines' quantity, of what is to be invoiced and of what is to be received — amount before and
// after the invoice discount, tax, total incl. tax, the purchase in LCY, quantity, parcels,
// weights, volume — and the pay-to vendor's balance (OnAfterGetRecord)
func init() {
	pages.OnAfterGetRecord(403, func(rec *gtables.PurchaseHeader, page *pages.Control) {
		for _, tab := range purchOrderStatisticsTabs {
			totals, err := purchpost.SumPurchLines(rec, tab.qtyType)
			if err != nil {
				return
			}
			totalPurchLine := totals.TotalPurchLine
			totalPurchLineLCY := totals.TotalPurchLineLCY
			page.SetValue(tab.prefix+"_line_amount", totalPurchLine.Amount.Add(totalPurchLine.Inv_discount_amount))
			page.SetValue(tab.prefix+"_inv_discount_amount", totalPurchLine.Inv_discount_amount)
			page.SetValue(tab.prefix+"_total", totalPurchLine.Amount)
			page.SetValue(tab.prefix+"_tax_amount", totals.TaxAmount)
			page.SetValue(tab.prefix+"_total_incl_tax", totalPurchLine.Amount_including_tax)
			page.SetValue(tab.prefix+"_purchase_lcy", totalPurchLineLCY.Amount)
			page.SetValue(tab.prefix+"_quantity", totalPurchLine.Quantity)
			page.SetValue(tab.prefix+"_parcels", totalPurchLine.Units_per_parcel)
			page.SetValue(tab.prefix+"_net_weight", totalPurchLine.Net_weight)
			page.SetValue(tab.prefix+"_gross_weight", totalPurchLine.Gross_weight)
			page.SetValue(tab.prefix+"_volume", totalPurchLine.Unit_volume)
		}

		vend := gtables.NewVendor(rec)
		if vend.Get(rec.Pay_to_vendor_no) {
			vend.CalcFields("balance_lcy")
		} else {
			vend.Init()
		}
		page.SetValue("balance_lcy", vend.Balance_lcy)
	})
}
