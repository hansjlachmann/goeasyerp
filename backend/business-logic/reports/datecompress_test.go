package reports_test

import (
	"testing"

	"github.com/hansjlachmann/goeasyerp/backend/business-logic/glcheck"
	"github.com/hansjlachmann/goeasyerp/backend/business-logic/reports"
	"github.com/hansjlachmann/goeasyerp/backend/foundation/system"
	ftables "github.com/hansjlachmann/goeasyerp/backend/foundation/tables"
	"github.com/hansjlachmann/goeasyerp/backend/foundation/types"
	gtables "github.com/hansjlachmann/goeasyerp/backend/generated/tables"
)

// closedYear closes last year (as Close Year does: its periods closed, they and the first
// period of this year date locked) — the demo's entries start in it — and returns its first
// and last date
func closedYear(t *testing.T, src ftables.Source) (types.Date, types.Date) {
	t.Helper()
	start := types.NewDate(2025, 1, 1)
	end := types.NewDate(2025, 12, 31)
	accountingPeriod := gtables.NewAccountingPeriod(src)
	accountingPeriod.SetRange("starting_date", start, end)
	accountingPeriod.ModifyAll("closed", true)
	accountingPeriod.SetRange("starting_date", start, end.AddDays(1))
	accountingPeriod.ModifyAll("date_locked", true)
	return start, end
}

func runCompression(t *testing.T, src ftables.Source, id int, options map[string]string) {
	t.Helper()
	_, err := reports.Execute(id, src.GetDB(), src.GetCompany(), src.GetDBType(), reports.RunOptions{Options: options})
	if err != nil {
		t.Fatalf("report %d: %v", id, err)
	}
}

// lastComprRegister: the last Date Compr. Register row
func lastComprRegister(t *testing.T, src ftables.Source) *gtables.DateComprRegister {
	t.Helper()
	dateComprReg := gtables.NewDateComprRegister(src)
	if !dateComprReg.FindLast() {
		t.Fatal("no Date Compr. Register")
	}
	return dateComprReg
}

// Date Compress General Ledger: the closed year's entries become one per account and month
// (and sign), the balances and the G/L stay the same, a G/L register and a Date Compr.
// Register record the run; the G/L Balance Check finds nothing
func TestDateCompressGeneralLedger(t *testing.T) {
	src := listCompany(t)
	start, end := closedYear(t, src)
	balances := func() map[string]string {
		got := map[string]string{}
		glAcc := gtables.NewGLAccount(src)
		glAcc.SetRange("account_type", gtables.GLAccount_Account_type.Posting)
		for found := glAcc.FindSet(); found; found = glAcc.Next() {
			glAcc.CalcFields("balance")
			got[glAcc.No.String()] = system.Round(glAcc.Balance, types.MustDecimal("0.01")).String()
		}
		return got
	}
	before := balances()
	glEntry := gtables.NewGLEntry(src)
	glEntry.SetRange("posting_date", start, end)
	countBefore := glEntry.Count()
	if countBefore < 100 {
		t.Fatalf("%d entries in the closed year", countBefore)
	}

	runCompression(t, src, reports.DateCompressGeneralLedgerID, map[string]string{"starting_date": start.String(), "ending_date": end.String(),
		"period_length": "2", "description": "Compressed", "retain_dimension_1_code": "true"})

	dateComprReg := lastComprRegister(t, src)
	countAfter := glEntry.Count()
	if dateComprReg.Table_id != 17 || dateComprReg.No_records_deleted != countBefore || dateComprReg.No_of_new_records != countAfter ||
		countAfter >= countBefore || dateComprReg.Retain_field_contents == "" {
		t.Errorf("register: table %d, %d deleted (%d before), %d new (%d after), retained %q", dateComprReg.Table_id,
			dateComprReg.No_records_deleted, countBefore, dateComprReg.No_of_new_records, countAfter, dateComprReg.Retain_field_contents)
	}
	for found := glEntry.FindSet(); found; found = glEntry.Next() {
		if glEntry.Posting_date.Day() != 1 || !glEntry.Prior_year_entry || glEntry.Description != "Compressed" {
			t.Fatalf("new entry %d: %s prior-year %v %q", glEntry.Entry_no, glEntry.Posting_date, glEntry.Prior_year_entry, glEntry.Description)
		}
	}
	glReg := gtables.NewGLRegister(src)
	if !glReg.Get(dateComprReg.Register_no) || glReg.From_entry_no == 0 || glReg.To_entry_no < glReg.From_entry_no {
		t.Errorf("G/L register %d: %d..%d", dateComprReg.Register_no, glReg.From_entry_no, glReg.To_entry_no)
	}
	after := balances()
	for no, balance := range before {
		if after[no] != balance {
			t.Errorf("account %s balance %s after, %s before", no, after[no], balance)
		}
	}
	result := glcheck.Check(src, nil)
	if !result.Balanced() {
		t.Errorf("G/L Balance Check after the compression: %d dates, %d transactions, %d gaps, %d register problems",
			len(result.Dates), len(result.Transactions), len(result.Gaps), len(result.RegisterList))
	}
}

