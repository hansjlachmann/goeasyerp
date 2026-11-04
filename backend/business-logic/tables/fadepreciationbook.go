package tables

import (
	apperrors "github.com/hansjlachmann/goeasyerp/backend/foundation/errors"
	"github.com/hansjlachmann/goeasyerp/backend/foundation/system"
	"github.com/hansjlachmann/goeasyerp/backend/foundation/types"
	gtables "github.com/hansjlachmann/goeasyerp/backend/generated/tables"
)

// fADepreciationBookLogic is the business logic of table 5612 FA Depreciation Book: the
// depreciation of a fixed asset in a depreciation book; its values are the FA ledger entries'
// totals (FlowFields)
type fADepreciationBookLogic struct{}

func init() { gtables.RegisterFADepreciationBookLogic(fADepreciationBookLogic{}) }

// OnInsert: the dates of the entries start blank; the description and main asset fields come
// from the fixed asset
func (fADepreciationBookLogic) OnInsert(rec *gtables.FADepreciationBook) error {
	rec.Acquisition_date = types.Date{}
	rec.G_l_acquisition_date = types.Date{}
	rec.Last_acquisition_cost_date = types.Date{}
	rec.Last_salvage_value_date = types.Date{}
	rec.Last_depreciation_date = types.Date{}
	rec.Last_write_down_date = types.Date{}
	rec.Last_appreciation_date = types.Date{}
	rec.Last_custom_1_date = types.Date{}
	rec.Last_custom_2_date = types.Date{}
	rec.Disposal_date = types.Date{}
	rec.Last_maintenance_date = types.Date{}
	fa := gtables.NewFixedAsset(rec)
	err := fa.GetOrError(rec.Fa_no)
	if err != nil {
		return err
	}
	deprBook := gtables.NewDepreciationBook(rec)
	err = deprBook.GetOrError(rec.Depreciation_book_code)
	if err != nil {
		return err
	}
	rec.Description = fa.Description
	rec.Main_asset_component = gtables.FADepreciationBookMain_asset_component(fa.Main_asset_component)
	rec.Component_of_main_asset = fa.Component_of_main_asset
	return nil
}

// OnModify sets Last Date Modified
func (fADepreciationBookLogic) OnModify(rec *gtables.FADepreciationBook) error {
	rec.Last_date_modified = system.Today()
	return nil
}

// OnDelete moves the entries of the fixed asset in the book away (MoveFAEntries)
func (fADepreciationBookLogic) OnDelete(rec *gtables.FADepreciationBook) error {
	return MoveFAEntries(rec)
}

// OnRename: a depreciation book of a fixed asset cannot be renamed
func (fADepreciationBookLogic) OnRename(rec *gtables.FADepreciationBook) error {
	return apperrors.RenameNotAllowed(gtables.FADepreciationBookTableName)
}

// OnValidate_Depreciation_method clears the fields the method does not use
func (fADepreciationBookLogic) OnValidate_Depreciation_method(rec *gtables.FADepreciationBook) error {
	err := modifyDeprFields(rec)
	if err != nil {
		return err
	}
	zero := types.ZeroDecimal()
	switch rec.Depreciation_method {
	case gtables.FADepreciationBook_Depreciation_method.StraightLine:
		rec.Declining_balance_percent = zero
		rec.Depreciation_table_code = ""
		rec.First_user_defined_depr_date = types.Date{}
	case gtables.FADepreciationBook_Depreciation_method.DecliningBalance1,
		gtables.FADepreciationBook_Depreciation_method.DecliningBalance2:
		rec.Straight_line_percent = zero
		rec.No_of_depreciation_years = zero
		rec.No_of_depreciation_months = zero
		rec.Fixed_depr_amount = zero
		rec.Depreciation_ending_date = types.Date{}
		rec.Depreciation_table_code = ""
		rec.First_user_defined_depr_date = types.Date{}
	case gtables.FADepreciationBook_Depreciation_method.DB1SL,
		gtables.FADepreciationBook_Depreciation_method.DB2SL:
		rec.Depreciation_table_code = ""
		rec.First_user_defined_depr_date = types.Date{}
	case gtables.FADepreciationBook_Depreciation_method.UserDefined:
		rec.Straight_line_percent = zero
		rec.No_of_depreciation_years = zero
		rec.No_of_depreciation_months = zero
		rec.Fixed_depr_amount = zero
		rec.Depreciation_ending_date = types.Date{}
		rec.Declining_balance_percent = zero
	case gtables.FADepreciationBook_Depreciation_method.Manual:
		rec.Straight_line_percent = zero
		rec.No_of_depreciation_years = zero
		rec.No_of_depreciation_months = zero
		rec.Fixed_depr_amount = zero
		rec.Depreciation_ending_date = types.Date{}
		rec.Declining_balance_percent = zero
		rec.Depreciation_table_code = ""
		rec.First_user_defined_depr_date = types.Date{}
	}
	return nil
}

