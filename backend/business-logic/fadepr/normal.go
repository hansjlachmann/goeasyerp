package fadepr

import (
	"math"

	apperrors "github.com/hansjlachmann/openerp/backend/foundation/errors"
	"github.com/hansjlachmann/openerp/backend/foundation/system"
	ftables "github.com/hansjlachmann/openerp/backend/foundation/tables"
	"github.com/hansjlachmann/openerp/backend/foundation/types"
	gtables "github.com/hansjlachmann/openerp/backend/generated/tables"
)

// deprMethod is the depreciation method of a calculation (the FA depreciation book's, and
// BelowZero for an asset depreciated below zero)
type deprMethod int

const (
	methodStraightLine deprMethod = iota
	methodDB1
	methodDB2
	methodDB1SL
	methodDB2SL
	methodUserDefined
	methodManual
	methodBelowZero
)

// normalDepr is codeunit 5611 Calculate Normal Depreciation: the depreciation of an asset in a
// book up to a date — straight-line (Straight-Line %, Fixed Depr. Amount or No. of Depreciation
// Years / Depreciation Ending Date over the remaining life), declining-balance 1 (on the book
// value less this fiscal year's depreciation) and 2 (exponential), DB1/SL and DB2/SL (the larger
// of the two), user-defined (a depreciation table), below zero; by the book's Periodic Depr. Date
// Calc. from the last entry or period by period since the last depreciation. Adjusted to the
// ending book value and final rounding amount, rounded per the book.
type normalDepr struct {
	src                      ftables.Source
	fa                       *gtables.FixedAsset
	deprBook                 *gtables.DepreciationBook
	faDeprBook               *gtables.FADepreciationBook
	deprBookCode             types.Code
	daysInFiscalYear         int
	entryAmounts             EntryAmounts
	minusBookValue           types.Decimal
	dateFromProjection       types.Date
	skipOnZero               bool
	untilDate                types.Date
	sign                     int
	firstDeprDate            types.Date
	numberOfDays             int
	numberOfDays2            int
	daysInPeriod             int
	useDeprStartingDate      bool
	bookValue, bookValue2    types.Decimal
	deprBasis                types.Decimal
	salvageValue             types.Decimal
	salvageValue2            types.Decimal
	acquisitionDate          types.Date
	disposalDate             types.Date
	method                   deprMethod
	deprStartingDate         types.Date
	firstUserDefinedDeprDate types.Date
	slPercent, dbPercent     types.Decimal
	fixedAmount              types.Decimal
	deprYears                types.Decimal
	deprTableCode            types.Code
	finalRoundingAmount      types.Decimal
	endingBookValue          types.Decimal
	amountBelowZero          types.Decimal
	percentBelowZero         types.Decimal
	factor                   types.Decimal
}

var (
	dec100 = types.NewDecimalFromInt(100)
	dec360 = types.NewDecimalFromInt(360)
)

func decInt(i int) types.Decimal { return types.NewDecimalFromInt(int64(i)) }

