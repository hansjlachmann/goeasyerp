package reports

import (
	"sort"

	"github.com/hansjlachmann/openerp/backend/business-logic/tables"
	"github.com/hansjlachmann/openerp/backend/foundation/system"
	"github.com/hansjlachmann/openerp/backend/foundation/types"
	gtables "github.com/hansjlachmann/openerp/backend/generated/tables"
)

// Report 1496 Suggest Bank Acc. Recon. Lines (Suggest Lines on the Bank Acc. Reconciliation):
// a line for each open bank account ledger entry of the statement period not on a statement —
// with Include Checks also for each open posted check (an entry with open checks comes through
// them), in date order — applied to its entry (codeunits 375/376).
//
// Report 1497 Trans. Bank Rec. to Gen. Jnl. (Transfer to General Journal): each bank account
// ledger entry line with a difference becomes a general journal line on the bank account for the
// difference, in the chosen batch.
const (
	SuggestBankAccReconLinesID = 1496
	TransBankRecToGenJnlID     = 1497
)

func init() {
	Register(SuggestBankAccReconLinesID, "suggest-bank-acc-recon-lines", func() Report { return &suggestBankAccReconLines{} })
	Register(TransBankRecToGenJnlID, "trans-bank-rec-to-gen-jnl", func() Report { return &transBankRecToGenJnl{} })
}

type suggestBankAccReconLines struct {
	bankAccRecon     *gtables.BankAccReconciliation
	bankAccReconLine *gtables.BankAccReconciliationLine
	lastLineNo       int
}

func (r *suggestBankAccReconLines) Definition() *Definition {
	return &Definition{
		OptionsCaption: "STATEMENT_PERIOD",
		Options: []Option{
			{Name: "starting_date", Type: "date"},
			{Name: "ending_date", Type: "date", Required: true},
			{Name: "include_checks", Type: "bool", Default: "false"},
		},
		Transaction:  true,
		ContextTable: gtables.BankAccReconciliationTableName,
		OnInitReport: r.onInitReport,
		DataItems: []*DataItem{{
			Name:             "bank_account",
			Table:            gtables.BankAccountTableName,
			Key:              []string{"no"},
			OnPreDataItem:    Trigger(r.bankAccOnPreDataItem),
			OnAfterGetRecord: Trigger(r.bankAccOnAfterGetRecord),
		}},
	}
}

// The reconciliation it runs from (SetStmt): the ending date is its statement date
func (r *suggestBankAccReconLines) onInitReport(run *Run) error {
	bankAccRecon, ok := run.Context().(*gtables.BankAccReconciliation)
	if !ok {
		return system.Error(run.Text("NO_RECONCILIATION"))
	}
	r.bankAccRecon = bankAccRecon
	run.SetOption("ending_date", bankAccRecon.Statement_date.String())
	return nil
}

func (r *suggestBankAccReconLines) bankAccOnPreDataItem(bankAcc *gtables.BankAccount, run *Run) error {
	if run.OptionDate("ending_date").IsZero() {
		return system.Error(run.Text("ENTER_ENDING_DATE"))
	}
	saved := gtables.NewBankAccReconciliation(run)
	err := saved.GetOrError(map[string]interface{}{"bank_account_no": r.bankAccRecon.Bank_account_no, "statement_no": r.bankAccRecon.Statement_no})
	if err != nil {
		return err
	}
	r.bankAccRecon = saved
	last := gtables.NewBankAccReconciliationLine(run)
	last.SetRange("bank_account_no", r.bankAccRecon.Bank_account_no)
	last.SetRange("statement_no", r.bankAccRecon.Statement_no)
	if last.FindLast() {
		r.lastLineNo = last.Statement_line_no
	}
	bankAcc.SetRange("no", r.bankAccRecon.Bank_account_no)
	return nil
}

// suggested is a bank account ledger entry or a check of the statement period
type suggested struct {
	date  types.Date
	bank  *gtables.BankAccountLedgerEntry
	check *gtables.CheckLedgerEntry
}

