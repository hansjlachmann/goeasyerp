package tables

import (
	gtables "github.com/hansjlachmann/goeasyerp/backend/generated/tables"
)

// changeLogSetupTableLogic is the business logic of table 403 Change_Log_Setup_Table: per
// table which changes the change log records (S47) — SUPER users only
type changeLogSetupTableLogic struct{}

func init() { gtables.RegisterChangeLogSetupTableLogic(changeLogSetupTableLogic{}) }

// OnInsert runs before a new record is inserted
func (changeLogSetupTableLogic) OnInsert(rec *gtables.ChangeLogSetupTable) error {
	return changeLogSetupChange()
}

// OnModify runs before a record is modified
func (changeLogSetupTableLogic) OnModify(rec *gtables.ChangeLogSetupTable) error {
	return changeLogSetupChange()
}

// OnDelete deletes the table's field lines with it
func (changeLogSetupTableLogic) OnDelete(rec *gtables.ChangeLogSetupTable) error {
	err := changeLogSetupChange()
	if err != nil {
		return err
	}
	changeLogSetupField := gtables.NewChangeLogSetupField(rec)
	changeLogSetupField.SetRange("table_no", rec.Table_no)
	changeLogSetupField.DeleteAll()
	return nil
}

// OnRename runs before a primary key field is changed (the field lines follow: table relation)
func (changeLogSetupTableLogic) OnRename(rec *gtables.ChangeLogSetupTable) error {
	return changeLogSetupChange()
}
