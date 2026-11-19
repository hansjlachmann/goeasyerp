package apps

import (
	"encoding/base64"
	"errors"
	"fmt"

	fcodeunits "github.com/hansjlachmann/goeasyerp/backend/foundation/codeunits"
	"github.com/hansjlachmann/goeasyerp/backend/foundation/functions"
	"github.com/hansjlachmann/goeasyerp/backend/foundation/i18n"
	gtables "github.com/hansjlachmann/goeasyerp/backend/generated/tables"
)

// Building the extension apps in the installation (App Build Setup: Builds By = Local Builder).
// Build Apps records a build request (App Build, Requested) with the build list as it is then;
// the builder (cmd/app-builder) builds the program with the apps into a bundle and records the
// result, and the backend switches to the bundle by itself.

func init() {
	functions.Register("table", gtables.AppBuildTableName, "ShowLog", functions.For(ShowLog))
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
