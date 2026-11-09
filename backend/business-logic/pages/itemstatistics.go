package pages

import (
	"github.com/hansjlachmann/goeasyerp/backend/business-logic/tables"
	fcodeunits "github.com/hansjlachmann/goeasyerp/backend/foundation/codeunits"
	"github.com/hansjlachmann/goeasyerp/backend/foundation/i18n"
	"github.com/hansjlachmann/goeasyerp/backend/foundation/pages"
	"github.com/hansjlachmann/goeasyerp/backend/foundation/system"
	ftables "github.com/hansjlachmann/goeasyerp/backend/foundation/tables"
	"github.com/hansjlachmann/goeasyerp/backend/foundation/types"
	gtables "github.com/hansjlachmann/goeasyerp/backend/generated/tables"
)

// Page 153 Item Statistics: the item's inventory, orders and availability and, for this
// accounting period, this fiscal year, last fiscal year and to date, its sales, adjustments,
// purchases and transfers in quantity and amount (OnAfterGetRecord; the periods of the work
// date, DateFilter-Calc). The production quantities (Scheduled Receipt/Need) are not part of
// this application.
//
// Page 304 Item Entry Statistics: the item's most recently posted receipt, purchase, positive
// adjustment, shipment, sale and negative adjustment, and its next and last order lines to be
// received and shipped (OnAfterGetRecord).
func init() {
	pages.OnAfterGetRecord(153, func(rec *gtables.Item, page *pages.Control) {
		currentDate := system.WorkDate()
		fiscalYearText := i18n.GetInstance().Message("FISCAL_YEAR_NAME", fcodeunits.CurrentLanguage())
		var itemDateFilter, itemDateName [4]string
		itemDateFilter[0], itemDateName[0] = tables.CreateAccountingPeriodFilter(rec, currentDate, 0)
		itemDateFilter[1], itemDateName[1] = tables.CreateFiscalYearFilter(rec, currentDate, 0, fiscalYearText)
		itemDateFilter[2], itemDateName[2] = tables.CreateFiscalYearFilter(rec, currentDate, -1, fiscalYearText)

		item := gtables.NewItem(rec)
		item.No = rec.No
		copyItemFlowFilters(rec, item)
		item.CalcFields("quantity_on_hand", "qty_on_purch_order", "qty_on_sales_order")
		qtyAvailable := item.Quantity_on_hand.Add(item.Qty_on_purch_order).Sub(item.Qty_on_sales_order)
		page.SetValue("quantity_on_hand", item.Quantity_on_hand)
		page.SetValue("qty_on_purch_order", item.Qty_on_purch_order)
		page.SetValue("qty_on_sales_order", item.Qty_on_sales_order)
		page.SetValue("qty_available", qtyAvailable)

		for i, column := range statisticsColumns {
			page.SetValue(column+"_name", itemDateName[i])
			// without accounting periods only To Date has amounts
			itemPeriod := gtables.NewItem(rec)
			itemPeriod.No = rec.No
			copyItemFlowFilters(rec, itemPeriod)
			if itemDateFilter[i] != "" || column == "to_date" {
				_ = itemPeriod.SetFlowFilter("date_filter", itemDateFilter[i])
				itemPeriod.CalcFields(
					"purchases_qty", "sales_qty", "positive_adjmt_qty", "negative_adjmt_qty",
					"purchases_lcy", "sales_lcy", "cogs_lcy", "transferred_qty", "transferred_lcy")
			}
			increasesQty := itemPeriod.Purchases_qty.Add(itemPeriod.Positive_adjmt_qty).Add(itemPeriod.Transferred_qty)
			decreasesQty := itemPeriod.Sales_qty.Add(itemPeriod.Negative_adjmt_qty)
			profit := itemPeriod.Sales_lcy.Sub(itemPeriod.Cogs_lcy)
			profitPct := types.ZeroDecimal()
			if !itemPeriod.Sales_lcy.IsZero() {
				profitPct = system.Round(profit.Mul(types.NewDecimal(100)).Div(itemPeriod.Sales_lcy), types.MustDecimal("0.1"))
			}
			page.SetValue(column+"_sales_qty", itemPeriod.Sales_qty)
			page.SetValue(column+"_negative_adjmt_qty", itemPeriod.Negative_adjmt_qty)
			page.SetValue(column+"_decreases_qty", decreasesQty)
			page.SetValue(column+"_sales_lcy", itemPeriod.Sales_lcy)
			page.SetValue(column+"_cogs_lcy", itemPeriod.Cogs_lcy)
			page.SetValue(column+"_profit_lcy", profit)
			page.SetValue(column+"_profit_pct", profitPct)
			page.SetValue(column+"_purchases_qty", itemPeriod.Purchases_qty)
			page.SetValue(column+"_positive_adjmt_qty", itemPeriod.Positive_adjmt_qty)
			page.SetValue(column+"_transferred_qty", itemPeriod.Transferred_qty)
			page.SetValue(column+"_increases_qty", increasesQty)
			page.SetValue(column+"_purchases_lcy", itemPeriod.Purchases_lcy)
		}
	})

	pages.OnAfterGetRecord(304, func(rec *gtables.Item, page *pages.Control) {
		s := itemEntryStatistics(rec)
		// Most Recently Posted (outbound quantities and amounts shown positive)
		setItemEntry(page, "receipt", s.itemLedgEntry[5], s.itemLedgEntry[5].Quantity, s.itemLedgEntry[5].Amount)
		setItemEntry(page, "purchase", s.itemLedgEntry[1], s.itemLedgEntry[1].Invoiced_quantity, s.itemLedgEntry[1].Amount)
		setItemEntry(page, "positive_adjmt", s.itemLedgEntry[3], s.itemLedgEntry[3].Quantity, s.itemLedgEntry[3].Amount)
		setItemEntry(page, "shipment", s.itemLedgEntry[6], s.itemLedgEntry[6].Quantity.Neg(), s.itemLedgEntry[6].Amount.Neg())
		setItemEntry(page, "sale", s.itemLedgEntry[2], s.itemLedgEntry[2].Invoiced_quantity.Neg(), s.itemLedgEntry[2].Amount.Neg())
		setItemEntry(page, "negative_adjmt", s.itemLedgEntry[4], s.itemLedgEntry[4].Quantity.Neg(), s.itemLedgEntry[4].Amount.Neg())
		// To Be Posted
		for k, prefix := range []string{"next_receipt", "last_receipt"} {
			purchLine := s.purchOrderLine[k]
			page.SetValue(prefix+"_date", purchLine.Expected_receipt_date)
			page.SetValue(prefix+"_order_no", purchLine.Document_no)
			page.SetValue(prefix+"_quantity", purchLine.Quantity)
			page.SetValue(prefix+"_unit_amount", purchLine.Direct_unit_cost)
			page.SetValue(prefix+"_qty_on_order", purchLine.Outstanding_quantity)
		}
		for k, prefix := range []string{"next_shipment", "last_shipment"} {
			salesLine := s.salesLine[k]
			page.SetValue(prefix+"_date", salesLine.Shipment_date)
			page.SetValue(prefix+"_order_no", salesLine.Document_no)
			page.SetValue(prefix+"_quantity", salesLine.Quantity)
			page.SetValue(prefix+"_unit_amount", salesLine.Unit_price)
			page.SetValue(prefix+"_qty_on_order", salesLine.Outstanding_quantity)
		}
	})
}