// postingCompany: the demo data with every ledger posted (items, sales, purchases, resources,
// fixed assets), in a transaction; last year closed
func postingCompany(t *testing.T) ftables.Source {
	t.Helper()
	src := listCompany(t)
	closedYear(t, src)
	return src
}

// countIn: the records of a table in the closed year (its date field)
func countIn(rec interface {
	SetRange(field string, values ...interface{})
	Count() int
}, dateField string) int {
	rec.SetRange(dateField, types.NewDate(2025, 1, 1), types.NewDate(2025, 12, 31))
	return rec.Count()
}

// Every ledger's date compression of the closed year: fewer entries afterwards, the totals the
// same, a Date Compr. Register row with the counts, the register of the new entries
func TestDateCompressLedgers(t *testing.T) {
	src := postingCompany(t)
	options := map[string]string{"starting_date": "2025-01-01", "ending_date": "2025-12-31", "period_length": "2"}
	cases := []struct {
		id       int
		tableID  int
		count    func() int
		total    func() string
		register bool
	}{
		{reports.DateCompressCustomerLedgerID, 21, func() int { return countIn(gtables.NewCustomerLedgerEntry(src), "posting_date") },
			func() string {
				e := gtables.NewCustomerLedgerEntry(src)
				_ = e.CalcSums("amount_lcy")
				return system.Round(e.Amount_lcy, types.MustDecimal("0.01")).String()
			}, true},
		{reports.DateCompressVendorLedgerID, 25, func() int { return countIn(gtables.NewVendorLedgerEntry(src), "posting_date") },
			func() string {
				e := gtables.NewVendorLedgerEntry(src)
				_ = e.CalcSums("amount_lcy")
				return system.Round(e.Amount_lcy, types.MustDecimal("0.01")).String()
			}, true},
		{reports.DateCompressBankAccLedgerID, 271, func() int { return countIn(gtables.NewBankAccountLedgerEntry(src), "posting_date") },
			func() string {
				e := gtables.NewBankAccountLedgerEntry(src)
				_ = e.CalcSums("amount_lcy")
				return system.Round(e.Amount_lcy, types.MustDecimal("0.01")).String()
			}, true},
		{reports.DateCompressTaxEntriesID, 254, func() int { return countIn(gtables.NewTaxEntry(src), "posting_date") },
			func() string {
				e := gtables.NewTaxEntry(src)
				_ = e.CalcSums("amount", "base")
				return system.Round(e.Amount, types.MustDecimal("0.01")).String() + "/" + system.Round(e.Base, types.MustDecimal("0.01")).String()
			}, true},
		{reports.DateCompressItemLedgerID, 32, func() int { return countIn(gtables.NewItemLedgerEntry(src), "posting_date") },
			func() string {
				e := gtables.NewItemLedgerEntry(src)
				_ = e.CalcSums("quantity", "amount")
				return system.Round(e.Quantity, types.MustDecimal("0.01")).String() + "/" + system.Round(e.Amount, types.MustDecimal("0.01")).String()
			}, true},
		{reports.DateCompressResourceLedgerID, 203, func() int { return countIn(gtables.NewResLedgerEntry(src), "posting_date") },
			func() string {
				e := gtables.NewResLedgerEntry(src)
				_ = e.CalcSums("quantity", "total_price")
				return system.Round(e.Quantity, types.MustDecimal("0.01")).String() + "/" + system.Round(e.Total_price, types.MustDecimal("0.01")).String()
			}, true},
		{reports.DateCompressFALedgerID, 5601, func() int { return countIn(gtables.NewFALedgerEntry(src), "fa_posting_date") },
			func() string {
				e := gtables.NewFALedgerEntry(src)
				_ = e.CalcSums("amount")
				return system.Round(e.Amount, types.MustDecimal("0.01")).String()
			}, true},
		{reports.DateCompressBOMLedgerID, 238, func() int { return countIn(gtables.NewBOMLedgerEntry(src), "posting_date") },
			func() string { return "" }, true},
		{reports.DateCompressMaintLedgerID, 5625, func() int { return countIn(gtables.NewMaintenanceLedgerEntry(src), "fa_posting_date") },
			func() string {
				e := gtables.NewMaintenanceLedgerEntry(src)
				_ = e.CalcSums("amount")
				return system.Round(e.Amount, types.MustDecimal("0.01")).String()
			}, true},
		{reports.DateCompressInsuranceLedgerID, 5629, func() int { return countIn(gtables.NewInsCoverageLedgerEntry(src), "posting_date") },
			func() string {
				e := gtables.NewInsCoverageLedgerEntry(src)
				_ = e.CalcSums("amount")
				return system.Round(e.Amount, types.MustDecimal("0.01")).String()
			}, true},
		{reports.DateComprGLBudgetEntriesID, 96, func() int { return countIn(gtables.NewGLBudgetEntry(src), "date") },
			func() string {
				e := gtables.NewGLBudgetEntry(src)
				_ = e.CalcSums("amount")
				return system.Round(e.Amount, types.MustDecimal("0.01")).String()
			}, false},
	}
	for _, c := range cases {
		before, totalBefore := c.count(), c.total()
		dateComprReg := gtables.NewDateComprRegister(src)
		registersBefore := dateComprReg.Count()
		runOptions := map[string]string{}
		for k, v := range options {
			runOptions[k] = v
		}
		if c.id == reports.DateCompressFALedgerID || c.id == reports.DateCompressItemLedgerID {
			runOptions["period_length"] = "4" // monthly entries: by year
		}
		if c.id == reports.DateCompressItemLedgerID {
			runCompression(t, src, reports.AdjustCostItemEntriesID, nil)
			before, totalBefore = c.count(), c.total()
		}
		runCompression(t, src, c.id, runOptions)
		after, totalAfter := c.count(), c.total()
		if totalAfter != totalBefore {
			t.Errorf("report %d: total %s after, %s before", c.id, totalAfter, totalBefore)
		}
		if after > before || before > 50 && after >= before {
			t.Errorf("report %d: %d entries after, %d before", c.id, after, before)
		}
		if c.register && before > 0 {
			if dateComprReg.Count() == registersBefore {
				t.Errorf("report %d: no Date Compr. Register", c.id)
				continue
			}
			last := lastComprRegister(t, src)
			if last.Table_id != c.tableID || last.No_records_deleted == 0 {
				t.Errorf("report %d: register table %d, %d deleted", c.id, last.Table_id, last.No_records_deleted)
			}
		}
		t.Logf("report %d: %d → %d entries", c.id, before, after)
	}
	result := glcheck.Check(src, nil)
	if !result.Balanced() {
		t.Errorf("G/L Balance Check after the compressions: %d dates, %d transactions, %d gaps, %d register problems",
			len(result.Dates), len(result.Transactions), len(result.Gaps), len(result.RegisterList))
	}

	// the registers left without entries are deleted
	for _, id := range []int{reports.DeleteEmptyGLRegistersID, reports.DeleteEmptyItemRegistersID, reports.DeleteEmptyResRegistersID,
		reports.DeleteEmptyFARegistersID, reports.DeleteEmptyBOMRegistersID, reports.DeleteEmptyInsuranceRegistersID} {
		runCompression(t, src, id, nil)
	}
	glReg := gtables.NewGLRegister(src)
	for found := glReg.FindSet(); found; found = glReg.Next() {
		glEntry := gtables.NewGLEntry(src)
		glEntry.SetRange("entry_no", glReg.From_entry_no, glReg.To_entry_no)
		custLedgEntry := gtables.NewCustomerLedgerEntry(src)
		custLedgEntry.SetRange("entry_no", glReg.From_entry_no, glReg.To_entry_no)
		if glEntry.IsEmpty() && custLedgEntry.IsEmpty() && glReg.From_entry_no != 0 {
			t.Errorf("empty G/L register %d left", glReg.No)
		}
	}
}

