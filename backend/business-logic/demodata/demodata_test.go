package demodata

import (
	"database/sql"
	"errors"
	"fmt"
	"math"
	"strings"
	"testing"
	"time"

	"github.com/hansjlachmann/openerp/backend/business-logic/tables"
	"github.com/hansjlachmann/openerp/backend/foundation/database"
	ftables "github.com/hansjlachmann/openerp/backend/foundation/tables"
	"github.com/hansjlachmann/openerp/backend/foundation/types"
	gtables "github.com/hansjlachmann/openerp/backend/generated/tables"
	_ "github.com/mattn/go-sqlite3"
)

const testCompany = "demo01"

// newTestDB opens an in-memory SQLite DB with the tables demo data writes to.
func newTestDB(t *testing.T) *sql.DB {
	t.Helper()
	db, err := sql.Open("sqlite3", ":memory:")
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	// One connection: every connection to :memory: is a separate database
	db.SetMaxOpenConns(1)
	t.Cleanup(func() { _ = db.Close() })

	// every company table (the demo data and the triggers it runs touch many)
	if err := tables.CreateCompanyTables(db, testCompany, database.DBTypeSQLite); err != nil {
		t.Fatalf("create tables: %v", err)
	}
	return db
}

func count(t *testing.T, db *sql.DB, table string) int {
	t.Helper()
	var n int
	if err := db.QueryRow(`SELECT COUNT(*) FROM "` + testCompany + `$` + table + `"`).Scan(&n); err != nil {
		t.Fatalf("count %s: %v", table, err)
	}
	return n
}

var today = time.Date(2026, 10, 5, 0, 0, 0, 0, time.Local)

func TestParseSize(t *testing.T) {
	for in, want := range map[string]Size{"": Small, "small": Small, " SMALL ": Small, "Large": Large} {
		got, err := ParseSize(in)
		if err != nil || got != want {
			t.Errorf("ParseSize(%q) = %q, %v; want %q", in, got, err, want)
		}
	}
	var sizeErr *SizeError
	if _, err := ParseSize("HUGE"); !errors.As(err, &sizeErr) {
		t.Errorf("ParseSize(HUGE) error = %v, want SizeError", err)
	}
}

