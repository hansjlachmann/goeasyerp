package migrations

import (
	"database/sql"
	"testing"

	_ "github.com/mattn/go-sqlite3"

	"github.com/hansjlachmann/goeasyerp/backend/foundation/database"
	fmigrations "github.com/hansjlachmann/goeasyerp/backend/foundation/migrations"
)

func TestRenumberObjects(t *testing.T) {
	db, err := sql.Open("sqlite3", ":memory:")
	if err != nil {
		t.Fatal(err)
	}
	db.SetMaxOpenConns(1)
	t.Cleanup(func() { _ = db.Close() })
	for _, stmt := range []string{
		`CREATE TABLE "c1$Job_Queue" (no VARCHAR(20) PRIMARY KEY, object_id_to_run INTEGER)`,
		`INSERT INTO "c1$Job_Queue" VALUES ('DEMO', 50100), ('MAIL', 455), ('SIFT', 50110)`,
		`CREATE TABLE "User_Preferences" (user_id VARCHAR(50), page_id INTEGER, preference_type VARCHAR(20), preference_name VARCHAR(50), PRIMARY KEY (user_id, page_id, preference_type, preference_name))`,
		// page 10 (Menus) becomes 900 and page 12 (Countries) becomes 10: the same key twice
		`INSERT INTO "User_Preferences" VALUES ('HANS', 10, 'COLUMNS', 'DEFAULT'), ('HANS', 12, 'COLUMNS', 'DEFAULT'), ('HANS', 22, 'COLUMNS', 'DEFAULT')`,
	} {
		if _, err := db.Exec(stmt); err != nil {
			t.Fatal(err)
		}
	}
	tx, err := db.Begin()
	if err != nil {
		t.Fatal(err)
	}
	// c2 has no Job_Queue table yet
	ctx := &fmigrations.Context{DB: db, DBType: database.DBTypeSQLite, Companies: []string{"c1", "c2"}}
	ctx.SetTx(tx)
	if err := (&Migration010RenumberObjects{}).Up(ctx); err != nil {
		t.Fatal(err)
	}
	if err := tx.Commit(); err != nil {
		t.Fatal(err)
	}

	query := func(q string) map[string]int {
		rows, err := db.Query(q)
		if err != nil {
			t.Fatal(err)
		}
		defer func() { _ = rows.Close() }()
		got := map[string]int{}
		for rows.Next() {
			var k string
			var v int
			if err := rows.Scan(&k, &v); err != nil {
				t.Fatal(err)
			}
			got[k] = v
		}
		return got
	}
	jobs := query(`SELECT no, object_id_to_run FROM "c1$Job_Queue"`)
	if jobs["DEMO"] != 901 || jobs["SIFT"] != 902 || jobs["MAIL"] != 455 {
		t.Errorf("job queue %v", jobs)
	}
	prefs := query(`SELECT CAST(page_id AS TEXT), page_id FROM "User_Preferences"`)
	if len(prefs) != 3 || prefs["900"] != 900 || prefs["10"] != 10 || prefs["22"] != 22 {
		t.Errorf("preferences %v, want pages 900, 10 and 22", prefs)
	}
}
