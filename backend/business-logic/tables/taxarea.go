package tables

import (
	gtables "github.com/hansjlachmann/goeasyerp/backend/generated/tables"
)

// taxAreaLogic is the business logic of table 318 Tax_Area.
// Add a trigger when it is needed: the available triggers are listed at the
// top of generated/tables/taxarea_gen.go. The field rules of the definition
// (required, length, range) are checked after OnInsert and OnModify.
type taxAreaLogic struct{}

func init() { gtables.RegisterTaxAreaLogic(taxAreaLogic{}) }

// OnInsert runs before a new record is inserted
func (taxAreaLogic) OnInsert(rec *gtables.TaxArea) error {
	return nil
}

// OnModify runs before a record is modified
func (taxAreaLogic) OnModify(rec *gtables.TaxArea) error {
	return nil
}

// OnDelete deletes the area's lines
func (taxAreaLogic) OnDelete(rec *gtables.TaxArea) error {
	taxAreaLine := gtables.NewTaxAreaLine(rec)
	taxAreaLine.SetRange("tax_area", rec.Code)
	taxAreaLine.DeleteAll()
	return nil
}

// OnRename runs before a primary key field is changed (rec.OldValue has the old value)
func (taxAreaLogic) OnRename(rec *gtables.TaxArea) error {
	return nil
}
