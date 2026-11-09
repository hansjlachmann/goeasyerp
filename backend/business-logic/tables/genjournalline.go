package tables

import (
	"github.com/hansjlachmann/goeasyerp/backend/business-logic/noseries"
	"github.com/hansjlachmann/goeasyerp/backend/business-logic/salestax"
	fcodeunits "github.com/hansjlachmann/goeasyerp/backend/foundation/codeunits"
	apperrors "github.com/hansjlachmann/goeasyerp/backend/foundation/errors"
	"github.com/hansjlachmann/goeasyerp/backend/foundation/system"
	"github.com/hansjlachmann/goeasyerp/backend/foundation/types"
	gtables "github.com/hansjlachmann/goeasyerp/backend/generated/tables"
)

// genJournalLineLogic is the business logic of table 81 Gen. Journal Line.
//
// Not migrated yet: the credit limit warning when an
// amount is entered (codeunit 312), budgeted fixed assets and the FA add.-currency factor, 1099
// fields (local field series), the additional reporting currency.
// The tax fields (Tax %, Tax Base Amount, tax posting groups) are calculated with sales tax.
type genJournalLineLogic struct{}

func init() { gtables.RegisterGenJournalLineLogic(genJournalLineLogic{}) }

const genJnlLineTable = gtables.GenJournalLineTableName

// OnInsert: the line's template and batch exist; a new line has no printed check
func (genJournalLineLogic) OnInsert(rec *gtables.GenJournalLine) error {
	rec.LockTable()
	genJnlTemplate := gtables.NewGenJournalTemplate(rec)
	err := genJnlTemplate.GetOrError(rec.Journal_template_name)
	if err != nil {
		return err
	}
	genJnlBatch := gtables.NewGenJournalBatch(rec)
	err = genJnlBatch.GetOrError(GenJnlBatchKey(rec.Journal_template_name, rec.Journal_batch_name))
	if err != nil {
		return err
	}
	rec.Check_printed = false
	return nil
}

// OnModify: a line with a printed check cannot change; clearing Applies-to ID takes it off the
// entries it marked
func (genJournalLineLogic) OnModify(rec *gtables.GenJournalLine) error {
	err := rec.TestField("check_printed", false)
	if err != nil {
		return err
	}
	xRec := rec.XRec()
	if rec.Applies_to_id == "" && xRec.Applies_to_id != "" {
		return ClearCustVendApplID(xRec)
	}
	return nil
}

// OnDelete: a line with a printed check cannot be deleted; its allocations go with it; its
// Applies-to ID is taken off the entries it marked
func (genJournalLineLogic) OnDelete(rec *gtables.GenJournalLine) error {
	err := rec.TestField("check_printed", false)
	if err != nil {
		return err
	}
	genJnlAlloc := gtables.NewGenJnlAllocation(rec)
	genJnlAlloc.SetRange("journal_template_name", rec.Journal_template_name)
	genJnlAlloc.SetRange("journal_batch_name", rec.Journal_batch_name)
	genJnlAlloc.SetRange("journal_line_no", rec.Line_no)
	genJnlAlloc.DeleteAll()
	return ClearCustVendApplID(rec)
}

// IsBalanceRecurringMethod: B Balance or RB Reversing Balance (the line posts the account's
// balance)
func IsBalanceRecurringMethod(method gtables.GenJournalLineRecurring_method) bool {
	return method == gtables.GenJournalLine_Recurring_method.BBalance ||
		method == gtables.GenJournalLine_Recurring_method.RBReversingBalance
}

// OnValidate_Recurring_method: the (Reversing) Balance method posts in LCY only
func (genJournalLineLogic) OnValidate_Recurring_method(rec *gtables.GenJournalLine) error {
	if IsBalanceRecurringMethod(rec.Recurring_method) {
		return rec.TestField("currency_code", types.Code(""))
	}
	return nil
}

// checkTaxInAlloc: a line with a general posting type — its allocations must have none
// (CheckTaxInAlloc)
func checkTaxInAlloc(rec *gtables.GenJournalLine) error {
	if rec.Gen_posting_type == gtables.GenJournalLine_Gen_posting_type.Blank {
		return nil
	}
	genJnlAlloc := gtables.NewGenJnlAllocation(rec)
	genJnlAlloc.SetRange("journal_template_name", rec.Journal_template_name)
	genJnlAlloc.SetRange("journal_batch_name", rec.Journal_batch_name)
	genJnlAlloc.SetRange("journal_line_no", rec.Line_no)
	for found := genJnlAlloc.FindSet(); found; found = genJnlAlloc.Next() {
		err := CheckGenJnlAllocTax(genJnlAlloc, rec)
		if err != nil {
			return err
		}
	}
	return nil
}

// OnRename: a line with a printed check cannot be renamed
func (genJournalLineLogic) OnRename(rec *gtables.GenJournalLine) error {
	return rec.TestField("check_printed", false)
}

// GenJnlBatchKey is the key of a Gen. Journal Batch (template, name)
func GenJnlBatchKey(template, name types.Code) map[string]interface{} {
	return map[string]interface{}{"journal_template_name": template, "name": name}
}

// isCustVendFA: Customer, Vendor or Fixed Asset — two of them cannot be posted against each other
func isCustVendFA(accountType int) bool {
	return accountType == 1 || accountType == 2 || accountType == 4
}

// isCustVendBank: Customer, Vendor or Bank Account — accounts without general posting groups
func isCustVendBank(accountType int) bool {
	return accountType == 1 || accountType == 2 || accountType == 3
}

// OnValidate_Account_type: the sides must allow posting; Account No. starts again
func (genJournalLineLogic) OnValidate_Account_type(rec *gtables.GenJournalLine) error {
	if isCustVendFA(int(rec.Account_type)) && isCustVendFA(int(rec.Bal_account_type)) {
		return apperrors.JnlAccountTypes(genJnlLineTable, "account_type", "bal_account_type")
	}
	err := rec.Validate("account_no", types.Code(""))
	if err != nil {
		return err
	}
	if isCustVendBank(int(rec.Account_type)) {
		err = rec.Validate("gen_posting_type", gtables.GenJournalLine_Gen_posting_type.Blank)
		if err != nil {
			return err
		}
		err = rec.Validate("gen_bus_posting_group", types.Code(""))
		if err != nil {
			return err
		}
		err = rec.Validate("gen_prod_posting_group", types.Code(""))
		if err != nil {
			return err
		}
	}
	err = clearGenJnlLineFAFields(rec)
	if err != nil {
		return err
	}
	UpdateGenJnlLineSource(rec)
	return nil
}

