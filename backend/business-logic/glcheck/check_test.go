package glcheck

import (
	"database/sql"
	"testing"
	"time"

	_ "github.com/mattn/go-sqlite3"

	"github.com/hansjlachmann/goeasyerp/backend/business-logic/demodata"
	"github.com/hansjlachmann/goeasyerp/backend/business-logic/itemjnl"
	"github.com/hansjlachmann/goeasyerp/backend/business-logic/salesposting"
	"github.com/hansjlachmann/goeasyerp/backend/business-logic/tables"
	"github.com/hansjlachmann/goeasyerp/backend/foundation/database"
	ftables "github.com/hansjlachmann/goeasyerp/backend/foundation/tables"
	"github.com/hansjlachmann/goeasyerp/backend/foundation/types"
	gtables "github.com/hansjlachmann/goeasyerp/backend/generated/tables"
)

// The demo company (generated G/L entries, posted sales, item and resource postings) passes the
// check; every kind of damage done to it directly in the database is found
func TestCheck(t *testing.T) {
	db, err := sql.Open("sqlite3", ":memory:")
	if err != nil {
		t.Fatal(err)
	}
	db.SetMaxOpenConns(1)
	t.Cleanup(func() { _ = db.Close() })
	if err := tables.CreateCompanyTables(db, "demo", database.DBTypeSQLite); err != nil {
		t.Fatal(err)
	}
	_, err = demodata.Create(db, "demo", database.DBTypeSQLite, demodata.Options{
		Today:           time.Date(2026, 10, 6, 0, 0, 0, 0, time.Local),
		ItemJnlPostLine: func(src ftables.Source) demodata.ItemJnlPoster { return itemjnl.NewItemJnlPostLine(src) },
		SalesPost:       func(src ftables.Source) demodata.SalesPoster { return salesposting.NewSalesPost(src) },
	})
	if err != nil {
		t.Fatal(err)
	}
	var lastEntry, lastReg int
	if err := db.QueryRow(`SELECT MAX(entry_no) FROM "demo$G_L_Entry"`).Scan(&lastEntry); err != nil {
		t.Fatal(err)
	}
	if err := db.QueryRow(`SELECT MAX(no) FROM "demo$G_L_Register"`).Scan(&lastReg); err != nil {
		t.Fatal(err)
	}

	result := Check(ftables.DBSource(db, "demo", database.DBTypeSQLite), nil)
	if !result.Balanced() || result.Entries != lastEntry || result.Registers == 0 {
		t.Fatalf("demo data: %+v", result)
	}

	// damaged: each case in a transaction of its own, rolled back
	damaged := func(name string, damage func(src ftables.Source) error) *Result {
		t.Helper()
		tx, err := db.Begin()
		if err != nil {
			t.Fatal(err)
		}
		defer func() { _ = tx.Rollback() }()
		src := ftables.DBSource(tx, "demo", database.DBTypeSQLite)
		if err := damage(src); err != nil {
			t.Fatalf("%s: %v", name, err)
		}
		return Check(src, nil)
	}
	sqlDamage := func(statement string) func(src ftables.Source) error {
		return func(src ftables.Source) error {
			_, err := src.GetDB().Exec(statement)
			return err
		}
	}

	// an amount changed: its date and its transaction do not balance
	result = damaged("amount", sqlDamage(`UPDATE "demo$G_L_Entry" SET amount = amount + 0.01 WHERE entry_no = 10`))
	if len(result.Dates) != 1 || len(result.Transactions) != 1 || !result.Dates[0].Amount.Equal(types.MustDecimal("0.01")) ||
		len(result.Gaps) != 0 || len(result.RegisterList) != 0 {
		t.Errorf("amount changed: %+v", result)
	}

	// an entry deleted: a gap, its register misses it, the date and transaction do not balance
	result = damaged("delete", sqlDamage(`DELETE FROM "demo$G_L_Entry" WHERE entry_no = 20`))
	if len(result.Gaps) != 1 || result.Gaps[0] != (Gap{20, 20}) || len(result.Dates) != 1 || len(result.Transactions) != 1 {
		t.Errorf("entry deleted: %+v", result)
	}
	if len(result.RegisterList) != 1 || result.RegisterList[0].Problem != EntriesMissing {
		t.Errorf("entry deleted, registers: %+v", result.RegisterList)
	}

	// balanced entries inserted without a register; a register overlapping another
	result = damaged("unregistered", func(src ftables.Source) error {
		for i, amount := range []string{"50", "-50"} {
			glEntry := gtables.NewGLEntry(src)
			glEntry.Entry_no = lastEntry + 1 + i
			glEntry.Gl_account_no = "1210"
			glEntry.Posting_date = types.MustDate("2026-10-01")
			glEntry.Amount = types.MustDecimal(amount)
			glEntry.Transaction_no = 999999
			if err := glEntry.InsertOrError(false); err != nil {
				return err
			}
		}
		glReg := gtables.NewGLRegister(src)
		glReg.No = lastReg + 1
		glReg.From_entry_no = 5
		glReg.To_entry_no = 6
		return glReg.InsertOrError(false)
	})
	if len(result.Dates) != 0 || len(result.Transactions) != 0 {
		t.Errorf("balanced inserts: %+v", result)
	}
	var notRegistered, overlaps int
	for _, problem := range result.RegisterList {
		switch problem.Problem {
		case NotRegistered:
			notRegistered++
			if problem.From != lastEntry+1 || problem.To != lastEntry+2 {
				t.Errorf("not registered %+v", problem)
			}
		case Overlaps:
			overlaps++
		}
	}
	if notRegistered != 1 || overlaps != 1 {
		t.Errorf("register problems: %+v", result.RegisterList)
	}
}