func TestCreateSmall(t *testing.T) {
	db := newTestDB(t)
	// The setup record exists already (its page was opened): the demo data fills it in
	var blank gtables.GeneralLedgerSetup
	blank.InitWithDBType(db, testCompany, database.DBTypeSQLite)
	if !blank.Insert(false) {
		t.Fatal("insert the blank setup record")
	}
	var lastPct int
	counts, err := Create(db, testCompany, database.DBTypeSQLite, Options{
		Size:     Small,
		Today:    today,
		Progress: func(pct int, _ Phase) { lastPct = pct },
	})
	if err != nil {
		t.Fatalf("Create: %v", err)
	}

	if counts.Countries != 3 || counts.PaymentTerms != 4 || counts.Customers != 20 || counts.MasterData != 621 || counts.GLEntries < 500 || counts.ResCapacityEntries < 2000 || counts.ResLedgerEntries < 500 {
		t.Errorf("counts = %+v, want 3 countries, 4 payment terms, 20 customers, 621 master data records, at least 500 G/L entries, 2000 capacity entries and 500 resource ledger entries", counts)
	}
	// The customers' comments, ship-to addresses and bank accounts; three fiscal years of
	// monthly periods and the next year's first, the oldest year closed
	if counts.CustomerData != 11 || counts.AccountingPeriods != 37 {
		t.Errorf("customer data %d, accounting periods %d, want 11 and 37", counts.CustomerData, counts.AccountingPeriods)
	}
	var shipTo gtables.ShipToAddress
	shipTo.InitWithDBType(db, testCompany, database.DBTypeSQLite)
	if !shipTo.GetByPK(types.Code("C00010"), types.Code("LAGER")) || shipTo.Name != "Fjordkraft Interiør AS" || shipTo.City != "Oslo" {
		t.Errorf("ship-to address C00010 LAGER: name %q, city %q", shipTo.Name, shipTo.City)
	}
	// Vendors with their ledger entries (Balance (LCY) is what is owed: the open invoices),
	// comments, order addresses and bank accounts; the VEND series continues after them
	if counts.Vendors != 8 || counts.VendorLedgerEntries < 8*12 || counts.VendorData != 22 {
		t.Errorf("vendors %d, vendor ledger entries %d, vendor data %d", counts.Vendors, counts.VendorLedgerEntries, counts.VendorData)
	}
	var vend gtables.Vendor
	vend.InitWithDBType(db, testCompany, database.DBTypeSQLite)
	if !vend.Get(types.Code("V00010")) || vend.Vendor_posting_group != "DOMESTIC" {
		t.Fatalf("vendor V00010: %q", vend.Vendor_posting_group)
	}
	vend.CalcFields("balance_lcy", "purchases_lcy", "comment")
	if !vend.Balance_lcy.IsPositive() || !vend.Purchases_lcy.IsPositive() || !vend.Comment {
		t.Errorf("V00010 balance %s, purchases %s, comment %v", vend.Balance_lcy, vend.Purchases_lcy, vend.Comment)
	}
	var vendSeries gtables.NoSeriesLine
	vendSeries.InitWithDBType(db, testCompany, database.DBTypeSQLite)
	if !vendSeries.GetByPK(types.Code("VEND"), 10000) || vendSeries.Last_no_used != "V00080" {
		t.Errorf("VEND last no. used %q", vendSeries.Last_no_used)
	}
	// Bank: every demo payment has its bank entry; B010 holds the balance of the customer and
	// vendor payments in kroner, older months reconciled with a statement
	if counts.BankLedgerEntries == 0 {
		t.Error("no bank ledger entries")
	}
	var bankAcc gtables.BankAccount
	bankAcc.InitWithDBType(db, testCompany, database.DBTypeSQLite)
	if !bankAcc.Get(types.Code("B010")) {
		t.Fatal("bank account B010")
	}
	bankAcc.CalcFields("balance")
	if bankAcc.Balance.IsZero() || bankAcc.Last_statement_no == "" || bankAcc.Balance_last_statement.IsZero() {
		t.Errorf("B010 balance %s, last statement %q %s", bankAcc.Balance, bankAcc.Last_statement_no, bankAcc.Balance_last_statement)
	}
	var bankSeries gtables.NoSeriesLine
	bankSeries.InitWithDBType(db, testCompany, database.DBTypeSQLite)
	if !bankSeries.GetByPK(types.Code("BANK"), 10000) || bankSeries.Last_no_used != "B030" {
		t.Errorf("BANK last no. used %q", bankSeries.Last_no_used)
	}
	// Fixed assets: the COMPANY book with the acquisition less the monthly depreciation, a main
	// asset with its component, the insured assets' coverage; FA series continues after FA0080
	if counts.FixedAssets != 8 || counts.FALedgerEntries <= 8 {
		t.Errorf("fixed assets %d, FA ledger entries %d", counts.FixedAssets, counts.FALedgerEntries)
	}
	var faBook gtables.FADepreciationBook
	faBook.InitWithDBType(db, testCompany, database.DBTypeSQLite)
	if !faBook.GetByPK(types.Code("FA0010"), types.Code("COMPANY")) {
		t.Fatal("FA0010 COMPANY book")
	}
	faBook.CalcFields("acquisition_cost", "depreciation", "book_value")
	if !faBook.Acquisition_cost.Equal(types.NewDecimal(485000)) || !faBook.Depreciation.IsNegative() ||
		!faBook.Book_value.Equal(faBook.Acquisition_cost.Add(faBook.Depreciation)) || !faBook.No_of_depreciation_months.Equal(types.NewDecimal(60)) {
		t.Errorf("FA0010: cost %s, depreciation %s, book value %s, months %s", faBook.Acquisition_cost, faBook.Depreciation, faBook.Book_value, faBook.No_of_depreciation_months)
	}
	var mainAsset gtables.FixedAsset
	mainAsset.InitWithDBType(db, testCompany, database.DBTypeSQLite)
	if !mainAsset.Get(types.Code("FA0070")) || mainAsset.Main_asset_component != gtables.FixedAsset_Main_asset_component.MainAsset {
		t.Errorf("FA0070 main asset/component %v", mainAsset.Main_asset_component)
	}
	var insurance gtables.Insurance
	insurance.InitWithDBType(db, testCompany, database.DBTypeSQLite)
	if !insurance.Get(types.Code("INS010")) {
		t.Fatal("insurance INS010")
	}
	insurance.CalcFields("total_value_insured")
	if !insurance.Total_value_insured.Equal(types.NewDecimal(485000 + 520000 + 265000 + 212000)) {
		t.Errorf("total value insured %s", insurance.Total_value_insured)
	}
	var closed int
	if err := db.QueryRow(`SELECT COUNT(*) FROM "` + testCompany + `$Accounting_Period" WHERE closed`).Scan(&closed); err != nil || closed != 12 {
		t.Errorf("%d closed periods, want 12 (%v)", closed, err)
	}
	// The number series continue after the demo customers and posted documents
	for series, prefix := range map[string]string{"CUST": "C00", "S-INV+": "SI0", "S-CR+": "SCM0"} {
		var line gtables.NoSeriesLine
		line.InitWithDBType(db, testCompany, database.DBTypeSQLite)
		if !line.GetByPK(types.Code(series), 10000) || !strings.HasPrefix(line.Last_no_used.String(), prefix) {
			t.Errorf("%s line = %+v", series, line.ToMap())
		}
	}
	var sales gtables.SalesReceivablesSetup
	sales.InitWithDBType(db, testCompany, database.DBTypeSQLite)
	if !sales.Get("") || sales.Customer_nos != "CUST" || sales.Posted_invoice_nos != "S-INV+" || !sales.Stockout_warning {
		t.Errorf("Sales & Receivables Setup = %+v", sales.ToMap())
	}
	// Master data: given fields and the tables' defaults for the others
	var eur gtables.Currency
	eur.InitWithDBType(db, testCompany, database.DBTypeSQLite)
	if !eur.Get("EUR") || eur.Description != "Euro" || eur.Unit_amount_rounding_precision.String() != "0.00001" || eur.Amount_decimal_places != "2:2" {
		t.Errorf("EUR = %+v", eur.ToMap())
	}
	var retail gtables.PriceGroup
	retail.InitWithDBType(db, testCompany, database.DBTypeSQLite)
	if !retail.Get("RETAIL") || retail.Allow_invoice_disc || !retail.Allow_quantity_disc {
		t.Errorf("RETAIL = %+v", retail.ToMap())
	}
	var dk gtables.CountryRegion
	dk.InitWithDBType(db, testCompany, database.DBTypeSQLite)
	if !dk.Get("DK") || dk.Eu_country_code != "DK" || dk.Address_format != gtables.CountryRegion_Address_format.CityPostCode {
		t.Errorf("DK = %+v", dk.ToMap())
	}
	// The setup record (already created by opening its page) and the exchange rates
	var setup gtables.GeneralLedgerSetup
	setup.InitWithDBType(db, testCompany, database.DBTypeSQLite)
	if !setup.Get("") || setup.Lcy_code != "NOK" || setup.Amount_decimal_places != "2:2" {
		t.Errorf("General Ledger Setup = %+v", setup.ToMap())
	}
	// The chart of accounts is indented, End-Totals total their range
	var acc gtables.GLAccount
	acc.InitWithDBType(db, testCompany, database.DBTypeSQLite)
	if !acc.Get("1210") || acc.Indentation != 2 || !acc.Get("1299") || acc.Totaling != "1200..1299" || acc.Indentation != 1 || !acc.Get("1000") || acc.Indentation != 0 {
		t.Errorf("chart of accounts not indented: %+v", acc.ToMap())
	}
	// G/L totals: every demo transaction balances, so the income statement (NET INCOME, a Total
	// over 3000..6999) and the two sides of the balance sheet add up to zero; the totals on
	// End-Total and Total accounts follow their Totaling
	total := func(no string) types.Decimal {
		var a gtables.GLAccount
		a.InitWithDBType(db, testCompany, database.DBTypeSQLite)
		if !a.Get(no) {
			t.Fatalf("account %s missing", no)
		}
		a.CalcFields("balance")
		return a.Balance
	}
	net, assets, liabilities := total("8999"), total("1999"), total("2999")
	cents := func(d types.Decimal) string { return d.Decimal.Round(2).String() } // SQLite sums in floating point
	if net.IsZero() || assets.IsZero() || cents(net.Add(assets).Add(liabilities)) != "0" {
		t.Errorf("net income %v + total assets %v + total liabilities and equity %v, want 0", net, assets, liabilities)
	}
	if gross := total("4900"); cents(gross) != cents(total("3199").Add(total("4199"))) {
		t.Errorf("gross profit %v, want revenue + cost of goods sold", gross)
	}
	if bank := total("1520"); bank.IsZero() {
		t.Error("no bank balance")
	}
	if !net.IsNegative() {
		t.Errorf("the demo company makes a loss: net income %v (a profit is a credit, negative)", net)
	}
	if eur.Realized_gains_acc != "6140" {
		t.Errorf("EUR realized gains account = %q", eur.Realized_gains_acc)
	}
	factor, err := tables.ExchangeRates(&eur).ExchangeRate(types.Today(), "EUR")
	if err != nil || factor.Decimal.Round(6).String() != "0.086957" {
		t.Errorf("EUR exchange rate today = %v %v", factor, err)
	}
	if counts.LedgerEntries < 20*6 {
		t.Errorf("only %d ledger entries", counts.LedgerEntries)
	}
	if got := count(t, db, "Customer"); got != 20 {
		t.Errorf("Customer rows = %d, want 20", got)
	}
	if got := count(t, db, "Customer Ledger Entry"); got != counts.LedgerEntries {
		t.Errorf("ledger rows = %d, want %d", got, counts.LedgerEntries)
	}
	if lastPct != 100 {
		t.Errorf("last progress = %d, want 100", lastPct)
	}

	// Mixed countries, with the country's own address and phone format
	var cust gtables.Customer
	cust.InitWithDBType(db, testCompany, database.DBTypeSQLite)
	if !cust.Get("C00080") {
		t.Fatal("customer C00080 not found")
	}
	if cust.Country_code.String() != "DK" || cust.Name.String() != "Nørrebro Møbler ApS" || cust.Search_name != "NØRREBRO MØBLER APS" {
		t.Errorf("C00080 = %s / %s / %s", cust.Country_code, cust.Name, cust.Search_name)
	}
	// A customer abroad: foreign posting groups and reminder terms, Nordic territory
	if cust.Customer_posting_group != "FOREIGN" || cust.Gen_bus_posting_group != "EXPORT" || cust.Tax_bus_posting_group != "EXPORT" ||
		cust.Reminder_terms_code != "FOREIGN" || cust.Territory_code != "NORDIC" || cust.Invoice_disc_code != "C00080" || cust.Blocked {
		t.Errorf("C00080 = %+v", cust.ToMap())
	}
	if !cust.Get("C00010") || cust.Customer_posting_group != "DOMESTIC" || cust.Territory_code != "EAST" || cust.Salesperson_code == "" {
		t.Errorf("C00010 = %+v", cust.ToMap())
	}
	if !cust.Get("C00070") || !cust.Blocked {
		t.Errorf("C00070 blocked = %v, want Yes", cust.Blocked)
	}

	// Balance = open invoice amounts; every closed invoice is settled by a payment
	var open, unbalanced int
	if err := db.QueryRow(`SELECT COUNT(*) FROM "demo01$Customer Ledger Entry" WHERE open = 1`).Scan(&open); err != nil {
		t.Fatal(err)
	}
	if open == 0 {
		t.Error("expected some open invoices")
	}
	if err := db.QueryRow(`SELECT COUNT(*) FROM "demo01$Customer Ledger Entry" WHERE open = 0 AND remaining_amt_lcy <> 0`).Scan(&unbalanced); err != nil {
		t.Fatal(err)
	}
	if unbalanced != 0 {
		t.Errorf("%d closed entries still have a remaining amount", unbalanced)
	}
}