// OnValidate_Account_no: the account's name, dimension 1, dimension 2, posting groups, payment
// terms and currency come to the line
func (genJournalLineLogic) OnValidate_Account_no(rec *gtables.GenJournalLine) error {
	if rec.Account_no == "" {
		if err := UpdateGenJnlLineBalance(rec); err != nil {
			return err
		}
		UpdateGenJnlLineSource(rec)
		xRec := rec.XRec()
		if xRec.Bal_account_no != "" {
			rec.Bal_gen_posting_type = gtables.GenJournalLine_Bal_gen_posting_type.Blank
			rec.Bal_gen_bus_posting_group = ""
			rec.Bal_gen_prod_posting_group = ""
			rec.Bal_tax_bus_posting_group = ""
			rec.Bal_tax_prod_posting_group = ""
			rec.Bal_tax_area_code = ""
			rec.Bal_tax_liable = false
			rec.Bal_tax_group_code = ""
		}
		return nil
	}

	switch rec.Account_type {
	case gtables.GenJournalLine_Account_type.GLAccount:
		glAcc := gtables.NewGLAccount(rec)
		err := glAcc.GetOrError(rec.Account_no)
		if err != nil {
			return err
		}
		err = checkGenJnlLineGLAcc(rec, glAcc)
		if err != nil {
			return err
		}
		replaceInfo := rec.Bal_account_no == ""
		if !replaceInfo {
			genJnlBatch := gtables.NewGenJournalBatch(rec)
			err = genJnlBatch.GetOrError(GenJnlBatchKey(rec.Journal_template_name, rec.Journal_batch_name))
			if err != nil {
				return err
			}
			replaceInfo = genJnlBatch.Bal_account_no != ""
		}
		if replaceInfo {
			rec.Description = glAcc.Name
			rec.Dimension_1_code = glAcc.Dimension_1_code
			rec.Dimension_2_code = glAcc.Dimension_2_code
		}
		if rec.Bal_account_no == "" ||
			rec.Bal_account_type == gtables.GenJournalLine_Bal_account_type.GLAccount ||
			rec.Bal_account_type == gtables.GenJournalLine_Bal_account_type.BankAccount {
			rec.Posting_group = ""
			rec.Salespers_purch_code = ""
			rec.Payment_terms_code = ""
		}
		if rec.Bal_account_no == "" {
			rec.Currency_code = ""
		}
		rec.Gen_posting_type = gtables.GenJournalLineGen_posting_type(glAcc.Gen_posting_type)
		rec.Gen_bus_posting_group = glAcc.Gen_bus_posting_group
		rec.Gen_prod_posting_group = glAcc.Gen_prod_posting_group
		rec.Tax_bus_posting_group = glAcc.Tax_bus_posting_group
		rec.Tax_prod_posting_group = glAcc.Tax_prod_posting_group
		rec.Tax_area_code = glAcc.Tax_area_code
		rec.Tax_liable = glAcc.Tax_liable
		rec.Tax_group_code = glAcc.Tax_group_code
		if !rec.Posting_date.IsZero() {
			if rec.Posting_date.IsClosing() {
				rec.Gen_posting_type = gtables.GenJournalLine_Gen_posting_type.Blank
				rec.Gen_bus_posting_group = ""
				rec.Gen_prod_posting_group = ""
				rec.Tax_bus_posting_group = ""
				rec.Tax_prod_posting_group = ""
			}
		}
	case gtables.GenJournalLine_Account_type.Customer:
		cust := gtables.NewCustomer(rec)
		err := cust.GetOrError(rec.Account_no)
		if err != nil {
			return err
		}
		err = cust.TestField("blocked", false)
		if err != nil {
			return err
		}
		rec.Description = types.Text(cust.Name)
		rec.Dimension_1_code = cust.Dimension_1_code
		rec.Dimension_2_code = cust.Dimension_2_code
		rec.Posting_group = cust.Customer_posting_group
		rec.Salespers_purch_code = cust.Salesperson_code
		rec.Payment_terms_code = cust.Payment_terms_code
		if setGenJnlLineCurrencyCode(rec, int(rec.Bal_account_type), rec.Bal_account_no) {
			err = cust.TestField("currency_code", rec.Currency_code)
			if err != nil {
				return err
			}
		} else {
			rec.Currency_code = cust.Currency_code
		}
		clearGenJnlLineGenPosting(rec)
	case gtables.GenJournalLine_Account_type.Vendor:
		vend := gtables.NewVendor(rec)
		err := vend.GetOrError(rec.Account_no)
		if err != nil {
			return err
		}
		err = vend.TestField("blocked", false)
		if err != nil {
			return err
		}
		rec.Description = types.Text(vend.Name)
		rec.Dimension_1_code = vend.Dimension_1_code
		rec.Dimension_2_code = vend.Dimension_2_code
		rec.Posting_group = vend.Vendor_posting_group
		rec.Salespers_purch_code = vend.Purchaser_code
		rec.Payment_terms_code = vend.Payment_terms_code
		if setGenJnlLineCurrencyCode(rec, int(rec.Bal_account_type), rec.Bal_account_no) {
			err = vend.TestField("currency_code", rec.Currency_code)
			if err != nil {
				return err
			}
		} else {
			rec.Currency_code = vend.Currency_code
		}
		clearGenJnlLineGenPosting(rec)
	case gtables.GenJournalLine_Account_type.BankAccount:
		bankAcc := gtables.NewBankAccount(rec)
		err := bankAcc.GetOrError(rec.Account_no)
		if err != nil {
			return err
		}
		err = bankAcc.TestField("blocked", false)
		if err != nil {
			return err
		}
		replaceInfo := rec.Bal_account_no == ""
		if !replaceInfo {
			genJnlBatch := gtables.NewGenJournalBatch(rec)
			err = genJnlBatch.GetOrError(GenJnlBatchKey(rec.Journal_template_name, rec.Journal_batch_name))
			if err != nil {
				return err
			}
			replaceInfo = genJnlBatch.Bal_account_no != ""
		}
		if replaceInfo {
			rec.Description = types.Text(bankAcc.Name)
			rec.Dimension_1_code = bankAcc.Dimension_1_code
			rec.Dimension_2_code = bankAcc.Dimension_2_code
		}
		if rec.Bal_account_no == "" ||
			rec.Bal_account_type == gtables.GenJournalLine_Bal_account_type.GLAccount ||
			rec.Bal_account_type == gtables.GenJournalLine_Bal_account_type.BankAccount {
			rec.Posting_group = ""
			rec.Salespers_purch_code = ""
			rec.Payment_terms_code = ""
		}
		if bankAcc.Currency_code == "" {
			if rec.Bal_account_no == "" {
				rec.Currency_code = ""
			}
		} else {
			if rec.Bal_account_type == gtables.GenJournalLine_Bal_account_type.BankAccount {
				err = bankAcc.TestField("currency_code", rec.Currency_code)
				if err != nil {
					return err
				}
			} else {
				rec.Currency_code = bankAcc.Currency_code
			}
		}
		clearGenJnlLineGenPosting(rec)
	case gtables.GenJournalLine_Account_type.FixedAsset:
		fa := gtables.NewFixedAsset(rec)
		err := checkGenJnlLineFA(fa, rec.Account_no)
		if err != nil {
			return err
		}
		rec.Description = fa.Description
		rec.Dimension_1_code = fa.Dimension_1_code
		rec.Dimension_2_code = fa.Dimension_2_code
		err = setGenJnlLineFADeprBook(rec, rec.Account_no)
		if err != nil {
			return err
		}
	}

	err := rec.Validate("currency_code", rec.Currency_code)
	if err != nil {
		return err
	}
	err = rec.Validate("tax_prod_posting_group", rec.Tax_prod_posting_group)
	if err != nil {
		return err
	}
	err = rec.Validate("payment_terms_code", rec.Payment_terms_code)
	if err != nil {
		return err
	}
	if err := UpdateGenJnlLineBalance(rec); err != nil {
		return err
	}
	UpdateGenJnlLineSource(rec)
	return nil
}

// clearGenJnlLineGenPosting: customers, vendors and bank accounts post without general and tax
// posting groups
func clearGenJnlLineGenPosting(rec *gtables.GenJournalLine) {
	rec.Gen_posting_type = gtables.GenJournalLine_Gen_posting_type.Blank
	rec.Gen_bus_posting_group = ""
	rec.Gen_prod_posting_group = ""
	rec.Tax_bus_posting_group = ""
	rec.Tax_prod_posting_group = ""
}

// checkGenJnlLineFA: a fixed asset on a journal line is not blocked, inactive or budgeted
func checkGenJnlLineFA(fa *gtables.FixedAsset, no types.Code) error {
	err := fa.GetOrError(no)
	if err != nil {
		return err
	}
	err = fa.TestField("blocked", false)
	if err != nil {
		return err
	}
	err = fa.TestField("inactive", false)
	if err != nil {
		return err
	}
	return fa.TestField("budgeted_asset", false)
}

// OnValidate_Posting_date: the document date follows; the exchange rate of the new date
func (genJournalLineLogic) OnValidate_Posting_date(rec *gtables.GenJournalLine) error {
	err := rec.Validate("document_date", rec.Posting_date)
	if err != nil {
		return err
	}
	return rec.Validate("currency_code", rec.Currency_code)
}

// OnValidate_Document_type: the payment terms apply to invoices
func (genJournalLineLogic) OnValidate_Document_type(rec *gtables.GenJournalLine) error {
	return rec.Validate("payment_terms_code", rec.Payment_terms_code)
}

