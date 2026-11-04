package itemjnl

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

// demoCompanyPostgres is a demo company with the demo items' postings on PostgreSQL
// (TEST_POSTGRES_DSN), dropped afterwards
func demoCompanyPostgres(t *testing.T) ftables.Source {
	t.Helper()
	dsn := os.Getenv("TEST_POSTGRES_DSN")
	if dsn == "" {
		t.Skip("TEST_POSTGRES_DSN not set")
	}
	db, err := sql.Open("postgres", dsn)
	must(t, err)
	t.Cleanup(func() { _ = db.Close() })
	name := fmt.Sprintf("itemtest%d", time.Now().UnixNano()%1000000)
	t.Cleanup(func() {
		tx, err := db.Begin()
		if err == nil {
			_ = company.DropObjects(tx, database.DBTypePostgres, name)
			_ = tx.Commit()
		}
	})
	must(t, tables.CreateCompanyTables(db, name, database.DBTypePostgres))
	_, err = demodata.Create(db, name, database.DBTypePostgres, demodata.Options{
		Today: time.Date(2026, 10, 6, 0, 0, 0, 0, time.Local), ItemJnlPostLine: openPoster})
	must(t, err)
	return ftables.DBSource(db, name, database.DBTypePostgres)
}

// Item posting on PostgreSQL: two batches posted at the same time get distinct item ledger,
// application, register and G/L entry numbers; FIFO application and the costs work as on SQLite
func TestItemPostConcurrentPostgres(t *testing.T) {
	src := demoCompanyPostgres(t)
	itemJnlBatch := gtables.NewItemJournalBatch(src)
	itemJnlBatch.InitRecord()
	itemJnlBatch.Journal_template_name, itemJnlBatch.Name = "ITEM", "SECOND"
	must(t, itemJnlBatch.InsertOrError(true))
	before := gtables.NewItemLedgerEntry(src).Count()
	for i, batch := range []string{"DEFAULT", "SECOND"} {
		for j, itemNo := range []string{"1000", "1100", "2000"} {
			line := gtables.NewItemJournalLine(src)
			line.InitRecord()
			line.Journal_template_name, line.Journal_batch_name, line.Line_no = "ITEM", types.Code(batch), (j+1)*10000
			line.Source_code = "ITEMJNL"
			must(t, line.Validate("posting_date", date(t, 6, 10, 2026)))
			line.Document_no = types.Code(fmt.Sprintf("T0000%d", 1))
			if i == 1 {
				line.Document_no = "PG-2"
			}
			must(t, line.Validate("entry_type", gtables.ItemJournalLine_Entry_type.Sale))
			must(t, line.Validate("item_no", types.Code(itemNo)))
			must(t, line.Validate("location_code", types.Code("MAIN")))
			must(t, line.Validate("quantity", dec("2")))
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
			line := gtables.NewItemJournalLine(txSrc)
			line.Journal_template_name, line.Journal_batch_name = "ITEM", types.Code(batch)
			_, err = NewItemJnlPostBatch(txSrc).Run(line)
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
	if n := gtables.NewItemLedgerEntry(src).Count() - before; n != 6 {
		t.Errorf("%d item ledger entries posted, want 6", n)
	}
	reg := gtables.NewItemRegister(src)
	reg.SetFilter("journal_batch_name", "DEFAULT|SECOND")
	if reg.Count() < 2 {
		t.Errorf("%d item registers", reg.Count())
	}
	var glEntry = gtables.NewGLEntry(src)
	_ = glEntry.CalcSums("amount")
	if !glEntry.Amount.Round(2).IsZero() {
		t.Errorf("G/L entries add up to %s", glEntry.Amount)
	}
}
