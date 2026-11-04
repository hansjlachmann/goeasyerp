// Package salestax is codeunit 398 Sales Tax Calculate: the sales tax of an amount in a tax area
// — the tax of each jurisdiction of the area (tax area lines, highest calculation order first)
// from the tax detail of its tax group in effect on the date: a percentage up to and above a
// maximum amount (sales tax, optionally tax on the tax before it) and an amount per quantity
// (excise tax).
package salestax

import (
	"github.com/shopspring/decimal"

	apperrors "github.com/hansjlachmann/goeasyerp/backend/foundation/errors"
	"github.com/hansjlachmann/goeasyerp/backend/foundation/system"
	ftables "github.com/hansjlachmann/goeasyerp/backend/foundation/tables"
	"github.com/hansjlachmann/goeasyerp/backend/foundation/types"
	gtables "github.com/hansjlachmann/goeasyerp/backend/generated/tables"
)

// SalesTaxCalculate holds what a calculation remembers: the maximum amounts used so far and the
// lines of InitSalesTaxLines
type SalesTaxCalculate struct {
	src ftables.Source

	// TaxDetailMaximumsTemp: the maximum amount/quantity left of each tax detail
	maximums map[string]decimal.Decimal

	// the lines of InitSalesTaxLines (TMPTaxDetail: the tax in Tax Below Maximum, the base in
	// Tax Above Maximum)
	lines                  []*gtables.TaxDetail
	lineIndex              int
	initialised            bool
	totalTaxAmountRounding decimal.Decimal
	totalForAllocation     decimal.Decimal
	remainingTaxDetails    int
}

// New opens the calculation in a database and company
func New(src ftables.Source) *SalesTaxCalculate {
	return &SalesTaxCalculate{src: src, maximums: map[string]decimal.Decimal{}}
}

// ClearMaximums starts again with the full maximum amounts (ClearSalesTaxMaximums)
func (c *SalesTaxCalculate) ClearMaximums() {
	c.maximums = map[string]decimal.Decimal{}
}

var hundred = decimal.NewFromInt(100)

// taxAreaLines walks the jurisdictions of a tax area from the highest calculation order down;
// it reports whether the calculation orders are unique (needed for tax on tax)
func (c *SalesTaxCalculate) taxAreaLines(taxAreaCode types.Code, each func(line *gtables.TaxAreaLine) error) (bool, error) {
	taxAreaLine := gtables.NewTaxAreaLine(c.src)
	taxAreaLine.SetCurrentKey("tax_area", "calculation_order")
	taxAreaLine.SetRange("tax_area", taxAreaCode)
	taxAreaLine.SetAscending(false)
	lastCalculationOrder := 0
	first := true
	violation := false
	for found := taxAreaLine.FindSetBuffered(); found; found = taxAreaLine.Next() {
		if !first && taxAreaLine.Calculation_order >= lastCalculationOrder {
			violation = true
		} else {
			lastCalculationOrder = taxAreaLine.Calculation_order
		}
		first = false
		err := each(taxAreaLine)
		if err != nil {
			return violation, err
		}
	}
	return violation, nil
}

// taxDetail is the tax detail of a jurisdiction, tax group (or the jurisdiction's general one)
// and tax type in effect on the date (the latest effective date; the group's own wins)
func (c *SalesTaxCalculate) taxDetail(jurisdiction, taxGroupCode types.Code, date types.Date, taxType gtables.TaxDetailTax_type) (*gtables.TaxDetail, bool) {
	taxDetail := gtables.NewTaxDetail(c.src)
	taxDetail.SetRange("tax_jurisdiction_code", jurisdiction)
	if taxGroupCode == "" {
		taxDetail.SetRange("tax_group_code", taxGroupCode)
	} else {
		taxDetail.SetFilter("tax_group_code", "''|"+taxGroupCode.String())
	}
	if date.IsZero() {
		date = system.WorkDate()
	}
	taxDetail.SetFilter("effective_date", "<="+date.String())
	taxDetail.SetRange("tax_type", taxType)
	if !taxDetail.FindLast() {
		return nil, false
	}
	return taxDetail, true
}

