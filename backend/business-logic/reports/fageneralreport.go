package reports

import (
	fcodeunits "github.com/hansjlachmann/goeasyerp/backend/foundation/codeunits"
	"github.com/hansjlachmann/goeasyerp/backend/foundation/system"
	"github.com/hansjlachmann/goeasyerp/backend/foundation/types"
	gtables "github.com/hansjlachmann/goeasyerp/backend/generated/tables"
)

// Codeunit 5626 FA General Report: what the fixed asset analysis reports (5600, 5610, 5630, 5635)
// share — the last date of a posting type, an asset's posted amount of a posting type (FA ledger
// or G/L) in a period, the date filters in the head and the check of the dates.
//
// A posting type or date type is the field number of the FA Depreciation Book field it stands for
// (classic FIELDNO). The classic tables 5644 FA Posting Type and 5645 FA Date Type only list those
// numbers with the fields' names for the request page; here they are the choices of the request
// page's options (faPostingTypeChoices, faDateTypeChoices), translated as the fields' captions.

// the FA Depreciation Book field numbers the posting and date types stand for
const (
	faAcquisitionCost         = 15
	faDepreciation            = 16
	faBookValueType           = 17
	faProceedsOnDisposal      = 18
	faGainLoss                = 19
	faWriteDown               = 20
	faAppreciation            = 21
	faCustom1                 = 22
	faCustom2                 = 23
	faDepreciableBasis        = 24
	faSalvageValue            = 25
	faBookValueOnDisposal     = 26
	faAcquisitionDate         = 30
	faGLAcquisitionDate       = 31
	faDisposalDate            = 32
	faLastAcquisitionCostDate = 33
	faLastDepreciationDate    = 34
	faLastWriteDownDate       = 35
	faLastAppreciationDate    = 36
	faLastCustom1Date         = 37
	faLastCustom2Date         = 38
	faLastSalvageValueDate    = 39
)

// faTypeChoice: a choice of a posting or date type option — its text key (reports.shared.texts)
// and the field number
type faTypeChoice struct {
	key     string
	fieldNo int
}

// faPostingTypeChoices (CreateTypes of table 5644): the posting types of FA entries (FA Entry,
// glEntry false: reports 5600) or of G/L entries (G/L Entry: report 5610), blank first
func faPostingTypeChoices(glEntry bool) []faTypeChoice {
	choices := []faTypeChoice{
		{"FA_TYPE_NONE", 0},
		{"FA_ACQUISITION_COST", faAcquisitionCost},
		{"FA_DEPRECIATION", faDepreciation},
		{"FA_WRITE_DOWN", faWriteDown},
		{"FA_APPRECIATION", faAppreciation},
		{"FA_CUSTOM_1", faCustom1},
		{"FA_CUSTOM_2", faCustom2},
		{"FA_PROCEEDS_ON_DISPOSAL", faProceedsOnDisposal},
		{"FA_GAIN_LOSS", faGainLoss},
	}
	if glEntry {
		return append(choices, faTypeChoice{"FA_BOOK_VALUE_ON_DISPOSAL", faBookValueOnDisposal})
	}
	return append(choices,
		faTypeChoice{"FA_BOOK_VALUE", faBookValueType},
		faTypeChoice{"FA_DEPRECIABLE_BASIS", faDepreciableBasis},
		faTypeChoice{"FA_SALVAGE_VALUE", faSalvageValue})
}

// faDateTypeChoices (CreateTypes of table 5645): the date types of FA entries (Acquisition Date)
// or of G/L entries (G/L Acquisition Date), blank first
func faDateTypeChoices(glEntry bool) []faTypeChoice {
	choices := []faTypeChoice{
		{"FA_TYPE_NONE", 0},
		{"FA_LAST_ACQUISITION_COST_DATE", faLastAcquisitionCostDate},
		{"FA_LAST_DEPRECIATION_DATE", faLastDepreciationDate},
		{"FA_LAST_WRITE_DOWN_DATE", faLastWriteDownDate},
		{"FA_LAST_APPRECIATION_DATE", faLastAppreciationDate},
		{"FA_LAST_CUSTOM_1_DATE", faLastCustom1Date},
		{"FA_LAST_CUSTOM_2_DATE", faLastCustom2Date},
		{"FA_DISPOSAL_DATE", faDisposalDate},
		{"FA_LAST_SALVAGE_VALUE_DATE", faLastSalvageValueDate},
	}
	if glEntry {
		return append(choices, faTypeChoice{"FA_G_L_ACQUISITION_DATE", faGLAcquisitionDate})
	}
	return append(choices, faTypeChoice{"FA_ACQUISITION_DATE", faAcquisitionDate})
}

