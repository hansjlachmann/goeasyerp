package tables

import (
	"errors"

	fcodeunits "github.com/hansjlachmann/goeasyerp/backend/foundation/codeunits"
	apperrors "github.com/hansjlachmann/goeasyerp/backend/foundation/errors"
	"github.com/hansjlachmann/goeasyerp/backend/foundation/mail"
	gtables "github.com/hansjlachmann/goeasyerp/backend/generated/tables"
)

// sMTPSetupLogic is the business logic of table 409 SMTP_Setup: where the server sends mail.
// Only SUPER users change it, and its server must not be an address the server refuses to
// connect to (S49: loopback, link-local, …; foundation/mail checks names again when sending).
type sMTPSetupLogic struct{}

func init() { gtables.RegisterSMTPSetupLogic(sMTPSetupLogic{}) }

// OnInsert runs before a new record is inserted
func (sMTPSetupLogic) OnInsert(rec *gtables.SMTPSetup) error {
	return smtpSetupSuperOnly()
}

// OnModify runs before a record is modified
func (sMTPSetupLogic) OnModify(rec *gtables.SMTPSetup) error {
	return smtpSetupSuperOnly()
}

// OnDelete runs before a record is deleted
func (sMTPSetupLogic) OnDelete(rec *gtables.SMTPSetup) error {
	return smtpSetupSuperOnly()
}

// OnRename runs before a primary key field is changed (rec.OldValue has the old value)
func (sMTPSetupLogic) OnRename(rec *gtables.SMTPSetup) error {
	return smtpSetupSuperOnly()
}

// OnValidate_Smtp_server refuses an address the server never connects to
func (sMTPSetupLogic) OnValidate_Smtp_server(rec *gtables.SMTPSetup) error {
	err := mail.CheckHost(rec.Smtp_server.String())
	var refused *mail.AddressRefusedError
	if errors.As(err, &refused) {
		return apperrors.SMTPServerNotAllowed(rec.Smtp_server.String(), refused.Reason)
	}
	return nil
}

// smtpSetupSuperOnly refuses a user who is not SUPER (code without a user — setup, tests — may)
func smtpSetupSuperOnly() error {
	access := fcodeunits.CurrentAccess()
	if access != nil && !access.IsSuper {
		return apperrors.SMTPSetupSuperOnly()
	}
	return nil
}
