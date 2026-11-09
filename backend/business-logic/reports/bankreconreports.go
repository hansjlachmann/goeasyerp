package reports

import (
	fcodeunits "github.com/hansjlachmann/goeasyerp/backend/foundation/codeunits"
	"github.com/hansjlachmann/goeasyerp/backend/foundation/types"
	gtables "github.com/hansjlachmann/goeasyerp/backend/generated/tables"
)

// The bank account reconciliation's reports:
//   - 1408 Bank Acc. Recon. - Test (Test Report on the reconciliation, Report Selections usage
//     B.Recon.Test): per reconciliation its head with what posting would refuse — the
//     statement date, the lines against the ending balance, the bank account — and its lines,
//     each with the entries applied to it checked (open, applied to this line, not on another
//     statement) and its applied amount, then the totals' checks;
//   - 1407 Bank Account Statement (Report Selections usage B.Stmt; Print on the statements and
//     Post and Print of the reconciliation): per statement its lines and totals.
const (
	BankAccountStatementID = 1407
	BankAccReconTestID     = 1408
)

func init() {
	Register(BankAccountStatementID, "bank-account-statement", func() Report { return &bankAccountStatement{} })
	Register(BankAccReconTestID, "bank-acc-recon-test", func() Report { return &bankAccReconTest{} })
	DefaultReports[gtables.ReportSelections_Usage.BStmt] = BankAccountStatementID
	DefaultReports[gtables.ReportSelections_Usage.BReconTest] = BankAccReconTestID
}

// Report 1408 Bank Acc. Recon. - Test
type bankAccReconTest struct {
	bankAccRecon                       *gtables.BankAccReconciliation
	headerErrors, lineErrors, footErrs []string
	totalAmount, totalAppliedAmount    types.Decimal
	totalDiff                          types.Decimal
}

func (r *bankAccReconTest) Definition() *Definition {
	return &Definition{
		ContextTable: gtables.BankAccReconciliationTableName,
		// run on a reconciliation (Report Selections from the reconciliation card): it alone
		OnInitReport: func(run *Run) error {
			bankAccRecon, ok := run.Context().(*gtables.BankAccReconciliation)
			if ok && bankAccRecon.Bank_account_no != "" {
				run.SetRequestFilter("Bank_Acc_Reconciliation", "bank_account_no", bankAccRecon.Bank_account_no.String())
				run.SetRequestFilter("Bank_Acc_Reconciliation", "statement_no", bankAccRecon.Statement_no.String())
			}
			return nil
		},
		OnPreReport: func(run *Run) error {
			setReportHeadValues(run)
			return nil
		},
		DataItems: []*DataItem{
			{
				Name: "Bank_Acc_Reconciliation", Table: gtables.BankAccReconciliationTableName, Key: []string{"bank_account_no", "statement_no"},
				RequestFilterFields: []string{"bank_account_no", "statement_no"},
				OnPreDataItem: Trigger(func(bankAccRecon *gtables.BankAccReconciliation, run *Run) error {
					setFilterText(run, bankAccRecon)
					return nil
				}),
				OnAfterGetRecord: Trigger(r.reconOnAfterGetRecord),
				DataItems: []*DataItem{
					r.errorLoop("Header_Error_Loop", &r.headerErrors),
					{
						Name: "Bank_Acc_Reconciliation_Line", Table: gtables.BankAccReconciliationLineTableName,
						Key:              []string{"bank_account_no", "statement_no", "statement_line_no"},
						Link:             map[string]string{"bank_account_no": "bank_account_no", "statement_no": "statement_no"},
						OnPreDataItem:    Trigger(r.lineOnPreDataItem),
						OnAfterGetRecord: Trigger(r.lineOnAfterGetRecord),
						OnPostDataItem:   Trigger(r.lineOnPostDataItem),
						DataItems:        []*DataItem{r.errorLoop("Line_Error_Loop", &r.lineErrors)},
					},
					r.errorLoop("Footer_Error_Loop", &r.footErrs),
				},
			},
		},
	}
}

// errorLoop is a data item printing one of the warning lists, one per row
func (r *bankAccReconTest) errorLoop(name string, errors *[]string) *DataItem {
	return &DataItem{
		Name: name, Table: IntegerTable,
		OnPreDataItem: Trigger(func(rec *Integer, run *Run) error {
			rec.SetRange("number", 1, len(*errors))
			return nil
		}),
		OnAfterGetRecord: Trigger(func(rec *Integer, run *Run) error {
			run.SetValue("error_text", (*errors)[rec.Number-1])
			return nil
		}),
	}
}

