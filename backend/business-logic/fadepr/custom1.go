package fadepr

import (
	apperrors "github.com/hansjlachmann/goeasyerp/backend/foundation/errors"
	ftables "github.com/hansjlachmann/goeasyerp/backend/foundation/tables"
	"github.com/hansjlachmann/goeasyerp/backend/foundation/types"
	gtables "github.com/hansjlachmann/goeasyerp/backend/generated/tables"
)

// custom1Depr is codeunit 5612 Calculate Custom 1 Depr.: with the book's Use Custom 1
// Depreciation and an asset with a Depr. Ending Date (Custom 1), the depreciation and a custom 1
// depreciation (an accelerated depreciation: Depr. This Year % (Custom 1) of the acquisition cost
// up to Accum. Depr. % (Custom 1) until the Depr. Ending Date (Custom 1)) — the normal
// depreciation then works on the acquisition cost until that date and on the book value at that
// date afterwards (personal property shortens the remaining life by the custom 1 years).
type custom1Depr struct {
	normalDepr
	extraDays               int
	numberOfDays4           int
	custom1DeprStartingDate types.Date
	custom1DeprUntil        types.Date
	custom1AccumPercent     types.Decimal
	custom1PercentThisYear  types.Decimal
	custom1PropertyClass    gtables.FADepreciationBookProperty_class_custom_1
	acquisitionCost         types.Decimal
	custom1Depr             types.Decimal
}

