package demodata

import (
	"database/sql"
	"fmt"
	"os"
	"testing"
	"time"

	_ "github.com/lib/pq"

	"github.com/hansjlachmann/goeasyerp/backend/business-logic/tables"
	"github.com/hansjlachmann/goeasyerp/backend/foundation/company"
	"github.com/hansjlachmann/goeasyerp/backend/foundation/database"
	ftables "github.com/hansjlachmann/goeasyerp/backend/foundation/tables"
	"github.com/hansjlachmann/goeasyerp/backend/foundation/types"
	gtables "github.com/hansjlachmann/goeasyerp/backend/generated/tables"
)

// On PostgreSQL a second run empties the company in its transaction — the entries' SIFT
// triggers fire on the deletes — and creates the same demo again with its totals
func TestCreateAgainReplacesTheDemoPostgres(t *testing.T) {
	dsn := os.Getenv("TEST_POSTGRES_DSN")
	if dsn == "" {
		t.Skip("TEST_POSTGRES_DSN not set")
	}
	db, err := sql.Open("postgres", dsn)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close() })
	name := fmt.Sprintf("demoagain%d", time.Now().UnixNano()%1000000)
	t.Cleanup(func() {
		tx, err := db.Begin()
		if err == nil {
			_ = company.DropObjects(tx, database.DBTypePostgres, name)
			_ = tx.Commit()
		}
	})
	if err := tables.CreateCompanyTables(db, name, database.DBTypePostgres); err != nil {
		t.Fatal(err)
	}
	first, err := Create(db, name, database.DBTypePostgres, Options{Today: today})
	if err != nil {
		t.Fatalf("first Create: %v", err)
	}
	src := ftables.DBSource(db, name, database.DBTypePostgres)
	balanceBefore := customerBalances(t, src)

	second, err := Create(db, name, database.DBTypePostgres, Options{Today: today})
	if err != nil {
		t.Fatalf("second Create: %v", err)
	}
	if second != first {
		t.Errorf("second run created other data:\n%+v\nfirst:\n%+v", second, first)
	}
	// the customers' balances come from the SIFT totals: the same as after the first run
	if got := customerBalances(t, src); got != balanceBefore {
		t.Errorf("customer balances after the second run %s, want %s", got, balanceBefore)
	}
}

// customerBalances is the sum of the customers' Balance (LCY) FlowFields
func customerBalances(t *testing.T, src ftables.Source) string {
	t.Helper()
	total := types.Decimal{}
	cust := gtables.NewCustomer(src)
	for found := cust.FindSet(); found; found = cust.Next() {
		cust.CalcFields("balance_lcy")
		total = total.Add(cust.Balance_lcy)
	}
	return total.String()
}