// OnValidate_Depreciation_starting_date recalculates the depreciation period
func (fADepreciationBookLogic) OnValidate_Depreciation_starting_date(rec *gtables.FADepreciationBook) error {
	err := modifyDeprFields(rec)
	if err != nil {
		return err
	}
	return calcDeprPeriod(rec)
}

// OnValidate_Straight_line_percent: only for a straight-line method; it replaces years and
// fixed amount
func (fADepreciationBookLogic) OnValidate_Straight_line_percent(rec *gtables.FADepreciationBook) error {
	err := modifyDeprFields(rec)
	if err != nil {
		return err
	}
	if !rec.Straight_line_percent.IsZero() && !linearMethod(rec) {
		return deprMethodError(rec)
	}
	adjustLinearMethod(rec, &rec.No_of_depreciation_years, &rec.Fixed_depr_amount)
	return nil
}

// OnValidate_No_of_depreciation_years gives the months and the ending date
func (fADepreciationBookLogic) OnValidate_No_of_depreciation_years(rec *gtables.FADepreciationBook) error {
	err := rec.TestField("depreciation_starting_date")
	if err != nil {
		return err
	}
	err = modifyDeprFields(rec)
	if err != nil {
		return err
	}
	if !rec.No_of_depreciation_years.IsZero() && !linearMethod(rec) {
		return deprMethodError(rec)
	}
	rec.No_of_depreciation_months = system.Round(rec.No_of_depreciation_years.Mul(types.NewDecimal(12)), types.MustDecimal("0.00000001"))
	adjustLinearMethod(rec, &rec.Straight_line_percent, &rec.Fixed_depr_amount)
	rec.Depreciation_ending_date = calcEndingDate(rec)
	return nil
}

// OnValidate_No_of_depreciation_months gives the years and the ending date
func (fADepreciationBookLogic) OnValidate_No_of_depreciation_months(rec *gtables.FADepreciationBook) error {
	err := rec.TestField("depreciation_starting_date")
	if err != nil {
		return err
	}
	err = modifyDeprFields(rec)
	if err != nil {
		return err
	}
	if !rec.No_of_depreciation_months.IsZero() && !linearMethod(rec) {
		return deprMethodError(rec)
	}
	rec.No_of_depreciation_years = system.Round(rec.No_of_depreciation_months.Div(types.NewDecimal(12)), types.MustDecimal("0.00000001"))
	adjustLinearMethod(rec, &rec.Straight_line_percent, &rec.Fixed_depr_amount)
	rec.Depreciation_ending_date = calcEndingDate(rec)
	return nil
}

// OnValidate_Fixed_depr_amount: only for a straight-line method; it replaces % and years
func (fADepreciationBookLogic) OnValidate_Fixed_depr_amount(rec *gtables.FADepreciationBook) error {
	err := modifyDeprFields(rec)
	if err != nil {
		return err
	}
	if !rec.Fixed_depr_amount.IsZero() && !linearMethod(rec) {
		return deprMethodError(rec)
	}
	adjustLinearMethod(rec, &rec.Straight_line_percent, &rec.No_of_depreciation_years)
	return nil
}

// OnValidate_Declining_balance_percent: below 100, only for a declining-balance method
func (fADepreciationBookLogic) OnValidate_Declining_balance_percent(rec *gtables.FADepreciationBook) error {
	if !rec.Declining_balance_percent.LessThan(types.NewDecimal(100)) {
		return apperrors.FieldMustNotBe(gtables.FADepreciationBookTableName, "declining_balance_percent", "100")
	}
	err := modifyDeprFields(rec)
	if err != nil {
		return err
	}
	if !rec.Declining_balance_percent.IsZero() && !decliningMethod(rec) {
		return deprMethodError(rec)
	}
	return nil
}

// OnValidate_Depreciation_table_code: only for the user-defined method
func (fADepreciationBookLogic) OnValidate_Depreciation_table_code(rec *gtables.FADepreciationBook) error {
	err := modifyDeprFields(rec)
	if err != nil {
		return err
	}
	if rec.Depreciation_table_code != "" && !userDefinedMethod(rec) {
		return deprMethodError(rec)
	}
	return nil
}

// OnValidate_Final_rounding_amount, Ending Book Value, FA Posting Group, FA Exchange Rate and
// the Custom 1 fields: only while the depreciation fields may change
func (fADepreciationBookLogic) OnValidate_Final_rounding_amount(rec *gtables.FADepreciationBook) error {
	return modifyDeprFields(rec)
}

