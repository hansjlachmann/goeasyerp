package tables

import (
	"strconv"

	apperrors "github.com/hansjlachmann/goeasyerp/backend/foundation/errors"
	"github.com/hansjlachmann/goeasyerp/backend/foundation/types"
	gtables "github.com/hansjlachmann/goeasyerp/backend/generated/tables"
)

// changeLogSetupFieldLogic is the business logic of table 404 Change_Log_Setup_Field: the
// fields of a table the change log records when its line says Some Fields (S47) — SUPER users
// only
type changeLogSetupFieldLogic struct{}

func init() { gtables.RegisterChangeLogSetupFieldLogic(changeLogSetupFieldLogic{}) }

// OnInsert runs before a new record is inserted
func (changeLogSetupFieldLogic) OnInsert(rec *gtables.ChangeLogSetupField) error {
	return changeLogSetupChange()
}

// OnModify runs before a record is modified
func (changeLogSetupFieldLogic) OnModify(rec *gtables.ChangeLogSetupField) error {
	return changeLogSetupChange()
}

// OnDelete runs before a record is deleted
func (changeLogSetupFieldLogic) OnDelete(rec *gtables.ChangeLogSetupField) error {
	return changeLogSetupChange()
}

// OnRename runs before a primary key field is changed
func (changeLogSetupFieldLogic) OnRename(rec *gtables.ChangeLogSetupField) error {
	return changeLogSetupChange()
}

// OnValidate_Field_no takes the field's name from the table's definition
func (changeLogSetupFieldLogic) OnValidate_Field_no(rec *gtables.ChangeLogSetupField) error {
	rec.Field_name = ""
	if rec.Field_no == 0 {
		return nil
	}
	table, ok := gtables.NewRecords()[rec.Table_no]
	if ok {
		for _, f := range table.GetFields() {
			if f.No == rec.Field_no {
				rec.Field_name = types.Text(f.Name)
				return nil
			}
		}
	}
	return apperrors.ChangeLogFieldUnknown(strconv.Itoa(rec.Table_no), strconv.Itoa(rec.Field_no))
}