// OnValidate_Bal_account_no: the balancing account's details come to the line (its name only
// when the line has no account)
func (genJournalLineLogic) OnValidate_Bal_account_no(rec *gtables.GenJournalLine) error {
	if rec.Bal_account_no == "" {
		if err := UpdateGenJnlLineBalance(rec); err != nil {
			return err
		}
		UpdateGenJnlLineSource(rec)
		return nil
	}

	switch rec.Bal_account_type {
	case gtables.GenJournalLine_Bal_account_type.GLAccount:
		glAcc := gtables.NewGLAccount(rec)
		err := glAcc.GetOrError(rec.Bal_account_no)
		if err != nil {
			return err
		}
		err = checkGenJnlLineGLAcc(rec, glAcc)
		if err != nil {
			return err
		}
		if rec.Account_no == "" {
			rec.Description = glAcc.Name
			rec.Dimension_1_code = glAcc.Dimension_1_code
			rec.Dimension_2_code = glAcc.Dimension_2_code
			rec.Currency_code = ""
		}
		if rec.Account_no == "" ||
			rec.Account_type == gtables.GenJournalLine_Account_type.GLAccount ||
			rec.Account_type == gtables.GenJournalLine_Account_type.BankAccount {
			rec.Posting_group = ""
			rec.Salespers_purch_code = ""
			rec.Payment_terms_code = ""
		}
		rec.Bal_gen_posting_type = gtables.GenJournalLineBal_gen_posting_type(glAcc.Gen_posting_type)
		rec.Bal_gen_bus_posting_group = glAcc.Gen_bus_posting_group
		rec.Bal_gen_prod_posting_group = glAcc.Gen_prod_posting_group
		rec.Bal_tax_bus_posting_group = glAcc.Tax_bus_posting_group
		rec.Bal_tax_prod_posting_group = glAcc.Tax_prod_posting_group
		rec.Bal_tax_area_code = glAcc.Tax_area_code
		rec.Bal_tax_liable = glAcc.Tax_liable
		rec.Bal_tax_group_code = glAcc.Tax_group_code
		if !rec.Posting_date.IsZero() {
			if rec.Posting_date.IsClosing() {
				rec.Bal_gen_bus_posting_group = ""
				rec.Bal_gen_prod_posting_group = ""
				rec.Bal_tax_bus_posting_group = ""
				rec.Bal_tax_prod_posting_group = ""
				rec.Bal_gen_posting_type = gtables.GenJournalLine_Bal_gen_posting_type.Blank
			}
		}
	case gtables.GenJournalLine_Bal_account_type.Customer:
		cust := gtables.NewCustomer(rec)
		err := cust.GetOrError(rec.Bal_account_no)
		if err != nil {
			return err
		}
		err = cust.TestField("blocked", false)
		if err != nil {
			return err
		}
		if rec.Account_no == "" {
			rec.Description = types.Text(cust.Name)
			rec.Dimension_1_code = cust.Dimension_1_code
			rec.Dimension_2_code = cust.Dimension_2_code
		}
		rec.Posting_group = cust.Customer_posting_group
		rec.Salespers_purch_code = cust.Salesperson_code
		rec.Payment_terms_code = cust.Payment_terms_code
		if rec.Account_no == "" || rec.Account_type == gtables.GenJournalLine_Account_type.GLAccount {
			rec.Currency_code = cust.Currency_code
		}
		if rec.Account_type == gtables.GenJournalLine_Account_type.BankAccount && rec.Currency_code == "" {
			rec.Currency_code = cust.Currency_code
		}
		clearGenJnlLineBalGenPosting(rec)
	case gtables.GenJournalLine_Bal_account_type.Vendor:
		vend := gtables.NewVendor(rec)
		err := vend.GetOrError(rec.Bal_account_no)
		if err != nil {
			return err
		}
		err = vend.TestField("blocked", false)
		if err != nil {
			return err
		}
		if rec.Account_no == "" {
			rec.Description = types.Text(vend.Name)
			rec.Dimension_1_code = vend.Dimension_1_code
			rec.Dimension_2_code = vend.Dimension_2_code
		}
		rec.Posting_group = vend.Vendor_posting_group
		rec.Salespers_purch_code = vend.Purchaser_code
		rec.Payment_terms_code = vend.Payment_terms_code
		if rec.Account_no == "" || rec.Account_type == gtables.GenJournalLine_Account_type.GLAccount {
			rec.Currency_code = vend.Currency_code
		}
		if rec.Account_type == gtables.GenJournalLine_Account_type.BankAccount && rec.Currency_code == "" {
			rec.Currency_code = vend.Currency_code
		}
		clearGenJnlLineBalGenPosting(rec)
	case gtables.GenJournalLine_Bal_account_type.BankAccount:
		bankAcc := gtables.NewBankAccount(rec)
		err := bankAcc.GetOrError(rec.Bal_account_no)
		if err != nil {
			return err
		}
		err = bankAcc.TestField("blocked", false)
		if err != nil {
			return err
		}
		if rec.Account_no == "" {
			rec.Description = types.Text(bankAcc.Name)
			rec.Dimension_1_code = bankAcc.Dimension_1_code
			rec.Dimension_2_code = bankAcc.Dimension_2_code
			rec.Currency_code = bankAcc.Currency_code
		}
		if rec.Account_no == "" ||
			rec.Account_type == gtables.GenJournalLine_Account_type.GLAccount ||
			rec.Account_type == gtables.GenJournalLine_Account_type.BankAccount {
			rec.Posting_group = ""
			rec.Salespers_purch_code = ""
			rec.Payment_terms_code = ""
		}
		if bankAcc.Currency_code == "" {
			if rec.Account_no == "" {
				rec.Currency_code = ""
			}
		} else {
			if rec.Account_type == gtables.GenJournalLine_Account_type.BankAccount {
				err = bankAcc.TestField("currency_code", rec.Currency_code)
				if err != nil {
					return err
				}
			} else {
				rec.Currency_code = bankAcc.Currency_code
			}
		}
		clearGenJnlLineBalGenPosting(rec)
	case gtables.GenJournalLine_Bal_account_type.FixedAsset:
		fa := gtables.NewFixedAsset(rec)
		err := checkGenJnlLineFA(fa, rec.Bal_account_no)
		if err != nil {
			return err
		}
		if rec.Account_no == "" {
			rec.Description = fa.Description
			rec.Dimension_1_code = fa.Dimension_1_code
			rec.Dimension_2_code = fa.Dimension_2_code
		}
		err = setGenJnlLineFADeprBook(rec, rec.Bal_account_no)
		if err != nil {
			return err
		}
	}

	err := rec.Validate("currency_code", rec.Currency_code)
	if err != nil {
		return err
	}
	err = rec.Validate("bal_tax_prod_posting_group", rec.Bal_tax_prod_posting_group)
	if err != nil {
		return err
	}
	err = rec.Validate("payment_terms_code", rec.Payment_terms_code)
	if err != nil {
		return err
	}
	if err := UpdateGenJnlLineBalance(rec); err != nil {
		return err
	}
	UpdateGenJnlLineSource(rec)
	return nil
}

// clearGenJnlLineBalGenPosting: a customer, vendor or bank balancing account has no general and
// tax posting groups
func clearGenJnlLineBalGenPosting(rec *gtables.GenJournalLine) {
	rec.Bal_gen_posting_type = gtables.GenJournalLine_Bal_gen_posting_type.Blank
	rec.Bal_gen_bus_posting_group = ""
	rec.Bal_gen_prod_posting_group = ""
	rec.Bal_tax_bus_posting_group = ""
	rec.Bal_tax_prod_posting_group = ""
}

// OnValidate_Currency_code: a bank account in a currency takes only its currency; the currency
// factor is the exchange rate of the posting date
//
// The classic code also takes the rate again when the user enters the same currency
// (CurrFieldNo); here when the currency or the posting date changed or there is no factor yet.
func (genJournalLineLogic) OnValidate_Currency_code(rec *gtables.GenJournalLine) error {
	if rec.Bal_account_type == gtables.GenJournalLine_Bal_account_type.BankAccount {
		bankAcc3 := gtables.NewBankAccount(rec)
		if bankAcc3.Get(rec.Bal_account_no) && bankAcc3.Currency_code != "" {
			err := bankAcc3.TestField("currency_code", rec.Currency_code)
			if err != nil {
				return err
			}
		}
	}
	if IsBalanceRecurringMethod(rec.Recurring_method) && rec.Currency_code != "" {
		return apperrors.JnlRecurringMethodCurrency(genJnlLineTable, rec.Recurring_method)
	}
	if rec.Account_type == gtables.GenJournalLine_Account_type.BankAccount {
		bankAcc3 := gtables.NewBankAccount(rec)
		if bankAcc3.Get(rec.Account_no) && bankAcc3.Currency_code != "" {
			err := bankAcc3.TestField("currency_code", rec.Currency_code)
			if err != nil {
				return err
			}
		}
	}
	if rec.Currency_code != "" {
		_, err := getGenJnlLineCurrency(rec)
		if err != nil {
			return err
		}
		xRec := rec.XRec()
		if rec.Currency_code != xRec.Currency_code ||
			rec.Posting_date != xRec.Posting_date ||
			rec.Currency_factor.IsZero() {
			rec.Currency_factor, err = ExchangeRates(rec).ExchangeRate(rec.Posting_date, rec.Currency_code)
			if err != nil {
				return err
			}
		}
	} else {
		rec.Currency_factor = types.Decimal{}
	}
	return rec.Validate("currency_factor", rec.Currency_factor)
}

// OnValidate_Amount: rounded to the currency; Amount (LCY) at the line's exchange rate; the
// tax amounts and the line balance follow
func (genJournalLineLogic) OnValidate_Amount(rec *gtables.GenJournalLine) error {
	currency, err := getGenJnlLineCurrency(rec)
	if err != nil {
		return err
	}
	rec.Amount = system.Round(rec.Amount, currency.Amount_rounding_precision)
	currFieldNo := fcodeunits.CurrFieldNo()
	if currFieldNo != 0 && currFieldNo != rec.FieldNo("applies_to_doc_no") &&
		((rec.Account_type == gtables.GenJournalLine_Account_type.Customer && rec.Account_no != "" && rec.Amount.IsPositive() &&
			currFieldNo != rec.FieldNo("bal_account_no")) ||
			(rec.Bal_account_type == gtables.GenJournalLine_Bal_account_type.Customer && rec.Bal_account_no != "" && rec.Amount.IsNegative() &&
				currFieldNo != rec.FieldNo("account_no"))) {
		err = CustCheckCrLimitGenJnlLine(rec)
		if err != nil {
			return err
		}
	}
	if rec.Currency_code == "" {
		rec.Amount_lcy = rec.Amount
	} else {
		amountLCY, err := ExchangeRates(rec).ExchangeAmtFCYToLCY(rec.Posting_date, rec.Currency_code, rec.Amount, rec.Currency_factor)
		if err != nil {
			return err
		}
		rec.Amount_lcy = system.Round(amountLCY)
	}
	err = validateGenJnlLineTaxPercent(rec)
	if err != nil {
		return err
	}
	err = validateGenJnlLineBalTaxPercent(rec)
	if err != nil {
		return err
	}
	if err := UpdateGenJnlLineBalance(rec); err != nil {
		return err
	}
	return nil
}

// OnValidate_Debit_amount: a debit amount (negative: a correction) is the amount
func (genJournalLineLogic) OnValidate_Debit_amount(rec *gtables.GenJournalLine) error {
	currency, err := getGenJnlLineCurrency(rec)
	if err != nil {
		return err
	}
	rec.Debit_amount = system.Round(rec.Debit_amount, currency.Amount_rounding_precision)
	rec.Correction = rec.Debit_amount.IsNegative()
	return rec.Validate("amount", rec.Debit_amount)
}

// OnValidate_Credit_amount: a credit amount (negative: a correction) is the negative amount
func (genJournalLineLogic) OnValidate_Credit_amount(rec *gtables.GenJournalLine) error {
	currency, err := getGenJnlLineCurrency(rec)
	if err != nil {
		return err
	}
	rec.Credit_amount = system.Round(rec.Credit_amount, currency.Amount_rounding_precision)
	rec.Correction = rec.Credit_amount.IsNegative()
	return rec.Validate("amount", rec.Credit_amount.Neg())
}

// OnValidate_Amount_lcy: without a currency it is the amount; in a currency it sets the
// amount (fixed exchange rates) or the currency factor
func (genJournalLineLogic) OnValidate_Amount_lcy(rec *gtables.GenJournalLine) error {
	if rec.Currency_code == "" {
		return rec.Validate("amount", rec.Amount_lcy)
	} else {
		err := rec.TestField("amount")
		if err != nil {
			return err
		}
		if checkGenJnlLineFixedCurrency(rec) {
			amount, err := ExchangeRates(rec).ExchangeAmtLCYToFCY(rec.Posting_date, rec.Currency_code, rec.Amount_lcy, rec.Currency_factor)
			if err != nil {
				return err
			}
			rec.Amount = system.Round(amount)
		} else {
			rec.Currency_factor = types.Decimal{Decimal: rec.Amount.Decimal.Div(rec.Amount_lcy.Decimal)}
		}
		err = validateGenJnlLineTaxPercent(rec)
		if err != nil {
			return err
		}
		err = validateGenJnlLineBalTaxPercent(rec)
		if err != nil {
			return err
		}
		if err := UpdateGenJnlLineBalance(rec); err != nil {
			return err
		}
	}
	return nil
}