// calculateNormal (Calculate): the depreciation amount (negative) and its number of days
func calculateNormal(src ftables.Source, faNo, deprBookCode types.Code, untilDate types.Date, entryAmounts EntryAmounts, dateFromProjection types.Date, daysInPeriod int) (types.Decimal, int, error) {
	c := &normalDepr{src: src, deprBookCode: deprBookCode, untilDate: untilDate, entryAmounts: entryAmounts,
		dateFromProjection: dateFromProjection, daysInPeriod: daysInPeriod}
	c.fa = gtables.NewFixedAsset(src)
	err := c.fa.GetOrError(faNo)
	if err != nil {
		return types.Decimal{}, 0, err
	}
	c.deprBook = gtables.NewDepreciationBook(src)
	err = c.deprBook.GetOrError(deprBookCode)
	if err != nil {
		return types.Decimal{}, 0, err
	}
	c.faDeprBook = gtables.NewFADepreciationBook(src)
	if !c.faDeprBook.Get(map[string]interface{}{"fa_no": faNo, "depreciation_book_code": deprBookCode}) {
		return types.Decimal{}, 0, nil
	}
	if daysInPeriod > 0 && c.deprBook.Periodic_depr_date_calc != gtables.DepreciationBook_Periodic_depr_date_calc.LastEntry {
		return types.Decimal{}, 0, apperrors.FAForceDaysLastEntry(gtables.DepreciationBookTableName, "periodic_depr_date_calc", gtables.DepreciationBook_Periodic_depr_date_calc.LastEntry)
	}
	err = c.transferValues()
	if err != nil || c.skipRecord() {
		return types.Decimal{}, 0, err
	}
	c.sign = 1
	if !c.faDeprBook.Use_fa_ledger_check {
		if c.deprBook.Use_fa_ledger_check {
			err = c.faDeprBook.TestField("use_fa_ledger_check", true)
			if err != nil {
				return types.Decimal{}, 0, err
			}
		}
		err = c.faDeprBook.TestField("fixed_depr_amount_below_zero", types.Decimal{})
		if err != nil {
			return types.Decimal{}, 0, err
		}
		err = c.faDeprBook.TestField("depr_below_zero_percent", types.Decimal{})
		if err != nil {
			return types.Decimal{}, 0, err
		}
		c.sign = GetSign(c.bookValue, c.deprBasis, c.salvageValue, c.minusBookValue)
		if c.sign == 0 {
			return types.Decimal{}, 0, nil
		}
		if c.sign == -1 {
			c.bookValue, c.deprBasis, c.salvageValue, c.minusBookValue = c.bookValue.Neg(), c.deprBasis.Neg(), c.salvageValue.Neg(), c.minusBookValue.Neg()
		}
	}
	if c.faDeprBook.Fixed_depr_amount_below_zero.IsPositive() || c.faDeprBook.Depr_below_zero_percent.IsPositive() {
		err = c.faDeprBook.TestField("use_fa_ledger_check", true)
		if err != nil {
			return types.Decimal{}, 0, err
		}
	}
	if !c.bookValue.Add(c.salvageValue).IsPositive() {
		c.skipOnZero = true
	}
	if !c.salvageValue.IsNegative() && !c.bookValue.GreaterThan(c.endingBookValue) {
		c.skipOnZero = true
	}
	if c.skipOnZero && !c.deprBook.Allow_depr_below_zero && !c.deprBook.Use_fa_ledger_check {
		return types.Decimal{}, 0, nil
	}
	if c.skipOnZero {
		c.method = methodBelowZero
	}
	amount, err := c.calculateDeprAmount()
	if err != nil {
		return types.Decimal{}, 0, err
	}
	deprAmount := decInt(c.sign).Mul(amount)
	if decInt(c.sign).Mul(deprAmount).IsPositive() {
		deprAmount = types.Decimal{}
	}
	return deprAmount, c.numberOfDays2, nil
}

// skipRecord (SkipRecord): a disposed, not yet acquired, manual, inactive or blocked asset
func (c *normalDepr) skipRecord() bool {
	return !c.disposalDate.IsZero() || c.acquisitionDate.IsZero() || c.method == methodManual ||
		c.acquisitionDate.After(c.untilDate) || c.fa.Inactive || c.fa.Blocked
}

