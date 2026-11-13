package reports

import (
	"strings"

	fcodeunits "github.com/hansjlachmann/goeasyerp/backend/foundation/codeunits"
	"github.com/hansjlachmann/goeasyerp/backend/foundation/system"
	"github.com/hansjlachmann/goeasyerp/backend/foundation/types"
	gtables "github.com/hansjlachmann/goeasyerp/backend/generated/tables"
)

// Report 5600 Fixed Asset - Analysis: per fixed asset in a depreciation book two dates and three
// amounts the user picks — a date type's last date (Date Field 1/2) and a posting type's amount
// before the starting date, its net change in the period or at the ending date (Amount Field 1–3
// with Period 1–3; Proceeds on Disposal and Gain/Loss only at the ending date) — with group
// totals and the total; Only Sold Assets: the assets disposed in the period (a sold asset is
// marked Sold, and its amounts at the ending date are 0 in the other report); Budget Report adds
// codeunit 5615's depreciation to come to Depreciation and Book Value.
const FixedAssetAnalysisID = 5600

// Report 5610 Fixed Asset - G/L Analysis: the same for what the assets posted to the G/L between
// the starting and ending date (Posting Date) — the posting types' amounts of category blank,
// Disposal or Bal. Disposal (Period 1–3), the date types' last posting dates; assets without
// amounts are left out.
const FixedAssetGLAnalysisID = 5610

func init() {
	Register(FixedAssetAnalysisID, "fixed-asset-analysis", func() Report {
		return &faAnalysis{id: FixedAssetAnalysisID}
	})
	Register(FixedAssetGLAnalysisID, "fixed-asset-g-l-analysis", func() Report {
		return &faAnalysis{id: FixedAssetGLAnalysisID, glEntry: true}
	})
}

// faAnalysis: reports 5600 and 5610. The values set for the layouts: head_1/head_2 (the date
// types), head_3…head_5 (the amounts' types and periods), date_1/date_2, amount_1…amount_3 (the
// asset), grp_amount_n (the group total), tot_amount_n (the total), sales_mark (Sold)
type faAnalysis struct {
	faBookValue // the book, dates, group totals and their texts (shared with 5605/5606)

	id            int
	glEntry       bool // 5610: the G/L's amounts and dates
	salesReport   bool // Only Sold Assets
	postingTypeNo [3]int
	period        [3]int
	dateTypeNo    [2]int
	total         [3]types.Decimal
}

func (r *faAnalysis) Definition() *Definition {
	postingTypes := faChoiceKeys(faPostingTypeChoices(r.glEntry))
	dateTypes := faChoiceKeys(faDateTypeChoices(r.glEntry))
	periods := []string{"PERIOD_BEFORE_STARTING_DATE", "PERIOD_NET_CHANGE", "PERIOD_AT_ENDING_DATE"}
	if r.glEntry {
		periods = []string{"BLANK", "PERIOD_DISPOSAL", "PERIOD_BAL_DISPOSAL"}
	}
	options := []Option{
		{Name: "depreciation_book", Type: "text", Required: true, Relation: &OptionRelation{Table: "Depreciation_Book", Columns: []string{"code", "description"}}},
		{Name: "starting_date", Type: "date"},
		{Name: "ending_date", Type: "date"},
		{Name: "date_field_1", Type: "option", Default: "1", Options: dateTypes},
		{Name: "date_field_2", Type: "option", Default: "1", Options: dateTypes},
		{Name: "amount_field_1", Type: "option", Default: "1", Options: postingTypes},
		{Name: "period_1", Type: "option", Default: "1", Options: periods},
		{Name: "amount_field_2", Type: "option", Default: "1", Options: postingTypes},
		{Name: "period_2", Type: "option", Default: "1", Options: periods},
		{Name: "amount_field_3", Type: "option", Default: "1", Options: postingTypes},
		{Name: "period_3", Type: "option", Default: "1", Options: periods},
		{Name: "group_totals", Type: "option", Default: "1",
			Options: []string{"GROUP_NONE", "GROUP_FA_CLASS", "GROUP_FA_SUBCLASS", "GROUP_FA_LOCATION", "GROUP_MAIN_ASSET", "GROUP_DIMENSION_1", "GROUP_DIMENSION_2"}},
		{Name: "print_per_fixed_asset", Type: "bool", Default: "false"},
		{Name: "only_sold_assets", Type: "bool", Default: "false"},
	}
	if !r.glEntry {
		options = append(options, Option{Name: "budget_report", Type: "bool", Default: "false"})
	}
	return &Definition{
		OptionsCaption: "OPTIONS",
		Options:        options,
		OnInitReport:   func(run *Run) error { return defaultDeprBook(run, "depreciation_book") },
		OnPreReport:    r.onPreReport,
		DataItems: []*DataItem{{
			Name:                "FixedAsset",
			Table:               gtables.FixedAssetTableName,
			Key:                 []string{"no"},
			RequestFilterFields: []string{"no", "fa_class_code", "fa_subclass_code", "budgeted_asset"},
			OnPreDataItem:       Trigger(r.faOnPreDataItem),
			OnAfterGetRecord:    Trigger(r.faOnAfterGetRecord),
			GroupFields:         []string{"group_head_line"},
			TotalValues:         []string{"amount_1", "amount_2", "amount_3"},
			OnGroupFooter:       GroupTrigger(r.faGroupFooter),
		}},
	}
}

