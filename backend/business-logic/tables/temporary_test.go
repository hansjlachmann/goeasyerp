package tables

import (
	"errors"
	"testing"

	"github.com/hansjlachmann/goeasyerp/backend/foundation/database"
	apperrors "github.com/hansjlachmann/goeasyerp/backend/foundation/errors"
	ftables "github.com/hansjlachmann/goeasyerp/backend/foundation/tables"
	"github.com/hansjlachmann/goeasyerp/backend/foundation/types"
	gtables "github.com/hansjlachmann/goeasyerp/backend/generated/tables"
)

// A temporary record keeps its records in memory: nothing reaches the database
func TestTemporaryRecord(t *testing.T) {
	db := newTestDB(t)
	if err := (&gtables.Customer{}).CreateTableWithDBType(db, "c1", database.DBTypeSQLite); err != nil {
		t.Fatal(err)
	}
	var real gtables.Customer
	real.InitWithDBType(db, "c1", database.DBTypeSQLite)
	real.No, real.Name = "R1", "Real one"
	if !real.Insert(false) {
		t.Fatal("insert real")
	}

	var temp gtables.Customer
	if err := temp.InitTemporary(db, "c1", database.DBTypeSQLite); err != nil {
		t.Fatal(err)
	}
	if !temp.IsTemporary() || real.IsTemporary() {
		t.Error("IsTemporary")
	}
	for _, c := range []struct{ no, name, city string }{{"T1", "Anna", "Oslo"}, {"T2", "Bjørn", "Bergen"}, {"T3", "Carl", "Oslo"}} {
		temp.Init()
		temp.No, temp.Name, temp.City = types.NewCode(c.no), types.NewText(c.name), types.NewText(c.city)
		if !temp.Insert(true) { // triggers run
			t.Fatalf("insert %s: %v", c.no, temp.TriggerError())
		}
	}
	if temp.Count() != 3 || real.Count() != 1 {
		t.Errorf("temporary %d, real %d", temp.Count(), real.Count())
	}
	if !temp.Get("T2") || temp.Name != "Bjørn" || real.Get("T2") {
		t.Errorf("Get T2: %+v", temp.ToMap())
	}

	// Modify while looping (the in-memory database has one connection)
	temp.SetRange("city", "Oslo")
	n := 0
	for ok := temp.FindSet(); ok; ok = temp.Next() {
		temp.Name = types.NewText(string(temp.Name) + " (Oslo)")
		if !temp.Modify(false) {
			t.Fatalf("modify %s", temp.No)
		}
		n++
	}
	if n != 2 || !temp.Get("T3") || temp.Name != "Carl (Oslo)" {
		t.Errorf("modified %d, T3 %q", n, temp.Name)
	}
	if temp.DeleteAll() != 2 || temp.Count() != 0 {
		t.Error("DeleteAll on the temporary record")
	}
	temp.Reset()
	if temp.Count() != 1 || real.Count() != 1 {
		t.Errorf("after DeleteAll: temporary %d, real %d", temp.Count(), real.Count())
	}

	// Each temporary record has its own records
	var other gtables.Customer
	if err := other.InitTemporary(db, "c1", database.DBTypeSQLite); err != nil {
		t.Fatal(err)
	}
	if other.Count() != 0 {
		t.Errorf("another temporary record sees %d records", other.Count())
	}
}

// Consistent(false): the transaction cannot be committed — it is rolled back
func TestConsistent(t *testing.T) {
	db := newTestDB(t)
	if err := (&gtables.CustomerLedgerEntry{}).CreateTableWithDBType(db, "c1", database.DBTypeSQLite); err != nil {
		t.Fatal(err)
	}
	post := func(no int, balanced bool) error {
		tx, err := db.Begin()
		if err != nil {
			t.Fatal(err)
		}
		var e gtables.CustomerLedgerEntry
		e.InitWithDBType(tx, "c1", database.DBTypeSQLite)
		e.Entry_no, e.Amount_lcy = no, types.MustDecimal("100")
		e.Consistent(false) // not balanced yet
		if !e.Insert(false) {
			t.Fatal("insert")
		}
		if balanced {
			e.Consistent(true)
		}
		return ftables.Commit(tx)
	}
	err := post(1, false)
	var appErr *apperrors.AppError
	if !errors.As(err, &appErr) || appErr.Code != apperrors.ErrInconsistent || appErr.Params[0] != gtables.CustomerLedgerEntryTableName {
		t.Errorf("inconsistent commit: %v", err)
	}
	if err := post(2, true); err != nil {
		t.Errorf("consistent commit: %v", err)
	}
	var e gtables.CustomerLedgerEntry
	e.InitWithDBType(db, "c1", database.DBTypeSQLite)
	if e.Count() != 1 || !e.Find("-") || e.Entry_no != 2 {
		t.Errorf("entries after the commits: %d (want only entry 2)", e.Count())
	}
}
