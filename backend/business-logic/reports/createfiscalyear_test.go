package reports

import (
	"testing"

	"github.com/hansjlachmann/goeasyerp/backend/foundation/database"
	ftables "github.com/hansjlachmann/goeasyerp/backend/foundation/tables"
	"github.com/hansjlachmann/goeasyerp/backend/foundation/types"
	gtables "github.com/hansjlachmann/goeasyerp/backend/generated/tables"
)

// Report 93 creates a fiscal year's periods and the next year's first period; the next run
// starts there; a year before a closed one is closed at once; a year inside the existing ones
// is refused
func TestCreateFiscalYear(t *testing.T) {
	db := newTestDB(t)
	if err := (&gtables.AccountingPeriod{}).CreateTableWithDBType(db, "c1", database.DBTypeSQLite); err != nil {
		t.Fatal(err)
	}
	periods := func() []*gtables.AccountingPeriod {
		t.Helper()
		var all []*gtables.AccountingPeriod
		p := gtables.NewAccountingPeriod(ftables.DBSource(db, "c1", database.DBTypeSQLite))
		for found := p.FindSet(); found; found = p.Next() {
			c := *p
			all = append(all, &c)
		}
		return all
	}
	run := func(options map[string]string) error {
		_, err := Execute(CreateFiscalYearID, db, "c1", database.DBTypeSQLite, RunOptions{Options: options})
		return err
	}

	if err := run(map[string]string{"starting_date": "2026-01-01"}); err != nil {
		t.Fatal(err)
	}
	all := periods()
	if len(all) != 13 || all[0].Name != "January" || !all[0].New_fiscal_year || !all[0].Date_locked ||
		all[1].New_fiscal_year || all[1].Date_locked || !all[12].New_fiscal_year || !all[12].Starting_date.Equal(types.NewDate(2027, 1, 1)) {
		t.Fatalf("first year: %d periods", len(all))
	}
	// The next year starts at the last period (the default of Starting Date)
	if err := run(nil); err != nil {
		t.Fatal(err)
	}
	if all = periods(); len(all) != 25 || !all[24].Starting_date.Equal(types.NewDate(2028, 1, 1)) {
		t.Fatalf("second year: %d periods", len(all))
	}
	// Quarters in a year before the first, date locked year: closed at once
	if err := run(map[string]string{"starting_date": "2025-01-01", "no_of_periods": "4", "period_length": "3M"}); err != nil {
		t.Fatal(err)
	}
	all = periods()
	if len(all) != 29 || !all[0].Closed || !all[3].Date_locked || !all[3].Starting_date.Equal(types.NewDate(2025, 10, 1)) || all[4].Closed {
		t.Errorf("year before: %d periods", len(all))
	}
	if err := run(map[string]string{"starting_date": "2026-06-01"}); err == nil {
		t.Error("a year inside the existing ones was created")
	}
}
