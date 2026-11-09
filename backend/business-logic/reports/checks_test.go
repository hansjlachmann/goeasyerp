package reports_test

import (
	"strings"
	"testing"

	"github.com/hansjlachmann/goeasyerp/backend/business-logic/checkmgt"
	"github.com/hansjlachmann/goeasyerp/backend/business-logic/genjnl"
	"github.com/hansjlachmann/goeasyerp/backend/business-logic/reports"
	ftables "github.com/hansjlachmann/goeasyerp/backend/foundation/tables"
	"github.com/hansjlachmann/goeasyerp/backend/foundation/types"
	gtables "github.com/hansjlachmann/goeasyerp/backend/generated/tables"
)

// Suggest Vendor Payments fills a payment journal with the vendors' open invoices; Check prints
// them as computer checks (check ledger entries Printed, lines Check Printed with the check
// nos.); voiding a check takes it back; posting the printed checks posts their check entries
func TestSuggestVendorPaymentsAndChecks(t *testing.T) {
	src := listCompany(t)

	template := gtables.NewGenJournalTemplate(src)
	template.Name = "PMTTEST"
	template.Type = gtables.GenJournalTemplate_Type.Payments
	mustOK(t, template.InsertOrError(true))
	batch := gtables.NewGenJournalBatch(src)
	batch.Journal_template_name, batch.Name = "PMTTEST", "B1"
	mustOK(t, batch.InsertOrError(false))
	context := gtables.NewGenJournalLine(src)
	context.Journal_template_name, context.Journal_batch_name = "PMTTEST", "B1"

	_, err := reports.Execute(reports.SuggestVendorPaymentsID, src.GetDB(), src.GetCompany(), src.GetDBType(), reports.RunOptions{
		Context: context,
		Options: map[string]string{
			"last_payment_date": "2026-12-31", "posting_date": "2026-10-06",
			"bal_account_type": "2", "bal_account_no": "B010", "bank_payment_type": "2",
			"starting_document_no": "PAY0001",
		},
		Filters: map[string][]ftables.FieldFilter{"Vendor": {{Field: "no", Expression: "V00010"}}},
	})
	mustOK(t, err)
	lines := func() []*gtables.GenJournalLine {
		line := gtables.NewGenJournalLine(src)
		line.SetRange("journal_template_name", "PMTTEST")
		line.SetRange("journal_batch_name", "B1")
		var all []*gtables.GenJournalLine
		for found := line.FindSetBuffered(); found; found = line.Next() {
			l := gtables.NewGenJournalLine(src)
			l.Copy(line)
			all = append(all, l)
		}
		return all
	}
	suggested := lines()
	t.Logf("suggested %d lines", len(suggested))
	if len(suggested) == 0 {
		t.Fatal("no payment suggested for V00010")
	}
	for _, l := range suggested {
		if l.Account_no != "V00010" || l.Bal_account_no != "B010" || l.Applies_to_doc_no == "" || !l.Amount.IsPositive() ||
			l.Bank_payment_type != gtables.GenJournalLine_Bank_payment_type.ComputerCheck {
			t.Errorf("suggested line %+v", map[string]interface{}{"account": l.Account_no, "bal": l.Bal_account_no, "applies_to": l.Applies_to_doc_no, "amount": l.Amount.String()})
		}
	}

	printChecks := func() string {
		result, err := reports.Execute(reports.CheckReportID, src.GetDB(), src.GetCompany(), src.GetDBType(), reports.RunOptions{
			Context: context,
			Output:  reports.OutputPDF,
			Options: map[string]string{"bank_account": "B010", "last_check_no": "CHK1000"},
		})
		mustOK(t, err)
		return printedTexts(result, 0)
	}
	texts := printChecks()
	if !strings.Contains(texts, "CHK1001") || !strings.Contains(texts, "AND") {
		t.Errorf("check printout:\n%s", texts)
	}
	printed := lines()
	for _, l := range printed {
		if !l.Check_printed || !strings.HasPrefix(l.Document_no.String(), "CHK") {
			t.Errorf("line %d not printed: %s %v", l.Line_no, l.Document_no, l.Check_printed)
		}
	}
	checkEntries := func(status gtables.CheckLedgerEntryEntry_status) int {
		entry := gtables.NewCheckLedgerEntry(src)
		entry.SetRange("bank_account_no", "B010")
		entry.SetRange("entry_status", status)
		entry.SetFilter("check_no", "CHK*")
		return entry.Count()
	}
	if n := checkEntries(gtables.CheckLedgerEntry_Entry_status.Printed); n != len(printed) {
		t.Errorf("%d printed check entries, want %d", n, len(printed))
	}
	bankAcc := gtables.NewBankAccount(src)
	mustOK(t, bankAcc.GetOrError(types.Code("B010")))
	if !strings.HasPrefix(bankAcc.Last_check_no.String(), "CHK1") {
		t.Errorf("Last Check No. %s", bankAcc.Last_check_no)
	}

	// void every check: the lines are unprinted again, the entries voided
	checkMgt := checkmgt.New(src)
	for _, l := range printed {
		current := gtables.NewGenJournalLine(src)
		mustOK(t, current.GetOrError(map[string]interface{}{"journal_template_name": "PMTTEST", "journal_batch_name": "B1", "line_no": l.Line_no}))
		mustOK(t, checkMgt.VoidCheck(current))
	}
	for _, l := range lines() {
		if l.Check_printed || l.Document_no != "" {
			t.Errorf("voided line %d: %s %v", l.Line_no, l.Document_no, l.Check_printed)
		}
	}
	if n := checkEntries(gtables.CheckLedgerEntry_Entry_status.Voided); n != len(printed) {
		t.Errorf("%d voided check entries, want %d", n, len(printed))
	}

	// print again on new numbers and post: the check entries are posted
	printChecks()
	reprinted := lines()
	post := genjnl.NewGenJnlPostBatch(src)
	line := gtables.NewGenJournalLine(src)
	line.Journal_template_name, line.Journal_batch_name = "PMTTEST", "B1"
	_, err = post.Run(line)
	mustOK(t, err)
	if n := checkEntries(gtables.CheckLedgerEntry_Entry_status.Posted); n != len(reprinted) {
		t.Errorf("%d posted check entries, want %d", n, len(reprinted))
	}
}

