package pages

import (
	"github.com/hansjlachmann/goeasyerp/backend/business-logic/tables"
	"github.com/hansjlachmann/goeasyerp/backend/foundation/pages"
	"github.com/hansjlachmann/goeasyerp/backend/foundation/system"
	ftables "github.com/hansjlachmann/goeasyerp/backend/foundation/tables"
	"github.com/hansjlachmann/goeasyerp/backend/foundation/types"
	gtables "github.com/hansjlachmann/goeasyerp/backend/generated/tables"
)

// The posted documents' statistics (F9 on the posted cards and lists; OnAfterGetRecord): pages
// 396 Sales Shipment and 399 Purchase Receipt Statistics — the lines' quantity, parcels, net and
// gross weight and volume —, 397 Sales Invoice and 398 Sales Credit Memo Statistics — also the
// amount before and after the invoice discount, the tax, the total incl. tax, the amount, cost
// and profit in LCY and the bill-to customer's balance against its credit limit —, 400 Purchase
// Invoice and 401 Purch. Credit Memo Statistics — the amounts, the amount in LCY and the pay-to
// vendor's balance.
const (
	salesShipmentStatisticsPageID = 396
	salesInvoiceStatisticsPageID  = 397
	salesCrMemoStatisticsPageID   = 398
	purchReceiptStatisticsPageID  = 399
	purchInvoiceStatisticsPageID  = 400
	purchCrMemoStatisticsPageID   = 401
)

// postedDocTotals: the classic forms' sums over the posted lines
type postedDocTotals struct {
	amount, amountInclTax, invDiscAmount, costLCY types.Decimal
	lineQty, netWeight, grossWeight, volume       types.Decimal
	parcels                                       types.Decimal
}

// addLine adds a posted line (the REPEAT … UNTIL of the forms)
func (t *postedDocTotals) addLine(amount, amountInclTax, invDiscAmount, unitCostLCY, quantity, netWeight, grossWeight, unitVolume, unitsPerParcel types.Decimal) {
	t.amount = t.amount.Add(amount)
	t.amountInclTax = t.amountInclTax.Add(amountInclTax)
	t.invDiscAmount = t.invDiscAmount.Add(invDiscAmount)
	t.costLCY = t.costLCY.Add(quantity.Mul(unitCostLCY))
	t.lineQty = t.lineQty.Add(quantity)
	t.netWeight = t.netWeight.Add(quantity.Mul(netWeight))
	t.grossWeight = t.grossWeight.Add(quantity.Mul(grossWeight))
	t.volume = t.volume.Add(quantity.Mul(unitVolume))
	if unitsPerParcel.IsPositive() {
		t.parcels = t.parcels.Add(system.Round(quantity.Div(unitsPerParcel), types.NewDecimalFromInt(1), ">"))
	}
}

func (t *postedDocTotals) setQuantities(page *pages.Control) {
	page.SetValue("quantity", t.lineQty)
	page.SetValue("parcels", t.parcels)
	page.SetValue("net_weight", t.netWeight)
	page.SetValue("gross_weight", t.grossWeight)
	page.SetValue("volume", t.volume)
}

// setAmounts: the amounts of the General tab, and the amount in LCY at the work date's rate
func (t *postedDocTotals) setAmounts(src ftables.Source, page *pages.Control, currencyCode types.Code, currencyFactor types.Decimal) types.Decimal {
	page.SetValue("line_amount", t.amount.Add(t.invDiscAmount))
	page.SetValue("inv_discount_amount", t.invDiscAmount)
	page.SetValue("total", t.amount)
	page.SetValue("tax_amount", t.amountInclTax.Sub(t.amount))
	page.SetValue("total_incl_tax", t.amountInclTax)
	amountLCY := t.amount
	if currencyCode != "" {
		exchanged, err := tables.ExchangeRates(src).ExchangeAmtFCYToLCY(system.WorkDate(), currencyCode, t.amount, currencyFactor)
		if err == nil {
			amountLCY = exchanged
		}
	}
	return amountLCY
}

