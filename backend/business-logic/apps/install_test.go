package apps

import (
	"context"
	"database/sql"
	"encoding/base64"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	fapps "github.com/hansjlachmann/goeasyerp/backend/foundation/apps"
	fcodeunits "github.com/hansjlachmann/goeasyerp/backend/foundation/codeunits"
	"github.com/hansjlachmann/goeasyerp/backend/foundation/database"
	"github.com/hansjlachmann/goeasyerp/backend/foundation/themes"
	"github.com/hansjlachmann/goeasyerp/backend/foundation/types"
	gtables "github.com/hansjlachmann/goeasyerp/backend/generated/tables"
)

func manifest(id, name, version, prefix, objects string, deps ...string) []byte {
	s := "application:\n  id: " + id + "\n  name: " + name + "\n  version: " + version + "\n  prefix: " + prefix +
		"\n  number_series:\n    objects: [" + objects + "]\n    fields: [" + objects + "]\n  dependencies:\n"
	for _, d := range deps {
		s += "    - " + d + "\n"
	}
	return []byte(s)
}

// Install records an app Pending Install after checking it against the planned apps; an
// installed app in another version is an update; Uninstall, Cancel Pending, the build list and
// the startup sync follow the pending states
func TestInstallUninstall(t *testing.T) {
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
	onBase := "{id: " + base.ID + "}"
	const loyaltyID, rewardsID = "aaaaaaaa-0000-4000-8000-000000000011", "aaaaaaaa-0000-4000-8000-000000000012"
	loyalty, err := fapps.Parse(manifest(loyaltyID, "Loyalty", "1.0.0", "lp", "{from: 50000, to: 50099}", onBase))
	if err != nil {
		t.Fatal(err)
	}
	if err := SyncInstalledApps(db, database.DBTypeSQLite, []*fapps.App{base, loyalty}); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { setInstalled(nil) })
	fcodeunits.SetCurrentContext("ADMIN", "Admin", "c", "en-US")
	t.Cleanup(fcodeunits.ClearCurrentContext)

	var srcRec gtables.InstalledApp
	srcRec.InitWithDBType(db, "", database.DBTypeSQLite)
	src := &srcRec
	row := func(id string) *gtables.InstalledApp {
		rec := gtables.NewInstalledApp(src)
		if !rec.Get(types.NewText(id)) {
			return nil
		}
		return rec
	}
	const commitA, commitB = "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa", "bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb"
	installAt := func(data []byte, commit string) error {
		_, err := installManifest(src, data, "https://github.com/acme/app", "v1", commit, "extensions")
		return err
	}
	install := func(data []byte) error { return installAt(data, commitA) }
	wantErr := func(what string, err error, want string) {
		t.Helper()
		if err == nil || !strings.Contains(err.Error(), want) {
			t.Errorf("%s: err = %v, want %q", what, err, want)
		}
	}

	rewards := manifest(rewardsID, "Rewards", "1.0.0", "rw", "{from: 51000, to: 51099}", onBase, "{id: "+loyaltyID+", name: Loyalty, min_version: 1.0.0}")
	if err := install(rewards); err != nil {
		t.Fatal(err)
	}
	if r := row(rewardsID); r == nil || r.Status != gtables.InstalledApp_Status.PendingInstall || len(r.Manifest) == 0 || r.Source_ref != "v1" || r.Source_commit != commitA {
		t.Fatalf("Rewards row %+v", r)
	}
	wantErr("a missing dependency", install(manifest("aaaaaaaa-0000-4000-8000-000000000013", "Gifts", "1.0.0", "gf", "{from: 52000, to: 52099}", onBase, "{id: aaaaaaaa-0000-4000-8000-0000000000ff, name: Shop}")), "Gifts needs Shop")
	wantErr("overlapping series", install(manifest("aaaaaaaa-0000-4000-8000-000000000014", "Other", "1.0.0", "ot", "{from: 50050, to: 50150}", onBase)), "overlaps")
	wantErr("without a prefix", install(manifest("aaaaaaaa-0000-4000-8000-000000000015", "Plain", "1.0.0", "", "{from: 53000, to: 53099}", onBase)), "prefix")
	loyalty100 := manifest(loyaltyID, "Loyalty", "1.0.0", "lp", "{from: 50000, to: 50099}", onBase)
	wantErr("the same version again (no source)", install(loyalty100), "installed already")
	// installed from Git at commit A: the same version again at A is installed already, its tag
	// moved to B is refused; installed before commits were recorded, it may be installed again to
	// pin it
	lr := row(loyaltyID)
	lr.Source = "https://github.com/acme/app"
	lr.Source_commit = commitA
	if err := lr.ModifyOrError(false); err != nil {
		t.Fatal(err)
	}
	wantErr("the same version again", install(loyalty100), "installed already")
	wantErr("the tag moved", installAt(loyalty100, commitB), "now points to commit "+commitB)
	lr = row(loyaltyID)
	lr.Source_commit = ""
	if err := lr.ModifyOrError(false); err != nil {
		t.Fatal(err)
	}
	if err := installAt(loyalty100, commitB); err != nil || row(loyaltyID).Source_commit != commitB {
		t.Fatalf("pin Loyalty 1.0.0: %v", err)
	}
	if _, err := CancelPending(row(loyaltyID)); err != nil {
		t.Fatal(err)
	}
	wantErr("an update below a dependent's need", install(manifest(loyaltyID, "Loyalty", "0.9.0", "lp", "{from: 50000, to: 50099}", onBase)), "Rewards needs Loyalty version 1.0.0")

	// an update of Loyalty, then the build list: base version, Loyalty before Rewards
	if err := install(manifest(loyaltyID, "Loyalty", "1.1.0", "lp", "{from: 50000, to: 50099}", onBase)); err != nil {
		t.Fatal(err)
	}
	wantErr("Uninstall Loyalty while Rewards depends on it", func() error { _, err := UninstallApp(row(loyaltyID)); return err }(), "Rewards depends on it")
	res, err := BuildList(row(base.ID))
	if err != nil {
		t.Fatal(err)
	}
	list, _ := base64.StdEncoding.DecodeString(res.Data["file"].(string))
	text := string(list)
	if !strings.Contains(text, "version: "+base.Version) || strings.Index(text, "name: Loyalty") > strings.Index(text, "name: Rewards") ||
		!strings.Contains(text, "version: 1.1.0") || !strings.Contains(text, "repository: https://github.com/acme/app") || !strings.Contains(text, "commit: "+commitA) {
		t.Errorf("build list:\n%s", text)
	}

	// Cancel Pending: a new app goes, an update goes back to the version built in
	if _, err := CancelPending(row(rewardsID)); err != nil || row(rewardsID) != nil {
		t.Errorf("cancel Rewards: %v, row %v", err, row(rewardsID))
	}
	if _, err := CancelPending(row(loyaltyID)); err != nil || row(loyaltyID).Version != "1.0.0" || row(loyaltyID).Status != gtables.InstalledApp_Status.Installed {
		t.Errorf("cancel the Loyalty update: %v %+v", err, row(loyaltyID))
	}

	// Uninstall, then a build without Loyalty removes its row; a pending install stays until built
	if _, err := UninstallApp(row(loyaltyID)); err != nil || row(loyaltyID).Status != gtables.InstalledApp_Status.PendingRemoval {
		t.Fatalf("uninstall Loyalty: %v", err)
	}
	wantErr("Uninstall the base", func() error { _, err := UninstallApp(row(base.ID)); return err }(), "always enabled")
	if err := install(manifest("aaaaaaaa-0000-4000-8000-000000000016", "Notes", "1.0.0", "nt", "{from: 54000, to: 54099}", onBase)); err != nil {
		t.Fatal(err)
	}
	if err := SyncInstalledApps(db, database.DBTypeSQLite, []*fapps.App{base}); err != nil {
		t.Fatal(err)
	}
	if row(loyaltyID) != nil || row("aaaaaaaa-0000-4000-8000-000000000016") == nil {
		t.Error("after a build without Loyalty: its row stays, or the pending install went")
	}
	notes, _ := fapps.Parse(manifest("aaaaaaaa-0000-4000-8000-000000000016", "Notes", "1.0.0", "nt", "{from: 54000, to: 54099}", onBase))
	if err := SyncInstalledApps(db, database.DBTypeSQLite, []*fapps.App{base, notes}); err != nil {
		t.Fatal(err)
	}
	if r := row(notes.ID); r == nil || r.Status != gtables.InstalledApp_Status.Installed || r.Source == "" {
		t.Errorf("Notes after its build: %+v", r)
	}
}

