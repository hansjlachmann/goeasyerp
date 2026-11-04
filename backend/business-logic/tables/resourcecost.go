package tables

import (
	apperrors "github.com/hansjlachmann/goeasyerp/backend/foundation/errors"
	gtables "github.com/hansjlachmann/goeasyerp/backend/generated/tables"
)

// resourceCostLogic is the business logic of table 202 Resource Cost.
type resourceCostLogic struct{}

func init() { gtables.RegisterResourceCostLogic(resourceCostLogic{}) }

// OnValidate_Code: a cost for all resources has no code
func (resourceCostLogic) OnValidate_Code(rec *gtables.ResourceCost) error {
	if rec.Code != "" && rec.Type == gtables.ResourceCost_Type.All {
		return apperrors.ResCodeWhenTypeAll(gtables.ResourceCostTableName, "code", "type", rec.Type)
	}
	return nil
}

// OnValidate_Cost_type: without a work type the cost is a fixed cost
func (resourceCostLogic) OnValidate_Cost_type(rec *gtables.ResourceCost) error {
	if rec.Work_type_code == "" {
		return rec.TestField("cost_type", gtables.ResourceCost_Cost_type.Fixed)
	}
	return nil
}
