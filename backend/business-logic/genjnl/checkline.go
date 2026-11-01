package genjnl

import (
	apperrors "github.com/hansjlachmann/openerp/backend/foundation/errors"
	"github.com/hansjlachmann/openerp/backend/foundation/system"
	"github.com/hansjlachmann/openerp/backend/foundation/types"
	gtables "github.com/hansjlachmann/openerp/backend/generated/tables"

	"github.com/hansjlachmann/openerp/backend/business-logic/fajnl"
	"github.com/hansjlachmann/openerp/backend/business-logic/tables"
)

// GenJnlCheckLine is codeunit 11 Gen. Jnl.-Check Line: the checks a journal line must pass
// before it is posted. One instance remembers the user's allowed posting dates. A line with a
// fixed asset is checked by codeunit 5631 (fajnl.FAJnlCheckLine) too.
//
// Not migrated yet: computer checks and electronic payments (check printing), recurring lines.
type GenJnlCheckLine struct {
	allowPostingFrom types.Date
	allowPostingTo   types.Date
	faJnlCheckLine   fajnl.FAJnlCheckLine
}

const jnlLine = gtables.GenJournalLineTableName

// RunCheck checks a journal line (OnRun)
func (c *GenJnlCheckLine) RunCheck(rec *gtables.GenJournalLine) error {
	if tables.GenJnlLineEmptyLine(rec) {
		return nil
	}
	err := rec.TestField("posting_date")
	if err != nil {
		return err
	}
	if rec.Posting_date.IsClosing() {
		if rec.Account_type != gtables.GenJournalLine_Account_type.GLAccount ||
			rec.Bal_account_type != gtables.GenJournalLine_Bal_account_type.GLAccount {
			return apperrors.JnlClosingDateGLOnly(jnlLine, "posting_date")
		}
		accountingPeriod := gtables.NewAccountingPeriod(rec)
		err = accountingPeriod.GetOrError(rec.Posting_date.Normal().AddDays(1))
		if err != nil {
			return err
		}
		err = accountingPeriod.TestField("new_fiscal_year", true)
		if err != nil {
			return err
		}
		err = accountingPeriod.TestField("date_locked", true)
		if err != nil {
			return err
		}
	}
	if c.DateNotAllowed(rec, rec.Posting_date) {
		return apperrors.JnlDateNotAllowed(jnlLine, "posting_date")
	}
	if !rec.Document_date.IsZero() {
		if rec.Document_date.IsClosing() &&
			(rec.Account_type != gtables.GenJournalLine_Account_type.GLAccount ||
				rec.Bal_account_type != gtables.GenJournalLine_Bal_account_type.GLAccount) {
			return apperrors.JnlClosingDateGLOnly(jnlLine, "document_date")
		}
	}
	err = rec.TestField("document_no")
	if err != nil {
		return err
	}
	if isCustVendFA(int(rec.Account_type)) && isCustVendFA(int(rec.Bal_account_type)) {
		return apperrors.JnlAccountTypes(jnlLine, "account_type", "bal_account_type")
	}
	if rec.Bal_account_no == "" {
		err = rec.TestField("account_no")
		if err != nil {
			return err
		}
	}
	if rec.Account_no != "" &&
		!rec.System_created_entry &&
		rec.Gen_posting_type == gtables.GenJournalLine_Gen_posting_type.Blank &&
		rec.Account_type != gtables.GenJournalLine_Account_type.FixedAsset {
		err = rec.TestField("amount")
		if err != nil {
			return err
		}
	}
	if rec.Amount.Mul(rec.Amount_lcy).IsNegative() {
		return apperrors.JnlSameSign(jnlLine, "amount_lcy", "amount")
	}
	if rec.Account_type == gtables.GenJournalLine_Account_type.GLAccount &&
		rec.Bal_account_type == gtables.GenJournalLine_Bal_account_type.GLAccount {
		err = rec.TestField("applies_to_doc_no", types.Code(""))
		if err != nil {
			return err
		}
	}

	if rec.Account_no != "" {
		switch rec.Account_type {
		case gtables.GenJournalLine_Account_type.GLAccount:
			if rec.Gen_bus_posting_group != "" || rec.Gen_prod_posting_group != "" ||
				rec.Tax_bus_posting_group != "" || rec.Tax_prod_posting_group != "" {
				err = rec.TestField("gen_posting_type")
				if err != nil {
					return err
				}
			}
			if rec.Gen_posting_type != gtables.GenJournalLine_Gen_posting_type.Blank &&
				rec.Vat_posting == gtables.GenJournalLine_Vat_posting.AutomaticVATEntry {
				if !rec.Tax_amount.Add(rec.Tax_base_amount).Equal(rec.Amount) {
					return apperrors.JnlTaxSum(jnlLine, "tax_amount", "tax_base_amount")
				}
				if rec.Currency_code != "" {
					if !rec.Tax_amount_lcy.Add(rec.Tax_base_amount_lcy).Equal(rec.Amount_lcy) {
						return apperrors.JnlTaxSum(jnlLine, "tax_amount_lcy", "tax_base_amount_lcy")
					}
				}
			}
		case gtables.GenJournalLine_Account_type.Customer, gtables.GenJournalLine_Account_type.Vendor:
			err = testNoGenPosting(rec, "gen_posting_type", "gen_bus_posting_group", "gen_prod_posting_group", "tax_bus_posting_group", "tax_prod_posting_group")
			if err != nil {
				return err
			}
			if rec.Document_type != gtables.GenJournalLine_Document_type.Blank {
				err = checkAmountSign(rec)
				if err != nil {
					return err
				}
			}
			if rec.Amount.Mul(rec.Sales_purch_lcy).IsNegative() {
				return apperrors.JnlSameSign(jnlLine, "sales_purch_lcy", "amount")
			}
		case gtables.GenJournalLine_Account_type.BankAccount:
			err = testNoGenPosting(rec, "gen_posting_type", "gen_bus_posting_group", "gen_prod_posting_group", "tax_bus_posting_group", "tax_prod_posting_group")
			if err != nil {
				return err
			}
			if rec.Amount.IsNegative() && rec.Bank_payment_type == gtables.GenJournalLine_Bank_payment_type.ComputerCheck {
				err = rec.TestField("check_printed", true)
				if err != nil {
					return err
				}
			}
		}
	}

	if rec.Bal_account_no != "" {
		switch rec.Bal_account_type {
		case gtables.GenJournalLine_Bal_account_type.GLAccount:
			if rec.Bal_gen_bus_posting_group != "" || rec.Bal_gen_prod_posting_group != "" {
				err = rec.TestField("bal_gen_posting_type")
				if err != nil {
					return err
				}
			}
			if rec.Bal_gen_posting_type != gtables.GenJournalLine_Bal_gen_posting_type.Blank &&
				rec.Vat_posting == gtables.GenJournalLine_Vat_posting.AutomaticVATEntry {
				if !rec.Bal_tax_amount.Add(rec.Bal_tax_base_amount).Equal(rec.Amount.Neg()) {
					return apperrors.JnlTaxSum(jnlLine, "bal_tax_amount", "bal_tax_base_amount")
				}
				if rec.Currency_code != "" {
					if !rec.Bal_tax_amount_lcy.Add(rec.Bal_tax_base_amount_lcy).Equal(rec.Amount_lcy.Neg()) {
						return apperrors.JnlTaxSum(jnlLine, "bal_tax_amount_lcy", "bal_tax_base_amount_lcy")
					}
				}
			}
		case gtables.GenJournalLine_Bal_account_type.Customer, gtables.GenJournalLine_Bal_account_type.Vendor:
			err = testNoGenPosting(rec, "bal_gen_posting_type", "bal_gen_bus_posting_group", "bal_gen_prod_posting_group", "bal_tax_bus_posting_group", "bal_tax_prod_posting_group")
			if err != nil {
				return err
			}
			if rec.Document_type != gtables.GenJournalLine_Document_type.Blank &&
				(rec.Account_type != gtables.GenJournalLine_Account_type.BankAccount ||
					rec.Document_type != gtables.GenJournalLine_Document_type.Payment ||
					!rec.System_created_entry) {
				paymentOrCreditMemo := rec.Document_type == gtables.GenJournalLine_Document_type.Payment ||
					rec.Document_type == gtables.GenJournalLine_Document_type.CreditMemo
				if (rec.Bal_account_type == gtables.GenJournalLine_Bal_account_type.Customer) == paymentOrCreditMemo {
					if rec.Amount.IsNegative() {
						return apperrors.JnlMustBePositive(jnlLine, "amount")
					}
				} else {
					if rec.Amount.IsPositive() {
						return apperrors.JnlMustBeNegative(jnlLine, "amount")
					}
				}
			}
			if rec.Amount.Mul(rec.Sales_purch_lcy).IsPositive() {
				return apperrors.JnlDifferentSign(jnlLine, "sales_purch_lcy", "amount")
			}
		case gtables.GenJournalLine_Bal_account_type.BankAccount:
			err = testNoGenPosting(rec, "bal_gen_posting_type", "bal_gen_bus_posting_group", "bal_gen_prod_posting_group", "bal_tax_bus_posting_group", "bal_tax_prod_posting_group")
			if err != nil {
				return err
			}
			if rec.Amount.IsPositive() && rec.Bank_payment_type == gtables.GenJournalLine_Bank_payment_type.ComputerCheck {
				err = rec.TestField("check_printed", true)
				if err != nil {
					return err
				}
			}
		}
	}

	if rec.Bank_payment_type == gtables.GenJournalLine_Bank_payment_type.ElectronicPayment {
		return apperrors.JnlCheckPrintingNotYet(jnlLine, "bank_payment_type", rec.Bank_payment_type)
	}
	if rec.Account_type == gtables.GenJournalLine_Account_type.GLAccount &&
		rec.Bal_account_type == gtables.GenJournalLine_Bal_account_type.GLAccount ||
		rec.Document_type != gtables.GenJournalLine_Document_type.Invoice {
		err = rec.TestField("pmt_discount_date", types.Date{})
		if err != nil {
			return err
		}
		err = rec.TestField("payment_discount_percent", types.Decimal{})
		if err != nil {
			return err
		}
	}
	if rec.Account_type == gtables.GenJournalLine_Account_type.GLAccount &&
		rec.Bal_account_type == gtables.GenJournalLine_Bal_account_type.GLAccount ||
		rec.Applies_to_doc_no != "" {
		err = rec.TestField("applies_to_id", types.Code(""))
		if err != nil {
			return err
		}
	}
	if rec.Account_type != gtables.GenJournalLine_Account_type.BankAccount &&
		rec.Bal_account_type != gtables.GenJournalLine_Bal_account_type.BankAccount {
		err = rec.TestField("bank_payment_type", gtables.GenJournalLine_Bank_payment_type.Blank)
		if err != nil {
			return err
		}
	}
	if rec.Account_type == gtables.GenJournalLine_Account_type.FixedAsset ||
		rec.Bal_account_type == gtables.GenJournalLine_Bal_account_type.FixedAsset {
		return c.faJnlCheckLine.CheckGenJnlLine(rec)
	}
	err = rec.TestField("depreciation_book_code", types.Code(""))
	if err != nil {
		return err
	}
	return rec.TestField("fa_posting_type", gtables.GenJournalLine_Fa_posting_type.Blank)
}