func TestCreateIsRepeatable(t *testing.T) {
	sum := func() (int, float64) {
		db := newTestDB(t)
		counts, err := Create(db, testCompany, database.DBTypeSQLite, Options{Today: today})
		if err != nil {
			t.Fatalf("Create: %v", err)
		}
		var total float64
		if err := db.QueryRow(`SELECT SUM(amount) FROM "demo01$Customer Ledger Entry"`).Scan(&total); err != nil {
			t.Fatal(err)
		}
		return counts.LedgerEntries, total
	}
	n1, t1 := sum()
	n2, t2 := sum()
	if n1 != n2 || t1 != t2 {
		t.Errorf("runs differ: %d entries / %.2f vs %d entries / %.2f", n1, t1, n2, t2)
	}
}

func TestCreateRefusesCompanyWithCustomers(t *testing.T) {
	db := newTestDB(t)
	if _, err := Create(db, testCompany, database.DBTypeSQLite, Options{Today: today}); err != nil {
		t.Fatalf("first Create: %v", err)
	}
	entries := count(t, db, "Customer Ledger Entry")

	_, err := Create(db, testCompany, database.DBTypeSQLite, Options{Today: today})
	if !errors.Is(err, ErrCompanyNotEmpty) {
		t.Fatalf("second Create error = %v, want ErrCompanyNotEmpty", err)
	}
	if got := count(t, db, "Customer Ledger Entry"); got != entries {
		t.Errorf("second run changed the ledger: %d -> %d entries", entries, got)
	}
}

