package reports_test

import (
	"database/sql"
	"encoding/base64"
	"strings"
	"testing"
	"time"

	"github.com/hansjlachmann/openerp/backend/business-logic/demodata"
	"github.com/hansjlachmann/openerp/backend/business-logic/glcheck"
	"github.com/hansjlachmann/openerp/backend/business-logic/itemjnl"
	"github.com/hansjlachmann/openerp/backend/business-logic/reports"
	"github.com/hansjlachmann/openerp/backend/business-logic/tables"
	"github.com/hansjlachmann/openerp/backend/foundation/database"
	ftables "github.com/hansjlachmann/openerp/backend/foundation/tables"
	"github.com/hansjlachmann/openerp/backend/foundation/types"
	gtables "github.com/hansjlachmann/openerp/backend/generated/tables"
)

// consolidationCompanies: the demo company as a subsidiary (its posting accounts consolidate to
// the accounts of the same numbers) and a group company with the same demo data and business
// unit SUB for it, in a transaction
func consolidationCompanies(t *testing.T) (subsidiary, group ftables.Source) {
	t.Helper()
	db, err := sql.Open("sqlite3", ":memory:")
	mustOK(t, err)
	db.SetMaxOpenConns(1)
	t.Cleanup(func() { _ = db.Close() })
	mustOK(t, (&gtables.Company{}).CreateTableWithDBType(db, "", database.DBTypeSQLite))
	for _, company := range []string{"sub", "group"} {
		mustOK(t, tables.CreateCompanyTables(db, company, database.DBTypeSQLite))
		_, err = demodata.Create(db, company, database.DBTypeSQLite, demodata.Options{
			Today:           time.Date(2026, 10, 6, 0, 0, 0, 0, time.Local),
			ItemJnlPostLine: func(src ftables.Source) demodata.ItemJnlPoster { return itemjnl.NewItemJnlPostLine(src) },
		})
		mustOK(t, err)
		_, err = db.Exec(`INSERT INTO "Company" (name) VALUES (?)`, company)
		mustOK(t, err)
	}
	_, err = db.Exec(`UPDATE "sub$G_L_Account" SET consol_debit_acc = no, consol_credit_acc = no WHERE account_type = 0`)
	mustOK(t, err)
	tx, err := ftables.BeginTransaction(db)
	mustOK(t, err)
	t.Cleanup(func() { _ = ftables.Rollback(tx) })
	subsidiary = ftables.DBSource(tx, "sub", database.DBTypeSQLite)
	group = ftables.DBSource(tx, "group", database.DBTypeSQLite)
	busUnit := gtables.NewBusinessUnit(group)
	busUnit.Init()
	busUnit.Code = "SUB"
	busUnit.Name = "Subsidiary"
	busUnit.Company_name = "sub"
	busUnit.Residual_account = "6130"
	busUnit.Exch_rate_gains_acc = "6160"
	busUnit.Exch_rate_loss_acc = "6170"
	mustOK(t, busUnit.InsertOrError(true))
	return subsidiary, group
}

// netChanges: the net change of each posting account in a company in 2026 (of a business unit
// filter, blank: all entries)
func netChanges(src ftables.Source, busUnitFilter string) map[types.Code]types.Decimal {
	glAcc := gtables.NewGLAccount(src)
	glAcc.SetRange("account_type", gtables.GLAccount_Account_type.Posting)
	_ = glAcc.SetFlowFilter("date_filter", "2026-01-01..2026-12-31")
	if busUnitFilter != "" {
		_ = glAcc.SetFlowFilter("business_unit_filter", busUnitFilter)
	}
	result := map[types.Code]types.Decimal{}
	for found := glAcc.FindSet(); found; found = glAcc.Next() {
		glAcc.CalcFields("net_change")
		if !glAcc.Net_change.IsZero() {
			result[glAcc.No] = glAcc.Net_change
		}
	}
	return result
}

func consolidationOptions(extra map[string]string) map[string]string {
	options := map[string]string{"starting_date": "2026-01-01", "ending_date": "2026-12-31", "document_no": "CONSOL-26"}
	for k, v := range extra {
		options[k] = v
	}
	return options
}

// Import Consolidation from DB: the group's G/L gets the subsidiary's net change of the period
// on each account under business unit SUB, balanced; a second import replaces the first
func TestImportConsolidationFromDB(t *testing.T) {
	subsidiary, group := consolidationCompanies(t)
	want := netChanges(subsidiary, "")
	if len(want) == 0 {
		t.Fatal("the subsidiary has no entries in 2026")
	}
	for run := 1; run <= 2; run++ {
		_, err := reports.Execute(reports.ImportConsolidationDBID, group.GetDB(), group.GetCompany(), group.GetDBType(), reports.RunOptions{Options: consolidationOptions(nil)})
		mustOK(t, err)
		got := netChanges(group, "SUB")
		if len(got) != len(want) {
			t.Fatalf("run %d: %d accounts consolidated, the subsidiary has %d", run, len(got), len(want))
		}
		for acc, amount := range want {
			if !got[acc].Round(2).Equal(amount.Round(2)) {
				t.Errorf("run %d: account %s %s, the subsidiary %s", run, acc, got[acc], amount)
			}
		}
		if r := glcheck.Check(group, nil); !r.Balanced() {
			t.Fatalf("run %d: the group's G/L is not balanced: %+v", run, r)
		}
	}
	cleared := gtables.NewGLEntry(group)
	cleared.SetRange("business_unit_code", types.Code("SUB"))
	cleared.SetRange("amount", types.Decimal{})
	if cleared.Count() == 0 {
		t.Error("the first import's entries were not cleared")
	}
}

