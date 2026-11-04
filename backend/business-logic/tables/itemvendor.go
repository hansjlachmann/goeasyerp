package tables

import (
	gtables "github.com/hansjlachmann/goeasyerp/backend/generated/tables"
)

// itemVendorLogic is the business logic of table 99 Item Vendor
type itemVendorLogic struct{}

func init() { gtables.RegisterItemVendorLogic(itemVendorLogic{}) }

// OnValidate_Vendor_no: the vendor's currency
func (itemVendorLogic) OnValidate_Vendor_no(rec *gtables.ItemVendor) error {
	vend := gtables.NewVendor(rec)
	err := vend.GetOrError(rec.Vendor_no)
	if err != nil {
		return err
	}
	rec.Currency_code = vend.Currency_code
	return nil
}
