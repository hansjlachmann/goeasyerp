package tables

import (
	fcodeunits "github.com/hansjlachmann/goeasyerp/backend/foundation/codeunits"
	apperrors "github.com/hansjlachmann/goeasyerp/backend/foundation/errors"
	"github.com/hansjlachmann/goeasyerp/backend/foundation/securitylog"
	"github.com/hansjlachmann/goeasyerp/backend/foundation/types"
	gtables "github.com/hansjlachmann/goeasyerp/backend/generated/tables"
)

// appAuditExceptionLogic is the business logic of table 2000000159 App_Audit_Exception: a
// finding of the app audit a SUPER user approved for one app's exact source (S55). Approving
// and withdrawing are security events; the change log records the records too.
type appAuditExceptionLogic struct{}

func init() { gtables.RegisterAppAuditExceptionLogic(appAuditExceptionLogic{}) }

// OnInsert: a SUPER user approves; who and when are recorded
func (appAuditExceptionLogic) OnInsert(rec *gtables.AppAuditException) error {
	err := appAuditSuperOnly()
	if err != nil {
		return err
	}
	rec.Approved_by = types.NewCode(fcodeunits.CurrentUserID())
	rec.Approved_at = types.Now()
	recordAppAuditException(rec, "approved")
	return nil
}

// OnModify: only the reason changes (the key is the approval)
func (appAuditExceptionLogic) OnModify(rec *gtables.AppAuditException) error {
	return appAuditSuperOnly()
}

// OnDelete: a SUPER user withdraws the approval
func (appAuditExceptionLogic) OnDelete(rec *gtables.AppAuditException) error {
	err := appAuditSuperOnly()
	if err != nil {
		return err
	}
	recordAppAuditException(rec, "withdrawn")
	return nil
}

// OnRename: an approval is not moved to another app, version, source or finding
func (appAuditExceptionLogic) OnRename(rec *gtables.AppAuditException) error {
	err := appAuditSuperOnly()
	if err != nil {
		return err
	}
	recordAppAuditException(rec, "changed")
	return nil
}

// appAuditSuperOnly refuses a user who is not SUPER (code without a user may)
func appAuditSuperOnly() error {
	access := fcodeunits.CurrentAccess()
	if access != nil && !access.IsSuper {
		return apperrors.AppAuditSuperOnly()
	}
	return nil
}

// recordAppAuditException notes the approval as a security event (Administration)
func recordAppAuditException(rec *gtables.AppAuditException, what string) {
	securitylog.Record(securitylog.Event{Kind: securitylog.Administration, UserID: fcodeunits.CurrentUserID(),
		Company: fcodeunits.CurrentCompany(),
		Details: "app audit exception " + what + ": " + rec.App_id.String() + " " + rec.Version.String() +
			" [" + rec.Rule.String() + "] " + rec.Finding_key.String()})
}
