package tables

import (
	"errors"

	fcodeunits "github.com/hansjlachmann/goeasyerp/backend/foundation/codeunits"
	"github.com/hansjlachmann/goeasyerp/backend/foundation/i18n"
	gtables "github.com/hansjlachmann/goeasyerp/backend/generated/tables"
)

// installedAppLogic is the business logic of table 2000000153 Installed_App.
// The apps are installed, removed, enabled and disabled by the SUPER-only functions of page 907
// (Install App, Uninstall App, Disable App …) and written by the server itself at startup; a
// user who is not SUPER changes no row directly through the table API.
type installedAppLogic struct{}

func init() { gtables.RegisterInstalledAppLogic(installedAppLogic{}) }

// OnInsert: a SUPER user only (code without a user — the server's own sync — may)
func (installedAppLogic) OnInsert(rec *gtables.InstalledApp) error {
	return installedAppSuperOnly()
}

// OnModify: a SUPER user only
func (installedAppLogic) OnModify(rec *gtables.InstalledApp) error {
	return installedAppSuperOnly()
}

// OnDelete: a SUPER user only
func (installedAppLogic) OnDelete(rec *gtables.InstalledApp) error {
	return installedAppSuperOnly()
}

// OnRename: a SUPER user only
func (installedAppLogic) OnRename(rec *gtables.InstalledApp) error {
	return installedAppSuperOnly()
}

// installedAppSuperOnly refuses a user who is not SUPER (internal code without a user is
// allowed)
func installedAppSuperOnly() error {
	access := fcodeunits.CurrentAccess()
	if access != nil && !access.IsSuper {
		return errors.New(i18n.GetInstance().Message("ERR_APP_SUPER_ONLY", fcodeunits.CurrentLanguage()))
	}
	return nil
}
