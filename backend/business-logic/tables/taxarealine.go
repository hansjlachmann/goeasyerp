package tables

import (
	gtables "github.com/hansjlachmann/goeasyerp/backend/generated/tables"
)

// taxAreaLineLogic is the business logic of table 319 Tax_Area_Line.
// Add a trigger when it is needed: the available triggers are listed at the
// top of generated/tables/taxarealine_gen.go. The field rules of the definition
// (required, length, range) are checked after OnInsert and OnModify.
type taxAreaLineLogic struct{}

func init() { gtables.RegisterTaxAreaLineLogic(taxAreaLineLogic{}) }

// OnInsert runs before a new record is inserted
func (taxAreaLineLogic) OnInsert(rec *gtables.TaxAreaLine) error {
	return nil
}

// OnModify runs before a record is modified
func (taxAreaLineLogic) OnModify(rec *gtables.TaxAreaLine) error {
	return nil
}

// OnDelete runs before a record is deleted
func (taxAreaLineLogic) OnDelete(rec *gtables.TaxAreaLine) error {
	return nil
}

// OnRename runs before a primary key field is changed (rec.OldValue has the old value)
func (taxAreaLineLogic) OnRename(rec *gtables.TaxAreaLine) error {
	return nil
}
