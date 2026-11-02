package usertime_test

import (
	"database/sql"
	"testing"
	"time"

	_ "github.com/mattn/go-sqlite3"

	"github.com/hansjlachmann/openerp/backend/business-logic/tables"
	"github.com/hansjlachmann/openerp/backend/business-logic/usertime"
	"github.com/hansjlachmann/openerp/backend/foundation/database"
	ftables "github.com/hansjlachmann/openerp/backend/foundation/tables"
	"github.com/hansjlachmann/openerp/backend/foundation/types"
	gtables "github.com/hansjlachmann/openerp/backend/generated/tables"
)

func mustOK(t *testing.T, err error) {
	t.Helper()
	if err != nil {
		t.Fatal(err)
	}
}

func minutes(t *testing.T, src ftables.Source, userID string, date types.Date) int64 {
	t.Helper()
	rec := gtables.NewUserTimeRegister(src)
	if !rec.GetByPK(types.Code(userID), date) {
		return -1
	}
	return rec.Minutes.IntPart()
}

// The minutes of a session go to the login date's register when the user's User Setup — or,
// without one, the General Ledger Setup — says Register Time; sessions of a day add up, a
// session under half a minute counts one minute
func TestRegister(t *testing.T) {
	db, err := sql.Open("sqlite3", ":memory:")
	mustOK(t, err)
	db.SetMaxOpenConns(1)
	t.Cleanup(func() { _ = db.Close() })
	mustOK(t, tables.CreateCompanyTables(db, "demo", database.DBTypeSQLite))
	src := ftables.DBSource(db, "demo", database.DBTypeSQLite)
	day := types.NewDate(2026, 11, 2)
	login := time.Date(2026, 11, 2, 8, 0, 0, 0, time.Local)

	// no setup registers time
	mustOK(t, usertime.Register(src, "ANNA", login, login.Add(time.Hour)))
	if m := minutes(t, src, "ANNA", day); m != -1 {
		t.Fatalf("registered %d minutes without Register Time", m)
	}

	glSetup := gtables.GetGeneralLedgerSetup(src)
	glSetup.Register_time = true
	mustOK(t, glSetup.InsertOrError(false))
	mustOK(t, usertime.Register(src, "ANNA", login, login.Add(90*time.Minute)))
	mustOK(t, usertime.Register(src, "ANNA", login.Add(4*time.Hour), login.Add(4*time.Hour+20*time.Second)))
	if m := minutes(t, src, "ANNA", day); m != 91 {
		t.Errorf("ANNA: %d minutes, want 91", m)
	}

	// the user's setup overrides the General Ledger Setup
	userSetup := gtables.NewUserSetup(src)
	userSetup.Init()
	userSetup.User_id = "BEN"
	mustOK(t, userSetup.InsertOrError(false))
	mustOK(t, usertime.Register(src, "BEN", login, login.Add(time.Hour)))
	if m := minutes(t, src, "BEN", day); m != -1 {
		t.Errorf("BEN without Register Time: %d minutes", m)
	}
}
