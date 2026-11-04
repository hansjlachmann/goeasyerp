package tables

import (
	gtables "github.com/hansjlachmann/goeasyerp/backend/generated/tables"
	"testing"

	"github.com/hansjlachmann/goeasyerp/backend/foundation/database"
	"github.com/hansjlachmann/goeasyerp/backend/foundation/types"
)

// Field names end up in SQL text (WHERE / ORDER BY / SET); only real columns may be used.

const injection = "no = no) OR 1=1 --"

func seedCustomers(t *testing.T) *gtables.Customer {
	t.Helper()
	db := newTestDB(t)
	if err := (&gtables.Customer{}).CreateTableWithDBType(db, "TEST", database.DBTypeSQLite); err != nil {
		t.Fatalf("create table: %v", err)
	}
	for _, no := range []string{"C1", "C2", "C3"} {
		c := &gtables.Customer{}
		c.InitWithDBType(db, "TEST", database.DBTypeSQLite)
		c.No, c.Name = types.NewCode(no), types.NewText("Name "+no)
		if !c.Insert(false) {
			t.Fatalf("insert %s failed", no)
		}
	}
	c := &gtables.Customer{}
	c.InitWithDBType(db, "TEST", database.DBTypeSQLite)
	return c
}

func TestHasColumn(t *testing.T) {
	c := &gtables.Customer{}
	for name, want := range map[string]bool{
		"no":                 true,
		"NO":                 true, // case-insensitive
		"payment_terms_code": true,
		"balance_lcy":        false, // FlowField: not a stored column
		"bogus":              false,
		injection:            false,
		"":                   false,
	} {
		if got := c.HasColumn(name); got != want {
			t.Errorf("HasColumn(%q) = %v, want %v", name, got, want)
		}
	}
}

func TestSetFilterUnknownFieldFailsClosed(t *testing.T) {
	c := seedCustomers(t)
	c.SetFilter(injection, "x")
	if n := c.Count(); n != 0 {
		t.Errorf("Count() with injected filter field = %d, want 0 (fail closed)", n)
	}
}

func TestSetRangeUnknownFieldFailsClosed(t *testing.T) {
	c := seedCustomers(t)
	c.SetRange(injection, "x")
	if n := c.Count(); n != 0 {
		t.Errorf("Count() with injected range field = %d, want 0 (fail closed)", n)
	}
}

func TestDeleteAllWithUnknownFilterDeletesNothing(t *testing.T) {
	c := seedCustomers(t)
	c.SetFilter("bogus", "C1")
	if n := c.DeleteAll(); n != 0 {
		t.Errorf("DeleteAll() with unknown filter field deleted %d rows, want 0", n)
	}
	c.ClearFilters()
	if n := c.Count(); n != 3 {
		t.Errorf("Count() after DeleteAll = %d, want 3", n)
	}
}

func TestSetFilterKnownFieldIsCaseInsensitive(t *testing.T) {
	c := seedCustomers(t)
	c.SetFilter("NO", "C2")
	if n := c.Count(); n != 1 {
		t.Errorf("Count() with filter NO=C2 = %d, want 1", n)
	}
}

func TestSetCurrentKeyIgnoresUnknownField(t *testing.T) {
	c := seedCustomers(t)
	c.SetCurrentKey(injection)
	var got []string
	if c.FindSet() {
		got = append(got, c.No.String())
		for c.Next() {
			got = append(got, c.No.String())
		}
	}
	if len(got) != 3 || got[0] != "C1" {
		t.Errorf("FindSet with injected sort key = %v, want all 3 rows in primary key order", got)
	}
}

func TestModifyAllUnknownFieldDoesNothing(t *testing.T) {
	c := seedCustomers(t)
	if n := c.ModifyAll("name = 'x', city", "Oslo"); n != 0 {
		t.Errorf("ModifyAll on injected field modified %d rows, want 0", n)
	}
	if n := c.ModifyAll("City", "Oslo"); n != 3 {
		t.Errorf("ModifyAll(City) modified %d rows, want 3", n)
	}
}