// calculateDeprAmount (CalculateDeprAmount)
func (c *normalDepr) calculateDeprAmount() (types.Decimal, error) {
	var err error
	if !c.dateFromProjection.IsZero() {
		c.firstDeprDate = c.dateFromProjection
	} else {
		c.firstDeprDate, err = GetFirstDeprDate(c.src, c.fa.No, c.deprBookCode)
		if err != nil {
			return types.Decimal{}, err
		}
		if c.firstDeprDate.After(c.untilDate) {
			return types.Decimal{}, nil
		}
		c.useDeprStartingDate, err = UseDeprStartingDate(c.src, c.fa.No, c.deprBookCode)
		if err != nil {
			return types.Decimal{}, err
		}
		if c.useDeprStartingDate {
			c.firstDeprDate = c.deprStartingDate
		}
	}
	if c.firstDeprDate.Before(c.deprStartingDate) {
		c.firstDeprDate = c.deprStartingDate
	}
	c.numberOfDays = DeprDays(c.firstDeprDate, c.untilDate)
	c.factor = types.NewDecimalFromInt(1)
	if c.numberOfDays <= 0 {
		return types.Decimal{}, nil
	}
	if c.daysInPeriod > 0 {
		c.factor = decInt(c.daysInPeriod).Div(decInt(c.numberOfDays))
		c.numberOfDays = c.daysInPeriod
	}
	amount := types.Decimal{}
	if c.useDeprStartingDate || !c.dateFromProjection.IsZero() || c.method == methodBelowZero ||
		c.deprBook.Periodic_depr_date_calc == gtables.DepreciationBook_Periodic_depr_date_calc.LastEntry {
		// Method Last Entry
		c.numberOfDays2 = c.numberOfDays
		switch c.method {
		case methodStraightLine:
			amount, err = c.calcSLAmount()
		case methodDB1:
			amount, err = c.calcDB1Amount()
		case methodDB2:
			amount = c.calcDB2Amount()
		case methodDB1SL, methodDB2SL:
			amount, err = c.calcDBSLAmount()
		case methodUserDefined:
			amount, err = c.calcUserDefinedAmount(c.untilDate)
		case methodBelowZero:
			amount = c.calcBelowZeroAmount()
		}
		if err != nil {
			return types.Decimal{}, err
		}
	} else {
		// Method Last Depreciation Entry: period by period
		var startingDate, endingDate types.Date
		c.numberOfDays, err = getDeprPeriod(c.src, c.fa.No, c.deprBookCode, c.untilDate, &startingDate, &endingDate)
		if err != nil {
			return types.Decimal{}, err
		}
		c.firstDeprDate = startingDate
		c.numberOfDays2 = DeprDays(c.firstDeprDate, c.untilDate)
		for c.numberOfDays > 0 {
			c.bookValue, c.deprBasis, c.salvageValue, c.minusBookValue, err = calculateDeprInPeriod(c.src, c.fa.No, c.deprBookCode, endingDate, amount, c.sign)
			if err != nil {
				return types.Decimal{}, err
			}
			if GetSign(c.bookValue, c.deprBasis, c.salvageValue, c.minusBookValue) != 1 {
				return types.Decimal{}, nil
			}
			var periodAmount types.Decimal
			switch c.method {
			case methodStraightLine:
				periodAmount, err = c.calcSLAmount()
			case methodDB1:
				periodAmount, err = c.calcDB1Amount()
			case methodDB2:
				periodAmount = c.calcDB2Amount()
			case methodUserDefined:
				periodAmount, err = c.calcUserDefinedAmount(endingDate)
			}
			if err != nil {
				return types.Decimal{}, err
			}
			amount = amount.Add(periodAmount)
			c.numberOfDays, err = getDeprPeriod(c.src, c.fa.No, c.deprBookCode, c.untilDate, &startingDate, &endingDate)
			if err != nil {
				return types.Decimal{}, err
			}
			c.firstDeprDate = startingDate
		}
	}
	if !amount.IsNegative() {
		return types.Decimal{}, nil
	}
	if !c.skipOnZero {
		amount = AdjustDepr(c.deprBook, amount, c.bookValue2.Abs(), c.salvageValue2.Abs().Neg(), c.endingBookValue, c.finalRoundingAmount)
	}
	return system.Round(amount), nil
}

// fractionOfYear: the days as a part of the fiscal year
func (c *normalDepr) fractionOfYear() types.Decimal {
	return decInt(c.numberOfDays).Div(decInt(c.daysInFiscalYear))
}