// onPreReport: the book, the options, the head (FA No./Description captions with Print per
// Fixed Asset)
func (r *faAnalysis) onPreReport(run *Run) error {
	deprBook, err := faReportDeprBook(run, "depreciation_book")
	if err != nil {
		return err
	}
	r.deprBookCode = deprBook.Code
	r.startingDate = run.OptionDate("starting_date")
	r.endingDate = run.OptionDate("ending_date")
	r.groupTotals = run.OptionInt("group_totals") - 1
	r.printDetails = run.OptionBool("print_per_fixed_asset")
	r.salesReport = run.OptionBool("only_sold_assets")
	r.budgetReport = !r.glEntry && run.OptionBool("budget_report")
	mainHeadLineText := run.Text("TITLE")
	if r.budgetReport {
		mainHeadLineText = run.Text("TITLE_BUDGET")
	}
	run.SetValue("main_head_line", mainHeadLineText)
	setReportHeadValues(run)
	run.SetValue("print_details", r.printDetails)
	run.SetValue("details_and_groups", r.printDetails && r.groupTotals > 0)
	return nil
}

// faOnPreDataItem: the order of the group totals, the types picked, the dates, the head lines
func (r *faAnalysis) faOnPreDataItem(rec *gtables.FixedAsset, run *Run) error {
	switch r.groupTotals {
	case 1:
		rec.SetCurrentKey("fa_class_code")
	case 2:
		rec.SetCurrentKey("fa_subclass_code")
	case 3:
		rec.SetCurrentKey("fa_location_code")
	case 4:
		rec.SetCurrentKey("component_of_main_asset")
	case 5:
		rec.SetCurrentKey("dimension_1_code")
	case 6:
		rec.SetCurrentKey("dimension_2_code")
	}
	setFilterText(run, rec)
	if r.glEntry {
		appendPostingDateFilter(run, rec.TableCaption(fcodeunits.CurrentLanguage()), r.startingDate, r.endingDate)
	} else {
		err := appendFAPostingFilter(run, rec, r.startingDate, r.endingDate)
		if err != nil {
			return err
		}
	}

	dateTypes := faDateTypeChoices(r.glEntry)
	postingTypes := faPostingTypeChoices(r.glEntry)
	var dateTypeNames [2]string
	var postingTypeNames [3]string
	for i := 0; i < 2; i++ {
		r.dateTypeNo[i], dateTypeNames[i] = faChoice(run, "date_field_"+itoa(i+1), dateTypes)
	}
	for i := 0; i < 3; i++ {
		r.postingTypeNo[i], postingTypeNames[i] = faChoice(run, "amount_field_"+itoa(i+1), postingTypes)
		r.period[i] = run.OptionInt("period_"+itoa(i+1)) - 1
		if r.period[i] < 0 {
			r.period[i] = 0
		}
	}
	r.makeGroupTotalText(run)
	err := faValidateDates(run, r.startingDate, r.endingDate)
	if err != nil {
		return err
	}
	r.makeDateHeadLine(run, dateTypeNames)
	for i := 0; i < 3; i++ {
		err = r.makeAmountHeadLine(run, i, postingTypeNames[i])
		if err != nil {
			return err
		}
	}
	return nil
}

