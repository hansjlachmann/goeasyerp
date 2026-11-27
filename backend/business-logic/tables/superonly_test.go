package tables

import (
	"errors"
	"testing"

	fcodeunits "github.com/hansjlachmann/goeasyerp/backend/foundation/codeunits"
	"github.com/hansjlachmann/goeasyerp/backend/foundation/database"
	apperrors "github.com/hansjlachmann/goeasyerp/backend/foundation/errors"
	"github.com/hansjlachmann/goeasyerp/backend/foundation/session"
	ftables "github.com/hansjlachmann/goeasyerp/backend/foundation/tables"
	"github.com/hansjlachmann/goeasyerp/backend/foundation/types"
	gtables "github.com/hansjlachmann/goeasyerp/backend/generated/tables"
)

// A new company gets all the tables of a company and a rename moves its data: only a SUPER
// user creates or renames one through the table API (code without a user may)
func TestCompanyInsertAndRenameSuperOnly(t *testing.T) {
	db := newTestDB(t)
	var comp gtables.Company
	if err := comp.CreateTableWithDBType(db, "", database.DBTypeSQLite); err != nil {
		t.Fatal(err)
	}
	comp.InitWithDBType(db, "", database.DBTypeSQLite)
	comp.Name = types.NewText("first")
	if !comp.Insert(true) {
		t.Fatalf("code without a user: %v", comp.TriggerError())
	}

	t.Cleanup(fcodeunits.ClearCurrentContext)
	fcodeunits.SetCurrentAccess(&session.Session{UserID: "CLERK", HasRoles: true})
	other := gtables.NewCompany(ftables.DBSource(db, "", database.DBTypeSQLite))
	other.Name = types.NewText("second")
	if other.Insert(true) {
		t.Fatal("a user without SUPER created a company")
	}
	var appErr *apperrors.AppError
	if !errors.As(other.TriggerError(), &appErr) || appErr.Code != apperrors.ErrCompanySuperOnly {
		t.Errorf("insert error %v", other.TriggerError())
	}

	existing := gtables.NewCompany(ftables.DBSource(db, "", database.DBTypeSQLite))
	if !existing.Get(types.NewText("first")) {
		t.Fatal("company first not found")
	}
	existing.Name = types.NewText("renamed")
	if existing.Modify(true) {
		t.Fatal("a user without SUPER renamed a company")
	}
	if !errors.As(existing.TriggerError(), &appErr) || appErr.Code != apperrors.ErrCompanySuperOnly {
		t.Errorf("rename error %v", existing.TriggerError())
	}

	fcodeunits.SetCurrentAccess(&session.Session{UserID: "ADMIN", HasRoles: true, IsSuper: true})
	if !other.Insert(true) {
		t.Errorf("a SUPER user: %v", other.TriggerError())
	}
}

// Apps are installed and removed by the SUPER-only functions of Installed Apps: a user who is
// not SUPER changes no Installed App row through the table API (the server's own sync may)
func TestInstalledAppSuperOnly(t *testing.T) {
	db := newTestDB(t)
	var app gtables.InstalledApp
	if err := app.CreateTableWithDBType(db, "", database.DBTypeSQLite); err != nil {
		t.Fatal(err)
	}
	app.InitWithDBType(db, "", database.DBTypeSQLite)
	app.App_id = types.NewText("11111111-1111-1111-1111-111111111111")
	app.Name = types.NewText("Test App")
	if !app.Insert(true) {
		t.Fatalf("code without a user: %v", app.TriggerError())
	}

	t.Cleanup(fcodeunits.ClearCurrentContext)
	fcodeunits.SetCurrentAccess(&session.Session{UserID: "CLERK", HasRoles: true})
	other := gtables.NewInstalledApp(ftables.DBSource(db, "", database.DBTypeSQLite))
	other.App_id = types.NewText("22222222-2222-2222-2222-222222222222")
	other.Name = types.NewText("Other App")
	if other.Insert(true) {
		t.Error("a user without SUPER inserted an app")
	}
	app.Name = types.NewText("Changed")
	if app.Modify(true) {
		t.Error("a user without SUPER modified an app")
	}
	if app.Delete(true) {
		t.Error("a user without SUPER deleted an app")
	}

	fcodeunits.SetCurrentAccess(&session.Session{UserID: "ADMIN", HasRoles: true, IsSuper: true})
	if !app.Delete(true) {
		t.Errorf("a SUPER user: %v", app.TriggerError())
	}
}

// A user's preferences and appearance are changed by that user or a SUPER user only
func TestOwnUserRowsOnly(t *testing.T) {
	db := newTestDB(t)
	var prefs gtables.UserPreferences
	if err := prefs.CreateTableWithDBType(db, "", database.DBTypeSQLite); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(fcodeunits.ClearCurrentContext)
	fcodeunits.SetCurrentAccess(&session.Session{UserID: "CLERK", HasRoles: true})

	own := gtables.NewUserPreferences(ftables.DBSource(db, "", database.DBTypeSQLite))
	own.User_id, own.Page_id = types.NewCode("CLERK"), 22
	own.Preference_type, own.Preference_name = types.NewCode("LAYOUT"), types.NewCode("DEFAULT")
	if !own.Insert(true) {
		t.Fatalf("own preferences: %v", own.TriggerError())
	}
	other := gtables.NewUserPreferences(ftables.DBSource(db, "", database.DBTypeSQLite))
	other.User_id, other.Page_id = types.NewCode("ADMIN"), 22
	other.Preference_type, other.Preference_name = types.NewCode("LAYOUT"), types.NewCode("DEFAULT")
	if other.Insert(true) {
		t.Fatal("a user wrote another user's preferences")
	}
	var appErr *apperrors.AppError
	if !errors.As(other.TriggerError(), &appErr) || appErr.Code != apperrors.ErrOwnUserRowsOnly {
		t.Errorf("error %v", other.TriggerError())
	}

	fcodeunits.SetCurrentAccess(&session.Session{UserID: "ADMIN", HasRoles: true, IsSuper: true})
	if !own.Delete(true) {
		t.Errorf("a SUPER user deleting another user's preferences: %v", own.TriggerError())
	}

	var pers gtables.UserPersonalization
	if err := pers.CreateTableWithDBType(db, "", database.DBTypeSQLite); err != nil {
		t.Fatal(err)
	}
	fcodeunits.SetCurrentAccess(&session.Session{UserID: "CLERK", HasRoles: true})
	theirs := DefaultUserPersonalization("ADMIN")
	theirs.InitWithDBType(db, "", database.DBTypeSQLite)
	if theirs.Insert(true) {
		t.Error("a user wrote another user's appearance")
	}
}
