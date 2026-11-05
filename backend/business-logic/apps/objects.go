package apps

import (
	"fmt"
	"strings"

	"github.com/hansjlachmann/goeasyerp/backend/business-logic/codeunits"
	"github.com/hansjlachmann/goeasyerp/backend/business-logic/reports"
	"github.com/hansjlachmann/goeasyerp/backend/foundation/database"
	"github.com/hansjlachmann/goeasyerp/backend/foundation/i18n"
	fobjects "github.com/hansjlachmann/goeasyerp/backend/foundation/objects"
	"github.com/hansjlachmann/goeasyerp/backend/foundation/pages"
	ftables "github.com/hansjlachmann/goeasyerp/backend/foundation/tables"
	"github.com/hansjlachmann/goeasyerp/backend/foundation/types"
	gtables "github.com/hansjlachmann/goeasyerp/backend/generated/tables"
)

// objectKey is an object's type and number
type objectKey struct {
	objectType gtables.ObjectType
	id         int
}

// objectRow is what the Object table holds of an object: its name, whether it runs, and its
// caption in a language (Object Translation; nil: the name in every language)
type objectRow struct {
	name     string
	runnable bool
	caption  func(language string) string
}

// captionIn is the object's caption in the language: its translation, else its name
func (row objectRow) captionIn(language string) string {
	if row.caption == nil {
		return row.name
	}
	caption := row.caption(language)
	if caption == "" || untranslated(caption) {
		return row.name
	}
	return caption
}

// untranslated: the translation service gave the key back ("pages.x.caption" — no translation)
func untranslated(text string) bool {
	return strings.HasSuffix(text, ".caption") && !strings.Contains(text, " ")
}

// Objects returns the objects of this installation with their (English) captions: the
// tables, pages, reports and codeunits — the registered codeunits (runnable) and those the
// business-logic packages declare (objects.Codeunit: library codeunits, with their classic
// names)
func Objects() map[objectKey]objectRow {
	objects := map[objectKey]objectRow{}
	for id, rec := range gtables.NewRecords() {
		caption := rec.GetTableName()
		var captionIn func(string) string
		if c, ok := rec.(interface{ GetCaption(string) string }); ok {
			caption = c.GetCaption("en-US")
			captionIn = c.GetCaption
		}
		objects[objectKey{gtables.Object_Type.Table, id}] = objectRow{name: caption, caption: captionIn}
	}
	ts := i18n.GetInstance()
	for id, page := range pages.GetRegistry().GetAllPages() {
		caption := page.Page.Caption
		if caption == "" {
			caption = page.Page.Name
		}
		pageName := page.Page.Name
		objects[objectKey{gtables.Object_Type.Page, id}] = objectRow{name: caption, caption: func(language string) string {
			return ts.PageCaption(pageName, language)
		}}
	}
	for id := range reports.List() {
		reportID := id
		objects[objectKey{gtables.Object_Type.Report, id}] = objectRow{name: reports.Caption(id, "en-US"), runnable: true, caption: func(language string) string {
			return reports.Caption(reportID, language)
		}}
	}
	codeunitCaption := func(id int) func(string) string {
		return func(language string) string {
			return ts.Translate(fmt.Sprintf("codeunits.%d.caption", id), language)
		}
	}
	for _, d := range fobjects.Declared() {
		if d.Type == fobjects.TypeCodeunit {
			objects[objectKey{gtables.Object_Type.Codeunit, d.ID}] = objectRow{name: d.Name, caption: codeunitCaption(d.ID)}
		}
	}
	for id, name := range codeunits.List() {
		if declared := fobjects.DeclaredName(fobjects.TypeCodeunit, id); declared != "" {
			name = declared
		}
		objects[objectKey{gtables.Object_Type.Codeunit, id}] = objectRow{name: name, runnable: true, caption: codeunitCaption(id)}
	}
	return objects
}