func TestCreateKeepsExistingSetup(t *testing.T) {
	db := newTestDB(t)
	var pt gtables.PaymentTerms
	pt.InitWithDBType(db, testCompany, database.DBTypeSQLite)
	pt.Code = "30 DAYS"
	pt.Description = "Our own 30 days"
	if !pt.Insert(true) {
		t.Fatal("insert payment terms")
	}

	counts, err := Create(db, testCompany, database.DBTypeSQLite, Options{Today: today})
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	if counts.PaymentTerms != 3 {
		t.Errorf("PaymentTerms inserted = %d, want 3 (one existed)", counts.PaymentTerms)
	}
	pt.Get("30 DAYS")
	if pt.Description.String() != "Our own 30 days" {
		t.Errorf("existing payment terms overwritten: %q", pt.Description)
	}
}

func TestGenerateLargeCustomers(t *testing.T) {
	var gen generatorFile
	var setup setupData
	if err := loadYAML("generator.yaml", &gen); err != nil {
		t.Fatal(err)
	}
	if err := loadYAML("setup.yaml", &setup); err != nil {
		t.Fatal(err)
	}
	customers := generateCustomers(newRand(), gen, setup, largeCustomerCount-20)
	if len(customers) != largeCustomerCount-20 {
		t.Fatalf("generated %d customers", len(customers))
	}
	seen := map[string]bool{}
	perCountry := map[string]int{}
	for _, c := range customers {
		if seen[c.No] {
			t.Fatalf("duplicate customer no %s", c.No)
		}
		seen[c.No] = true
		perCountry[c.Country]++
		if len([]rune(c.Name)) > 50 || len([]rune(c.Address)) > 50 || len(c.Phone) > 30 {
			t.Fatalf("value too long for its field: %+v", c)
		}
	}
	if customers[0].No != "C00201" || len(perCountry) != 3 {
		t.Errorf("first no = %s, countries = %v", customers[0].No, perCountry)
	}
}

