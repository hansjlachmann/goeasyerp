package tables

import (
	"github.com/hansjlachmann/goeasyerp/backend/foundation/flowfilter"
	"github.com/hansjlachmann/goeasyerp/backend/foundation/types"
	gtables "github.com/hansjlachmann/goeasyerp/backend/generated/tables"
)

// ExpectedQtyOnHand (codeunit 5400 Available Management): the item's quantity on hand plus what
// the purchase orders bring and less what the sales orders take up to the end of its Date Filter
// (less extraNetNeed), in its Location/Variant/Bin/Dimension Filters. With calcAvailable,
// available is the quantity on hand less every sales order up to plannedOrderReceiptDate (blank:
// all) plus the purchase orders up to the end of the Date Filter. calcQtyOnHand calculates the
// quantity on hand, otherwise the item's is used. Production (scheduled receipts and needs) is
// not migrated.
func ExpectedQtyOnHand(newItem *gtables.Item, calcQtyOnHand, calcAvailable bool, extraNetNeed types.Decimal, available *types.Decimal, plannedOrderReceiptDate types.Date) types.Decimal {
	item := gtables.NewItem(newItem)
	item.Copy(newItem)
	for _, f := range newItem.FlowFilterExpressions() {
		_ = item.SetFlowFilter(f.Field, f.Expression)
	}
	upToDateFilter := flowfilter.UpperLimit(flowfilter.KindDate, newItem.Date_filter)

	*available = types.Decimal{}

	if calcQtyOnHand {
		item.CalcFields("quantity_on_hand")
	}

	if calcAvailable {
		if plannedOrderReceiptDate.IsZero() {
			_ = item.SetFlowFilter("date_filter", "")
		} else {
			_ = item.SetFlowFilter("date_filter", ".."+plannedOrderReceiptDate.String())
		}
		item.CalcFields("qty_on_sales_order")
		initialQtyAvailable := item.Quantity_on_hand.Sub(item.Qty_on_sales_order).Sub(extraNetNeed)

		_ = item.SetFlowFilter("date_filter", upToDateFilter)
		item.CalcFields("qty_on_purch_order")

		*available = initialQtyAvailable.Add(item.Qty_on_purch_order)
	}

	_ = item.SetFlowFilter("date_filter", upToDateFilter)

	item.CalcFields("qty_on_purch_order", "qty_on_sales_order")

	return item.Quantity_on_hand.Add(item.Qty_on_purch_order).Sub(item.Qty_on_sales_order).Sub(extraNetNeed)
}
