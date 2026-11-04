package tables

import (
	"database/sql"
	gtables "github.com/hansjlachmann/goeasyerp/backend/generated/tables"
	"testing"

	"github.com/hansjlachmann/goeasyerp/backend/foundation/database"
	"github.com/hansjlachmann/goeasyerp/backend/foundation/types"
)

// The list search is case-insensitive for non-ASCII letters on SQLite too (the application's
// SQLite driver has a Unicode lower(); the built-in one only folds A–Z).
func TestSearchIsCaseInsensitiveForNonASCII(t *testing.T) {
	db, err := sql.Open(database.SQLiteDriver, ":memory:")
	if err != nil {
		t.Fatal(err)
	}
	db.SetMaxOpenConns(1)
	t.Cleanup(func() { _ = db.Close() })
	if err := (&gtables.Customer{}).CreateTableWithDBType(db, "test", database.DBTypeSQLite); err != nil {
		t.Fatal(err)
	}
	for no, name := range map[string]string{"C1": "ØRSTA SYKKEL", "C2": "Ærlig Åse AS", "C3": "Oslo"} {
		var c gtables.Customer
		c.InitWithDBType(db, "test", database.DBTypeSQLite)
		c.No, c.Name = types.NewCode(no), types.NewText(name)
		if !c.Insert(false) {
			t.Fatalf("insert %s", no)
		}
	}
	for search, want := range map[string]int{"ørsta": 1, "ØRSTA": 1, "ærlig åse": 1, "ÅSE": 1, "oslo": 1, "s": 3} {
		var c gtables.Customer
		c.InitWithDBType(db, "test", database.DBTypeSQLite)
		c.SetSearch([]string{"name"}, search)
		if got := c.Count(); got != want {
			t.Errorf("search %q: %d customers, want %d", search, got, want)
		}
	}

	var lowered string
	var null sql.NullString
	if err := db.QueryRow(`SELECT lower('ÆØÅ Straße'), lower(NULL)`).Scan(&lowered, &null); err != nil || lowered != "æøå straße" || null.Valid {
		t.Errorf("lower = %q, %v (%v); want æøå straße and NULL", lowered, null, err)
	}
}