// CalcFieldsForRecords (list pages) must give the same FlowField values as CalcFields
// per record (card page), including records without ledger entries and key lists
// longer than one query chunk.
func TestCalcFieldsForRecordsMatchesCalcFields(t *testing.T) {
	db := newTestDB(t)
	if _, err := Create(db, testCompany, database.DBTypeSQLite, Options{Today: today}); err != nil {
		t.Fatalf("Create: %v", err)
	}

	var cust gtables.Customer
	cust.InitWithDBType(db, testCompany, database.DBTypeSQLite)
	var records []map[string]interface{}
	if cust.FindSet() {
		records = append(records, cust.ToMap())
		for cust.Next() {
			records = append(records, cust.ToMap())
		}
	}
	if len(records) != 20 {
		t.Fatalf("read %d customers, want 20", len(records))
	}
	// Expected values: CalcFields per record, after the result set is closed (the
	// test DB has a single connection)
	want := map[string]map[string]interface{}{}
	for _, rec := range records {
		var one gtables.Customer
		one.InitWithDBType(db, testCompany, database.DBTypeSQLite)
		no := rec["no"].(string)
		if !one.Get(no) {
			t.Fatalf("customer %s not found", no)
		}
		one.CalcFields()
		calc := one.ToMap()
		want[no] = map[string]interface{}{
			"balance_lcy": calc["balance_lcy"], "sales_lcy": calc["sales_lcy"], "no_of_ledger_entries": calc["no_of_ledger_entries"],
		}
	}
	// Few keys: one grouped query with an IN list
	few := make([]map[string]interface{}, len(records))
	for i, rec := range records {
		few[i] = map[string]interface{}{"no": rec["no"]}
	}
	cust.CalcFieldsForRecords(few)
	for _, rec := range few {
		no := rec["no"].(string)
		for field, w := range want[no] {
			if fmt.Sprint(rec[field]) != fmt.Sprint(w) {
				t.Errorf("IN list: %s.%s = %v, want %v", no, field, rec[field], w)
			}
		}
	}

	// Many keys (more than one chunk, incl. customers without entries): one grouped
	// query over the whole ledger
	for i := 0; i < 1200; i++ {
		no := fmt.Sprintf("X%05d", i)
		records = append(records, map[string]interface{}{"no": no})
		want[no] = map[string]interface{}{"balance_lcy": "0", "sales_lcy": "0", "no_of_ledger_entries": 0}
	}

	cust.CalcFieldsForRecords(records)

	for _, rec := range records {
		no := rec["no"].(string)
		for field, w := range want[no] {
			if fmt.Sprint(rec[field]) != fmt.Sprint(w) {
				t.Errorf("%s.%s = %v, want %v", no, field, rec[field], w)
			}
		}
	}
	if fmt.Sprint(want["C00010"]["no_of_ledger_entries"]) == "0" {
		t.Error("C00010 should have ledger entries")
	}
}

func TestCalcFieldsForRecordsOnlyRequested(t *testing.T) {
	db := newTestDB(t)
	if _, err := Create(db, testCompany, database.DBTypeSQLite, Options{Today: today}); err != nil {
		t.Fatalf("Create: %v", err)
	}
	var cust gtables.Customer
	cust.InitWithDBType(db, testCompany, database.DBTypeSQLite)
	records := []map[string]interface{}{{"no": "C00010"}}
	cust.CalcFieldsForRecords(records, "sales_lcy")
	if _, ok := records[0]["sales_lcy"]; !ok {
		t.Error("sales_lcy not calculated")
	}
	if _, ok := records[0]["balance_lcy"]; ok {
		t.Error("balance_lcy calculated although not requested")
	}
}

