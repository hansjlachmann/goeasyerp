package tables

import (
	gtables "github.com/hansjlachmann/goeasyerp/backend/generated/tables"
)

// custInvoiceDiscLogic is the business logic of table 19 Cust_Invoice_Disc.
// Add a trigger when it is needed: the available triggers are listed at the
// top of generated/tables/custinvoicedisc_gen.go. The field rules of the definition
// (required, length, range) are checked after OnInsert and OnModify.
type custInvoiceDiscLogic struct{}

func init() { gtables.RegisterCustInvoiceDiscLogic(custInvoiceDiscLogic{}) }

// OnInsert runs before a new record is inserted
func (custInvoiceDiscLogic) OnInsert(rec *gtables.CustInvoiceDisc) error {
	return nil
}

// OnModify runs before a record is modified
func (custInvoiceDiscLogic) OnModify(rec *gtables.CustInvoiceDisc) error {
	return nil
}

// OnDelete runs before a record is deleted
func (custInvoiceDiscLogic) OnDelete(rec *gtables.CustInvoiceDisc) error {
	return nil
}

// OnRename runs before a primary key field is changed (rec.OldValue has the old value)
func (custInvoiceDiscLogic) OnRename(rec *gtables.CustInvoiceDisc) error {
	return nil
}
