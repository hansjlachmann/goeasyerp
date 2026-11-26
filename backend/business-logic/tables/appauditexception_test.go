package tables

import (
	"errors"
	"testing"

	fcodeunits "github.com/hansjlachmann/goeasyerp/backend/foundation/codeunits"
	"github.com/hansjlachmann/goeasyerp/backend/foundation/database"
	apperrors "github.com/hansjlachmann/goeasyerp/backend/foundation/errors"
	"github.com/hansjlachmann/goeasyerp/backend/foundation/session"
	gtables "github.com/hansjlachmann/goeasyerp/backend/generated/tables"
)

// Only SUPER users approve a finding of the app audit; who and when are recorded (S55)
func TestAppAuditExceptionSuperOnly(t *testing.T) {
	db := newTestDB(t)
	if err := (&gtables.AppAuditException{}).CreateTableWithDBType(db, "", database.DBTypeSQLite); err != nil {
		t.Fatal(err)
	}
	fcodeunits.SetCurrentContext("CLERK", "Clerk", "main", "en-US")
	fcodeunits.SetCurrentAccess(&session.Session{UserID: "CLERK", HasRoles: true})
	t.Cleanup(fcodeunits.ClearCurrentContext)
	exception := &gtables.AppAuditException{}
	exception.InitWithDBType(db, "", database.DBTypeSQLite)
	exception.App_id = "app"
	exception.Version = "1.0.0"
	exception.Source_id = "abc"
	exception.Rule = "import"
	exception.Finding_key = "os"
	exception.Reason = "reviewed"
	if exception.Insert(true) {
		t.Fatal("a user without SUPER approved a finding")
	}
	var appErr *apperrors.AppError
	if !errors.As(exception.TriggerError(), &appErr) || appErr.Code != apperrors.ErrAppAuditSuperOnly {
		t.Errorf("error %v", exception.TriggerError())
	}
	fcodeunits.SetCurrentContext("ADMIN", "Admin", "main", "en-US")
	fcodeunits.SetCurrentAccess(&session.Session{UserID: "ADMIN", HasRoles: true, IsSuper: true})
	if !exception.Insert(true) {
		t.Fatal(exception.TriggerError())
	}
	if exception.Approved_by != "ADMIN" || exception.Approved_at.IsZero() {
		t.Errorf("approved by %q at %v", exception.Approved_by, exception.Approved_at)
	}
}
