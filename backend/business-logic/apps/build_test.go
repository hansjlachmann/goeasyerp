package apps

import (
	"archive/zip"
	"bytes"
	"database/sql"
	"strings"
	"testing"

	fapps "github.com/hansjlachmann/goeasyerp/backend/foundation/apps"
	fcodeunits "github.com/hansjlachmann/goeasyerp/backend/foundation/codeunits"
	"github.com/hansjlachmann/goeasyerp/backend/foundation/database"
	"github.com/hansjlachmann/goeasyerp/backend/foundation/types"
	gtables "github.com/hansjlachmann/goeasyerp/backend/generated/tables"
)

// buildTestDB: a database with the app tables, the base application installed and a SUPER-less
// user context (no roles: not checked)
func buildTestDB(t *testing.T) (*gtables.InstalledApp, *fapps.App) {
	t.Helper()
	db, err := sql.Open("sqlite3", ":memory:")
	if err != nil {
		t.Fatal(err)
	}
	db.SetMaxOpenConns(1)
	t.Cleanup(func() { _ = db.Close() })
	for _, rec := range []interface {
		CreateTableWithDBType(database.Executor, string, database.DBType) error
	}{&gtables.InstalledApp{}, &gtables.AppPackage{}, &gtables.AppBuildSetup{}, &gtables.AppBuild{}} {
		if err := rec.CreateTableWithDBType(db, "", database.DBTypeSQLite); err != nil {
			t.Fatal(err)
		}
	}
	base, err := fapps.LoadBase()
	if err != nil {
		t.Fatal(err)
	}
	if err := SyncInstalledApps(db, database.DBTypeSQLite, []*fapps.App{base}); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { setInstalled(nil) })
	fcodeunits.SetCurrentContext("ADMIN", "Admin", "c", "en-US")
	t.Cleanup(fcodeunits.ClearCurrentContext)
	var src gtables.InstalledApp
	src.InitWithDBType(db, "", database.DBTypeSQLite)
	return &src, base
}

// setBuildSetup sets App Build Setup
func setBuildSetup(t *testing.T, src *gtables.InstalledApp, local, allowUpload bool) {
	t.Helper()
	setup := gtables.GetAppBuildSetup(src)
	if local {
		setup.Builds_by = gtables.AppBuildSetup_Builds_by.LocalBuilder
	} else {
		setup.Builds_by = gtables.AppBuildSetup_Builds_by.CIPipeline
	}
	setup.Allow_upload = allowUpload
	probe := gtables.NewAppBuildSetup(src)
	var err error
	if probe.Get(types.NewCode("")) {
		err = setup.ModifyOrError(false)
	} else {
		err = setup.InsertOrError(false)
	}
	if err != nil {
		t.Fatal(err)
	}
}

func zipOf(t *testing.T, files map[string]string) []byte {
	t.Helper()
	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)
	for name, content := range files {
		w, err := zw.Create(name)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := w.Write([]byte(content)); err != nil {
			t.Fatal(err)
		}
	}
	if err := zw.Close(); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

// Upload App needs Allow Upload; a package's app is checked like Install App and recorded
// Pending Install from the package, which is stored; a pipeline's build list refuses it
func TestUploadApp(t *testing.T) {
	src, base := buildTestDB(t)
	const id = "aaaaaaaa-0000-4000-8000-000000000021"
	onBase := "{id: " + base.ID + "}"
	pkg := zipOf(t, map[string]string{
		"extensions/application.yaml": string(manifest(id, "Notes", "1.0.0", "nt", "{from: 54000, to: 54099}", onBase)),
		"extensions/go.mod":           "module example.com/notes",
	})

	if _, err := UploadApp(src); err == nil || !strings.Contains(err.Error(), "not allowed") {
		t.Errorf("upload without Allow Upload: %v", err)
	}
	setBuildSetup(t, src, true, true)

	if _, err := uploadPackage(src, []byte("not a zip")); err == nil || !strings.Contains(err.Error(), "not an app package") {
		t.Errorf("a file that is no package: %v", err)
	}
	if _, err := uploadPackage(src, pkg); err != nil {
		t.Fatal(err)
	}
	row := gtables.NewInstalledApp(src)
	if !row.Get(types.NewText(id)) || row.Status != gtables.InstalledApp_Status.PendingInstall ||
		row.Source_type != gtables.InstalledApp_Source_type.Package || row.Source_ref != "1.0.0" {
		t.Fatalf("row %+v", row)
	}
	stored := gtables.NewAppPackage(src)
	if !stored.Get(map[string]interface{}{"app_id": id, "version": "1.0.0"}) || !bytes.Equal(stored.Package, pkg) ||
		len(stored.Sha256) != 64 || stored.Size != len(pkg) {
		t.Fatalf("package %+v", stored)
	}

	// a refused app stores no package
	other := zipOf(t, map[string]string{"application.yaml": string(manifest("aaaaaaaa-0000-4000-8000-000000000022", "Clash", "1.0.0", "cl", "{from: 54050, to: 54150}", onBase))})
	if _, err := uploadPackage(src, other); err == nil || !strings.Contains(err.Error(), "overlaps") {
		t.Errorf("an overlapping app: %v", err)
	}
	if gtables.NewAppPackage(src).Get(map[string]interface{}{"app_id": "aaaaaaaa-0000-4000-8000-000000000022", "version": "1.0.0"}) {
		t.Error("the refused app's package was stored")
	}

	if _, err := BuildList(src); err == nil || !strings.Contains(err.Error(), "uploaded as a package") {
		t.Errorf("build list with a package app: %v", err)
	}
}
