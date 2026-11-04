package apps

import (
	"database/sql"
	"fmt"
	"strings"
	"testing"

	_ "github.com/mattn/go-sqlite3"

	fapps "github.com/hansjlachmann/goeasyerp/backend/foundation/apps"
	fcodeunits "github.com/hansjlachmann/goeasyerp/backend/foundation/codeunits"
	"github.com/hansjlachmann/goeasyerp/backend/foundation/database"
	"github.com/hansjlachmann/goeasyerp/backend/foundation/i18n"
	ftables "github.com/hansjlachmann/goeasyerp/backend/foundation/tables"
	"github.com/hansjlachmann/goeasyerp/backend/foundation/types"
	gtables "github.com/hansjlachmann/goeasyerp/backend/generated/tables"
)

// Every table, page and codeunit of this repository is in the base application's series
func TestObjectNumbersInBaseApplication(t *testing.T) {
	base, err := fapps.LoadBase()
	if err != nil {
		t.Fatal(err)
	}
	if err := CheckObjectNumbers([]*fapps.App{base}); err != nil {
		t.Fatal(err)
	}
	// With smaller series the objects above them are named
	small := *base
	small.NumberSeries.Objects = []fapps.Range{{From: 1, To: 100}}
	small.SystemTables = nil
	err = CheckObjectNumbers([]*fapps.App{&small})
	for _, want := range []string{"table 2000000002 User", "page 409", "codeunit 901 "} {
		if err == nil || !strings.Contains(err.Error(), want) {
			t.Errorf("error %v, want it to name %q", err, want)
		}
	}
}

func TestSyncInstalledApps(t *testing.T) {
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
	for range 2 { // the second run finds the row unchanged
		if err := SyncInstalledApps(db, database.DBTypeSQLite, []*fapps.App{base}); err != nil {
			t.Fatal(err)
		}
	}
	var rec gtables.InstalledApp
	rec.InitWithDBType(db, "", database.DBTypeSQLite)
	if !rec.Get(types.NewText(base.ID)) {
		t.Fatal("installed app not written")
	}
	if rec.Name.String() != base.Name || rec.Object_series.String() != "1–999, 1000–1299, 1400–1499, 5400–5499, 5600–5699, 5700–5799, 9300–9399" || rec.System_tables.String() != "2000000000–2000000199" {
		t.Errorf("row %v", rec.ToMap())
	}
	if n := rec.Count(); n != 1 {
		t.Errorf("%d rows, want 1", n)
	}
}