// calculateCustom1 (Calculate): the depreciation and custom 1 amounts and their days
func calculateCustom1(src ftables.Source, faNo, deprBookCode types.Code, untilDate types.Date, entryAmounts EntryAmounts, dateFromProjection types.Date, daysInPeriod int) (deprAmount, custom1Amount types.Decimal, numberOfDays, custom1NumberOfDays int, err error) {
	c := &custom1Depr{normalDepr: normalDepr{src: src, deprBookCode: deprBookCode, untilDate: untilDate, entryAmounts: entryAmounts,
		dateFromProjection: dateFromProjection, daysInPeriod: daysInPeriod}}
	c.fa = gtables.NewFixedAsset(src)
	if err = c.fa.GetOrError(faNo); err != nil {
		return
	}
	c.deprBook = gtables.NewDepreciationBook(src)
	if err = c.deprBook.GetOrError(deprBookCode); err != nil {
		return
	}
	c.faDeprBook = gtables.NewFADepreciationBook(src)
	if !c.faDeprBook.Get(map[string]interface{}{"fa_no": faNo, "depreciation_book_code": deprBookCode}) {
		return
	}
	for _, check := range []func() error{
		func() error { return c.deprBook.TestField("allow_depr_below_zero", false) },
		func() error { return c.faDeprBook.TestField("fixed_depr_amount_below_zero", types.Decimal{}) },
		func() error { return c.faDeprBook.TestField("depr_below_zero_percent", types.Decimal{}) },
		func() error {
			return c.deprBook.TestField("periodic_depr_date_calc", gtables.DepreciationBook_Periodic_depr_date_calc.LastEntry)
		},
		func() error { return c.faDeprBook.TestField("property_class_custom_1") },
	} {
		if err = check(); err != nil {
			return
		}
	}
	setup := gtables.NewFAPostingTypeSetup(src)
	if err = setup.GetOrError(map[string]interface{}{"depreciation_book_code": deprBookCode, "fa_posting_type": int(gtables.FAPostingTypeSetup_Fa_posting_type.Custom1)}); err != nil {
		return
	}
	for _, check := range []func() error{
		func() error { return setup.TestField("part_of_book_value", true) },
		func() error { return setup.TestField("part_of_depreciable_basis", false) },
		func() error { return setup.TestField("include_in_depr_calculation", true) },
		func() error { return setup.TestField("sign", gtables.FAPostingTypeSetup_Sign.Credit) },
	} {
		if err = check(); err != nil {
			return
		}
	}
	if err = c.transferCustom1Values(); err != nil || c.skipRecord() {
		return
	}
	c.sign = 1
	if !c.faDeprBook.Use_fa_ledger_check {
		if c.deprBook.Use_fa_ledger_check {
			if err = c.faDeprBook.TestField("use_fa_ledger_check", true); err != nil {
				return
			}
		}
		c.sign = getCustom1Sign(c.bookValue, c.acquisitionCost, c.custom1Depr, c.salvageValue, c.minusBookValue)
		if c.sign == 0 {
			return
		}
		if c.sign == -1 {
			c.bookValue, c.acquisitionCost, c.custom1Depr, c.salvageValue, c.minusBookValue =
				c.bookValue.Neg(), c.acquisitionCost.Neg(), c.custom1Depr.Neg(), c.salvageValue.Neg(), c.minusBookValue.Neg()
		}
	}
	if !c.bookValue.Add(c.salvageValue).IsPositive() {
		return
	}
	if !c.salvageValue.IsNegative() && !c.bookValue.GreaterThan(c.endingBookValue) {
		return
	}
	faName := func() (string, string) { return faNo.String(), deprBookCode.String() }
	if !c.dateFromProjection.IsZero() {
		c.firstDeprDate = c.dateFromProjection
	} else {
		c.firstDeprDate, err = GetFirstDeprDate(src, faNo, deprBookCode)
		if err != nil || c.firstDeprDate.After(c.untilDate) || c.firstDeprDate.IsZero() {
			return
		}
		if c.custom1DeprUntil.IsZero() || !c.firstDeprDate.After(c.custom1DeprUntil) {
			c.useDeprStartingDate, err = UseDeprStartingDate(src, faNo, deprBookCode)
			if err != nil {
				return
			}
			if c.useDeprStartingDate {
				c.firstDeprDate = c.deprStartingDate
			}
		}
		if c.firstDeprDate.Before(c.deprStartingDate) {
			c.firstDeprDate = c.deprStartingDate
		}
		if c.firstDeprDate.After(c.untilDate) {
			return
		}
	}
	if c.useDeprStartingDate {
		c.extraDays = DeprDays(c.custom1DeprStartingDate, c.deprStartingDate) - 1
	}
	if !c.custom1DeprUntil.IsZero() && !c.firstDeprDate.After(c.custom1DeprUntil) && c.untilDate.After(c.custom1DeprUntil) {
		n, b := faName()
		err = apperrors.FACustom1EntryOnDate(n, b, c.custom1DeprUntil.String())
		return
	}
	c.numberOfDays = DeprDays(c.firstDeprDate, c.untilDate)
	if c.numberOfDays <= 0 {
		return
	}
	if c.daysInPeriod > 0 {
		c.numberOfDays4 = c.numberOfDays
		c.numberOfDays = c.daysInPeriod
		c.extraDays = 0
	}
	c.deprBasis, err = c.calcDeprBasis()
	if err != nil {
		return
	}
	switch c.method {
	case methodStraightLine:
		deprAmount = c.calcCustom1SLAmount()
	case methodDB1:
		deprAmount, err = c.calcDB1Amount()
	case methodDB2:
		deprAmount = c.calcDB2Amount()
	case methodUserDefined:
		deprAmount, err = c.calcCustom1TableAmount()
	}
	if err != nil {
		return
	}
	custom1Amount = c.acquisitionCost.Mul(c.calcCustom1DeprPercent()).Div(dec100).Neg()
	deprAmount, custom1Amount = AdjustCustom1(c.deprBook, deprAmount, custom1Amount, c.bookValue, c.salvageValue, c.endingBookValue, c.finalRoundingAmount)
	s := decInt(c.sign)
	return s.Mul(deprAmount), s.Mul(custom1Amount), c.numberOfDays, c.numberOfDays + c.extraDays, nil
}

