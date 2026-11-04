package genjnl

import (
	"strings"
	"testing"

	"github.com/hansjlachmann/goeasyerp/backend/business-logic/tables"
	ftables "github.com/hansjlachmann/goeasyerp/backend/foundation/tables"
	"github.com/hansjlachmann/goeasyerp/backend/foundation/types"
	gtables "github.com/hansjlachmann/goeasyerp/backend/generated/tables"
)

// postBatch posts a batch in one transaction as Gen. Jnl.-Post does (rolled back on an error)
func postBatch(t *testing.T, src ftables.Source, template, batch string) (PostBatchResult, error) {
	t.Helper()
	tx, err := ftables.BeginTransaction(src.GetDB())
	must(t, err)
	txSrc := ftables.DBSource(tx, src.GetCompany(), src.GetDBType())
	line := newJnlLine(txSrc, template, batch, 0)
	result, err := NewGenJnlPostBatch(txSrc).Run(line)
	if err != nil {
		_ = ftables.Rollback(tx)
		return result, err
	}
	return result, ftables.Commit(tx)
}

// jnlLine adds a line to a batch: account (G/L when accType 0), amount, balancing account
// (none when balNo is "")
type lineSpec struct {
	lineNo     int
	docNo      string
	accType    gtables.GenJournalLineAccount_type
	accNo      string
	amount     string
	balType    gtables.GenJournalLineBal_account_type
	balNo      string
	bankPmt    gtables.GenJournalLineBank_payment_type
	postingDay int
	// customers and vendors
	docType        gtables.GenJournalLineDocument_type
	appliesDocType gtables.GenJournalLineApplies_to_doc_type
	appliesDocNo   string
	appliesID      string
	pmtDiscPercent string
	pmtDiscDay     int
}

func addLine(t *testing.T, src ftables.Source, template, batch string, l lineSpec) *gtables.GenJournalLine {
	t.Helper()
	line := newJnlLine(src, template, batch, l.lineNo)
	day := l.postingDay
	if day == 0 {
		day = 15
	}
	must(t, line.Validate("posting_date", date(t, day, 10, 2026)))
	line.Document_no = types.Code(l.docNo)
	genJnlTemplate := gtables.NewGenJournalTemplate(src)
	must(t, genJnlTemplate.GetOrError(types.Code(template)))
	line.Source_code = genJnlTemplate.Source_code // as SetUpNewLine gives it
	must(t, line.Validate("document_type", l.docType))
	must(t, line.Validate("account_type", l.accType))
	must(t, line.Validate("account_no", l.accNo))
	must(t, line.Validate("bal_account_type", l.balType))
	must(t, line.Validate("bal_account_no", l.balNo))
	must(t, line.Validate("amount", dec(l.amount)))
	if l.bankPmt != 0 {
		must(t, line.Validate("bank_payment_type", l.bankPmt))
	}
	line.Applies_to_doc_type = l.appliesDocType
	line.Applies_to_doc_no = types.Code(l.appliesDocNo)
	line.Applies_to_id = types.Code(l.appliesID)
	if l.pmtDiscPercent != "" {
		line.Payment_discount_percent = dec(l.pmtDiscPercent)
		line.Pmt_discount_date = date(t, l.pmtDiscDay, 10, 2026)
	}
	must(t, line.InsertOrError(true))
	return line
}

func countEntries(src ftables.Source) (gl, bank, check, reg int) {
	return gtables.NewGLEntry(src).Count(), gtables.NewBankAccountLedgerEntry(src).Count(),
		gtables.NewCheckLedgerEntry(src).Count(), gtables.NewGLRegister(src).Count()
}

