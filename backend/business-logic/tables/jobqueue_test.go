package tables

import (
	"strings"
	"testing"

	fcodeunits "github.com/hansjlachmann/goeasyerp/backend/foundation/codeunits"
	"github.com/hansjlachmann/goeasyerp/backend/foundation/database"
	"github.com/hansjlachmann/goeasyerp/backend/foundation/session"
	"github.com/hansjlachmann/goeasyerp/backend/foundation/types"
	gtables "github.com/hansjlachmann/goeasyerp/backend/generated/tables"
)

func TestJobQueueNotificationEmail(t *testing.T) {
	var jq gtables.JobQueue
	jq.InitWithDBType(nil, "TEST", database.DBTypeSQLite)

	if err := jq.ValidateField("notification_email", " ops@example.com;hans@example.com ; "); err != nil {
		t.Fatalf("valid list rejected: %v", err)
	}
	if got := jq.Notification_email.String(); got != "ops@example.com; hans@example.com" {
		t.Errorf("stored list = %q, want it tidied up", got)
	}
	if err := jq.ValidateField("notification_email", ""); err != nil {
		t.Errorf("blank list rejected: %v", err)
	}

	for _, tc := range []struct{ list, message string }{
		{"ops@example.com; ops@", "Notification E-mail: 'ops@' is not a valid e-mail address"},
		{"ops@example.com; OPS@example.com", "Notification E-mail: 'OPS@example.com' is listed more than once"},
	} {
		err := jq.ValidateField("notification_email", tc.list)
		if err == nil || err.Error() != tc.message {
			t.Errorf("%q: error %v, want %q", tc.list, err, tc.message)
		}
	}
}

func TestJobQueueNotificationLanguage(t *testing.T) {
	db := newTestDB(t)
	var language gtables.Language
	language.InitWithDBType(db, "", database.DBTypeSQLite)
	if err := language.CreateTableWithDBType(db, "", database.DBTypeSQLite); err != nil {
		t.Fatal(err)
	}
	language.Code = types.NewCode("NB-NO")
	language.Translation_key = types.NewText("nb-NO")
	if !language.Insert(true) {
		t.Fatal("insert language")
	}

	var jq gtables.JobQueue
	jq.InitWithDBType(db, "TEST", database.DBTypeSQLite)
	if err := jq.ValidateField("notification_language", "NB-NO"); err != nil {
		t.Errorf("existing language rejected: %v", err)
	}
	if err := jq.ValidateField("notification_language", "XX-XX"); err == nil {
		t.Error("unknown language accepted")
	}
}

// A Job Queue entry runs as the user who saved it: saving stamps the user and needs Execute
// permission on its codeunit; without a user context (scheduler, startup) nothing is stamped
func TestJobQueueUser(t *testing.T) {
	db := newTestDB(t)
	if err := (&gtables.JobQueue{}).CreateTableWithDBType(db, "TEST", database.DBTypeSQLite); err != nil {
		t.Fatal(err)
	}
	newJob := func(no string, codeunit int) *gtables.JobQueue {
		jq := &gtables.JobQueue{}
		jq.InitWithDBType(db, "TEST", database.DBTypeSQLite)
		jq.No = types.NewCode(no)
		jq.Object_id_to_run = codeunit
		return jq
	}

	if jq := newJob("J0", 6); !jq.Insert(true) || jq.User_id != "" {
		t.Fatalf("without a user context: inserted with user %q (%v)", jq.User_id, jq.TriggerError())
	}

	reader := &session.Session{UserID: "READER1", Company: "TEST"}
	reader.SetPermissions(true, false, map[string]session.TablePermission{"CODEUNIT 6": {Execute: true}})
	fcodeunits.SetCurrentContext("READER1", "READER1", "TEST", "en-US")
	fcodeunits.SetCurrentAccess(reader)
	t.Cleanup(fcodeunits.ClearCurrentContext)

	jq := newJob("J1", 6)
	if !jq.Insert(true) {
		t.Fatalf("insert with Execute: %v", jq.TriggerError())
	}
	if jq.User_id != "READER1" {
		t.Errorf("User ID = %q, want READER1", jq.User_id)
	}
	if denied := newJob("J2", 7); denied.Insert(true) {
		t.Error("insert without Execute on codeunit 7 was allowed")
	} else if msg := denied.TriggerError().Error(); !strings.Contains(msg, "READER1") || !strings.Contains(msg, "routine 7") {
		t.Errorf("refusal message %q", msg)
	}
	jq.Object_id_to_run = 7
	if jq.Modify(true) {
		t.Error("changing the codeunit to one without Execute was allowed")
	}
}

// A report or codeunit Permission row is keyed by its object
func TestPermissionObjectKey(t *testing.T) {
	var perm gtables.Permission
	perm.InitWithDBType(nil, "", database.DBTypeSQLite)
	if err := perm.ValidateField("object_type", gtables.Permission_Object_type.Codeunit); err != nil {
		t.Fatal(err)
	}
	if err := perm.ValidateField("object_id", 6); err != nil {
		t.Fatal(err)
	}
	if perm.Table_name != "CODEUNIT 6" {
		t.Errorf("codeunit row key = %q", perm.Table_name)
	}
	if err := perm.ValidateField("object_type", gtables.Permission_Object_type.Report); err != nil {
		t.Fatal(err)
	}
	if perm.Table_name != "REPORT 6" {
		t.Errorf("report row key = %q", perm.Table_name)
	}
	var data gtables.Permission
	data.InitWithDBType(nil, "", database.DBTypeSQLite)
	data.Table_name = types.NewCode("CUSTOMER")
	if err := data.ValidateField("object_id", 0); err != nil || data.Table_name != "CUSTOMER" {
		t.Errorf("table data row key changed to %q (%v)", data.Table_name, err)
	}
}
