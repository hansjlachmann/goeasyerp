package reports

import (
	fcodeunits "github.com/hansjlachmann/openerp/backend/foundation/codeunits"
	"github.com/hansjlachmann/openerp/backend/foundation/system"
	"github.com/hansjlachmann/openerp/backend/foundation/types"
	gtables "github.com/hansjlachmann/openerp/backend/generated/tables"
)

// Reports 5605 Fixed Asset - Book Value 01 and 5606 Fixed Asset - Book Value 02 share the
// calculation (faBookValue): per fixed asset in a depreciation book its acquisition cost,
// depreciation, write-down, appreciation, custom 1 and custom 2 before the starting date, their
// net change in the period and — disposed in the period — their disposal, the totals at the ending
// date and the book value at both dates (codeunit 5626 FA General Report's CalcFAPostedAmount);
// group totals by class, subclass, location, main asset, dimension 1 or 2; Print per Fixed
// Asset (PrintDetails). Budget reports are left out (budgeted assets are not migrated).
//
// The groups are the data item's group sections, grouped by the value group_head_line (the
// asset's class, subclass … — the option the user picks; blank without group totals): the group
// footer (OnGroupFooter) sets the group's amounts from the group totals of the asset's amounts
// (TotalValues). The values set for the layouts, per prefix ("" the asset, "grp_" the group
// total, "tot_" the total): <p>start_<n>, <p>net_<n>, <p>disp_<n>, <p>end_<n> for the types 1–6
// (and <p>rstart_<n> … with Include Reclassification), <p>show_<n> (the line is printed and the
// type has an amount), <p>bv_start, <p>bv_end.

const faBookValueTypes = 6

// faBookValueAmounts: the amounts of one line (an asset, a group, the total)
type faBookValueAmounts struct {
	start, netChange, disposal                      [faBookValueTypes]types.Decimal
	reclassStart, reclassNetChange, reclassDisposal [faBookValueTypes]types.Decimal
}

func (a *faBookValueAmounts) add(b *faBookValueAmounts) {
	for j := 0; j < faBookValueTypes; j++ {
		a.start[j] = a.start[j].Add(b.start[j])
		a.netChange[j] = a.netChange[j].Add(b.netChange[j])
		a.disposal[j] = a.disposal[j].Add(b.disposal[j])
		a.reclassStart[j] = a.reclassStart[j].Add(b.reclassStart[j])
		a.reclassNetChange[j] = a.reclassNetChange[j].Add(b.reclassNetChange[j])
		a.reclassDisposal[j] = a.reclassDisposal[j].Add(b.reclassDisposal[j])
	}
}

type faBookValue struct {
	addPostingTypes          bool // 5605: write-down … custom 2 count as acquisition or depreciation (FA Posting Type Setup)
	reclassify               bool // 5606: Include Reclassification
	deprBookCode             types.Code
	startingDate, endingDate types.Date
	groupTotals              int // 0 none, 1 FA Class, 2 FA Subclass, 3 FA Location, 4 Main Asset, 5 Dimension 1, 6 Dimension 2
	printDetails             bool
	faDeprBook               *gtables.FADepreciationBook
	acquisitionDate          types.Date
	disposalDate             types.Date
	total                    faBookValueAmounts
}

// faBookValueTotalValues: the asset's amounts its groups total (DataItem.TotalValues)
func faBookValueTotalValues() []string {
	var names []string
	for j := 1; j <= faBookValueTypes; j++ {
		n := itoa(j)
		names = append(names, "start_"+n, "net_"+n, "disp_"+n, "rstart_"+n, "rnet_"+n, "rdisp_"+n)
	}
	return names
}

// faGroupFooter (the classic group footer): the group's amounts from the totals of its assets'
// amounts, its text; no group footer without group totals
func (r *faBookValue) faGroupFooter(fa *gtables.FixedAsset, group string, run *Run) error {
	if r.groupTotals == 0 {
		return Skip()
	}
	var a faBookValueAmounts
	for j := 1; j <= faBookValueTypes; j++ {
		n := itoa(j)
		a.start[j-1] = run.GroupTotal("start_" + n)
		a.netChange[j-1] = run.GroupTotal("net_" + n)
		a.disposal[j-1] = run.GroupTotal("disp_" + n)
		a.reclassStart[j-1] = run.GroupTotal("rstart_" + n)
		a.reclassNetChange[j-1] = run.GroupTotal("rnet_" + n)
		a.reclassDisposal[j-1] = run.GroupTotal("rdisp_" + n)
	}
	r.setAmountValues(run, "grp_", &a, true)
	run.SetValue("group_total_text", run.Text("GROUP_TOTAL", run.Value("group_head_line")))
	return nil
}

