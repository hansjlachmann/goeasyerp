package tables

import (
	"github.com/hansjlachmann/goeasyerp/backend/foundation/system"
	gtables "github.com/hansjlachmann/goeasyerp/backend/generated/tables"
)

// orderAddressLogic is the business logic of table 224 Order Address
type orderAddressLogic struct{}

func init() { gtables.RegisterOrderAddressLogic(orderAddressLogic{}) }

// OnInsert: a new order address gets the vendor's name
func (orderAddressLogic) OnInsert(rec *gtables.OrderAddress) error {
	vend := gtables.NewVendor(rec)
	err := vend.GetOrError(rec.Vendor_no)
	if err != nil {
		return err
	}
	rec.Name = vend.Name
	return nil
}

// OnModify sets Last Date Modified
func (orderAddressLogic) OnModify(rec *gtables.OrderAddress) error {
	rec.Last_date_modified = system.Today()
	return nil
}

// OnRename sets Last Date Modified
func (orderAddressLogic) OnRename(rec *gtables.OrderAddress) error {
	rec.Last_date_modified = system.Today()
	return nil
}

// OnValidate_Post_code: a known post code gives the city
func (orderAddressLogic) OnValidate_Post_code(rec *gtables.OrderAddress) error {
	postCode := gtables.NewPostCode(rec)
	if rec.Post_code != "" && postCode.Get(rec.Post_code) {
		rec.City = postCode.City
	}
	return nil
}
