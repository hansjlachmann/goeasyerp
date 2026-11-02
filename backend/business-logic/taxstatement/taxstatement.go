// Package taxstatement is the tax statement: the amount of a statement line (report 12's
// CalcLineTotal, also used by the preview page 474) and codeunit 340 TaxStmtManagement (the
// template and statement a Tax Statement page shows).
package taxstatement

import (
	"strings"

	apperrors "github.com/hansjlachmann/openerp/backend/foundation/errors"
	"github.com/hansjlachmann/openerp/backend/foundation/system"
	ftables "github.com/hansjlachmann/openerp/backend/foundation/tables"
	"github.com/hansjlachmann/openerp/backend/foundation/types"
	gtables "github.com/hansjlachmann/openerp/backend/generated/tables"
)

// Selection: which tax entries a statement includes
type Selection int

const (
	SelectionOpen Selection = iota
	SelectionClosed
	SelectionOpenAndClosed
)

// PeriodSelection: the tax entries of the period only, or up to its end
type PeriodSelection int

const (
	BeforeAndWithinPeriod PeriodSelection = iota
	WithinPeriod
)

// maxLevels is how deep row totals may nest (classic RowNo array)
const maxLevels = 6

// Calculator calculates statement lines for a period and selection
type Calculator struct {
	Src             ftables.Source
	DateFilter      string // the statement's Date Filter ("" = all)
	Selection       Selection
	PeriodSelection PeriodSelection
	PrintInIntegers bool // amounts rounded down to whole numbers
	rowNo           [maxLevels]types.Code
}

// LineTotal (CalcLineTotal): the amount of a statement line — an account total's net change in
// the period, a tax entry total's amounts, the sum of the rows a row total names (a loop or
// nesting deeper than six rows is an error naming the rows); a line's Calculate With reverses
// it
func (c *Calculator) LineTotal(line *gtables.TaxStatementLine) (types.Decimal, error) {
	var total types.Decimal
	ok, err := c.calcLineTotal(line, &total, 0)
	if err != nil {
		return types.Decimal{}, err
	}
	if !ok {
		return types.Decimal{}, c.loopError(line)
	}
	return total, nil
}

func (c *Calculator) calcLineTotal(line *gtables.TaxStatementLine, total *types.Decimal, level int) (bool, error) {
	switch line.Type {
	case gtables.TaxStatementLine_Type.AccountTotaling:
		var amount types.Decimal
		if line.Account_totaling != "" {
			glAcc := gtables.NewGLAccount(c.Src)
			glAcc.SetFilter("no", line.Account_totaling.String())
			for found := glAcc.FindSetBuffered(); found; found = glAcc.Next() {
				if c.DateFilter != "" {
					err := glAcc.SetFlowFilter("date_filter", c.DateFilter)
					if err != nil {
						return false, err
					}
				}
				glAcc.CalcFields("net_change")
				amount = amount.Add(glAcc.Net_change)
			}
		}
		c.addTotal(line, amount, total)
	case gtables.TaxStatementLine_Type.TaxEntryTotaling:
		amount, err := c.taxEntryAmount(line)
		if err != nil {
			return false, err
		}
		c.addTotal(line, amount, total)
	case gtables.TaxStatementLine_Type.RowTotaling:
		if level >= maxLevels {
			return false, nil
		}
		level++
		c.rowNo[level-1] = line.Row_no
		if line.Row_totaling == "" {
			return true, nil
		}
		rows := gtables.NewTaxStatementLine(c.Src)
		rows.SetRange("statement_template_name", line.Statement_template_name)
		rows.SetRange("statement_name", line.Statement_name)
		rows.SetFilter("row_no", line.Row_totaling.String())
		for found := rows.FindSetBuffered(); found; found = rows.Next() {
			row := gtables.NewTaxStatementLine(c.Src)
			row.Copy(rows)
			ok, err := c.calcLineTotal(row, total, level)
			if err != nil {
				return false, err
			}
			if !ok {
				return false, nil
			}
		}
	}
	return true, nil
}

