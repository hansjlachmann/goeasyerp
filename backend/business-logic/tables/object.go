package tables

import (
	gtables "github.com/hansjlachmann/goeasyerp/backend/generated/tables"
)

// objectLogic is the business logic of table 2000000001 Object.
// Add a trigger when it is needed: the available triggers are listed at the
// top of generated/tables/object_gen.go. The field rules of the definition
// (required, length, range) are checked after OnInsert and OnModify.
type objectLogic struct{}

func init() { gtables.RegisterObjectLogic(objectLogic{}) }

// OnInsert runs before a new record is inserted
func (objectLogic) OnInsert(rec *gtables.Object) error {
	return nil
}

// OnModify runs before a record is modified
func (objectLogic) OnModify(rec *gtables.Object) error {
	return nil
}

// OnDelete runs before a record is deleted
func (objectLogic) OnDelete(rec *gtables.Object) error {
	return nil
}

// OnRename runs before a primary key field is changed (rec.OldValue has the old value)
func (objectLogic) OnRename(rec *gtables.Object) error {
	return nil
}
