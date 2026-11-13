package reports

import (
	"sort"

	"github.com/hansjlachmann/goeasyerp/backend/business-logic/fadepr"
	"github.com/hansjlachmann/goeasyerp/backend/business-logic/fajnl"
	"github.com/hansjlachmann/goeasyerp/backend/business-logic/tables"
	fcodeunits "github.com/hansjlachmann/goeasyerp/backend/foundation/codeunits"
	"github.com/hansjlachmann/goeasyerp/backend/foundation/i18n"
	"github.com/hansjlachmann/goeasyerp/backend/foundation/system"
	"github.com/hansjlachmann/goeasyerp/backend/foundation/types"
	gtables "github.com/hansjlachmann/goeasyerp/backend/generated/tables"
)

// Report 5607 Fixed Asset - Projected Value: per fixed asset in a depreciation book the
// depreciation (and custom 1) codeunit 5610 calculates period by period from the First to the
// Last Depreciation Date — periods of Number of Days, or the accounting periods, never across the
// end of a fiscal year or the custom 1 ending date — with the book value after each; with Posted
// Entries From the asset's posted entries from that date first; with Projected Disposal an asset
// with a Projected Disposal Date in the period is depreciated up to it and its projected proceeds
// and gain/loss are shown; group totals by class, subclass, location, main asset, dimension 1 or
// 2; the totals; with Print Amounts per Date the projected amounts per date (and group) on a page
// of their own (the classic temporary table 5641 FA Buffer Projection as a Go map). Inactive,
// disposed, not acquired assets and those depreciated past the Last Depreciation Date are left
// out; budgeted assets are included (filter Budgeted Asset).
const FixedAssetProjectedValueID = 5607

func init() {
	Register(FixedAssetProjectedValueID, "fixed-asset-projected-value", func() Report { return &faProjectedValue{} })
}

// faBufferProjection: a line of FA Buffer Projection — the projected amounts of a date (and
// group)
type faBufferProjection struct {
	codeName      string
	faPostingDate types.Date
	entryNo       int
	depreciation  types.Decimal
	custom1       types.Decimal
}

type faProjectedValue struct {
	faBookValue // the book, the group totals and their texts (shared with 5605/5606)

	deprBook                   *gtables.DepreciationBook
	fa                         *gtables.FixedAsset
	faDeprBook                 *gtables.FADepreciationBook
	startingDate, endingDate   types.Date // of the current asset (a projected disposal ends it earlier)
	startingDate2, endingDate2 types.Date // the request page's dates
	includePostedFrom          types.Date
	periodLength               int
	useAccountingPeriod        bool
	daysInFirstPeriod          int
	printAmountsPerDate        bool
	projectedDisposal          bool
	doProjectedDisposal        bool
	daysInFiscalYear           int
	newFiscalYear              types.Date
	endFiscalYear              types.Date
	custom1DeprUntil           types.Date
	untilDate                  types.Date
	dateFromProjection         types.Date
	deprText, deprText2        string
	custom1Text                string
	deprCustom1Text            string

	entryAmounts        fadepr.EntryAmounts
	assetAmounts        [4]types.Decimal // depreciation, custom 1, projected proceeds, projected gain/loss
	groupAmounts        [4]types.Decimal
	totalAmounts        [4]types.Decimal
	groupBookValue      types.Decimal // TotalBookValue[1]
	totalBookValue      types.Decimal // TotalBookValue[2]
	deprAmount          types.Decimal
	custom1Amount       types.Decimal
	numberOfDays        int
	custom1NumberOfDays int
	postedBookValue     types.Decimal // the posted entries' running book value
	buffer              []*faBufferProjection
	bufferIndex         int
}

