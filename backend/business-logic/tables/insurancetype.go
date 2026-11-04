package tables

import (
	gtables "github.com/hansjlachmann/goeasyerp/backend/generated/tables"
)

// insuranceTypeLogic is the business logic of table 5630 Insurance_Type.
// Add a trigger when it is needed: the available triggers are listed at the
// top of generated/tables/insurancetype_gen.go. The field rules of the definition
// (required, length, range) are checked after OnInsert and OnModify.
type insuranceTypeLogic struct{}

func init() { gtables.RegisterInsuranceTypeLogic(insuranceTypeLogic{}) }

// OnInsert runs before a new record is inserted
func (insuranceTypeLogic) OnInsert(rec *gtables.InsuranceType) error {
	return nil
}

// OnModify runs before a record is modified
func (insuranceTypeLogic) OnModify(rec *gtables.InsuranceType) error {
	return nil
}

// OnDelete runs before a record is deleted
func (insuranceTypeLogic) OnDelete(rec *gtables.InsuranceType) error {
	return nil
}

// OnRename runs before a primary key field is changed (rec.OldValue has the old value)
func (insuranceTypeLogic) OnRename(rec *gtables.InsuranceType) error {
	return nil
}