func maximumKey(t *gtables.TaxDetail) string {
	return t.Tax_jurisdiction_code.String() + "|" + t.Tax_group_code.String() + "|" + t.Tax_type.String() + "|" + t.Effective_date.String()
}

// maximumLeft is the maximum amount/quantity a tax detail has left (TaxDetailMaximumsTemp)
func (c *SalesTaxCalculate) maximumLeft(t *gtables.TaxDetail) decimal.Decimal {
	key := maximumKey(t)
	left, ok := c.maximums[key]
	if !ok {
		left = t.Maximum_amount_qty.Decimal
		c.maximums[key] = left
	}
	return left
}

// salesTaxOf: the sales tax of a base amount by a tax detail (its maximum left goes down)
func (c *SalesTaxCalculate) salesTaxOf(taxDetail *gtables.TaxDetail, taxBaseAmount, quantity decimal.Decimal, perQuantity bool) decimal.Decimal {
	maxAmountPerQty := c.maximumLeft(taxDetail)
	key := maximumKey(taxDetail)
	if taxBaseAmount.Abs().LessThanOrEqual(maxAmountPerQty) || taxDetail.Maximum_amount_qty.IsZero() {
		if perQuantity {
			c.maximums[key] = c.maximums[key].Sub(quantity)
		} else {
			c.maximums[key] = c.maximums[key].Sub(taxBaseAmount)
		}
		return taxBaseAmount.Mul(taxDetail.Tax_below_maximum.Decimal).Div(hundred)
	}
	maxAmount := taxBaseAmount.Div(taxBaseAmount.Abs()).Mul(maxAmountPerQty)
	c.maximums[key] = decimal.Zero
	return maxAmount.Mul(taxDetail.Tax_below_maximum.Decimal).Add(taxBaseAmount.Sub(maxAmount).Mul(taxDetail.Tax_above_maximum.Decimal)).Div(hundred)
}

// exciseTaxOf: the excise tax of a quantity by a tax detail
func (c *SalesTaxCalculate) exciseTaxOf(taxDetail *gtables.TaxDetail, quantity decimal.Decimal) decimal.Decimal {
	maxAmountPerQty := c.maximumLeft(taxDetail)
	key := maximumKey(taxDetail)
	if quantity.Abs().LessThanOrEqual(maxAmountPerQty) || taxDetail.Maximum_amount_qty.IsZero() {
		c.maximums[key] = c.maximums[key].Sub(quantity)
		return quantity.Mul(taxDetail.Tax_below_maximum.Decimal)
	}
	maxAmount := quantity.Div(quantity.Abs()).Mul(maxAmountPerQty)
	c.maximums[key] = decimal.Zero
	return maxAmount.Mul(taxDetail.Tax_below_maximum.Decimal).Add(quantity.Sub(maxAmount).Mul(taxDetail.Tax_above_maximum.Decimal))
}

func exchangeFactor(exchangeRate types.Decimal) decimal.Decimal {
	if exchangeRate.IsZero() {
		return decimal.NewFromInt(1)
	}
	return exchangeRate.Decimal.Div(hundred)
}

