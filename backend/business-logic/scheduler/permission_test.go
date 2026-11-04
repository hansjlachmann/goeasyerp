package scheduler

import (
	"strings"
	"testing"

	"github.com/hansjlachmann/goeasyerp/backend/business-logic/codeunits"
	fcodeunits "github.com/hansjlachmann/goeasyerp/backend/foundation/codeunits"
	"github.com/hansjlachmann/goeasyerp/backend/foundation/database"
	"github.com/hansjlachmann/goeasyerp/backend/foundation/system"
	"github.com/hansjlachmann/goeasyerp/backend/foundation/types"
	gtables "github.com/hansjlachmann/goeasyerp/backend/generated/tables"
)

// userCodeunit records the user each run sees (USERID)
type userCodeunit struct{ seen *[]string }

func (userCodeunit) ID() int             { return 99992 }
func (userCodeunit) Name() string        { return "scheduler-user-test" }
func (userCodeunit) SourceTable() string { return "" }
func (userCodeunit) UsesProgress() bool  { return false }
func (c userCodeunit) Run(interface{}) (fcodeunits.Result, error) {
	*c.seen = append(*c.seen, system.UserId())
	return fcodeunits.Result{Success: true}, nil
}

// A scheduled job runs as its user and checks that user's Execute permission at every run: a
// user without it gets the job set to Error; a job without a user (saved before jobs had one)
// runs as SCHEDULER
func TestRunJobChecksTheJobUser(t *testing.T) {
	db := newTestDB(t)
	db.SetMaxOpenConns(1)
	var seen []string
	codeunits.Register(99992, "scheduler-user-test", func(database.Executor, string, database.DBType) fcodeunits.Codeunit {
		return userCodeunit{&seen}
	})
	if err := (&gtables.UserMember{}).CreateTableWithDBType(db, "", database.DBTypeSQLite); err != nil {
		t.Fatal(err)
	}
	if err := (&gtables.Permission{}).CreateTableWithDBType(db, "", database.DBTypeSQLite); err != nil {
		t.Fatal(err)
	}
	for _, user := range []string{"CLERK1", "CLERK2"} {
		member := &gtables.UserMember{}
		member.InitWithDBType(db, "", database.DBTypeSQLite)
		member.User_id, member.Role_id = types.NewCode(user), types.NewCode(user+"-ROLE")
		if !member.Insert(false) {
			t.Fatalf("insert member %s", user)
		}
	}
	perm := &gtables.Permission{}
	perm.InitWithDBType(db, "", database.DBTypeSQLite)
	perm.Role_id = types.NewCode("CLERK1-ROLE")
	perm.Object_type = gtables.Permission_Object_type.Codeunit
	perm.Object_id = 99992
	perm.Can_execute = true
	if !perm.Insert(true) {
		t.Fatalf("insert permission: %v", perm.TriggerError())
	}

	s := New(db, database.DBTypeSQLite, func() ([]string, error) { return []string{testCompany}, nil })
	for _, tc := range []struct {
		job, user string
		wantRun   string
		wantError bool
	}{
		{"ALLOWED", "CLERK1", "CLERK1", false},
		{"DENIED", "CLERK2", "", true},
		{"LEGACY", "", "SCHEDULER", false},
	} {
		seedJob(t, db, tc.job, gtables.JobQueue_Recurrence.Hourly, 0)
		job := getJob(t, db, tc.job)
		job.Object_id_to_run = 99992
		job.User_id = types.NewCode(tc.user)
		if !job.Modify(false) {
			t.Fatalf("%s: modify", tc.job)
		}
		seen = nil
		s.runJob(testCompany, &job)
		ran := strings.Join(seen, ",")
		if ran != tc.wantRun {
			t.Errorf("%s: ran as %q, want %q", tc.job, ran, tc.wantRun)
		}
		after := getJob(t, db, tc.job)
		if (after.Status == gtables.JobQueue_Status.Error) != tc.wantError {
			t.Errorf("%s: status %v, want error=%v", tc.job, after.Status, tc.wantError)
		}
		if after.User_id.String() != tc.user {
			t.Errorf("%s: rescheduling changed the user to %q", tc.job, after.User_id)
		}
	}
}
