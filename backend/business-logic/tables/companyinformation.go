package tables

import (
	gtables "github.com/hansjlachmann/goeasyerp/backend/generated/tables"
)

// companyInformationLogic is the business logic of table 79 Company Information: a known post
// code gives the city (also for the ship-to address)
type companyInformationLogic struct{}

func init() { gtables.RegisterCompanyInformationLogic(companyInformationLogic{}) }

// OnValidate_Post_code: a known post code gives the city
func (companyInformationLogic) OnValidate_Post_code(rec *gtables.CompanyInformation) error {
	postCode := gtables.NewPostCode(rec)
	if rec.Post_code != "" && postCode.Get(rec.Post_code) {
		rec.City = postCode.City
	}
	return nil
}

// OnValidate_Ship_to_post_code: a known post code gives the ship-to city
func (companyInformationLogic) OnValidate_Ship_to_post_code(rec *gtables.CompanyInformation) error {
	postCode := gtables.NewPostCode(rec)
	if rec.Ship_to_post_code != "" && postCode.Get(rec.Ship_to_post_code) {
		rec.Ship_to_city = postCode.City
	}
	return nil
}