// A balanced batch posts its G/L entries — one transaction per document, a G/L Register over
// them — and the posted lines are deleted
func TestPostGLBatch(t *testing.T) {
	src := demoCompany(t)
	gl0, _, _, reg0 := countEntries(src)
	lastGL := gtables.NewGLEntry(src)
	lastGL.FindLast()

	addLine(t, src, "GENERAL", "DEFAULT", lineSpec{lineNo: 10000, docNo: "G00001", accNo: "1210", amount: "1000"})
	addLine(t, src, "GENERAL", "DEFAULT", lineSpec{lineNo: 20000, docNo: "G00001", accNo: "1230", amount: "-1000"})
	addLine(t, src, "GENERAL", "DEFAULT", lineSpec{lineNo: 30000, docNo: "G00002", accNo: "1250", amount: "250", balNo: "1270"})

	result, err := postBatch(t, src, "GENERAL", "DEFAULT")
	must(t, err)
	gl1, _, _, reg1 := countEntries(src)
	if gl1-gl0 != 4 || reg1-reg0 != 1 || result.RegisterNo == 0 || result.JournalBatchName != "DEFAULT" {
		t.Fatalf("posted %d G/L entries, %d registers (result %+v)", gl1-gl0, reg1-reg0, result)
	}
	glReg := gtables.NewGLRegister(src)
	must(t, glReg.GetOrError(result.RegisterNo))
	if glReg.From_entry_no != lastGL.Entry_no+1 || glReg.To_entry_no != lastGL.Entry_no+4 || glReg.Journal_batch_name != "DEFAULT" || glReg.Source_code != "GENJNL" {
		t.Errorf("register %+v (last entry before %d)", glReg, lastGL.Entry_no)
	}
	entry := gtables.NewGLEntry(src)
	entry.SetRange("entry_no", glReg.From_entry_no, glReg.To_entry_no)
	transactions := map[int]bool{}
	var total types.Decimal
	for found := entry.FindSet(); found; found = entry.Next() {
		transactions[entry.Transaction_no] = true
		total = total.Add(entry.Amount)
		if entry.Document_no == "G00002" && entry.Gl_account_no == "1270" && !entry.Amount.Equal(dec("-250")) {
			t.Errorf("balancing entry %s", entry.Amount)
		}
		if entry.Gl_account_no == "1210" && (!entry.Debit_amount.Equal(dec("1000")) || entry.Description != "Land and Buildings") {
			t.Errorf("entry 1210: debit %s description %q", entry.Debit_amount, entry.Description)
		}
	}
	if !total.IsZero() || len(transactions) != 2 {
		t.Errorf("entries add up to %s in %d transactions", total, len(transactions))
	}
	lines := gtables.NewGenJournalLine(src)
	lines.SetRange("journal_template_name", "GENERAL")
	if lines.Count() != 0 {
		t.Errorf("posted lines left in the journal: %d", lines.Count())
	}
	// the batch's number series used G00002
	noSeriesLine := gtables.NewNoSeriesLine(src)
	noSeriesLine.SetRange("series_code", "GJNL-GEN")
	noSeriesLine.FindFirst()
	if noSeriesLine.Last_no_used != "G00002" {
		t.Errorf("GJNL-GEN last no. used %q", noSeriesLine.Last_no_used)
	}
}

// An unbalanced batch is refused and nothing is posted
func TestPostUnbalancedBatch(t *testing.T) {
	src := demoCompany(t)
	gl0, _, _, reg0 := countEntries(src)
	addLine(t, src, "GENERAL", "DEFAULT", lineSpec{lineNo: 10000, docNo: "G00001", accNo: "1210", amount: "1000"})
	addLine(t, src, "GENERAL", "DEFAULT", lineSpec{lineNo: 20000, docNo: "G00001", accNo: "1230", amount: "-900"})
	_, err := postBatch(t, src, "GENERAL", "DEFAULT")
	if err == nil || !strings.Contains(err.Error(), "out of balance by 100") {
		t.Fatalf("unbalanced: %v", err)
	}
	gl1, _, _, reg1 := countEntries(src)
	lines := gtables.NewGenJournalLine(src)
	if gl1 != gl0 || reg1 != reg0 || lines.Count() != 2 {
		t.Errorf("after the refusal: %d G/L entries, %d registers, %d lines", gl1-gl0, reg1-reg0, lines.Count())
	}
}