// calcSLAmount (CalcSLAmount): Straight-Line % of the depreciable basis, the Fixed Depr. Amount,
// or the book value over the remaining life of the depreciation years
func (c *normalDepr) calcSLAmount() (types.Decimal, error) {
	if c.slPercent.IsPositive() {
		return c.slPercent.Div(dec100).Mul(c.fractionOfYear()).Mul(c.deprBasis).Neg(), nil
	}
	if c.fixedAmount.IsPositive() {
		return c.fixedAmount.Mul(decInt(c.numberOfDays)).Div(decInt(c.daysInFiscalYear)).Neg(), nil
	}
	if c.deprYears.IsPositive() {
		remainingLife := c.deprYears.Mul(decInt(c.daysInFiscalYear)).Sub(decInt(DeprDays(c.deprStartingDate, Yesterday(c.firstDeprDate))))
		if remainingLife.LessThan(types.NewDecimalFromInt(1)) {
			return c.bookValue.Neg(), nil
		}
		return c.bookValue.Add(c.salvageValue).Sub(c.minusBookValue).Mul(decInt(c.numberOfDays)).Div(remainingLife).Neg(), nil
	}
	return types.Decimal{}, nil
}

// calcDB1Amount (CalcDB1Amount): Declining-Balance % of the book value less this fiscal year's
// depreciation
func (c *normalDepr) calcDB1Amount() (types.Decimal, error) {
	var deprInFiscalYear types.Decimal
	if c.dateFromProjection.IsZero() {
		var err error
		deprInFiscalYear, err = DeprInFiscalYear(c.src, c.fa.No, c.deprBookCode, c.untilDate)
		if err != nil {
			return types.Decimal{}, err
		}
	} else {
		deprInFiscalYear = c.entryAmounts[2]
	}
	base := c.bookValue.Sub(c.minusBookValue).Sub(decInt(c.sign).Mul(deprInFiscalYear))
	return c.dbPercent.Div(dec100).Mul(c.fractionOfYear()).Mul(base).Neg(), nil
}

// calcDB2Amount (CalcDB2Amount): the book value times 1 - (1 - DB %) ^ (days / days in year)
func (c *normalDepr) calcDB2Amount() types.Decimal {
	return power(c.dbPercent, c.fractionOfYear()).Mul(c.bookValue.Sub(c.minusBookValue)).Neg()
}

// power: 1 - (1 - percent/100) ^ exponent
func power(percent, exponent types.Decimal) types.Decimal {
	base := types.NewDecimalFromInt(1).Sub(percent.Div(dec100)).Float64()
	return types.NewDecimalFromInt(1).Sub(types.NewDecimal(math.Pow(base, exponent.Float64())))
}

// calcDBSLAmount (CalcDBSLAmount): the larger of the declining-balance and straight-line amounts
func (c *normalDepr) calcDBSLAmount() (types.Decimal, error) {
	var dbAmount types.Decimal
	var err error
	if c.method == methodDB1SL {
		dbAmount, err = c.calcDB1Amount()
		if err != nil {
			return types.Decimal{}, err
		}
	} else {
		dbAmount = c.calcDB2Amount()
	}
	slAmount, err := c.calcSLAmount()
	if err != nil {
		return types.Decimal{}, err
	}
	if slAmount.LessThan(dbAmount) {
		return slAmount, nil
	}
	return dbAmount, nil
}

// calcUserDefinedAmount (CalcUserDefinedAmount): the depreciation table's percent of the period
// times the depreciable basis
func (c *normalDepr) calcUserDefinedAmount(endingDate types.Date) (types.Decimal, error) {
	percent, err := getTablePercent(c.src, c.deprBook, c.deprTableCode, c.firstUserDefinedDeprDate, c.firstDeprDate, endingDate)
	if err != nil {
		return types.Decimal{}, err
	}
	return percent.Mul(c.deprBasis).Mul(c.factor).Neg(), nil
}

// calcBelowZeroAmount (CalcBelowZeroAmount): Depr. below Zero % of the depreciable basis or the
// Fixed Depr. Amount below Zero
func (c *normalDepr) calcBelowZeroAmount() types.Decimal {
	if c.percentBelowZero.IsPositive() {
		return c.percentBelowZero.Div(dec100).Mul(c.fractionOfYear()).Mul(c.deprBasis).Neg()
	}
	if c.amountBelowZero.IsPositive() {
		return c.amountBelowZero.Mul(decInt(c.numberOfDays)).Div(decInt(c.daysInFiscalYear)).Neg()
	}
	return types.Decimal{}
}