// makeDateHeadLine (MakeDateHeadLine): the date types' names, with Print per Fixed Asset
func (r *faAnalysis) makeDateHeadLine(run *Run, dateTypeNames [2]string) {
	for i := 0; i < 2; i++ {
		head := ""
		if r.printDetails {
			head = dateTypeNames[i]
		}
		run.SetValue("head_"+itoa(i+1), head)
	}
}

// makeAmountHeadLine (MakeAmountHeadLine): "<posting type> <period>" — 5600: Proceeds on
// Disposal and Gain/Loss only at the ending date, before the starting date needs a starting
// date; 5610: Bal. Disposal only for write-down, appreciation, custom 1 and custom 2, Proceeds on
// Disposal and Gain/Loss in category blank, Book Value on Disposal in Disposal
func (r *faAnalysis) makeAmountHeadLine(run *Run, i int, postingType string) error {
	run.SetValue("head_"+itoa(i+3), "")
	postingTypeNo := r.postingTypeNo[i]
	if postingTypeNo == 0 {
		return nil
	}
	fa := gtables.NewFADepreciationBook(run)
	lang := fcodeunits.CurrentLanguage()
	caption := func(field string) string { return fa.GetFieldCaption(field, lang) }
	if r.glEntry {
		if r.period[i] == 2 {
			if postingTypeNo != faWriteDown && postingTypeNo != faAppreciation && postingTypeNo != faCustom1 && postingTypeNo != faCustom2 {
				return system.Error(run.Text("BAL_DISPOSAL_ONLY_WITH", run.Text("PERIOD_BAL_DISPOSAL"),
					caption("write_down"), caption("appreciation"), caption("custom_1"), caption("custom_2")))
			}
		}
		switch postingTypeNo {
		case faProceedsOnDisposal, faGainLoss:
			r.period[i] = 0
		case faBookValueOnDisposal:
			r.period[i] = 1
		}
		periods := []string{"BLANK", "PERIOD_DISPOSAL", "PERIOD_BAL_DISPOSAL"}
		run.SetValue("head_"+itoa(i+3), strings.TrimSpace(postingType+" "+run.Text(periods[r.period[i]])))
		return nil
	}
	periods := []string{"PERIOD_BEFORE_STARTING_DATE", "PERIOD_NET_CHANGE", "PERIOD_AT_ENDING_DATE"}
	switch postingTypeNo {
	case faProceedsOnDisposal, faGainLoss:
		if r.period[i] != faAtEndingDate {
			return system.Error(run.Text("ONLY_AT_ENDING_DATE", caption("proceeds_on_disposal"), caption("gain_loss"), run.Text("PERIOD_AT_ENDING_DATE")))
		}
	}
	if r.period[i] == faBeforeStartingDate {
		if r.startingDate.IsZero() {
			return system.Error(run.Text("STARTING_DATE_FOR_OPTION", run.Text(periods[r.period[i]])))
		}
	}
	run.SetValue("head_"+itoa(i+3), postingType+" "+run.Text(periods[r.period[i]]))
	return nil
}

