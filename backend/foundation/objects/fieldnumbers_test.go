package objects

import (
	"database/sql"
	"fmt"
	"strings"
	"testing"

	_ "github.com/mattn/go-sqlite3"

	"github.com/hansjlachmann/goeasyerp/backend/foundation/database"
	"github.com/hansjlachmann/goeasyerp/backend/foundation/tables"
)

// numberedTable is a company table with numbered fields, as a version of its YAML would give
type numberedTable struct {
	fields []tables.FieldInfo // stored fields in order: column TEXT each
}

func (t *numberedTable) GetTableID() int      { return 18 }
func (t *numberedTable) GetTableName() string { return "Customer" }
func (t *numberedTable) GetTableSchema() string {
	cols := make([]string, len(t.fields))
	for i, f := range t.fields {
		cols[i] = f.Name + " TEXT"
		if i == 0 {
			cols[i] += " PRIMARY KEY"
		}
	}
	return "(\n" + strings.Join(cols, ",\n") + "\n)"
}
func (t *numberedTable) GetPostgresTableSchema() string { return t.GetTableSchema() }
func (t *numberedTable) CreateTable(db database.Executor, company string) error {
	return t.CreateTableWithDBType(db, company, database.DBTypeSQLite)
}
func (t *numberedTable) CreateTableWithDBType(db database.Executor, company string, _ database.DBType) error {
	_, err := db.Exec(fmt.Sprintf(`CREATE TABLE "%s$Customer" %s`, company, t.GetTableSchema()))
	return err
}
func (t *numberedTable) GetFields() []tables.FieldInfo { return t.fields }

func syncVersion(t *testing.T, db *sql.DB, fields ...tables.FieldInfo) error {
	t.Helper()
	r := NewObjectRegistry()
	if err := r.RegisterTable(18, &numberedTable{fields: fields}); err != nil {
		t.Fatal(err)
	}
	return r.InitializeCompanyTablesWithDBType(db, "c1", database.DBTypeSQLite)
}

func TestFieldNumbersRenameAndGuard(t *testing.T) {
	db, err := sql.Open("sqlite3", ":memory:")
	if err != nil {
		t.Fatal(err)
	}
	db.SetMaxOpenConns(1)
	t.Cleanup(func() { _ = db.Close() })

	no := tables.FieldInfo{No: 1, Name: "no"}
	phone := tables.FieldInfo{No: 9, Name: "phonenumber"}
	balance := tables.FieldInfo{No: 59, Name: "balance", FlowField: true} // not a column
	if err := syncVersion(t, db, no, phone, balance); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(`INSERT INTO "c1$Customer" (no, phonenumber) VALUES ('C1', '555-1234')`); err != nil {
		t.Fatal(err)
	}

	// Field 9 renamed in the YAML: the column is renamed and keeps the data
	if err := syncVersion(t, db, no, tables.FieldInfo{No: 9, Name: "phone_no"}, balance); err != nil {
		t.Fatal(err)
	}
	var phoneNo string
	if err := db.QueryRow(`SELECT phone_no FROM "c1$Customer" WHERE no = 'C1'`).Scan(&phoneNo); err != nil || phoneNo != "555-1234" {
		t.Fatalf("phone_no = %q, %v; want the data of the renamed column", phoneNo, err)
	}
	if _, err := db.Exec(`SELECT phonenumber FROM "c1$Customer"`); err == nil {
		t.Error("the old column still exists (added again)")
	}

	// A field whose number changed stops the sync
	err = syncVersion(t, db, no, tables.FieldInfo{No: 10, Name: "phone_no"})
	if err == nil || !strings.Contains(err.Error(), "phone_no: 9 → 10") {
		t.Errorf("changed number: %v", err)
	}

	// A new field is added as before and recorded
	if err := syncVersion(t, db, no, tables.FieldInfo{No: 9, Name: "phone_no"}, tables.FieldInfo{No: 102, Name: "e_mail"}); err != nil {
		t.Fatal(err)
	}
	known, err := knownFieldNumbers(db, database.DBTypeSQLite, "c1$Customer")
	if err != nil || known[9] != "phone_no" || known[102] != "e_mail" || known[59] != "" {
		t.Errorf("recorded %v, %v", known, err)
	}
}