// OnValidate_Currency_factor: only with a currency; the amount is converted again
func (genJournalLineLogic) OnValidate_Currency_factor(rec *gtables.GenJournalLine) error {
	if rec.Currency_code == "" && !rec.Currency_factor.IsZero() {
		return apperrors.JnlWithoutField(genJnlLineTable, "currency_factor", "currency_code")
	}
	return rec.Validate("amount", rec.Amount)
}

// OnValidate_Quantity: the amount (its tax) again
func (genJournalLineLogic) OnValidate_Quantity(rec *gtables.GenJournalLine) error {
	return rec.Validate("amount", rec.Amount)
}

// OnValidate_Payment_terms_code: an invoice of a customer or vendor gets its due date and
// payment discount from the payment terms; other lines are due on the posting date
func (genJournalLineLogic) OnValidate_Payment_terms_code(rec *gtables.GenJournalLine) error {
	rec.Due_date = types.Date{}
	rec.Pmt_discount_date = types.Date{}
	rec.Payment_discount_percent = types.Decimal{}
	if rec.Account_type != gtables.GenJournalLine_Account_type.GLAccount ||
		rec.Bal_account_type != gtables.GenJournalLine_Bal_account_type.GLAccount {
		if rec.Document_type == gtables.GenJournalLine_Document_type.Invoice &&
			rec.Payment_terms_code != "" && !rec.Document_date.IsZero() {
			paymentTerms := gtables.NewPaymentTerms(rec)
			err := paymentTerms.GetOrError(rec.Payment_terms_code)
			if err != nil {
				return err
			}
			rec.Due_date, err = CalcDateFormula(paymentTerms.Due_date_calculation, rec.Document_date)
			if err != nil {
				return err
			}
			rec.Pmt_discount_date, err = CalcDateFormula(paymentTerms.Discount_date_calculation, rec.Document_date)
			if err != nil {
				return err
			}
			rec.Payment_discount_percent = paymentTerms.Discount_percent
		} else {
			rec.Due_date = rec.Posting_date
		}
	}
	return nil
}

// OnValidate_Applies_to_id: a changed Applies-to ID is taken off the entries the old one marked
func (genJournalLineLogic) OnValidate_Applies_to_id(rec *gtables.GenJournalLine) error {
	xRec := rec.XRec()
	if rec.Applies_to_id != xRec.Applies_to_id && xRec.Applies_to_id != "" {
		return ClearCustVendApplID(xRec)
	}
	return nil
}

// OnValidate_Gen_posting_type: customers, vendors and bank accounts have none; Settlement is
// the system's
func (genJournalLineLogic) OnValidate_Gen_posting_type(rec *gtables.GenJournalLine) error {
	if isCustVendBank(int(rec.Account_type)) {
		err := rec.TestField("gen_posting_type", gtables.GenJournalLine_Gen_posting_type.Blank)
		if err != nil {
			return err
		}
	}
	if rec.Gen_posting_type == gtables.GenJournalLine_Gen_posting_type.Settlement && !rec.System_created_entry {
		return apperrors.JnlOptionInternal(genJnlLineTable, "gen_posting_type", rec.Gen_posting_type)
	}
	err := checkTaxInAlloc(rec)
	if err != nil {
		return err
	}
	if rec.Gen_posting_type > 0 {
		return rec.Validate("tax_prod_posting_group", rec.Tax_prod_posting_group)
	}
	return nil
}

// OnValidate_Gen_bus_posting_group: the group's default tax business posting group follows
func (genJournalLineLogic) OnValidate_Gen_bus_posting_group(rec *gtables.GenJournalLine) error {
	if isCustVendBank(int(rec.Account_type)) {
		err := rec.TestField("gen_bus_posting_group", types.Code(""))
		if err != nil {
			return err
		}
	}
	xRec := rec.XRec()
	if xRec.Gen_bus_posting_group != rec.Gen_bus_posting_group {
		genBusPostingGroup := gtables.NewGenBusinessPostingGroup(rec)
		autoInsert, err := ValidateTaxBusPostingGroup(genBusPostingGroup, rec.Gen_bus_posting_group)
		if err != nil {
			return err
		}
		if autoInsert {
			return rec.Validate("tax_bus_posting_group", genBusPostingGroup.Def_tax_bus_posting_group)
		}
	}
	return nil
}

// OnValidate_Gen_prod_posting_group: the group's default tax product posting group follows
func (genJournalLineLogic) OnValidate_Gen_prod_posting_group(rec *gtables.GenJournalLine) error {
	if isCustVendBank(int(rec.Account_type)) {
		err := rec.TestField("gen_prod_posting_group", types.Code(""))
		if err != nil {
			return err
		}
	}
	xRec := rec.XRec()
	if xRec.Gen_prod_posting_group != rec.Gen_prod_posting_group {
		genProdPostingGroup := gtables.NewGenProductPostingGroup(rec)
		autoInsert, err := ValidateTaxProdPostingGroup(genProdPostingGroup, rec.Gen_prod_posting_group)
		if err != nil {
			return err
		}
		if autoInsert {
			return rec.Validate("tax_prod_posting_group", genProdPostingGroup.Def_tax_prod_posting_group)
		}
	}
	return nil
}

// OnValidate_Bal_account_type: not Employee; the sides must allow posting; Bal. Account No.
// starts again
func (genJournalLineLogic) OnValidate_Bal_account_type(rec *gtables.GenJournalLine) error {
	if rec.Bal_account_type == gtables.GenJournalLine_Bal_account_type.Employee {
		return apperrors.JnlOptionNotAllowed(genJnlLineTable, "bal_account_type", rec.Bal_account_type)
	}
	if isCustVendFA(int(rec.Account_type)) && isCustVendFA(int(rec.Bal_account_type)) {
		return apperrors.JnlAccountTypes(genJnlLineTable, "account_type", "bal_account_type")
	}
	err := rec.Validate("bal_account_no", types.Code(""))
	if err != nil {
		return err
	}
	if isCustVendBank(int(rec.Bal_account_type)) {
		err = rec.Validate("bal_gen_posting_type", gtables.GenJournalLine_Bal_gen_posting_type.Blank)
		if err != nil {
			return err
		}
		err = rec.Validate("bal_gen_bus_posting_group", types.Code(""))
		if err != nil {
			return err
		}
		err = rec.Validate("bal_gen_prod_posting_group", types.Code(""))
		if err != nil {
			return err
		}
	}
	err = clearGenJnlLineFAFields(rec)
	if err != nil {
		return err
	}
	UpdateGenJnlLineSource(rec)
	return nil
}

// OnValidate_Bal_gen_posting_type: customers, vendors and bank accounts have none; Settlement
// is the system's
func (genJournalLineLogic) OnValidate_Bal_gen_posting_type(rec *gtables.GenJournalLine) error {
	if isCustVendBank(int(rec.Bal_account_type)) {
		err := rec.TestField("bal_gen_posting_type", gtables.GenJournalLine_Bal_gen_posting_type.Blank)
		if err != nil {
			return err
		}
	}
	if rec.Bal_gen_posting_type == gtables.GenJournalLine_Bal_gen_posting_type.Settlement && !rec.System_created_entry {
		return apperrors.JnlOptionInternal(genJnlLineTable, "bal_gen_posting_type", rec.Bal_gen_posting_type)
	}
	if rec.Bal_gen_posting_type > 0 {
		return rec.Validate("bal_tax_prod_posting_group", rec.Bal_tax_prod_posting_group)
	}
	return nil
}

// OnValidate_Bal_gen_bus_posting_group: the group's default tax business posting group follows
func (genJournalLineLogic) OnValidate_Bal_gen_bus_posting_group(rec *gtables.GenJournalLine) error {
	if isCustVendBank(int(rec.Bal_account_type)) {
		err := rec.TestField("bal_gen_bus_posting_group", types.Code(""))
		if err != nil {
			return err
		}
	}
	xRec := rec.XRec()
	if xRec.Bal_gen_bus_posting_group != rec.Bal_gen_bus_posting_group {
		genBusPostingGroup := gtables.NewGenBusinessPostingGroup(rec)
		autoInsert, err := ValidateTaxBusPostingGroup(genBusPostingGroup, rec.Bal_gen_bus_posting_group)
		if err != nil {
			return err
		}
		if autoInsert {
			return rec.Validate("bal_tax_bus_posting_group", genBusPostingGroup.Def_tax_bus_posting_group)
		}
	}
	return nil
}

// OnValidate_Bal_gen_prod_posting_group: the group's default tax product posting group follows
func (genJournalLineLogic) OnValidate_Bal_gen_prod_posting_group(rec *gtables.GenJournalLine) error {
	if isCustVendBank(int(rec.Bal_account_type)) {
		err := rec.TestField("bal_gen_prod_posting_group", types.Code(""))
		if err != nil {
			return err
		}
	}
	xRec := rec.XRec()
	if xRec.Bal_gen_prod_posting_group != rec.Bal_gen_prod_posting_group {
		genProdPostingGroup := gtables.NewGenProductPostingGroup(rec)
		autoInsert, err := ValidateTaxProdPostingGroup(genProdPostingGroup, rec.Bal_gen_prod_posting_group)
		if err != nil {
			return err
		}
		if autoInsert {
			return rec.Validate("bal_tax_prod_posting_group", genProdPostingGroup.Def_tax_prod_posting_group)
		}
	}
	return nil
}

