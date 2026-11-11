package periods

import (
	"database/sql"
	"testing"

	"github.com/hansjlachmann/goeasyerp/backend/foundation/database"
	ftables "github.com/hansjlachmann/goeasyerp/backend/foundation/tables"
	"github.com/hansjlachmann/goeasyerp/backend/foundation/types"
	gtables "github.com/hansjlachmann/goeasyerp/backend/generated/tables"
)

// Closing Entries: Include ends the filter at the period's closing date; Exclude cuts out the
// closing date of every fiscal year ending in the filter (its own end too)
func TestClosingEntriesDateFilter(t *testing.T) {
	db, err := sql.Open("sqlite3", ":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	if err := (&gtables.AccountingPeriod{}).CreateTableWithDBType(db, "c1", database.DBTypeSQLite); err != nil {
		t.Fatal(err)
	}
	src := ftables.DBSource(db, "c1", database.DBTypeSQLite)
	for _, start := range []types.Date{types.NewDate(2025, 1, 1), types.NewDate(2026, 1, 1), types.NewDate(2027, 1, 1)} {
		accountingPeriod := gtables.NewAccountingPeriod(src)
		accountingPeriod.Starting_date = start
		accountingPeriod.New_fiscal_year = true
		if err := accountingPeriod.InsertOrError(false); err != nil {
			t.Fatal(err)
		}
	}
	year2026 := Period{Start: types.NewDate(2026, 1, 1), End: types.NewDate(2026, 12, 31)}
	june2026 := Period{Start: types.NewDate(2026, 6, 1), End: types.NewDate(2026, 6, 30)}
	for _, c := range []struct {
		period         Period
		balanceAtDate  bool
		closingEntries ClosingEntries
		want           string
	}{
		{year2026, false, IncludeClosingEntries, "2026-01-01..C2026-12-31"},
		{year2026, false, ExcludeClosingEntries, "2026-01-01..2026-12-31"},
		{june2026, false, ExcludeClosingEntries, "2026-06-01..C2026-06-30"},
		{june2026, true, IncludeClosingEntries, "..C2026-06-30"},
		{june2026, true, ExcludeClosingEntries, "..2024-12-31|2025-01-01..2025-12-31|2026-01-01..C2026-06-30"},
		{year2026, true, ExcludeClosingEntries, "..2024-12-31|2025-01-01..2025-12-31|2026-01-01..2026-12-31"},
	} {
		if got := ClosingEntriesDateFilter(src, c.period, c.balanceAtDate, c.closingEntries); got != c.want {
			t.Errorf("%v balance at date %v closing %d: %q, want %q", c.period.Start, c.balanceAtDate, c.closingEntries, got, c.want)
		}
	}
}