// isCustVendFA: Customer, Vendor or Fixed Asset (the account type numbers of both sides)
func isCustVendFA(accountType int) bool {
	return accountType == 1 || accountType == 2 || accountType == 4
}

// testNoGenPosting: customers, vendors and bank accounts post without general and tax posting
// groups
func testNoGenPosting(rec *gtables.GenJournalLine, postingType, genBus, genProd, taxBus, taxProd string) error {
	var err error
	if postingType == "gen_posting_type" {
		err = rec.TestField(postingType, gtables.GenJournalLine_Gen_posting_type.Blank)
	} else {
		err = rec.TestField(postingType, gtables.GenJournalLine_Bal_gen_posting_type.Blank)
	}
	if err != nil {
		return err
	}
	for _, field := range []string{genBus, genProd, taxBus, taxProd} {
		err = rec.TestField(field, types.Code(""))
		if err != nil {
			return err
		}
	}
	return nil
}

// checkAmountSign: the sign of a customer's or vendor's amount fits the document type (a
// customer's invoice positive, its payment and credit memo negative — a payment applied to a
// credit memo positive; a vendor's the other way round)
func checkAmountSign(rec *gtables.GenJournalLine) error {
	paymentToCreditMemo := rec.Applies_to_doc_type == gtables.GenJournalLine_Applies_to_doc_type.CreditMemo && rec.Applies_to_doc_no != ""
	if rec.Account_type == gtables.GenJournalLine_Account_type.Customer {
		switch rec.Document_type {
		case gtables.GenJournalLine_Document_type.CreditMemo:
			return errorIfPositiveAmt(rec)
		case gtables.GenJournalLine_Document_type.Payment:
			if paymentToCreditMemo {
				return errorIfNegativeAmt(rec)
			} else {
				return errorIfPositiveAmt(rec)
			}
		default:
			return errorIfNegativeAmt(rec)
		}
	} else {
		switch rec.Document_type {
		case gtables.GenJournalLine_Document_type.CreditMemo:
			return errorIfNegativeAmt(rec)
		case gtables.GenJournalLine_Document_type.Payment:
			if paymentToCreditMemo {
				return errorIfPositiveAmt(rec)
			} else {
				return errorIfNegativeAmt(rec)
			}
		default:
			return errorIfPositiveAmt(rec)
		}
	}
}