// At 50 % the group gets half of each amount; what rounding leaves goes to the residual account
func TestImportConsolidationShare(t *testing.T) {
	subsidiary, group := consolidationCompanies(t)
	busUnit := gtables.NewBusinessUnit(group)
	mustOK(t, busUnit.GetOrError(types.Code("SUB")))
	busUnit.Consolidation_pct = types.NewDecimalFromInt(50)
	mustOK(t, busUnit.ModifyOrError(true))
	_, err := reports.Execute(reports.ImportConsolidationDBID, group.GetDB(), group.GetCompany(), group.GetDBType(), reports.RunOptions{Options: consolidationOptions(nil)})
	mustOK(t, err)
	got := netChanges(group, "SUB")
	two := types.NewDecimalFromInt(2)
	for acc, amount := range netChanges(subsidiary, "") {
		if acc == "6130" {
			continue // the residual account
		}
		if half := amount.Div(two); got[acc].Sub(half).Abs().GreaterThan(types.MustDecimal("0.01")) {
			t.Errorf("account %s %s, half of %s", acc, got[acc], amount)
		}
	}
	if r := glcheck.Check(group, nil); !r.Balanced() {
		t.Fatalf("the group's G/L is not balanced: %+v", r)
	}
}

// Export Consolidation writes the subsidiary's G/L as a file; Consolidation - Test File finds
// nothing missing and Import Consolidation from File consolidates it as the database import does
func TestConsolidationFile(t *testing.T) {
	subsidiary, group := consolidationCompanies(t)
	result, err := reports.Execute(reports.ExportConsolidationID, subsidiary.GetDB(), subsidiary.GetCompany(), subsidiary.GetDBType(), reports.RunOptions{Options: consolidationOptions(nil)})
	mustOK(t, err)
	if !strings.HasPrefix(string(result.File), "sub\t2026-01-01\t2026-12-31\t0\t0\t0\r\nA\t") || result.FileName == "" {
		t.Fatalf("file %q: %.200q", result.FileName, result.File)
	}
	file := base64.StdEncoding.EncodeToString(result.File)

	test, err := reports.Execute(reports.ConsolidationTestFileID, group.GetDB(), group.GetCompany(), group.GetDBType(), reports.RunOptions{Output: reports.OutputPDF, Options: map[string]string{"file": file}})
	mustOK(t, err)
	if texts := printedTexts(test, 0); strings.Contains(texts, "SUB") {
		t.Errorf("the test file finds problems: %s", texts)
	}

	_, err = reports.Execute(reports.ImportConsolidationFileID, group.GetDB(), group.GetCompany(), group.GetDBType(), reports.RunOptions{Options: map[string]string{"file": file, "document_no": "CONSOL-F"}})
	mustOK(t, err)
	got := netChanges(group, "SUB")
	for acc, amount := range netChanges(subsidiary, "") {
		if !got[acc].Round(2).Equal(amount.Round(2)) {
			t.Errorf("account %s %s, the subsidiary %s", acc, got[acc], amount)
		}
	}
	if r := glcheck.Check(group, nil); !r.Balanced() {
		t.Fatalf("the group's G/L is not balanced: %+v", r)
	}

	_, err = reports.Execute(reports.ImportConsolidationFileID, group.GetDB(), group.GetCompany(), group.GetDBType(), reports.RunOptions{Options: map[string]string{"file": base64.StdEncoding.EncodeToString([]byte("nonsense")), "document_no": "X"}})
	if err == nil {
		t.Error("a file of another format imported")
	}
}

// Consolidation - Test Database lists a consolidation account that is not in the group
func TestConsolidationTestDatabase(t *testing.T) {
	subsidiary, group := consolidationCompanies(t)
	glAcc := gtables.NewGLAccount(subsidiary)
	glAcc.SetRange("account_type", gtables.GLAccount_Account_type.Posting)
	acc := ""
	for account := range netChanges(subsidiary, "") {
		acc = string(account)
		break
	}
	mustOK(t, glAcc.GetOrError(types.Code(acc)))
	glAcc.Consol_debit_acc = "X-MISSING"
	glAcc.Consol_credit_acc = "X-MISSING"
	mustOK(t, glAcc.ModifyOrError(false))
	result, err := reports.Execute(reports.ConsolidationTestDatabaseID, group.GetDB(), group.GetCompany(), group.GetDBType(), reports.RunOptions{Output: reports.OutputPDF, Options: consolidationOptions(nil)})
	mustOK(t, err)
	texts := printedTexts(result, 0)
	if !strings.Contains(texts, "X-MISSING") || !strings.Contains(texts, "("+acc+")") {
		t.Errorf("missing account not listed: %s", texts)
	}
}