// copyItemFlowFilters gives item the FlowFilters of rec (the page's record) but its Date Filter
func copyItemFlowFilters(rec, item *gtables.Item) {
	item.Dimension_1_filter = rec.Dimension_1_filter
	item.Dimension_2_filter = rec.Dimension_2_filter
	item.Location_filter = rec.Location_filter
	item.Variant_filter = rec.Variant_filter
	item.Bin_filter = rec.Bin_filter
}

// setItemEntry sets the values of one row of Most Recently Posted: the entry's date, document
// no., quantity, unit amount and amount (a blank entry: none found)
func setItemEntry(page *pages.Control, prefix string, itemLedgEntry *gtables.ItemLedgerEntry, quantity, amount types.Decimal) {
	page.SetValue(prefix+"_date", itemLedgEntry.Posting_date)
	page.SetValue(prefix+"_document_no", itemLedgEntry.Document_no)
	page.SetValue(prefix+"_quantity", quantity)
	page.SetValue(prefix+"_unit_amount", itemLedgEntry.Unit_amount)
	page.SetValue(prefix+"_amount", amount)
}

// itemEntryStats holds what form 304 shows: ItemLedgEntry[1..6] (Purchase, Sale, Positive
// Adjustment, Negative Adjustment, Receipt, Shipment; index 0 unused, as the classic arrays
// start at 1), the next and last purchase order line and the next and last sales order line
type itemEntryStats struct {
	itemLedgEntry  [7]*gtables.ItemLedgerEntry
	purchOrderLine [2]*gtables.PurchaseLine
	salesLine      [2]*gtables.SalesLine
}