func (fADepreciationBookLogic) OnValidate_Ending_book_value(rec *gtables.FADepreciationBook) error {
	return modifyDeprFields(rec)
}

func (fADepreciationBookLogic) OnValidate_Fa_posting_group(rec *gtables.FADepreciationBook) error {
	return modifyDeprFields(rec)
}

func (fADepreciationBookLogic) OnValidate_Fa_exchange_rate(rec *gtables.FADepreciationBook) error {
	return modifyDeprFields(rec)
}

func (fADepreciationBookLogic) OnValidate_Depr_starting_date_custom_1(rec *gtables.FADepreciationBook) error {
	return modifyDeprFields(rec)
}

func (fADepreciationBookLogic) OnValidate_Depr_ending_date_custom_1(rec *gtables.FADepreciationBook) error {
	return modifyDeprFields(rec)
}

func (fADepreciationBookLogic) OnValidate_Accum_depr_percent_custom_1(rec *gtables.FADepreciationBook) error {
	return modifyDeprFields(rec)
}

func (fADepreciationBookLogic) OnValidate_Property_class_custom_1(rec *gtables.FADepreciationBook) error {
	return modifyDeprFields(rec)
}

// OnValidate_Depreciation_ending_date: only for a straight-line method; it gives the period
func (fADepreciationBookLogic) OnValidate_Depreciation_ending_date(rec *gtables.FADepreciationBook) error {
	err := rec.TestField("depreciation_starting_date")
	if err != nil {
		return err
	}
	if !rec.Depreciation_ending_date.IsZero() && !linearMethod(rec) {
		return deprMethodError(rec)
	}
	err = modifyDeprFields(rec)
	if err != nil {
		return err
	}
	return calcDeprPeriod(rec)
}

// OnValidate_Fixed_depr_amount_below_zero: only in a book that allows depreciation below zero,
// with the FA ledger check
func (fADepreciationBookLogic) OnValidate_Fixed_depr_amount_below_zero(rec *gtables.FADepreciationBook) error {
	err := modifyDeprFields(rec)
	if err != nil {
		return err
	}
	rec.Depr_below_zero_percent = types.ZeroDecimal()
	if rec.Fixed_depr_amount_below_zero.IsPositive() {
		return testBelowZero(rec)
	}
	return nil
}

// OnValidate_Depr_below_zero_percent: as Fixed Depr. Amount below Zero
func (fADepreciationBookLogic) OnValidate_Depr_below_zero_percent(rec *gtables.FADepreciationBook) error {
	err := modifyDeprFields(rec)
	if err != nil {
		return err
	}
	rec.Fixed_depr_amount_below_zero = types.ZeroDecimal()
	if rec.Depr_below_zero_percent.IsPositive() {
		return testBelowZero(rec)
	}
	return nil
}

// OnValidate_First_user_defined_depr_date: only for the user-defined method
func (fADepreciationBookLogic) OnValidate_First_user_defined_depr_date(rec *gtables.FADepreciationBook) error {
	err := modifyDeprFields(rec)
	if err != nil {
		return err
	}
	if !rec.First_user_defined_depr_date.IsZero() && !userDefinedMethod(rec) {
		return deprMethodError(rec)
	}
	return nil
}

// OnValidate_Use_fa_ledger_check: switched off only where the book does not use it and nothing
// is depreciated below zero
func (fADepreciationBookLogic) OnValidate_Use_fa_ledger_check(rec *gtables.FADepreciationBook) error {
	if !rec.Use_fa_ledger_check {
		deprBook := gtables.NewDepreciationBook(rec)
		err := deprBook.GetOrError(rec.Depreciation_book_code)
		if err != nil {
			return err
		}
		err = deprBook.TestField("use_fa_ledger_check", false)
		if err != nil {
			return err
		}
		err = rec.TestField("fixed_depr_amount_below_zero", types.ZeroDecimal())
		if err != nil {
			return err
		}
		err = rec.TestField("depr_below_zero_percent", types.ZeroDecimal())
		if err != nil {
			return err
		}
	}
	return modifyDeprFields(rec)
}

func testBelowZero(rec *gtables.FADepreciationBook) error {
	deprBook := gtables.NewDepreciationBook(rec)
	err := deprBook.GetOrError(rec.Depreciation_book_code)
	if err != nil {
		return err
	}
	err = deprBook.TestField("allow_depr_below_zero", true)
	if err != nil {
		return err
	}
	return rec.TestField("use_fa_ledger_check", true)
}

