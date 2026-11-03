package tables

import (
	gtables "github.com/hansjlachmann/openerp/backend/generated/tables"
)

// binLogic is the business logic of table 5400 Bin.
// Add a trigger when it is needed: the available triggers are listed at the
// top of generated/tables/bin_gen.go. The field rules of the definition
// (required, length, range) are checked after OnInsert and OnModify.
type binLogic struct{}

func init() { gtables.RegisterBinLogic(binLogic{}) }

// OnInsert runs before a new record is inserted
func (binLogic) OnInsert(rec *gtables.Bin) error {
	return nil
}

// OnModify runs before a record is modified
func (binLogic) OnModify(rec *gtables.Bin) error {
	return nil
}

// OnDelete runs before a record is deleted
func (binLogic) OnDelete(rec *gtables.Bin) error {
	return nil
}

// OnRename runs before a primary key field is changed (rec.OldValue has the old value)
func (binLogic) OnRename(rec *gtables.Bin) error {
	return nil
}
