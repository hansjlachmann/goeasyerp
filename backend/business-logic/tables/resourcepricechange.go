package tables

import (
	apperrors "github.com/hansjlachmann/goeasyerp/backend/foundation/errors"
	gtables "github.com/hansjlachmann/goeasyerp/backend/generated/tables"
)

// resourcePriceChangeLogic is the business logic of table 335 Resource Price Change.
type resourcePriceChangeLogic struct{}

func init() { gtables.RegisterResourcePriceChangeLogic(resourcePriceChangeLogic{}) }

// OnValidate_Code: a price change for all resources has no code
func (resourcePriceChangeLogic) OnValidate_Code(rec *gtables.ResourcePriceChange) error {
	if rec.Code != "" && rec.Type == gtables.ResourcePriceChange_Type.All {
		return apperrors.ResCodeWhenTypeAll(gtables.ResourcePriceChangeTableName, "code", "type", rec.Type)
	}
	return nil
}
