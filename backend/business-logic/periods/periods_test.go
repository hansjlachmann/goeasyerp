package periods

import (
	"database/sql"
	"testing"
	"time"

	_ "github.com/mattn/go-sqlite3"

	"github.com/hansjlachmann/goeasyerp/backend/foundation/database"
	ftables "github.com/hansjlachmann/goeasyerp/backend/foundation/tables"
	"github.com/hansjlachmann/goeasyerp/backend/foundation/types"
	gtables "github.com/hansjlachmann/goeasyerp/backend/generated/tables"
)

func d(y, m, day int) types.Date { return types.NewDate(y, time.Month(m), day) }

// Calendar periods around a date, the steps between them and their names
func TestCalendarPeriods(t *testing.T) {
	date := d(2026, 10, 28) // a Wednesday
	for _, c := range []struct {
		periodType PeriodType
		search     string
		start, end types.Date
		name       string
	}{
		{Day, "=", date, date, ""},
		{Week, "=", d(2026, 10, 26), d(2026, 11, 1), "44.2026"},
		{Month, "=", d(2026, 10, 1), d(2026, 10, 31), ""},
		{Quarter, "=", d(2026, 10, 1), d(2026, 12, 31), "4/2026"},
		{Year, "=", d(2026, 1, 1), d(2026, 12, 31), "2026"},
		{Month, "<", d(2026, 9, 1), d(2026, 9, 30), ""},
		{Quarter, ">", d(2027, 1, 1), d(2027, 3, 31), "1/2027"},
	} {
		p, ok := FindDate(nil, c.search, date, c.periodType)
		if !ok || !p.Start.Equal(c.start) || !p.End.Equal(c.end) || p.Name != c.name {
			t.Errorf("type %d %q: %s..%s %q, want %s..%s %q", c.periodType, c.search, p.Start, p.End, p.Name, c.start, c.end, c.name)
		}
	}
	month, _ := FindDate(nil, "=", date, Month)
	next, steps := NextDate(nil, 3, month, Month)
	if steps != 3 || !next.Start.Equal(d(2027, 1, 1)) || !next.End.Equal(d(2027, 1, 31)) {
		t.Errorf("3 months on: %s..%s (%d steps)", next.Start, next.End, steps)
	}
	prev, steps := NextDate(nil, -2, month, Month)
	if steps != -2 || !prev.Start.Equal(d(2026, 8, 1)) {
		t.Errorf("2 months back: %s (%d steps)", prev.Start, steps)
	}
	// 1 January 2027 is in week 53 of 2026
	if name := CreatePeriodFormat(Week, d(2026, 12, 28)); name != "53.2026" {
		t.Errorf("week of 28 Dec 2026: %q", name)
	}
	// a week starting in December that is week 1 of the next year is named by its Sunday
	if name := CreatePeriodFormat(Week, d(2025, 12, 29)); name != "1.2026" {
		t.Errorf("week of 29 Dec 2025: %q", name)
	}
}

// Accounting periods: from the Accounting Period table, the last one never ends
func TestAccountingPeriods(t *testing.T) {
	db, err := sql.Open("sqlite3", ":memory:")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close() })
	if err := (&gtables.AccountingPeriod{}).CreateTableWithDBType(db, "c1", database.DBTypeSQLite); err != nil {
		t.Fatal(err)
	}
	src := ftables.DBSource(db, "c1", database.DBTypeSQLite)
	for _, m := range []int{1, 2, 3} {
		ap := gtables.NewAccountingPeriod(src)
		ap.Starting_date, ap.Name = d(2026, m, 1), types.Text([]string{"", "January", "February", "March"}[m])
		if err := ap.InsertOrError(false); err != nil {
			t.Fatal(err)
		}
	}
	p, ok := FindDate(src, "", d(2026, 2, 14), AccountingPeriod)
	if !ok || !p.Start.Equal(d(2026, 2, 1)) || !p.End.Equal(d(2026, 2, 28)) || p.Name != "February" {
		t.Errorf("period of 14 Feb: %s..%s %q", p.Start, p.End, p.Name)
	}
	last, steps := NextDate(src, 5, p, AccountingPeriod)
	if steps != 1 || last.Name != "March" || last.End.Year() != 9999 {
		t.Errorf("5 steps on: %q to %s (%d steps), want March, never ending, 1 step", last.Name, last.End, steps)
	}
	if _, ok := FindDate(src, "=", d(2025, 6, 1), AccountingPeriod); ok {
		t.Error("a period before the first accounting period found with =")
	}
}
