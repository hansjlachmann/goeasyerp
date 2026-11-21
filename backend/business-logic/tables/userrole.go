package tables

import (
	apperrors "github.com/hansjlachmann/goeasyerp/backend/foundation/errors"
	"github.com/hansjlachmann/goeasyerp/backend/foundation/session"
	ftables "github.com/hansjlachmann/goeasyerp/backend/foundation/tables"
	gtables "github.com/hansjlachmann/goeasyerp/backend/generated/tables"
)

// userRoleLogic is the business logic of table 2000000004 User_Role. A role holds in every
// company, so only a user who is SUPER in all companies changes one (securityadmin.go).
type userRoleLogic struct{}

func init() { gtables.RegisterUserRoleLogic(userRoleLogic{}) }

// OnInsert: a SUPER user of all companies only
func (userRoleLogic) OnInsert(rec *gtables.UserRole) error {
	return checkRolesAdmin(rec)
}

// OnModify: a SUPER user of all companies only
func (userRoleLogic) OnModify(rec *gtables.UserRole) error {
	return checkRolesAdmin(rec)
}

// OnDelete deletes the role's permissions (members keep the role from being deleted); a SUPER
// user of all companies only
func (userRoleLogic) OnDelete(rec *gtables.UserRole) error {
	err := checkRolesAdmin(rec)
	if err != nil {
		return err
	}
	permission := gtables.NewPermission(rec)
	permission.SetRange("role_id", rec.Code)
	permission.DeleteAll()
	return nil
}

// OnRename: a SUPER user of all companies only
func (userRoleLogic) OnRename(rec *gtables.UserRole) error {
	return checkRolesAdmin(rec)
}

// checkRolesAdmin refuses a user who is not SUPER in all companies
func checkRolesAdmin(src ftables.Source) error {
	if !superIn(src, "") {
		return apperrors.RolesSuperOnly()
	}
	session.SecurityChanged()
	return nil
}