// SyncObjects writes the table Object (classic Object/AllObj): one row per table, page, report
// and codeunit of this installation (new ones inserted, changed captions updated, objects no
// longer there deleted) — what fields with a relation to objects look up (Report Selections'
// Report ID)
func SyncObjects(db database.Executor, dbType database.DBType) error {
	objects := Objects()
	src := ftables.DBSource(db, "", dbType)
	rec := gtables.NewObject(src)
	stored := map[objectKey]bool{}
	for found := rec.FindSetBuffered(); found; found = rec.Next() {
		key := objectKey{rec.Type, rec.Id}
		row, ok := objects[key]
		name := row.name
		switch {
		case !ok:
			old := gtables.NewObject(src)
			old.Copy(rec)
			if err := old.DeleteOrError(false); err != nil {
				return fmt.Errorf("object %d %d: %w", key.objectType, key.id, err)
			}
			continue
		case rec.Name.String() != truncate(name, 80) || rec.Runnable != row.runnable:
			changed := gtables.NewObject(src)
			changed.Copy(rec)
			changed.Name = types.NewText(truncate(name, 80))
			changed.Runnable = row.runnable
			if err := changed.ModifyOrError(false); err != nil {
				return fmt.Errorf("object %d %d: %w", key.objectType, key.id, err)
			}
		}
		stored[key] = true
	}
	for key, row := range objects {
		if stored[key] {
			continue
		}
		obj := gtables.NewObject(src)
		obj.Init()
		obj.Type = key.objectType
		obj.Id = key.id
		obj.Name = types.NewText(truncate(row.name, 80))
		obj.Runnable = row.runnable
		if err := obj.InsertOrError(false); err != nil {
			return fmt.Errorf("object %d %d: %w", key.objectType, key.id, err)
		}
	}
	return syncObjectTranslations(src, objects)
}

// syncObjectTranslations writes table 377 Object Translation: every object's caption in every
// language of the translation files (new ones inserted, changed captions updated, those of
// objects or languages no longer there deleted) — what Object's Caption reads in the user's
// language
func syncObjectTranslations(src ftables.Source, objects map[objectKey]objectRow) error {
	type translationKey struct {
		objectKey
		language string
	}
	want := map[translationKey]string{}
	for key, row := range objects {
		for _, language := range i18n.GetInstance().GetSupportedLanguages() {
			// Language ID is a Code field: upper case ("NB-NO")
			want[translationKey{key, strings.ToUpper(language)}] = truncate(row.captionIn(language), 80)
		}
	}
	rec := gtables.NewObjectTranslation(src)
	for found := rec.FindSetBuffered(); found; found = rec.Next() {
		key := translationKey{objectKey{gtables.ObjectType(rec.Object_type), rec.Object_id}, rec.Language_id.String()}
		caption, ok := want[key]
		delete(want, key)
		switch {
		case !ok:
			old := gtables.NewObjectTranslation(src)
			old.Copy(rec)
			err := old.DeleteOrError(false)
			if err != nil {
				return fmt.Errorf("object translation %d %d %s: %w", key.objectType, key.id, key.language, err)
			}
		case rec.Object_name.String() != caption:
			changed := gtables.NewObjectTranslation(src)
			changed.Copy(rec)
			changed.Object_name = types.NewText(caption)
			err := changed.ModifyOrError(false)
			if err != nil {
				return fmt.Errorf("object translation %d %d %s: %w", key.objectType, key.id, key.language, err)
			}
		}
	}
	var inserts []*gtables.ObjectTranslation
	for key, caption := range want {
		t := gtables.NewObjectTranslation(src)
		t.Object_type = gtables.ObjectTranslationObject_type(key.objectType)
		t.Object_id = key.id
		t.Language_id = types.NewCode(key.language)
		t.Object_name = types.NewText(caption)
		inserts = append(inserts, t)
	}
	if len(inserts) == 0 {
		return nil
	}
	return gtables.NewObjectTranslation(src).InsertAll(inserts, false)
}

func truncate(s string, n int) string {
	r := []rune(s)
	if len(r) > n {
		return string(r[:n])
	}
	return s
}