// faChoiceKeys: the text keys of the choices (Option.Options)
func faChoiceKeys(choices []faTypeChoice) []string {
	keys := make([]string, len(choices))
	for i, choice := range choices {
		keys[i] = choice.key
	}
	return keys
}

// faChoice (CheckPostingType, CheckDateType): the type the option picked — its field number and
// name (blank: 0)
func faChoice(run *Run, option string, choices []faTypeChoice) (int, string) {
	i := run.OptionInt(option) - 1
	if i <= 0 || i >= len(choices) {
		return 0, ""
	}
	return choices[i].fieldNo, run.Text(choices[i].key)
}

// faPostingTypeFilter: the FA ledger entries' posting type of a posting type number (ok false: a
// type that is not one posting type — Book Value, Depreciable Basis)
func faPostingTypeFilter(postingType int) (gtables.FALedgerEntryFa_posting_type, bool) {
	t := gtables.FALedgerEntry_Fa_posting_type
	switch postingType {
	case faAcquisitionCost, faLastAcquisitionCostDate, faAcquisitionDate, faGLAcquisitionDate:
		return t.AcquisitionCost, true
	case faDepreciation, faLastDepreciationDate:
		return t.Depreciation, true
	case faWriteDown, faLastWriteDownDate:
		return t.WriteDown, true
	case faAppreciation, faLastAppreciationDate:
		return t.Appreciation, true
	case faCustom1, faLastCustom1Date:
		return t.Custom1, true
	case faCustom2, faLastCustom2Date:
		return t.Custom2, true
	case faProceedsOnDisposal, faDisposalDate:
		return t.ProceedsOnDisposal, true
	case faGainLoss:
		return t.GainLoss, true
	case faSalvageValue, faLastSalvageValueDate:
		return t.SalvageValue, true
	case faBookValueOnDisposal:
		return t.BookValueOnDisposal, true
	}
	return 0, false
}

// faEndOfTime: a blank ending date means no end (classic 12319999D)
func faEndOfTime(endingDate types.Date) types.Date {
	if endingDate.IsZero() {
		return types.NewDate(9999, 12, 31)
	}
	return endingDate
}

// faGetLastDate (GetLastDate): the asset's last date of a date type in the book up to the ending
// date — the first for the acquisition and disposal dates —; the posting date of the G/L
// (glEntry, category blank), else the FA posting date
func faGetLastDate(src *Run, faNo types.Code, postingType int, endingDate types.Date, deprBookCode types.Code, glEntry bool) types.Date {
	if postingType == 0 {
		return types.Date{}
	}
	endingDate = faEndOfTime(endingDate)
	faLedgEntry := gtables.NewFALedgerEntry(src)
	if glEntry {
		faLedgEntry.SetCurrentKey("fa_no", "depreciation_book_code", "fa_posting_category", "fa_posting_type", "posting_date")
		faLedgEntry.SetRange("depreciation_book_code", deprBookCode)
		faLedgEntry.SetRange("fa_no", faNo)
		faLedgEntry.SetRange("fa_posting_category", gtables.FALedgerEntry_Fa_posting_category.Blank)
		faLedgEntry.SetRange("posting_date", types.Date{}, endingDate)
	} else {
		// DepreciationCalc.SetFAFilter(FALedgEntry, FANo, DeprBookCode, TRUE)
		faLedgEntry.SetCurrentKey("fa_no", "depreciation_book_code", "fa_posting_category", "fa_posting_type", "fa_posting_date")
		faLedgEntry.SetRange("fa_no", faNo)
		faLedgEntry.SetRange("depreciation_book_code", deprBookCode)
		faLedgEntry.SetRange("fa_posting_category", gtables.FALedgerEntry_Fa_posting_category.Blank)
		faLedgEntry.SetRange("fa_posting_date", types.Date{}, endingDate)
	}
	firstLast := "+"
	postingTypeFilter, ok := faPostingTypeFilter(postingType)
	if ok {
		faLedgEntry.SetRange("fa_posting_type", postingTypeFilter)
	}
	switch postingType {
	case faAcquisitionDate, faGLAcquisitionDate, faDisposalDate:
		firstLast = "-"
	}
	if faLedgEntry.Find(firstLast) {
		if glEntry {
			return faLedgEntry.Posting_date
		}
		return faLedgEntry.Fa_posting_date
	}
	return types.Date{}
}

