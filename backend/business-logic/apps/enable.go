package apps

import (
	"errors"
	"sync"

	fapps "github.com/hansjlachmann/goeasyerp/backend/foundation/apps"
	fcodeunits "github.com/hansjlachmann/goeasyerp/backend/foundation/codeunits"
	"github.com/hansjlachmann/goeasyerp/backend/foundation/database"
	"github.com/hansjlachmann/goeasyerp/backend/foundation/extensions"
	"github.com/hansjlachmann/goeasyerp/backend/foundation/functions"
	"github.com/hansjlachmann/goeasyerp/backend/foundation/i18n"
	ftables "github.com/hansjlachmann/goeasyerp/backend/foundation/tables"
	gtables "github.com/hansjlachmann/goeasyerp/backend/generated/tables"
)

// An extension app can be switched off and on (Installed Apps, page 907; SUPER only). A
// disabled app stays built in and keeps its data, but its business logic, event subscribers and
// field checks do not run, its page parts and menu entries do not show, and its objects and
// functions are refused (backend/foundation/extensions). The base application is never
// disabled; an app is not disabled while an enabled app depends on it, nor enabled while an app
// it depends on is disabled.

func init() {
	functions.Register("table", gtables.InstalledAppTableName, "EnableApp", functions.For(EnableApp))
	functions.Register("table", gtables.InstalledAppTableName, "DisableApp", functions.For(DisableApp))
}

var (
	installedMu sync.RWMutex
	installed   []*fapps.App // the installed apps, base first (SyncInstalledApps)
)

// setInstalled keeps the installed apps for the dependency checks and tells extensions how an
// object's app is found (by number series; the base application is "")
func setInstalled(list []*fapps.App) {
	installedMu.Lock()
	installed = list
	installedMu.Unlock()
	extensions.SetObjectApps(func(objectType string, id int) string {
		installedMu.RLock()
		defer installedMu.RUnlock()
		for i, a := range installed {
			if a.ObjectAllowed(objectType, id) {
				if i == 0 {
					return ""
				}
				return a.ID
			}
		}
		return ""
	})
}

// LoadDisabled reads the disabled apps (Installed_App Disabled) into the running program — at
// startup and again now and then, so several servers follow a switch made on one of them
func LoadDisabled(db database.Executor, dbType database.DBType) error {
	var rec gtables.InstalledApp
	rec.InitWithDBType(db, "", dbType)
	return loadDisabled(&rec)
}

func loadDisabled(src ftables.Source) error {
	rec := gtables.NewInstalledApp(src)
	rec.SetRange("disabled", true)
	var ids []string
	for found := rec.FindSet(); found; found = rec.Next() {
		ids = append(ids, rec.App_id.String())
	}
	extensions.SetDisabled(ids...)
	return nil
}

// EnableApp switches an extension app on again
func EnableApp(rec *gtables.InstalledApp) (fcodeunits.Result, error) {
	return setDisabled(rec, false)
}

// DisableApp switches an extension app off
func DisableApp(rec *gtables.InstalledApp) (fcodeunits.Result, error) {
	return setDisabled(rec, true)
}

func setDisabled(rec *gtables.InstalledApp, disable bool) (fcodeunits.Result, error) {
	ts := i18n.GetInstance()
	lang := fcodeunits.CurrentLanguage()
	access := fcodeunits.CurrentAccess()
	if access != nil && !access.IsSuper {
		recordRefused("enable/disable app (SUPER only)")
		return fcodeunits.Result{}, errors.New(ts.Message("ERR_APP_SUPER_ONLY", lang))
	}
	installedMu.RLock()
	list := installed
	installedMu.RUnlock()
	if len(list) > 0 && rec.App_id.String() == list[0].ID {
		return fcodeunits.Result{}, errors.New(ts.MessageWithParams("ERR_APP_BASE_ALWAYS_ON", lang, rec.Name.String()))
	}
	if rec.Disabled == disable {
		return fcodeunits.Result{Success: true}, nil
	}

	disabledNow := map[string]bool{}
	other := gtables.NewInstalledApp(rec)
	other.SetRange("disabled", true)
	for found := other.FindSet(); found; found = other.Next() {
		disabledNow[other.App_id.String()] = true
	}
	id := rec.App_id.String()
	for _, a := range list {
		for _, d := range a.Dependencies {
			// disabling: an enabled app depends on this one
			if disable && d.ID == id && a.ID != id && !disabledNow[a.ID] {
				return fcodeunits.Result{}, errors.New(ts.MessageWithParams("ERR_APP_DEPENDED_ON", lang, rec.Name.String(), a.Name))
			}
			// enabling: this app depends on a disabled one
			if !disable && a.ID == id && disabledNow[d.ID] {
				return fcodeunits.Result{}, errors.New(ts.MessageWithParams("ERR_APP_DEPENDENCY_DISABLED", lang, rec.Name.String(), d.Name))
			}
		}
	}

	rec.Disabled = disable
	err := rec.ModifyOrError(false)
	if err != nil {
		return fcodeunits.Result{}, err
	}
	err = loadDisabled(rec)
	if err != nil {
		return fcodeunits.Result{}, err
	}
	key := "APP_ENABLED"
	action := "enable app "
	if disable {
		key = "APP_DISABLED"
		action = "disable app "
	}
	recordAdministration(action + rec.Name.String())
	text := ts.MessageWithParams(key, lang, rec.Name.String())
	return fcodeunits.Result{Success: true, Message: text, Refresh: true}, nil
}
