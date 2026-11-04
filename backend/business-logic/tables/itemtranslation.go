package tables

import (
	gtables "github.com/hansjlachmann/goeasyerp/backend/generated/tables"
)

// itemTranslationLogic is the business logic of table 30 Item Translation (none: the table's relations and
// required fields are its rules)
type itemTranslationLogic struct{}

func init() { gtables.RegisterItemTranslationLogic(itemTranslationLogic{}) }
