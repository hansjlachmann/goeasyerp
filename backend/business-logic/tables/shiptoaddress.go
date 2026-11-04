package tables

import (
	"github.com/hansjlachmann/goeasyerp/backend/foundation/system"
	gtables "github.com/hansjlachmann/goeasyerp/backend/generated/tables"
)

// shipToAddressLogic is the business logic of table 222 Ship-to Address
type shipToAddressLogic struct{}

func init() { gtables.RegisterShipToAddressLogic(shipToAddressLogic{}) }

// OnInsert: a new ship-to address gets the customer's name
func (shipToAddressLogic) OnInsert(rec *gtables.ShipToAddress) error {
	cust := gtables.NewCustomer(rec)
	err := cust.GetOrError(rec.Customer_no)
	if err != nil {
		return err
	}
	rec.Name = cust.Name
	return nil
}

// OnModify sets Last Date Modified
func (shipToAddressLogic) OnModify(rec *gtables.ShipToAddress) error {
	rec.Last_date_modified = system.Today()
	return nil
}

// OnRename sets Last Date Modified
func (shipToAddressLogic) OnRename(rec *gtables.ShipToAddress) error {
	rec.Last_date_modified = system.Today()
	return nil
}

// OnValidate_Post_code: a known post code gives the city
func (shipToAddressLogic) OnValidate_Post_code(rec *gtables.ShipToAddress) error {
	postCode := gtables.NewPostCode(rec)
	if rec.Post_code != "" && postCode.Get(rec.Post_code) {
		rec.City = postCode.City
	}
	return nil
}