// reconOnAfterGetRecord: the statement date, the lines' statement amount against the ending
// balance less the last statement's balance, and the bank account
func (r *bankAccReconTest) reconOnAfterGetRecord(bankAccRecon *gtables.BankAccReconciliation, run *Run) error {
	language := fcodeunits.CurrentLanguage()
	r.bankAccRecon = bankAccRecon
	r.headerErrors, r.footErrs = nil, nil
	run.SetPageNo(1)
	if bankAccRecon.Statement_date.IsZero() {
		r.headerErrors = append(r.headerErrors, run.Text("MUST_BE_SPECIFIED", bankAccRecon.GetFieldCaption("statement_date", language)))
	}

	bankAccReconLine := gtables.NewBankAccReconciliationLine(run)
	bankAccReconLine.SetRange("bank_account_no", bankAccRecon.Bank_account_no)
	bankAccReconLine.SetRange("statement_no", bankAccRecon.Statement_no)
	err := bankAccReconLine.CalcSums("statement_amount")
	if err != nil {
		return err
	}
	if !bankAccReconLine.Statement_amount.Equal(bankAccRecon.Statement_ending_balance.Sub(bankAccRecon.Balance_last_statement)) {
		r.headerErrors = append(r.headerErrors, run.Text("NOT_EQUAL_TOTAL_BALANCE", bankAccRecon.GetFieldCaption("statement_ending_balance", language)))
	}

	bankAcc := gtables.NewBankAccount(run)
	if bankAcc.Get(bankAccRecon.Bank_account_no) {
		if bankAcc.Blocked {
			r.headerErrors = append(r.headerErrors, run.Text("MUST_BE_FOR", bankAcc.GetFieldCaption("blocked", language), formatValue(false, "", language), bankAcc.TableCaption(language), bankAccRecon.Bank_account_no))
		}
	} else {
		r.headerErrors = append(r.headerErrors, run.Text("NOT_EXIST", bankAcc.TableCaption(language), bankAccRecon.Bank_account_no))
	}
	return nil
}

func (r *bankAccReconTest) lineOnPreDataItem(line *gtables.BankAccReconciliationLine, run *Run) error {
	r.totalAmount, r.totalAppliedAmount, r.totalDiff = types.ZeroDecimal(), types.ZeroDecimal(), types.ZeroDecimal()
	return nil
}

// lineOnAfterGetRecord: the entries applied to the line — open, applied to it, their check or
// bank entry consistent — add up to its Applied Amount
func (r *bankAccReconTest) lineOnAfterGetRecord(line *gtables.BankAccReconciliationLine, run *Run) error {
	language := fcodeunits.CurrentLanguage()
	r.lineErrors = nil
	appliedAmount := types.ZeroDecimal()
	mustBeFor := func(caption string, value interface{}, tableCaption string, entryNo int) {
		r.lineErrors = append(r.lineErrors, run.Text("MUST_BE_FOR", caption, formatValue(value, "", language), tableCaption, entryNo))
	}
	switch line.Type {
	case gtables.BankAccReconciliationLine_Type.BankAccountLedgerEntry:
		bankAccLedgEntry := gtables.NewBankAccountLedgerEntry(run)
		bankAccLedgEntry.SetRange("bank_account_no", line.Bank_account_no)
		bankAccLedgEntry.SetRange("open", true)
		bankAccLedgEntry.SetRange("statement_status", gtables.BankAccountLedgerEntry_Statement_status.BankAccEntryApplied)
		bankAccLedgEntry.SetRange("statement_no", line.Statement_no)
		bankAccLedgEntry.SetRange("statement_line_no", line.Statement_line_no)
		for found := bankAccLedgEntry.FindSetBuffered(); found; found = bankAccLedgEntry.Next() {
			appliedAmount = appliedAmount.Add(bankAccLedgEntry.Amount)
			checkLedgEntry := gtables.NewCheckLedgerEntry(run)
			checkLedgEntry.SetRange("bank_account_ledger_entry_no", bankAccLedgEntry.Entry_no)
			checkLedgEntry.SetRange("open", true)
			for foundCheck := checkLedgEntry.FindSetBuffered(); foundCheck; foundCheck = checkLedgEntry.Next() {
				caption := func(field string) string { return checkLedgEntry.GetFieldCaption(field, language) }
				tableCaption := checkLedgEntry.TableCaption(language)
				if checkLedgEntry.Statement_status != gtables.CheckLedgerEntry_Statement_status.BankAccEntryApplied {
					mustBeFor(caption("statement_status"), optionCaption(gtables.CheckLedgerEntryTableName, "statement_status", gtables.CheckLedgerEntry_Statement_status.BankAccEntryApplied.String(), language), tableCaption, checkLedgEntry.Entry_no)
				}
				if checkLedgEntry.Statement_no != "" {
					mustBeFor(caption("statement_no"), "", tableCaption, checkLedgEntry.Entry_no)
				}
				if checkLedgEntry.Statement_line_no != 0 {
					mustBeFor(caption("statement_line_no"), 0, tableCaption, checkLedgEntry.Entry_no)
				}
			}
		}
	case gtables.BankAccReconciliationLine_Type.CheckLedgerEntry:
		checkLedgEntry := gtables.NewCheckLedgerEntry(run)
		checkLedgEntry.SetRange("bank_account_no", line.Bank_account_no)
		checkLedgEntry.SetRange("open", true)
		checkLedgEntry.SetRange("statement_status", gtables.CheckLedgerEntry_Statement_status.CheckEntryApplied)
		checkLedgEntry.SetRange("statement_no", line.Statement_no)
		checkLedgEntry.SetRange("statement_line_no", line.Statement_line_no)
		for found := checkLedgEntry.FindSetBuffered(); found; found = checkLedgEntry.Next() {
			appliedAmount = appliedAmount.Sub(checkLedgEntry.Amount)
			bankAccLedgEntry := gtables.NewBankAccountLedgerEntry(run)
			if !bankAccLedgEntry.Get(checkLedgEntry.Bank_account_ledger_entry_no) {
				continue
			}
			caption := func(field string) string { return bankAccLedgEntry.GetFieldCaption(field, language) }
			tableCaption := bankAccLedgEntry.TableCaption(language)
			if !bankAccLedgEntry.Open {
				mustBeFor(caption("open"), true, tableCaption, bankAccLedgEntry.Entry_no)
			}
			if bankAccLedgEntry.Statement_status != gtables.BankAccountLedgerEntry_Statement_status.CheckEntryApplied {
				mustBeFor(caption("statement_status"), optionCaption(gtables.BankAccountLedgerEntryTableName, "statement_status", gtables.BankAccountLedgerEntry_Statement_status.CheckEntryApplied.String(), language), tableCaption, bankAccLedgEntry.Entry_no)
			}
			if bankAccLedgEntry.Statement_no != "" {
				mustBeFor(caption("statement_no"), "", tableCaption, bankAccLedgEntry.Entry_no)
			}
			if bankAccLedgEntry.Statement_line_no != 0 {
				mustBeFor(caption("statement_line_no"), 0, tableCaption, bankAccLedgEntry.Entry_no)
			}
		}
	case gtables.BankAccReconciliationLine_Type.Difference:
		r.totalDiff = r.totalDiff.Add(line.Statement_amount)
	}
	if !line.Applied_amount.Equal(appliedAmount) {
		r.lineErrors = append(r.lineErrors, run.Text("MUST_BE", line.GetFieldCaption("applied_amount", language), formatValue(appliedAmount, "", language)))
	}
	r.totalAmount = r.totalAmount.Add(line.Statement_amount)
	r.totalAppliedAmount = r.totalAppliedAmount.Add(appliedAmount)
	return nil
}