// transferValues (TransferValues): the FA depreciation book's terms and amounts
func (c *normalDepr) transferValues() error {
	b := c.faDeprBook
	faNo, deprBookCode := c.fa.No.String(), c.deprBookCode.String()
	err := b.TestField("depreciation_starting_date")
	if err != nil {
		return err
	}
	if b.Depreciation_method == gtables.FADepreciationBook_Depreciation_method.UserDefined {
		err = b.TestField("depreciation_table_code")
		if err == nil {
			err = b.TestField("first_user_defined_depr_date")
		}
		if err != nil {
			return err
		}
	}
	switch b.Depreciation_method {
	case gtables.FADepreciationBook_Depreciation_method.DecliningBalance1, gtables.FADepreciationBook_Depreciation_method.DecliningBalance2,
		gtables.FADepreciationBook_Depreciation_method.DB1SL, gtables.FADepreciationBook_Depreciation_method.DB2SL:
		if !b.Declining_balance_percent.LessThan(dec100) {
			return apperrors.FADBPercent100(faNo, deprBookCode)
		}
	}
	if c.deprBook.Periodic_depr_date_calc == gtables.DepreciationBook_Periodic_depr_date_calc.LastDeprEntry &&
		b.Depreciation_method != gtables.FADepreciationBook_Depreciation_method.StraightLine {
		return apperrors.FAMethodStraightLine(faNo, deprBookCode)
	}
	if c.dateFromProjection.IsZero() {
		b.CalcFields("book_value")
		c.bookValue = b.Book_value
	} else {
		c.bookValue = c.entryAmounts[0]
	}
	c.minusBookValue, err = GetMinusBookValue(c.src, c.fa.No, c.deprBookCode, types.Date{}, types.Date{})
	if err != nil {
		return err
	}
	b.CalcFields("depreciable_basis", "salvage_value")
	c.deprBasis = b.Depreciable_basis
	c.salvageValue = b.Salvage_value
	c.bookValue2 = c.bookValue
	c.salvageValue2 = c.salvageValue
	c.method = deprMethod(b.Depreciation_method)
	c.deprStartingDate = b.Depreciation_starting_date
	c.deprTableCode = b.Depreciation_table_code
	c.firstUserDefinedDeprDate = b.First_user_defined_depr_date
	if b.Depreciation_method == gtables.FADepreciationBook_Depreciation_method.UserDefined && c.firstUserDefinedDeprDate.After(c.deprStartingDate) {
		return apperrors.FADateNotLater(gtables.FADepreciationBookTableName, "first_user_defined_depr_date", "depreciation_starting_date", faNo, deprBookCode)
	}
	c.slPercent = b.Straight_line_percent
	c.dbPercent = b.Declining_balance_percent
	c.deprYears = b.No_of_depreciation_years
	if !b.Depreciation_ending_date.IsZero() {
		if b.Depreciation_starting_date.After(b.Depreciation_ending_date) {
			return apperrors.FADateNotLater(gtables.FADepreciationBookTableName, "depreciation_starting_date", "depreciation_ending_date", faNo, deprBookCode)
		}
		c.deprYears = decInt(DeprDays(b.Depreciation_starting_date, b.Depreciation_ending_date)).Div(dec360)
	}
	c.fixedAmount = b.Fixed_depr_amount
	c.finalRoundingAmount = b.Final_rounding_amount
	if c.finalRoundingAmount.IsZero() {
		c.finalRoundingAmount = c.deprBook.Default_final_rounding_amount
	}
	c.endingBookValue = b.Ending_book_value
	if c.endingBookValue.IsZero() {
		c.endingBookValue = c.deprBook.Default_ending_book_value
	}
	c.acquisitionDate = b.Acquisition_date
	c.disposalDate = b.Disposal_date
	c.percentBelowZero = b.Depr_below_zero_percent
	c.amountBelowZero = b.Fixed_depr_amount_below_zero
	c.daysInFiscalYear = c.deprBook.No_of_days_in_fiscal_year
	if c.daysInFiscalYear == 0 {
		c.daysInFiscalYear = 360
	}
	return nil
}