func (r *faProjectedValue) Definition() *Definition {
	return &Definition{
		OptionsCaption: "OPTIONS",
		Options: []Option{
			{Name: "depreciation_book", Type: "text", Required: true, Relation: &OptionRelation{Table: "Depreciation_Book", Columns: []string{"code", "description"}}},
			{Name: "first_depreciation_date", Type: "date", Required: true},
			{Name: "last_depreciation_date", Type: "date", Required: true},
			{Name: "number_of_days", Type: "int"},
			{Name: "no_of_days_in_first_period", Type: "int"},
			{Name: "posted_entries_from", Type: "date"},
			{Name: "group_totals", Type: "option", Default: "1",
				Options: []string{"GROUP_NONE", "GROUP_FA_CLASS", "GROUP_FA_SUBCLASS", "GROUP_FA_LOCATION", "GROUP_MAIN_ASSET", "GROUP_DIMENSION_1", "GROUP_DIMENSION_2"}},
			{Name: "print_per_fixed_asset", Type: "bool", Default: "false"},
			{Name: "projected_disposal", Type: "bool", Default: "false"},
			{Name: "print_amounts_per_date", Type: "bool", Default: "false"},
			{Name: "use_accounting_period", Type: "bool", Default: "false"},
		},
		OnInitReport: func(run *Run) error { return defaultDeprBook(run, "depreciation_book") },
		OnPreReport:  r.onPreReport,
		DataItems: []*DataItem{
			{
				Name:                "FixedAsset",
				Table:               gtables.FixedAssetTableName,
				Key:                 []string{"no"},
				RequestFilterFields: []string{"no", "fa_class_code", "fa_subclass_code", "budgeted_asset"},
				OnPreDataItem:       Trigger(r.faOnPreDataItem),
				OnAfterGetRecord:    Trigger(r.faOnAfterGetRecord),
				GroupFields:         []string{"group_head_line"},
				OnGroupFooter:       GroupTrigger(r.faOnGroupFooter),
				DataItems: []*DataItem{
					{
						Name:             "FALedgerEntry",
						Table:            gtables.FALedgerEntryTableName,
						Key:              []string{"fa_no", "depreciation_book_code", "fa_posting_date"},
						OnPreDataItem:    Trigger(r.faLedgEntryOnPreDataItem),
						OnAfterGetRecord: Trigger(r.faLedgEntryOnAfterGetRecord),
					},
					{
						Name:             "ProjectedDepreciation",
						Table:            IntegerTable,
						OnPreDataItem:    Trigger(r.projectedDeprOnPreDataItem),
						OnAfterGetRecord: Trigger(r.projectedDeprOnAfterGetRecord),
					},
				},
			},
			{
				Name:          "ProjectionTotal",
				Table:         IntegerTable,
				OnPreDataItem: Trigger(r.projectionTotalOnPreDataItem),
			},
			{
				Name:             "Buffer",
				Table:            IntegerTable,
				OnPreDataItem:    Trigger(r.bufferOnPreDataItem),
				OnAfterGetRecord: Trigger(r.bufferOnAfterGetRecord),
			},
		},
	}
}

