package apps

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"os"
	"strings"

	fapps "github.com/hansjlachmann/goeasyerp/backend/foundation/apps"
	fcodeunits "github.com/hansjlachmann/goeasyerp/backend/foundation/codeunits"
	"github.com/hansjlachmann/goeasyerp/backend/foundation/database"
	"github.com/hansjlachmann/goeasyerp/backend/foundation/i18n"
	ftables "github.com/hansjlachmann/goeasyerp/backend/foundation/tables"
	"github.com/hansjlachmann/goeasyerp/backend/foundation/types"
	gtables "github.com/hansjlachmann/goeasyerp/backend/generated/tables"
)

// Publishing an app from VS Code (Extension App development): the GoEasyERP VS Code extension
// sends the app's package to the development endpoint (api/handlers/devapps.go), which publishes
// it here — the same checks and app audit as Upload App, then a build of the installation's own
// builder — and the server switches to the build by itself. The same version may be published
// again (the developer's next F5). Only a server started for development offers it: production
// takes apps as released versions (Install App, Upload App).

const (
	// EnvDevEndpoint: "true" offers the development endpoint (the development compose file sets it)
	EnvDevEndpoint = "GOEASYERP_DEV_ENDPOINT"
	// EnvDevEndpointRemote: "true" lets it answer other machines too (a shared test server);
	// without it only this machine (the developer's own container)
	EnvDevEndpointRemote = "GOEASYERP_DEV_ENDPOINT_REMOTE"
)

// devMaxUsers: a database with more users is not taken for a development database
const devMaxUsers = 10

// DevelopmentRequested: the server was started with the development endpoint
func DevelopmentRequested() bool {
	return envTrue(EnvDevEndpoint)
}

// DevelopmentRemoteAllowed: the development endpoint may answer other machines
func DevelopmentRemoteAllowed() bool {
	return envTrue(EnvDevEndpointRemote)
}

func envTrue(name string) bool {
	v := strings.ToLower(strings.TrimSpace(os.Getenv(name)))
	return v == "true" || v == "1" || v == "yes"
}

// DevelopmentAllowed: the database may take development publishes — it does not look like a
// production database: every company with customers is a demo company, and it has at most
// devMaxUsers users. reason says why not.
func DevelopmentAllowed(db database.Executor, dbType database.DBType) (bool, string) {
	src := ftables.DBSource(db, "", dbType)
	user := gtables.NewUser(src)
	users := user.Count()
	if users > devMaxUsers {
		return false, fmt.Sprintf("the database has %d users (a development database has at most %d)", users, devMaxUsers)
	}
	// the companies first, then their customers (one connection: no query while the list is read)
	var others []string
	company := gtables.NewCompany(src)
	for found := company.FindSet(); found; found = company.Next() {
		if !company.Demo_company {
			others = append(others, company.Name.String())
		}
	}
	for _, name := range others {
		customer := gtables.NewCustomer(ftables.DBSource(db, name, dbType))
		if !customer.IsEmpty() {
			return false, fmt.Sprintf("company %s has customers and is not a demo company", name)
		}
	}
	return true, ""
}

// DevPublish is the result of a development publish
type DevPublish struct {
	Build   int    `json:"build_no"`
	AppID   string `json:"app_id"`
	Name    string `json:"name"`
	Version string `json:"version"`
	SHA256  string `json:"sha256"`
}

// PublishDevelopment publishes an app package from VS Code: SUPER only, the installation's own
// builder; the package is checked and audited as Upload App does (an *AuditError when the audit
// refuses it), recorded — the same version replacing its package —, and a build is requested.
// Refused while another build is requested or running (nothing is recorded then).
func PublishDevelopment(src *gtables.InstalledApp, data []byte) (*DevPublish, error) {
	ts := i18n.GetInstance()
	lang := fcodeunits.CurrentLanguage()
	err := superOnly(lang)
	if err != nil {
		return nil, err
	}
	setup := gtables.GetAppBuildSetup(src)
	if setup.Builds_by != gtables.AppBuildSetup_Builds_by.LocalBuilder {
		return nil, errors.New(ts.Message("ERR_APP_LOCAL_BUILDER_ONLY", lang))
	}
	err = refusePending(src)
	if err != nil {
		return nil, err
	}
	pkg, err := fapps.ReadPackage(data)
	if err != nil {
		return nil, errors.New(ts.MessageWithParams("ERR_APP_PACKAGE", lang, err.Error()))
	}
	_, err = uploadPackageFrom(src, data, true)
	if err != nil {
		return nil, err
	}
	sum := sha256.Sum256(data)
	build, err := RequestBuild(src, gtables.AppBuild_Trigger.Development)
	if err != nil {
		return nil, err
	}
	recordAdministration(fmt.Sprintf("publish app %s %s from VS Code (build %d)", pkg.App.Name, pkg.App.Version, build.Entry_no))
	return &DevPublish{Build: build.Entry_no, AppID: pkg.App.ID, Name: pkg.App.Name, Version: pkg.App.Version,
		SHA256: hex.EncodeToString(sum[:])}, nil
}

// UnpublishDevelopment takes a published app out again (Uninstall App) and requests the build
// without it
func UnpublishDevelopment(src *gtables.InstalledApp, appID string) (*DevPublish, error) {
	ts := i18n.GetInstance()
	lang := fcodeunits.CurrentLanguage()
	err := superOnly(lang)
	if err != nil {
		return nil, err
	}
	err = refusePending(src)
	if err != nil {
		return nil, err
	}
	rec := gtables.NewInstalledApp(src)
	if !rec.Get(types.NewText(appID)) {
		return nil, errors.New(ts.MessageWithParams("ERR_APP_DEV_NOT_PUBLISHED", lang, appID))
	}
	name, version := rec.Name.String(), rec.Version.String()
	_, err = UninstallApp(rec)
	if err != nil {
		return nil, err
	}
	build, err := RequestBuild(src, gtables.AppBuild_Trigger.Development)
	if err != nil {
		return nil, err
	}
	recordAdministration(fmt.Sprintf("unpublish app %s %s from VS Code (build %d)", name, version, build.Entry_no))
	return &DevPublish{Build: build.Entry_no, AppID: appID, Name: name, Version: version}, nil
}
