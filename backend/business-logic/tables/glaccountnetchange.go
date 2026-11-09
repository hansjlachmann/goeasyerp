package tables

import (
	gtables "github.com/hansjlachmann/goeasyerp/backend/generated/tables"
)

// gLAccountNetChangeLogic is the business logic of table 269 G_L_Account_Net_Change.
// Add a trigger when it is needed: the available triggers are listed at the
// top of generated/tables/glaccountnetchange_gen.go. The field rules of the definition
// (required, length, range) are checked after OnInsert and OnModify.
type gLAccountNetChangeLogic struct{}

func init() { gtables.RegisterGLAccountNetChangeLogic(gLAccountNetChangeLogic{}) }

// OnInsert runs before a new record is inserted
func (gLAccountNetChangeLogic) OnInsert(rec *gtables.GLAccountNetChange) error {
	return nil
}

// OnModify runs before a record is modified
func (gLAccountNetChangeLogic) OnModify(rec *gtables.GLAccountNetChange) error {
	return nil
}

// OnDelete runs before a record is deleted
func (gLAccountNetChangeLogic) OnDelete(rec *gtables.GLAccountNetChange) error {
	return nil
}

// OnRename runs before a primary key field is changed (rec.OldValue has the old value)
func (gLAccountNetChangeLogic) OnRename(rec *gtables.GLAccountNetChange) error {
	return nil
}
