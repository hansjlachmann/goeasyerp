package tables

import (
	gtables "github.com/hansjlachmann/goeasyerp/backend/generated/tables"
)

// userMemberLogic is the business logic of table 2000000003 User_Member.
// Add a trigger when it is needed: the available triggers are listed at the
// top of generated/tables/usermember_gen.go. The field rules of the definition
// (required, length, range) are checked after OnInsert and OnModify.
type userMemberLogic struct{}

func init() { gtables.RegisterUserMemberLogic(userMemberLogic{}) }

// OnInsert runs before a new record is inserted
func (userMemberLogic) OnInsert(rec *gtables.UserMember) error {
	return nil
}

// OnModify runs before a record is modified
func (userMemberLogic) OnModify(rec *gtables.UserMember) error {
	return nil
}

// OnDelete runs before a record is deleted
func (userMemberLogic) OnDelete(rec *gtables.UserMember) error {
	return nil
}

// OnRename runs before a primary key field is changed (rec.OldValue has the old value)
func (userMemberLogic) OnRename(rec *gtables.UserMember) error {
	return nil
}
