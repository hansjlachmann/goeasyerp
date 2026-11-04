package tables

import (
	gtables "github.com/hansjlachmann/goeasyerp/backend/generated/tables"
)

// custItemDiscountLogic is the business logic of table 20 Cust./Item Discount (none: the table's relations, ranges and
// required fields are its rules)
type custItemDiscountLogic struct{}

func init() { gtables.RegisterCustItemDiscountLogic(custItemDiscountLogic{}) }
