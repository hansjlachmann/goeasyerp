package purchposting

import (
	"github.com/hansjlachmann/goeasyerp/backend/business-logic/projectjnl"
	"github.com/hansjlachmann/goeasyerp/backend/business-logic/salestax"
	"github.com/hansjlachmann/goeasyerp/backend/business-logic/tables"
	"github.com/hansjlachmann/goeasyerp/backend/foundation/system"
	"github.com/hansjlachmann/goeasyerp/backend/foundation/types"
	gtables "github.com/hansjlachmann/goeasyerp/backend/generated/tables"
)

// postProjectJnlLine posts the invoiced share of a line for a project as project usage
// (codeunit 90's "Post job entries"): the project entry only, through codeunit 202, at the
// line's cost with its tax (use tax calculated) and its price in LCY
func (p *PurchPost) postProjectJnlLine(purchLine *gtables.PurchaseLine, qtyToInvoice, qtyToInvoiceBase types.Decimal, lineFCY, lineLCY *gtables.PurchaseLine) error {
	if (purchLine.Project_no == "") || qtyToInvoice.IsZero() {
		return nil
	}
	purchHeader := p.purchHeader
	projectJnlLine := gtables.NewProjectJournalLine(p.src)
	projectJnlLine.Init()
	projectJnlLine.Posting_date = purchHeader.Posting_date
	projectJnlLine.Document_date = purchHeader.Document_date
	projectJnlLine.Country_code = purchHeader.Vat_country_code
	projectJnlLine.Reason_code = purchHeader.Reason_code
	projectJnlLine.Project_no = purchLine.Project_no
	projectJnlLine.No = purchLine.No
	projectJnlLine.Description = purchLine.Description
	projectJnlLine.Unit_of_measure_code = purchLine.Unit_of_measure_code
	projectJnlLine.Location_code = purchLine.Location_code
	projectJnlLine.Posting_group = purchLine.Posting_group
	projectJnlLine.Dimension_1_code = purchLine.Dimension_1_code
	projectJnlLine.Dimension_2_code = purchLine.Dimension_2_code
	projectJnlLine.Phase_code = purchLine.Phase_code
	projectJnlLine.Task_code = purchLine.Task_code
	projectJnlLine.Step_code = purchLine.Step_code
	projectJnlLine.Gen_bus_posting_group = purchLine.Gen_bus_posting_group
	projectJnlLine.Gen_prod_posting_group = purchLine.Gen_prod_posting_group
	projectJnlLine.Transaction_type = purchLine.Transaction_type
	projectJnlLine.Transport_method = purchLine.Transport_method
	projectJnlLine.Entry_exit_point = purchLine.Entry_point
	projectJnlLine.Area = purchLine.Area
	projectJnlLine.Transaction_specification = purchLine.Transaction_specification
	projectJnlLine.Entry_type = gtables.ProjectJournalLine_Entry_type.Usage
	projectJnlLine.Document_no = p.genJnlLineDocNo
	projectJnlLine.External_document_no = p.genJnlLineExtDocNo
	switch purchLine.Type {
	case gtables.PurchaseLine_Type.GLAccount:
		projectJnlLine.Type = gtables.ProjectJournalLine_Type.AccountGL
	case gtables.PurchaseLine_Type.Item:
		projectJnlLine.Type = gtables.ProjectJournalLine_Type.Item
	}
	projectJnlLine.Quantity = qtyToInvoice.Mul(p.sign)
	projectJnlLine.Quantity_base = qtyToInvoiceBase.Mul(p.sign)
	projectJnlLine.Qty_per_unit_of_measure = purchLine.Qty_per_unit_of_measure
	totalCost := lineLCY.Amount_including_tax
	if (purchLine.Tax_calculation_type == gtables.PurchaseLine_Tax_calculation_type.SalesTax) && purchLine.Use_tax {
		taxAmount, err := salestax.New(purchLine).CalculateTax(purchLine.Tax_area_code, purchLine.Tax_group_code, purchLine.Tax_liable,
			purchHeader.Posting_date, lineLCY.Amount, lineLCY.Quantity, purchHeader.Currency_factor)
		if err != nil {
			return err
		}
		totalCost = lineLCY.Amount.Add(taxAmount)
	}
	projectJnlLine.Total_cost = totalCost.Mul(p.sign)
	glSetup := gtables.GetGeneralLedgerSetup(p.src)
	projectJnlLine.Direct_unit_cost = system.Round(projectJnlLine.Total_cost.Div(projectJnlLine.Quantity), unitAmountPrecision(glSetup))
	projectJnlLine.Total_cost = system.Round(projectJnlLine.Total_cost)
	projectJnlLine.Total_price = system.Round(purchLine.Unit_price_lcy.Mul(projectJnlLine.Quantity).Mul(purchLine.Qty_per_unit_of_measure))
	projectJnlLine.Source_code = p.srcCode
	projectJnlLine.Post_project_entry_only = true
	projectJnlLine.Posting_no_series = purchHeader.Posting_no_series
	projectJnlLine.Source_currency_code = purchHeader.Currency_code
	precision := p.amountRoundingPrecision(glSetup)
	projectJnlLine.Source_currency_total_cost = system.Round(purchLine.Unit_cost.Mul(projectJnlLine.Quantity), precision)
	projectJnlLine.Source_currency_total_price = system.Round(purchLine.Unit_price_lcy.Mul(projectJnlLine.Quantity).Mul(purchLine.Qty_per_unit_of_measure), precision)
	if purchHeader.Currency_code != "" {
		// the price in the document's currency
		currExchRate := tables.ExchangeRates(p.src)
		unitPriceFCY, err := currExchRate.ExchangeAmtLCYToFCY(purchHeader.Posting_date, purchHeader.Currency_code, purchLine.Unit_price_lcy, purchHeader.Currency_factor)
		if err != nil {
			return err
		}
		projectJnlLine.Source_currency_total_price = system.Round(unitPriceFCY.Mul(projectJnlLine.Quantity).Mul(purchLine.Qty_per_unit_of_measure), precision)
	}
	if p.projectJnlPostLine == nil {
		p.projectJnlPostLine = projectjnl.NewProjectJnlPostLine(p.src)
	}
	return p.projectJnlPostLine.Run(projectJnlLine)
}

// unitAmountPrecision: the General Ledger Setup's unit-amount rounding precision
func unitAmountPrecision(glSetup *gtables.GeneralLedgerSetup) types.Decimal {
	if glSetup.Unit_amount_rounding_precision.IsZero() {
		return types.MustDecimal("0.00001")
	}
	return glSetup.Unit_amount_rounding_precision
}

// amountRoundingPrecision: the document currency's amount rounding precision (LCY: the General
// Ledger Setup's)
func (p *PurchPost) amountRoundingPrecision(glSetup *gtables.GeneralLedgerSetup) types.Decimal {
	if p.purchHeader.Currency_code != "" {
		currency := gtables.NewCurrency(p.src)
		if currency.Get(p.purchHeader.Currency_code) && !currency.Amount_rounding_precision.IsZero() {
			return currency.Amount_rounding_precision
		}
	}
	if glSetup.Amount_rounding_precision.IsZero() {
		return types.MustDecimal("0.01")
	}
	return glSetup.Amount_rounding_precision
}
