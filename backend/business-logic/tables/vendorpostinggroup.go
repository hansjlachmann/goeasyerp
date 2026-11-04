package tables

import (
	gtables "github.com/hansjlachmann/goeasyerp/backend/generated/tables"
)

// vendorPostingGroupLogic is the business logic of table 93 Vendor Posting Group
type vendorPostingGroupLogic struct{}

func init() { gtables.RegisterVendorPostingGroupLogic(vendorPostingGroupLogic{}) }

// OnValidate_Service_charge_acc: a posting account with a Gen. Prod. Posting Group and Direct Posting
func (vendorPostingGroupLogic) OnValidate_Service_charge_acc(rec *gtables.VendorPostingGroup) error {
	return checkPostingGroupGLAcc(rec, rec.Service_charge_acc, true, true)
}

// OnValidate_Payment_disc_acc: a posting account
func (vendorPostingGroupLogic) OnValidate_Payment_disc_acc(rec *gtables.VendorPostingGroup) error {
	return checkPostingGroupGLAcc(rec, rec.Payment_disc_acc, false, false)
}

// OnValidate_Invoice_rounding_account: a posting account with a Gen. Prod. Posting Group
func (vendorPostingGroupLogic) OnValidate_Invoice_rounding_account(rec *gtables.VendorPostingGroup) error {
	return checkPostingGroupGLAcc(rec, rec.Invoice_rounding_account, true, false)
}
