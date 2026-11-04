package tables

import (
	"github.com/hansjlachmann/goeasyerp/backend/business-logic/noseries"
	apperrors "github.com/hansjlachmann/goeasyerp/backend/foundation/errors"
	"github.com/hansjlachmann/goeasyerp/backend/foundation/functions"
	"github.com/hansjlachmann/goeasyerp/backend/foundation/system"
	"github.com/hansjlachmann/goeasyerp/backend/foundation/types"
	gtables "github.com/hansjlachmann/goeasyerp/backend/generated/tables"
)

// bankAccountLogic is the business logic of table 270 Bank Account.
//
// Not migrated yet: 94 Balance Last Statement asks before a change (a table trigger cannot ask
// yet).
type bankAccountLogic struct{}

func init() {
	gtables.RegisterBankAccountLogic(bankAccountLogic{})
	// the No. field's "…" (Bank Account Card): pick another number series
	functions.Register("table", gtables.BankAccountTableName, "AssistEdit", functions.ForAssistEdit(AssistEditBankAccount), functions.WithAssistEdit())
}

// OnInsert: a bank account without No. gets the next number of the Bank Account Nos. series of
// the General Ledger Setup
func (bankAccountLogic) OnInsert(rec *gtables.BankAccount) error {
	if rec.No == "" {
		glSetup := gtables.GetGeneralLedgerSetup(rec)
		err := glSetup.TestField("bank_account_nos")
		if err != nil {
			return err
		}
		noSeriesMgt := noseries.New(rec)
		xRec := rec.XRec()
		err = noSeriesMgt.InitSeries(glSetup.Bank_account_nos, xRec.No_series, types.Date{}, &rec.No, &rec.No_series)
		if err != nil {
			return err
		}
	}
	return nil
}

// OnModify sets Last Date Modified
func (bankAccountLogic) OnModify(rec *gtables.BankAccount) error {
	rec.Last_date_modified = system.Today()
	return nil
}

// OnRename sets Last Date Modified
func (bankAccountLogic) OnRename(rec *gtables.BankAccount) error {
	rec.Last_date_modified = system.Today()
	return nil
}

// OnDelete moves the bank account's ledger entries away (MoveBankAccEntries); its comment
// lines are deleted
func (bankAccountLogic) OnDelete(rec *gtables.BankAccount) error {
	err := moveBankAccEntries(rec)
	if err != nil {
		return err
	}
	DeleteComments(rec, gtables.CommentLine_Table_name.BankAccount, rec.No)
	return nil
}

// moveBankAccEntries: refused while the bank account has entries in a fiscal year that is not
// closed — every entry, as long as there are no accounting periods — or open entries;
// otherwise its ledger entries and check ledger entries lose their Bank Account No.
func moveBankAccEntries(rec *gtables.BankAccount) error {
	bankAccLedgEntry := gtables.NewBankAccountLedgerEntry(rec)
	bankAccLedgEntry.LockTable()
	bankAccLedgEntry.SetCurrentKey("bank_account_no", "posting_date")
	bankAccLedgEntry.SetRange("bank_account_no", rec.No)
	accountingPeriod := gtables.NewAccountingPeriod(rec)
	accountingPeriod.SetRange("closed", false)
	if accountingPeriod.FindFirst() {
		bankAccLedgEntry.SetFilter("posting_date", ">="+accountingPeriod.Starting_date.String())
	}
	if !bankAccLedgEntry.IsEmpty() {
		return apperrors.DeleteHasEntries(gtables.BankAccountTableName, rec.No.String())
	}
	bankAccLedgEntry.SetRange("posting_date")
	bankAccLedgEntry.SetRange("open", true)
	if !bankAccLedgEntry.IsEmpty() {
		return apperrors.DeleteOpenEntries(gtables.BankAccountTableName, rec.No.String())
	}
	bankAccLedgEntry.SetRange("open")
	bankAccLedgEntry.ModifyAll("bank_account_no", types.Code(""))
	checkLedgEntry := gtables.NewCheckLedgerEntry(rec)
	checkLedgEntry.SetCurrentKey("bank_account_no")
	checkLedgEntry.SetRange("bank_account_no", rec.No)
	checkLedgEntry.ModifyAll("bank_account_no", types.Code(""))
	return nil
}