// OnValidate_Bank_payment_type: only for lines with a bank account
func (genJournalLineLogic) OnValidate_Bank_payment_type(rec *gtables.GenJournalLine) error {
	if rec.Bank_payment_type != gtables.GenJournalLine_Bank_payment_type.Blank &&
		rec.Account_type != gtables.GenJournalLine_Account_type.BankAccount &&
		rec.Bal_account_type != gtables.GenJournalLine_Bal_account_type.BankAccount {
		return apperrors.JnlBankAccountRequired(genJnlLineTable, "account_type", "bal_account_type")
	}
	if rec.Account_type == gtables.GenJournalLine_Account_type.FixedAsset &&
		rec.Bank_payment_type != gtables.GenJournalLine_Bank_payment_type.Blank {
		return rec.FieldError("account_type", "")
	}
	return nil
}

// OnValidate_Correction: the debit/credit split of the amount follows
func (genJournalLineLogic) OnValidate_Correction(rec *gtables.GenJournalLine) error {
	return rec.Validate("amount", rec.Amount)
}

// OnValidate_Document_date: the due date and payment discount follow
func (genJournalLineLogic) OnValidate_Document_date(rec *gtables.GenJournalLine) error {
	return rec.Validate("payment_terms_code", rec.Payment_terms_code)
}

// OnValidate_Source_type: the source comes from the line's accounts
func (genJournalLineLogic) OnValidate_Source_type(rec *gtables.GenJournalLine) error {
	if genJnlLineHasSource(rec) {
		UpdateGenJnlLineSource(rec)
	} else {
		rec.Source_no = ""
	}
	return nil
}

// OnValidate_Source_no: the source comes from the line's accounts
func (genJournalLineLogic) OnValidate_Source_no(rec *gtables.GenJournalLine) error {
	if genJnlLineHasSource(rec) {
		UpdateGenJnlLineSource(rec)
	}
	return nil
}

func genJnlLineHasSource(rec *gtables.GenJournalLine) bool {
	return rec.Account_type != gtables.GenJournalLine_Account_type.GLAccount && rec.Account_no != "" ||
		rec.Bal_account_type != gtables.GenJournalLine_Bal_account_type.GLAccount && rec.Bal_account_no != ""
}

// GenJnlLineEmptyLine: a line without account and amount (a system-created line with a
// balancing account is not empty) — EmptyLine
func GenJnlLineEmptyLine(rec *gtables.GenJournalLine) bool {
	return rec.Account_no == "" && rec.Amount.IsZero() &&
		(rec.Bal_account_no == "" || !rec.System_created_entry)
}

// UpdateGenJnlLineBalance sets Debit/Credit Amount from the amount and Balance (LCY): what the
// line adds to the journal's balance (0 when it has both an account and a balancing account);
// the line's allocations get their shares and are part of the balance — UpdateLineBalance
func UpdateGenJnlLineBalance(rec *gtables.GenJournalLine) error {
	if rec.Amount.IsPositive() && !rec.Correction || rec.Amount.IsNegative() && rec.Correction {
		rec.Debit_amount = rec.Amount
		rec.Credit_amount = types.Decimal{}
	} else {
		rec.Debit_amount = types.Decimal{}
		rec.Credit_amount = rec.Amount.Neg()
	}
	if rec.Currency_code == "" {
		rec.Amount_lcy = rec.Amount
	}
	switch {
	case rec.Account_no != "" && rec.Bal_account_no != "":
		rec.Balance_lcy = types.Decimal{}
	case rec.Bal_account_no != "":
		rec.Balance_lcy = rec.Amount_lcy.Neg()
	default:
		rec.Balance_lcy = rec.Amount_lcy
	}
	return UpdateGenJnlAllocations(rec)
}

// SetUpNewGenJnlLine gives a new line its values (page OnNewRecord): the posting date and
// document no. of the last line — the next document no. below a balanced last line — or
// today and the batch's next number in an empty batch; the account type and document type of
// the last line; source code, reason code, posting no. series and balancing account of the
// template and batch — SetUpNewLine
func SetUpNewGenJnlLine(rec, lastGenJnlLine *gtables.GenJournalLine, balance types.Decimal, bottomLine bool) error {
	genJnlTemplate := gtables.NewGenJournalTemplate(rec)
	err := genJnlTemplate.GetOrError(rec.Journal_template_name)
	if err != nil {
		return err
	}
	genJnlBatch := gtables.NewGenJournalBatch(rec)
	err = genJnlBatch.GetOrError(GenJnlBatchKey(rec.Journal_template_name, rec.Journal_batch_name))
	if err != nil {
		return err
	}
	genJnlLine := gtables.NewGenJournalLine(rec)
	genJnlLine.SetRange("journal_template_name", rec.Journal_template_name)
	genJnlLine.SetRange("journal_batch_name", rec.Journal_batch_name)
	if genJnlLine.FindFirst() {
		rec.Posting_date = lastGenJnlLine.Posting_date
		rec.Document_date = lastGenJnlLine.Posting_date
		rec.Document_no = lastGenJnlLine.Document_no
		if bottomLine &&
			balance.Sub(lastGenJnlLine.Balance_lcy).IsZero() &&
			!GenJnlLineEmptyLine(lastGenJnlLine) {
			rec.Document_no = types.Code(system.IncStr(string(rec.Document_no)))
		}
	} else {
		rec.Posting_date = system.WorkDate()
		rec.Document_date = system.WorkDate()
		if genJnlBatch.No_series != "" {
			noSeriesMgt := noseries.New(rec)
			rec.Document_no = noSeriesMgt.TryGetNextNo(genJnlBatch.No_series, rec.Posting_date)
		}
	}
	rec.Recurring_method = lastGenJnlLine.Recurring_method
	rec.Account_type = lastGenJnlLine.Account_type
	rec.Document_type = lastGenJnlLine.Document_type
	rec.Source_code = genJnlTemplate.Source_code
	rec.Reason_code = genJnlBatch.Reason_code
	rec.Posting_no_series = genJnlBatch.Posting_no_series
	rec.Bal_account_type = gtables.GenJournalLineBal_account_type(genJnlBatch.Bal_account_type)
	if isCustVendFA(int(rec.Account_type)) && isCustVendFA(int(rec.Bal_account_type)) {
		rec.Account_type = gtables.GenJournalLine_Account_type.GLAccount
	}
	err = rec.Validate("bal_account_no", genJnlBatch.Bal_account_no)
	if err != nil {
		return err
	}
	rec.Description = ""
	return nil
}

// setGenJnlLineCurrencyCode sets the line's currency to the other side's account's currency
// and tells whether it has one — SetCurrencyCode
func setGenJnlLineCurrencyCode(rec *gtables.GenJournalLine, accType2 int, accNo2 types.Code) bool {
	rec.Currency_code = ""
	if accNo2 != "" {
		switch accType2 {
		case 1:
			cust2 := gtables.NewCustomer(rec)
			if cust2.Get(accNo2) {
				rec.Currency_code = cust2.Currency_code
			}
		case 2:
			vend2 := gtables.NewVendor(rec)
			if vend2.Get(accNo2) {
				rec.Currency_code = vend2.Currency_code
			}
		case 3:
			bankAcc2 := gtables.NewBankAccount(rec)
			if bankAcc2.Get(accNo2) {
				rec.Currency_code = bankAcc2.Currency_code
			}
		}
	}
	return rec.Currency_code != ""
}

// getGenJnlLineCurrency returns the line's currency (its rounding precisions), the local
// currency's when it has none — GetCurrency; a line of an additional-currency amount only rounds
// as the additional reporting currency
func getGenJnlLineCurrency(rec *gtables.GenJournalLine) (*gtables.Currency, error) {
	currency := gtables.NewCurrency(rec)
	currencyCode := rec.Currency_code
	if rec.Additional_currency_posting == gtables.GenJournalLine_Additional_currency_posting.AdditionalCurrencyAmountOnly {
		currencyCode = gtables.GetGeneralLedgerSetup(rec).Additional_reporting_currency
	}
	if currencyCode == "" {
		InitRoundingPrecision(currency)
		return currency, nil
	}
	err := currency.GetOrError(currencyCode)
	if err != nil {
		return nil, err
	}
	err = currency.TestField("amount_rounding_precision")
	if err != nil {
		return nil, err
	}
	return currency, nil
}

// UpdateGenJnlLineSource: the line's source is its customer, vendor, bank account or fixed
// asset — of the account or of the balancing account, when only one of them is one
func UpdateGenJnlLineSource(rec *gtables.GenJournalLine) {
	sourceExists1 := rec.Account_type != gtables.GenJournalLine_Account_type.GLAccount && rec.Account_no != ""
	sourceExists2 := rec.Bal_account_type != gtables.GenJournalLine_Bal_account_type.GLAccount && rec.Bal_account_no != ""
	switch {
	case sourceExists1 && !sourceExists2:
		rec.Source_type = gtables.GenJournalLineSource_type(rec.Account_type)
		rec.Source_no = rec.Account_no
	case sourceExists2 && !sourceExists1:
		rec.Source_type = gtables.GenJournalLineSource_type(rec.Bal_account_type)
		rec.Source_no = rec.Bal_account_no
	default:
		rec.Source_type = gtables.GenJournalLine_Source_type.Blank
		rec.Source_no = ""
	}
}

// checkGenJnlLineGLAcc: a posting account that is not blocked; on a journal line it must
// allow direct posting (not system-created lines, not closing dates) — CheckGLAcc
func checkGenJnlLineGLAcc(rec *gtables.GenJournalLine, glAcc *gtables.GLAccount) error {
	err := CheckGLAcc(glAcc)
	if err != nil {
		return err
	}
	if glAcc.Direct_posting || rec.Journal_template_name == "" || rec.System_created_entry {
		return nil
	}
	if !rec.Posting_date.IsZero() {
		if rec.Posting_date.IsClosing() {
			return nil
		}
	}
	return glAcc.TestField("direct_posting", true)
}

