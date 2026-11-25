package appbundle

import (
	"database/sql"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	_ "github.com/mattn/go-sqlite3"

	"github.com/hansjlachmann/goeasyerp/backend/foundation/database"
	ftables "github.com/hansjlachmann/goeasyerp/backend/foundation/tables"
	"github.com/hansjlachmann/goeasyerp/backend/foundation/types"
	gtables "github.com/hansjlachmann/goeasyerp/backend/generated/tables"
)

const testBaseID = "760e20ec-7a66-4e3f-b6e5-83c94db5a4bd"

func launchDB(t *testing.T) (*sql.DB, ftables.Source) {
	t.Helper()
	db, err := sql.Open("sqlite3", ":memory:")
	if err != nil {
		t.Fatal(err)
	}
	db.SetMaxOpenConns(1)
	t.Cleanup(func() { _ = db.Close() })
	for _, rec := range []interface {
		CreateTableWithDBType(database.Executor, string, database.DBType) error
	}{&gtables.InstalledApp{}, &gtables.AppBuildSetup{}, &gtables.AppBuild{}} {
		if err := rec.CreateTableWithDBType(db, "", database.DBTypeSQLite); err != nil {
			t.Fatal(err)
		}
	}
	return db, ftables.DBSource(db, "", database.DBTypeSQLite)
}

// writeBundle writes a bundle folder with a program and its build.json
func writeBundle(t *testing.T, buildsDir string, no int, version string) string {
	t.Helper()
	dir := Dir(buildsDir, no)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, BinaryName), []byte("program "+version), 0o755); err != nil {
		t.Fatal(err)
	}
	sha, err := FileSHA256(filepath.Join(dir, BinaryName))
	if err != nil {
		t.Fatal(err)
	}
	if err := WriteManifest(dir, &Manifest{BuildNo: no, BaseVersion: version, BinarySHA256: sha}); err != nil {
		t.Fatal(err)
	}
	return sha
}

// The launcher: the standard program without the local builder or apps; the newest verified
// bundle of the version; maintenance while apps are installed and there is no usable bundle
func TestDecide(t *testing.T) {
	db, src := launchDB(t)
	builds := t.TempDir()
	decide := func() Decision {
		t.Helper()
		d, err := Decide(db, database.DBTypeSQLite, builds, "1.2.0", testBaseID)
		if err != nil {
			t.Fatal(err)
		}
		return d
	}
	if d := decide(); d.Bundle != nil || d.Maintenance {
		t.Errorf("no setup: %v", d)
	}

	setup := gtables.GetAppBuildSetup(src)
	setup.Builds_by = gtables.AppBuildSetup_Builds_by.LocalBuilder
	if err := setup.InsertOrError(false); err != nil {
		t.Fatal(err)
	}
	if d := decide(); d.Bundle != nil || d.Maintenance {
		t.Errorf("no apps: %v", d)
	}

	app := gtables.NewInstalledApp(src)
	app.InitRecord()
	app.App_id = types.NewText("aaaaaaaa-0000-4000-8000-000000000001")
	app.Status = gtables.InstalledApp_Status.Installed
	if err := app.InsertOrError(false); err != nil {
		t.Fatal(err)
	}
	if d := decide(); !d.Maintenance {
		t.Errorf("an app and no bundle: %v", d)
	}

	addBuild := func(no int, version, sha string, status gtables.AppBuildStatus) {
		t.Helper()
		b := gtables.NewAppBuild(src)
		b.Init()
		b.Entry_no = no
		b.Status = status
		b.Base_version = types.NewText(version)
		b.Binary_sha256 = types.NewText(sha)
		if err := b.InsertOrError(false); err != nil {
			t.Fatal(err)
		}
	}
	shaOld := writeBundle(t, builds, 1, "1.1.0")
	addBuild(1, "1.1.0", shaOld, gtables.AppBuild_Status.Succeeded)
	if d := decide(); !d.Maintenance {
		t.Errorf("only a bundle of another version: %v", d)
	}
	sha2 := writeBundle(t, builds, 2, "1.2.0")
	addBuild(2, "1.2.0", sha2, gtables.AppBuild_Status.Succeeded)
	addBuild(3, "1.2.0", "", gtables.AppBuild_Status.Failed)
	if d := decide(); d.Bundle == nil || d.Bundle.No != 2 || d.Dir != Dir(builds, 2) {
		t.Errorf("bundle 2: %v", d)
	}

	// a changed program is not run
	if err := os.WriteFile(filepath.Join(builds, "2", BinaryName), []byte("changed"), 0o755); err != nil {
		t.Fatal(err)
	}
	if d := decide(); d.Bundle != nil || !d.Maintenance || !strings.Contains(d.Reason, "not the one built") {
		t.Errorf("a changed bundle: %v (%s)", d, d.Reason)
	}

	// back to a CI pipeline: the standard program
	setup.Builds_by = gtables.AppBuildSetup_Builds_by.CIPipeline
	if err := setup.ModifyOrError(false); err != nil {
		t.Fatal(err)
	}
	if d := decide(); d.Bundle != nil || d.Maintenance {
		t.Errorf("CI pipeline: %v", d)
	}
}

