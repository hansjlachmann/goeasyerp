package tables

import (
	gtables "github.com/hansjlachmann/goeasyerp/backend/generated/tables"
)

// printerSelectionLogic is the business logic of table 78 Printer_Selection.
// Add a trigger when it is needed: the available triggers are listed at the
// top of generated/tables/printerselection_gen.go. The field rules of the definition
// (required, length, range) are checked after OnInsert and OnModify.
type printerSelectionLogic struct{}

func init() { gtables.RegisterPrinterSelectionLogic(printerSelectionLogic{}) }

// OnInsert runs before a new record is inserted
func (printerSelectionLogic) OnInsert(rec *gtables.PrinterSelection) error {
	return nil
}

// OnModify runs before a record is modified
func (printerSelectionLogic) OnModify(rec *gtables.PrinterSelection) error {
	return nil
}

// OnDelete runs before a record is deleted
func (printerSelectionLogic) OnDelete(rec *gtables.PrinterSelection) error {
	return nil
}

// OnRename runs before a primary key field is changed (rec.OldValue has the old value)
func (printerSelectionLogic) OnRename(rec *gtables.PrinterSelection) error {
	return nil
}
