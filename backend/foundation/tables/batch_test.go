package tables

import (
	"database/sql"
	"errors"
	"testing"

	_ "github.com/mattn/go-sqlite3"

	"github.com/hansjlachmann/goeasyerp/backend/foundation/database"
)

func TestInsertRows(t *testing.T) {
	db, err := sql.Open("sqlite3", ":memory:")
	if err != nil {
		t.Fatal(err)
	}
	db.SetMaxOpenConns(1)
	t.Cleanup(func() { _ = db.Close() })
	if _, err := db.Exec(`CREATE TABLE "c$T" (no INTEGER PRIMARY KEY, name TEXT)`); err != nil {
		t.Fatal(err)
	}

	// 2,500 rows: three statements (at most maxBatchRows rows each)
	var rows [][]interface{}
	for i := 1; i <= 2500; i++ {
		rows = append(rows, []interface{}{i, "x"})
	}
	if err := InsertRows(db, database.DBTypeSQLite, "c$T", []string{"no", "name"}, rows); err != nil {
		t.Fatal(err)
	}
	var n int
	if err := db.QueryRow(`SELECT COUNT(*) FROM "c$T"`).Scan(&n); err != nil || n != 2500 {
		t.Fatalf("%d rows (%v), want 2500", n, err)
	}

	// A failing statement reports the index of its first row; earlier statements stay written
	rows = nil
	for i := 3001; i <= 4200; i++ {
		rows = append(rows, []interface{}{i, "y"})
	}
	rows[1100][0] = 1 // duplicate key in the second statement
	err = InsertRows(db, database.DBTypeSQLite, "c$T", []string{"no", "name"}, rows)
	var batchErr *BatchInsertError
	if !errors.As(err, &batchErr) || batchErr.Index != 1000 || batchErr.Trigger {
		t.Fatalf("error %v, want a database BatchInsertError at index 1000", err)
	}
	if err := db.QueryRow(`SELECT COUNT(*) FROM "c$T"`).Scan(&n); err != nil || n != 3500 {
		t.Fatalf("%d rows (%v), want 3500 (first statement written)", n, err)
	}

	// A row with the wrong number of values
	err = InsertRows(db, database.DBTypeSQLite, "c$T", []string{"no", "name"}, [][]interface{}{{5000, "z"}, {5001}})
	if !errors.As(err, &batchErr) || batchErr.Index != 1 {
		t.Fatalf("error %v, want BatchInsertError at index 1", err)
	}
}

// Postgres placeholders are numbered through the whole statement ($1 … $n).
func TestInsertRowsPostgresPlaceholders(t *testing.T) {
	ex := &recordingExecutor{}
	rows := [][]interface{}{{1, "a"}, {2, "b"}}
	if err := InsertRows(ex, database.DBTypePostgres, `c$T`, []string{"no", "name"}, rows); err != nil {
		t.Fatal(err)
	}
	want := `INSERT INTO "c$T" (no, name) VALUES ($1, $2), ($3, $4)`
	if len(ex.queries) != 1 || ex.queries[0] != want {
		t.Fatalf("queries %q, want %q", ex.queries, want)
	}
}

type recordingExecutor struct {
	database.Executor
	queries []string
}

func (r *recordingExecutor) Exec(query string, args ...interface{}) (sql.Result, error) {
	r.queries = append(r.queries, query)
	return nil, nil
}
