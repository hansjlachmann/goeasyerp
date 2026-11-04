// Package itemprice is codeunit 52 Item-Find Price: the sales price of an item for a price
// group, currency, unit of measure and date.
package itemprice

import (
	"github.com/hansjlachmann/goeasyerp/backend/foundation/types"
	gtables "github.com/hansjlachmann/goeasyerp/backend/generated/tables"
)

// FindPrice (Run with an Item Price record): the item's price with the latest starting date
// up to the record's Starting Date, for its price group, currency and unit of measure or
// without them (the most specific in the key order wins); without one the item's own Unit
// Price, tax and invoice discount setting. rec gets the price found.
func FindPrice(rec *gtables.ItemPrice) error {
	itemPrice := gtables.NewItemPrice(rec)
	itemPrice.SetCurrentKey("item_no", "price_group_code", "unit_of_measure_code", "currency_code", "starting_date")
	itemPrice.SetRange("item_no", rec.Item_no)
	itemPrice.SetFilter("price_group_code", rec.Price_group_code.String()+"|''")
	itemPrice.SetFilter("currency_code", rec.Currency_code.String()+"|''")
	itemPrice.SetFilter("unit_of_measure_code", rec.Unit_of_measure_code.String()+"|''")
	itemPrice.SetRange("starting_date", types.Date{}, rec.Starting_date)
	if !itemPrice.FindLast() {
		item := gtables.NewItem(rec)
		err := item.GetOrError(rec.Item_no)
		if err != nil {
			return err
		}
		itemPrice.Item_no = rec.Item_no
		itemPrice.Price_group_code = ""
		itemPrice.Currency_code = ""
		itemPrice.Unit_of_measure_code = ""
		itemPrice.Starting_date = types.Date{}
		itemPrice.Unit_price = item.Unit_price
		itemPrice.Price_includes_tax = item.Price_includes_tax
		itemPrice.Tax_bus_posting_gr_price = item.Tax_bus_posting_gr_price
		itemPrice.Allow_quantity_disc = true
		itemPrice.Allow_cust_item_disc = true
		itemPrice.Allow_invoice_disc = item.Allow_invoice_disc
	}
	rec.Price_group_code = itemPrice.Price_group_code
	rec.Currency_code = itemPrice.Currency_code
	rec.Unit_of_measure_code = itemPrice.Unit_of_measure_code
	rec.Starting_date = itemPrice.Starting_date
	rec.Unit_price = itemPrice.Unit_price
	rec.Price_includes_tax = itemPrice.Price_includes_tax
	rec.Tax_bus_posting_gr_price = itemPrice.Tax_bus_posting_gr_price
	rec.Allow_quantity_disc = itemPrice.Allow_quantity_disc
	rec.Allow_cust_item_disc = itemPrice.Allow_cust_item_disc
	rec.Allow_invoice_disc = itemPrice.Allow_invoice_disc
	return nil
}