// lineOnPostDataItem: the lines' statement amounts are what was applied plus the differences,
// and the total difference is the difference lines'
func (r *bankAccReconTest) lineOnPostDataItem(line *gtables.BankAccReconciliationLine, run *Run) error {
	language := fcodeunits.CurrentLanguage()
	if !r.totalAmount.Equal(r.totalAppliedAmount.Add(r.totalDiff)) {
		r.footErrs = append(r.footErrs, run.Text("APPLICATION_WRONG"))
	}
	bankAccReconLine := gtables.NewBankAccReconciliationLine(run)
	bankAccReconLine.SetRange("bank_account_no", r.bankAccRecon.Bank_account_no)
	bankAccReconLine.SetRange("statement_no", r.bankAccRecon.Statement_no)
	err := bankAccReconLine.CalcSums("difference")
	if err != nil {
		return err
	}
	if !bankAccReconLine.Difference.Equal(r.totalDiff) {
		r.footErrs = append(r.footErrs, run.Text("TOTAL_DIFFERENCE", formatValue(bankAccReconLine.Difference, "", language), formatValue(r.totalDiff, "", language)))
	}
	return nil
}

// Report 1407 Bank Account Statement
type bankAccountStatement struct{}

func (r *bankAccountStatement) Definition() *Definition {
	return &Definition{
		ContextTable: gtables.BankAccountStatementTableName,
		// run on a statement (Report Selections from a statement): it alone
		OnInitReport: func(run *Run) error {
			bankAccStmt, ok := run.Context().(*gtables.BankAccountStatement)
			if ok && bankAccStmt.Bank_account_no != "" {
				run.SetRequestFilter("Bank_Account_Statement", "bank_account_no", bankAccStmt.Bank_account_no.String())
				run.SetRequestFilter("Bank_Account_Statement", "statement_no", bankAccStmt.Statement_no.String())
			}
			return nil
		},
		OnPreReport: func(run *Run) error {
			setReportHeadValues(run)
			return nil
		},
		DataItems: []*DataItem{{
			Name: "Bank_Account_Statement", Table: gtables.BankAccountStatementTableName, Key: []string{"bank_account_no", "statement_no"},
			RequestFilterFields: []string{"bank_account_no", "statement_no"},
			OnPreDataItem: Trigger(func(bankAccStmt *gtables.BankAccountStatement, run *Run) error {
				setFilterText(run, bankAccStmt)
				return nil
			}),
			DataItems: []*DataItem{{
				Name: "Bank_Account_Statement_Line", Table: gtables.BankAccountStatementLineTableName,
				Key:  []string{"bank_account_no", "statement_no", "statement_line_no"},
				Link: map[string]string{"bank_account_no": "bank_account_no", "statement_no": "statement_no"},
			}},
		}},
	}
}