func (r *suggestBankAccReconLines) bankAccOnAfterGetRecord(bankAcc *gtables.BankAccount, run *Run) error {
	startDate, endDate := run.OptionDate("starting_date"), run.OptionDate("ending_date")
	includeChecks := run.OptionBool("include_checks")
	var items []suggested
	bankAccLedgEntry := gtables.NewBankAccountLedgerEntry(run)
	bankAccLedgEntry.SetCurrentKey("bank_account_no", "posting_date")
	bankAccLedgEntry.SetRange("bank_account_no", bankAcc.No)
	bankAccLedgEntry.SetRange("posting_date", startDate, endDate)
	bankAccLedgEntry.SetRange("open", true)
	bankAccLedgEntry.SetRange("statement_status", gtables.BankAccountLedgerEntry_Statement_status.Open)
	for found := bankAccLedgEntry.FindSetBuffered(); found; found = bankAccLedgEntry.Next() {
		e := gtables.NewBankAccountLedgerEntry(run)
		e.Copy(bankAccLedgEntry)
		items = append(items, suggested{date: e.Posting_date, bank: e})
	}
	if includeChecks {
		checkLedgEntry := gtables.NewCheckLedgerEntry(run)
		checkLedgEntry.SetRange("bank_account_no", bankAcc.No)
		checkLedgEntry.SetRange("check_date", startDate, endDate)
		checkLedgEntry.SetFilter("entry_status", "3|4") // Posted | Financially Voided
		checkLedgEntry.SetRange("open", true)
		checkLedgEntry.SetRange("statement_status", gtables.CheckLedgerEntry_Statement_status.Open)
		for found := checkLedgEntry.FindSetBuffered(); found; found = checkLedgEntry.Next() {
			c := gtables.NewCheckLedgerEntry(run)
			c.Copy(checkLedgEntry)
			items = append(items, suggested{date: c.Check_date, check: c})
		}
		// an entry with open checks is reconciled through its checks
		var kept []suggested
		for _, it := range items {
			if it.bank != nil {
				checks := gtables.NewCheckLedgerEntry(run)
				checks.SetRange("bank_account_ledger_entry_no", it.bank.Entry_no)
				checks.SetRange("open", true)
				if checks.FindFirst() {
					continue
				}
			}
			kept = append(kept, it)
		}
		items = kept
	}
	// date order; on the same date the bank account ledger entry first
	sort.SliceStable(items, func(i, j int) bool {
		if !items[i].date.Equal(items[j].date) {
			return items[i].date.Before(items[j].date)
		}
		return items[i].bank != nil && items[j].bank == nil
	})
	for _, it := range items {
		var err error
		if it.bank != nil {
			err = r.enterBankAccLine(run, it.bank)
		} else {
			err = r.enterCheckLine(run, it.check)
		}
		if err != nil {
			return err
		}
	}
	return nil
}

func (r *suggestBankAccReconLines) newLine(run *Run) *gtables.BankAccReconciliationLine {
	r.lastLineNo += 10000
	line := gtables.NewBankAccReconciliationLine(run)
	line.Init()
	line.Bank_account_no = r.bankAccRecon.Bank_account_no
	line.Statement_no = r.bankAccRecon.Statement_no
	line.Statement_line_no = r.lastLineNo
	return line
}

// enterBankAccLine: a line applied to the bank account ledger entry (EnterBankAccLine)
func (r *suggestBankAccReconLines) enterBankAccLine(run *Run, bankAccLedgEntry *gtables.BankAccountLedgerEntry) error {
	line := r.newLine(run)
	line.Transaction_date = bankAccLedgEntry.Posting_date
	line.Description = bankAccLedgEntry.Description
	line.Document_no = bankAccLedgEntry.Document_no
	line.Statement_amount = bankAccLedgEntry.Remaining_amount
	line.Applied_amount = line.Statement_amount
	line.Type = gtables.BankAccReconciliationLine_Type.BankAccountLedgerEntry
	line.Applied_entries = 1
	err := tables.SetBankAccReconNo(bankAccLedgEntry, line)
	if err != nil {
		return err
	}
	return line.InsertOrError(false)
}

// enterCheckLine: a line applied to the check (EnterCheckLine)
func (r *suggestBankAccReconLines) enterCheckLine(run *Run, checkLedgEntry *gtables.CheckLedgerEntry) error {
	line := r.newLine(run)
	line.Transaction_date = checkLedgEntry.Check_date
	line.Description = checkLedgEntry.Description
	line.Statement_amount = checkLedgEntry.Amount.Neg()
	line.Applied_amount = line.Statement_amount
	line.Type = gtables.BankAccReconciliationLine_Type.CheckLedgerEntry
	line.Check_no = checkLedgEntry.Check_no
	line.Applied_entries = 1
	err := tables.SetCheckReconNo(checkLedgEntry, line)
	if err != nil {
		return err
	}
	return line.InsertOrError(false)
}

