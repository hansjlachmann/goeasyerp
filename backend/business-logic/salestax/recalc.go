package salestax

import (
	"github.com/hansjlachmann/goeasyerp/backend/foundation/types"
	gtables "github.com/hansjlachmann/goeasyerp/backend/generated/tables"
)

// Recalculate Sales Tax (codeunit 398 RecalcSalesLines / RecalcPurchLines; Functions ›
// Recalculate Sales Tax on the sales and purchase documents): each line calculates its tax on its
// own amount, rounded per line; the tax of a group of lines — the same type, tax area and group,
// posting groups, dimension 1, dimension 2, project (and use tax on purchases) — calculated on
// their total can differ by the rounding. Per group the tax is calculated on the total tax base
// and quantity, and the difference to the lines' tax goes to the group's last line's Amount
// Including Tax.

// recalcGroup is one group of lines (classic TempLine1: the totals of the tax base and quantity;
// TempLine2: the totals of the tax already calculated), with its last line
type recalcGroup struct {
	key                     string
	taxAreaCode, taxGroup   types.Code
	taxLiable               bool
	taxBaseAmount, quantity types.Decimal
	amountInclTax, amount   types.Decimal
	lastLineNo              int
}

// recalcGroups adds a line to its group (in the order the groups first occur)
func recalcGroups(groups []*recalcGroup, key string, taxAreaCode, taxGroupCode types.Code, taxLiable bool, taxBaseAmount, quantity, amountInclTax, amount types.Decimal, lineNo int) []*recalcGroup {
	for _, group := range groups {
		if group.key == key {
			group.taxBaseAmount = group.taxBaseAmount.Add(taxBaseAmount)
			group.quantity = group.quantity.Add(quantity)
			group.amountInclTax = group.amountInclTax.Add(amountInclTax)
			group.amount = group.amount.Add(amount)
			group.lastLineNo = lineNo
			return groups
		}
	}
	return append(groups, &recalcGroup{key: key, taxAreaCode: taxAreaCode, taxGroup: taxGroupCode, taxLiable: taxLiable,
		taxBaseAmount: taxBaseAmount, quantity: quantity, amountInclTax: amountInclTax, amount: amount, lastLineNo: lineNo})
}

// RecalcSalesLines (RecalcSalesLines): the sales document's tax recalculated per group of lines
func (c *SalesTaxCalculate) RecalcSalesLines(salesHeader *gtables.SalesHeader) error {
	err := salesHeader.TestField("customer_posting_group")
	if err != nil {
		return err
	}
	for _, salesLineType := range []gtables.SalesLineType{gtables.SalesLine_Type.GLAccount, gtables.SalesLine_Type.Item, gtables.SalesLine_Type.Resource} {
		salesLine := gtables.NewSalesLine(salesHeader)
		salesLine.LockTable()
		salesLine.SetRange("document_type", int(salesHeader.Document_type))
		salesLine.SetRange("document_no", salesHeader.No)
		salesLine.SetRange("type", salesLineType)
		salesLine.SetRange("tax_liable", true)
		var groups []*recalcGroup
		for found := salesLine.FindSetBuffered(); found; found = salesLine.Next() {
			if salesLine.Tax_calculation_type == gtables.SalesLine_Tax_calculation_type.SalesTax {
				key := salesLine.Tax_area_code.String() + "\x00" + salesLine.Tax_group_code.String() + "\x00" +
					salesLine.Gen_prod_posting_group.String() + "\x00" + salesLine.Tax_prod_posting_group.String() + "\x00" +
					salesLine.Gen_bus_posting_group.String() + "\x00" + salesLine.Tax_bus_posting_group.String() + "\x00" +
					salesLine.Dimension_1_code.String() + "\x00" + salesLine.Dimension_2_code.String() + "\x00" + salesLine.Project_no.String()
				groups = recalcGroups(groups, key, salesLine.Tax_area_code, salesLine.Tax_group_code, salesLine.Tax_liable,
					salesLine.Tax_base_amount, salesLine.Quantity, salesLine.Amount_including_tax, salesLine.Amount, salesLine.Line_no)
			}
		}
		for _, group := range groups {
			taxAmount, err := c.CalculateTax(group.taxAreaCode, group.taxGroup, group.taxLiable, salesHeader.Posting_date,
				group.taxBaseAmount, group.quantity, salesHeader.Currency_factor)
			if err != nil {
				return err
			}
			totalLinesTax := group.amountInclTax.Sub(group.amount)
			totalChangeNeeded := taxAmount.Sub(totalLinesTax)
			if !totalChangeNeeded.IsZero() {
				lastLine := gtables.NewSalesLine(salesHeader)
				err = lastLine.GetOrError(map[string]interface{}{"document_type": int(salesHeader.Document_type), "document_no": salesHeader.No, "line_no": group.lastLineNo})
				if err != nil {
					return err
				}
				lastLine.Amount_including_tax = lastLine.Amount_including_tax.Add(totalChangeNeeded)
				err = lastLine.ModifyOrError(false)
				if err != nil {
					return err
				}
			}
		}
	}
	return nil
}

