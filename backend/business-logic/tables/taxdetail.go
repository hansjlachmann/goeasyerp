package tables

import (
	gtables "github.com/hansjlachmann/goeasyerp/backend/generated/tables"
)

// taxDetailLogic is the business logic of table 322 Tax Detail: a detail takes Calculate Tax on
// Tax from its jurisdiction
type taxDetailLogic struct{}

func init() { gtables.RegisterTaxDetailLogic(taxDetailLogic{}) }

// OnInsert: Calculate Tax on Tax of the jurisdiction
func (taxDetailLogic) OnInsert(rec *gtables.TaxDetail) error {
	return setTaxDetailTaxOnTax(rec)
}

// OnRename: Calculate Tax on Tax of the (new) jurisdiction
func (taxDetailLogic) OnRename(rec *gtables.TaxDetail) error {
	return setTaxDetailTaxOnTax(rec)
}

func setTaxDetailTaxOnTax(rec *gtables.TaxDetail) error {
	taxJurisdiction := gtables.NewTaxJurisdiction(rec)
	err := taxJurisdiction.GetOrError(rec.Tax_jurisdiction_code)
	if err != nil {
		return err
	}
	rec.Calculate_tax_on_tax = taxJurisdiction.Calculate_tax_on_tax
	return nil
}