// onPreReport (OnPreReport): the book, the dates, the period length, the texts of the posting
// types
func (r *faProjectedValue) onPreReport(run *Run) error {
	deprBook, err := faReportDeprBook(run, "depreciation_book")
	if err != nil {
		return err
	}
	r.deprBook = deprBook
	r.deprBookCode = deprBook.Code
	r.startingDate = run.OptionDate("first_depreciation_date")
	r.endingDate = run.OptionDate("last_depreciation_date")
	r.periodLength = run.OptionInt("number_of_days")
	r.daysInFirstPeriod = run.OptionInt("no_of_days_in_first_period")
	r.includePostedFrom = run.OptionDate("posted_entries_from")
	r.groupTotals = run.OptionInt("group_totals") - 1
	r.printDetails = run.OptionBool("print_per_fixed_asset")
	r.projectedDisposal = run.OptionBool("projected_disposal")
	r.printAmountsPerDate = run.OptionBool("print_amounts_per_date")
	// the request page's Use Accounting Period clears Number of Days
	r.useAccountingPeriod = run.OptionBool("use_accounting_period")
	if r.useAccountingPeriod {
		r.periodLength = 0
	}
	if r.periodLength < 0 || r.daysInFirstPeriod < 0 {
		return system.Error(run.Text("NUMBER_OF_DAYS_RANGE"), 360)
	}
	setReportHeadValues(run)
	r.makeGroupTotalText(run)
	err = r.validateDates(run)
	if err != nil {
		return err
	}
	if deprBook.No_of_days_in_fiscal_year > 0 {
		r.daysInFiscalYear = deprBook.No_of_days_in_fiscal_year
	} else {
		r.daysInFiscalYear = 360
	}
	if r.periodLength == 0 {
		r.periodLength = r.daysInFiscalYear
	}
	if r.periodLength <= 5 || r.periodLength > r.daysInFiscalYear {
		return system.Error(run.Text("NUMBER_OF_DAYS_RANGE"), r.daysInFiscalYear)
	}
	ts := i18n.GetInstance()
	language := fcodeunits.CurrentLanguage()
	r.deprText = ts.OptionCaption(gtables.FALedgerEntryTableName, "fa_posting_type", "depreciation", language)
	if deprBook.Use_custom_1_depreciation {
		r.deprText2 = r.deprText
		r.custom1Text = ts.OptionCaption(gtables.FALedgerEntryTableName, "fa_posting_type", "custom_1", language)
		r.deprCustom1Text = r.deprText + " + " + r.custom1Text
	}
	faDeprBook := gtables.NewFADepreciationBook(run)
	run.SetValue("main_head_line", run.Text("TITLE"))
	run.SetValue("print_details", r.printDetails)
	run.SetValue("details_and_groups", r.printDetails && r.groupTotals > 0)
	run.SetValue("use_custom_1", deprBook.Use_custom_1_depreciation)
	run.SetValue("custom_1_and_groups", deprBook.Use_custom_1_depreciation && r.groupTotals > 0)
	run.SetValue("projected_disposal", r.projectedDisposal)
	run.SetValue("depr_text", r.deprText)
	run.SetValue("depr_text_2", r.deprText2)
	run.SetValue("custom_1_text", r.custom1Text)
	run.SetValue("depr_custom_1_text", r.deprCustom1Text)
	run.SetValue("sales_price_fieldname", faDeprBook.GetFieldCaption("projected_proceeds_on_disposal", language))
	run.SetValue("gain_loss_fieldname", run.Text("PROJECTED_GAIN_LOSS"))
	if r.printDetails {
		fa := gtables.NewFixedAsset(run)
		run.SetValue("fa_no_caption", fa.GetFieldCaption("no", language))
		run.SetValue("fa_description_caption", fa.GetFieldCaption("description", language))
	}
	return nil
}

// validateDates (ValidateDates, codeunit 5626 ValidateDeprDates): both dates, in order, no closing
// dates; the request page's dates kept for each asset
func (r *faProjectedValue) validateDates(run *Run) error {
	if r.endingDate.IsZero() || r.startingDate.IsZero() {
		return system.Error(run.Text("SPECIFY_DEPR_DATES"))
	}
	if r.startingDate.After(r.endingDate) {
		return system.Error(run.Text("FIRST_LATER_THAN_LAST"))
	}
	if !system.NormalDate(r.startingDate).Equal(r.startingDate) || !system.NormalDate(r.endingDate).Equal(r.endingDate) {
		return system.Error(run.Text("NO_CLOSING_DATES"))
	}
	r.endingDate2 = r.endingDate
	r.startingDate2 = r.startingDate
	return nil
}

// faOnAfterGetRecord: the assets with the book that are not skipped; a projected disposal in the
// period ends the asset's projection on its date
func (r *faProjectedValue) faOnAfterGetRecord(fa *gtables.FixedAsset, run *Run) error {
	r.fa = fa
	r.faDeprBook = gtables.NewFADepreciationBook(run)
	if !r.faDeprBook.Get(map[string]interface{}{"fa_no": fa.No, "depreciation_book_code": r.deprBookCode}) {
		return Skip()
	}
	if r.skipRecord(fa) {
		return Skip()
	}
	r.startingDate = r.startingDate2
	r.endingDate = r.endingDate2
	r.doProjectedDisposal = false
	if r.projectedDisposal && !r.faDeprBook.Projected_disposal_date.IsZero() && !r.faDeprBook.Projected_disposal_date.After(r.endingDate) {
		r.endingDate = r.faDeprBook.Projected_disposal_date
		if r.startingDate.After(r.endingDate) {
			r.startingDate = r.endingDate
		}
		r.doProjectedDisposal = true
	}
	groupHeadLine := ""
	if r.groupTotals > 0 {
		groupHeadLine = r.makeGroupHeadLine(run, fa)
	}
	run.SetValue("group_head_line", groupHeadLine)
	return nil
}