// OnValidate_No: a number typed by hand must be allowed by the Bank Account Nos. series (Manual
// Nos.) and takes the bank account out of its series; a new bank account saved with the
// number its series just issued (AssistEdit) keeps that series
func (bankAccountLogic) OnValidate_No(rec *gtables.BankAccount) error {
	xRec := rec.XRec()
	if rec.No != xRec.No {
		glSetup := gtables.GetGeneralLedgerSetup(rec)
		noSeriesMgt := noseries.New(rec)
		series, issued := noSeriesMgt.IssuedBy(glSetup.Bank_account_nos, rec.No)
		if issued && xRec.No == "" {
			rec.No_series = series
		} else {
			err := noSeriesMgt.TestManual(glSetup.Bank_account_nos)
			if err != nil {
				return err
			}
			rec.No_series = ""
		}
	}
	return nil
}

// OnValidate_Name: Search Name follows the name while it was blank or the old name
func (bankAccountLogic) OnValidate_Name(rec *gtables.BankAccount) error {
	xRec := rec.XRec()
	if rec.Search_name == "" || string(rec.Search_name) == system.UpperCase(xRec.Name.String()) {
		rec.Search_name = types.NewCode(system.CopyStr(rec.Name.String(), 1, 50))
	}
	return nil
}

// OnValidate_Currency_code: only while the bank account's balance is zero and it has no open
// ledger entries
func (bankAccountLogic) OnValidate_Currency_code(rec *gtables.BankAccount) error {
	xRec := rec.XRec()
	if rec.Currency_code != xRec.Currency_code {
		bankAcc := gtables.NewBankAccount(rec)
		bankAcc.No = rec.No
		bankAcc.CalcFields("balance", "balance_lcy")
		err := bankAcc.TestField("balance", types.ZeroDecimal())
		if err != nil {
			return err
		}
		err = bankAcc.TestField("balance_lcy", types.ZeroDecimal())
		if err != nil {
			return err
		}
		bankAccLedgEntry := gtables.NewBankAccountLedgerEntry(rec)
		bankAccLedgEntry.SetCurrentKey("bank_account_no", "open")
		bankAccLedgEntry.SetRange("bank_account_no", rec.No)
		bankAccLedgEntry.SetRange("open", true)
		if bankAccLedgEntry.FindLast() {
			return apperrors.ChangeOpenEntries(gtables.BankAccountTableName, "currency_code")
		}
	}
	return nil
}

// OnValidate_Post_code: a known post code gives the city
func (bankAccountLogic) OnValidate_Post_code(rec *gtables.BankAccount) error {
	postCode := gtables.NewPostCode(rec)
	if rec.Post_code != "" && postCode.Get(rec.Post_code) {
		rec.City = postCode.City
	}
	return nil
}

// AssistEditBankAccount lets the user pick another number series for the bank account — the
// Bank Account Nos. series or one related to it — and gives the bank account that series' next
// number (AssistEdit of the No. field); false when the user cancelled
func AssistEditBankAccount(rec *gtables.BankAccount) (bool, error) {
	glSetup := gtables.GetGeneralLedgerSetup(rec)
	err := glSetup.TestField("bank_account_nos")
	if err != nil {
		return false, err
	}
	noSeriesMgt := noseries.New(rec)
	xRec := rec.XRec()
	noSeries := rec.No_series
	picked, err := noSeriesMgt.SelectSeries(glSetup.Bank_account_nos, xRec.No_series, &noSeries)
	if err != nil {
		return false, err
	}
	if picked {
		err = noSeriesMgt.SetSeries(&rec.No)
		if err != nil {
			return false, err
		}
		rec.No_series = noSeries
	}
	return picked, nil
}
