package tables

import (
	gtables "github.com/hansjlachmann/goeasyerp/backend/generated/tables"
)

// vendorInvoiceDiscLogic is the business logic of table 24 Vendor_Invoice_Disc.
// Add a trigger when it is needed: the available triggers are listed at the
// top of generated/tables/vendorinvoicedisc_gen.go. The field rules of the definition
// (required, length, range) are checked after OnInsert and OnModify.
type vendorInvoiceDiscLogic struct{}

func init() { gtables.RegisterVendorInvoiceDiscLogic(vendorInvoiceDiscLogic{}) }

// OnInsert runs before a new record is inserted
func (vendorInvoiceDiscLogic) OnInsert(rec *gtables.VendorInvoiceDisc) error {
	return nil
}

// OnModify runs before a record is modified
func (vendorInvoiceDiscLogic) OnModify(rec *gtables.VendorInvoiceDisc) error {
	return nil
}

// OnDelete runs before a record is deleted
func (vendorInvoiceDiscLogic) OnDelete(rec *gtables.VendorInvoiceDisc) error {
	return nil
}

// OnRename runs before a primary key field is changed (rec.OldValue has the old value)
func (vendorInvoiceDiscLogic) OnRename(rec *gtables.VendorInvoiceDisc) error {
	return nil
}
