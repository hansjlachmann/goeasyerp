package tables

import (
	gtables "github.com/hansjlachmann/goeasyerp/backend/generated/tables"
)

// itemPurchQtyDiscLogic is the business logic of table 100 Item_Purch_Qty_Disc.
// Add a trigger when it is needed: the available triggers are listed at the
// top of generated/tables/itempurchqtydisc_gen.go. The field rules of the definition
// (required, length, range) are checked after OnInsert and OnModify.
type itemPurchQtyDiscLogic struct{}

func init() { gtables.RegisterItemPurchQtyDiscLogic(itemPurchQtyDiscLogic{}) }

// OnInsert runs before a new record is inserted
func (itemPurchQtyDiscLogic) OnInsert(rec *gtables.ItemPurchQtyDisc) error {
	return nil
}

// OnModify runs before a record is modified
func (itemPurchQtyDiscLogic) OnModify(rec *gtables.ItemPurchQtyDisc) error {
	return nil
}

// OnDelete runs before a record is deleted
func (itemPurchQtyDiscLogic) OnDelete(rec *gtables.ItemPurchQtyDisc) error {
	return nil
}

// OnRename runs before a primary key field is changed (rec.OldValue has the old value)
func (itemPurchQtyDiscLogic) OnRename(rec *gtables.ItemPurchQtyDisc) error {
	return nil
}
