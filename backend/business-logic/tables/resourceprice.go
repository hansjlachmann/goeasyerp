package tables

import (
	apperrors "github.com/hansjlachmann/goeasyerp/backend/foundation/errors"
	gtables "github.com/hansjlachmann/goeasyerp/backend/generated/tables"
)

// resourcePriceLogic is the business logic of table 201 Resource Price.
type resourcePriceLogic struct{}

func init() { gtables.RegisterResourcePriceLogic(resourcePriceLogic{}) }

// OnValidate_Code: a price for all resources has no code
func (resourcePriceLogic) OnValidate_Code(rec *gtables.ResourcePrice) error {
	if rec.Code != "" && rec.Type == gtables.ResourcePrice_Type.All {
		return apperrors.ResCodeWhenTypeAll(gtables.ResourcePriceTableName, "code", "type", rec.Type)
	}
	return nil
}
