package tables

import (
	"database/sql"
	"testing"

	"github.com/hansjlachmann/goeasyerp/backend/foundation/types"
)

// cursorDB is a SQLite table with testFields' stored columns and three rows
func cursorDB(t *testing.T) *sql.DB {
	t.Helper()
	db, err := sql.Open("sqlite3", ":memory:")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close() })
	db.SetMaxOpenConns(1)
	_, err = db.Exec(`CREATE TABLE "Test" (code TEXT PRIMARY KEY, name TEXT, amount TEXT, date TEXT, picture BLOB,
		blocked BOOLEAN, status INTEGER, count INTEGER, formula TEXT)`)
	if err != nil {
		t.Fatal(err)
	}
	for _, row := range []struct {
		code, amount  string
		status, count int
	}{{"A", "16.2", 1, 3}, {"B", "100", 2, 1}, {"C", "0", 0, 2}} {
		_, err := db.Exec(`INSERT INTO "Test" VALUES (?, 'n', ?, '2026-11-20', NULL, 1, ?, ?, '')`, row.code, row.amount, row.status, row.count)
		if err != nil {
			t.Fatal(err)
		}
	}
	return db
}

func testRecOn(db *sql.DB, r *testRec, q *Query, c *Cursor, stores *int) *Rec {
	return &Rec{
		Fields: testFields, Ref: r.ref, QT: testQT(true), Q: q, Cur: c, Exec: db,
		Table: "Test", Store: func() { *stores++ },
	}
}

func TestRecReads(t *testing.T) {
	db := cursorDB(t)
	r := &testRec{Balance: types.NewDecimalFromInt(9)}
	var q Query
	var c Cursor
	stores := 0
	rec := testRecOn(db, r, &q, &c, &stores)

	if !rec.Get([]interface{}{"B"}) || r.Code != "B" || r.Status != 2 || !r.Blocked || r.Amount.String() != "100" || r.Date.String() != "2026-11-20" {
		t.Fatalf("Get B: %+v", r)
	}
	if rec.Get([]interface{}{"X"}) {
		t.Error("Get X found a record")
	}
	// decimals stored as text sort as numbers: 0, 16.2, 100
	q.SetCurrentKey(rec.QT, "amount")
	if !rec.FindFirst() || r.Code != "C" {
		t.Errorf("FindFirst = %s", r.Code)
	}
	if !rec.FindLast() || r.Code != "B" || q.Descending {
		t.Errorf("FindLast = %s", r.Code)
	}
	if n := rec.Count(); n != 3 {
		t.Errorf("Count = %d", n)
	}
	// after Find: Next moves on in the key
	if !rec.FindFirst() || !rec.Next(nil, rec.FindInKey) || r.Code != "A" || !rec.Next([]int{-1}, rec.FindInKey) || r.Code != "C" {
		t.Errorf("Next after FindFirst = %s", r.Code)
	}
	if stores == 0 {
		t.Error("reads store the old values")
	}
}

func TestRecFindSet(t *testing.T) {
	db := cursorDB(t)
	r := &testRec{}
	var q Query
	var c Cursor
	stores := 0
	rec := testRecOn(db, r, &q, &c, &stores)
	q.SetCurrentKey(rec.QT, "count")

	var codes string
	for found := rec.FindSet(); found; found = rec.Next(nil, rec.FindInKey) {
		codes += string(r.Code)
	}
	if codes != "BCA" {
		t.Errorf("FindSet order = %s", codes)
	}

	r.Balance = types.NewDecimalFromInt(9)
	if !rec.FindSetBuffered() || r.Code != "B" || !r.Balance.IsZero() {
		t.Fatalf("FindSetBuffered: %+v", r)
	}
	if !rec.Next([]int{2}, nil) || r.Code != "A" || !rec.Next([]int{-1}, nil) || r.Code != "C" || rec.Next([]int{5}, nil) {
		t.Errorf("buffered Next = %s", r.Code)
	}

	rec.Buffered = true
	c.SetPage(1, 1)
	if !rec.FindSet() || r.Code != "C" || rec.Next(nil, nil) {
		t.Errorf("paged buffered FindSet = %s", r.Code)
	}
	c.SetPage(0, 0)
	q.SetRange(rec.QT, "code", "X")
	if rec.FindSet() {
		t.Error("FindSet without records")
	}
}
