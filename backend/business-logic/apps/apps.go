// Package apps connects the installed apps (application.yaml, backend/foundation/apps) with
// the objects of this installation: every table, page and codeunit number must be in the
// series of an installed app, and the table Installed_App lists the apps.
package apps

import (
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/hansjlachmann/goeasyerp/backend/business-logic/codeunits"
	"github.com/hansjlachmann/goeasyerp/backend/business-logic/reports"
	fapps "github.com/hansjlachmann/goeasyerp/backend/foundation/apps"
	"github.com/hansjlachmann/goeasyerp/backend/foundation/database"
	fobjects "github.com/hansjlachmann/goeasyerp/backend/foundation/objects"
	"github.com/hansjlachmann/goeasyerp/backend/foundation/pages"
	"github.com/hansjlachmann/goeasyerp/backend/foundation/types"
	gtables "github.com/hansjlachmann/goeasyerp/backend/generated/tables"
)

// CheckObjectNumbers returns an error listing every table, page, report and codeunit (registered
// or declared) whose number is not in the series of one of the installed apps
func CheckObjectNumbers(installed []*fapps.App) error {
	err := fapps.CheckOverlap(installed)
	if err != nil {
		return err
	}
	allowed := func(objectType string, id int) bool {
		for _, a := range installed {
			if a.ObjectAllowed(objectType, id) {
				return true
			}
		}
		return false
	}
	var outside []string
	for id, rec := range gtables.NewRecords() {
		if !allowed(fapps.Table, id) {
			outside = append(outside, fmt.Sprintf("table %d %s", id, rec.GetTableName()))
		}
	}
	for id, page := range pages.GetRegistry().GetAllPages() {
		if !allowed(fapps.Page, id) {
			outside = append(outside, fmt.Sprintf("page %d %s", id, page.Page.Name))
		}
	}
	for id, name := range codeunits.List() {
		if !allowed(fapps.Codeunit, id) {
			outside = append(outside, fmt.Sprintf("codeunit %d %s", id, name))
		}
	}
	for id, name := range reports.List() {
		if !allowed(fapps.Report, id) {
			outside = append(outside, fmt.Sprintf("report %d %s", id, name))
		}
	}
	// the objects the business-logic packages declare (library codeunits)
	for _, d := range fobjects.Declared() {
		if d.Type == fobjects.TypeCodeunit && !allowed(fapps.Codeunit, d.ID) {
			outside = append(outside, fmt.Sprintf("codeunit %d %s (%s)", d.ID, d.Name, d.Package))
		}
	}
	if len(outside) > 0 {
		sort.Strings(outside)
		return fmt.Errorf("objects outside the number series of the installed apps: %s", strings.Join(outside, "; "))
	}
	return nil
}

// SyncInstalledApps writes one Installed_App row per installed app (inserted the first time,
// updated when the app's data changed)
func SyncInstalledApps(db database.Executor, dbType database.DBType, installed []*fapps.App) error {
	now := types.NewDateTimeFromTime(time.Now())
	for _, a := range installed {
		var rec gtables.InstalledApp
		rec.InitWithDBType(db, "", dbType)
		exists := rec.Get(types.NewText(a.ID))
		if !exists {
			rec.InitRecord()
			rec.App_id = types.NewText(a.ID)
			rec.Installed_at = now
		}
		before := rec.ToMap()
		rec.Name = types.NewText(a.Name)
		rec.Publisher = types.NewText(a.Publisher)
		rec.Contact = types.NewText(a.Contact)
		rec.Version = types.NewText(a.Version)
		rec.Object_series = types.NewText(seriesText(a.NumberSeries.Objects))
		rec.Field_series = types.NewText(seriesText(a.NumberSeries.Fields))
		rec.System_tables = types.NewText(seriesText(a.SystemTables))
		if exists && sameApp(before, rec.ToMap()) {
			continue
		}
		rec.Updated_at = now
		ok := exists && rec.Modify(true) || !exists && rec.Insert(true)
		if !ok {
			err := rec.TriggerError()
			if err != nil {
				return fmt.Errorf("installed app %s: %w", a.Name, err)
			}
			return fmt.Errorf("installed app %s could not be saved", a.Name)
		}
	}
	return nil
}

func sameApp(a, b map[string]interface{}) bool {
	for _, f := range []string{"name", "publisher", "contact", "version", "object_series", "field_series", "system_tables"} {
		if fmt.Sprint(a[f]) != fmt.Sprint(b[f]) {
			return false
		}
	}
	return true
}

func seriesText(ranges []fapps.Range) string {
	parts := make([]string, len(ranges))
	for i, r := range ranges {
		parts[i] = r.String()
	}
	return strings.Join(parts, ", ")
}
