package tables

import (
	"github.com/hansjlachmann/openerp/backend/foundation/database"
	apperrors "github.com/hansjlachmann/openerp/backend/foundation/errors"
	"github.com/hansjlachmann/openerp/backend/foundation/system"
	"github.com/hansjlachmann/openerp/backend/foundation/types"
	gtables "github.com/hansjlachmann/openerp/backend/generated/tables"
)

// gLAccountLogic is the business logic of table 15 G_L_Account.
// Add a trigger when it is needed: the available triggers are listed at the
// top of generated/tables/glaccount_gen.go. The field rules of the definition
// (required, length, range) are checked after OnInsert and OnModify.
//
// Not migrated yet (their tables come later): OnDelete moves the account's G/L entries
// (codeunit 361) and deletes its extended texts.
type gLAccountLogic struct{}

func init() { gtables.RegisterGLAccountLogic(gLAccountLogic{}) }

// OnInsert runs before a new record is inserted
func (gLAccountLogic) OnInsert(rec *gtables.GLAccount) error {
	return nil
}

// OnModify sets Last Date Modified
func (gLAccountLogic) OnModify(rec *gtables.GLAccount) error {
	rec.Last_date_modified = system.Today()
	return nil
}

// OnDelete deletes the account's budget entries and comment lines
func (gLAccountLogic) OnDelete(rec *gtables.GLAccount) error {
	glBudgetEntry := gtables.NewGLBudgetEntry(rec)
	glBudgetEntry.SetCurrentKey("budget_name", "gl_account_no", "date")
	glBudgetEntry.SetRange("gl_account_no", rec.No)
	glBudgetEntry.DeleteAll()

	DeleteComments(rec, gtables.CommentLine_Table_name.GLAccount, rec.No)
	return nil
}

// OnRename sets Last Date Modified
func (gLAccountLogic) OnRename(rec *gtables.GLAccount) error {
	rec.Last_date_modified = system.Today()
	return nil
}

// OnValidate_Name: Search Name follows the name while it was blank or the old name
func (gLAccountLogic) OnValidate_Name(rec *gtables.GLAccount) error {
	xRec := rec.XRec()
	if rec.Search_name == "" || string(rec.Search_name) == system.UpperCase(xRec.Name.String()) {
		rec.Search_name = types.NewCode(system.CopyStr(rec.Name.String(), 1, 30))
	}
	return nil
}

// OnValidate_Account_type: a posting account with ledger or budget entries stays one; clears
// Totaling; a posting account allows direct posting, other types do not
func (gLAccountLogic) OnValidate_Account_type(rec *gtables.GLAccount) error {
	xRec := rec.XRec()
	if rec.Account_type != gtables.GLAccount_Account_type.Posting &&
		xRec.Account_type == gtables.GLAccount_Account_type.Posting {
		glEntry := gtables.NewGLEntry(rec)
		glEntry.SetCurrentKey("gl_account_no")
		glEntry.SetRange("gl_account_no", rec.No)
		if glEntry.FindFirst() {
			return apperrors.GLAccHasLedgerEntries(gtables.GLAccountTableName)
		}
		glBudgetEntry := gtables.NewGLBudgetEntry(rec)
		glBudgetEntry.SetCurrentKey("budget_name", "gl_account_no")
		glBudgetEntry.SetRange("gl_account_no", rec.No)
		if glBudgetEntry.FindFirst() {
			return apperrors.GLAccHasBudgetEntries(gtables.GLAccountTableName)
		}
	}
	rec.Totaling = ""
	if rec.Account_type == gtables.GLAccount_Account_type.Posting {
		if rec.Account_type != xRec.Account_type {
			rec.Direct_posting = true
		}
	} else {
		rec.Direct_posting = false
	}
	return nil
}

// OnValidate_Totaling: only Total and End-Total accounts total other accounts
func (gLAccountLogic) OnValidate_Totaling(rec *gtables.GLAccount) error {
	switch rec.Account_type {
	case gtables.GLAccount_Account_type.Total, gtables.GLAccount_Account_type.EndTotal:
		return nil
	}
	return apperrors.FieldMustNotBe(gtables.GLAccountTableName, "account_type", rec.Account_type.String())
}

