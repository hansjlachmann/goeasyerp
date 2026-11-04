package tables

import (
	gtables "github.com/hansjlachmann/goeasyerp/backend/generated/tables"
)

// resourceGroupLogic is the business logic of table 152 Resource Group.
type resourceGroupLogic struct{}

func init() { gtables.RegisterResourceGroupLogic(resourceGroupLogic{}) }

// OnDelete deletes the group's capacity entries, costs, prices and comment lines
func (resourceGroupLogic) OnDelete(rec *gtables.ResourceGroup) error {
	resCapacityEntry := gtables.NewResCapacityEntry(rec)
	resCapacityEntry.SetCurrentKey("resource_group_no")
	resCapacityEntry.SetRange("resource_group_no", rec.No)
	resCapacityEntry.DeleteAll()

	resCost := gtables.NewResourceCost(rec)
	resCost.SetRange("type", gtables.ResourceCost_Type.GroupResource)
	resCost.SetRange("code", rec.No)
	resCost.DeleteAll()

	resPrice := gtables.NewResourcePrice(rec)
	resPrice.SetRange("type", gtables.ResourcePrice_Type.GroupResource)
	resPrice.SetRange("code", rec.No)
	resPrice.DeleteAll()

	DeleteComments(rec, gtables.CommentLine_Table_name.ResourceGroup, rec.No)
	return nil
}
