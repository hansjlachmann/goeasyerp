package tables

import (
	gtables "github.com/hansjlachmann/goeasyerp/backend/generated/tables"
)

// objectTranslationLogic is the business logic of table 377 Object Translation.
// Add a trigger when it is needed: the available triggers are listed at the
// top of generated/tables/objecttranslation_gen.go. The field rules of the definition
// (required, length, range) are checked after OnInsert and OnModify.
type objectTranslationLogic struct{}

func init() { gtables.RegisterObjectTranslationLogic(objectTranslationLogic{}) }

// OnInsert runs before a new record is inserted
func (objectTranslationLogic) OnInsert(rec *gtables.ObjectTranslation) error {
	return nil
}

// OnModify runs before a record is modified
func (objectTranslationLogic) OnModify(rec *gtables.ObjectTranslation) error {
	return nil
}

// OnDelete runs before a record is deleted
func (objectTranslationLogic) OnDelete(rec *gtables.ObjectTranslation) error {
	return nil
}

// OnRename runs before a primary key field is changed (rec.OldValue has the old value)
func (objectTranslationLogic) OnRename(rec *gtables.ObjectTranslation) error {
	return nil
}