// OnValidate_Gen_bus_posting_group fills in the group's default Tax Bus. Posting Group
func (gLAccountLogic) OnValidate_Gen_bus_posting_group(rec *gtables.GLAccount) error {
	xRec := rec.XRec()
	if xRec.Gen_bus_posting_group != rec.Gen_bus_posting_group {
		genBusPostingGroup := gtables.NewGenBusinessPostingGroup(rec)
		autoInsert, err := ValidateTaxBusPostingGroup(genBusPostingGroup, rec.Gen_bus_posting_group)
		if err != nil {
			return err
		}
		if autoInsert {
			err = rec.Validate("tax_bus_posting_group", genBusPostingGroup.Def_tax_bus_posting_group)
			if err != nil {
				return err
			}
		}
	}
	return nil
}

// OnValidate_Gen_prod_posting_group fills in the group's default Tax Prod. Posting Group
func (gLAccountLogic) OnValidate_Gen_prod_posting_group(rec *gtables.GLAccount) error {
	xRec := rec.XRec()
	if xRec.Gen_prod_posting_group != rec.Gen_prod_posting_group {
		genProdPostingGroup := gtables.NewGenProductPostingGroup(rec)
		autoInsert, err := ValidateTaxProdPostingGroup(genProdPostingGroup, rec.Gen_prod_posting_group)
		if err != nil {
			return err
		}
		if autoInsert {
			err = rec.Validate("tax_prod_posting_group", genProdPostingGroup.Def_tax_prod_posting_group)
			if err != nil {
				return err
			}
		}
	}
	return nil
}

// SetupNewGLAcc gives a new account the Income/Balance of the account above it: xRec when the
// new account goes below it, otherwise the account before xRec (page OnNewRecord)
func SetupNewGLAcc(rec, xRec *gtables.GLAccount, belowXRec bool) {
	above := xRec
	if !belowXRec {
		glAcc := gtables.NewGLAccount(rec)
		glAcc.No = xRec.No
		if xRec.No == "" || !glAcc.Find("<") {
			glAcc.InitRecord()
		}
		above = glAcc
	}
	rec.Income_balance = above.Income_balance
}

// GetCurrencyCode is the currency of the account's additional-currency amounts: the General
// Ledger Setup's Additional Reporting Currency
func GetCurrencyCode(rec *gtables.GLAccount) types.Code {
	return gtables.GetGeneralLedgerSetup(rec).Additional_reporting_currency
}

// IndentChartOfAccounts indents every account between a Begin-Total and its End-Total one
// level and sets each End-Total's Totaling to "<Begin-Total no.>..<End-Total no.>" (in account
// order). progress is told each account (the progress window).
func IndentChartOfAccounts(db database.Executor, company string, dbType database.DBType, progress func(no string)) error {
	var all gtables.GLAccount
	all.InitWithDBType(db, company, dbType)
	all.SetCurrentKey("no")
	var accounts []types.Code
	for found := all.FindSet(); found; found = all.Next() {
		accounts = append(accounts, all.No)
	}
	var begins []types.Code
	for _, no := range accounts {
		if progress != nil {
			progress(no.String())
		}
		var acc gtables.GLAccount
		acc.InitWithDBType(db, company, dbType)
		if !acc.Get(no) {
			continue
		}
		if acc.Account_type == gtables.GLAccount_Account_type.EndTotal {
			if len(begins) == 0 {
				return apperrors.EndTotalWithoutBeginTotal(no.String())
			}
			acc.Totaling = types.NewText(begins[len(begins)-1].String() + ".." + no.String())
			begins = begins[:len(begins)-1]
		}
		acc.Indentation = len(begins)
		err := acc.ModifyOrError(false)
		if err != nil {
			return err
		}
		if acc.Account_type == gtables.GLAccount_Account_type.BeginTotal {
			begins = append(begins, no)
		}
	}
	return nil
}

// CheckGLAcc returns an error unless the account is a posting account that is not blocked
func CheckGLAcc(rec *gtables.GLAccount) error {
	err := rec.TestField("account_type", gtables.GLAccount_Account_type.Posting)
	if err != nil {
		return err
	}
	return rec.TestField("blocked", false)
}