// The posting date must be within the allowed posting dates (User Setup, else G/L Setup)
func TestPostDateNotAllowed(t *testing.T) {
	src := demoCompany(t)
	glSetup := gtables.GetGeneralLedgerSetup(src)
	glSetup.Allow_posting_from = date(t, 1, 11, 2026)
	must(t, glSetup.ModifyOrError(false))
	addLine(t, src, "GENERAL", "DEFAULT", lineSpec{lineNo: 10000, docNo: "G00001", accNo: "1210", amount: "10", balNo: "1230"})
	_, err := postBatch(t, src, "GENERAL", "DEFAULT")
	if err == nil || !strings.Contains(err.Error(), "allowed posting dates") {
		t.Fatalf("posting before Allow Posting From: %v", err)
	}
}

// A bank payment with a manual check posts the bank ledger entry, the check ledger entry and
// the G/L entry on the bank's posting group account; a check no. is used once
func TestPostBankManualCheck(t *testing.T) {
	src := demoCompany(t)
	_, bank0, check0, _ := countEntries(src)
	addLine(t, src, "PAYMENT", "DEFAULT", lineSpec{lineNo: 10000, docNo: "CHK001", accType: gtables.GenJournalLine_Account_type.BankAccount,
		accNo: "B010", amount: "-500", balNo: "1270", bankPmt: gtables.GenJournalLine_Bank_payment_type.ManualCheck})
	_, err := postBatch(t, src, "PAYMENT", "DEFAULT")
	must(t, err)
	_, bank1, check1, _ := countEntries(src)
	if bank1-bank0 != 1 || check1-check0 != 1 {
		t.Fatalf("bank entries %d, check entries %d", bank1-bank0, check1-check0)
	}
	bankEntry := gtables.NewBankAccountLedgerEntry(src)
	bankEntry.FindLast()
	if !bankEntry.Amount.Equal(dec("-500")) || !bankEntry.Credit_amount.Equal(dec("500")) || !bankEntry.Open || bankEntry.Bank_account_no != "B010" {
		t.Errorf("bank entry %+v", bankEntry)
	}
	glEntry := gtables.NewGLEntry(src)
	glEntry.SetRange("entry_no", bankEntry.Entry_no)
	if !glEntry.FindFirst() || glEntry.Gl_account_no != "1520" || !glEntry.Amount.Equal(dec("-500")) {
		t.Errorf("bank G/L entry %s %s", glEntry.Gl_account_no, glEntry.Amount)
	}
	checkEntry := gtables.NewCheckLedgerEntry(src)
	checkEntry.FindLast()
	if checkEntry.Check_no != "CHK001" || !checkEntry.Amount.Equal(dec("500")) || checkEntry.Bank_account_ledger_entry_no != bankEntry.Entry_no {
		t.Errorf("check entry %+v", checkEntry)
	}

	addLine(t, src, "PAYMENT", "DEFAULT", lineSpec{lineNo: 10000, docNo: "CHK001", accType: gtables.GenJournalLine_Account_type.BankAccount,
		accNo: "B010", amount: "-50", balNo: "1270", bankPmt: gtables.GenJournalLine_Bank_payment_type.ManualCheck})
	_, err = postBatch(t, src, "PAYMENT", "DEFAULT")
	if err == nil || !strings.Contains(err.Error(), "already exists") {
		t.Errorf("the same check no. again: %v", err)
	}
}