// ClearCustVendApplID takes the line's Applies-to ID off the open entries of its customer or
// vendor it marked
func ClearCustVendApplID(rec *gtables.GenJournalLine) error {
	accType := int(rec.Account_type)
	accNo := rec.Account_no
	if rec.Bal_account_type == gtables.GenJournalLine_Bal_account_type.Customer ||
		rec.Bal_account_type == gtables.GenJournalLine_Bal_account_type.Vendor {
		accType = int(rec.Bal_account_type)
		accNo = rec.Bal_account_no
	}
	switch accType {
	case 1:
		if rec.Applies_to_id != "" {
			custLedgEntry := gtables.NewCustomerLedgerEntry(rec)
			custLedgEntry.SetCurrentKey("customer_no", "open")
			custLedgEntry.SetRange("customer_no", accNo)
			custLedgEntry.SetRange("open", true)
			custLedgEntry.SetRange("applies_to_id", rec.Applies_to_id)
			for found := custLedgEntry.FindSetBuffered(); found; found = custLedgEntry.Next() { // modified in the loop: read all first
				custLedgEntry.Applies_to_id = ""
				err := custLedgEntry.ModifyOrError(false)
				if err != nil {
					return err
				}
			}
		}
	case 2:
		if rec.Applies_to_id != "" {
			vendLedgEntry := gtables.NewVendorLedgerEntry(rec)
			vendLedgEntry.SetCurrentKey("vendor_no", "open")
			vendLedgEntry.SetRange("vendor_no", accNo)
			vendLedgEntry.SetRange("open", true)
			vendLedgEntry.SetRange("applies_to_id", rec.Applies_to_id)
			for found := vendLedgEntry.FindSetBuffered(); found; found = vendLedgEntry.Next() { // modified in the loop: read all first
				vendLedgEntry.Applies_to_id = ""
				err := vendLedgEntry.ModifyOrError(false)
				if err != nil {
					return err
				}
			}
		}
	}
	return nil
}

// checkGenJnlLineFixedCurrency: the exchange rate of the line's currency (and its relational
// currency) is fixed for both amounts — CheckFixedCurrency
func checkGenJnlLineFixedCurrency(rec *gtables.GenJournalLine) bool {
	currencyExchRate := gtables.NewCurrencyExchangeRate(rec)
	currencyExchRate.SetRange("currency_code", rec.Currency_code)
	currencyExchRate.SetRange("starting_date", types.Date{}, rec.Posting_date)
	if !currencyExchRate.FindLast() {
		return false
	}
	if currencyExchRate.Relational_currency_code == "" {
		return currencyExchRate.Fix_exchange_rate_amount == gtables.CurrencyExchangeRate_Fix_exchange_rate_amount.Both
	}
	if currencyExchRate.Fix_exchange_rate_amount != gtables.CurrencyExchangeRate_Fix_exchange_rate_amount.Both {
		return false
	}
	currencyExchRate.SetRange("currency_code", currencyExchRate.Relational_currency_code)
	if currencyExchRate.FindLast() {
		return currencyExchRate.Fix_exchange_rate_amount == gtables.CurrencyExchangeRate_Fix_exchange_rate_amount.Both
	}
	return false
}

// validateGenJnlLineTaxPercent / validateGenJnlLineBalTaxPercent: VALIDATE("Tax %") and
// VALIDATE("Bal. Tax %") — the tax of the line's account and balancing account
func validateGenJnlLineTaxPercent(rec *gtables.GenJournalLine) error {
	return rec.Validate("tax_percent", rec.Tax_percent)
}

func validateGenJnlLineBalTaxPercent(rec *gtables.GenJournalLine) error {
	return rec.Validate("bal_tax_percent", rec.Bal_tax_percent)
}

// OnValidate_Tax_percent: the tax in the amount of the account side — by the tax calculation
// type of its tax posting setup (sales tax: the amount includes the tax, except use tax of a
// purchase), the base and the (LCY) amounts
func (genJournalLineLogic) OnValidate_Tax_percent(rec *gtables.GenJournalLine) error {
	currency, err := getGenJnlLineCurrency(rec)
	if err != nil {
		return err
	}
	switch rec.Tax_calculation_type {
	case gtables.GenJournalLine_Tax_calculation_type.NormalVAT, gtables.GenJournalLine_Tax_calculation_type.ReverseChargeVAT:
		rec.Tax_amount = rec.Amount.Sub(system.Round(rec.Amount.Div(hundredPercent(rec.Tax_percent)), currency.Amount_rounding_precision))
	case gtables.GenJournalLine_Tax_calculation_type.FullVAT:
		rec.Tax_amount = rec.Amount
	case gtables.GenJournalLine_Tax_calculation_type.SalesTax:
		if rec.Gen_posting_type == gtables.GenJournalLine_Gen_posting_type.Purchase && rec.Use_tax {
			rec.Tax_amount = types.Decimal{}
			rec.Tax_percent = types.Decimal{}
		} else {
			amountExclTax, err := salestax.New(rec).ReverseCalculateTax(rec.Tax_area_code, rec.Tax_group_code, rec.Tax_liable,
				rec.Posting_date, rec.Amount, rec.Quantity, rec.Currency_factor)
			if err != nil {
				return err
			}
			rec.Tax_amount = rec.Amount.Sub(amountExclTax)
			if !rec.Amount.Sub(rec.Tax_amount).IsZero() {
				rec.Tax_percent = system.Round(types.NewDecimalFromInt(100).Mul(rec.Tax_amount).Div(rec.Amount.Sub(rec.Tax_amount)), types.MustDecimal("0.00001"))
			} else {
				rec.Tax_percent = types.Decimal{}
			}
		}
	}
	rec.Tax_base_amount = rec.Amount.Sub(rec.Tax_amount)
	if rec.Currency_code == "" {
		rec.Tax_amount_lcy = rec.Tax_amount
	} else {
		taxAmountLCY, err := ExchangeRates(rec).ExchangeAmtFCYToLCY(rec.Posting_date, rec.Currency_code, rec.Tax_amount, rec.Currency_factor)
		if err != nil {
			return err
		}
		rec.Tax_amount_lcy = system.Round(taxAmountLCY)
	}
	rec.Tax_base_amount_lcy = rec.Amount_lcy.Sub(rec.Tax_amount_lcy)
	return nil
}

// OnValidate_Bal_tax_percent: the tax of the balancing account side (the opposite sign)
func (genJournalLineLogic) OnValidate_Bal_tax_percent(rec *gtables.GenJournalLine) error {
	currency, err := getGenJnlLineCurrency(rec)
	if err != nil {
		return err
	}
	switch rec.Bal_tax_calculation_type {
	case gtables.GenJournalLine_Bal_tax_calculation_type.NormalVAT, gtables.GenJournalLine_Bal_tax_calculation_type.ReverseChargeVAT:
		rec.Bal_tax_amount = rec.Amount.Sub(system.Round(rec.Amount.Div(hundredPercent(rec.Bal_tax_percent)), currency.Amount_rounding_precision)).Neg()
	case gtables.GenJournalLine_Bal_tax_calculation_type.FullVAT:
		rec.Bal_tax_amount = rec.Amount.Neg()
	case gtables.GenJournalLine_Bal_tax_calculation_type.SalesTax:
		if rec.Bal_gen_posting_type == gtables.GenJournalLine_Bal_gen_posting_type.Purchase && rec.Bal_use_tax {
			rec.Bal_tax_amount = types.Decimal{}
			rec.Bal_tax_percent = types.Decimal{}
		} else {
			amountExclTax, err := salestax.New(rec).ReverseCalculateTax(rec.Bal_tax_area_code, rec.Bal_tax_group_code, rec.Bal_tax_liable,
				rec.Posting_date, rec.Amount, rec.Quantity, rec.Currency_factor)
			if err != nil {
				return err
			}
			rec.Bal_tax_amount = rec.Amount.Sub(amountExclTax).Neg()
			if !rec.Amount.Add(rec.Bal_tax_amount).IsZero() {
				rec.Bal_tax_percent = system.Round(types.NewDecimalFromInt(100).Mul(rec.Bal_tax_amount.Neg()).Div(rec.Amount.Add(rec.Bal_tax_amount)), types.MustDecimal("0.00001"))
			} else {
				rec.Bal_tax_percent = types.Decimal{}
			}
		}
	}
	rec.Bal_tax_base_amount = rec.Amount.Add(rec.Bal_tax_amount).Neg()
	if rec.Currency_code == "" {
		rec.Bal_tax_amount_lcy = rec.Bal_tax_amount
	} else {
		taxAmountLCY, err := ExchangeRates(rec).ExchangeAmtFCYToLCY(rec.Posting_date, rec.Currency_code, rec.Bal_tax_amount, rec.Currency_factor)
		if err != nil {
			return err
		}
		rec.Bal_tax_amount_lcy = system.Round(taxAmountLCY)
	}
	rec.Bal_tax_base_amount_lcy = rec.Amount_lcy.Add(rec.Bal_tax_amount_lcy).Neg()
	return nil
}

// hundredPercent is 1 + percent / 100
func hundredPercent(percent types.Decimal) types.Decimal {
	return types.NewDecimalFromInt(1).Add(percent.Div(types.NewDecimalFromInt(100)))
}

