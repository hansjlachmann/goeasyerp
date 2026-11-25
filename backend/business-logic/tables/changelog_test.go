package tables

import (
	"database/sql"
	"errors"
	"testing"

	fcodeunits "github.com/hansjlachmann/goeasyerp/backend/foundation/codeunits"
	"github.com/hansjlachmann/goeasyerp/backend/foundation/database"
	apperrors "github.com/hansjlachmann/goeasyerp/backend/foundation/errors"
	"github.com/hansjlachmann/goeasyerp/backend/foundation/session"
	ftables "github.com/hansjlachmann/goeasyerp/backend/foundation/tables"
	"github.com/hansjlachmann/goeasyerp/backend/foundation/types"
	gtables "github.com/hansjlachmann/goeasyerp/backend/generated/tables"
	_ "github.com/mattn/go-sqlite3"
)

// changeLogDB: a database with the change log's tables, User, and Payment Terms in company main,
// and the change log registered on it
func changeLogDB(t *testing.T) (*sql.DB, *changeLog) {
	t.Helper()
	db, err := sql.Open("sqlite3", "file:"+t.Name()+"?mode=memory&cache=shared")
	if err != nil {
		t.Fatal(err)
	}
	db.SetMaxOpenConns(1)
	t.Cleanup(func() { _ = db.Close() })
	sqlite := database.DBTypeSQLite
	for _, table := range []interface {
		CreateTableWithDBType(db database.Executor, company string, dbType database.DBType) error
	}{&gtables.ChangeLogSetup{}, &gtables.ChangeLogSetupTable{}, &gtables.ChangeLogSetupField{}, &gtables.ChangeLogEntry{}, &gtables.User{}} {
		if err := table.CreateTableWithDBType(db, "", sqlite); err != nil {
			t.Fatal(err)
		}
	}
	if err := (&gtables.PaymentTerms{}).CreateTableWithDBType(db, "main", sqlite); err != nil {
		t.Fatal(err)
	}
	stop := RegisterChangeLog(db, sqlite)
	t.Cleanup(stop)
	changeLogMu.Lock()
	c := current
	changeLogMu.Unlock()
	fcodeunits.SetCurrentContext("KARI", "Kari", "main", "en-US")
	t.Cleanup(fcodeunits.ClearCurrentContext)
	return db, c
}

// changeLogEntries: the entries written so far, in order
func changeLogEntries(t *testing.T, db *sql.DB) []gtables.ChangeLogEntry {
	t.Helper()
	entry := &gtables.ChangeLogEntry{}
	entry.InitWithDBType(db, "", database.DBTypeSQLite)
	var got []gtables.ChangeLogEntry
	for found := entry.FindSet(); found; found = entry.Next() {
		got = append(got, *entry)
	}
	return got
}

// The built-in tables are recorded field by field — who, the company, the key, before and
// after —, a password only as changed, the fields that change at every login not at all
func TestChangeLogRecordsDefaultTables(t *testing.T) {
	db, _ := changeLogDB(t)
	sqlite := database.DBTypeSQLite

	user := &gtables.User{}
	user.InitWithDBType(db, "", sqlite)
	user.User_id = types.NewCode("ole")
	user.User_name = "Ole"
	user.Active = true
	if err := SetPassword(user, "long-secret-for-him"); err != nil {
		t.Fatal(err)
	}
	if !user.Insert(false) {
		t.Fatal(user.TriggerError())
	}
	inserted := changeLogEntries(t, db)
	byField := map[string]gtables.ChangeLogEntry{}
	for _, e := range inserted {
		if e.Type_of_change != gtables.ChangeLogEntry_Type_of_change.Insertion || e.User_id != "KARI" ||
			e.Primary_key != "OLE" || e.Table_no != gtables.UserTableID || e.Company != "" {
			t.Errorf("insertion entry: %+v", e)
		}
		byField[e.Field_name.String()] = e
	}
	if len(byField) < 3 || byField["User Name"].New_value != "Ole" {
		t.Errorf("the insertion's fields: %+v", byField)
	}
	for _, e := range inserted {
		if e.Old_value != "" || (e.Field_no == fieldNo(gtables.UserTableID, "password_hash") && e.New_value != "(changed)") {
			t.Errorf("a value written out: %+v", e)
		}
	}

	// a modification: only the changed field; the login fields not at all
	n := len(inserted)
	user.User_name = "Ole Hansen"
	UpdateLastLogin(user)
	if !user.Modify(false) {
		t.Fatal(user.TriggerError())
	}
	modified := changeLogEntries(t, db)[n:]
	if len(modified) != 1 || modified[0].Field_name != "User Name" || modified[0].Old_value != "Ole" || modified[0].New_value != "Ole Hansen" ||
		modified[0].Type_of_change != gtables.ChangeLogEntry_Type_of_change.Modification || modified[0].Entry_no != n+1 {
		t.Errorf("the modification: %+v", modified)
	}

	// a deletion: the values the record had
	n += len(modified)
	if !user.Delete(false) {
		t.Fatal("delete failed")
	}
	deleted := changeLogEntries(t, db)[n:]
	if len(deleted) == 0 {
		t.Fatal("no deletion entries")
	}
	for _, e := range deleted {
		if e.Type_of_change != gtables.ChangeLogEntry_Type_of_change.Deletion || e.New_value != "" && e.New_value != "(changed)" {
			t.Errorf("deletion entry: %+v", e)
		}
	}

	// a table outside the list is not recorded
	n = len(changeLogEntries(t, db))
	paymentTerms := &gtables.PaymentTerms{}
	paymentTerms.InitWithDBType(db, "main", sqlite)
	paymentTerms.Code = "30D"
	if !paymentTerms.Insert(false) {
		t.Fatal(paymentTerms.TriggerError())
	}
	if got := len(changeLogEntries(t, db)); got != n {
		t.Errorf("Payment Terms recorded without a setup line: %d entries", got-n)
	}
}