// Customer FlowFields read SIFT totals (key customer_open of Customer Ledger Entry). They
// must equal sums over the entries — also after entries are changed through the table API
// (Modify closes an invoice, Delete removes one).
func TestFlowFieldsFromSIFTMatchEntries(t *testing.T) {
	db := newTestDB(t)
	if _, err := Create(db, testCompany, database.DBTypeSQLite, Options{Today: today}); err != nil {
		t.Fatalf("Create: %v", err)
	}
	const entries = `"demo01$Customer Ledger Entry"`

	check := func(label string) {
		t.Helper()
		rows, err := db.Query(`SELECT c.no,
			(SELECT COALESCE(SUM(CAST(remaining_amt_lcy AS REAL)), 0) FROM ` + entries + ` e WHERE e.customer_no = c.no AND e.open = 1),
			(SELECT COALESCE(SUM(CAST(sales_lcy AS REAL)), 0) FROM ` + entries + ` e WHERE e.customer_no = c.no),
			(SELECT COUNT(*) FROM ` + entries + ` e WHERE e.customer_no = c.no)
			FROM "demo01$Customer" c`)
		if err != nil {
			t.Fatal(err)
		}
		want := map[string][3]float64{}
		for rows.Next() {
			var no string
			var balance, sales, count float64
			if err := rows.Scan(&no, &balance, &sales, &count); err != nil {
				t.Fatal(err)
			}
			want[no] = [3]float64{balance, sales, count}
		}
		_ = rows.Close()

		for no, w := range want {
			var cust gtables.Customer
			cust.InitWithDBType(db, testCompany, database.DBTypeSQLite)
			if !cust.Get(no) {
				t.Fatalf("customer %s not found", no)
			}
			cust.CalcFields()
			got := [3]float64{cust.Balance_due_lcy.Float64(), cust.Sales_lcy.Float64(), float64(cust.No_of_ledger_entries)}
			for i := range got {
				if math.Abs(got[i]-w[i]) > 0.005 {
					t.Fatalf("%s: %s FlowFields (balance due, sales, count) = %v, want %v", label, no, got, w)
				}
			}
		}
	}
	check("after demo data")

	// Close the first open invoice through the table API (moves it to the closed totals)
	var entry gtables.CustomerLedgerEntry
	entry.InitWithDBType(db, testCompany, database.DBTypeSQLite)
	entry.SetRange("open", true)
	if !entry.FindFirst() {
		t.Fatal("no open entry")
	}
	entry.Open = false
	entry.Remaining_amt_lcy = types.NewDecimal(0)
	entry.Remaining_amount = types.NewDecimal(0)
	if !entry.Modify(true) {
		t.Fatal("Modify failed")
	}
	check("after closing an invoice")

	// Delete an entry
	entry.Reset()
	if !entry.FindLast() || !entry.Delete(true) {
		t.Fatal("Delete failed")
	}
	check("after deleting an entry")

	// The totals table is what the FlowFields read: it holds at most 2 rows per customer
	var totalsRows int
	if err := db.QueryRow(`SELECT COUNT(*) FROM "demo01$Customer Ledger Entry$SIFT$customer_open"`).Scan(&totalsRows); err != nil {
		t.Fatal(err)
	}
	if totalsRows == 0 || totalsRows > 40 {
		t.Errorf("totals rows = %d, want 1..40 for 20 customers", totalsRows)
	}
}