// faPeriod: the period of an amount — before the starting date, the net change, at the ending
// date (CalcFAPostedAmount's Period)
const (
	faBeforeStartingDate = 0
	faNetChange          = 1
	faAtEndingDate       = 2
)

// faCalcFAPostedAmount (CalcFAPostedAmount): the asset's amount of a posting type in the book
// for the period — Book Value / Depreciable Basis: the entries that are part of it; the budget's
// depreciation before the starting date (beforeAmount) and up to the ending date (untilAmount)
// added to Depreciation and Book Value
func faCalcFAPostedAmount(src *Run, faNo types.Code, postingType, period int, startingDate, endingDate types.Date, deprBookCode types.Code, beforeAmount, untilAmount types.Decimal, onlyReclassified, onlyBookValue bool) types.Decimal {
	if postingType == 0 {
		return types.Decimal{}
	}
	endingDate = faEndOfTime(endingDate)
	faLedgEntry := gtables.NewFALedgerEntry(src)
	switch postingType {
	case faBookValueType:
		faLedgEntry.SetCurrentKey("fa_no", "depreciation_book_code", "part_of_book_value")
	case faDepreciableBasis:
		faLedgEntry.SetCurrentKey("fa_no", "depreciation_book_code", "part_of_depreciable_basis")
	default:
		faLedgEntry.SetCurrentKey("fa_no", "depreciation_book_code", "fa_posting_category", "fa_posting_type", "fa_posting_date")
		faLedgEntry.SetRange("fa_posting_category", gtables.FALedgerEntry_Fa_posting_category.Blank)
	}
	faLedgEntry.SetRange("fa_no", faNo)
	faLedgEntry.SetRange("depreciation_book_code", deprBookCode)
	if onlyReclassified {
		faLedgEntry.SetRange("reclassification_entry", true)
	}
	if onlyBookValue {
		faLedgEntry.SetRange("part_of_book_value", true)
	}
	switch postingType {
	case faBookValueType:
		faLedgEntry.SetRange("part_of_book_value", true)
	case faDepreciableBasis:
		faLedgEntry.SetRange("part_of_depreciable_basis", true)
	default:
		postingTypeFilter, ok := faPostingTypeFilter(postingType)
		if ok {
			faLedgEntry.SetRange("fa_posting_type", postingTypeFilter)
		}
	}
	switch period {
	case faBeforeStartingDate:
		faLedgEntry.SetRange("fa_posting_date", types.Date{}, startingDate.AddDays(-1))
	case faNetChange:
		faLedgEntry.SetRange("fa_posting_date", startingDate, endingDate)
	case faAtEndingDate:
		faLedgEntry.SetRange("fa_posting_date", types.Date{}, endingDate)
	}
	if faLedgEntry.CalcSums("amount") != nil {
		return types.Decimal{}
	}
	amount := faLedgEntry.Amount
	if postingType == faBookValueType || postingType == faDepreciation {
		switch period {
		case faBeforeStartingDate:
			amount = amount.Add(beforeAmount)
		case faNetChange:
			amount = amount.Sub(beforeAmount).Add(untilAmount)
		case faAtEndingDate:
			amount = amount.Add(untilAmount)
		}
	}
	return amount
}