// setCustomer: the sales amount, cost and profit in LCY and the customer's balance against its
// credit limit
func (t *postedDocTotals) setCustomer(src ftables.Source, page *pages.Control, amountLCY types.Decimal, custNo types.Code) {
	profitLCY := amountLCY.Sub(t.costLCY)
	profitPct := types.Decimal{}
	if !amountLCY.IsZero() {
		profitPct = system.Round(types.NewDecimalFromInt(100).Mul(profitLCY).Div(amountLCY), types.MustDecimal("0.1"))
	}
	page.SetValue("sales_lcy", amountLCY)
	page.SetValue("cost_lcy", t.costLCY)
	page.SetValue("profit_lcy", profitLCY)
	page.SetValue("profit_pct", profitPct)
	cust := gtables.NewCustomer(src)
	if cust.Get(custNo) {
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
}

// setVendor: the amount in LCY and the vendor's balance
func (t *postedDocTotals) setVendor(src ftables.Source, page *pages.Control, amountLCY types.Decimal, vendNo types.Code) {
	page.SetValue("amount_lcy", amountLCY)
	vend := gtables.NewVendor(src)
	if vend.Get(vendNo) {
		vend.CalcFields("balance_lcy")
	} else {
		vend.Init()
	}
	page.SetValue("balance_lcy", vend.Balance_lcy)
}

func init() {
	pages.OnAfterGetRecord(salesShipmentStatisticsPageID, func(rec *gtables.SalesShipmentHeader, page *pages.Control) {
		var t postedDocTotals
		salesShptLine := gtables.NewSalesShipmentLine(rec)
		salesShptLine.SetRange("document_no", rec.No)
		for found := salesShptLine.FindSetBuffered(); found; found = salesShptLine.Next() {
			t.addLine(types.Decimal{}, types.Decimal{}, types.Decimal{}, types.Decimal{}, salesShptLine.Quantity,
				salesShptLine.Net_weight, salesShptLine.Gross_weight, salesShptLine.Unit_volume, salesShptLine.Units_per_parcel)
		}
		t.setQuantities(page)
	})
	pages.OnAfterGetRecord(salesInvoiceStatisticsPageID, func(rec *gtables.SalesInvoiceHeader, page *pages.Control) {
		var t postedDocTotals
		salesInvLine := gtables.NewSalesInvoiceLine(rec)
		salesInvLine.SetRange("document_no", rec.No)
		for found := salesInvLine.FindSetBuffered(); found; found = salesInvLine.Next() {
			t.addLine(salesInvLine.Amount, salesInvLine.Amount_including_tax, salesInvLine.Inv_discount_amount, salesInvLine.Unit_cost_lcy,
				salesInvLine.Quantity, salesInvLine.Net_weight, salesInvLine.Gross_weight, salesInvLine.Unit_volume, salesInvLine.Units_per_parcel)
		}
		t.setQuantities(page)
		amountLCY := t.setAmounts(rec, page, rec.Currency_code, rec.Currency_factor)
		t.setCustomer(rec, page, amountLCY, rec.Bill_to_customer_no)
	})
	pages.OnAfterGetRecord(salesCrMemoStatisticsPageID, func(rec *gtables.SalesCrMemoHeader, page *pages.Control) {
		var t postedDocTotals
		salesCrMemoLine := gtables.NewSalesCrMemoLine(rec)
		salesCrMemoLine.SetRange("document_no", rec.No)
		for found := salesCrMemoLine.FindSetBuffered(); found; found = salesCrMemoLine.Next() {
			t.addLine(salesCrMemoLine.Amount, salesCrMemoLine.Amount_including_tax, salesCrMemoLine.Inv_discount_amount, salesCrMemoLine.Unit_cost_lcy,
				salesCrMemoLine.Quantity, salesCrMemoLine.Net_weight, salesCrMemoLine.Gross_weight, salesCrMemoLine.Unit_volume, salesCrMemoLine.Units_per_parcel)
		}
		t.setQuantities(page)
		amountLCY := t.setAmounts(rec, page, rec.Currency_code, rec.Currency_factor)
		t.setCustomer(rec, page, amountLCY, rec.Bill_to_customer_no)
	})
	pages.OnAfterGetRecord(purchReceiptStatisticsPageID, func(rec *gtables.PurchRcptHeader, page *pages.Control) {
		var t postedDocTotals
		purchRcptLine := gtables.NewPurchRcptLine(rec)
		purchRcptLine.SetRange("document_no", rec.No)
		for found := purchRcptLine.FindSetBuffered(); found; found = purchRcptLine.Next() {
			t.addLine(types.Decimal{}, types.Decimal{}, types.Decimal{}, types.Decimal{}, purchRcptLine.Quantity,
				purchRcptLine.Net_weight, purchRcptLine.Gross_weight, purchRcptLine.Unit_volume, purchRcptLine.Units_per_parcel)
		}
		t.setQuantities(page)
	})
	pages.OnAfterGetRecord(purchInvoiceStatisticsPageID, func(rec *gtables.PurchInvHeader, page *pages.Control) {
		var t postedDocTotals
		purchInvLine := gtables.NewPurchInvLine(rec)
		purchInvLine.SetRange("document_no", rec.No)
		for found := purchInvLine.FindSetBuffered(); found; found = purchInvLine.Next() {
			t.addLine(purchInvLine.Amount, purchInvLine.Amount_including_tax, purchInvLine.Inv_discount_amount, types.Decimal{},
				purchInvLine.Quantity, purchInvLine.Net_weight, purchInvLine.Gross_weight, purchInvLine.Unit_volume, purchInvLine.Units_per_parcel)
		}
		t.setQuantities(page)
		amountLCY := t.setAmounts(rec, page, rec.Currency_code, rec.Currency_factor)
		t.setVendor(rec, page, amountLCY, rec.Pay_to_vendor_no)
	})
	pages.OnAfterGetRecord(purchCrMemoStatisticsPageID, func(rec *gtables.PurchCrMemoHdr, page *pages.Control) {
		var t postedDocTotals
		purchCrMemoLine := gtables.NewPurchCrMemoLine(rec)
		purchCrMemoLine.SetRange("document_no", rec.No)
		for found := purchCrMemoLine.FindSetBuffered(); found; found = purchCrMemoLine.Next() {
			t.addLine(purchCrMemoLine.Amount, purchCrMemoLine.Amount_including_tax, purchCrMemoLine.Inv_discount_amount, types.Decimal{},
				purchCrMemoLine.Quantity, purchCrMemoLine.Net_weight, purchCrMemoLine.Gross_weight, purchCrMemoLine.Unit_volume, purchCrMemoLine.Units_per_parcel)
		}
		t.setQuantities(page)
		amountLCY := t.setAmounts(rec, page, rec.Currency_code, rec.Currency_factor)
		t.setVendor(rec, page, amountLCY, rec.Pay_to_vendor_no)
	})
}