// A batch with a posting number series gives the posted documents its numbers; a batch with a
// number in its name moves to the next name after posting
func TestPostingNoSeriesAndNextBatch(t *testing.T) {
	src := demoCompany(t)
	batch := gtables.NewGenJournalBatch(src)
	batch.Journal_template_name = "GENERAL"
	batch.Name = "BATCH1"
	batch.Posting_no_series = "GJNL-SALES"
	must(t, batch.InsertOrError(true))
	addLine(t, src, "GENERAL", "BATCH1", lineSpec{lineNo: 10000, docNo: "X1", accNo: "1210", amount: "10", balNo: "1230"})
	addLine(t, src, "GENERAL", "BATCH1", lineSpec{lineNo: 20000, docNo: "X2", accNo: "1210", amount: "20", balNo: "1230"})
	line := gtables.NewGenJournalLine(src)
	line.SetRange("journal_batch_name", "BATCH1")
	line.ModifyAll("posting_no_series", "GJNL-SALES")

	result, err := postBatch(t, src, "GENERAL", "BATCH1")
	must(t, err)
	if result.JournalBatchName != "BATCH2" {
		t.Errorf("next batch: %q", result.JournalBatchName)
	}
	newBatch := gtables.NewGenJournalBatch(src)
	if !newBatch.Get(tables.GenJnlBatchKey("GENERAL", "BATCH2")) || newBatch.Get(tables.GenJnlBatchKey("GENERAL", "BATCH1")) {
		t.Error("BATCH1 must have moved to BATCH2")
	}
	entry := gtables.NewGLEntry(src)
	entry.SetRange("document_no", "GS00001")
	if entry.Count() != 2 {
		t.Errorf("document X1 posted as GS00001: %d entries", entry.Count())
	}
	entry.SetRange("document_no", "GS00002")
	if entry.Count() != 2 {
		t.Errorf("document X2 posted as GS00002: %d entries", entry.Count())
	}
	// a batch without a number series gets a new empty line
	newLines := gtables.NewGenJournalLine(src)
	newLines.SetRange("journal_batch_name", "BATCH2")
	if newLines.Count() != 1 {
		t.Errorf("new empty line in BATCH2: %d", newLines.Count())
	}
}

// A G/L account's Dimension 1 Posting rule applies to the line
func TestPostDepartmentPosting(t *testing.T) {
	src := demoCompany(t)
	glAcc := gtables.NewGLAccount(src)
	must(t, glAcc.GetOrError(types.Code("1210")))
	glAcc.Dimension_1_posting = gtables.GLAccount_Dimension_1_posting.CodeMandatory
	must(t, glAcc.ModifyOrError(false))
	addLine(t, src, "GENERAL", "DEFAULT", lineSpec{lineNo: 10000, docNo: "G00001", accNo: "1210", amount: "10", balNo: "1230"})
	_, err := postBatch(t, src, "GENERAL", "DEFAULT")
	if err == nil || !strings.Contains(err.Error(), "mandatory when posting to G/L account 1210") {
		t.Errorf("department mandatory: %v", err)
	}
}

// Each posting writes its own G/L Register (the second one after the first, not over it)
func TestTwoPostingsTwoRegisters(t *testing.T) {
	src := demoCompany(t)
	addLine(t, src, "GENERAL", "DEFAULT", lineSpec{lineNo: 10000, docNo: "G00001", accNo: "1210", amount: "10", balNo: "1230"})
	first, err := postBatch(t, src, "GENERAL", "DEFAULT")
	must(t, err)
	addLine(t, src, "GENERAL", "DEFAULT", lineSpec{lineNo: 10000, docNo: "G00002", accNo: "1210", amount: "20", balNo: "1230"})
	second, err := postBatch(t, src, "GENERAL", "DEFAULT")
	must(t, err)
	reg1 := gtables.NewGLRegister(src)
	reg2 := gtables.NewGLRegister(src)
	if second.RegisterNo != first.RegisterNo+1 || !reg1.Get(first.RegisterNo) || !reg2.Get(second.RegisterNo) ||
		reg1.To_entry_no >= reg2.From_entry_no {
		t.Errorf("registers %d (%d..%d) and %d (%d..%d)", first.RegisterNo, reg1.From_entry_no, reg1.To_entry_no,
			second.RegisterNo, reg2.From_entry_no, reg2.To_entry_no)
	}
}
