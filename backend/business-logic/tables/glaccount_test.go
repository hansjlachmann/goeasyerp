package tables

import (
	"testing"

	"github.com/hansjlachmann/goeasyerp/backend/foundation/database"
	"github.com/hansjlachmann/goeasyerp/backend/foundation/system"
	"github.com/hansjlachmann/goeasyerp/backend/foundation/types"
	gtables "github.com/hansjlachmann/goeasyerp/backend/generated/tables"
)

func newGLDB(t *testing.T) database.Executor {
	t.Helper()
	db := newTestDB(t)
	for _, tbl := range []interface {
		CreateTableWithDBType(database.Executor, string, database.DBType) error
	}{&gtables.GLAccount{}, &gtables.GenBusinessPostingGroup{}, &gtables.GenProductPostingGroup{}, &gtables.GeneralLedgerSetup{},
		&gtables.TaxBusinessPostingGroup{}, &gtables.TaxProductPostingGroup{}} {
		if err := tbl.CreateTableWithDBType(db, "c1", database.DBTypeSQLite); err != nil {
			t.Fatal(err)
		}
	}
	return db
}

func addAccount(t *testing.T, db database.Executor, no string, typ gtables.GLAccountAccount_type, ib gtables.GLAccountIncome_balance) {
	t.Helper()
	var a gtables.GLAccount
	a.InitWithDBType(db, "c1", database.DBTypeSQLite)
	a.InitRecord()
	a.No = types.NewCode(no)
	a.Account_type = typ
	a.Income_balance = ib
	if !a.Insert(false) {
		t.Fatalf("insert %s", no)
	}
}

func TestGLAccountTriggers(t *testing.T) {
	db := newGLDB(t)
	at := gtables.GLAccount_Account_type
	var a gtables.GLAccount
	a.InitWithDBType(db, "c1", database.DBTypeSQLite)
	a.InitRecord()
	if !a.Direct_posting {
		t.Error("Direct Posting is not on by default")
	}
	// Search Name follows the name while it was blank or the old name
	if err := a.Validate("name", types.NewText("Bank Account")); err != nil || a.Search_name != "BANK ACCOUNT" {
		t.Errorf("search name: %q %v", a.Search_name, err)
	}
	a.Search_name = "MY BANK"
	_ = a.Validate("name", types.NewText("Bank"))
	if a.Search_name != "MY BANK" {
		t.Errorf("a search name of its own was overwritten: %q", a.Search_name)
	}
	// Account Type: Totaling cleared, Direct Posting off for non-posting accounts
	a.Totaling = "1000..1999"
	if err := a.Validate("account_type", at.EndTotal); err != nil || a.Totaling != "" || a.Direct_posting {
		t.Errorf("account type End-Total: totaling %q, direct posting %v, %v", a.Totaling, a.Direct_posting, err)
	}
	if err := a.Validate("totaling", types.NewText("1000..1999")); err != nil {
		t.Errorf("totaling on an End-Total: %v", err)
	}
	// stored as End-Total (xRec), then back to Posting: Direct Posting on again
	a.No = "9999"
	if !a.Insert(false) || !a.Get(types.Code("9999")) {
		t.Fatal("store the account")
	}
	if err := a.Validate("account_type", at.Posting); err != nil || !a.Direct_posting {
		t.Errorf("back to Posting: direct posting %v %v", a.Direct_posting, err)
	}
	if err := a.Validate("totaling", types.NewText("1000")); err == nil || message(t, err, "en-US") != "Account Type must not be Posting" {
		t.Errorf("totaling on a posting account: %v", err)
	}

	// The general posting groups fill in their default tax groups (Auto Insert Default)
	var bus gtables.GenBusinessPostingGroup
	bus.InitWithDBType(db, "c1", database.DBTypeSQLite)
	bus.InitRecord()
	bus.Code, bus.Def_tax_bus_posting_group = "DOMESTIC", "DOM"
	var prod gtables.GenProductPostingGroup
	prod.InitWithDBType(db, "c1", database.DBTypeSQLite)
	prod.InitRecord()
	prod.Code, prod.Def_tax_prod_posting_group, prod.Auto_insert_default = "RETAIL", "STD", false
	var taxBus gtables.TaxBusinessPostingGroup
	taxBus.InitWithDBType(db, "c1", database.DBTypeSQLite)
	taxBus.Code = "DOM"
	if !bus.Insert(false) || !prod.Insert(false) || !taxBus.Insert(false) {
		t.Fatal("insert posting groups")
	}
	if err := a.Validate("gen_bus_posting_group", types.NewCode("DOMESTIC")); err != nil || a.Tax_bus_posting_group != "DOM" {
		t.Errorf("tax bus. posting group = %q %v", a.Tax_bus_posting_group, err)
	}
	if err := a.Validate("gen_prod_posting_group", types.NewCode("RETAIL")); err != nil || a.Tax_prod_posting_group != "" {
		t.Errorf("no auto insert: tax prod. posting group = %q %v", a.Tax_prod_posting_group, err)
	}

	// Last Date Modified on modify
	a.No = "1000"
	if !a.Insert(false) || !a.Get("1000") {
		t.Fatal("insert 1000")
	}
	a.Name = "Changed"
	if !a.Modify(true) || !a.Last_date_modified.Equal(system.Today()) {
		t.Errorf("last date modified = %v", a.Last_date_modified)
	}
}