// DateNotAllowed: the posting date is outside the period the user may post in — the user's
// User Setup, else the General Ledger Setup (no end date: no limit)
func (c *GenJnlCheckLine) DateNotAllowed(rec *gtables.GenJournalLine, postingDate types.Date) bool {
	if c.allowPostingFrom.IsZero() && c.allowPostingTo.IsZero() {
		if system.UserId() != "" {
			userSetup := gtables.NewUserSetup(rec)
			if userSetup.Get(types.Code(system.UserId())) {
				c.allowPostingFrom = userSetup.Allow_posting_from
				c.allowPostingTo = userSetup.Allow_posting_to
			}
		}
		if c.allowPostingFrom.IsZero() && c.allowPostingTo.IsZero() {
			glSetup := gtables.GetGeneralLedgerSetup(rec)
			c.allowPostingFrom = glSetup.Allow_posting_from
			c.allowPostingTo = glSetup.Allow_posting_to
		}
		if c.allowPostingTo.IsZero() {
			c.allowPostingTo, _ = system.DMY2Date(31, 12, 9999)
		}
	}
	return postingDate.Normal().Before(c.allowPostingFrom) || c.allowPostingTo.Before(postingDate.Normal())
}

func errorIfPositiveAmt(rec *gtables.GenJournalLine) error {
	if rec.Amount.IsPositive() {
		return apperrors.JnlMustBeNegative(jnlLine, "amount")
	}
	return nil
}

func errorIfNegativeAmt(rec *gtables.GenJournalLine) error {
	if rec.Amount.IsNegative() {
		return apperrors.JnlMustBePositive(jnlLine, "amount")
	}
	return nil
}