// RecalcPurchLines (RecalcPurchLines): the purchase document's tax recalculated per group of lines
// (use tax lines and the others apart)
func (c *SalesTaxCalculate) RecalcPurchLines(purchHeader *gtables.PurchaseHeader) error {
	err := purchHeader.TestField("vendor_posting_group")
	if err != nil {
		return err
	}
	for _, purchLineType := range []gtables.PurchaseLineType{gtables.PurchaseLine_Type.GLAccount, gtables.PurchaseLine_Type.Item, gtables.PurchaseLine_Type.FixedAsset} {
		purchLine := gtables.NewPurchaseLine(purchHeader)
		purchLine.LockTable()
		purchLine.SetRange("document_type", int(purchHeader.Document_type))
		purchLine.SetRange("document_no", purchHeader.No)
		purchLine.SetRange("type", purchLineType)
		purchLine.SetRange("tax_liable", true)
		var groups []*recalcGroup
		for found := purchLine.FindSetBuffered(); found; found = purchLine.Next() {
			if purchLine.Tax_calculation_type == gtables.PurchaseLine_Tax_calculation_type.SalesTax {
				useTax := "0"
				if purchLine.Use_tax {
					useTax = "1"
				}
				key := purchLine.Tax_area_code.String() + "\x00" + purchLine.Tax_group_code.String() + "\x00" +
					purchLine.Gen_prod_posting_group.String() + "\x00" + purchLine.Tax_prod_posting_group.String() + "\x00" +
					purchLine.Gen_bus_posting_group.String() + "\x00" + purchLine.Tax_bus_posting_group.String() + "\x00" + useTax + "\x00" +
					purchLine.Dimension_1_code.String() + "\x00" + purchLine.Dimension_2_code.String() + "\x00" + purchLine.Project_no.String()
				groups = recalcGroups(groups, key, purchLine.Tax_area_code, purchLine.Tax_group_code, purchLine.Tax_liable,
					purchLine.Tax_base_amount, purchLine.Quantity, purchLine.Amount_including_tax, purchLine.Amount, purchLine.Line_no)
			}
		}
		for _, group := range groups {
			taxAmount, err := c.CalculateTax(group.taxAreaCode, group.taxGroup, group.taxLiable, purchHeader.Posting_date,
				group.taxBaseAmount, group.quantity, purchHeader.Currency_factor)
			if err != nil {
				return err
			}
			totalLinesTax := group.amountInclTax.Sub(group.amount)
			totalChangeNeeded := taxAmount.Sub(totalLinesTax)
			if !totalChangeNeeded.IsZero() {
				lastLine := gtables.NewPurchaseLine(purchHeader)
				err = lastLine.GetOrError(map[string]interface{}{"document_type": int(purchHeader.Document_type), "document_no": purchHeader.No, "line_no": group.lastLineNo})
				if err != nil {
					return err
				}
				lastLine.Amount_including_tax = lastLine.Amount_including_tax.Add(totalChangeNeeded)
				err = lastLine.ModifyOrError(false)
				if err != nil {
					return err
				}
			}
		}
	}
	return nil
}
