package tables

import (
	gtables "github.com/hansjlachmann/goeasyerp/backend/generated/tables"
)

// itemSalesQtyDiscLogic is the business logic of table 29 Item Sales Qty. Disc. (none: the table's relations, ranges and
// required fields are its rules)
type itemSalesQtyDiscLogic struct{}

func init() { gtables.RegisterItemSalesQtyDiscLogic(itemSalesQtyDiscLogic{}) }
