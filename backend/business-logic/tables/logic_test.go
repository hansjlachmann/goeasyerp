package tables

import (
	"errors"
	"strings"
	"testing"

	"github.com/hansjlachmann/goeasyerp/backend/foundation/database"
	"github.com/hansjlachmann/goeasyerp/backend/foundation/types"
	gtables "github.com/hansjlachmann/goeasyerp/backend/generated/tables"
)

type misspelledTrigger struct{}

func (misspelledTrigger) OnValidate_Nmae(rec *gtables.Customer) error { return nil }

type wrongSignature struct{}

func (wrongSignature) OnInsert() error { return nil }

type helperOnly struct{}

func (helperOnly) Describe(rec *gtables.Customer) string { return rec.No.String() }

func panicMessage(f func()) (msg string) {
	defer func() {
		if r := recover(); r != nil {
			msg = r.(string)
		}
	}()
	f()
	return ""
}

// A trigger the table does not have would never run: registering it stops the program,
// naming the method (as a compile error would).
func TestRegisterRejectsUnknownTriggers(t *testing.T) {
	msg := panicMessage(func() { gtables.RegisterCustomerLogic(misspelledTrigger{}) })
	if !strings.Contains(msg, "OnValidate_Nmae is not a trigger of table Customer") {
		t.Errorf("misspelled trigger: panic %q", msg)
	}
	msg = panicMessage(func() { gtables.RegisterCustomerLogic(wrongSignature{}) })
	if !strings.Contains(msg, "OnInsert of table Customer must be func(rec *Customer) error") {
		t.Errorf("wrong signature: panic %q", msg)
	}
	// Methods that are not named like triggers are the logic's own helpers
	var c gtables.Customer
	if msg := panicMessage(func() { c.SetLogic(helperOnly{}) }); msg != "" {
		t.Errorf("helper method: panic %q", msg)
	}
}

type orderLogic struct {
	name string
	log  *[]string
	fail bool
}

func (l orderLogic) OnInsert(rec *gtables.Customer) error {
	*l.log = append(*l.log, l.name)
	if l.fail {
		return errors.New(l.name + " refused")
	}
	return nil
}

func (l orderLogic) OnValidate_City(rec *gtables.Customer) error {
	*l.log = append(*l.log, l.name+":city")
	return nil
}

// Every registered logic runs, in registration order (the base application, then
// extensions); the first error stops the operation.
func TestLogicRunsInOrderAndStopsAtFirstError(t *testing.T) {
	db := newTestDB(t)
	if err := (&gtables.Customer{}).CreateTableWithDBType(db, "TEST", database.DBTypeSQLite); err != nil {
		t.Fatalf("create table: %v", err)
	}

	setupCustomerNos(t, db, "TEST", true)
	var log []string
	c := &gtables.Customer{}
	c.InitWithDBType(db, "TEST", database.DBTypeSQLite)
	c.SetLogic(customerLogic{}, orderLogic{name: "base", log: &log}, orderLogic{name: "extension", log: &log})
	if err := c.ValidateField("city", "Oslo"); err != nil {
		t.Fatal(err)
	}
	if !c.Insert(true) {
		t.Fatalf("Insert: %v", c.TriggerError())
	}
	if got := strings.Join(log, ","); got != "base:city,extension:city,base,extension" {
		t.Errorf("triggers ran %q", got)
	}
	if c.No != "C00001" {
		t.Errorf("no %q: the Customer OnInsert did not number the customer", c.No)
	}

	log = nil
	c.SetLogic(orderLogic{name: "base", log: &log, fail: true}, orderLogic{name: "extension", log: &log})
	c.No = types.NewCode("C2")
	if c.Insert(true) {
		t.Fatal("Insert succeeded, want the first OnInsert to refuse")
	}
	if got := strings.Join(log, ","); got != "base" {
		t.Errorf("triggers ran %q, want only base", got)
	}
	if err := c.TriggerError(); err == nil || err.Error() != "base refused" {
		t.Errorf("TriggerError() = %v", err)
	}
}

// Without runTrigger no trigger and no field check runs (Insert(false))
func TestNoTriggersWithoutRunTrigger(t *testing.T) {
	db := newTestDB(t)
	if err := (&gtables.Customer{}).CreateTableWithDBType(db, "TEST", database.DBTypeSQLite); err != nil {
		t.Fatalf("create table: %v", err)
	}
	var log []string
	c := &gtables.Customer{}
	c.InitWithDBType(db, "TEST", database.DBTypeSQLite)
	c.SetLogic(orderLogic{name: "base", log: &log, fail: true})
	c.No = types.NewCode("C1")
	if !c.Insert(false) {
		t.Fatal("Insert(false) failed")
	}
	if len(log) != 0 {
		t.Errorf("triggers ran %v", log)
	}
}
