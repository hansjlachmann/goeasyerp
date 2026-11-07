package tables

import (
	gtables "github.com/hansjlachmann/goeasyerp/backend/generated/tables"
)

// userRoleLogic is the business logic of table 2000000004 User_Role.
// Add a trigger when it is needed: the available triggers are listed at the
// top of generated/tables/userrole_gen.go. The field rules of the definition
// (required, length, range) are checked after OnInsert and OnModify.
type userRoleLogic struct{}

func init() { gtables.RegisterUserRoleLogic(userRoleLogic{}) }

// OnInsert runs before a new record is inserted
func (userRoleLogic) OnInsert(rec *gtables.UserRole) error {
	return nil
}

// OnModify runs before a record is modified
func (userRoleLogic) OnModify(rec *gtables.UserRole) error {
	return nil
}

// OnDelete deletes the role's permissions (members keep the role from being deleted)
func (userRoleLogic) OnDelete(rec *gtables.UserRole) error {
	permission := gtables.NewPermission(rec)
	permission.SetRange("role_id", rec.Code)
	permission.DeleteAll()
	return nil
}

// OnRename runs before a primary key field is changed (rec.OldValue has the old value)
func (userRoleLogic) OnRename(rec *gtables.UserRole) error {
	return nil
}