// addTotal (CalcTotalAmount): the line's amount with its sign (rounded down with Round to Whole
// Numbers) added to the total
func (c *Calculator) addTotal(line *gtables.TaxStatementLine, amount types.Decimal, total *types.Decimal) {
	if line.Calculate_with == gtables.TaxStatementLine_Calculate_with.OppositeSign {
		amount = amount.Neg()
	}
	if c.PrintInIntegers && line.Print {
		amount = system.Round(amount, types.NewDecimalFromInt(1), "<")
	}
	*total = total.Add(amount)
}

// taxEntryAmount: the tax entries of the line's type, posting groups, jurisdiction and use tax —
// in the period (or up to its end) and the selection — summed by the line's amount type
func (c *Calculator) taxEntryAmount(line *gtables.TaxStatementLine) (types.Decimal, error) {
	taxEntry := gtables.NewTaxEntry(c.Src)
	taxEntry.SetCurrentKey("type", "closed", "tax_bus_posting_group", "tax_prod_posting_group", "tax_jurisdiction_code", "use_tax", "posting_date")
	taxEntry.SetRange("type", int(line.Gen_posting_type))
	taxEntry.SetRange("tax_bus_posting_group", line.Tax_bus_posting_group)
	taxEntry.SetRange("tax_prod_posting_group", line.Tax_prod_posting_group)
	taxEntry.SetRange("tax_jurisdiction_code", line.Tax_jurisdiction_code)
	taxEntry.SetRange("use_tax", line.Use_tax)
	if c.DateFilter != "" {
		if c.PeriodSelection == BeforeAndWithinPeriod {
			_, to := dateRange(c.DateFilter)
			taxEntry.SetRange("posting_date", types.Date{}, to)
		} else {
			taxEntry.SetFilter("posting_date", c.DateFilter)
		}
	}
	switch c.Selection {
	case SelectionOpen:
		taxEntry.SetRange("closed", false)
	case SelectionClosed:
		taxEntry.SetRange("closed", true)
	}
	field := map[gtables.TaxStatementLineAmount_type]string{
		gtables.TaxStatementLine_Amount_type.Amount:           "amount",
		gtables.TaxStatementLine_Amount_type.Base:             "base",
		gtables.TaxStatementLine_Amount_type.UnrealizedAmount: "unrealized_amount",
		gtables.TaxStatementLine_Amount_type.UnrealizedBase:   "unrealized_base",
	}[line.Amount_type]
	if field == "" {
		return types.Decimal{}, nil
	}
	err := taxEntry.CalcSums(field)
	if err != nil {
		return types.Decimal{}, err
	}
	switch line.Amount_type {
	case gtables.TaxStatementLine_Amount_type.Base:
		return taxEntry.Base, nil
	case gtables.TaxStatementLine_Amount_type.UnrealizedAmount:
		return taxEntry.Unrealized_amount, nil
	case gtables.TaxStatementLine_Amount_type.UnrealizedBase:
		return taxEntry.Unrealized_base, nil
	}
	return taxEntry.Amount, nil
}

// loopError: the rows of a row total that goes in a circle or nests too deep (FIELDERROR on Row
// No. with the rows)
func (c *Calculator) loopError(line *gtables.TaxStatementLine) error {
	var parts []string
	for _, r := range c.rowNo {
		parts = append(parts, r.String())
	}
	return apperrors.TaxStmtRowLoop(gtables.TaxStatementLineTableName, line.Row_no.String(), strings.Join(parts, " => ")+" => ...")
}

// dateRange: the first and last date of a date filter ("a..b", "..b", "a"); an open end is zero
func dateRange(expr string) (from, to types.Date) {
	fromText, toText, isRange := strings.Cut(strings.TrimSpace(expr), "..")
	if !isRange {
		toText = fromText
	}
	from, _ = types.NewDateFromString(strings.TrimSpace(fromText))
	to, _ = types.NewDateFromString(strings.TrimSpace(toText))
	if to.IsZero() {
		to = types.NewDate(9999, 12, 31)
	}
	return from, to
}
