package tables

import (
	gtables "github.com/hansjlachmann/goeasyerp/backend/generated/tables"
)

// itemPriceLogic is the business logic of table 28 Item Price
type itemPriceLogic struct{}

func init() { gtables.RegisterItemPriceLogic(itemPriceLogic{}) }

// OnValidate_Price_group_code: a price group's price takes the group's tax and discount settings
func (itemPriceLogic) OnValidate_Price_group_code(rec *gtables.ItemPrice) error {
	if rec.Price_group_code != "" {
		priceGr := gtables.NewPriceGroup(rec)
		err := priceGr.GetOrError(rec.Price_group_code)
		if err != nil {
			return err
		}
		rec.Price_includes_tax = priceGr.Price_includes_tax
		rec.Tax_bus_posting_gr_price = priceGr.Tax_bus_posting_gr_price
		rec.Allow_quantity_disc = priceGr.Allow_quantity_disc
		rec.Allow_cust_item_disc = priceGr.Allow_cust_item_disc
		rec.Allow_invoice_disc = priceGr.Allow_invoice_disc
	}
	return nil
}