// CalculateTax returns the sales tax of an amount (without tax) in a tax area for a tax group on
// a date; nothing when not tax liable or without area or group
func (c *SalesTaxCalculate) CalculateTax(taxAreaCode, taxGroupCode types.Code, taxLiable bool, date types.Date, amount, quantity, exchangeRate types.Decimal) (types.Decimal, error) {
	taxAmount := decimal.Zero
	if !taxLiable || taxAreaCode == "" || taxGroupCode == "" || amount.IsZero() && quantity.IsZero() {
		return types.Decimal{}, nil
	}
	factor := exchangeFactor(exchangeRate)
	amountX := amount.Decimal.Mul(factor)
	taxOnTaxCalculated := false
	violation, err := c.taxAreaLines(taxAreaCode, func(taxAreaLine *gtables.TaxAreaLine) error {
		taxDetail, ok := c.taxDetail(taxAreaLine.Tax_jurisdiction_code, taxGroupCode, date, gtables.TaxDetail_Tax_type.SalesTax)
		if ok {
			taxOnTaxCalculated = taxOnTaxCalculated || taxDetail.Calculate_tax_on_tax
			taxBaseAmount := amountX
			if taxDetail.Calculate_tax_on_tax {
				taxBaseAmount = amountX.Add(taxAmount)
			}
			taxAmount = taxAmount.Add(c.salesTaxOf(taxDetail, taxBaseAmount, quantity.Decimal, false))
		}
		taxDetail, ok = c.taxDetail(taxAreaLine.Tax_jurisdiction_code, taxGroupCode, date, gtables.TaxDetail_Tax_type.ExciseTax)
		if ok {
			taxAmount = taxAmount.Add(c.exciseTaxOf(taxDetail, quantity.Decimal))
		}
		return nil
	})
	if err != nil {
		return types.Decimal{}, err
	}
	if taxOnTaxCalculated && violation {
		return types.Decimal{}, apperrors.SalesTaxCalculationOrder(taxAreaCode.String())
	}
	return system.Round(types.Decimal{Decimal: taxAmount.Div(factor)}), nil
}