// Date Compr. G/L Budget Entries works on any year: this year's budget by year — one entry per
// account, the totals the same; Delete Check Ledger Entries and Delete Phys. Inventory Ledger
// delete the closed year's entries
func TestDateComprGLBudgetAndDeletes(t *testing.T) {
	src := postingCompany(t)
	glBudgetEntry := gtables.NewGLBudgetEntry(src)
	before := glBudgetEntry.Count()
	_ = glBudgetEntry.CalcSums("amount")
	total := system.Round(glBudgetEntry.Amount, types.MustDecimal("0.01"))
	if before == 0 {
		t.Fatal("no demo budget entries")
	}
	first := gtables.NewGLBudgetEntry(src)
	first.SetCurrentKey("date")
	first.FindFirst()
	last := gtables.NewGLBudgetEntry(src)
	last.SetCurrentKey("date")
	last.FindLast()
	runCompression(t, src, reports.DateComprGLBudgetEntriesID, map[string]string{"starting_date": first.Date.String(), "ending_date": last.Date.String(), "period_length": "4"})
	glBudgetEntry.Reset()
	after := glBudgetEntry.Count()
	_ = glBudgetEntry.CalcSums("amount")
	if after >= before || !system.Round(glBudgetEntry.Amount, types.MustDecimal("0.01")).Equal(total) {
		t.Errorf("budget entries %d → %d, total %s → %s", before, after, total, glBudgetEntry.Amount)
	}

	options := map[string]string{"starting_date": "2025-01-01", "ending_date": "2025-12-31"}
	checkLedgEntry := gtables.NewCheckLedgerEntry(src)
	checkLedgEntry.SetRange("check_date", types.NewDate(2025, 1, 1), types.NewDate(2025, 12, 31))
	checkLedgEntry.SetFilter("entry_status", "<>1")
	checkLedgEntry.SetRange("open", true)
	openBefore := checkLedgEntry.Count()
	runCompression(t, src, reports.DeleteCheckLedgerEntriesID, options)
	// the closed checks of the closed year are deleted; the open ones (not reconciled) stay
	if n := checkLedgEntry.Count(); n != openBefore {
		t.Errorf("%d open check ledger entries left, %d before", n, openBefore)
	}
	checkLedgEntry.SetRange("open", false)
	if n := checkLedgEntry.Count(); n != 0 {
		t.Errorf("%d closed check ledger entries of the closed year left", n)
	}
	registersBefore := gtables.NewDateComprRegister(src).Count()
	physBefore := countIn(gtables.NewPhysInventoryLedgerEntry(src), "posting_date")
	runCompression(t, src, reports.DeletePhysInventoryLedgerID, options)
	if n := countIn(gtables.NewPhysInventoryLedgerEntry(src), "posting_date"); n != 0 {
		t.Errorf("%d phys. inventory ledger entries of the closed year left", n)
	}
	if physBefore > 0 {
		last := lastComprRegister(t, src)
		if gtables.NewDateComprRegister(src).Count() != registersBefore+1 || last.Table_id != gtables.PhysInventoryLedgerEntryTableID || last.No_records_deleted != physBefore {
			t.Errorf("Delete Phys. Inventory Ledger's register: table %d, %d deleted (want %d)", last.Table_id, last.No_records_deleted, physBefore)
		}
	}
}

