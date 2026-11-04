package tables

import (
	gtables "github.com/hansjlachmann/goeasyerp/backend/generated/tables"
)

// postCodeLogic is the business logic of table 225 Post_Code.
// Add a trigger when it is needed: the available triggers are listed at the
// top of generated/tables/postcode_gen.go. The field rules of the definition
// (required, length, range) are checked after OnInsert and OnModify.
type postCodeLogic struct{}

func init() { gtables.RegisterPostCodeLogic(postCodeLogic{}) }

// OnInsert runs before a new record is inserted
func (postCodeLogic) OnInsert(rec *gtables.PostCode) error {
	return nil
}

// OnModify runs before a record is modified
func (postCodeLogic) OnModify(rec *gtables.PostCode) error {
	return nil
}

// OnDelete runs before a record is deleted
func (postCodeLogic) OnDelete(rec *gtables.PostCode) error {
	return nil
}

// OnRename runs before a primary key field is changed (rec.OldValue has the old value)
func (postCodeLogic) OnRename(rec *gtables.PostCode) error {
	return nil
}