// Customer Date Filter (FlowFilter) applied to Posting Date: FlowFields with a date filter
// read the customer_open_date totals and must equal sums over the entries in that period,
// on the card (CalcFields) and in the list (CalcFieldsForRecords); without a filter they
// are unchanged.
func TestDateFilterFlowFields(t *testing.T) {
	db := newTestDB(t)
	if _, err := Create(db, testCompany, database.DBTypeSQLite, Options{Today: today}); err != nil {
		t.Fatalf("Create: %v", err)
	}
	const entries = `"demo01$Customer Ledger Entry"`
	expected := func(no, cond string, args ...interface{}) [3]float64 {
		t.Helper()
		var balance, sales, count float64
		q := `SELECT
			(SELECT COALESCE(SUM(CAST(amount_lcy AS REAL)), 0) FROM ` + entries + ` WHERE customer_no = ? AND ` + cond + `),
			(SELECT COALESCE(SUM(CAST(sales_lcy AS REAL)), 0) FROM ` + entries + ` WHERE customer_no = ? AND ` + cond + `),
			(SELECT COUNT(*) FROM ` + entries + ` WHERE customer_no = ? AND ` + cond + `)`
		all := append(append(append([]interface{}{no}, args...), append([]interface{}{no}, args...)...), append([]interface{}{no}, args...)...)
		if err := db.QueryRow(q, all...).Scan(&balance, &sales, &count); err != nil {
			t.Fatal(err)
		}
		return [3]float64{balance, sales, count}
	}
	same := func(label string, got, want [3]float64) {
		t.Helper()
		for i := range got {
			if math.Abs(got[i]-want[i]) > 0.005 {
				t.Errorf("%s: (net change, sales, count) = %v, want %v", label, got, want)
				return
			}
		}
	}

	cases := []struct {
		filter string
		cond   string
		args   []interface{}
	}{
		{"", "1=1", nil},
		{"2026-04-01..2026-06-30", "posting_date BETWEEN ? AND ?", []interface{}{"2026-04-01", "2026-06-30"}},
		{"..2026-03-31", "posting_date <= ?", []interface{}{"2026-03-31"}},
		{"2026-09-01..", "posting_date >= ?", []interface{}{"2026-09-01"}},
		{"2026-01-01..2026-01-31|2026-08-01..2026-08-31", "(posting_date BETWEEN ? AND ? OR posting_date BETWEEN ? AND ?)", []interface{}{"2026-01-01", "2026-01-31", "2026-08-01", "2026-08-31"}},
	}
	for _, tc := range cases {
		// Card path
		for _, no := range []string{"C00010", "C00080", "C00150"} {
			var cust gtables.Customer
			cust.InitWithDBType(db, testCompany, database.DBTypeSQLite)
			if !cust.Get(no) {
				t.Fatalf("customer %s not found", no)
			}
			if err := cust.SetFlowFilter("date_filter", tc.filter); err != nil {
				t.Fatalf("SetFlowFilter(%q): %v", tc.filter, err)
			}
			cust.CalcFields()
			got := [3]float64{cust.Net_change_lcy.Float64(), cust.Sales_lcy.Float64(), float64(cust.No_of_ledger_entries)}
			same("card "+no+" filter "+tc.filter, got, expected(no, tc.cond, tc.args...))
		}

		// List path (all 20 customers at once)
		var list gtables.Customer
		list.InitWithDBType(db, testCompany, database.DBTypeSQLite)
		if err := list.SetFlowFilter("date_filter", tc.filter); err != nil {
			t.Fatal(err)
		}
		var records []map[string]interface{}
		if list.FindSet() {
			records = append(records, list.ToMap())
			for list.Next() {
				records = append(records, list.ToMap())
			}
		}
		list.CalcFieldsForRecords(records)
		for _, rec := range records {
			no := rec["no"].(string)
			var got [3]float64
			for i, field := range []string{"net_change_lcy", "sales_lcy", "no_of_ledger_entries"} {
				if _, err := fmt.Sscan(fmt.Sprint(rec[field]), &got[i]); err != nil {
					t.Fatalf("%s %s = %v: %v", no, field, rec[field], err)
				}
			}
			same("list "+no+" filter "+tc.filter, got, expected(no, tc.cond, tc.args...))
		}
	}

	// Invalid expressions and unknown FlowFilter fields are rejected
	var cust gtables.Customer
	cust.InitWithDBType(db, testCompany, database.DBTypeSQLite)
	for _, bad := range []string{"31.03.26", "2026-13-01..", "yesterday"} {
		if err := cust.SetFlowFilter("date_filter", bad); err == nil {
			t.Errorf("SetFlowFilter(%q) accepted", bad)
		}
	}
	if err := cust.SetFlowFilter("name", "x"); err == nil {
		t.Error("SetFlowFilter on a normal field accepted")
	}
}

// Renaming a customer (changing its No.) carries the new No. over to its Customer Ledger
// Entries (Rename) — Customer No. and Sell-to Customer No. — and the SIFT totals move
// with them. A rename rolled back with its transaction leaves everything as it was.
func TestRenameCustomerUpdatesLedgerEntries(t *testing.T) {
	db := newTestDB(t)
	if _, err := Create(db, testCompany, database.DBTypeSQLite, Options{Today: today}); err != nil {
		t.Fatalf("Create: %v", err)
	}
	count := func(where string, args ...interface{}) int {
		t.Helper()
		var n int
		if err := db.QueryRow(`SELECT COUNT(*) FROM "demo01$Customer Ledger Entry" WHERE `+where, args...).Scan(&n); err != nil {
			t.Fatal(err)
		}
		return n
	}
	balance := func(no string) float64 {
		t.Helper()
		var c gtables.Customer
		c.InitWithDBType(db, testCompany, database.DBTypeSQLite)
		if !c.Get(no) {
			t.Fatalf("customer %s not found", no)
		}
		c.CalcFields()
		return c.Balance_lcy.Float64()
	}
	entries := count("customer_no = ?", "C00010")
	sellTo := count("sell_to_customer_no = ?", "C00010")
	wantBalance := balance("C00010")
	if entries == 0 {
		t.Fatal("C00010 has no entries")
	}

	rename := func(from, to string) *gtables.Customer {
		var c gtables.Customer
		c.InitWithDBType(db, testCompany, database.DBTypeSQLite)
		if !c.Get(from) {
			t.Fatalf("customer %s not found", from)
		}
		c.No = types.NewCode(to)
		return &c
	}

	// Rolled back: nothing changes (load the record first: the test DB has one connection,
	// which the transaction then holds)
	c := rename("C00010", "C99999")
	tx, err := db.Begin()
	if err != nil {
		t.Fatal(err)
	}
	c.SetDB(tx)
	if !c.Modify(true) {
		t.Fatal("Modify in transaction failed")
	}
	_ = tx.Rollback()
	if count("customer_no = ?", "C00010") != entries || count("customer_no = ?", "C99999") != 0 {
		t.Fatal("rolled-back rename changed the ledger entries")
	}

	// Committed rename: entries and totals follow
	c = rename("C00010", "C99999")
	if !c.Modify(true) {
		t.Fatalf("rename failed: %v", c.TriggerError())
	}
	if got := count("customer_no = ?", "C99999"); got != entries {
		t.Errorf("entries on C99999 = %d, want %d", got, entries)
	}
	if got := count("sell_to_customer_no = ?", "C99999"); got != sellTo {
		t.Errorf("sell-to entries on C99999 = %d, want %d", got, sellTo)
	}
	if got := count("customer_no = ? OR sell_to_customer_no = ?", "C00010", "C00010"); got != 0 {
		t.Errorf("%d entries still refer to C00010", got)
	}
	if got := balance("C99999"); math.Abs(got-wantBalance) > 0.005 {
		t.Errorf("balance after rename = %.2f, want %.2f", got, wantBalance)
	}
	var totalsRows int
	if err := db.QueryRow(`SELECT COUNT(*) FROM "demo01$Customer Ledger Entry$SIFT$customer_open" WHERE customer_no = 'C00010'`).Scan(&totalsRows); err != nil || totalsRows != 0 {
		t.Errorf("SIFT totals still on C00010 (%d rows, %v)", totalsRows, err)
	}
}

