package tables

import (
	gtables "github.com/hansjlachmann/goeasyerp/backend/generated/tables"
)

// vendorBankAccountLogic is the business logic of table 288 Vendor Bank Account
type vendorBankAccountLogic struct{}

func init() { gtables.RegisterVendorBankAccountLogic(vendorBankAccountLogic{}) }

// OnValidate_Post_code: a known post code gives the city
func (vendorBankAccountLogic) OnValidate_Post_code(rec *gtables.VendorBankAccount) error {
	postCode := gtables.NewPostCode(rec)
	if rec.Post_code != "" && postCode.Get(rec.Post_code) {
		rec.City = postCode.City
	}
	return nil
}
