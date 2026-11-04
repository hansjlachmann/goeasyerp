package pages

import (
	"database/sql"
	"fmt"
	"testing"

	"github.com/hansjlachmann/goeasyerp/backend/foundation/database"
	"github.com/hansjlachmann/goeasyerp/backend/foundation/pages"
	"github.com/hansjlachmann/goeasyerp/backend/foundation/system"
	ftables "github.com/hansjlachmann/goeasyerp/backend/foundation/tables"
	"github.com/hansjlachmann/goeasyerp/backend/foundation/types"
	gtables "github.com/hansjlachmann/goeasyerp/backend/generated/tables"
)

// Page 152 shows the purchases positive (classic -Sum of the entries' Purchase (LCY)) and
// the balance owed
func TestVendorStatistics(t *testing.T) {
	db, err := sql.Open("sqlite3", ":memory:")
	if err != nil {
		t.Fatal(err)
	}
	db.SetMaxOpenConns(1)
	t.Cleanup(func() { _ = db.Close() })
	src := ftables.DBSource(db, "c1", database.DBTypeSQLite)
	for _, tbl := range []interface {
		CreateTableWithDBType(database.Executor, string, database.DBType) error
	}{&gtables.Vendor{}, &gtables.VendorLedgerEntry{}, &gtables.AccountingPeriod{}} {
		if err := tbl.CreateTableWithDBType(db, "c1", database.DBTypeSQLite); err != nil {
			t.Fatal(err)
		}
	}
	vend := gtables.NewVendor(src)
	vend.No = "V1"
	if err := vend.InsertOrError(false); err != nil {
		t.Fatal(err)
	}
	entry := gtables.NewVendorLedgerEntry(src)
	entry.Entry_no, entry.Vendor_no, entry.Posting_date, entry.Open = 1, "V1", system.WorkDate(), true
	entry.Amount_lcy, entry.Remaining_amt_lcy, entry.Purchase_lcy = types.NewDecimal(-250), types.NewDecimal(-250), types.NewDecimal(-250)
	if err := entry.InsertOrError(false); err != nil {
		t.Fatal(err)
	}
	control, err := pages.RunAfterGetRecordControl(152, vend)
	if err != nil {
		t.Fatal(err)
	}
	v := control.Values()
	if fmt.Sprint(v["to_date_purchases_lcy"]) != "250" || fmt.Sprint(v["total_lcy"]) != "250" || fmt.Sprint(v["this_period_purchases_lcy"]) != "0" {
		t.Errorf("values %v", v)
	}
}
