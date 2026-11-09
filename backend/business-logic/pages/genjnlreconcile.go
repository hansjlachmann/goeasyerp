package pages

import (
	"github.com/hansjlachmann/goeasyerp/backend/foundation/pages"
	"github.com/hansjlachmann/goeasyerp/backend/foundation/system"
	"github.com/hansjlachmann/goeasyerp/backend/foundation/types"
	gtables "github.com/hansjlachmann/goeasyerp/backend/generated/tables"
)

// Page 345 Reconciliation (Reconcile, Ctrl+F9 on the general journals and the journal batch
// list): per G/L account marked Reconciliation Account — and the G/L account of each bank
// account in the batch — the net change the batch would post (the amounts excluding tax) and
// the account's balance after posting (form 345 SetGenJnlLine into a temporary G/L Account Net
// Change). The page parameters journal_template_name and journal_batch_name are the batch.
const reconciliationPageID = 345

func init() {
	pages.OnFindRecords(reconciliationPageID, func(rec *gtables.GLAccountNetChange, page *pages.Open) error {
		return GenJnlReconcile(rec, types.Code(page.Param("journal_template_name")), types.Code(page.Param("journal_batch_name")))
	})
}

// genJnlReconcile holds what form 345's SetGenJnlLine works with
type genJnlReconcile struct {
	glAccNetChange      *gtables.GLAccountNetChange
	bankAcc             *gtables.BankAccount
	bankAccPostingGroup *gtables.BankAccountPostingGroup
}

// GenJnlReconcile fills glAccNetChange, a temporary G/L Account Net Change, with the batch's net
// change per reconciliation account (SetGenJnlLine)
func GenJnlReconcile(glAccNetChange *gtables.GLAccountNetChange, templateName, batchName types.Code) error {
	r := &genJnlReconcile{
		glAccNetChange:      glAccNetChange,
		bankAcc:             gtables.NewBankAccount(glAccNetChange),
		bankAccPostingGroup: gtables.NewBankAccountPostingGroup(glAccNetChange),
	}
	glAccNetChange.DeleteAll()
	glAcc := gtables.NewGLAccount(glAccNetChange)
	glAcc.SetRange("reconciliation_account", true)
	for found := glAcc.FindSetBuffered(); found; found = glAcc.Next() {
		err := r.insertGLAccNetChange(glAcc)
		if err != nil {
			return err
		}
	}
	if templateName == "" || batchName == "" {
		return nil
	}
	genJnlLine := gtables.NewGenJournalLine(glAccNetChange)
	genJnlLine.SetRange("journal_template_name", templateName)
	genJnlLine.SetRange("journal_batch_name", batchName)
	for found := genJnlLine.FindSetBuffered(); found; found = genJnlLine.Next() {
		err := r.saveNetChange(int(genJnlLine.Account_type), genJnlLine.Account_no,
			system.Round(genJnlLine.Amount_lcy.Div(types.NewDecimalFromInt(1).Add(genJnlLine.Tax_percent.Div(types.NewDecimalFromInt(100)))), types.MustDecimal("0.01")))
		if err != nil {
			return err
		}
		err = r.saveNetChange(int(genJnlLine.Bal_account_type), genJnlLine.Bal_account_no,
			system.Round(genJnlLine.Amount_lcy.Div(types.NewDecimalFromInt(1).Add(genJnlLine.Bal_tax_percent.Div(types.NewDecimalFromInt(100)))), types.MustDecimal("0.01")).Neg())
		if err != nil {
			return err
		}
	}
	return nil
}

// saveNetChange adds a line's amount to its account's net change: a G/L account only when it is
// a reconciliation account, a bank account on the G/L account of its posting group (added when
// missing); other account types are left out
func (r *genJnlReconcile) saveNetChange(accType int, accNo types.Code, netChange types.Decimal) error {
	if accNo == "" {
		return nil
	}
	glAccNetChange := r.glAccNetChange
	switch accType {
	case int(gtables.GenJournalLine_Account_type.GLAccount):
		if !glAccNetChange.Get(accNo) {
			return nil
		}
	case int(gtables.GenJournalLine_Account_type.BankAccount):
		if accNo != r.bankAcc.No {
			err := r.bankAcc.GetOrError(accNo)
			if err != nil {
				return err
			}
			err = r.bankAcc.TestField("bank_acc_posting_group")
			if err != nil {
				return err
			}
			err = r.bankAccPostingGroup.GetOrError(r.bankAcc.Bank_acc_posting_group)
			if err != nil {
				return err
			}
			err = r.bankAccPostingGroup.TestField("g_l_bank_account_no")
			if err != nil {
				return err
			}
		}
		accNo = r.bankAccPostingGroup.G_l_bank_account_no
		if !glAccNetChange.Get(accNo) {
			glAcc := gtables.NewGLAccount(glAccNetChange)
			err := glAcc.GetOrError(accNo)
			if err != nil {
				return err
			}
			err = r.insertGLAccNetChange(glAcc)
			if err != nil {
				return err
			}
		}
	default:
		return nil
	}
	glAccNetChange.Net_change_in_jnl = glAccNetChange.Net_change_in_jnl.Add(netChange)
	glAccNetChange.Balance_after_posting = glAccNetChange.Balance_after_posting.Add(netChange)
	return glAccNetChange.ModifyOrError(false)
}

// insertGLAccNetChange adds the account with its balance (InsertGLAccNetChange)
func (r *genJnlReconcile) insertGLAccNetChange(glAcc *gtables.GLAccount) error {
	glAcc.CalcFields("balance_at_date")
	glAccNetChange := r.glAccNetChange
	glAccNetChange.Init()
	glAccNetChange.No = glAcc.No
	glAccNetChange.Name = types.Text(system.CopyStr(glAcc.Name.String(), 1, 50))
	glAccNetChange.Net_change_in_jnl = types.ZeroDecimal()
	glAccNetChange.Balance_after_posting = glAcc.Balance_at_date
	return glAccNetChange.InsertOrError(false)
}
