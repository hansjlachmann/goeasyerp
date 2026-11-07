package tables

import (
	gtables "github.com/hansjlachmann/goeasyerp/backend/generated/tables"
)

// financeChargeTermsLogic is the business logic of table 5 Finance_Charge_Terms.
// Add a trigger when it is needed: the available triggers are listed at the
// top of generated/tables/financechargeterms_gen.go. The field rules of the definition
// (required, length, range) are checked after OnInsert and OnModify.
type financeChargeTermsLogic struct{}

func init() { gtables.RegisterFinanceChargeTermsLogic(financeChargeTermsLogic{}) }

// OnInsert runs before a new record is inserted
func (financeChargeTermsLogic) OnInsert(rec *gtables.FinanceChargeTerms) error {
	return nil
}

// OnModify runs before a record is modified
func (financeChargeTermsLogic) OnModify(rec *gtables.FinanceChargeTerms) error {
	return nil
}

// OnDelete deletes the terms' texts
func (financeChargeTermsLogic) OnDelete(rec *gtables.FinanceChargeTerms) error {
	finChrgText := gtables.NewFinanceChargeText(rec)
	finChrgText.SetRange("fin_charge_terms_code", rec.Code)
	finChrgText.DeleteAll()
	return nil
}

// OnRename runs before a primary key field is changed (rec.OldValue has the old value)
func (financeChargeTermsLogic) OnRename(rec *gtables.FinanceChargeTerms) error {
	return nil
}
