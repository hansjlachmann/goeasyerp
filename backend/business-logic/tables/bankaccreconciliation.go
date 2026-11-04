package tables

import (
	fcodeunits "github.com/hansjlachmann/goeasyerp/backend/foundation/codeunits"
	apperrors "github.com/hansjlachmann/goeasyerp/backend/foundation/errors"
	"github.com/hansjlachmann/goeasyerp/backend/foundation/i18n"
	"github.com/hansjlachmann/goeasyerp/backend/foundation/system"
	"github.com/hansjlachmann/goeasyerp/backend/foundation/types"
	gtables "github.com/hansjlachmann/goeasyerp/backend/generated/tables"
)

// bankAccReconciliationLogic is the business logic of table 273 Bank Acc. Reconciliation: a
// bank statement of a bank account being reconciled; its number follows the account's Last
// Statement No., and must not be a posted statement's.
type bankAccReconciliationLogic struct{}

func init() { gtables.RegisterBankAccReconciliationLogic(bankAccReconciliationLogic{}) }

// OnInsert: the statement's number is not posted yet; the bank account's Last Statement No.
// becomes it
func (bankAccReconciliationLogic) OnInsert(rec *gtables.BankAccReconciliation) error {
	err := rec.TestField("bank_account_no")
	if err == nil {
		err = rec.TestField("statement_no")
	}
	if err != nil {
		return err
	}
	postedStmt := gtables.NewBankAccountStatement(rec)
	if postedStmt.Get(map[string]interface{}{"bank_account_no": rec.Bank_account_no, "statement_no": rec.Statement_no}) {
		return apperrors.BankRecStatementExists(rec.Statement_no.String())
	}
	bankAcc := gtables.NewBankAccount(rec)
	err = bankAcc.GetOrError(rec.Bank_account_no)
	if err != nil {
		return err
	}
	bankAcc.Last_statement_no = rec.Statement_no
	return bankAcc.ModifyOrError(false)
}

// OnDelete: the lines go with it (their applications removed)
func (bankAccReconciliationLogic) OnDelete(rec *gtables.BankAccReconciliation) error {
	line := gtables.NewBankAccReconciliationLine(rec)
	line.SetRange("bank_account_no", rec.Bank_account_no)
	line.SetRange("statement_no", rec.Statement_no)
	var lines []*gtables.BankAccReconciliationLine
	for found := line.FindSet(); found; found = line.Next() {
		l := gtables.NewBankAccReconciliationLine(rec)
		l.Copy(line)
		lines = append(lines, l)
	}
	for _, l := range lines {
		err := l.DeleteOrError(true)
		if err != nil {
			return err
		}
	}
	return nil
}

func (bankAccReconciliationLogic) OnRename(rec *gtables.BankAccReconciliation) error {
	return apperrors.RenameNotAllowed(gtables.BankAccReconciliationTableName)
}

// OnValidate_Bank_account_no: a new statement takes the number after the account's last one and
// its balance
func (bankAccReconciliationLogic) OnValidate_Bank_account_no(rec *gtables.BankAccReconciliation) error {
	if rec.Statement_no != "" {
		return nil
	}
	bankAcc := gtables.NewBankAccount(rec)
	err := bankAcc.GetOrError(rec.Bank_account_no)
	if err != nil {
		return err
	}
	rec.Statement_no = types.Code(system.IncStr(bankAcc.Last_statement_no.String()))
	rec.Balance_last_statement = bankAcc.Balance_last_statement
	return nil
}

// OnValidate_Balance_last_statement: a balance other than the bank account's is kept only when
// the user confirms it (asked in a dialog)
func (bankAccReconciliationLogic) OnValidate_Balance_last_statement(rec *gtables.BankAccReconciliation) error {
	bankAcc := gtables.NewBankAccount(rec)
	err := bankAcc.GetOrError(rec.Bank_account_no)
	if err != nil {
		return err
	}
	if !rec.Balance_last_statement.Equal(bankAcc.Balance_last_statement) && system.GuiAllowed() {
		language := fcodeunits.CurrentLanguage()
		ts := i18n.GetInstance()
		if !system.Confirm(ts.Message("BANK_REC_BALANCE_LAST_STATEMENT", language),
			ts.FieldCaption(gtables.BankAccReconciliationTableName, "balance_last_statement", language),
			ts.FieldCaption(gtables.BankAccountTableName, "balance_last_statement", language),
			ts.TableCaption(gtables.BankAccountTableName, language)) {
			rec.Balance_last_statement = rec.XRec().Balance_last_statement
		}
	}
	return nil
}