// adjustLinearMethod: one of the three straight-line inputs (%, years, fixed amount) is used,
// the other two are cleared
func adjustLinearMethod(rec *gtables.FADepreciationBook, amount1, amount2 *types.Decimal) {
	*amount1 = types.ZeroDecimal()
	*amount2 = types.ZeroDecimal()
	if rec.No_of_depreciation_years.IsZero() {
		rec.No_of_depreciation_months = types.ZeroDecimal()
		rec.Depreciation_ending_date = types.Date{}
	}
}

// modifyDeprFields: once the asset is depreciated, written down, appreciated or disposed, its
// depreciation fields change only in a book that allows it
func modifyDeprFields(rec *gtables.FADepreciationBook) error {
	if !rec.Last_depreciation_date.IsZero() ||
		!rec.Last_write_down_date.IsZero() ||
		!rec.Last_appreciation_date.IsZero() ||
		!rec.Last_custom_1_date.IsZero() ||
		!rec.Last_custom_2_date.IsZero() ||
		!rec.Disposal_date.IsZero() {
		deprBook := gtables.NewDepreciationBook(rec)
		err := deprBook.GetOrError(rec.Depreciation_book_code)
		if err != nil {
			return err
		}
		return deprBook.TestField("allow_changes_in_depr_fields", true)
	}
	return nil
}

// calcDeprPeriod: the years and months between the starting and the ending date
func calcDeprPeriod(rec *gtables.FADepreciationBook) error {
	if rec.Depreciation_starting_date.IsZero() {
		rec.Depreciation_ending_date = types.Date{}
		rec.No_of_depreciation_years = types.ZeroDecimal()
		rec.No_of_depreciation_months = types.ZeroDecimal()
	}
	if rec.Depreciation_starting_date.IsZero() || rec.Depreciation_ending_date.IsZero() {
		rec.No_of_depreciation_years = types.ZeroDecimal()
		rec.No_of_depreciation_months = types.ZeroDecimal()
	} else {
		if rec.Depreciation_starting_date.After(rec.Depreciation_ending_date) {
			return apperrors.DateLater(gtables.FADepreciationBookTableName, "depreciation_starting_date", "depreciation_ending_date")
		}
		months := types.NewDecimalFromInt(int64(DeprDays(rec.Depreciation_starting_date, rec.Depreciation_ending_date))).Div(types.NewDecimal(30))
		rec.No_of_depreciation_months = system.Round(months, types.MustDecimal("0.00000001"))
		rec.No_of_depreciation_years = system.Round(rec.No_of_depreciation_months.Div(types.NewDecimal(12)), types.MustDecimal("0.00000001"))
		adjustLinearMethod(rec, &rec.Straight_line_percent, &rec.Fixed_depr_amount)
	}
	return nil
}

// calcEndingDate: the last day of the depreciation period (years of 360 days)
func calcEndingDate(rec *gtables.FADepreciationBook) types.Date {
	if rec.No_of_depreciation_years.IsZero() {
		return types.Date{}
	}
	days := system.Round(rec.No_of_depreciation_years.Mul(types.NewDecimal(360)), types.NewDecimal(1)).IntPart()
	endingDate := CalculateDate(rec.Depreciation_starting_date, int(days))
	endingDate = Yesterday(endingDate)
	if endingDate.Before(rec.Depreciation_starting_date) {
		endingDate = rec.Depreciation_starting_date
	}
	return endingDate
}

func linearMethod(rec *gtables.FADepreciationBook) bool {
	switch rec.Depreciation_method {
	case gtables.FADepreciationBook_Depreciation_method.StraightLine,
		gtables.FADepreciationBook_Depreciation_method.DB1SL,
		gtables.FADepreciationBook_Depreciation_method.DB2SL:
		return true
	}
	return false
}

func decliningMethod(rec *gtables.FADepreciationBook) bool {
	switch rec.Depreciation_method {
	case gtables.FADepreciationBook_Depreciation_method.DecliningBalance1,
		gtables.FADepreciationBook_Depreciation_method.DecliningBalance2,
		gtables.FADepreciationBook_Depreciation_method.DB1SL,
		gtables.FADepreciationBook_Depreciation_method.DB2SL:
		return true
	}
	return false
}

func userDefinedMethod(rec *gtables.FADepreciationBook) bool {
	return rec.Depreciation_method == gtables.FADepreciationBook_Depreciation_method.UserDefined
}

// deprMethodError: the field does not go with the depreciation method
func deprMethodError(rec *gtables.FADepreciationBook) error {
	return apperrors.FieldMustNotBeValue(gtables.FADepreciationBookTableName, "depreciation_method", rec.Depreciation_method)
}