// Summarize per Vendor: one payment line per vendor applied by Applies-to ID; its check lists the
// invoices of the ID on the stub and is one check ledger entry
func TestSuggestVendorPaymentsSummarized(t *testing.T) {
	src := listCompany(t)
	template := gtables.NewGenJournalTemplate(src)
	template.Name = "PMTSUM"
	template.Type = gtables.GenJournalTemplate_Type.Payments
	mustOK(t, template.InsertOrError(true))
	batch := gtables.NewGenJournalBatch(src)
	batch.Journal_template_name, batch.Name = "PMTSUM", "B1"
	mustOK(t, batch.InsertOrError(false))
	context := gtables.NewGenJournalLine(src)
	context.Journal_template_name, context.Journal_batch_name = "PMTSUM", "B1"
	_, err := reports.Execute(reports.SuggestVendorPaymentsID, src.GetDB(), src.GetCompany(), src.GetDBType(), reports.RunOptions{
		Context: context,
		Options: map[string]string{
			"last_payment_date": "2026-12-31", "posting_date": "2026-10-06", "summarize_per_vendor": "true",
			"bal_account_type": "2", "bal_account_no": "B010", "bank_payment_type": "2", "starting_document_no": "SUM0001",
		},
		Filters: map[string][]ftables.FieldFilter{"Vendor": {{Field: "no", Expression: "V00010"}}},
	})
	mustOK(t, err)
	line := gtables.NewGenJournalLine(src)
	line.SetRange("journal_template_name", "PMTSUM")
	if n := line.Count(); n != 1 {
		t.Fatalf("%d lines, want one per vendor", n)
	}
	if !line.FindFirst() || line.Applies_to_id == "" {
		t.Fatal("no Applies-to ID on the summarized line")
	}
	result, err := reports.Execute(reports.CheckReportID, src.GetDB(), src.GetCompany(), src.GetDBType(), reports.RunOptions{
		Context: context, Output: reports.OutputPDF,
		Options: map[string]string{"bank_account": "B010", "last_check_no": "SUMCHK100"},
	})
	mustOK(t, err)
	texts := printedTexts(result, 0)
	entry := gtables.NewCheckLedgerEntry(src)
	entry.SetRange("check_no", "SUMCHK101")
	entry.SetRange("entry_status", gtables.CheckLedgerEntry_Entry_status.Printed)
	if !entry.FindFirst() || !entry.Amount.Equal(line.Amount) {
		t.Errorf("check SUMCHK101 not printed for %s:\n%s", line.Amount, texts)
	}
}