// The application.yaml is read through the GitHub API's contents endpoint at the version given
func TestFetchApplicationFile(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/repos/acme/loyalty/contents/extensions/application.yaml" || r.URL.Query().Get("ref") != "v1.2.0" {
			http.NotFound(w, r)
			return
		}
		_, _ = w.Write([]byte("application: {}"))
	}))
	t.Cleanup(srv.Close)
	f := themes.NewFetcherWithBase(srv.URL, srv.URL, "")
	repo, err := themes.ParseRepoURL("https://github.com/acme/loyalty")
	if err != nil {
		t.Fatal(err)
	}
	repo.Ref = "v1.2.0"
	data, err := f.File(context.Background(), repo, "extensions/application.yaml", manifestMax)
	if err != nil || string(data) != "application: {}" {
		t.Errorf("file %q, err %v", data, err)
	}
	if _, err := f.File(context.Background(), repo, "../secret", manifestMax); err == nil {
		t.Error("a path with .. accepted")
	}
}

// The build list gives the repository's address without the version: git clones it at the tag
// (Install App of 0.1.163 and before stored the address with /tree/<tag>)
func TestRepositoryAddress(t *testing.T) {
	for in, want := range map[string]string{
		"https://github.com/acme/app/tree/v1.0.0": "https://github.com/acme/app",
		"https://github.com/acme/app":             "https://github.com/acme/app",
		"https://github.com/acme/app.git":         "https://github.com/acme/app",
		"package 0123456789ab":                    "package 0123456789ab",
	} {
		if got := repositoryAddress(in); got != want {
			t.Errorf("%s: %s, want %s", in, got, want)
		}
	}
}