// skipRecord (SkipRecord): inactive, not acquired (by the Last Depreciation Date), depreciated
// after it or disposed
func (r *faProjectedValue) skipRecord(fa *gtables.FixedAsset) bool {
	return fa.Inactive ||
		r.faDeprBook.Acquisition_date.IsZero() ||
		r.faDeprBook.Acquisition_date.After(r.endingDate) ||
		r.faDeprBook.Last_depreciation_date.After(r.endingDate) ||
		!r.faDeprBook.Disposal_date.IsZero()
}

// faOnGroupFooter: the group's amounts (the classic GroupFooter sections); the next group starts
// from zero (InitGroupTotals)
func (r *faProjectedValue) faOnGroupFooter(fa *gtables.FixedAsset, group string, run *Run) error {
	if r.groupTotals == 0 {
		return Skip()
	}
	run.SetValue("group_total_text", run.Text("GROUP_TOTAL", run.Value("group_head_line")))
	run.SetValue("grp_depr", r.groupAmounts[0])
	run.SetValue("grp_custom_1", r.groupAmounts[1])
	run.SetValue("grp_depr_custom_1", r.groupAmounts[0].Add(r.groupAmounts[1]))
	run.SetValue("grp_sales_price", r.groupAmounts[2])
	run.SetValue("grp_gain_loss", r.groupAmounts[3])
	run.SetValue("grp_book_value", r.groupBookValue)
	r.groupAmounts = [4]types.Decimal{}
	r.groupBookValue = types.Decimal{}
	return nil
}

// faLedgEntryOnPreDataItem: the asset's posted entries in the book, printed from Posted Entries
// From (per asset only)
func (r *faProjectedValue) faLedgEntryOnPreDataItem(rec *gtables.FALedgerEntry, run *Run) error {
	rec.SetRange("fa_no", r.fa.No)
	rec.SetRange("depreciation_book_code", r.deprBookCode)
	r.postedBookValue = types.Decimal{}
	if r.includePostedFrom.IsZero() || !r.printDetails {
		return Break()
	}
	return nil
}

// faLedgEntryOnAfterGetRecord: the running book value of the entries; those before Posted
// Entries From are not printed
func (r *faProjectedValue) faLedgEntryOnAfterGetRecord(rec *gtables.FALedgerEntry, run *Run) error {
	if rec.Part_of_book_value {
		r.postedBookValue = r.postedBookValue.Add(rec.Amount)
	}
	run.SetValue("posted_book_value", r.postedBookValue)
	if rec.Fa_posting_date.Before(r.includePostedFrom) {
		return Skip()
	}
	return nil
}

// projectedDeprOnPreDataItem: the asset's book value and amounts to start from (the classic body
// section's TransferValues), the periods 1, 2, …
func (r *faProjectedValue) projectedDeprOnPreDataItem(rec *Integer, run *Run) error {
	err := r.transferValues(run)
	if err != nil {
		return err
	}
	run.SetValue("fa_no", r.fa.No.String())
	run.SetValue("fa_description", r.fa.Description.String())
	rec.SetRange("number", 1, 1000000)
	return nil
}