func TestIndentChartOfAccounts(t *testing.T) {
	db := newGLDB(t)
	at, bs := gtables.GLAccount_Account_type, gtables.GLAccount_Income_balance.BalanceSheet
	for _, a := range []struct {
		no  string
		typ gtables.GLAccountAccount_type
	}{{"1000", at.Heading}, {"1100", at.BeginTotal}, {"1110", at.BeginTotal}, {"1120", at.Posting}, {"1190", at.EndTotal}, {"1200", at.Posting}, {"1999", at.EndTotal}, {"2000", at.Posting}} {
		addAccount(t, db, a.no, a.typ, bs)
	}
	var seen []string
	if err := IndentChartOfAccounts(db, "c1", database.DBTypeSQLite, func(no string) { seen = append(seen, no) }); err != nil {
		t.Fatal(err)
	}
	want := map[string]int{"1000": 0, "1100": 0, "1110": 1, "1120": 2, "1190": 1, "1200": 1, "1999": 0, "2000": 0}
	for no, level := range want {
		var a gtables.GLAccount
		a.InitWithDBType(db, "c1", database.DBTypeSQLite)
		if !a.Get(no) || a.Indentation != level {
			t.Errorf("%s: indentation %d, want %d", no, a.Indentation, level)
		}
	}
	var a gtables.GLAccount
	a.InitWithDBType(db, "c1", database.DBTypeSQLite)
	if a.Get("1999"); a.Totaling != "1100..1999" {
		t.Errorf("1999 totaling = %q", a.Totaling)
	}
	if len(seen) != 8 {
		t.Errorf("progress told %d accounts", len(seen))
	}

	addAccount(t, db, "0999", at.EndTotal, bs)
	if err := IndentChartOfAccounts(db, "c1", database.DBTypeSQLite, nil); err == nil || message(t, err, "en-US") != "End-Total 0999 is missing a matching Begin-Total." {
		t.Errorf("End-Total without Begin-Total: %v", err)
	}
}

func TestSetupNewGLAcc(t *testing.T) {
	db := newGLDB(t)
	ib := gtables.GLAccount_Income_balance
	addAccount(t, db, "1000", gtables.GLAccount_Account_type.Posting, ib.BalanceSheet)
	addAccount(t, db, "3000", gtables.GLAccount_Account_type.Posting, ib.IncomeStatement)
	var x, rec gtables.GLAccount
	x.InitWithDBType(db, "c1", database.DBTypeSQLite)
	rec.InitWithDBType(db, "c1", database.DBTypeSQLite)
	x.Get("3000")
	SetupNewGLAcc(&rec, &x, true) // below 3000
	if rec.Income_balance != ib.IncomeStatement {
		t.Error("below 3000: not its Income/Balance")
	}
	SetupNewGLAcc(&rec, &x, false) // above 3000: the account before it, 1000
	if rec.Income_balance != ib.BalanceSheet {
		t.Error("above 3000: not the Income/Balance of 1000")
	}
}

// Record filters with comparisons and open ranges (classic SETFILTER("No.", '<%1'))
func TestRecordFilterComparisons(t *testing.T) {
	db := newGLDB(t)
	for _, no := range []string{"1000", "2000", "3000", "4000"} {
		addAccount(t, db, no, gtables.GLAccount_Account_type.Posting, gtables.GLAccount_Income_balance.IncomeStatement)
	}
	count := func(filter string) int {
		var a gtables.GLAccount
		a.InitWithDBType(db, "c1", database.DBTypeSQLite)
		a.SetFilter("no", filter)
		n := 0
		if a.FindSet() {
			for {
				n++
				if !a.Next() {
					break
				}
			}
		}
		return n
	}
	for filter, want := range map[string]int{"<3000": 2, "<=3000": 3, ">1000": 3, ">=4000": 1, "2000..": 3, "..2000": 2, "1000..2000|4000": 3, "<>1000": 3} {
		if got := count(filter); got != want {
			t.Errorf("%s: %d accounts, want %d", filter, got, want)
		}
	}
}