// OnValidate_Tax_base_amount: the amount from the base (the tax on top)
func (genJournalLineLogic) OnValidate_Tax_base_amount(rec *gtables.GenJournalLine) error {
	currency, err := getGenJnlLineCurrency(rec)
	if err != nil {
		return err
	}
	rec.Tax_base_amount = system.Round(rec.Tax_base_amount, currency.Amount_rounding_precision)
	switch rec.Tax_calculation_type {
	case gtables.GenJournalLine_Tax_calculation_type.NormalVAT, gtables.GenJournalLine_Tax_calculation_type.ReverseChargeVAT:
		rec.Amount = system.Round(rec.Tax_base_amount.Mul(hundredPercent(rec.Tax_percent)), currency.Amount_rounding_precision)
	case gtables.GenJournalLine_Tax_calculation_type.FullVAT:
		if !rec.Tax_base_amount.IsZero() {
			return apperrors.JnlTaxBaseFullVAT(genJnlLineTable, "tax_base_amount", rec.Tax_calculation_type)
		}
	case gtables.GenJournalLine_Tax_calculation_type.SalesTax:
		if rec.Gen_posting_type == gtables.GenJournalLine_Gen_posting_type.Purchase && rec.Use_tax {
			rec.Tax_amount = types.Decimal{}
			rec.Tax_percent = types.Decimal{}
			rec.Amount = rec.Tax_base_amount.Add(rec.Tax_amount)
		} else {
			rec.Tax_amount, err = salestax.New(rec).CalculateTax(rec.Tax_area_code, rec.Tax_group_code, rec.Tax_liable,
				rec.Posting_date, rec.Tax_base_amount, rec.Quantity, rec.Currency_factor)
			if err != nil {
				return err
			}
			if !rec.Tax_base_amount.IsZero() {
				rec.Tax_percent = system.Round(types.NewDecimalFromInt(100).Mul(rec.Tax_amount).Div(rec.Tax_base_amount), types.MustDecimal("0.00001"))
			} else {
				rec.Tax_percent = types.Decimal{}
			}
			rec.Amount = rec.Tax_base_amount.Add(rec.Tax_amount)
		}
	}
	return rec.Validate("amount", rec.Amount)
}

// OnValidate_Bal_tax_base_amount: the amount from the balancing account's base
func (genJournalLineLogic) OnValidate_Bal_tax_base_amount(rec *gtables.GenJournalLine) error {
	currency, err := getGenJnlLineCurrency(rec)
	if err != nil {
		return err
	}
	rec.Bal_tax_base_amount = system.Round(rec.Bal_tax_base_amount, currency.Amount_rounding_precision)
	switch rec.Bal_tax_calculation_type {
	case gtables.GenJournalLine_Bal_tax_calculation_type.NormalVAT, gtables.GenJournalLine_Bal_tax_calculation_type.ReverseChargeVAT:
		rec.Amount = system.Round(rec.Bal_tax_base_amount.Neg().Mul(hundredPercent(rec.Bal_tax_percent)), currency.Amount_rounding_precision)
	case gtables.GenJournalLine_Bal_tax_calculation_type.FullVAT:
		if !rec.Bal_tax_base_amount.IsZero() {
			return apperrors.JnlTaxBaseFullVAT(genJnlLineTable, "bal_tax_base_amount", rec.Bal_tax_calculation_type)
		}
	case gtables.GenJournalLine_Bal_tax_calculation_type.SalesTax:
		if rec.Bal_gen_posting_type == gtables.GenJournalLine_Bal_gen_posting_type.Purchase && rec.Bal_use_tax {
			rec.Bal_tax_amount = types.Decimal{}
			rec.Bal_tax_percent = types.Decimal{}
			rec.Amount = rec.Bal_tax_base_amount.Neg().Sub(rec.Bal_tax_amount)
		} else {
			rec.Bal_tax_amount, err = salestax.New(rec).CalculateTax(rec.Bal_tax_area_code, rec.Bal_tax_group_code, rec.Bal_tax_liable,
				rec.Posting_date, rec.Bal_tax_base_amount, rec.Quantity, rec.Currency_factor)
			if err != nil {
				return err
			}
			if !rec.Bal_tax_base_amount.IsZero() {
				rec.Bal_tax_percent = system.Round(types.NewDecimalFromInt(100).Mul(rec.Bal_tax_amount).Div(rec.Bal_tax_base_amount), types.MustDecimal("0.00001"))
			} else {
				rec.Bal_tax_percent = types.Decimal{}
			}
			rec.Amount = rec.Bal_tax_base_amount.Neg().Sub(rec.Bal_tax_amount)
		}
	}
	return rec.Validate("amount", rec.Amount)
}

// The tax fields of the account side: the tax again
func (genJournalLineLogic) OnValidate_Tax_area_code(rec *gtables.GenJournalLine) error {
	return validateGenJnlLineTaxPercent(rec)
}
func (genJournalLineLogic) OnValidate_Tax_liable(rec *gtables.GenJournalLine) error {
	return validateGenJnlLineTaxPercent(rec)
}
func (genJournalLineLogic) OnValidate_Tax_group_code(rec *gtables.GenJournalLine) error {
	return validateGenJnlLineTaxPercent(rec)
}

// OnValidate_Use_tax: only purchases pay use tax
func (genJournalLineLogic) OnValidate_Use_tax(rec *gtables.GenJournalLine) error {
	err := rec.TestField("gen_posting_type", gtables.GenJournalLine_Gen_posting_type.Purchase)
	if err != nil {
		return err
	}
	return validateGenJnlLineTaxPercent(rec)
}

// The tax fields of the balancing account side: its tax again
func (genJournalLineLogic) OnValidate_Bal_tax_area_code(rec *gtables.GenJournalLine) error {
	return validateGenJnlLineBalTaxPercent(rec)
}
func (genJournalLineLogic) OnValidate_Bal_tax_liable(rec *gtables.GenJournalLine) error {
	return validateGenJnlLineBalTaxPercent(rec)
}
func (genJournalLineLogic) OnValidate_Bal_tax_group_code(rec *gtables.GenJournalLine) error {
	return validateGenJnlLineBalTaxPercent(rec)
}
func (genJournalLineLogic) OnValidate_Bal_use_tax(rec *gtables.GenJournalLine) error {
	err := rec.TestField("bal_gen_posting_type", gtables.GenJournalLine_Bal_gen_posting_type.Purchase)
	if err != nil {
		return err
	}
	return validateGenJnlLineBalTaxPercent(rec)
}

// OnValidate_Tax_bus_posting_group: the tax calculation follows
func (genJournalLineLogic) OnValidate_Tax_bus_posting_group(rec *gtables.GenJournalLine) error {
	return rec.Validate("tax_prod_posting_group", rec.Tax_prod_posting_group)
}

// OnValidate_Tax_prod_posting_group: a line with a general posting type takes the tax
// calculation type (and VAT %) of its tax posting setup
func (genJournalLineLogic) OnValidate_Tax_prod_posting_group(rec *gtables.GenJournalLine) error {
	err := checkTaxInAlloc(rec)
	if err != nil {
		return err
	}
	rec.Tax_percent = types.Decimal{}
	rec.Tax_calculation_type = gtables.GenJournalLine_Tax_calculation_type.NormalVAT
	if rec.Gen_posting_type != gtables.GenJournalLine_Gen_posting_type.Blank {
		taxPostingSetup := gtables.NewTaxPostingSetup(rec)
		if !taxPostingSetup.Get(map[string]interface{}{"tax_bus_posting_group": rec.Tax_bus_posting_group, "tax_prod_posting_group": rec.Tax_prod_posting_group}) {
			taxPostingSetup.Init()
		}
		rec.Tax_calculation_type = gtables.GenJournalLineTax_calculation_type(taxPostingSetup.Tax_calculation_type)
		switch rec.Tax_calculation_type {
		case gtables.GenJournalLine_Tax_calculation_type.NormalVAT:
			rec.Tax_percent = taxPostingSetup.Vat_percent
		case gtables.GenJournalLine_Tax_calculation_type.FullVAT:
			switch rec.Gen_posting_type {
			case gtables.GenJournalLine_Gen_posting_type.Sale:
				err := taxPostingSetup.TestField("sales_vat_account")
				if err != nil {
					return err
				}
				err = rec.TestField("account_no", taxPostingSetup.Sales_vat_account)
				if err != nil {
					return err
				}
			case gtables.GenJournalLine_Gen_posting_type.Purchase:
				err := taxPostingSetup.TestField("purchase_vat_account")
				if err != nil {
					return err
				}
				err = rec.TestField("account_no", taxPostingSetup.Purchase_vat_account)
				if err != nil {
					return err
				}
			}
		}
	}
	return validateGenJnlLineTaxPercent(rec)
}

// OnValidate_Bal_tax_bus_posting_group: the balancing account's tax calculation follows
func (genJournalLineLogic) OnValidate_Bal_tax_bus_posting_group(rec *gtables.GenJournalLine) error {
	return rec.Validate("bal_tax_prod_posting_group", rec.Bal_tax_prod_posting_group)
}

