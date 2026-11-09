package tables

import (
	gtables "github.com/hansjlachmann/goeasyerp/backend/generated/tables"
)

// currencyForFinChargeTermsLogic is the business logic of table 328 Currency_for_Fin_Charge_Terms.
// Add a trigger when it is needed: the available triggers are listed at the
// top of generated/tables/currencyforfinchargeterms_gen.go. The field rules of the definition
// (required, length, range) are checked after OnInsert and OnModify.
type currencyForFinChargeTermsLogic struct{}

func init() { gtables.RegisterCurrencyForFinChargeTermsLogic(currencyForFinChargeTermsLogic{}) }

// OnInsert runs before a new record is inserted
func (currencyForFinChargeTermsLogic) OnInsert(rec *gtables.CurrencyForFinChargeTerms) error {
	return nil
}

// OnModify runs before a record is modified
func (currencyForFinChargeTermsLogic) OnModify(rec *gtables.CurrencyForFinChargeTerms) error {
	return nil
}

// OnDelete runs before a record is deleted
func (currencyForFinChargeTermsLogic) OnDelete(rec *gtables.CurrencyForFinChargeTerms) error {
	return nil
}

// OnRename runs before a primary key field is changed (rec.OldValue has the old value)
func (currencyForFinChargeTermsLogic) OnRename(rec *gtables.CurrencyForFinChargeTerms) error {
	return nil
}
