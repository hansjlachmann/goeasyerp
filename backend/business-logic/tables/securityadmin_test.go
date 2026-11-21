package tables

import (
	"strings"
	"testing"

	fcodeunits "github.com/hansjlachmann/goeasyerp/backend/foundation/codeunits"
	"github.com/hansjlachmann/goeasyerp/backend/foundation/database"
	"github.com/hansjlachmann/goeasyerp/backend/foundation/session"
	"github.com/hansjlachmann/goeasyerp/backend/foundation/types"
	gtables "github.com/hansjlachmann/goeasyerp/backend/generated/tables"
)

// Users, roles and permissions are administered by SUPER users only: a user who may write
// these tables cannot make themselves SUPER, widen a role or take over another user (S21)
func TestSecurityTablesSuperOnly(t *testing.T) {
	db := newTestDB(t)
	sqlite := database.DBTypeSQLite
	for _, create := range []func() error{
		func() error { return (&gtables.User{}).CreateTableWithDBType(db, "", sqlite) },
		func() error { return (&gtables.UserMember{}).CreateTableWithDBType(db, "", sqlite) },
		func() error { return (&gtables.UserRole{}).CreateTableWithDBType(db, "", sqlite) },
		func() error { return (&gtables.Permission{}).CreateTableWithDBType(db, "", sqlite) },
	} {
		if err := create(); err != nil {
			t.Fatal(err)
		}
	}
	member := func(user, role, company string) *gtables.UserMember {
		m := &gtables.UserMember{}
		m.InitWithDBType(db, "", sqlite)
		m.User_id = types.NewCode(user)
		m.Role_id = types.NewCode(role)
		m.Company = types.NewText(company)
		return m
	}
	user := func(id string) *gtables.User {
		u := &gtables.User{}
		u.InitWithDBType(db, "", sqlite)
		u.User_id = types.NewCode(id)
		return u
	}
	role := func(code string) *gtables.UserRole {
		r := &gtables.UserRole{}
		r.InitWithDBType(db, "", sqlite)
		r.Code = types.NewCode(code)
		return r
	}
	permission := func(roleID, table string) *gtables.Permission {
		p := &gtables.Permission{}
		p.InitWithDBType(db, "", sqlite)
		p.Role_id = types.NewCode(roleID)
		p.Table_name = types.NewCode(table)
		return p
	}

	// Setup without a user: allowed
	for _, id := range []string{"ADMIN", "CSUPER", "CLERK", "VICTIM"} {
		if u := user(id); !u.Insert(true) {
			t.Fatalf("insert user %s: %v", id, u.TriggerError())
		}
	}
	for _, r := range []string{"SUPER", "CLERK"} {
		if rec := role(r); !rec.Insert(true) {
			t.Fatalf("insert role %s: %v", r, rec.TriggerError())
		}
	}
	for _, m := range [][3]string{{"ADMIN", "SUPER", ""}, {"CSUPER", "SUPER", "a"}, {"CLERK", "CLERK", "a"}, {"VICTIM", "CLERK", "b"}} {
		if rec := member(m[0], m[1], m[2]); !rec.Insert(true) {
			t.Fatalf("insert member %v: %v", m, rec.TriggerError())
		}
	}
	if p := permission("CLERK", "CUSTOMER"); !p.Insert(true) {
		t.Fatalf("insert permission: %v", p.TriggerError())
	}

	// as works as the user in company a; isSuper is what the session says there
	as := func(id string, isSuper bool) {
		access := &session.Session{UserID: id, Company: "a"}
		access.SetPermissions(true, isSuper, map[string]session.TablePermission{
			"USER_MEMBER": {Read: true, Insert: true, Modify: true, Delete: true},
			"USER":        {Read: true, Insert: true, Modify: true, Delete: true},
			"PERMISSION":  {Read: true, Insert: true, Modify: true, Delete: true},
			"USER_ROLE":   {Read: true, Insert: true, Modify: true, Delete: true},
		})
		fcodeunits.SetCurrentContext(id, id, "a", "en-US")
		fcodeunits.SetCurrentAccess(access)
	}
	t.Cleanup(fcodeunits.ClearCurrentContext)
	refused := func(what string, ok bool, err error, want string) {
		t.Helper()
		if ok {
			t.Errorf("%s was allowed", what)
			return
		}
		if err == nil || !strings.Contains(err.Error(), want) {
			t.Errorf("%s: refusal %v, want %q in it", what, err, want)
		}
	}
	allowed := func(what string, ok bool, err error) {
		t.Helper()
		if !ok {
			t.Errorf("%s was refused: %v", what, err)
		}
	}
	changePassword := func(id string) (bool, error) {
		u := user(id)
		if !u.Get(id) {
			t.Fatalf("user %s not found", id)
		}
		if err := SetPassword(u, "taken-over"); err != nil {
			t.Fatal(err)
		}
		ok := u.Modify(true)
		return ok, u.TriggerError()
	}

	// A clerk with write rights on every security table
	as("CLERK", false)
	m := member("CLERK", "SUPER", "")
	refused("clerk makes themselves SUPER in all companies", m.Insert(true), m.TriggerError(), "all companies")
	m = member("CLERK", "SUPER", "a")
	refused("clerk makes themselves SUPER in company a", m.Insert(true), m.TriggerError(), "company a")
	m = member("CLERK", "CLERK", "b")
	refused("clerk gives themselves a role in company b", m.Insert(true), m.TriggerError(), "company b")
	m = member("ADMIN", "SUPER", "")
	refused("clerk removes the administrator's SUPER role", m.Delete(true), m.TriggerError(), "all companies")
	ok, err := changePassword("ADMIN")
	refused("clerk sets the administrator's password", ok, err, "ADMIN")
	ok, err = changePassword("CLERK")
	allowed("clerk changes their own password", ok, err)
	p := permission("CLERK", "USER")
	refused("clerk widens their role", p.Insert(true), p.TriggerError(), "roles and their permissions")
	p = permission("CLERK", "CUSTOMER")
	p.Can_delete = true
	refused("clerk changes a permission", p.Modify(true), p.TriggerError(), "roles and their permissions")
	refused("clerk deletes a permission", p.Delete(true), p.TriggerError(), "roles and their permissions")
	r := role("CLERK2")
	refused("clerk makes a role", r.Insert(true), r.TriggerError(), "roles and their permissions")
	u := user("ADMIN")
	refused("clerk deletes the administrator", u.Delete(true), u.TriggerError(), "ADMIN")

	// SUPER in company a only
	as("CSUPER", true)
	m = member("VICTIM", "CLERK", "a")
	allowed("company SUPER gives a role in its company", m.Insert(true), m.TriggerError())
	m = member("CSUPER", "SUPER", "")
	refused("company SUPER makes itself SUPER in all companies", m.Insert(true), m.TriggerError(), "all companies")
	m = member("CLERK", "CLERK", "b")
	refused("company SUPER gives a role in another company", m.Insert(true), m.TriggerError(), "company b")
	ok, err = changePassword("CLERK")
	allowed("company SUPER sets the password of a user of its company", ok, err)
	ok, err = changePassword("VICTIM")
	refused("company SUPER sets the password of a user with roles in another company", ok, err, "VICTIM")
	ok, err = changePassword("ADMIN")
	refused("company SUPER sets the administrator's password", ok, err, "ADMIN")
	p = permission("CLERK", "USER")
	refused("company SUPER changes a role (it holds in every company)", p.Insert(true), p.TriggerError(), "roles and their permissions")

	// SUPER in all companies
	as("ADMIN", true)
	m = member("CLERK", "CLERK", "b")
	allowed("SUPER gives a role in company b", m.Insert(true), m.TriggerError())
	ok, err = changePassword("VICTIM")
	allowed("SUPER sets another user's password", ok, err)
	p = permission("CLERK", "USER")
	allowed("SUPER changes a role", p.Insert(true), p.TriggerError())
	m = member("CSUPER", "SUPER", "a")
	allowed("SUPER removes a SUPER role", m.Delete(true), m.TriggerError())
}