// ReverseCalculateTax returns the amount without tax of an amount that includes the sales tax
// (the inverse of CalculateTax, piecewise linear with the maximum amounts)
func (c *SalesTaxCalculate) ReverseCalculateTax(taxAreaCode, taxGroupCode types.Code, taxLiable bool, date types.Date, totalAmount, quantity, exchangeRate types.Decimal) (types.Decimal, error) {
	if !taxLiable || taxAreaCode == "" || taxGroupCode == "" || totalAmount.IsZero() && quantity.IsZero() {
		return totalAmount, nil
	}
	factor := exchangeFactor(exchangeRate)
	total := totalAmount.Decimal.Mul(factor)
	one := decimal.NewFromInt(1)
	// Inclination, Constant and MaxRangeAmount per step (1-based, as the classic arrays)
	inclination := make([]decimal.Decimal, 12)
	constant := make([]decimal.Decimal, 12)
	maxRangeAmount := make([]decimal.Decimal, 12)
	steps := 1
	taxOnTaxCalculated := false
	addLess := func(i int, inclinationLess decimal.Decimal, taxOnTax bool) {
		if taxOnTax {
			inclination[i] = inclination[i].Add(one.Add(inclination[i]).Mul(inclinationLess))
			constant[i] = one.Add(inclinationLess).Mul(constant[i])
		} else {
			inclination[i] = inclination[i].Add(inclinationLess)
		}
	}
	violation, err := c.taxAreaLines(taxAreaCode, func(taxAreaLine *gtables.TaxAreaLine) error {
		taxDetail, ok := c.taxDetail(taxAreaLine.Tax_jurisdiction_code, taxGroupCode, date, gtables.TaxDetail_Tax_type.SalesTax)
		if ok {
			taxOnTaxCalculated = taxOnTaxCalculated || taxDetail.Calculate_tax_on_tax
			inclinationLess := taxDetail.Tax_below_maximum.Decimal.Div(hundred)
			inclinationHigher := taxDetail.Tax_above_maximum.Decimal.Div(hundred)
			maximum := taxDetail.Maximum_amount_qty.Decimal
			if maximum.IsZero() {
				for i := 1; i <= steps; i++ {
					addLess(i, inclinationLess, taxDetail.Calculate_tax_on_tax)
				}
			} else {
				constantHigher := taxDetail.Tax_below_maximum.Decimal.Sub(taxDetail.Tax_above_maximum.Decimal).Div(hundred).Mul(maximum)
				var splitAmount decimal.Decimal
				if taxDetail.Calculate_tax_on_tax {
					found := false
					for i := 1; i < steps; i++ {
						maxTaxAmount := one.Add(inclination[i]).Mul(maxRangeAmount[i]).Add(constant[i])
						if maximum.Abs().LessThan(maxTaxAmount) {
							splitAmount = maximum.Abs().Div(maximum).Mul(maximum.Abs().Sub(constant[i]).Div(one.Add(inclination[i])))
							found = true
							break
						}
					}
					if !found {
						splitAmount = maximum.Abs().Div(maximum).Mul(maximum.Abs().Sub(constant[steps])).Div(one.Add(inclination[steps]))
					}
				} else {
					splitAmount = maximum
				}
				inserted := false
				for i := 1; i <= steps; i++ {
					switch {
					case maxRangeAmount[i].LessThan(splitAmount) && maxRangeAmount[i].IsPositive():
						addLess(i, inclinationLess, taxDetail.Calculate_tax_on_tax)
					case maxRangeAmount[i].Equal(splitAmount):
						addLess(i, inclinationLess, taxDetail.Calculate_tax_on_tax)
						inserted = true
					case maxRangeAmount[i].GreaterThan(splitAmount) || maxRangeAmount[i].IsZero():
						if inserted {
							addLess(i, inclinationHigher, taxDetail.Calculate_tax_on_tax)
							constant[i] = constant[i].Add(constantHigher)
						} else {
							steps = steps + 1
							for j := steps; j >= i+1; j-- {
								inclination[j] = inclination[j-1]
								constant[j] = constant[j-1]
								maxRangeAmount[j] = maxRangeAmount[j-1]
							}
							addLess(i, inclinationLess, taxDetail.Calculate_tax_on_tax)
							maxRangeAmount[i] = splitAmount
							inserted = true
						}
					}
				}
			}
		}
		taxDetail, ok = c.taxDetail(taxAreaLine.Tax_jurisdiction_code, taxGroupCode, date, gtables.TaxDetail_Tax_type.ExciseTax)
		if ok {
			var constantHigher decimal.Decimal
			if quantity.Decimal.Abs().LessThanOrEqual(taxDetail.Maximum_amount_qty.Decimal) || taxDetail.Maximum_amount_qty.IsZero() {
				constantHigher = quantity.Decimal.Mul(taxDetail.Tax_below_maximum.Decimal)
			} else {
				maxAmount := quantity.Decimal.Div(quantity.Decimal.Abs()).Mul(taxDetail.Maximum_amount_qty.Decimal)
				constantHigher = maxAmount.Mul(taxDetail.Tax_below_maximum.Decimal).Add(quantity.Decimal.Sub(maxAmount).Mul(taxDetail.Tax_above_maximum.Decimal))
			}
			constantHigher = constantHigher.Abs()
			for i := 1; i <= steps; i++ {
				constant[i] = constant[i].Add(constantHigher)
			}
		}
		return nil
	})
	if err != nil {
		return types.Decimal{}, err
	}
	if taxOnTaxCalculated && violation {
		return types.Decimal{}, apperrors.SalesTaxCalculationOrder(taxAreaCode.String())
	}
	var amount decimal.Decimal
	found := false
	for i := 1; i < steps; i++ {
		maxTaxAmount := one.Add(inclination[i]).Mul(maxRangeAmount[i]).Add(constant[i])
		if total.Abs().LessThan(maxTaxAmount) {
			if !total.IsZero() {
				amount = total.Abs().Div(total).Mul(total.Abs().Sub(constant[i]).Div(one.Add(inclination[i])))
			}
			found = true
			break
		}
	}
	if !found && !total.IsZero() {
		amount = total.Abs().Div(total).Mul(total.Abs().Sub(constant[steps])).Div(one.Add(inclination[steps]))
	}
	return system.Round(types.Decimal{Decimal: amount.Div(factor)}), nil
}