type transBankRecToGenJnl struct {
	bankAccRecon *gtables.BankAccReconciliation
	genJnlLine   *gtables.GenJournalLine
}

func (r *transBankRecToGenJnl) Definition() *Definition {
	return &Definition{
		OptionsCaption: "OPTIONS",
		Options: []Option{
			{Name: "gen_journal_template", Type: "text", Required: true, Relation: &OptionRelation{Table: "Gen_Journal_Template", Columns: []string{"name", "description"}}},
			{Name: "gen_journal_batch", Type: "text", Required: true, Relation: &OptionRelation{Table: "Gen_Journal_Batch", Columns: []string{"journal_template_name", "name", "description"}}},
		},
		Transaction:  true,
		ContextTable: gtables.BankAccReconciliationTableName,
		OnInitReport: r.onInitReport,
		DataItems: []*DataItem{{
			Name:             "bank_acc_reconciliation_line",
			Table:            gtables.BankAccReconciliationLineTableName,
			Key:              []string{"bank_account_no", "statement_no", "statement_line_no"},
			OnPreDataItem:    Trigger(r.lineOnPreDataItem),
			OnAfterGetRecord: Trigger(r.lineOnAfterGetRecord),
		}},
	}
}

// The reconciliation it runs from (SetBankAccRecon)
func (r *transBankRecToGenJnl) onInitReport(run *Run) error {
	bankAccRecon, ok := run.Context().(*gtables.BankAccReconciliation)
	if !ok {
		return system.Error(run.Text("NO_RECONCILIATION"))
	}
	r.bankAccRecon = bankAccRecon
	return nil
}

// lineOnPreDataItem: the batch exists; new lines go after its last line, dated the work date
func (r *transBankRecToGenJnl) lineOnPreDataItem(line *gtables.BankAccReconciliationLine, run *Run) error {
	templateName := types.Code(system.UpperCase(run.Option("gen_journal_template")))
	batchName := types.Code(system.UpperCase(run.Option("gen_journal_batch")))
	genJnlTemplate := gtables.NewGenJournalTemplate(run)
	err := genJnlTemplate.GetOrError(templateName)
	if err != nil {
		return err
	}
	genJnlBatch := gtables.NewGenJournalBatch(run)
	err = genJnlBatch.GetOrError(tables.GenJnlBatchKey(templateName, batchName))
	if err != nil {
		return err
	}
	r.genJnlLine = gtables.NewGenJournalLine(run)
	r.genJnlLine.SetRange("journal_template_name", templateName)
	r.genJnlLine.SetRange("journal_batch_name", batchName)
	r.genJnlLine.LockTable()
	if !r.genJnlLine.FindLast() {
		r.genJnlLine.Journal_template_name = templateName
		r.genJnlLine.Journal_batch_name = batchName
	}
	line.SetRange("bank_account_no", r.bankAccRecon.Bank_account_no)
	line.SetRange("statement_no", r.bankAccRecon.Statement_no)
	return nil
}

// lineOnAfterGetRecord: a bank account ledger entry line with a difference gives a journal line
// on the bank account for it
func (r *transBankRecToGenJnl) lineOnAfterGetRecord(line *gtables.BankAccReconciliationLine, run *Run) error {
	if line.Difference.IsZero() || line.Type > gtables.BankAccReconciliationLine_Type.BankAccountLedgerEntry {
		return Skip()
	}
	genJnlLine := gtables.NewGenJournalLine(run)
	genJnlLine.Init()
	genJnlLine.Journal_template_name = r.genJnlLine.Journal_template_name
	genJnlLine.Journal_batch_name = r.genJnlLine.Journal_batch_name
	genJnlLine.Line_no = r.genJnlLine.Line_no + 10000
	r.genJnlLine.Line_no = genJnlLine.Line_no
	err := genJnlLine.Validate("posting_date", line.Transaction_date)
	if err != nil {
		return err
	}
	genJnlLine.Account_type = gtables.GenJournalLine_Account_type.BankAccount
	err = genJnlLine.Validate("account_no", r.bankAccRecon.Bank_account_no)
	if err != nil {
		return err
	}
	genJnlLine.Document_no = line.Document_no
	genJnlLine.Description = line.Description
	err = genJnlLine.Validate("amount", line.Difference)
	if err != nil {
		return err
	}
	return genJnlLine.InsertOrError(true)
}
