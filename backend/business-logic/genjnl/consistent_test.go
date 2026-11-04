package genjnl

import (
	"errors"
	"testing"

	apperrors "github.com/hansjlachmann/goeasyerp/backend/foundation/errors"
	ftables "github.com/hansjlachmann/goeasyerp/backend/foundation/tables"
	gtables "github.com/hansjlachmann/goeasyerp/backend/generated/tables"
)

// postLines posts journal lines one by one through the line posting in one transaction, as the
// document postings do (no batch balance check before), and commits with the balance check
func postLines(t *testing.T, src ftables.Source, lines ...lineSpec) error {
	t.Helper()
	for _, l := range lines {
		addLine(t, src, "GENERAL", "DEFAULT", l)
	}
	tx, err := ftables.BeginTransaction(src.GetDB())
	must(t, err)
	txSrc := ftables.DBSource(tx, src.GetCompany(), src.GetDBType())
	postLine := NewGenJnlPostLine(txSrc)
	for _, l := range lines {
		line := newJnlLine(txSrc, "GENERAL", "DEFAULT", l.lineNo)
		must(t, line.GetOrError(map[string]interface{}{"journal_template_name": "GENERAL", "journal_batch_name": "DEFAULT", "line_no": l.lineNo}))
		err = postLine.RunWithoutCheck(line)
		if err != nil {
			_ = ftables.Rollback(tx)
			return err
		}
	}
	return ftables.Commit(tx)
}

// isInconsistent: the commit was refused by the balance check (the G/L entries named)
func isInconsistent(err error) bool {
	var appErr *apperrors.AppError
	return errors.As(err, &appErr) && (appErr.Code == apperrors.ErrGLNotBalanced || appErr.Code == apperrors.ErrInconsistent)
}

// G/L entries that do not balance are never committed: the commit is refused with
// ERR_INCONSISTENT and nothing of the posting is saved
func TestConsistentRefusesUnbalancedPosting(t *testing.T) {
	src := demoCompany(t)
	gl0, _, _, reg0 := countEntries(src)
	err := postLines(t, src, lineSpec{lineNo: 10000, docNo: "G00001", accNo: "1210", amount: "1000"})
	if !isInconsistent(err) {
		t.Fatalf("an unbalanced posting must be refused by the balance check, got %v", err)
	}
	// the message names the document, its date and the difference; the details hold the totals
	// (the document marked) and the entries
	var appErr *apperrors.AppError
	errors.As(err, &appErr)
	if appErr.Code != apperrors.ErrGLNotBalanced || appErr.Message("en-US") !=
		"The transaction cannot be completed because the G/L entries do not balance: document G00001 on 10/15/2026 is off by 1,000.00. Nothing was saved." {
		t.Errorf("message: %s", appErr.Message("en-US"))
	}
	details := appErr.Details
	if details == nil || len(details.Sections) != 2 {
		t.Fatalf("details %+v", details)
	}
	totals, entries := details.Sections[0], details.Sections[1]
	if len(totals.Rows) != 1 || totals.Rows[0][1] != "G00001" || totals.Rows[0][2] != "1,000.00" || len(totals.Marked) != 1 {
		t.Errorf("totals %+v", totals)
	}
	if len(entries.Rows) != 1 || entries.Rows[0][3] != "1210" || entries.Rows[0][4] != "Land and Buildings" || entries.Rows[0][7] != "1,000.00" {
		t.Errorf("entries %+v", entries)
	}
	gl1, _, _, reg1 := countEntries(src)
	if gl1 != gl0 || reg1 != reg0 {
		t.Errorf("after the refusal: %d G/L entries, %d registers saved", gl1-gl0, reg1-reg0)
	}
	if gtables.NewGenJournalLine(src).Count() != 1 {
		t.Error("the journal line must still be there")
	}
}

// Lines that balance on their posting date commit
func TestConsistentBalancedPosting(t *testing.T) {
	src := demoCompany(t)
	gl0, _, _, _ := countEntries(src)
	err := postLines(t, src,
		lineSpec{lineNo: 10000, docNo: "G00001", accNo: "1210", amount: "1000"},
		lineSpec{lineNo: 20000, docNo: "G00001", accNo: "1230", amount: "-1000"})
	must(t, err)
	gl1, _, _, _ := countEntries(src)
	if gl1-gl0 != 2 {
		t.Errorf("posted %d G/L entries", gl1-gl0)
	}
}

// Entries that add up to 0 in total but not per posting date are refused (the date-weighted
// balance sums)
func TestConsistentPerPostingDate(t *testing.T) {
	src := demoCompany(t)
	gl0, _, _, _ := countEntries(src)
	err := postLines(t, src,
		lineSpec{lineNo: 10000, docNo: "G00001", accNo: "1210", amount: "1000", postingDay: 15},
		lineSpec{lineNo: 20000, docNo: "G00001", accNo: "1230", amount: "-1000", postingDay: 16})
	if !isInconsistent(err) {
		t.Fatalf("balanced in total but not per date must be refused, got %v", err)
	}
	gl1, _, _, _ := countEntries(src)
	if gl1 != gl0 {
		t.Errorf("%d G/L entries saved", gl1-gl0)
	}
}

// The balance check on PostgreSQL (TEST_POSTGRES_DSN): an unbalanced posting and one balanced
// in total but not per date are refused with nothing saved; a balanced one commits
func TestConsistentPostgres(t *testing.T) {
	src := demoCompanyPostgres(t)
	gl0, _, _, reg0 := countEntries(src)
	err := postLines(t, src, lineSpec{lineNo: 10000, docNo: "PG00001", accNo: "1210", amount: "1000"})
	if !isInconsistent(err) {
		t.Fatalf("an unbalanced posting must be refused, got %v", err)
	}
	err = postLines(t, src,
		lineSpec{lineNo: 20000, docNo: "PG00002", accNo: "1210", amount: "500", postingDay: 15},
		lineSpec{lineNo: 30000, docNo: "PG00002", accNo: "1230", amount: "-500", postingDay: 16})
	if !isInconsistent(err) {
		t.Fatalf("balanced in total but not per date must be refused, got %v", err)
	}
	gl1, _, _, reg1 := countEntries(src)
	if gl1 != gl0 || reg1 != reg0 {
		t.Errorf("after the refusals: %d G/L entries, %d registers saved", gl1-gl0, reg1-reg0)
	}
	err = postLines(t, src,
		lineSpec{lineNo: 40000, docNo: "PG00003", accNo: "1210", amount: "250"},
		lineSpec{lineNo: 50000, docNo: "PG00003", accNo: "1230", amount: "-250"})
	must(t, err)
	gl2, _, _, _ := countEntries(src)
	if gl2-gl0 != 2 {
		t.Errorf("a balanced posting saved %d G/L entries", gl2-gl0)
	}
}