// faOnAfterGetRecord: the asset's dates and amounts (assets without the book or skipped by
// SkipRecord are left out; 5610 also those without amounts) and its group
func (r *faAnalysis) faOnAfterGetRecord(fa *gtables.FixedAsset, run *Run) error {
	r.faDeprBook = gtables.NewFADepreciationBook(run)
	if !r.faDeprBook.Get(map[string]interface{}{"fa_no": fa.No, "depreciation_book_code": r.deprBookCode}) {
		return Skip()
	}
	if r.skipAnalysisRecord(fa) {
		return Skip()
	}

	var dates [2]types.Date
	for i := 0; i < 2; i++ {
		dates[i] = faGetLastDate(run, fa.No, r.dateTypeNo[i], r.endingDate, r.deprBookCode, r.glEntry)
	}
	var amounts [3]types.Decimal
	if r.glEntry {
		for i := 0; i < 3; i++ {
			amounts[i] = faCalcGLPostedAmount(run, fa.No, r.postingTypeNo[i], gtables.FALedgerEntryFa_posting_category(r.period[i]), r.startingDate, r.endingDate, r.deprBookCode)
		}
		if amounts[0].IsZero() && amounts[1].IsZero() && amounts[2].IsZero() {
			return Skip()
		}
	} else {
		r.beforeAmount = types.Decimal{}
		r.endingAmount = types.Decimal{}
		if r.budgetReport {
			var err error
			r.beforeAmount, r.endingAmount, err = r.budgetDepreciation.Calculate(run, fa.No, getStartingDate(r.startingDate), r.endingDate, r.deprBookCode)
			if err != nil {
				return err
			}
		}
		for i := 0; i < 3; i++ {
			if r.setAmountToZero(r.postingTypeNo[i], r.period[i]) {
				amounts[i] = types.Decimal{}
			} else {
				amounts[i] = faCalcFAPostedAmount(run, fa.No, r.postingTypeNo[i], r.period[i], r.startingDate, r.endingDate, r.deprBookCode, r.beforeAmount, r.endingAmount, false, false)
			}
		}
		run.SetValue("sales_mark", r.setSalesMark(run))
	}

	groupHeadLine := ""
	if r.groupTotals > 0 {
		groupHeadLine = r.makeGroupHeadLine(run, fa)
	}
	run.SetValue("group_head_line", groupHeadLine)
	for i := 0; i < 2; i++ {
		run.SetValue("date_"+itoa(i+1), dates[i])
	}
	for i := 0; i < 3; i++ {
		run.SetValue("amount_"+itoa(i+1), amounts[i])
		r.total[i] = r.total[i].Add(amounts[i])
		run.SetValue("tot_amount_"+itoa(i+1), r.total[i])
	}
	return nil
}

// skipAnalysisRecord (SkipRecord): inactive or not acquired; 5600: acquired after the ending
// date, disposed before the starting date, with Only Sold Assets not disposed in the period;
// 5610: with Only Sold Assets not disposed
func (r *faAnalysis) skipAnalysisRecord(fa *gtables.FixedAsset) bool {
	r.acquisitionDate = r.faDeprBook.Acquisition_date
	r.disposalDate = r.faDeprBook.Disposal_date
	if r.glEntry {
		return fa.Inactive || r.acquisitionDate.IsZero() || r.salesReport && r.disposalDate.IsZero()
	}
	if fa.Inactive {
		return true
	}
	if r.acquisitionDate.IsZero() {
		return true
	}
	if r.acquisitionDate.After(r.endingDate) && !r.endingDate.IsZero() {
		return true
	}
	if r.salesReport && r.disposalDate.IsZero() {
		return true
	}
	if r.salesReport && !r.endingDate.IsZero() &&
		(r.disposalDate.After(r.endingDate) || r.disposalDate.Before(r.startingDate)) {
		return true
	}
	if !r.salesReport && !r.disposalDate.IsZero() && r.disposalDate.Before(r.startingDate) {
		return true
	}
	return false
}

// setSalesMark (SetSalesMark): Sold when disposed by the ending date
func (r *faAnalysis) setSalesMark(run *Run) string {
	if !r.disposalDate.IsZero() {
		if r.endingDate.IsZero() || !r.disposalDate.After(r.endingDate) {
			return run.Text("SOLD")
		}
	}
	return ""
}

// setAmountToZero (SetAmountToZero): a sold asset's amounts at the ending date are 0 unless
// Only Sold Assets (not Proceeds on Disposal and Gain/Loss)
func (r *faAnalysis) setAmountToZero(postingTypeNo, period int) bool {
	switch postingTypeNo {
	case faProceedsOnDisposal, faGainLoss:
		return false
	}
	if !r.salesReport && period == faAtEndingDate && !r.disposalDate.IsZero() &&
		(r.endingDate.IsZero() || !r.disposalDate.After(r.endingDate)) {
		return true
	}
	return false
}

// faGroupFooter (the group footer): the group's amounts and text; none without group totals
func (r *faAnalysis) faGroupFooter(fa *gtables.FixedAsset, group string, run *Run) error {
	if r.groupTotals == 0 {
		return Skip()
	}
	for i := 1; i <= 3; i++ {
		run.SetValue("grp_amount_"+itoa(i), run.GroupTotal("amount_"+itoa(i)))
	}
	run.SetValue("group_total_text", run.Text("GROUP_TOTAL", run.Value("group_head_line")))
	return nil
}