// A setup line records the table: Some Fields only the marked fields; the change log switched
// off records nothing; a temporary record and a rolled back change leave no entry
func TestChangeLogSetup(t *testing.T) {
	db, c := changeLogDB(t)
	sqlite := database.DBTypeSQLite

	line := &gtables.ChangeLogSetupTable{}
	line.InitWithDBType(db, "", sqlite)
	line.Table_no = gtables.PaymentTermsTableID
	line.Log_insertion = gtables.ChangeLogSetupTable_Log_insertion.AllFields
	line.Log_modification = gtables.ChangeLogSetupTable_Log_modification.SomeFields
	if !line.Insert(true) {
		t.Fatal(line.TriggerError())
	}
	field := &gtables.ChangeLogSetupField{}
	field.InitWithDBType(db, "", sqlite)
	field.Table_no = gtables.PaymentTermsTableID
	field.Field_no = fieldNo(gtables.PaymentTermsTableID, "description")
	if err := field.Validate("field_no", field.Field_no); err != nil {
		t.Fatal(err)
	}
	field.Log_modification = true
	if !field.Insert(true) {
		t.Fatal(field.TriggerError())
	}
	if field.Field_name != "description" {
		t.Errorf("field name %q", field.Field_name)
	}
	if err := c.reload(); err != nil {
		t.Fatal(err)
	}
	n := len(changeLogEntries(t, db)) // the setup's own changes are recorded too

	paymentTerms := &gtables.PaymentTerms{}
	paymentTerms.InitWithDBType(db, "main", sqlite)
	paymentTerms.Code = "14D"
	paymentTerms.Description = "14 days"
	if !paymentTerms.Insert(false) {
		t.Fatal(paymentTerms.TriggerError())
	}
	inserted := changeLogEntries(t, db)[n:]
	values := map[string]string{}
	for _, e := range inserted {
		values[e.Field_name.String()] = e.New_value.String()
		if e.Company != "main" || e.Primary_key != "14D" {
			t.Errorf("insertion entry: %+v", e)
		}
	}
	if values["Code"] != "14D" || values["Description"] != "14 days" || values["Active"] != "Yes" {
		t.Fatalf("insertion with all fields (the defaults too): %v", values)
	}
	n += len(inserted)
	paymentTerms.Description = "Fourteen days"
	paymentTerms.Due_date_calculation = "14D"
	if !paymentTerms.Modify(false) {
		t.Fatal(paymentTerms.TriggerError())
	}
	modified := changeLogEntries(t, db)[n:]
	if len(modified) != 1 || modified[0].New_value != "Fourteen days" {
		t.Errorf("Some Fields: %+v", modified)
	}
	n += len(modified)
	if !paymentTerms.Delete(false) {
		t.Fatal("delete failed")
	}
	if got := len(changeLogEntries(t, db)); got != n {
		t.Errorf("deletion recorded with Log Deletion blank: %d", got-n)
	}

	// a temporary record is never recorded
	temporary := &gtables.PaymentTerms{}
	if err := temporary.InitTemporary(db, "main", sqlite); err != nil {
		t.Fatal(err)
	}
	temporary.Code = "TEMP"
	temporary.Insert(false)
	if got := len(changeLogEntries(t, db)); got != n {
		t.Errorf("a temporary record recorded: %d", got-n)
	}

	// a rolled back change leaves no entry
	tx, err := ftables.BeginTransaction(db)
	if err != nil {
		t.Fatal(err)
	}
	inTx := &gtables.PaymentTerms{}
	inTx.InitWithDBType(tx, "main", sqlite)
	inTx.Code = "60D"
	inTx.Insert(false)
	_ = ftables.Rollback(tx)
	if got := len(changeLogEntries(t, db)); got != n {
		t.Errorf("a rolled back insert recorded: %d", got-n)
	}

	// switched off: nothing
	changeLogSetup := gtables.GetChangeLogSetup(ftables.DBSource(db, "", sqlite))
	changeLogSetup.Change_log_activated = false
	if !changeLogSetup.Insert(true) {
		t.Fatal(changeLogSetup.TriggerError())
	}
	if err := c.reload(); err != nil {
		t.Fatal(err)
	}
	n = len(changeLogEntries(t, db))
	paymentTerms.Code = "7D"
	paymentTerms.Insert(false)
	if got := len(changeLogEntries(t, db)); got != n {
		t.Errorf("recorded while switched off: %d", got-n)
	}
}

// Only SUPER users change what the change log records
func TestChangeLogSetupSuperOnly(t *testing.T) {
	db, _ := changeLogDB(t)
	fcodeunits.SetCurrentAccess(&session.Session{UserID: "CLERK", HasRoles: true})
	line := &gtables.ChangeLogSetupTable{}
	line.InitWithDBType(db, "", database.DBTypeSQLite)
	line.Table_no = gtables.PaymentTermsTableID
	if line.Insert(true) {
		t.Fatal("a user without SUPER set up the change log")
	}
	var appErr *apperrors.AppError
	if !errors.As(line.TriggerError(), &appErr) || appErr.Code != apperrors.ErrChangeLogSuperOnly {
		t.Errorf("error %v", line.TriggerError())
	}
	fcodeunits.SetCurrentAccess(&session.Session{UserID: "ADMIN", HasRoles: true, IsSuper: true})
	if !line.Insert(true) {
		t.Errorf("a SUPER user: %v", line.TriggerError())
	}
}