// faCalcGLPostedAmount (CalcGLPostedAmount): the asset's G/L amount of a posting type in the
// book posted (Posting Date) between the dates in a posting category — blank, Disposal or Bal.
// Disposal
func faCalcGLPostedAmount(src *Run, faNo types.Code, postingType int, category gtables.FALedgerEntryFa_posting_category, startingDate, endingDate types.Date, deprBookCode types.Code) types.Decimal {
	if postingType == 0 {
		return types.Decimal{}
	}
	endingDate = faEndOfTime(endingDate)
	faLedgEntry := gtables.NewFALedgerEntry(src)
	faLedgEntry.SetCurrentKey("fa_no", "depreciation_book_code", "fa_posting_category", "fa_posting_type", "posting_date")
	faLedgEntry.SetRange("fa_no", faNo)
	faLedgEntry.SetRange("depreciation_book_code", deprBookCode)
	faLedgEntry.SetRange("fa_posting_category", category)
	faLedgEntry.SetRange("posting_date", startingDate, endingDate)
	postingTypeFilter, ok := faPostingTypeFilter(postingType)
	if ok {
		faLedgEntry.SetRange("fa_posting_type", postingTypeFilter)
	}
	if faLedgEntry.CalcSums("amount") != nil {
		return types.Decimal{}
	}
	return faLedgEntry.Amount
}

// faDateRangeText: "a..b" (an open end blank) in the user's date format
func faDateRangeText(startingDate, endingDate types.Date) string {
	lang := fcodeunits.CurrentLanguage()
	text := ""
	if !startingDate.IsZero() {
		text = formatDate(startingDate, lang)
	}
	text += ".."
	if !endingDate.IsZero() {
		text += formatDate(endingDate, lang)
	}
	return text
}

// appendFAPostingFilter (AppendFAPostingFilter): the dates as the asset's FA Posting Date Filter
// — added to the filters in the head (filter_text)
func appendFAPostingFilter(run *Run, fa *gtables.FixedAsset, startingDate, endingDate types.Date) error {
	if startingDate.IsZero() && endingDate.IsZero() {
		return nil
	}
	expr := ""
	if !startingDate.IsZero() {
		expr = startingDate.String()
	}
	expr += ".."
	if !endingDate.IsZero() {
		expr += endingDate.String()
	}
	err := fa.SetFlowFilter("fa_posting_date_filter", expr)
	if err != nil {
		return err
	}
	appendFilterText(run, fa.TableCaption(fcodeunits.CurrentLanguage()),
		fa.GetFieldCaption("fa_posting_date_filter", fcodeunits.CurrentLanguage())+": "+faDateRangeText(startingDate, endingDate))
	return nil
}

// appendPostingDateFilter (AppendPostingDateFilter): "Posting Date Filter: a..b" added to the
// filters in the head
func appendPostingDateFilter(run *Run, tableCaption string, startingDate, endingDate types.Date) {
	appendFilterText(run, tableCaption, run.Text("POSTING_DATE_FILTER", faDateRangeText(startingDate, endingDate)))
}

// appendFilterText: a filter added to filter_text (setFilterText first), the table caption
// first when there was none
func appendFilterText(run *Run, tableCaption, filter string) {
	filterText, _ := run.Value("filter_text").(string)
	if filterText == "" {
		run.SetValue("filter_text", tableCaption+": "+filter)
		return
	}
	run.SetValue("filter_text", filterText+", "+filter)
}

// faValidateDates (ValidateDates): both dates, in order, no closing dates
func faValidateDates(run *Run, startingDate, endingDate types.Date) error {
	if endingDate.IsZero() || startingDate.IsZero() {
		return system.Error(run.Text("SPECIFY_DATES"))
	}
	if startingDate.After(endingDate) {
		return system.Error(run.Text("STARTING_DATE_LATER"))
	}
	if system.NormalDate(startingDate) != startingDate || system.NormalDate(endingDate) != endingDate {
		return system.Error(run.Text("NO_CLOSING_DATES"))
	}
	return nil
}
