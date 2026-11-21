package tables

import (
	"errors"

	fapps "github.com/hansjlachmann/goeasyerp/backend/foundation/apps"
	fcodeunits "github.com/hansjlachmann/goeasyerp/backend/foundation/codeunits"
	"github.com/hansjlachmann/goeasyerp/backend/foundation/i18n"
	gtables "github.com/hansjlachmann/goeasyerp/backend/generated/tables"
)

// appBuildSetupLogic is the business logic of table 2000000156 App_Build_Setup.
// Add a trigger when it is needed: the available triggers are listed at the
// top of generated/tables/appbuildsetup_gen.go. The field rules of the definition
// (required, length, range) are checked after OnInsert and OnModify.
type appBuildSetupLogic struct{}

func init() { gtables.RegisterAppBuildSetupLogic(appBuildSetupLogic{}) }

// OnInsert: as OnModify (the setup's record is made the first time it is saved)
func (l appBuildSetupLogic) OnInsert(rec *gtables.AppBuildSetup) error {
	return l.OnModify(rec)
}

// OnModify: how apps are built and where they may come from — a SUPER user only (code without a
// user may)
func (appBuildSetupLogic) OnModify(rec *gtables.AppBuildSetup) error {
	access := fcodeunits.CurrentAccess()
	if access != nil && !access.IsSuper {
		return errors.New(i18n.GetInstance().Message("ERR_APP_SUPER_ONLY", fcodeunits.CurrentLanguage()))
	}
	return nil
}

// OnValidate_Allowed_repositories: GitHub owners and owner/repository entries only
func (appBuildSetupLogic) OnValidate_Allowed_repositories(rec *gtables.AppBuildSetup) error {
	_, err := fapps.ParseAllowedRepositories(rec.Allowed_repositories.String())
	if err != nil {
		return errors.New(i18n.GetInstance().MessageWithParams("ERR_APP_ALLOWED_REPOSITORIES", fcodeunits.CurrentLanguage(), err.Error()))
	}
	return nil
}

// OnDelete runs before a record is deleted
func (appBuildSetupLogic) OnDelete(rec *gtables.AppBuildSetup) error {
	return nil
}

// OnRename runs before a primary key field is changed (rec.OldValue has the old value)
func (appBuildSetupLogic) OnRename(rec *gtables.AppBuildSetup) error {
	return nil
}