// The guards of the deleting reports: never as a scheduled job; the G/L compression takes no
// filter; open and unrealized tax entries and open project entries are not compressed
func TestDateCompressionGuards(t *testing.T) {
	for _, id := range []int{reports.DateCompressGeneralLedgerID, reports.DateCompressTaxEntriesID, reports.DeleteCheckLedgerEntriesID, reports.DeletePhysInventoryLedgerID, reports.DateComprGLBudgetEntriesID} {
		if !reports.DeletesEntries(id) {
			t.Errorf("report %d must be marked DeletesEntries", id)
		}
	}
	if reports.DeletesEntries(reports.DeleteEmptyGLRegistersID) {
		t.Error("Delete Empty G/L Registers deletes no entries")
	}

	src := postingCompany(t)
	options := map[string]string{"starting_date": "2025-01-01", "ending_date": "2025-12-31", "period_length": "2"}
	glEntry := gtables.NewGLEntry(src)
	before := glEntry.Count()
	_, err := reports.Execute(reports.DateCompressGeneralLedgerID, src.GetDB(), src.GetCompany(), src.GetDBType(), reports.RunOptions{Options: options,
		Filters: map[string][]ftables.FieldFilter{"G_L_Entry": {{Field: "gl_account_no", Expression: "6000"}}}})
	if err == nil || glEntry.Count() != before {
		t.Fatalf("G/L compression with an account filter: %v, %d entries (%d before)", err, glEntry.Count(), before)
	}

	// entries of the closed year (the demo has none): a settled tax entry, an open one, a settled
	// one with unrealized tax still to realize; a closed and an open project entry
	for i, c := range []struct {
		closed     bool
		unrealized string
	}{{true, "0"}, {false, "0"}, {true, "25"}, {true, "0"}} {
		taxEntry := gtables.NewTaxEntry(src)
		taxEntry.Entry_no = 900001 + i
		taxEntry.Posting_date = types.NewDate(2025, 3, 10+i)
		taxEntry.Type = gtables.TaxEntry_Type.Sale
		taxEntry.Base = types.MustDecimal("-100")
		taxEntry.Amount = types.MustDecimal("-25")
		taxEntry.Closed = c.closed
		taxEntry.Remaining_unrealized_amount = types.MustDecimal(c.unrealized)
		if err := taxEntry.InsertOrError(false); err != nil {
			t.Fatal(err)
		}
	}
	project := gtables.NewProject(src)
	if !project.FindFirst() {
		t.Fatal("no demo project")
	}
	for i, open := range []bool{false, true, false} {
		projectLedgEntry := gtables.NewProjectLedgerEntry(src)
		projectLedgEntry.Entry_no = 900001 + i
		projectLedgEntry.Project_no = project.No
		projectLedgEntry.Posting_date = types.NewDate(2025, 3, 10+i)
		projectLedgEntry.Quantity = types.MustDecimal("1")
		projectLedgEntry.Open = open
		if open {
			projectLedgEntry.Remaining_amount = types.MustDecimal("10")
		}
		if err := projectLedgEntry.InsertOrError(false); err != nil {
			t.Fatal(err)
		}
	}
	openTax := func() int {
		taxEntry := gtables.NewTaxEntry(src)
		taxEntry.SetRange("closed", false)
		return taxEntry.Count()
	}
	openTaxBefore := openTax()
	unrealized := func() int {
		taxEntry := gtables.NewTaxEntry(src)
		taxEntry.SetFilter("remaining_unrealized_amount", "<>0")
		return taxEntry.Count()
	}
	unrealizedBefore := unrealized()
	settled := func() int {
		taxEntry := gtables.NewTaxEntry(src)
		taxEntry.SetRange("entry_no", 900001, 900004)
		return taxEntry.Count()
	}
	openProject := func() int {
		projectLedgEntry := gtables.NewProjectLedgerEntry(src)
		projectLedgEntry.SetRange("open", true)
		return projectLedgEntry.Count()
	}
	openProjectBefore := openProject()
	runCompression(t, src, reports.DateCompressTaxEntriesID, options)
	runCompression(t, src, reports.DateCompressProjectLedgerID, options)
	if openTax() != openTaxBefore || openTaxBefore == 0 {
		t.Errorf("open tax entries %d → %d", openTaxBefore, openTax())
	}
	if unrealized() != unrealizedBefore || unrealizedBefore == 0 {
		t.Errorf("tax entries with unrealized tax %d → %d", unrealizedBefore, unrealized())
	}
	if settled() != 2 {
		t.Errorf("%d of the 4 test tax entries left, want the open and the unrealized one (the two settled ones compressed)", settled())
	}
	if !gtables.NewProjectLedgerEntry(src).Get(900002) {
		t.Error("the open project entry was compressed")
	}
	if openProject() != openProjectBefore || openProjectBefore == 0 {
		t.Errorf("open project entries %d → %d", openProjectBefore, openProject())
	}
	t.Logf("open tax entries kept: %d, open project entries kept: %d", openTaxBefore, openProjectBefore)
}
