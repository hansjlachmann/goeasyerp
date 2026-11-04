package tables

import (
	gtables "github.com/hansjlachmann/goeasyerp/backend/generated/tables"
)

// inventoryPostingGroupLogic is the business logic of table 94 Inventory Posting Group
type inventoryPostingGroupLogic struct{}

func init() { gtables.RegisterInventoryPostingGroupLogic(inventoryPostingGroupLogic{}) }

// OnValidate_Inventory_account: a posting account that is not blocked
func (inventoryPostingGroupLogic) OnValidate_Inventory_account(rec *gtables.InventoryPostingGroup) error {
	return checkSetupGLAcc(rec, rec.Inventory_account)
}
