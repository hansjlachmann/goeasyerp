package apps

import (
	"database/sql"
	"strings"
	"testing"

	fapps "github.com/hansjlachmann/goeasyerp/backend/foundation/apps"
	fcodeunits "github.com/hansjlachmann/goeasyerp/backend/foundation/codeunits"
	"github.com/hansjlachmann/goeasyerp/backend/foundation/database"
	"github.com/hansjlachmann/goeasyerp/backend/foundation/extensions"
	"github.com/hansjlachmann/goeasyerp/backend/foundation/session"
	"github.com/hansjlachmann/goeasyerp/backend/foundation/types"
	gtables "github.com/hansjlachmann/goeasyerp/backend/generated/tables"
)

// An extension app is disabled and enabled again by its row; the base application never, an
// app on which an enabled app depends not, an app whose dependency is disabled not; only SUPER
func TestEnableDisableApp(t *testing.T) {
	db, err := sql.Open("sqlite3", ":memory:")
	if err != nil {
		t.Fatal(err)
	}
	db.SetMaxOpenConns(1)
	t.Cleanup(func() { _ = db.Close() })
	if err := (&gtables.InstalledApp{}).CreateTableWithDBType(db, "", database.DBTypeSQLite); err != nil {
		t.Fatal(err)
	}
	base, err := fapps.LoadBase()
	if err != nil {
		t.Fatal(err)
	}
	loyalty := &fapps.App{ID: "aaaaaaaa-0000-4000-8000-000000000001", Name: "Loyalty", Version: "1.0.0",
		Dependencies: []fapps.Dependency{{ID: base.ID}}}
	loyalty.NumberSeries.Objects = []fapps.Range{{From: 50000, To: 50099}}
	rewards := &fapps.App{ID: "aaaaaaaa-0000-4000-8000-000000000002", Name: "Rewards", Version: "1.0.0",
		Dependencies: []fapps.Dependency{{ID: base.ID}, {ID: loyalty.ID, Name: "Loyalty"}}}
	if err := SyncInstalledApps(db, database.DBTypeSQLite, []*fapps.App{base, loyalty, rewards}); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { extensions.SetDisabled(); setInstalled(nil) })
	fcodeunits.SetCurrentContext("ADMIN", "Admin", "c", "en-US")
	t.Cleanup(fcodeunits.ClearCurrentContext)

	row := func(app *fapps.App) *gtables.InstalledApp {
		t.Helper()
		var rec gtables.InstalledApp
		rec.InitWithDBType(db, "", database.DBTypeSQLite)
		if !rec.Get(types.NewText(app.ID)) {
			t.Fatalf("no row of %s", app.Name)
		}
		return &rec
	}
	refused := func(what string, fn func(*gtables.InstalledApp) (fcodeunits.Result, error), app *fapps.App, want string) {
		t.Helper()
		if _, err := fn(row(app)); err == nil || !strings.Contains(err.Error(), want) {
			t.Errorf("%s: err = %v, want %q", what, err, want)
		}
	}

	refused("the base", DisableApp, base, "always enabled")
	refused("Loyalty while Rewards depends on it", DisableApp, loyalty, "Rewards depends on it")
	if _, err := DisableApp(row(rewards)); err != nil {
		t.Fatal(err)
	}
	if extensions.Enabled(rewards.ID) || !row(rewards).Disabled {
		t.Error("Rewards was not disabled")
	}
	if _, err := DisableApp(row(loyalty)); err != nil {
		t.Fatalf("Loyalty after Rewards: %v", err)
	}
	if !extensions.ObjectEnabled(fapps.Page, 21) || extensions.ObjectEnabled(fapps.Page, 50000) {
		t.Error("a disabled app's page is not refused, or a base page is")
	}
	refused("Rewards while Loyalty is disabled", EnableApp, rewards, "depends on Loyalty, which is disabled")
	if _, err := EnableApp(row(loyalty)); err != nil {
		t.Fatal(err)
	}
	if !extensions.Enabled(loyalty.ID) || extensions.Enabled(rewards.ID) {
		t.Error("enabled/disabled apps after enabling Loyalty")
	}

	// a user with roles who is not SUPER may not switch apps; another server finds the state
	fcodeunits.SetCurrentAccess(&session.Session{HasRoles: true})
	refused("not SUPER", EnableApp, rewards, "Only a SUPER user")
	extensions.SetDisabled()
	if err := LoadDisabled(db, database.DBTypeSQLite); err != nil || extensions.Enabled(rewards.ID) {
		t.Errorf("LoadDisabled: %v, Rewards enabled %v", err, extensions.Enabled(rewards.ID))
	}
}
