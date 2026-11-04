package tables

import (
	"github.com/hansjlachmann/goeasyerp/backend/foundation/system"
	gtables "github.com/hansjlachmann/goeasyerp/backend/generated/tables"
)

// setupChecklistCommentLogic is the business logic of table 345 Setup_Checklist_Comment.
// Add a trigger when it is needed: the available triggers are listed at the
// top of generated/tables/setupchecklistcomment_gen.go. The field rules of the definition
// (required, length, range) are checked after OnInsert and OnModify.
type setupChecklistCommentLogic struct{}

func init() { gtables.RegisterSetupChecklistCommentLogic(setupChecklistCommentLogic{}) }

// OnInsert runs before a new record is inserted
func (setupChecklistCommentLogic) OnInsert(rec *gtables.SetupChecklistComment) error {
	return nil
}

// OnModify runs before a record is modified
func (setupChecklistCommentLogic) OnModify(rec *gtables.SetupChecklistComment) error {
	return nil
}

// OnDelete runs before a record is deleted
func (setupChecklistCommentLogic) OnDelete(rec *gtables.SetupChecklistComment) error {
	return nil
}

// OnRename runs before a primary key field is changed (rec.OldValue has the old value)
func (setupChecklistCommentLogic) OnRename(rec *gtables.SetupChecklistComment) error {
	return nil
}

// SetUpNewSetupChecklistComment (SetUpNewLine): the first comment of a table gets the work date
func SetUpNewSetupChecklistComment(rec *gtables.SetupChecklistComment) {
	commentLine := gtables.NewSetupChecklistComment(rec)
	commentLine.SetRange("table_id", rec.Table_id)
	if !commentLine.FindFirst() {
		rec.Date = system.WorkDate()
	}
}
