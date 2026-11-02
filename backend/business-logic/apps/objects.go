package apps

import (
	"fmt"

	"github.com/hansjlachmann/openerp/backend/business-logic/codeunits"
	"github.com/hansjlachmann/openerp/backend/business-logic/reports"
	"github.com/hansjlachmann/openerp/backend/foundation/database"
	"github.com/hansjlachmann/openerp/backend/foundation/pages"
	ftables "github.com/hansjlachmann/openerp/backend/foundation/tables"
	"github.com/hansjlachmann/openerp/backend/foundation/types"
	gtables "github.com/hansjlachmann/openerp/backend/generated/tables"
)

// objectKey is an object's type and number
type objectKey struct {
	objectType gtables.ObjectType
	id         int
}

// Objects returns the objects of this installation with their (English) captions: the
// tables, pages, reports and codeunits
func Objects() map[objectKey]string {
	objects := map[objectKey]string{}
	for id, rec := range gtables.NewRecords() {
		caption := rec.GetTableName()
		if c, ok := rec.(interface{ GetCaption(string) string }); ok {
			caption = c.GetCaption("en-US")
		}
		objects[objectKey{gtables.Object_Type.Table, id}] = caption
	}
	for id, page := range pages.GetRegistry().GetAllPages() {
		caption := page.Page.Caption
		if caption == "" {
			caption = page.Page.Name
		}
		objects[objectKey{gtables.Object_Type.Page, id}] = caption
	}
	for id := range reports.List() {
		objects[objectKey{gtables.Object_Type.Report, id}] = reports.Caption(id, "en-US")
	}
	for id, name := range codeunits.List() {
		objects[objectKey{gtables.Object_Type.Codeunit, id}] = name
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
		name, ok := objects[key]
		switch {
		case !ok:
			old := gtables.NewObject(src)
			old.Copy(rec)
			if err := old.DeleteOrError(false); err != nil {
				return fmt.Errorf("object %d %d: %w", key.objectType, key.id, err)
			}
			continue
		case rec.Name.String() != truncate(name, 80):
			changed := gtables.NewObject(src)
			changed.Copy(rec)
			changed.Name = types.NewText(truncate(name, 80))
			if err := changed.ModifyOrError(false); err != nil {
				return fmt.Errorf("object %d %d: %w", key.objectType, key.id, err)
			}
		}
		stored[key] = true
	}
	for key, name := range objects {
		if stored[key] {
			continue
		}
		obj := gtables.NewObject(src)
		obj.Init()
		obj.Type = key.objectType
		obj.Id = key.id
		obj.Name = types.NewText(truncate(name, 80))
		if err := obj.InsertOrError(false); err != nil {
			return fmt.Errorf("object %d %d: %w", key.objectType, key.id, err)
		}
	}
	return nil
}

func truncate(s string, n int) string {
	r := []rune(s)
	if len(r) > n {
		return string(r[:n])
	}
	return s
}