// faBookValueOptions: the request page of reports 5605 and 5606
func faBookValueOptions() []Option {
	return []Option{
		{Name: "depreciation_book", Type: "text", Required: true, Relation: &OptionRelation{Table: "Depreciation_Book", Columns: []string{"code", "description"}}},
		{Name: "starting_date", Type: "date", Required: true},
		{Name: "ending_date", Type: "date", Required: true},
		{Name: "group_totals", Type: "option", Default: "1",
			Options: []string{"GROUP_NONE", "GROUP_FA_CLASS", "GROUP_FA_SUBCLASS", "GROUP_FA_LOCATION", "GROUP_MAIN_ASSET", "GROUP_DIMENSION_1", "GROUP_DIMENSION_2"}},
		{Name: "print_per_fixed_asset", Type: "bool", Default: "false"},
	}
}

// onPreReport: the book, the dates (ValidateDates), the head and group texts
func (r *faBookValue) onPreReport(run *Run) error {
	deprBook, err := faReportDeprBook(run, "depreciation_book")
	if err != nil {
		return err
	}
	r.deprBookCode = deprBook.Code
	r.startingDate = run.OptionDate("starting_date")
	r.endingDate = run.OptionDate("ending_date")
	r.groupTotals = run.OptionInt("group_totals") - 1
	r.printDetails = run.OptionBool("print_per_fixed_asset")
	err = r.validateDates(run)
	if err != nil {
		return err
	}
	setReportHeadValues(run)
	run.SetValue("print_details", r.printDetails)
	run.SetValue("group_totals", r.groupTotals > 0)
	run.SetValue("details_and_groups", r.printDetails && r.groupTotals > 0)
	run.SetValue("print_column_heads", r.groupTotals == 0 || !r.printDetails)
	lang := fcodeunits.CurrentLanguage()
	run.SetValue("start_text", formatDate(r.startingDate.AddDays(-1), lang))
	run.SetValue("end_text", formatDate(r.endingDate, lang))
	r.makeGroupTotalText(run)
	return nil
}

// validateDates (codeunit 5626 ValidateDates): both dates, in order
func (r *faBookValue) validateDates(run *Run) error {
	if r.endingDate.IsZero() || r.startingDate.IsZero() {
		return system.Error(run.Text("SPECIFY_DATES"))
	}
	if r.startingDate.After(r.endingDate) {
		return system.Error(run.Text("STARTING_DATE_LATER"))
	}
	return nil
}

// faOnPreDataItem: the order of the group totals (SETCURRENTKEY) and the filters in the head
func (r *faBookValue) faOnPreDataItem(rec *gtables.FixedAsset, run *Run) error {
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
	return nil
}

// faOnAfterGetRecord: the asset's amounts (assets without the book, inactive, not acquired by the
// ending date or disposed before the starting date are skipped) and its group
func (r *faBookValue) faOnAfterGetRecord(fa *gtables.FixedAsset, run *Run) error {
	r.faDeprBook = gtables.NewFADepreciationBook(run)
	if !r.faDeprBook.Get(map[string]interface{}{"fa_no": fa.No, "depreciation_book_code": r.deprBookCode}) {
		return Skip()
	}
	if r.skipRecord(fa) {
		return Skip()
	}
	var amounts faBookValueAmounts
	for i := 1; i <= faBookValueTypes; i++ {
		postingType := gtables.FALedgerEntryFa_posting_type(i - 1)
		if !r.startingDate.IsZero() {
			amounts.start[i-1] = r.calcFAPostedAmount(run, fa.No, postingType, 0, false)
			if r.reclassify {
				amounts.reclassStart[i-1] = r.calcFAPostedAmount(run, fa.No, postingType, 0, true)
			}
		}
		amounts.netChange[i-1] = r.calcFAPostedAmount(run, fa.No, postingType, 1, false)
		if r.reclassify {
			amounts.reclassNetChange[i-1] = r.calcFAPostedAmount(run, fa.No, postingType, 1, true)
		}
		if r.getPeriodDisposal() {
			amounts.disposal[i-1] = amounts.start[i-1].Add(amounts.netChange[i-1]).Neg()
			amounts.reclassDisposal[i-1] = amounts.reclassStart[i-1].Add(amounts.reclassNetChange[i-1]).Neg()
		}
		if r.addPostingTypes && i >= 3 {
			err := r.addPostingType(run, &amounts, i)
			if err != nil {
				return err
			}
		}
	}

	// the asset's group (the group sections' group field): blank without group totals
	groupHeadLine := ""
	if r.groupTotals > 0 {
		groupHeadLine = r.makeGroupHeadLine(run, fa)
	}
	run.SetValue("group_head_line", groupHeadLine)

	r.setAmountValues(run, "", &amounts, r.printDetails)
	r.total.add(&amounts)
	r.setAmountValues(run, "tot_", &r.total, true)
	return nil
}

