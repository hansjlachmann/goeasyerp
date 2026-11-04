package tables

import (
	"github.com/hansjlachmann/goeasyerp/backend/foundation/session"
	"github.com/hansjlachmann/goeasyerp/backend/foundation/types"
	gtables "github.com/hansjlachmann/goeasyerp/backend/generated/tables"
)

// permissionLogic is the business logic of table 2000000005 Permission: a row gives a role
// rights on table data (Read, Insert, Modify, Delete) or Execute on a report or codeunit. The
// key of a report or codeunit row is "REPORT 93" / "CODEUNIT 6" (session.ObjectKey), filled
// from Object Type and Object ID.
type permissionLogic struct{}

func init() { gtables.RegisterPermissionLogic(permissionLogic{}) }

// OnInsert fills the key of a report or codeunit row
func (permissionLogic) OnInsert(rec *gtables.Permission) error {
	setPermissionObjectKey(rec)
	return nil
}

// OnValidate_Object_type: a report or codeunit row is keyed by its object
func (permissionLogic) OnValidate_Object_type(rec *gtables.Permission) error {
	setPermissionObjectKey(rec)
	return nil
}

// OnValidate_Object_id: a report or codeunit row is keyed by its object
func (permissionLogic) OnValidate_Object_id(rec *gtables.Permission) error {
	setPermissionObjectKey(rec)
	return nil
}

func setPermissionObjectKey(rec *gtables.Permission) {
	switch rec.Object_type {
	case gtables.Permission_Object_type.Report:
		rec.Table_name = types.NewCode(session.ObjectKey(session.ObjectReport, rec.Object_id))
	case gtables.Permission_Object_type.Codeunit:
		rec.Table_name = types.NewCode(session.ObjectKey(session.ObjectCodeunit, rec.Object_id))
	}
}
