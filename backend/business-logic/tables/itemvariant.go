package tables

import (
	gtables "github.com/hansjlachmann/goeasyerp/backend/generated/tables"
)

// itemVariantLogic is the business logic of table 5401 Item Variant (none: the table's relations and
// required fields are its rules)
type itemVariantLogic struct{}

func init() { gtables.RegisterItemVariantLogic(itemVariantLogic{}) }
