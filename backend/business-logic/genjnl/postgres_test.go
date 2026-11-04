package genjnl

import (
	"database/sql"
	"fmt"
	"os"
	"sync"
	"testing"
	"time"

	_ "github.com/lib/pq"

	"github.com/hansjlachmann/goeasyerp/backend/business-logic/demodata"
	"github.com/hansjlachmann/goeasyerp/backend/business-logic/tables"
	"github.com/hansjlachmann/goeasyerp/backend/foundation/company"
	"github.com/hansjlachmann/goeasyerp/backend/foundation/database"
	ftables "github.com/hansjlachmann/goeasyerp/backend/foundation/tables"
	"github.com/hansjlachmann/goeasyerp/backend/foundation/types"
	gtables "github.com/hansjlachmann/goeasyerp/backend/generated/tables"
)

// demoCompanyPostgres is a demo company on PostgreSQL (TEST_POSTGRES_DSN), dropped afterwards
func demoCompanyPostgres(t *testing.T) ftables.Source {
	t.Helper()
	dsn := os.Getenv("TEST_POSTGRES_DSN")
	if dsn == "" {
		t.Skip("TEST_POSTGRES_DSN not set")
	}
	db, err := sql.Open("postgres", dsn)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close() })
	name := fmt.Sprintf("jnltest%d", time.Now().UnixNano()%1000000)
	t.Cleanup(func() {
		tx, err := db.Begin()
		if err == nil {
			_ = company.DropObjects(tx, database.DBTypePostgres, name)
			_ = tx.Commit()
		}
	})
	must(t, tables.CreateCompanyTables(db, name, database.DBTypePostgres))
	_, err = demodata.Create(db, name, database.DBTypePostgres, demodata.Options{Today: time.Date(2026, 10, 6, 0, 0, 0, 0, time.Local)})
	must(t, err)
	return ftables.DBSource(db, name, database.DBTypePostgres)
}

// Posting on PostgreSQL: the posting reads and writes in one transaction (one connection) —
// two batches posted at the same time get distinct entry and register numbers
func TestPostConcurrentPostgres(t *testing.T) {
	src := demoCompanyPostgres(t)
	for _, batchName := range []string{"PARALLEL1", "PARALLEL2"} {
		batch := gtables.NewGenJournalBatch(src)
		batch.Journal_template_name = "GENERAL"
		batch.Name = types.Code(batchName)
		must(t, batch.InsertOrError(true))
		for i := 1; i <= 5; i++ {
			addLine(t, src, "GENERAL", batchName, lineSpec{lineNo: i * 10000, docNo: fmt.Sprintf("%s-%d", batchName, i), accNo: "1210", amount: "10", balNo: "1230"})
		}
	}
	gl0, _, _, reg0 := countEntries(src)
	var wg sync.WaitGroup
	errs := make([]error, 2)
	for i, batchName := range []string{"PARALLEL1", "PARALLEL2"} {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, errs[i] = postBatch(t, src, "GENERAL", batchName)
		}()
	}
	wg.Wait()
	for _, err := range errs {
		must(t, err)
	}
	gl1, _, _, reg1 := countEntries(src)
	if gl1-gl0 != 20 || reg1-reg0 != 2 {
		t.Errorf("posted %d G/L entries and %d registers, want 20 and 2", gl1-gl0, reg1-reg0)
	}
	glReg := gtables.NewGLRegister(src)
	glReg.SetFilter("no", fmt.Sprintf(">%d", reg0))
	var ranges [][2]int
	for found := glReg.FindSet(); found; found = glReg.Next() {
		ranges = append(ranges, [2]int{glReg.From_entry_no, glReg.To_entry_no})
	}
	if len(ranges) == 2 && ranges[0][1] >= ranges[1][0] && ranges[1][1] >= ranges[0][0] {
		t.Errorf("the registers overlap: %v", ranges)
	}
}

// Closing dates on PostgreSQL (TIMESTAMP date columns): a line on C2025-12-31 posts entries that
// read back as closing, a filter on the closing date finds them and one on 31-12 does not, and
// the SIFT totals of the G/L account leave them out up to 31-12 and take them in up to C31-12
func TestClosingDatePostgres(t *testing.T) {
	src := demoCompanyPostgres(t)
	accountingPeriod := gtables.NewAccountingPeriod(src)
	accountingPeriod.SetRange("starting_date", types.NewDate(2025, 1, 1), types.NewDate(2025, 12, 31))
	accountingPeriod.ModifyAll("closed", true)
	accountingPeriod.SetRange("starting_date", types.NewDate(2025, 1, 1), types.NewDate(2026, 1, 1))
	accountingPeriod.ModifyAll("date_locked", true)

	netChange := func(dateFilter string) types.Decimal {
		glAcc := gtables.NewGLAccount(src)
		must(t, glAcc.GetOrError(types.Code("3110")))
		must(t, glAcc.SetFlowFilter("date_filter", dateFilter))
		glAcc.CalcFields("net_change")
		return glAcc.Net_change
	}
	year0, closing0 := netChange("2025-01-01..2025-12-31"), netChange("2025-01-01..C2025-12-31")

	line := newJnlLine(src, "GENERAL", "DEFAULT", 10000)
	must(t, line.Validate("posting_date", types.NewDate(2025, 12, 31).Closing()))
	line.Document_no = "CLOSE25"
	must(t, line.Validate("account_no", types.Code("3110")))
	must(t, line.Validate("bal_account_no", types.Code("2120")))
	must(t, line.Validate("amount", dec("100")))
	must(t, line.InsertOrError(true))
	_, err := postBatch(t, src, "GENERAL", "DEFAULT")
	must(t, err)

	glEntry := gtables.NewGLEntry(src)
	glEntry.SetRange("document_no", types.Code("CLOSE25"))
	if glEntry.Count() != 2 || !glEntry.FindFirst() || glEntry.Posting_date.String() != "C2025-12-31" {
		t.Fatalf("%d entries, first on %s", glEntry.Count(), glEntry.Posting_date)
	}
	glEntry.SetFilter("posting_date", "C2025-12-31")
	if glEntry.Count() != 2 {
		t.Errorf("the closing date filter finds %d entries", glEntry.Count())
	}
	glEntry.SetRange("posting_date", types.NewDate(2025, 12, 31))
	if glEntry.Count() != 0 {
		t.Errorf("the filter on 31-12 finds %d closing entries", glEntry.Count())
	}
	if got := netChange("2025-01-01..2025-12-31"); !got.Equal(year0) {
		t.Errorf("up to 31-12: %s, want %s", got, year0)
	}
	if got := netChange("2025-01-01..C2025-12-31").Sub(closing0); !got.Equal(dec("100")) {
		t.Errorf("up to C31-12 changed by %s, want 100", got)
	}
}