// The consolidated trial balances show the business unit's amounts and the eliminations report
// a journal line on an account
func TestConsolidatedTrialBalances(t *testing.T) {
	_, group := consolidationCompanies(t)
	_, err := reports.Execute(reports.ImportConsolidationDBID, group.GetDB(), group.GetCompany(), group.GetDBType(), reports.RunOptions{Options: consolidationOptions(nil)})
	mustOK(t, err)
	period := map[string]string{"starting_date": "2026-01-01", "ending_date": "2026-12-31"}
	for _, id := range []int{reports.ConsolidatedTrialBalanceID, reports.ConsolidatedTrialBalance4ID} {
		result, err := reports.Execute(id, group.GetDB(), group.GetCompany(), group.GetDBType(), reports.RunOptions{Output: reports.OutputPDF, Options: period})
		mustOK(t, err)
		if texts := printedTexts(result, 0); !strings.Contains(texts, "SUB") {
			t.Errorf("report %d without the business unit: %.300s", id, texts)
		}
	}

	line := gtables.NewGenJournalLine(group)
	line.Init()
	line.Journal_template_name = "GENERAL"
	line.Journal_batch_name = "DEFAULT"
	line.Line_no = 10000
	mustOK(t, line.Validate("posting_date", types.NewDate(2026, 12, 31)))
	line.Document_no = "ELIM1"
	mustOK(t, line.Validate("account_no", types.Code("1210")))
	mustOK(t, line.Validate("amount", types.NewDecimalFromInt(-123)))
	line.Description = "Intercompany elimination"
	mustOK(t, line.InsertOrError(false))
	options := map[string]string{"starting_date": "2026-01-01", "ending_date": "2026-12-31", "business_unit": "SUB", "journal_template": "GENERAL", "journal_batch": "DEFAULT"}
	result, err := reports.Execute(reports.ConsolidationEliminationsID, group.GetDB(), group.GetCompany(), group.GetDBType(), reports.RunOptions{Output: reports.OutputPDF, Options: options})
	mustOK(t, err)
	if texts := printedTexts(result, 0); !strings.Contains(texts, "Intercompany elimination") || !strings.Contains(texts, "-123.00") {
		t.Errorf("the elimination is not printed: %.500s", texts)
	}
}

// A business unit in euro: income statement accounts at the income factor, balance sheet
// accounts at the balance factor with the difference as an exchange rate adjustment, the
// opening balance adjusted when the balance factor changed — the group's G/L stays balanced
func TestImportConsolidationCurrency(t *testing.T) {
	_, group := consolidationCompanies(t)
	busUnit := gtables.NewBusinessUnit(group)
	mustOK(t, busUnit.GetOrError(types.Code("SUB")))
	busUnit.Currency_code = "EUR"
	busUnit.Income_currency_factor = types.MustDecimal("0.0870")
	busUnit.Balance_currency_factor = types.MustDecimal("0.0850")
	busUnit.Last_balance_currency_factor = types.MustDecimal("0.0850")
	mustOK(t, busUnit.ModifyOrError(false))
	_, err := reports.Execute(reports.ImportConsolidationDBID, group.GetDB(), group.GetCompany(), group.GetDBType(), reports.RunOptions{Options: consolidationOptions(nil)})
	mustOK(t, err)
	if r := glcheck.Check(group, nil); !r.Balanced() {
		t.Fatalf("the group's G/L is not balanced: %+v", r)
	}
	adjustments := netChanges(group, "SUB")
	if adjustments["6160"].IsZero() && adjustments["6170"].IsZero() {
		t.Error("no exchange rate adjustment posted")
	}

	// the balance factor changes: the next import adjusts the opening balance sheet
	mustOK(t, busUnit.GetOrError(types.Code("SUB")))
	busUnit.Balance_currency_factor = types.MustDecimal("0.0800")
	mustOK(t, busUnit.ModifyOrError(false))
	_, err = reports.Execute(reports.ImportConsolidationDBID, group.GetDB(), group.GetCompany(), group.GetDBType(), reports.RunOptions{Options: consolidationOptions(map[string]string{"document_no": "CONSOL-26B"})})
	mustOK(t, err)
	mustOK(t, busUnit.GetOrError(types.Code("SUB")))
	if !busUnit.Last_balance_currency_factor.Equal(types.MustDecimal("0.08")) {
		t.Errorf("last balance factor %s", busUnit.Last_balance_currency_factor)
	}
	if r := glcheck.Check(group, nil); !r.Balanced() {
		t.Fatalf("the group's G/L is not balanced after the second import: %+v", r)
	}
}
