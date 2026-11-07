package tables

import (
	gtables "github.com/hansjlachmann/goeasyerp/backend/generated/tables"
)

// taxJurisdictionLogic is the business logic of table 320 Tax Jurisdiction
type taxJurisdictionLogic struct{}

func init() { gtables.RegisterTaxJurisdictionLogic(taxJurisdictionLogic{}) }

// OnDelete deletes the jurisdiction's tax details
func (taxJurisdictionLogic) OnDelete(rec *gtables.TaxJurisdiction) error {
	taxDetail := gtables.NewTaxDetail(rec)
	taxDetail.SetRange("tax_jurisdiction_code", rec.Code)
	taxDetail.DeleteAll()
	return nil
}

// OnValidate_Unrealized_tax_type: unrealized tax needs it in the General Ledger Setup
func (taxJurisdictionLogic) OnValidate_Unrealized_tax_type(rec *gtables.TaxJurisdiction) error {
	if rec.Unrealized_tax_type > 0 {
		glSetup := gtables.GetGeneralLedgerSetup(rec)
		return glSetup.TestField("unrealized_tax", true)
	}
	return nil
}

// OnValidate_Calculate_tax_on_tax: the jurisdiction's tax details follow
func (taxJurisdictionLogic) OnValidate_Calculate_tax_on_tax(rec *gtables.TaxJurisdiction) error {
	taxDetail := gtables.NewTaxDetail(rec)
	taxDetail.SetRange("tax_jurisdiction_code", rec.Code)
	taxDetail.ModifyAll("calculate_tax_on_tax", rec.Calculate_tax_on_tax)
	return nil
}

// OnValidate_Adjust_for_payment_discount: needs Adjust for Payment Disc. in the General Ledger
// Setup
func (taxJurisdictionLogic) OnValidate_Adjust_for_payment_discount(rec *gtables.TaxJurisdiction) error {
	if rec.Adjust_for_payment_discount {
		glSetup := gtables.GetGeneralLedgerSetup(rec)
		return glSetup.TestField("adjust_for_payment_disc", true)
	}
	return nil
}
