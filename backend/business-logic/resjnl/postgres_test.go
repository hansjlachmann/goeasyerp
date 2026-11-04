package resjnl_test

import (
	"database/sql"
	"fmt"
	"os"
	"sync"
	"testing"
	"time"

	_ "github.com/lib/pq"

	"github.com/hansjlachmann/goeasyerp/backend/business-logic/demodata"
	"github.com/hansjlachmann/goeasyerp/backend/business-logic/resjnl"
	"github.com/hansjlachmann/goeasyerp/backend/business-logic/tables"
	"github.com/hansjlachmann/goeasyerp/backend/foundation/company"
	"github.com/hansjlachmann/goeasyerp/backend/foundation/database"
	ftables "github.com/hansjlachmann/goeasyerp/backend/foundation/tables"
	"github.com/hansjlachmann/goeasyerp/backend/foundation/types"
	gtables "github.com/hansjlachmann/goeasyerp/backend/generated/tables"
)

// Resource posting on PostgreSQL (TEST_POSTGRES_DSN): two batches posted at the same time get
// distinct resource ledger entry and register numbers
func TestResPostConcurrentPostgres(t *testing.T) {
	dsn := os.Getenv("TEST_POSTGRES_DSN")
	if dsn == "" {
		t.Skip("TEST_POSTGRES_DSN not set")
	}
	db, err := sql.Open("postgres", dsn)
	must(t, err)
	t.Cleanup(func() { _ = db.Close() })
	name := fmt.Sprintf("restest%d", time.Now().UnixNano()%1000000)
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
	src := ftables.DBSource(db, name, database.DBTypePostgres)

	resJnlBatch := gtables.NewResJournalBatch(src)
	resJnlBatch.InitRecord()
	resJnlBatch.Journal_template_name, resJnlBatch.Name = "RESOURCES", "SECOND"
	must(t, resJnlBatch.InsertOrError(true))
	before := gtables.NewResLedgerEntry(src).Count()
	for _, batch := range []string{"DEFAULT", "SECOND"} {
		for j, resNo := range []string{"R0010", "R0040", "R0070"} {
			line := gtables.NewResJournalLine(src)
			line.InitRecord()
			line.Journal_template_name, line.Journal_batch_name, line.Line_no = "RESOURCES", types.Code(batch), (j+1)*10000
			line.Document_no = types.Code("PG-" + batch)
			must(t, line.Validate("posting_date", types.NewDate(2026, 10, 6)))
			must(t, line.Validate("resource_no", types.Code(resNo)))
			must(t, line.Validate("quantity", dec("3")))
			must(t, line.InsertOrError(true))
		}
	}
	var wg sync.WaitGroup
	errs := make([]error, 2)
	for i, batch := range []string{"DEFAULT", "SECOND"} {
		wg.Add(1)
		go func(i int, batch string) {
			defer wg.Done()
			tx, err := ftables.BeginTransaction(src.GetDB())
			if err != nil {
				errs[i] = err
				return
			}
			txSrc := ftables.DBSource(tx, src.GetCompany(), src.GetDBType())
			line := gtables.NewResJournalLine(txSrc)
			line.Journal_template_name, line.Journal_batch_name = "RESOURCES", types.Code(batch)
			_, err = resjnl.NewResJnlPostBatch(txSrc).Run(line)
			if err != nil {
				_ = ftables.Rollback(tx)
				errs[i] = err
				return
			}
			errs[i] = ftables.Commit(tx)
		}(i, batch)
	}
	wg.Wait()
	for _, err := range errs {
		must(t, err)
	}
	if n := gtables.NewResLedgerEntry(src).Count() - before; n != 6 {
		t.Errorf("%d resource ledger entries posted, want 6", n)
	}
	// the two postings' registers cover distinct entry ranges
	last := gtables.NewResourceRegister(src)
	if !last.FindLast() {
		t.Fatal("no resource register")
	}
	other := gtables.NewResourceRegister(src)
	must(t, other.GetOrError(last.No-1))
	if other.To_entry_no >= last.From_entry_no || last.To_entry_no-last.From_entry_no != 2 || other.To_entry_no-other.From_entry_no != 2 {
		t.Errorf("registers %d %d..%d and %d %d..%d", other.No, other.From_entry_no, other.To_entry_no, last.No, last.From_entry_no, last.To_entry_no)
	}
}
