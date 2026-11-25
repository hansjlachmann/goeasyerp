package handlers

import (
	"database/sql"
	"testing"
	"time"

	"github.com/hansjlachmann/goeasyerp/backend/business-logic/tables"
	"github.com/hansjlachmann/goeasyerp/backend/foundation/database"
	"github.com/hansjlachmann/goeasyerp/backend/foundation/types"
	gtables "github.com/hansjlachmann/goeasyerp/backend/generated/tables"
)

// A token stays good while its user exists and is active and it is not older than the user's
// Sessions Valid From, which a log out, a deactivation and a password another user sets move
// to now (S22)
func TestUserCheckRevokesTokens(t *testing.T) {
	db, err := sql.Open("sqlite3", "file:"+t.Name()+"?mode=memory&cache=shared")
	if err != nil {
		t.Fatal(err)
	}
	db.SetMaxOpenConns(1)
	t.Cleanup(func() { _ = db.Close() })
	sqlite := database.DBTypeSQLite
	if err := (&gtables.User{}).CreateTableWithDBType(db, "", sqlite); err != nil {
		t.Fatal(err)
	}
	if err := (&gtables.UserMember{}).CreateTableWithDBType(db, "", sqlite); err != nil {
		t.Fatal(err)
	}
	user := func() *gtables.User {
		u := &gtables.User{}
		u.InitWithDBType(db, "", sqlite)
		if !u.Get("KARI") {
			t.Fatal("user KARI not found")
		}
		return u
	}
	kari := &gtables.User{}
	kari.InitWithDBType(db, "", sqlite)
	kari.User_id = types.NewCode("KARI")
	kari.Active = true
	if err := tables.SetPassword(kari, "long-secret-1"); err != nil {
		t.Fatal(err)
	}
	if !kari.Insert(true) {
		t.Fatal(kari.TriggerError())
	}

	check := NewUserCheck(db, sqlite)
	issued := time.Now().Add(-time.Minute).Truncate(time.Second)
	if !check("KARI", issued) {
		t.Fatal("a token of an active user was refused")
	}
	if check("NOBODY", issued) {
		t.Error("a token of a user who does not exist was accepted")
	}

	// log out: every token issued so far is refused, one issued at the same second (logging in
	// again at once) is not
	h := &AuthHandler{db: db, dbType: sqlite}
	h.revokeSessions("KARI")
	if check("KARI", issued) {
		t.Error("a token issued before the log out was accepted")
	}
	if !check("KARI", time.Now().Truncate(time.Second)) {
		t.Error("a token issued after the log out was refused")
	}

	// a password set by another user (here: code without a user) logs the user out everywhere
	u := user()
	u.Sessions_valid_from = types.DateTime{}
	u.Modify(false)
	if err := tables.SetPassword(u, "long-secret-2"); err != nil {
		t.Fatal(err)
	}
	if !u.Modify(true) {
		t.Fatal(u.TriggerError())
	}
	if check("KARI", issued) {
		t.Error("a token issued before an administrator set the password was accepted")
	}

	// deactivated: no token is accepted
	u = user()
	u.Active = false
	if !u.Modify(true) {
		t.Fatal(u.TriggerError())
	}
	if check("KARI", time.Now().Add(time.Minute)) {
		t.Error("a token of a deactivated user was accepted")
	}
}