// calcFAPostedAmount (codeunit 5626 CalcFAPostedAmount): the asset's amount of a posting type in
// the book — period 0 before the starting date, 1 the net change in the period — that is part of
// the book value (category blank), only the reclassification entries when onlyReclassified
func (r *faBookValue) calcFAPostedAmount(run *Run, faNo types.Code, postingType gtables.FALedgerEntryFa_posting_type, period int, onlyReclassified bool) types.Decimal {
	faLedgEntry := gtables.NewFALedgerEntry(run)
	faLedgEntry.SetCurrentKey("fa_no", "depreciation_book_code", "fa_posting_category", "fa_posting_type", "fa_posting_date")
	faLedgEntry.SetRange("fa_posting_category", gtables.FALedgerEntry_Fa_posting_category.Blank)
	faLedgEntry.SetRange("fa_no", faNo)
	faLedgEntry.SetRange("depreciation_book_code", r.deprBookCode)
	if onlyReclassified {
		faLedgEntry.SetRange("reclassification_entry", true)
	}
	faLedgEntry.SetRange("part_of_book_value", true)
	faLedgEntry.SetRange("fa_posting_type", postingType)
	switch period {
	case 0:
		faLedgEntry.SetRange("fa_posting_date", types.Date{}, r.startingDate.AddDays(-1))
	case 1:
		faLedgEntry.SetRange("fa_posting_date", r.startingDate, r.endingDate)
	}
	if faLedgEntry.CalcSums("amount") != nil {
		return types.Decimal{}
	}
	return faLedgEntry.Amount
}

// addPostingType (AddPostingType, report 5605): a write-down, appreciation or custom amount
// counts as depreciation or acquisition cost when its FA Posting Type Setup says so
func (r *faBookValue) addPostingType(run *Run, amounts *faBookValueAmounts, i int) error {
	faPostingTypeSetup := gtables.NewFAPostingTypeSetup(run)
	err := faPostingTypeSetup.GetOrError(map[string]interface{}{"depreciation_book_code": r.deprBookCode, "fa_posting_type": gtables.FAPostingTypeSetupFa_posting_type(i - 3)})
	if err != nil {
		return err
	}
	j := 0
	if faPostingTypeSetup.Depreciation_type {
		j = 2
	} else {
		if faPostingTypeSetup.Acquisition_type {
			j = 1
		}
	}
	if j > 0 {
		amounts.start[j-1] = amounts.start[j-1].Add(amounts.start[i-1])
		amounts.start[i-1] = types.Decimal{}
		amounts.netChange[j-1] = amounts.netChange[j-1].Add(amounts.netChange[i-1])
		amounts.netChange[i-1] = types.Decimal{}
		amounts.disposal[j-1] = amounts.disposal[j-1].Add(amounts.disposal[i-1])
		amounts.disposal[i-1] = types.Decimal{}
	}
	return nil
}

// skipRecord (SkipRecord): inactive, not acquired (by the ending date) or disposed before the
// starting date
func (r *faBookValue) skipRecord(fa *gtables.FixedAsset) bool {
	r.acquisitionDate = r.faDeprBook.Acquisition_date
	r.disposalDate = r.faDeprBook.Disposal_date
	return fa.Inactive ||
		r.acquisitionDate.IsZero() ||
		r.acquisitionDate.After(r.endingDate) && !r.endingDate.IsZero() ||
		!r.disposalDate.IsZero() && r.disposalDate.Before(r.startingDate)
}