// Before the tables exist (a new database) the launcher runs the standard program
func TestDecideWithoutTables(t *testing.T) {
	db, err := sql.Open("sqlite3", ":memory:")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close() })
	d, err := Decide(db, database.DBTypeSQLite, t.TempDir(), "1.0.0", testBaseID)
	if err != nil || d.Bundle != nil || d.Maintenance {
		t.Errorf("%v %v", d, err)
	}
}

// In maintenance /health answers 200 and the API 503 with the message in the browser's language
func TestMaintenanceHandler(t *testing.T) {
	status := &MaintenanceStatus{version: "1.2.0"}
	h := maintenanceHandler(status)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/health", nil))
	if rec.Code != http.StatusOK {
		t.Errorf("/health: %d", rec.Code)
	}
	req := httptest.NewRequest(http.MethodPost, "/api/auth/login", nil)
	req.Header.Set("Accept-Language", "nb-NO,nb;q=0.9,en;q=0.8")
	rec = httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	var body maintenanceAnswer
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	if rec.Code != http.StatusServiceUnavailable || body.Success || !body.Maintenance || body.Failed ||
		!strings.Contains(body.Error, "1.2.0") || body.Title == "" || body.Title == "APP_MAINTENANCE_TITLE" ||
		body.Retry == "" || body.Retry == "APP_MAINTENANCE_RETRY" {
		t.Errorf("API: %d %+v", rec.Code, body)
	}
	status.set("go build failed")
	rec = httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/api/session", nil))
	body = maintenanceAnswer{}
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	if !body.Failed || !strings.Contains(body.Error, "go build failed") || body.Title == "APP_MAINTENANCE_TITLE_FAILED" {
		t.Errorf("a failed build: %+v", body)
	}
	if language("da,en;q=0.5") != "da-DK" || language("fr") != "en-US" {
		t.Error("language")
	}
}

// While the server starts a newer build the notice says so (the page checks more often)
func TestStartingAnswer(t *testing.T) {
	rec := httptest.NewRecorder()
	maintenanceHandler(&MaintenanceStatus{starting: true}).ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/api/version", nil))
	var body maintenanceAnswer
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	if rec.Code != http.StatusServiceUnavailable || !body.Maintenance || !body.Starting || body.Failed ||
		body.Error == "" || body.Error == "APP_MAINTENANCE_STARTING" {
		t.Errorf("%d %+v", rec.Code, body)
	}
}

// Builds By is read before migrations run: until migration 058 has swapped the stored values,
// 1 is Local Builder; after it (and in a new database) 0 is
func TestLocalBuilderAcrossTheSwap(t *testing.T) {
	db, err := sql.Open("sqlite3", ":memory:")
	if err != nil {
		t.Fatal(err)
	}
	db.SetMaxOpenConns(1)
	t.Cleanup(func() { _ = db.Close() })
	if _, err := db.Exec(`CREATE TABLE "App_Build_Setup" (primary_key TEXT PRIMARY KEY, builds_by INTEGER)`); err != nil {
		t.Fatal(err)
	}
	local := func(value int) bool {
		t.Helper()
		if _, err := db.Exec(`INSERT OR REPLACE INTO "App_Build_Setup" VALUES ('', ?)`, value); err != nil {
			t.Fatal(err)
		}
		got, err := LocalBuilder(db, database.DBTypeSQLite)
		if err != nil {
			t.Fatal(err)
		}
		return got
	}
	noSetup := func() bool {
		t.Helper()
		if _, err := db.Exec(`DELETE FROM "App_Build_Setup"`); err != nil {
			t.Fatal(err)
		}
		got, err := LocalBuilder(db, database.DBTypeSQLite)
		if err != nil {
			t.Fatal(err)
		}
		return got
	}
	if !local(0) || local(1) {
		t.Error("without migrations: 0 is Local Builder")
	}
	if !noSetup() {
		t.Error("a new database with the setup never saved: not the default Local Builder")
	}
	if _, err := db.Exec(`CREATE TABLE "_migrations" (version INTEGER PRIMARY KEY, status TEXT)`); err != nil {
		t.Fatal(err)
	}
	if noSetup() {
		t.Error("before migration 058 the setup never saved means CI Pipeline (the old default)")
	}
	if !local(1) || local(0) {
		t.Error("before migration 058: 1 is Local Builder")
	}
	if _, err := db.Exec(`INSERT INTO "_migrations" VALUES (?, 'completed')`, BuildsByLocalFirstMigration); err != nil {
		t.Fatal(err)
	}
	if !local(0) || local(1) {
		t.Error("after migration 058: 0 is Local Builder")
	}
	if !noSetup() {
		t.Error("after migration 058 the setup never saved means Local Builder (the default)")
	}
}
