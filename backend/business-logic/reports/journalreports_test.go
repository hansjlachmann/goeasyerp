package reports_test

import (
	"strings"
	"testing"

	"github.com/hansjlachmann/goeasyerp/backend/business-logic/reports"
	ftables "github.com/hansjlachmann/goeasyerp/backend/foundation/tables"
	"github.com/hansjlachmann/goeasyerp/backend/foundation/types"
	gtables "github.com/hansjlachmann/goeasyerp/backend/generated/tables"
)

// runOnRecord prints a report with the record's filters as its request filters
func runOnRecord(t *testing.T, src ftables.Source, id int, rec ftables.Table) string {
	t.Helper()
	result, err := reports.Execute(id, src.GetDB(), src.GetCompany(), src.GetDBType(), reports.RunOptions{Output: reports.OutputPDF, Record: rec})
	if err != nil {
		t.Fatalf("report %d: %v", id, err)
	}
	if result.Pages == 0 {
		t.Fatalf("report %d: no printout", id)
	}
	return printedTexts(result, 0)
}

func wantPrinted(t *testing.T, id int, texts string, want ...string) {
	t.Helper()
	for _, w := range want {
		if !strings.Contains(texts, w) {
			t.Errorf("report %d does not print %q:\n%s", id, w, texts)
		}
	}
}

// 5602 Fixed Asset Journal - Test and 5622 Insurance Journal - Test print a batch's lines with
// the warnings posting would give; 5623 Insurance Register a register's coverage entries
func TestFAAndInsuranceJournalReports(t *testing.T) {
	src := listCompany(t)

	faTemplate := gtables.NewFAJournalTemplate(src)
	faTemplate.Name = "FATEST"
	mustOK(t, faTemplate.InsertOrError(true))
	faBatch := gtables.NewFAJournalBatch(src)
	faBatch.Journal_template_name, faBatch.Name = "FATEST", "B1"
	mustOK(t, faBatch.InsertOrError(false))
	faLine := gtables.NewFAJournalLine(src)
	faLine.Journal_template_name, faLine.Journal_batch_name, faLine.Line_no = "FATEST", "B1", 10000
	faLine.Fa_no, faLine.Document_no, faLine.Description = "NOSUCHFA", "", "Machine"
	faLine.Fa_posting_date = types.NewDate(2026, 10, 1)
	faLine.Amount = types.NewDecimalFromInt(1000)
	mustOK(t, faLine.InsertOrError(false))
	if faTemplate.Test_report_id != reports.FAJnlTestID || faTemplate.Posting_report_id != 5603 {
		t.Errorf("FA template reports %d/%d, want 5602/5603", faTemplate.Test_report_id, faTemplate.Posting_report_id)
	}
	faBatch.SetRecFilter()
	texts := runOnRecord(t, src, reports.FAJnlTestID, faBatch)
	wantPrinted(t, reports.FAJnlTestID, texts, "NOSUCHFA", "Machine", "Document No. must be specified.",
		"Depreciation Book Code must be specified.", "does not exist.")

	insTemplate := gtables.NewInsuranceJournalTemplate(src)
	insTemplate.Name = "INSTEST"
	mustOK(t, insTemplate.InsertOrError(true))
	insBatch := gtables.NewInsuranceJournalBatch(src)
	insBatch.Journal_template_name, insBatch.Name = "INSTEST", "B1"
	mustOK(t, insBatch.InsertOrError(false))
	insLine := gtables.NewInsuranceJournalLine(src)
	insLine.Journal_template_name, insLine.Journal_batch_name, insLine.Line_no = "INSTEST", "B1", 10000
	insLine.Insurance_no, insLine.Document_no = "INS1", "D1"
	mustOK(t, insLine.InsertOrError(false))
	insBatch.SetRecFilter()
	texts = runOnRecord(t, src, reports.InsuranceJnlTestID, insBatch)
	wantPrinted(t, reports.InsuranceJnlTestID, texts, "INS1", "Posting Date must be specified.", "FA No. must be specified.")

	entry := gtables.NewInsCoverageLedgerEntry(src)
	entry.Entry_no, entry.Insurance_no, entry.Fa_no, entry.Document_no = 999999, "INS1", "FA1", "COV1"
	entry.Posting_date, entry.Amount = types.NewDate(2026, 10, 1), types.NewDecimalFromInt(2500)
	mustOK(t, entry.InsertOrError(false))
	register := gtables.NewInsuranceRegister(src)
	register.No, register.From_entry_no, register.To_entry_no = 999999, 999999, 999999
	mustOK(t, register.InsertOrError(false))
	register.SetRecFilter()
	texts = runOnRecord(t, src, reports.InsuranceRegisterID, register)
	wantPrinted(t, reports.InsuranceRegisterID, texts, "COV1", "INS1", "2,500.00")
}

// 1408 Bank Acc. Recon. - Test warns about lines that do not add up to the ending balance and a
// line whose applied amount is not what is applied; 1407 Bank Account Statement prints a
// statement's lines
func TestBankAccReconReports(t *testing.T) {
	src := listCompany(t)

	recon := gtables.NewBankAccReconciliation(src)
	recon.Bank_account_no, recon.Statement_no = "B010", "TEST1"
	recon.Statement_ending_balance = types.NewDecimalFromInt(500)
	mustOK(t, recon.InsertOrError(false))
	line := gtables.NewBankAccReconciliationLine(src)
	line.Bank_account_no, line.Statement_no, line.Statement_line_no = "B010", "TEST1", 10000
	line.Description, line.Statement_amount = "Deposit", types.NewDecimalFromInt(300)
	line.Applied_amount = types.NewDecimalFromInt(300)
	mustOK(t, line.InsertOrError(false))
	recon.SetRecFilter()
	texts := runOnRecord(t, src, reports.BankAccReconTestID, recon)
	wantPrinted(t, reports.BankAccReconTestID, texts, "Deposit", "Statement Date must be specified.",
		"Statement Ending Balance is not equal to Total Balance.", "Applied Amount must be 0.00.")

	stmt := gtables.NewBankAccountStatement(src)
	stmt.Bank_account_no, stmt.Statement_no = "B010", "ST-TEST"
	mustOK(t, stmt.InsertOrError(false))
	stmtLine := gtables.NewBankAccountStatementLine(src)
	stmtLine.Bank_account_no, stmtLine.Statement_no, stmtLine.Statement_line_no = "B010", "ST-TEST", 10000
	stmtLine.Description, stmtLine.Statement_amount = "Fee", types.NewDecimalFromInt(-25)
	mustOK(t, stmtLine.InsertOrError(false))
	stmt.SetRecFilter()
	texts = runOnRecord(t, src, reports.BankAccountStatementID, stmt)
	wantPrinted(t, reports.BankAccountStatementID, texts, "ST-TEST", "Fee", "-25.00")
}
