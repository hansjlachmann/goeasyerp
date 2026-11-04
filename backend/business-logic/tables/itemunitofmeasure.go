package tables

import (
	apperrors "github.com/hansjlachmann/goeasyerp/backend/foundation/errors"
	"github.com/hansjlachmann/goeasyerp/backend/foundation/types"
	gtables "github.com/hansjlachmann/goeasyerp/backend/generated/tables"
)

// itemUnitOfMeasureLogic is the business logic of table 5404 Item Unit of Measure
type itemUnitOfMeasureLogic struct{}

func init() { gtables.RegisterItemUnitOfMeasureLogic(itemUnitOfMeasureLogic{}) }

// OnValidate_Qty_per_unit_of_measure: greater than 0; the item's base unit — and every unit of
// an item with Costing Method Specific — is 1
func (itemUnitOfMeasureLogic) OnValidate_Qty_per_unit_of_measure(rec *gtables.ItemUnitOfMeasure) error {
	if !rec.Qty_per_unit_of_measure.IsPositive() {
		return apperrors.ItemUoMQtyNotPositive(gtables.ItemUnitOfMeasureTableName, "qty_per_unit_of_measure")
	}
	item := gtables.NewItem(rec)
	err := item.GetOrError(rec.Item_no)
	if err != nil {
		return err
	}
	if item.Base_unit_of_measure == rec.Code || item.Costing_method == gtables.Item_Costing_method.Specific {
		return rec.TestField("qty_per_unit_of_measure", types.NewDecimalFromInt(1))
	}
	return nil
}