// getPeriodDisposal (GetPeriodDisposal): disposed by the ending date
func (r *faBookValue) getPeriodDisposal() bool {
	if !r.disposalDate.IsZero() {
		if r.endingDate.IsZero() || !r.disposalDate.After(r.endingDate) {
			return true
		}
	}
	return false
}

// makeGroupTotalText (MakeGroupTotalText): "Group Totals: <field>" in the head
func (r *faBookValue) makeGroupTotalText(run *Run) {
	fa := gtables.NewFixedAsset(run)
	lang := fcodeunits.CurrentLanguage()
	field := ""
	switch r.groupTotals {
	case 1:
		field = "fa_class_code"
	case 2:
		field = "fa_subclass_code"
	case 3:
		field = "fa_location_code"
	case 4:
		field = "main_asset_component"
	case 5:
		field = "dimension_1_code"
	case 6:
		field = "dimension_2_code"
	}
	groupCodeName := ""
	if field != "" {
		groupCodeName = run.Text("GROUP_TOTALS", fa.GetFieldCaption(field, lang))
	}
	run.SetValue("group_code_name", groupCodeName)
}

// makeGroupHeadLine (MakeGroupHeadLine): the asset's group — its class, subclass, location, main
// asset, dimension 1 or 2 (***** when blank)
func (r *faBookValue) makeGroupHeadLine(run *Run, fa *gtables.FixedAsset) string {
	groupHeadLine := ""
	switch r.groupTotals {
	case 1:
		groupHeadLine = fa.Fa_class_code.String()
	case 2:
		groupHeadLine = fa.Fa_subclass_code.String()
	case 3:
		groupHeadLine = fa.Fa_location_code.String()
	case 4:
		groupHeadLine = run.Text("MAIN_ASSET", fa.Component_of_main_asset.String())
		if fa.Component_of_main_asset == "" {
			groupHeadLine = groupHeadLine + " *****"
		}
	case 5:
		groupHeadLine = fa.Dimension_1_code.String()
	case 6:
		groupHeadLine = fa.Dimension_2_code.String()
	}
	if groupHeadLine == "" {
		groupHeadLine = "*****"
	}
	return groupHeadLine
}

// setAmountValues (CreateGroupTotals, CreateTotals): a line's amounts as the layout's values
func (r *faBookValue) setAmountValues(run *Run, prefix string, a *faBookValueAmounts, visible bool) {
	var bookValueAtStartingDate, bookValueAtEndingDate types.Decimal
	for j := 1; j <= faBookValueTypes; j++ {
		totalEnding := a.start[j-1].Add(a.netChange[j-1]).Add(a.disposal[j-1])
		reclassTotalEnding := a.reclassStart[j-1].Add(a.reclassNetChange[j-1]).Add(a.reclassDisposal[j-1])
		n := itoa(j)
		run.SetValue(prefix+"start_"+n, a.start[j-1])
		run.SetValue(prefix+"net_"+n, a.netChange[j-1])
		run.SetValue(prefix+"disp_"+n, a.disposal[j-1])
		run.SetValue(prefix+"end_"+n, totalEnding)
		run.SetValue(prefix+"rstart_"+n, a.reclassStart[j-1])
		run.SetValue(prefix+"rnet_"+n, a.reclassNetChange[j-1])
		run.SetValue(prefix+"rdisp_"+n, a.reclassDisposal[j-1])
		run.SetValue(prefix+"rend_"+n, reclassTotalEnding)
		show := !a.start[j-1].IsZero() || !a.netChange[j-1].IsZero() || !a.disposal[j-1].IsZero() || !totalEnding.IsZero() ||
			!a.reclassStart[j-1].IsZero() || !a.reclassNetChange[j-1].IsZero() || !a.reclassDisposal[j-1].IsZero() || !reclassTotalEnding.IsZero()
		run.SetValue(prefix+"show_"+n, visible && show)
		bookValueAtEndingDate = bookValueAtEndingDate.Add(totalEnding)
		bookValueAtStartingDate = bookValueAtStartingDate.Add(a.start[j-1])
	}
	run.SetValue(prefix+"bv_start", bookValueAtStartingDate)
	run.SetValue(prefix+"bv_end", bookValueAtEndingDate)
}
