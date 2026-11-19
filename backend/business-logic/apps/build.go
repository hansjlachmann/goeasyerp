package apps

import (
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	fapps "github.com/hansjlachmann/goeasyerp/backend/foundation/apps"
	fcodeunits "github.com/hansjlachmann/goeasyerp/backend/foundation/codeunits"
	"github.com/hansjlachmann/goeasyerp/backend/foundation/functions"
	"github.com/hansjlachmann/goeasyerp/backend/foundation/i18n"
	"github.com/hansjlachmann/goeasyerp/backend/foundation/system"
	"github.com/hansjlachmann/goeasyerp/backend/foundation/types"
	gtables "github.com/hansjlachmann/goeasyerp/backend/generated/tables"
)

// Building the extension apps in the installation (App Build Setup: Builds By = Local Builder).
// Build Apps records a build request (App Build, Requested) with the build list as it is then;
// the builder (cmd/app-builder) builds the program with the apps into a bundle and records the
// result, and the backend switches to the bundle by itself.

func init() {
	functions.Register("table", gtables.AppBuildTableName, "ShowLog", functions.For(ShowLog))
	functions.Register("table", gtables.InstalledAppTableName, "BuildApps", functions.For(BuildApps))
}

// ShowLog gives the builder's output of a build as a text file
func ShowLog(rec *gtables.AppBuild) (fcodeunits.Result, error) {
	lang := fcodeunits.CurrentLanguage()
	err := superOnly(lang)
	if err != nil {
		return fcodeunits.Result{}, err
	}
	if len(rec.Log) == 0 {
		return fcodeunits.Result{}, errors.New(i18n.GetInstance().Message("ERR_APP_BUILD_NO_LOG", lang))
	}
	return fcodeunits.Result{Success: true, Data: map[string]interface{}{
		"file":     base64.StdEncoding.EncodeToString(rec.Log),
		"filename": fmt.Sprintf("app-build-%d.log", rec.Entry_no),
	}}, nil
}

// BuildApps asks the installation's builder to build the program with the planned apps (App
// Build Setup: Builds By = Local Builder): an App Build request with the build list as it is now
func BuildApps(rec *gtables.InstalledApp) (fcodeunits.Result, error) {
	ts := i18n.GetInstance()
	lang := fcodeunits.CurrentLanguage()
	err := superOnly(lang)
	if err != nil {
		return fcodeunits.Result{}, err
	}
	setup := gtables.GetAppBuildSetup(rec)
	if setup.Builds_by != gtables.AppBuildSetup_Builds_by.LocalBuilder {
		return fcodeunits.Result{}, errors.New(ts.Message("ERR_APP_LOCAL_BUILDER_ONLY", lang))
	}
	build, err := RequestBuild(rec, gtables.AppBuild_Trigger.User)
	if err != nil {
		return fcodeunits.Result{}, err
	}
	text := ts.MessageWithParams("APP_BUILD_REQUESTED", lang, fmt.Sprint(build.Entry_no), build.Apps.String())
	return fcodeunits.Result{Success: true, Message: text}, nil
}

// RequestBuild records a build request of the planned apps (refused while another build is
// requested or running)
func RequestBuild(src *gtables.InstalledApp, trigger gtables.AppBuildTrigger) (*gtables.AppBuild, error) {
	ts := i18n.GetInstance()
	lang := fcodeunits.CurrentLanguage()
	pending := gtables.NewAppBuild(src)
	pending.SetFilter("status", fmt.Sprintf("%d|%d", gtables.AppBuild_Status.Requested, gtables.AppBuild_Status.Running))
	if pending.FindFirst() {
		return nil, errors.New(ts.MessageWithParams("ERR_APP_BUILD_PENDING", lang, fmt.Sprint(pending.Entry_no)))
	}
	list, err := plannedBuild(src)
	if err != nil {
		return nil, err
	}
	snapshot, err := json.Marshal(list)
	if err != nil {
		return nil, err
	}
	build := gtables.NewAppBuild(src)
	entryNo := 1
	if build.FindLast() {
		entryNo = build.Entry_no + 1
	}
	build.Init()
	build.Entry_no = entryNo
	build.Status = gtables.AppBuild_Status.Requested
	build.Base_version = types.NewText(list.Base.Version)
	build.Trigger = trigger
	build.Requested_by = types.NewCode(system.CopyStr(fcodeunits.CurrentUserID(), 1, 50))
	build.Requested_at = types.NewDateTimeFromTime(time.Now())
	build.Apps = types.NewText(system.CopyStr(buildAppsText(list, lang), 1, 250))
	build.Build_list = snapshot
	err = build.InsertOrError(false)
	if err != nil {
		return nil, err
	}
	return build, nil
}

// buildAppsText: the apps of a build list as text (Name version, …)
func buildAppsText(list fapps.BuildList, lang string) string {
	if len(list.Apps) == 0 {
		return i18n.GetInstance().Message("APP_BUILD_NO_APPS", lang)
	}
	parts := make([]string, len(list.Apps))
	for i, a := range list.Apps {
		parts[i] = a.Name + " " + a.Version
	}
	return strings.Join(parts, ", ")
}
