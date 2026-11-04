package session

import (
	"database/sql"
	"testing"

	_ "github.com/mattn/go-sqlite3"

	"github.com/hansjlachmann/goeasyerp/backend/foundation/database"
)

func TestObjectKey(t *testing.T) {
	if got := ObjectKey(ObjectCodeunit, 6); got != "CODEUNIT 6" {
		t.Errorf("ObjectKey codeunit = %q", got)
	}
	if got := ObjectKey("report", 93); got != "REPORT 93" {
		t.Errorf("ObjectKey report = %q", got)
	}
}

func TestCanExecute(t *testing.T) {
	noRoles := &Session{}
	if !noRoles.CanExecute(ObjectCodeunit, 6) {
		t.Error("a user without roles may run every codeunit")
	}
	super := &Session{}
	super.SetPermissions(true, true, nil)
	if !super.CanExecute(ObjectReport, 93) {
		t.Error("SUPER may run every report")
	}
	reader := &Session{}
	reader.SetPermissions(true, false, map[string]TablePermission{
		"CODEUNIT 6": {Execute: true},
		"REPORT 93":  {Read: true},
	})
	if !reader.CanExecute(ObjectCodeunit, 6) {
		t.Error("an Execute row allows the codeunit")
	}
	if reader.CanExecute(ObjectReport, 93) {
		t.Error("a row without Execute does not allow the report")
	}
	if reader.CanExecute(ObjectCodeunit, 7) {
		t.Error("no row: not allowed")
	}
}

// Execute rows merge across roles; rows from before execute permissions (can_execute NULL)
// read as not allowed
func TestLoadPermissionsExecute(t *testing.T) {
	db, err := sql.Open("sqlite3", ":memory:")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close() })
	for _, stmt := range []string{
		`CREATE TABLE "User_Member" (user_id TEXT, role_id TEXT, company TEXT)`,
		`CREATE TABLE "Permission" (role_id TEXT, table_name TEXT, can_read BOOLEAN, can_insert BOOLEAN, can_modify BOOLEAN, can_delete BOOLEAN, can_execute BOOLEAN)`,
		`INSERT INTO "User_Member" VALUES ('HANS', 'READER', ''), ('HANS', 'CLERK', 'test'), ('HANS', 'OTHER', 'other')`,
		`INSERT INTO "Permission" VALUES ('READER', 'Customer', 1, 0, 0, 0, NULL)`,
		`INSERT INTO "Permission" VALUES ('READER', 'CODEUNIT 6', 0, 0, 0, 0, 0)`,
		`INSERT INTO "Permission" VALUES ('CLERK', 'CODEUNIT 6', 0, 0, 0, 0, 1)`,
		`INSERT INTO "Permission" VALUES ('OTHER', 'REPORT 93', 0, 0, 0, 0, 1)`,
	} {
		if _, err := db.Exec(stmt); err != nil {
			t.Fatalf("%s: %v", stmt, err)
		}
	}
	hasRoles, isSuper, perms := LoadPermissions(db, database.DBTypeSQLite, "HANS", "test")
	if !hasRoles || isSuper {
		t.Fatalf("hasRoles=%v isSuper=%v", hasRoles, isSuper)
	}
	s := &Session{}
	s.SetPermissions(hasRoles, isSuper, perms)
	if !s.CanExecute(ObjectCodeunit, 6) {
		t.Error("CLERK's Execute row should allow codeunit 6")
	}
	if s.CanExecute(ObjectReport, 93) {
		t.Error("the role of another company must not allow report 93")
	}
	if !s.HasPermission("Customer", "read") || perms["CUSTOMER"].Execute {
		t.Errorf("table row: %+v", perms["CUSTOMER"])
	}
}
