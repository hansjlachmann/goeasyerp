package reports_test

import (
	"bytes"
	"database/sql"
	"strings"
	"testing"
	"time"

	"github.com/hansjlachmann/openerp/backend/business-logic/demodata"
	"github.com/hansjlachmann/openerp/backend/business-logic/itemjnl"
	"github.com/hansjlachmann/openerp/backend/business-logic/reports"
	"github.com/hansjlachmann/openerp/backend/business-logic/tables"
	"github.com/hansjlachmann/openerp/backend/foundation/database"
	"github.com/hansjlachmann/openerp/backend/foundation/format"
	ftables "github.com/hansjlachmann/openerp/backend/foundation/tables"
	"github.com/hansjlachmann/openerp/backend/foundation/types"
	gtables "github.com/hansjlachmann/openerp/backend/generated/tables"
)

// listCompany: the demo data with the items' postings, in a transaction (as the API runs a report)
func listCompany(t *testing.T) ftables.Source {
	t.Helper()
	db, err := sql.Open("sqlite3", ":memory:")
	mustOK(t, err)
	db.SetMaxOpenConns(1)
	t.Cleanup(func() { _ = db.Close() })
	mustOK(t, tables.CreateCompanyTables(db, "demo", database.DBTypeSQLite))
	_, err = demodata.Create(db, "demo", database.DBTypeSQLite, demodata.Options{
		Today:           time.Date(2026, 10, 6, 0, 0, 0, 0, time.Local),
		ItemJnlPostLine: func(src ftables.Source) demodata.ItemJnlPoster { return itemjnl.NewItemJnlPostLine(src) },
	})
	mustOK(t, err)
	tx, err := ftables.BeginTransaction(db)
	mustOK(t, err)
	t.Cleanup(func() { _ = ftables.Rollback(tx) })
	return ftables.DBSource(tx, "demo", database.DBTypeSQLite)
}

func runList(t *testing.T, src ftables.Source, id int, options map[string]string) string {
	t.Helper()
	result, err := reports.Execute(id, src.GetDB(), src.GetCompany(), src.GetDBType(), reports.RunOptions{Output: reports.OutputPDF, Options: options})
	if err != nil {
		t.Fatalf("report %d: %v", id, err)
	}
	if !bytes.HasPrefix(result.PDF, []byte("%PDF-")) || result.Pages == 0 {
		t.Fatalf("report %d: no printout", id)
	}
	return printedTexts(result, 0)
}

func amountText(d types.Decimal) string { return format.Decimal(d, "2:2", "en-US") }

// Every list report of the main menu prints from the demo data: the accounts, entries and
// items it lists, and totals that agree with the ledgers
func TestListReports(t *testing.T) {
	src := listCompany(t)

	cases := []struct {
		id      int
		options map[string]string
		want    []string
	}{
		{reports.ChartOfAccountsID, nil, []string{"1210", "Land and Buildings"}},
		{reports.GLRegisterID, nil, []string{"1210"}},
		{reports.DetailTrialBalanceID, nil, []string{"1520"}},
		{reports.CustomerListID, nil, []string{"C00010"}},
		{reports.CustomerDetailTrialBal, nil, []string{"C00010"}},
		{reports.VendorListID, nil, []string{"V00010"}},
		{reports.VendorDetailTrialBal, nil, []string{"V00010"}},
		{reports.InventoryListID, nil, []string{"1000"}},
		{reports.InventoryValuationID, nil, []string{"1000"}},
		{reports.ResourceListID, nil, []string{"R0010"}},
		{reports.BankAccountListID, nil, []string{"B010"}},
		{reports.BankAccDetailTrialBalID, nil, []string{"B010"}},
		{reports.AgedAccountsPayable, map[string]string{"aged_as_of": "2026-10-06", "print_details": "true"}, nil},
	}
	for _, c := range cases {
		texts := runList(t, src, c.id, c.options)
		for _, w := range c.want {
			if !strings.Contains(texts, w) {
				t.Errorf("report %d does not print %q", c.id, w)
			}
		}
	}

	// Trial Balance: the posting accounts' net change debit and credit agree (a balanced G/L)
	acc := gtables.NewGLAccount(src)
	acc.SetRange("account_type", gtables.GLAccount_Account_type.Posting)
	var debit types.Decimal
	for found := acc.FindSetBuffered(); found; found = acc.Next() {
		acc.CalcFields("net_change")
		if acc.Net_change.IsPositive() {
			debit = debit.Add(acc.Net_change)
		}
	}
	texts := runList(t, src, reports.TrialBalanceID, nil)
	if n := strings.Count(texts, amountText(debit)); n < 4 {
		t.Errorf("trial balance: total %s printed %d times, want 4 (net change and balance, debit and credit)", amountText(debit), n)
	}

	// Aged Accounts Receivable: the total is the open entries' remaining amount (LCY)
	cle := gtables.NewCustomerLedgerEntry(src)
	cle.SetRange("open", true)
	cle.SetRange("posting_date", types.Date{}, types.NewDate(2026, 10, 6))
	mustOK(t, cle.CalcSums("remaining_amt_lcy"))
	texts = runList(t, src, reports.AgedAccountsReceivable, map[string]string{"aged_as_of": "2026-10-06"})
	if !strings.Contains(texts, amountText(cle.Remaining_amt_lcy)) {
		t.Errorf("aged accounts receivable: no total %s", amountText(cle.Remaining_amt_lcy))
	}

	// Customer - List: the total of the balances (LCY)
	cust := gtables.NewCustomer(src)
	var balance types.Decimal
	for found := cust.FindSetBuffered(); found; found = cust.Next() {
		cust.CalcFields("balance_lcy")
		balance = balance.Add(cust.Balance_lcy)
	}
	texts = runList(t, src, reports.CustomerListID, nil)
	if !strings.Contains(texts, amountText(balance)) {
		t.Errorf("customer list: no total %s", amountText(balance))
	}
}
