package tables

import (
	gtables "github.com/hansjlachmann/goeasyerp/backend/generated/tables"
)

// unitOfMeasureLogic is the business logic of table 204 Unit of Measure
type unitOfMeasureLogic struct{}

func init() { gtables.RegisterUnitOfMeasureLogic(unitOfMeasureLogic{}) }

// OnDelete deletes the unit's translations
func (unitOfMeasureLogic) OnDelete(rec *gtables.UnitOfMeasure) error {
	unitOfMeasureTranslation := gtables.NewUnitOfMeasureTranslation(rec)
	unitOfMeasureTranslation.SetRange("code", rec.Code)
	unitOfMeasureTranslation.DeleteAll()
	return nil
}
