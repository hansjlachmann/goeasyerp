package tables

import (
	gtables "github.com/hansjlachmann/goeasyerp/backend/generated/tables"
)

// unitOfMeasureTranslationLogic is the business logic of table 5402 Unit of Measure Translation
// (none: the table's relations and required fields are its rules)
type unitOfMeasureTranslationLogic struct{}

func init() { gtables.RegisterUnitOfMeasureTranslationLogic(unitOfMeasureTranslationLogic{}) }