// transferValues (TransferValues): the asset's book value, custom 1 and depreciation in the
// fiscal year as the entry amounts the calculation starts from; its fiscal year
func (r *faProjectedValue) transferValues(run *Run) error {
	r.faDeprBook.CalcFields("book_value", "depreciation", "custom_1")
	r.dateFromProjection = types.Date{}
	r.entryAmounts[0] = r.faDeprBook.Book_value
	r.entryAmounts[1] = r.faDeprBook.Custom_1
	deprInFiscalYear, err := fadepr.DeprInFiscalYear(run, r.fa.No, r.deprBookCode, r.startingDate)
	if err != nil {
		return err
	}
	r.entryAmounts[2] = deprInFiscalYear
	r.groupBookValue = r.groupBookValue.Add(r.faDeprBook.Book_value)
	r.totalBookValue = r.totalBookValue.Add(r.faDeprBook.Book_value)
	r.newFiscalYear, err = fadepr.GetFiscalYear(run, r.deprBookCode, r.startingDate)
	if err != nil {
		return err
	}
	r.endFiscalYear = tables.CalculateDate(fadepr.Yesterday(r.newFiscalYear), r.daysInFiscalYear)
	if r.deprBook.Use_custom_1_depreciation {
		r.custom1DeprUntil = r.faDeprBook.Depr_ending_date_custom_1
	} else {
		r.custom1DeprUntil = types.Date{}
	}
	if !r.custom1DeprUntil.IsZero() {
		r.entryAmounts[3] = r.getDeprBasis(run)
	}
	r.untilDate = types.Date{}
	r.assetAmounts = [4]types.Decimal{}
	return nil
}

// projectedDeprOnAfterGetRecord: the next period's depreciation until the Last Depreciation Date
// or until nothing is left to depreciate; when the projection ends, the asset's totals (and a
// projected disposal's gain/loss) for its footer
func (r *faProjectedValue) projectedDeprOnAfterGetRecord(rec *Integer, run *Run) error {
	if !r.untilDate.Before(r.endingDate) {
		return r.endAsset(run)
	}
	var done bool
	var err error
	if rec.Number == 1 {
		done, err = r.calculateFirstDeprAmount(run)
	} else {
		done, err = r.calculateSecondDeprAmount(run)
	}
	if err != nil {
		return err
	}
	if !done {
		return r.endAsset(run)
	}
	r.updateTotals()
	run.SetValue("until_date", r.untilDate)
	run.SetValue("number_of_days", r.numberOfDays)
	run.SetValue("depr_amount", r.deprAmount)
	run.SetValue("book_value_after_depr", r.entryAmounts[0].Sub(r.custom1Amount))
	run.SetValue("custom_1_number_of_days", r.custom1NumberOfDays)
	run.SetValue("custom_1_amount", r.custom1Amount)
	run.SetValue("book_value", r.entryAmounts[0])
	run.SetValue("show_custom_1_line", r.deprBook.Use_custom_1_depreciation && r.printDetails && !r.custom1Amount.IsZero())
	return nil
}

// endAsset: the projection of the asset ends (CurrReport.BREAK) — its footer's values
func (r *faProjectedValue) endAsset(run *Run) error {
	if r.doProjectedDisposal {
		err := r.calculateGainLoss(run)
		if err != nil {
			return err
		}
	}
	run.SetValue("asset_depr", r.assetAmounts[0])
	run.SetValue("asset_custom_1", r.assetAmounts[1])
	run.SetValue("asset_depr_custom_1", r.assetAmounts[0].Add(r.assetAmounts[1]))
	run.SetValue("asset_sales_price", r.assetAmounts[2])
	run.SetValue("asset_gain_loss", r.assetAmounts[3])
	run.SetValue("asset_book_value", r.entryAmounts[0])
	run.SetValue("show_asset_custom_1", r.deprBook.Use_custom_1_depreciation && r.printDetails && !r.assetAmounts[1].IsZero())
	run.SetValue("show_asset_disposal", r.doProjectedDisposal && r.printDetails)
	return Break()
}

