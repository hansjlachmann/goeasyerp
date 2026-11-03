package salesposting

import (
	"github.com/hansjlachmann/openerp/backend/business-logic/projectjnl"
	"github.com/hansjlachmann/openerp/backend/foundation/system"
	"github.com/hansjlachmann/openerp/backend/foundation/types"
	gtables "github.com/hansjlachmann/openerp/backend/generated/tables"
)

// postProjectJnlLine posts the invoiced share of a line for a project as a project sale
// (codeunit 80's "Post job entry"): the project entry only, through codeunit 202 — the line's
// amounts are already in the G/L and its item, resource or account entries. Only invoices and
// credit memos take a project (orders are refused).
func (p *SalesPost) postProjectJnlLine(salesLine *gtables.SalesLine, qtyToInvoice, qtyToInvoiceBase types.Decimal, lineFCY, lineLCY *gtables.SalesLine) error {
	salesHeader := p.salesHeader
	if (salesHeader.Document_type != gtables.SalesHeader_Document_type.Invoice) && (salesHeader.Document_type != gtables.SalesHeader_Document_type.CreditMemo) {
		err := salesLine.TestField("project_no", types.Code(""))
		if err != nil {
			return err
		}
	}
	if (salesLine.Project_no == "") || qtyToInvoice.IsZero() {
		return nil
	}
	projectJnlLine := gtables.NewProjectJournalLine(p.src)
	projectJnlLine.Init()
	projectJnlLine.Posting_date = salesHeader.Posting_date
	projectJnlLine.Document_date = salesHeader.Document_date
	projectJnlLine.Country_code = salesHeader.Vat_country_code
	projectJnlLine.Reason_code = salesHeader.Reason_code
	projectJnlLine.Project_no = salesLine.Project_no
	projectJnlLine.No = salesLine.No
	projectJnlLine.Description = salesLine.Description
	projectJnlLine.Unit_of_measure_code = salesLine.Unit_of_measure_code
	projectJnlLine.Location_code = salesLine.Location_code
	projectJnlLine.Posting_group = salesLine.Posting_group
	projectJnlLine.Dimension_1_code = salesLine.Dimension_1_code
	projectJnlLine.Dimension_2_code = salesLine.Dimension_2_code
	projectJnlLine.Work_type_code = salesLine.Work_type_code
	projectJnlLine.Phase_code = salesLine.Phase_code
	projectJnlLine.Task_code = salesLine.Task_code
	projectJnlLine.Step_code = salesLine.Step_code
	projectJnlLine.Applies_to_entry = salesLine.Appl_to_project_entry
	projectJnlLine.Applies_to_id = salesLine.Project_applies_to_id
	projectJnlLine.Apply_and_close_project = salesLine.Apply_and_close_project
	projectJnlLine.Gen_bus_posting_group = salesLine.Gen_bus_posting_group
	projectJnlLine.Gen_prod_posting_group = salesLine.Gen_prod_posting_group
	projectJnlLine.Transaction_type = salesLine.Transaction_type
	projectJnlLine.Transport_method = salesLine.Transport_method
	projectJnlLine.Entry_exit_point = salesLine.Exit_point
	projectJnlLine.Area = salesLine.Area
	projectJnlLine.Transaction_specification = salesLine.Transaction_specification
	projectJnlLine.Entry_type = gtables.ProjectJournalLine_Entry_type.Sale
	projectJnlLine.Document_no = p.genJnlLineDocNo
	projectJnlLine.External_document_no = salesHeader.External_document_no
	switch salesLine.Type {
	case gtables.SalesLine_Type.GLAccount:
		projectJnlLine.Type = gtables.ProjectJournalLine_Type.AccountGL
	case gtables.SalesLine_Type.Item:
		projectJnlLine.Type = gtables.ProjectJournalLine_Type.Item
	case gtables.SalesLine_Type.Resource:
		projectJnlLine.Type = gtables.ProjectJournalLine_Type.Resource
	}
	// an invoice's amounts are reversed first (classic ReverseAmount): a sale entry turns the
	// signs again
	projectJnlLine.Quantity = qtyToInvoice.Mul(p.sign)
	projectJnlLine.Quantity_base = qtyToInvoiceBase.Mul(p.sign)
	projectJnlLine.Total_cost = lineLCY.Unit_cost_lcy.Mul(p.sign)
	projectJnlLine.Total_price = lineLCY.Amount.Mul(p.sign)
	projectJnlLine.Source_code = p.srcCode
	projectJnlLine.Post_project_entry_only = true
	projectJnlLine.Posting_no_series = salesHeader.Posting_no_series
	projectJnlLine.Source_currency_code = salesHeader.Currency_code
	projectJnlLine.Source_currency_total_cost = system.Round(salesLine.Unit_cost.Mul(projectJnlLine.Quantity), p.amountRoundingPrecision())
	projectJnlLine.Source_currency_total_price = lineFCY.Amount.Mul(p.sign)
	if p.projectJnlPostLine == nil {
		p.projectJnlPostLine = projectjnl.NewProjectJnlPostLine(p.src)
	}
	return p.projectJnlPostLine.Run(projectJnlLine)
}

// amountRoundingPrecision: the document currency's amount rounding precision (LCY: the General
// Ledger Setup's)
func (p *SalesPost) amountRoundingPrecision() types.Decimal {
	if p.salesHeader.Currency_code != "" {
		currency := gtables.NewCurrency(p.src)
		if currency.Get(p.salesHeader.Currency_code) && !currency.Amount_rounding_precision.IsZero() {
			return currency.Amount_rounding_precision
		}
	}
	glSetup := gtables.GetGeneralLedgerSetup(p.src)
	if glSetup.Amount_rounding_precision.IsZero() {
		return types.MustDecimal("0.01")
	}
	return glSetup.Amount_rounding_precision
}
