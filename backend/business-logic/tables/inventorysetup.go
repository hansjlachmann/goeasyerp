package tables

import (
	gtables "github.com/hansjlachmann/goeasyerp/backend/generated/tables"
)

// inventorySetupLogic is the business logic of table 313 Inventory Setup (none: the table's relations and
// required fields are its rules)
type inventorySetupLogic struct{}

func init() { gtables.RegisterInventorySetupLogic(inventorySetupLogic{}) }