// calcCustom1SLAmount (CalcSLAmount): straight-line on the custom 1 basis
func (c *custom1Depr) calcCustom1SLAmount() types.Decimal {
	if c.slPercent.IsPositive() {
		return c.deprBasis.Mul(c.calcSLPercent()).Div(dec100).Neg()
	}
	if c.fixedAmount.IsPositive() {
		return c.fixedAmount.Mul(decInt(c.numberOfDays)).Div(decInt(c.daysInFiscalYear)).Neg()
	}
	if c.deprYears.IsPositive() {
		if c.custom1DeprUntil.IsZero() || c.untilDate.After(c.custom1DeprUntil) {
			remainingLife := c.deprYears.Mul(decInt(c.daysInFiscalYear)).Add(decInt(c.numberOfCustom1DeprDays())).
				Sub(decInt(DeprDays(c.deprStartingDate, Yesterday(c.firstDeprDate))))
			if remainingLife.LessThan(types.NewDecimalFromInt(1)) {
				return c.bookValue.Neg()
			}
			return c.bookValue.Add(c.salvageValue).Sub(c.minusBookValue).Mul(decInt(c.numberOfDays)).Div(remainingLife).Neg()
		}
		return c.acquisitionCost.Mul(decInt(c.numberOfDays)).Div(c.deprYears).Div(decInt(c.daysInFiscalYear)).Neg()
	}
	return types.Decimal{}
}

// calcCustom1TableAmount (CalcCustom1Amount): the depreciation table's percent of the
// acquisition cost
func (c *custom1Depr) calcCustom1TableAmount() (types.Decimal, error) {
	factor := types.NewDecimalFromInt(1)
	if c.daysInPeriod > 0 {
		factor = decInt(c.daysInPeriod).Div(decInt(c.numberOfDays4))
	}
	percent, err := getTablePercent(c.src, c.deprBook, c.deprTableCode, c.firstUserDefinedDeprDate, c.firstDeprDate, c.untilDate)
	if err != nil {
		return types.Decimal{}, err
	}
	return percent.Mul(c.acquisitionCost).Mul(factor).Neg(), nil
}

// calcSLPercent (CalcSLPercent): the straight-line percent of the period — after the custom 1
// depreciation of personal property spread over the years left
func (c *custom1Depr) calcSLPercent() types.Decimal {
	fraction := decInt(c.numberOfDays).Div(decInt(c.daysInFiscalYear))
	if !c.slPercent.IsPositive() {
		return types.Decimal{}
	}
	if c.custom1PropertyClass == gtables.FADepreciationBook_Property_class_custom_1.RealProperty ||
		c.custom1DeprUntil.IsZero() || !c.untilDate.After(c.custom1DeprUntil) {
		return c.slPercent.Mul(fraction)
	}
	yearsOfCustom1Depr := decInt(DeprDays(c.custom1DeprStartingDate, c.custom1DeprUntil)).Div(decInt(c.daysInFiscalYear))
	calcDeprYears := dec100.Div(c.slPercent)
	if !calcDeprYears.Sub(yearsOfCustom1Depr).GreaterThan(types.MustDecimal("0.001")) {
		return types.Decimal{}
	}
	return dec100.Mul(fraction).Div(calcDeprYears.Sub(yearsOfCustom1Depr))
}

// calcCustom1DeprPercent (CalcCustom1DeprPercent): this year's custom 1 percent for the days, at
// most what is left of the accumulated percent
func (c *custom1Depr) calcCustom1DeprPercent() types.Decimal {
	if c.custom1DeprUntil.IsZero() || c.untilDate.After(c.custom1DeprUntil) || c.acquisitionCost.LessThan(types.MustDecimal("0.01")) {
		return types.Decimal{}
	}
	maxPercent := c.custom1AccumPercent.Sub(c.custom1Depr.Neg().Mul(dec100).Div(c.acquisitionCost))
	if maxPercent.IsNegative() {
		return types.Decimal{}
	}
	currentPercent := c.custom1PercentThisYear.Mul(decInt(c.numberOfDays + c.extraDays)).Div(decInt(c.daysInFiscalYear))
	if currentPercent.GreaterThan(maxPercent) {
		currentPercent = maxPercent
	}
	return currentPercent
}