// calculateFirstDeprAmount (CalculateFirstDeprAmount): from the First Depreciation Date, period
// by period until a period has depreciation
func (r *faProjectedValue) calculateFirstDeprAmount(run *Run) (bool, error) {
	done := false
	firstTime := true
	r.untilDate = r.startingDate
	for {
		if !firstTime {
			r.getNextDate(run)
		}
		firstTime = false
		result, err := fadepr.Calculate(run, r.fa.No, r.deprBookCode, r.untilDate, r.entryAmounts, types.Date{}, r.daysInFirstPeriod)
		if err != nil {
			return false, err
		}
		r.setResult(result)
		done = !r.deprAmount.IsZero() || !r.custom1Amount.IsZero()
		if !r.untilDate.Before(r.endingDate) || done {
			break
		}
	}
	deprInFiscalYear, err := fadepr.DeprInFiscalYear(run, r.fa.No, r.deprBookCode, r.untilDate)
	if err != nil {
		return false, err
	}
	r.entryAmounts[2] = deprInFiscalYear
	return done, nil
}

// calculateSecondDeprAmount (CalculateSecondDeprAmount): the next period, from the day after the
// last
func (r *faProjectedValue) calculateSecondDeprAmount(run *Run) (bool, error) {
	r.getNextDate(run)
	result, err := fadepr.Calculate(run, r.fa.No, r.deprBookCode, r.untilDate, r.entryAmounts, r.dateFromProjection, 0)
	if err != nil {
		return false, err
	}
	r.setResult(result)
	return r.calculationDone(run, !r.deprAmount.IsZero() || !r.custom1Amount.IsZero(), r.dateFromProjection)
}

func (r *faProjectedValue) setResult(result fadepr.Result) {
	r.deprAmount = result.DeprAmount
	r.custom1Amount = result.Custom1Amount
	r.numberOfDays = result.NumberOfDays
	r.custom1NumberOfDays = result.Custom1NumberOfDays
}

// getNextDate (GetNextDate): the end of the next period — never past the custom 1 ending date,
// the end of the fiscal year or the Last Depreciation Date; a new fiscal year starts its
// depreciation in the year at 0
func (r *faProjectedValue) getNextDate(run *Run) {
	untilDate2 := r.getPeriodEndingDate(run, r.untilDate)
	if !r.custom1DeprUntil.IsZero() {
		if r.untilDate.Before(r.custom1DeprUntil) && untilDate2.After(r.custom1DeprUntil) {
			untilDate2 = r.custom1DeprUntil
		}
	}
	if r.untilDate.Before(r.endFiscalYear) && untilDate2.After(r.endFiscalYear) {
		untilDate2 = r.endFiscalYear
	}
	if r.untilDate.Equal(r.endFiscalYear) {
		r.entryAmounts[2] = types.Decimal{}
		r.newFiscalYear = fadepr.ToMorrow(r.endFiscalYear)
		r.endFiscalYear = tables.CalculateDate(r.endFiscalYear, r.daysInFiscalYear)
	}
	r.dateFromProjection = fadepr.ToMorrow(r.untilDate)
	r.untilDate = untilDate2
	if !r.untilDate.Before(r.endingDate) {
		r.untilDate = r.endingDate
	}
}

// getPeriodEndingDate (GetPeriodEndingDate): Number of Days after the date, or the end of the
// next accounting period (its length the period length from now on, a period shorter than 6 days
// or longer than the fiscal year counting as the fiscal year)
func (r *faProjectedValue) getPeriodEndingDate(run *Run, periodEndingDate types.Date) types.Date {
	if !r.useAccountingPeriod {
		return tables.CalculateDate(periodEndingDate, r.periodLength)
	}
	accountingPeriod := gtables.NewAccountingPeriod(run)
	accountingPeriod.SetFilter("starting_date", ">="+fadepr.ToMorrow(periodEndingDate).AddDays(1).String())
	if accountingPeriod.FindFirst() {
		untilDate2 := fadepr.Yesterday(accountingPeriod.Starting_date)
		r.periodLength = fadepr.DeprDays(fadepr.ToMorrow(periodEndingDate), untilDate2)
		if r.periodLength <= 5 || r.periodLength > r.daysInFiscalYear {
			r.periodLength = r.daysInFiscalYear
		}
		return untilDate2
	}
	return tables.CalculateDate(periodEndingDate, r.periodLength)
}