// copyItemFilter is COPYFILTER of an item FlowFilter onto a field of another record: the
// FlowFilter's expression, or no filter when it has none
func copyItemFilter(expression string, to ftables.FilterTarget, toFieldName string) {
	if expression != "" {
		to.SetFilter(toFieldName, expression)
	} else {
		to.SetRange(toFieldName)
	}
}

// itemEntryStatistics is form 304's OnAfterGetRecord: per entry type the latest entry within
// the item's Variant, Drop Shipment, Location and Bin Filters (purchases and sales: the latest
// invoiced and the latest received/shipped), and the outstanding order lines with the
// earliest and the latest receipt/shipment date
func itemEntryStatistics(rec *gtables.Item) itemEntryStats {
	var s itemEntryStats
	for k := range s.itemLedgEntry {
		s.itemLedgEntry[k] = gtables.NewItemLedgerEntry(rec)
	}
	for k := range s.purchOrderLine {
		s.purchOrderLine[k] = gtables.NewPurchaseLine(rec)
		s.salesLine[k] = gtables.NewSalesLine(rec)
	}

	itemLedgEntry2 := gtables.NewItemLedgerEntry(rec)
	itemLedgEntry2.SetCurrentKey("entry_type", "item_no", "variant_code", "drop_shipment", "location_code", "posting_date")
	itemLedgEntry2.SetRange("item_no", rec.No)
	copyFilters := func() {
		copyItemFilter(rec.Variant_filter, itemLedgEntry2, "variant_code")
		copyItemFilter(rec.Drop_shipment_filter, itemLedgEntry2, "drop_shipment")
		copyItemFilter(rec.Location_filter, itemLedgEntry2, "location_code")
		copyItemFilter(rec.Bin_filter, itemLedgEntry2, "bin_code")
	}
	for j := 1; j <= 4; j++ {
		itemLedgEntry2.SetRange("entry_type", gtables.ItemLedgerEntryEntry_type(j-1)) // Purchase,Sale,Positive Adjmt.,Negative Adjmt.
		copyFilters()
		if j == 1 || j == 2 { // Purchase,Sale
			itemLedgEntry2.SetFilter("invoiced_quantity", "<>0")
			s.findLastItemEntry(j, itemLedgEntry2, copyFilters)
			itemLedgEntry2.SetRange("invoiced_quantity")
			itemLedgEntry2.SetFilter("quantity", "<>0")
			s.findLastItemEntry(j+4, itemLedgEntry2, copyFilters) // Receipt,Shipment
			itemLedgEntry2.SetRange("quantity")
		} else {
			s.findLastItemEntry(j, itemLedgEntry2, copyFilters)
		}
	}

	purchLine2 := gtables.NewPurchaseLine(rec)
	purchLine2.SetRange("document_type", gtables.PurchaseLine_Document_type.Order)
	purchLine2.SetRange("type", gtables.PurchaseLine_Type.Item)
	purchLine2.SetRange("no", rec.No)
	purchLine2.SetFilter("outstanding_quantity", "<>0")
	copyItemFilter(rec.Variant_filter, purchLine2, "variant_code")
	copyItemFilter(rec.Location_filter, purchLine2, "location_code")
	copyItemFilter(rec.Bin_filter, purchLine2, "bin_code")
	for found := purchLine2.FindSet(); found; found = purchLine2.Next() {
		if s.purchOrderLine[0].Document_no == "" || purchLine2.Expected_receipt_date.Before(s.purchOrderLine[0].Expected_receipt_date) ||
			(purchLine2.Expected_receipt_date.Equal(s.purchOrderLine[0].Expected_receipt_date) &&
				purchLine2.Document_no < s.purchOrderLine[0].Document_no) {
			s.purchOrderLine[0].Copy(purchLine2)
		}
		if s.purchOrderLine[1].Document_no == "" || purchLine2.Expected_receipt_date.After(s.purchOrderLine[1].Expected_receipt_date) ||
			(purchLine2.Expected_receipt_date.Equal(s.purchOrderLine[1].Expected_receipt_date) &&
				purchLine2.Document_no > s.purchOrderLine[1].Document_no) {
			s.purchOrderLine[1].Copy(purchLine2)
		}
	}

	salesLine2 := gtables.NewSalesLine(rec)
	salesLine2.SetRange("document_type", gtables.SalesLine_Document_type.Order)
	salesLine2.SetRange("type", gtables.SalesLine_Type.Item)
	salesLine2.SetRange("no", rec.No)
	copyItemFilter(rec.Variant_filter, salesLine2, "variant_code")
	salesLine2.SetFilter("outstanding_quantity", "<>0")
	copyItemFilter(rec.Location_filter, salesLine2, "location_code")
	copyItemFilter(rec.Bin_filter, salesLine2, "bin_code")
	for found := salesLine2.FindSet(); found; found = salesLine2.Next() {
		if s.salesLine[0].Document_no == "" || salesLine2.Shipment_date.Before(s.salesLine[0].Shipment_date) ||
			(salesLine2.Shipment_date.Equal(s.salesLine[0].Shipment_date) &&
				salesLine2.Document_no < s.salesLine[0].Document_no) {
			s.salesLine[0].Copy(salesLine2)
		}
		if s.salesLine[1].Document_no == "" || salesLine2.Shipment_date.After(s.salesLine[1].Shipment_date) ||
			(salesLine2.Shipment_date.Equal(s.salesLine[1].Shipment_date) &&
				salesLine2.Document_no > s.salesLine[1].Document_no) {
			s.salesLine[1].Copy(salesLine2)
		}
	}
	return s
}

