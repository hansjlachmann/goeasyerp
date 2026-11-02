package tables

import (
	gtables "github.com/hansjlachmann/openerp/backend/generated/tables"
)

// itemPriceChangeLogic is the business logic of table 40 Item_Price_Change.
// Add a trigger when it is needed: the available triggers are listed at the
// top of generated/tables/itempricechange_gen.go. The field rules of the definition
// (required, length, range) are checked after OnInsert and OnModify.
type itemPriceChangeLogic struct{}

func init() { gtables.RegisterItemPriceChangeLogic(itemPriceChangeLogic{}) }

// OnInsert runs before a new record is inserted
func (itemPriceChangeLogic) OnInsert(rec *gtables.ItemPriceChange) error {
	return nil
}

// OnModify runs before a record is modified
func (itemPriceChangeLogic) OnModify(rec *gtables.ItemPriceChange) error {
	return nil
}

// OnDelete runs before a record is deleted
func (itemPriceChangeLogic) OnDelete(rec *gtables.ItemPriceChange) error {
	return nil
}

// OnRename runs before a primary key field is changed (rec.OldValue has the old value)
func (itemPriceChangeLogic) OnRename(rec *gtables.ItemPriceChange) error {
	return nil
}

// OnValidate_Price_group_code: a price group's suggested price takes the group's tax and
// discount settings
func (itemPriceChangeLogic) OnValidate_Price_group_code(rec *gtables.ItemPriceChange) error {
	if rec.Price_group_code != "" {
		priceGr := gtables.NewPriceGroup(rec)
		err := priceGr.GetOrError(rec.Price_group_code)
		if err != nil {
			return err
		}
		rec.Price_includes_tax = priceGr.Price_includes_tax
		rec.Allow_quantity_disc = priceGr.Allow_quantity_disc
		rec.Allow_cust_item_disc = priceGr.Allow_cust_item_disc
		rec.Allow_invoice_disc = priceGr.Allow_invoice_disc
	}
	return nil
}