// updateTotals (UpdateTotals): the period's amounts added to the asset's book value, the asset's,
// the group's and the totals; the date's line of the amounts per date
func (r *faProjectedValue) updateTotals() {
	r.entryAmounts[0] = r.entryAmounts[0].Add(r.deprAmount).Add(r.custom1Amount)
	if !r.custom1DeprUntil.IsZero() {
		if !r.untilDate.After(r.custom1DeprUntil) {
			r.entryAmounts[3] = r.entryAmounts[3].Add(r.deprAmount).Add(r.custom1Amount)
		}
	}
	r.entryAmounts[1] = r.entryAmounts[1].Add(r.custom1Amount)
	r.entryAmounts[2] = r.entryAmounts[2].Add(r.deprAmount).Add(r.custom1Amount)
	r.assetAmounts[0] = r.assetAmounts[0].Add(r.deprAmount)
	r.assetAmounts[1] = r.assetAmounts[1].Add(r.custom1Amount)
	r.groupAmounts[0] = r.groupAmounts[0].Add(r.deprAmount)
	r.groupAmounts[1] = r.groupAmounts[1].Add(r.custom1Amount)
	r.totalAmounts[0] = r.totalAmounts[0].Add(r.deprAmount)
	r.totalAmounts[1] = r.totalAmounts[1].Add(r.custom1Amount)
	r.groupBookValue = r.groupBookValue.Add(r.deprAmount).Add(r.custom1Amount)
	r.totalBookValue = r.totalBookValue.Add(r.deprAmount).Add(r.custom1Amount)

	if !r.untilDate.IsZero() || r.printAmountsPerDate {
		codeName := ""
		if r.groupTotals > 0 {
			codeName = r.groupCode(r.fa)
		}
		for _, line := range r.buffer {
			if line.faPostingDate.Equal(r.untilDate) && line.codeName == codeName {
				line.depreciation = line.depreciation.Add(r.deprAmount)
				line.custom1 = line.custom1.Add(r.custom1Amount)
				return
			}
		}
		r.buffer = append(r.buffer, &faBufferProjection{codeName: codeName, faPostingDate: r.untilDate, entryNo: len(r.buffer) + 1,
			depreciation: r.deprAmount, custom1: r.custom1Amount})
	}
}

// groupCode: the asset's value of the group field (FA Buffer Projection's Code Name)
func (r *faProjectedValue) groupCode(fa *gtables.FixedAsset) string {
	switch r.groupTotals {
	case 1:
		return fa.Fa_class_code.String()
	case 2:
		return fa.Fa_subclass_code.String()
	case 3:
		return fa.Fa_location_code.String()
	case 4:
		return fa.Component_of_main_asset.String()
	case 5:
		return fa.Dimension_1_code.String()
	case 6:
		return fa.Dimension_2_code.String()
	}
	return ""
}

// getDeprBasis (GetDeprBasis): the book value up to the custom 1 ending date
func (r *faProjectedValue) getDeprBasis(run *Run) types.Decimal {
	faLedgEntry := gtables.NewFALedgerEntry(run)
	faLedgEntry.SetCurrentKey("fa_no", "depreciation_book_code", "part_of_book_value", "fa_posting_date")
	faLedgEntry.SetRange("fa_no", r.fa.No)
	faLedgEntry.SetRange("depreciation_book_code", r.deprBookCode)
	faLedgEntry.SetRange("part_of_book_value", true)
	faLedgEntry.SetRange("fa_posting_date", types.Date{}, r.custom1DeprUntil)
	if faLedgEntry.CalcSums("amount") != nil {
		return types.Decimal{}
	}
	return faLedgEntry.Amount
}