// calcDeprBasis (CalcDeprBasis): the acquisition cost until the Depr. Ending Date (Custom 1),
// after it the book value at that date
func (c *custom1Depr) calcDeprBasis() (types.Decimal, error) {
	if c.custom1DeprUntil.IsZero() || !c.untilDate.After(c.custom1DeprUntil) {
		return c.acquisitionCost, nil
	}
	faLedgEntry := gtables.NewFALedgerEntry(c.src)
	faLedgEntry.SetRange("fa_no", c.fa.No)
	faLedgEntry.SetRange("depreciation_book_code", c.deprBookCode)
	faLedgEntry.SetRange("part_of_book_value", true)
	faLedgEntry.SetRange("fa_posting_date", types.Date{}, c.custom1DeprUntil)
	err := faLedgEntry.CalcSums("amount")
	if err != nil {
		return types.Decimal{}, err
	}
	if c.sign == -1 && faLedgEntry.Amount.IsPositive() {
		return types.Decimal{}, apperrors.FACustom1BookValuePositive(c.fa.No.String(), c.deprBookCode.String(), c.custom1DeprUntil.String())
	}
	if c.dateFromProjection.IsZero() {
		return faLedgEntry.Amount.Abs(), nil
	}
	return c.entryAmounts[3], nil
}

// numberOfCustom1DeprDays (NumberOfCustom1DeprDays): the custom 1 days added to the remaining
// life (real property; personal property none)
func (c *custom1Depr) numberOfCustom1DeprDays() int {
	if c.custom1PropertyClass == gtables.FADepreciationBook_Property_class_custom_1.PersonalProperty {
		return 0
	}
	return DeprDays(c.custom1DeprStartingDate, c.custom1DeprUntil)
}

// transferCustom1Values (TransferValues)
func (c *custom1Depr) transferCustom1Values() error {
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
	case gtables.FADepreciationBook_Depreciation_method.DecliningBalance1, gtables.FADepreciationBook_Depreciation_method.DecliningBalance2:
		if !b.Declining_balance_percent.LessThan(dec100) {
			return apperrors.FADBPercent100(faNo, deprBookCode)
		}
	}
	if c.dateFromProjection.IsZero() {
		b.CalcFields("book_value", "acquisition_cost", "custom_1", "salvage_value")
		c.bookValue = b.Book_value
		c.custom1Depr = b.Custom_1
	} else {
		b.CalcFields("acquisition_cost", "salvage_value")
		c.bookValue = c.entryAmounts[0]
		c.custom1Depr = c.entryAmounts[1]
	}
	c.minusBookValue, err = GetMinusBookValue(c.src, c.fa.No, c.deprBookCode, types.Date{}, types.Date{})
	if err != nil {
		return err
	}
	c.acquisitionCost = b.Acquisition_cost
	c.salvageValue = b.Salvage_value
	c.bookValue2 = b.Book_value
	c.salvageValue2 = b.Salvage_value
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
	c.daysInFiscalYear = c.deprBook.No_of_days_in_fiscal_year
	if c.daysInFiscalYear == 0 {
		c.daysInFiscalYear = 360
	}
	c.custom1DeprStartingDate = b.Depr_starting_date_custom_1
	c.custom1DeprUntil = b.Depr_ending_date_custom_1
	c.custom1AccumPercent = b.Accum_depr_percent_custom_1
	c.custom1PercentThisYear = b.Depr_this_year_percent_custom_1
	c.custom1PropertyClass = b.Property_class_custom_1
	if c.custom1DeprStartingDate.IsZero() {
		c.custom1DeprStartingDate = c.deprStartingDate
	}
	if c.custom1DeprStartingDate.After(c.deprStartingDate) {
		return apperrors.FADateNotLater(gtables.FADepreciationBookTableName, "depr_starting_date_custom_1", "depreciation_starting_date", faNo, deprBookCode)
	}
	if !c.custom1DeprUntil.IsZero() && c.custom1DeprUntil.Before(c.deprStartingDate) {
		return apperrors.FADateNotLater(gtables.FADepreciationBookTableName, "depreciation_starting_date", "depr_ending_date_custom_1", faNo, deprBookCode)
	}
	if (b.Depreciation_method == gtables.FADepreciationBook_Depreciation_method.DB1SL || b.Depreciation_method == gtables.FADepreciationBook_Depreciation_method.DB2SL) &&
		!c.custom1DeprUntil.IsZero() {
		return apperrors.FANotTogetherWith(gtables.FADepreciationBookTableName, "depr_ending_date_custom_1", "depreciation_method", b.Depreciation_method)
	}
	return nil
}