// InitSalesTaxLines calculates the tax of an amount per jurisdiction for posting (GetSalesTaxLine
// returns them); the total must be the tax amount of the journal line (desiredTaxAmount)
func (c *SalesTaxCalculate) InitSalesTaxLines(taxAreaCode, taxGroupCode types.Code, taxLiable bool, amount, quantity types.Decimal, date types.Date, desiredTaxAmount types.Decimal) error {
	taxAmount := decimal.Zero
	c.initialised = true
	c.lines = nil
	c.lineIndex = 0
	c.remainingTaxDetails = 0
	if taxAreaCode == "" || taxGroupCode == "" {
		return nil
	}
	taxOnTaxCalculated := false
	addLine := func(taxDetail *gtables.TaxDetail, addedTaxAmount, taxBaseAmount decimal.Decimal) {
		tmpTaxDetail := gtables.NewTaxDetail(c.src)
		tmpTaxDetail.Copy(taxDetail)
		tmpTaxDetail.Tax_below_maximum = types.Decimal{Decimal: addedTaxAmount}
		tmpTaxDetail.Tax_above_maximum = types.Decimal{Decimal: taxBaseAmount}
		c.lines = append(c.lines, tmpTaxDetail)
		c.remainingTaxDetails = c.remainingTaxDetails + 1
	}
	violation, err := c.taxAreaLines(taxAreaCode, func(taxAreaLine *gtables.TaxAreaLine) error {
		taxBaseAmount := amount.Decimal
		taxDetail, ok := c.taxDetail(taxAreaLine.Tax_jurisdiction_code, taxGroupCode, date, gtables.TaxDetail_Tax_type.SalesTax)
		if ok {
			taxOnTaxCalculated = taxOnTaxCalculated || taxDetail.Calculate_tax_on_tax
			if taxDetail.Calculate_tax_on_tax {
				taxBaseAmount = amount.Decimal.Add(taxAmount)
			}
			addedTaxAmount := decimal.Zero
			if taxLiable {
				addedTaxAmount = c.salesTaxOf(taxDetail, taxBaseAmount, quantity.Decimal, true)
			}
			taxAmount = taxAmount.Add(addedTaxAmount)
			addLine(taxDetail, addedTaxAmount, taxBaseAmount)
		}
		taxDetail, ok = c.taxDetail(taxAreaLine.Tax_jurisdiction_code, taxGroupCode, date, gtables.TaxDetail_Tax_type.ExciseTax)
		if ok {
			addedTaxAmount := decimal.Zero
			if taxLiable {
				addedTaxAmount = c.exciseTaxOf(taxDetail, quantity.Decimal)
			}
			taxAmount = taxAmount.Add(addedTaxAmount)
			addLine(taxDetail, addedTaxAmount, taxBaseAmount)
		}
		return nil
	})
	if err != nil {
		return err
	}
	rounded := system.Round(types.Decimal{Decimal: taxAmount})
	if taxOnTaxCalculated && violation {
		return apperrors.SalesTaxCalculationOrder(taxAreaCode.String())
	}
	if !rounded.Equal(desiredTaxAmount) {
		return apperrors.SalesTaxAmountIncorrect(taxAreaCode.String(), taxGroupCode.String())
	}
	c.totalForAllocation = desiredTaxAmount.Decimal
	return nil
}

// GetSalesTaxLine returns the next jurisdiction's tax and base of InitSalesTaxLines (rounding
// carried to the next line); false when there are no more
func (c *SalesTaxCalculate) GetSalesTaxLine() (taxDetail *gtables.TaxDetail, taxAmount, taxBaseAmount types.Decimal, ok bool) {
	if !c.initialised {
		return nil, types.Decimal{}, types.Decimal{}, false
	}
	if c.lineIndex >= len(c.lines) {
		c.initialised = false
		return nil, types.Decimal{}, types.Decimal{}, false
	}
	if c.lineIndex == 0 {
		c.totalTaxAmountRounding = decimal.Zero
	}
	line := c.lines[c.lineIndex]
	c.lineIndex = c.lineIndex + 1
	taxBaseAmount = system.Round(line.Tax_above_maximum)
	amount := line.Tax_below_maximum.Decimal
	if !amount.IsZero() {
		taxAmount = system.Round(types.Decimal{Decimal: amount.Add(c.totalTaxAmountRounding)})
		c.totalTaxAmountRounding = amount.Add(c.totalTaxAmountRounding).Sub(taxAmount.Decimal)
	}
	if c.remainingTaxDetails == 0 || amount.Abs().GreaterThan(c.totalForAllocation.Abs()) {
		amount = c.totalForAllocation
	}
	c.totalForAllocation = c.totalForAllocation.Sub(amount)
	if line.Tax_below_maximum.IsZero() {
		taxAmount = types.Decimal{}
	}
	return line, taxAmount, taxBaseAmount, true
}