// HEAVY: 200 customers with about 1,000 entries each over two years (generation only — the
// insert is measured on Postgres, not in unit tests).
func TestGenerateHeavy(t *testing.T) {
	var gen generatorFile
	var setup setupData
	var small customersFile
	for name, out := range map[string]interface{}{"generator.yaml": &gen, "setup.yaml": &setup, "customers.yaml": &small} {
		if err := loadYAML(name, out); err != nil {
			t.Fatal(err)
		}
	}
	rng := newRand()
	customers := append(append([]customerData{}, small.Customers...), generateCustomers(rng, gen, setup, heavyCustomerCount-len(small.Customers))...)
	dueDays := map[string]int{}
	for _, pt := range setup.PaymentTerms {
		dueDays[pt.Code] = pt.DueDays
	}
	entries := generateEntries(rng, customers, dueDays, today, Heavy)
	if len(customers) != 200 {
		t.Fatalf("%d customers", len(customers))
	}
	perCustomer := len(entries) / len(customers)
	if perCustomer < 800 || perCustomer > 1100 {
		t.Errorf("%d entries per customer, want about 1,000", perCustomer)
	}
	oldest := today
	for _, e := range entries {
		if e.date.Before(oldest) {
			oldest = e.date
		}
	}
	if span := today.Sub(oldest).Hours() / 24; span < 700 {
		t.Errorf("entries span %.0f days, want about 730", span)
	}
	if size, err := ParseSize("heavy"); err != nil || size != Heavy {
		t.Errorf("ParseSize(heavy) = %v, %v", size, err)
	}
	t.Logf("HEAVY: %d customers, %d entries", len(customers), len(entries))
}

// InsertAll runs every record's OnInsert trigger before writing: a failing trigger stops the
// batch with the record's index and nothing of it is written.
func TestInsertAllRunsTriggers(t *testing.T) {
	db := newTestDB(t)
	newEntry := func(no int, description string) *gtables.CustomerLedgerEntry {
		rec := &gtables.CustomerLedgerEntry{}
		rec.InitWithDBType(db, testCompany, database.DBTypeSQLite)
		rec.Entry_no = no
		rec.Customer_no = types.NewCode("C00010")
		rec.Description = types.NewText(description)
		return rec
	}
	var target gtables.CustomerLedgerEntry
	target.InitWithDBType(db, testCompany, database.DBTypeSQLite)

	tooLong := strings.Repeat("x", 101) // field check after OnInsert: description max. 100
	err := target.InsertAll([]*gtables.CustomerLedgerEntry{newEntry(1, "a"), newEntry(2, tooLong), newEntry(3, "c")}, true)
	var batchErr *ftables.BatchInsertError
	if !errors.As(err, &batchErr) || batchErr.Index != 1 || !batchErr.Trigger {
		t.Fatalf("error %v, want a trigger BatchInsertError at index 1", err)
	}
	if n := count(t, db, gtables.CustomerLedgerEntryTableName); n != 0 {
		t.Fatalf("%d entries written after a failed trigger, want 0", n)
	}

	if err := target.InsertAll([]*gtables.CustomerLedgerEntry{newEntry(1, "a"), newEntry(2, "b")}, true); err != nil {
		t.Fatal(err)
	}
	if n := count(t, db, gtables.CustomerLedgerEntryTableName); n != 2 {
		t.Fatalf("%d entries, want 2", n)
	}
}
