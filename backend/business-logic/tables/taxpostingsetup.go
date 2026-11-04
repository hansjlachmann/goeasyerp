package tables

import (
	apperrors "github.com/hansjlachmann/goeasyerp/backend/foundation/errors"
	gtables "github.com/hansjlachmann/goeasyerp/backend/generated/tables"
)

// taxPostingSetupLogic is the business logic of table 325 Tax Posting Setup: the VAT fields are
// not for Sales Tax (its accounts are the jurisdictions')
type taxPostingSetupLogic struct{}

func init() { gtables.RegisterTaxPostingSetupLogic(taxPostingSetupLogic{}) }

// OnValidate_Vat_percent: not for Sales Tax
func (taxPostingSetupLogic) OnValidate_Vat_percent(rec *gtables.TaxPostingSetup) error {
	return testNotSalesTax(rec, "vat_percent")
}

// OnValidate_Unrealized_tax_type: not for Sales Tax; unrealized tax needs it in the General
// Ledger Setup
func (taxPostingSetupLogic) OnValidate_Unrealized_tax_type(rec *gtables.TaxPostingSetup) error {
	err := testNotSalesTax(rec, "unrealized_tax_type")
	if err != nil {
		return err
	}
	if rec.Unrealized_tax_type > 0 {
		glSetup := gtables.GetGeneralLedgerSetup(rec)
		return glSetup.TestField("unrealized_tax", true)
	}
	return nil
}

// OnValidate_Adjust_for_payment_discount: not for Sales Tax; needs Adjust for Payment Disc. in
// the General Ledger Setup
func (taxPostingSetupLogic) OnValidate_Adjust_for_payment_discount(rec *gtables.TaxPostingSetup) error {
	err := testNotSalesTax(rec, "adjust_for_payment_discount")
	if err != nil {
		return err
	}
	if rec.Adjust_for_payment_discount {
		glSetup := gtables.GetGeneralLedgerSetup(rec)
		return glSetup.TestField("adjust_for_payment_disc", true)
	}
	return nil
}

func (taxPostingSetupLogic) OnValidate_Sales_vat_account(rec *gtables.TaxPostingSetup) error {
	return checkSetupGLAcc(rec, rec.Sales_vat_account)
}
func (taxPostingSetupLogic) OnValidate_Sales_vat_unreal_account(rec *gtables.TaxPostingSetup) error {
	return checkSetupGLAcc(rec, rec.Sales_vat_unreal_account)
}
func (taxPostingSetupLogic) OnValidate_Purchase_vat_account(rec *gtables.TaxPostingSetup) error {
	return checkSetupGLAcc(rec, rec.Purchase_vat_account)
}
func (taxPostingSetupLogic) OnValidate_Purch_vat_unreal_account(rec *gtables.TaxPostingSetup) error {
	return checkSetupGLAcc(rec, rec.Purch_vat_unreal_account)
}
func (taxPostingSetupLogic) OnValidate_Reverse_chrg_vat_acc(rec *gtables.TaxPostingSetup) error {
	return checkSetupGLAcc(rec, rec.Reverse_chrg_vat_acc)
}
func (taxPostingSetupLogic) OnValidate_Reverse_chrg_vat_unreal_acc(rec *gtables.TaxPostingSetup) error {
	return checkSetupGLAcc(rec, rec.Reverse_chrg_vat_unreal_acc)
}

// testNotSalesTax: a VAT field cannot be filled in for Sales Tax
func testNotSalesTax(rec *gtables.TaxPostingSetup, field string) error {
	if rec.Tax_calculation_type == gtables.TaxPostingSetup_Tax_calculation_type.SalesTax {
		return apperrors.TaxNotForSalesTax(gtables.TaxPostingSetupTableName, field, rec.Tax_calculation_type)
	}
	return nil
}