func TestSyncObjects(t *testing.T) {
	db, err := sql.Open("sqlite3", ":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	if err := (&gtables.Object{}).CreateTableWithDBType(db, "", database.DBTypeSQLite); err != nil {
		t.Fatal(err)
	}
	if err := (&gtables.ObjectTranslation{}).CreateTableWithDBType(db, "", database.DBTypeSQLite); err != nil {
		t.Fatal(err)
	}
	// a translation of an object that is no longer there
	gone := gtables.NewObjectTranslation(ftables.DBSource(db, "", database.DBTypeSQLite))
	gone.Object_type, gone.Object_id, gone.Language_id, gone.Object_name = gtables.ObjectTranslation_Object_type.Report, 99999, types.NewCode("en-US"), "Gone"
	if err := gone.InsertOrError(false); err != nil {
		t.Fatal(err)
	}
	// an object that is no longer there, and one whose caption changed
	for _, o := range []struct {
		objectType gtables.ObjectType
		id         int
		name       string
	}{{gtables.Object_Type.Report, 99999, "Gone"}, {gtables.Object_Type.Report, 204, "Old caption"}} {
		obj := gtables.NewObject(ftables.DBSource(db, "", database.DBTypeSQLite))
		obj.Type, obj.Id, obj.Name = o.objectType, o.id, types.NewText(o.name)
		if err := obj.InsertOrError(false); err != nil {
			t.Fatal(err)
		}
	}
	for run := 1; run <= 2; run++ {
		if err := SyncObjects(db, database.DBTypeSQLite); err != nil {
			t.Fatalf("run %d: %v", run, err)
		}
	}
	obj := gtables.NewObject(ftables.DBSource(db, "", database.DBTypeSQLite))
	if n := obj.Count(); n != len(Objects()) {
		t.Errorf("%d objects, want %d", n, len(Objects()))
	}
	get := func(objectType gtables.ObjectType, id int) string {
		obj.Reset()
		obj.SetRange("type", objectType)
		obj.SetRange("id", id)
		if !obj.FindFirst() {
			return ""
		}
		return obj.Name.String()
	}
	if got := get(gtables.Object_Type.Report, 99999); got != "" {
		t.Errorf("report 99999 still there (%q)", got)
	}
	if got := get(gtables.Object_Type.Report, 204); got == "" || got == "Old caption" {
		t.Errorf("report 204 %q, want its caption", got)
	}
	if got := get(gtables.Object_Type.Table, 77); got != "Report Selections" {
		t.Errorf("table 77 %q, want Report Selections", got)
	}
	if got := get(gtables.Object_Type.Page, 306); got == "" {
		t.Error("page 306 missing")
	}

	// Object Translation: every object in every language, the gone one deleted
	translations := gtables.NewObjectTranslation(ftables.DBSource(db, "", database.DBTypeSQLite))
	languages := i18n.GetInstance().GetSupportedLanguages()
	if n := translations.Count(); n != len(Objects())*len(languages) {
		t.Errorf("%d object translations, want %d objects × %d languages", n, len(Objects()), len(languages))
	}
	translations.SetRange("object_id", 99999)
	if !translations.IsEmpty() {
		t.Error("the translation of report 99999 is still there")
	}

	// Object Caption: the caption in the user's language (any letter case of the language)
	caption := func(language string, objectType gtables.ObjectType, id int) string {
		fcodeunits.SetCurrentContext("", "", "", language)
		defer fcodeunits.ClearCurrentContext()
		obj := gtables.NewObject(ftables.DBSource(db, "", database.DBTypeSQLite))
		if !obj.Get(map[string]interface{}{"type": objectType, "id": id}) {
			return ""
		}
		obj.CalcFields("caption")
		return obj.Caption.String()
	}
	for _, c := range []struct {
		language   string
		objectType gtables.ObjectType
		id         int
		want       string
	}{
		{"en-US", gtables.Object_Type.Codeunit, 80, "Sales-Post"},
		{"nb-NO", gtables.Object_Type.Codeunit, 80, "Salg – bokfør"},
		{"NB-NO", gtables.Object_Type.Codeunit, 80, "Salg – bokfør"},
		{"da-DK", gtables.Object_Type.Codeunit, 80, "Salg – bogfør"},
		{"en-US", gtables.Object_Type.Table, 18, "Customer"},
		{"nb-NO", gtables.Object_Type.Table, 18, "Kunde"},
		{"da-DK", gtables.Object_Type.Table, 18, "Debitor"},
		{"nb-NO", gtables.Object_Type.Page, 22, i18n.GetInstance().PageCaption("Customer List", "nb-NO")},
	} {
		if got := caption(c.language, c.objectType, c.id); got != c.want {
			t.Errorf("%s %d in %s: caption %q, want %q", c.objectType, c.id, c.language, got, c.want)
		}
	}
}

// Every codeunit has a caption in every language (translations/<lang>/codeunits.yaml,
// codeunits.<number>.caption): Object Caption shows it — a new codeunit needs one
func TestCodeunitCaptionsTranslated(t *testing.T) {
	ts := i18n.GetInstance()
	for key := range Objects() {
		if key.objectType != gtables.Object_Type.Codeunit {
			continue
		}
		for _, language := range ts.GetSupportedLanguages() {
			k := fmt.Sprintf("codeunits.%d.caption", key.id)
			if text := ts.Translate(k, language); text == k {
				t.Errorf("codeunit %d has no caption in %s (%s in translations/%s/codeunits.yaml)", key.id, language, k, language)
			}
		}
	}
}
