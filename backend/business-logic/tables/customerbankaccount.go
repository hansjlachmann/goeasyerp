package tables

import (
	gtables "github.com/hansjlachmann/goeasyerp/backend/generated/tables"
)

// customerBankAccountLogic is the business logic of table 287 Customer Bank Account
type customerBankAccountLogic struct{}

func init() { gtables.RegisterCustomerBankAccountLogic(customerBankAccountLogic{}) }

// OnValidate_Post_code: a known post code gives the city
func (customerBankAccountLogic) OnValidate_Post_code(rec *gtables.CustomerBankAccount) error {
	postCode := gtables.NewPostCode(rec)
	if rec.Post_code != "" && postCode.Get(rec.Post_code) {
		rec.City = postCode.City
	}
	return nil
}
