package tables

import (
	gtables "github.com/hansjlachmann/goeasyerp/backend/generated/tables"
)

// paymentTermsLogic is the business logic of table 3 Payment Terms.
// Add a trigger when it is needed: the available triggers are listed at the
// top of generated/tables/paymentterms_gen.go. The field rules of the definition
// (required, length, range) are checked after OnInsert and OnModify.
type paymentTermsLogic struct{}

func init() { gtables.RegisterPaymentTermsLogic(paymentTermsLogic{}) }

// OnInsert runs before a new record is inserted
func (paymentTermsLogic) OnInsert(rec *gtables.PaymentTerms) error {
	return nil
}

// OnModify runs before a record is modified
func (paymentTermsLogic) OnModify(rec *gtables.PaymentTerms) error {
	return nil
}

// OnDelete runs before a record is deleted
func (paymentTermsLogic) OnDelete(rec *gtables.PaymentTerms) error {
	return nil
}

// OnRename runs before a primary key field is changed (rec.OldValue has the old value)
func (paymentTermsLogic) OnRename(rec *gtables.PaymentTerms) error {
	return nil
}
