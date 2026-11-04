package reports_test

import (
	"strings"
	"testing"

	"github.com/hansjlachmann/goeasyerp/backend/business-logic/genjnl"
	"github.com/hansjlachmann/goeasyerp/backend/business-logic/glcheck"
	"github.com/hansjlachmann/goeasyerp/backend/business-logic/reports"
	ftables "github.com/hansjlachmann/goeasyerp/backend/foundation/tables"
	"github.com/hansjlachmann/goeasyerp/backend/foundation/types"
	gtables "github.com/hansjlachmann/goeasyerp/backend/generated/tables"
)

// incomeStatementNetChange: the net change of the income statement accounts in a date filter
func incomeStatementNetChange(t *testing.T, src ftables.Source, dateFilter string) types.Decimal {
	t.Helper()
	glAcc := gtables.NewGLAccount(src)
	glAcc.SetRange("account_type", gtables.GLAccount_Account_type.Posting)
	glAcc.SetRange("income_balance", gtables.GLAccount_Income_balance.IncomeStatement)
	mustOK(t, glAcc.SetFlowFilter("date_filter", dateFilter))
	var total types.Decimal
	for found := glAcc.FindSet(); found; found = glAcc.Next() {
		glAcc.CalcFields("net_change")
		total = total.Add(glAcc.Net_change)
	}
	return total.Round(2)
}

// Close Income Statement for 2025 (closed): its journal lines clear each income statement
// account on C2025-12-31 with the total on Retained Earnings; posted, the year up to its closing
// date has no result left, while the year up to 31-12 still shows it; the entries read back
// with their closing date and a filter on it finds them
func TestCloseIncomeStatement(t *testing.T) {
	src := listCompany(t)
	closedYear(t, src)
	result := incomeStatementNetChange(t, src, "2025-01-01..2025-12-31")
	if result.IsZero() {
		t.Fatal("no 2025 result in the demo")
	}
	retained0 := glBalance(src, "2120")

	options := map[string]string{"journal_template": "GENERAL", "journal_batch": "DEFAULT", "document_no": "CLOSE25",
		"retained_earnings_acc": "2120"}
	options["fiscal_year_ending_date"] = "2026-06-30"
	_, err := reports.Execute(reports.CloseIncomeStatementID, src.GetDB(), src.GetCompany(), src.GetDBType(), reports.RunOptions{Options: options})
	if err == nil || !strings.Contains(err.Error(), "fiscal year does not exist") {
		t.Fatalf("a date that does not end a fiscal year: %v", err)
	}
	options["fiscal_year_ending_date"] = "2026-12-31"
	_, err = reports.Execute(reports.CloseIncomeStatementID, src.GetDB(), src.GetCompany(), src.GetDBType(), reports.RunOptions{Options: options})
	if err == nil || !strings.Contains(err.Error(), "must be closed") {
		t.Fatalf("a year not closed: %v", err)
	}
	// without a date: the end of the last closed fiscal year (2025)
	delete(options, "fiscal_year_ending_date")
	_, err = reports.Execute(reports.CloseIncomeStatementID, src.GetDB(), src.GetCompany(), src.GetDBType(), reports.RunOptions{Options: options})
	mustOK(t, err)
	options["fiscal_year_ending_date"] = "2025-12-31"

	lines := gtables.NewGenJournalLine(src)
	lines.SetRange("journal_template_name", types.Code("GENERAL"))
	lines.SetRange("journal_batch_name", types.Code("DEFAULT"))
	if lines.Count() < 2 || !lines.FindFirst() {
		t.Fatalf("%d journal lines", lines.Count())
	}
	if !lines.Posting_date.IsClosing() || lines.Posting_date.String() != "C2025-12-31" || lines.Document_no != "CLOSE25" {
		t.Errorf("first line: posting date %s, document %s", lines.Posting_date, lines.Document_no)
	}
	mustOK(t, lines.CalcSums("amount"))
	if !lines.Amount.Round(2).IsZero() {
		t.Errorf("the lines add up to %s", lines.Amount)
	}

	_, err = genjnl.NewGenJnlPostBatch(src).Run(lines)
	mustOK(t, err)
	if got := incomeStatementNetChange(t, src, "2025-01-01..C2025-12-31"); !got.IsZero() {
		t.Errorf("the income statement up to the closing date: %s, want 0", got)
	}
	if got := incomeStatementNetChange(t, src, "2025-01-01..2025-12-31"); !got.Equal(result) {
		t.Errorf("the year up to 31-12: %s, want %s (closing entries left out)", got, result)
	}
	if got := glBalance(src, "2120").Sub(retained0); !got.Equal(result) {
		t.Errorf("retained earnings changed by %s, want %s", got, result)
	}
	closing := gtables.NewGLEntry(src)
	closing.SetFilter("posting_date", "C2025-12-31")
	n := closing.Count()
	if n == 0 || !closing.FindFirst() || !closing.Posting_date.IsClosing() {
		t.Fatalf("%d entries on the closing date; first %s", n, closing.Posting_date)
	}
	sameDay := gtables.NewGLEntry(src)
	sameDay.SetRange("posting_date", types.NewDate(2025, 12, 31))
	for found := sameDay.FindSet(); found; found = sameDay.Next() {
		if sameDay.Posting_date.IsClosing() {
			t.Fatal("the filter on 31-12 found a closing entry")
		}
	}
	if r := glcheck.Check(src, nil); !r.Balanced() {
		t.Errorf("G/L not balanced: %+v", r)
	}

	// again: nothing left to close
	_, err = reports.Execute(reports.CloseIncomeStatementID, src.GetDB(), src.GetCompany(), src.GetDBType(), reports.RunOptions{Options: options})
	mustOK(t, err)
	if lines.Count() != 0 {
		t.Errorf("a second run wrote %d lines", lines.Count())
	}
}