// OnValidate_Bal_tax_prod_posting_group: the balancing account's tax calculation type
func (genJournalLineLogic) OnValidate_Bal_tax_prod_posting_group(rec *gtables.GenJournalLine) error {
	rec.Bal_tax_percent = types.Decimal{}
	rec.Bal_tax_calculation_type = gtables.GenJournalLine_Bal_tax_calculation_type.NormalVAT
	if rec.Bal_gen_posting_type != gtables.GenJournalLine_Bal_gen_posting_type.Blank {
		taxPostingSetup := gtables.NewTaxPostingSetup(rec)
		if !taxPostingSetup.Get(map[string]interface{}{"tax_bus_posting_group": rec.Bal_tax_bus_posting_group, "tax_prod_posting_group": rec.Bal_tax_prod_posting_group}) {
			taxPostingSetup.Init()
		}
		rec.Bal_tax_calculation_type = gtables.GenJournalLineBal_tax_calculation_type(taxPostingSetup.Tax_calculation_type)
		switch rec.Bal_tax_calculation_type {
		case gtables.GenJournalLine_Bal_tax_calculation_type.NormalVAT:
			rec.Bal_tax_percent = taxPostingSetup.Vat_percent
		case gtables.GenJournalLine_Bal_tax_calculation_type.FullVAT:
			switch rec.Bal_gen_posting_type {
			case gtables.GenJournalLine_Bal_gen_posting_type.Sale:
				err := taxPostingSetup.TestField("sales_vat_account")
				if err != nil {
					return err
				}
				err = rec.TestField("bal_account_no", taxPostingSetup.Sales_vat_account)
				if err != nil {
					return err
				}
			case gtables.GenJournalLine_Bal_gen_posting_type.Purchase:
				err := taxPostingSetup.TestField("purchase_vat_account")
				if err != nil {
					return err
				}
				err = rec.TestField("bal_account_no", taxPostingSetup.Purchase_vat_account)
				if err != nil {
					return err
				}
			}
		}
	}
	return validateGenJnlLineBalTaxPercent(rec)
}

// CalcDateFormula calculates a date from a date formula field: an empty field (no formula set
// up) leaves the date as it is, as a blank DateFormula does
func CalcDateFormula(formula types.Code, date types.Date) (types.Date, error) {
	if formula == "" {
		return date, nil
	}
	return system.CalcDate(string(formula), date)
}

// setGenJnlLineFADeprBook: a fixed asset line without a depreciation book takes the FA Setup's
// default book when the asset has it; the asset's posting group in the book
func setGenJnlLineFADeprBook(rec *gtables.GenJournalLine, faNo types.Code) error {
	faDeprBook := gtables.NewFADepreciationBook(rec)
	if rec.Depreciation_book_code == "" {
		faSetup := gtables.GetFASetup(rec)
		rec.Depreciation_book_code = faSetup.Default_depr_book
		if !faDeprBook.Get(map[string]interface{}{"fa_no": faNo, "depreciation_book_code": rec.Depreciation_book_code}) {
			rec.Depreciation_book_code = ""
		}
	}
	if rec.Depreciation_book_code != "" {
		err := faDeprBook.GetOrError(map[string]interface{}{"fa_no": faNo, "depreciation_book_code": rec.Depreciation_book_code})
		if err != nil {
			return err
		}
		rec.Posting_group = faDeprBook.Fa_posting_group
	}
	return getFAAddCurrExchRate(rec)
}

// getFAAddCurrExchRate (GetFAAddCurrExchRate): a fixed asset line whose depreciation book posts
// the line's FA posting type at the asset's own additional-currency rate takes the FA
// depreciation book's FA Add.-Currency Factor (codeunit 12 exchanges the line with it)
func getFAAddCurrExchRate(rec *gtables.GenJournalLine) error {
	rec.Fa_add_currency_factor = types.Decimal{}
	if rec.Fa_posting_type != gtables.GenJournalLine_Fa_posting_type.Blank && rec.Depreciation_book_code != "" {
		var faNo types.Code
		if rec.Account_type == gtables.GenJournalLine_Account_type.FixedAsset {
			faNo = rec.Account_no
		}
		if rec.Bal_account_type == gtables.GenJournalLine_Bal_account_type.FixedAsset {
			faNo = rec.Bal_account_no
		}
		if faNo != "" {
			deprBook := gtables.NewDepreciationBook(rec)
			err := deprBook.GetOrError(rec.Depreciation_book_code)
			if err != nil {
				return err
			}
			var useFAAddCurrExchRate bool
			switch rec.Fa_posting_type {
			case gtables.GenJournalLine_Fa_posting_type.AcquisitionCost:
				useFAAddCurrExchRate = deprBook.Add_curr_exch_rate_acq_cost
			case gtables.GenJournalLine_Fa_posting_type.Depreciation:
				useFAAddCurrExchRate = deprBook.Add_curr_exch_rate_depr
			case gtables.GenJournalLine_Fa_posting_type.WriteDown:
				useFAAddCurrExchRate = deprBook.Add_curr_exch_rate_write_down
			case gtables.GenJournalLine_Fa_posting_type.Appreciation:
				useFAAddCurrExchRate = deprBook.Add_curr_exch_rate_apprec
			case gtables.GenJournalLine_Fa_posting_type.Custom1:
				useFAAddCurrExchRate = deprBook.Add_curr_exch_rate_custom_1
			case gtables.GenJournalLine_Fa_posting_type.Custom2:
				useFAAddCurrExchRate = deprBook.Add_curr_exch_rate_custom_2
			case gtables.GenJournalLine_Fa_posting_type.Disposal:
				useFAAddCurrExchRate = deprBook.Add_curr_exch_rate_disp
			case gtables.GenJournalLine_Fa_posting_type.Maintenance:
				useFAAddCurrExchRate = deprBook.Add_curr_exch_rate_maint
			}
			if useFAAddCurrExchRate {
				faDeprBook := gtables.NewFADepreciationBook(rec)
				err = faDeprBook.GetOrError(map[string]interface{}{"fa_no": faNo, "depreciation_book_code": rec.Depreciation_book_code})
				if err != nil {
					return err
				}
				err = faDeprBook.TestField("fa_add_currency_factor")
				if err != nil {
					return err
				}
				rec.Fa_add_currency_factor = faDeprBook.Fa_add_currency_factor
			}
		}
	}
	return nil
}

// clearGenJnlLineFAFields: a line without a fixed asset on either side has no depreciation book
// and no FA posting type
func clearGenJnlLineFAFields(rec *gtables.GenJournalLine) error {
	if rec.Account_type != gtables.GenJournalLine_Account_type.FixedAsset &&
		rec.Bal_account_type != gtables.GenJournalLine_Bal_account_type.FixedAsset {
		rec.Depreciation_book_code = ""
		return rec.Validate("fa_posting_type", gtables.GenJournalLine_Fa_posting_type.Blank)
	}
	return nil
}

// OnValidate_Fa_posting_type: a line without a fixed asset and posting type has none of the FA
// fields; an insurance only on an acquisition, a maintenance code only on a maintenance
func (genJournalLineLogic) OnValidate_Fa_posting_type(rec *gtables.GenJournalLine) error {
	if rec.Account_type != gtables.GenJournalLine_Account_type.FixedAsset &&
		rec.Bal_account_type != gtables.GenJournalLine_Bal_account_type.FixedAsset &&
		rec.Fa_posting_type == gtables.GenJournalLine_Fa_posting_type.Blank {
		rec.Fa_posting_date = types.Date{}
		rec.Salvage_value = types.Decimal{}
		rec.No_of_depreciation_days = 0
		rec.Depr_until_fa_posting_date = false
		rec.Depr_acquisition_cost = false
		rec.Maintenance_code = ""
		rec.Insurance_no = ""
		rec.Duplicate_in_depreciation_book = ""
		rec.Use_duplication_list = false
		rec.Fa_reclassification_entry = false
		rec.Fa_error_entry_no = 0
	}
	if rec.Fa_posting_type != gtables.GenJournalLine_Fa_posting_type.AcquisitionCost {
		err := rec.TestField("insurance_no", types.Code(""))
		if err != nil {
			return err
		}
	}
	if rec.Fa_posting_type != gtables.GenJournalLine_Fa_posting_type.Maintenance {
		err := rec.TestField("maintenance_code", types.Code(""))
		if err != nil {
			return err
		}
	}
	return getFAAddCurrExchRate(rec)
}

// OnValidate_Depreciation_book_code: the fixed asset's posting group in the book
func (genJournalLineLogic) OnValidate_Depreciation_book_code(rec *gtables.GenJournalLine) error {
	if rec.Depreciation_book_code == "" {
		return nil
	}
	faDeprBook := gtables.NewFADepreciationBook(rec)
	if rec.Account_no != "" && rec.Account_type == gtables.GenJournalLine_Account_type.FixedAsset {
		err := faDeprBook.GetOrError(map[string]interface{}{"fa_no": rec.Account_no, "depreciation_book_code": rec.Depreciation_book_code})
		if err != nil {
			return err
		}
		rec.Posting_group = faDeprBook.Fa_posting_group
	}
	if rec.Bal_account_no != "" && rec.Bal_account_type == gtables.GenJournalLine_Bal_account_type.FixedAsset {
		err := faDeprBook.GetOrError(map[string]interface{}{"fa_no": rec.Bal_account_no, "depreciation_book_code": rec.Depreciation_book_code})
		if err != nil {
			return err
		}
		rec.Posting_group = faDeprBook.Fa_posting_group
	}
	return getFAAddCurrExchRate(rec)
}

// OnValidate_Maintenance_code: only on a maintenance
func (genJournalLineLogic) OnValidate_Maintenance_code(rec *gtables.GenJournalLine) error {
	if rec.Maintenance_code != "" {
		return rec.TestField("fa_posting_type", gtables.GenJournalLine_Fa_posting_type.Maintenance)
	}
	return nil
}

// OnValidate_Insurance_no: only on an acquisition
func (genJournalLineLogic) OnValidate_Insurance_no(rec *gtables.GenJournalLine) error {
	if rec.Insurance_no != "" {
		return rec.TestField("fa_posting_type", gtables.GenJournalLine_Fa_posting_type.AcquisitionCost)
	}
	return nil
}

// OnValidate_Duplicate_in_depreciation_book: not with the duplication list
func (genJournalLineLogic) OnValidate_Duplicate_in_depreciation_book(rec *gtables.GenJournalLine) error {
	rec.Use_duplication_list = false
	return nil
}

// OnValidate_Use_duplication_list: not with a book to duplicate in
func (genJournalLineLogic) OnValidate_Use_duplication_list(rec *gtables.GenJournalLine) error {
	rec.Duplicate_in_depreciation_book = ""
	return nil
}