// calculateGainLoss (CalculateGainLoss): the projected proceeds and the gain/loss of disposing of
// the asset at its book value after the projected depreciation (codeunit 5605)
func (r *faProjectedValue) calculateGainLoss(run *Run) error {
	entryAmounts, err := fajnl.CalcGainLoss(run, r.fa.No, r.deprBookCode)
	if err != nil {
		return err
	}
	r.assetAmounts[2] = r.faDeprBook.Projected_proceeds_on_disposal
	if !entryAmounts[0].IsZero() {
		r.assetAmounts[3] = entryAmounts[0]
	} else {
		r.assetAmounts[3] = entryAmounts[1]
	}
	r.assetAmounts[3] = r.assetAmounts[3].Add(r.assetAmounts[0]).Add(r.assetAmounts[1]).Sub(r.faDeprBook.Projected_proceeds_on_disposal)
	r.groupAmounts[2] = r.groupAmounts[2].Add(r.assetAmounts[2])
	r.groupAmounts[3] = r.groupAmounts[3].Add(r.assetAmounts[3])
	r.totalAmounts[2] = r.totalAmounts[2].Add(r.assetAmounts[2])
	r.totalAmounts[3] = r.totalAmounts[3].Add(r.assetAmounts[3])
	return nil
}

// calculationDone (CalculationDone): a period without depreciation ends the projection — not a
// User-Defined method's period whose table percentage is 0
func (r *faProjectedValue) calculationDone(run *Run, done bool, firstDeprDate types.Date) (bool, error) {
	if done || r.faDeprBook.Depreciation_method != gtables.FADepreciationBook_Depreciation_method.UserDefined {
		return done, nil
	}
	percent, err := fadepr.TablePercent(run, r.deprBookCode, r.faDeprBook.Depreciation_table_code,
		r.faDeprBook.First_user_defined_depr_date, firstDeprDate, r.untilDate)
	if err != nil {
		return false, err
	}
	return percent.IsZero(), nil
}

// projectionTotalOnPreDataItem: the totals (one line)
func (r *faProjectedValue) projectionTotalOnPreDataItem(rec *Integer, run *Run) error {
	rec.SetRange("number", 1, 1)
	run.SetValue("tot_depr", r.totalAmounts[0])
	run.SetValue("tot_custom_1", r.totalAmounts[1])
	run.SetValue("tot_depr_custom_1", r.totalAmounts[0].Add(r.totalAmounts[1]))
	run.SetValue("tot_sales_price", r.totalAmounts[2])
	run.SetValue("tot_gain_loss", r.totalAmounts[3])
	run.SetValue("tot_book_value", r.totalBookValue)
	return nil
}

// bufferOnPreDataItem: the projected amounts per date (Print Amounts per Date) on a new page, in
// the order of FA Buffer Projection's key (Code Name, FA Posting Date, Entry No.)
func (r *faProjectedValue) bufferOnPreDataItem(rec *Integer, run *Run) error {
	if !r.printAmountsPerDate || len(r.buffer) == 0 {
		return Break()
	}
	sort.SliceStable(r.buffer, func(i, j int) bool {
		a, b := r.buffer[i], r.buffer[j]
		if a.codeName != b.codeName {
			return a.codeName < b.codeName
		}
		if !a.faPostingDate.Equal(b.faPostingDate) {
			return a.faPostingDate.Before(b.faPostingDate)
		}
		return a.entryNo < b.entryNo
	})
	run.NewPage()
	language := fcodeunits.CurrentLanguage()
	groupCodeName2 := ""
	if r.groupTotals > 0 {
		groupCodeName2 = gtables.NewFixedAsset(run).GetFieldCaption(r.groupField(), language)
	}
	run.SetValue("group_code_name_2", groupCodeName2)
	rec.SetRange("number", 1, len(r.buffer))
	return nil
}

func (r *faProjectedValue) bufferOnAfterGetRecord(rec *Integer, run *Run) error {
	line := r.buffer[rec.Number-1]
	run.SetValue("buf_code_name", line.codeName)
	run.SetValue("buf_fa_posting_date", line.faPostingDate)
	run.SetValue("buf_depreciation", line.depreciation)
	run.SetValue("buf_custom_1", line.custom1)
	return nil
}

// groupField: the field of the group totals
func (r *faProjectedValue) groupField() string {
	return [...]string{"", "fa_class_code", "fa_subclass_code", "fa_location_code", "component_of_main_asset", "dimension_1_code", "dimension_2_code"}[r.groupTotals]
}