// findLastItemEntry (FindLastItemEntry): the latest entry (posting date, then entry no.) of
// each variant, drop shipment, location and bin within the filters becomes ItemLedgEntry[k]
// when it is later than the one found so far
func (s *itemEntryStats) findLastItemEntry(k int, itemLedgEntry2 *gtables.ItemLedgerEntry, copyFilters func()) {
	for found := itemLedgEntry2.Find("-"); found; found = itemLedgEntry2.Next() {
		itemLedgEntry2.SetRange("variant_code", itemLedgEntry2.Variant_code)
		itemLedgEntry2.SetRange("drop_shipment", itemLedgEntry2.Drop_shipment)
		itemLedgEntry2.SetRange("location_code", itemLedgEntry2.Location_code)
		itemLedgEntry2.SetRange("bin_code", itemLedgEntry2.Bin_code)
		itemLedgEntry2.Find("+")

		if itemLedgEntry2.Posting_date.After(s.itemLedgEntry[k].Posting_date) ||
			(itemLedgEntry2.Posting_date.Equal(s.itemLedgEntry[k].Posting_date) &&
				itemLedgEntry2.Entry_no > s.itemLedgEntry[k].Entry_no) {
			s.itemLedgEntry[k].Copy(itemLedgEntry2)
		}

		copyFilters()
	}
}
