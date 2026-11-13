package reports

import (
	"strings"

	fcodeunits "github.com/hansjlachmann/goeasyerp/backend/foundation/codeunits"
	"github.com/hansjlachmann/goeasyerp/backend/foundation/system"
	"github.com/hansjlachmann/goeasyerp/backend/foundation/types"
	gtables "github.com/hansjlachmann/goeasyerp/backend/generated/tables"
)

// The maintenance printouts (main menu Assets & Depreciations → Reports → Maintenance):
//   - 5630 Maintenance - Analysis: per fixed asset in a depreciation book three maintenance
//     amounts — a maintenance code's (blank: all) amount before the starting date, its net change
//     or at the ending date, by FA Posting Date or Posting Date (Date Selection) — with group
//     totals and the total; assets without amounts are left out.
//   - 5633 Maintenance Register: the FA registers in the filters with the maintenance ledger
//     entries they wrote (landscape), the register's and the grand total.
//   - 5634 Maintenance - Details: per fixed asset its maintenance ledger entries in a book (FA
//     Posting Date Filter: those in the dates); New Page per FA; assets without entries are not
//     printed.
//   - 5635 Maintenance - Next Service: the fixed assets (not budgeted, not inactive) whose Next
//     Service Date is between the starting and ending date.
const (
	MaintenanceAnalysisID    = 5630
	MaintenanceRegisterID    = 5633
	MaintenanceDetailsID     = 5634
	MaintenanceNextServiceID = 5635
)

func init() {
	Register(MaintenanceAnalysisID, "maintenance-analysis", func() Report { return &maintenanceAnalysis{} })
	Register(MaintenanceRegisterID, "maintenance-register", func() Report { return &maintenanceRegister{} })
	Register(MaintenanceDetailsID, "maintenance-details", func() Report { return &maintenanceDetails{} })
	Register(MaintenanceNextServiceID, "maintenance-next-service", func() Report { return &maintenanceNextService{} })
}

// --- 5630 Maintenance - Analysis ---

// maintenanceAnalysis: the values set for the layout — head_1…head_3 (code and period),
// amount_1…amount_3, grp_amount_n, tot_amount_n
type maintenanceAnalysis struct {
	faBookValue // the book, dates and group totals (shared with 5605/5606)

	maintenanceCode [3]types.Code
	period          [3]int
	dateSelection   int // 0 FA Posting Date, 1 Posting Date
	total           [3]types.Decimal
}

func (r *maintenanceAnalysis) Definition() *Definition {
	periods := []string{"PERIOD_BEFORE_STARTING_DATE", "PERIOD_NET_CHANGE", "PERIOD_AT_ENDING_DATE"}
	maintenance := &OptionRelation{Table: "Maintenance", Columns: []string{"code", "description"}}
	return &Definition{
		OptionsCaption: "OPTIONS",
		Options: []Option{
			{Name: "depreciation_book", Type: "text", Required: true, Relation: &OptionRelation{Table: "Depreciation_Book", Columns: []string{"code", "description"}}},
			{Name: "starting_date", Type: "date"},
			{Name: "ending_date", Type: "date"},
			{Name: "date_selection", Type: "option", Default: "1", Options: []string{"FA_POSTING_DATE", "POSTING_DATE"}},
			{Name: "amount_field_1", Type: "text", Relation: maintenance},
			{Name: "period_1", Type: "option", Default: "1", Options: periods},
			{Name: "amount_field_2", Type: "text", Relation: maintenance},
			{Name: "period_2", Type: "option", Default: "1", Options: periods},
			{Name: "amount_field_3", Type: "text", Relation: maintenance},
			{Name: "period_3", Type: "option", Default: "1", Options: periods},
			{Name: "group_totals", Type: "option", Default: "1",
				Options: []string{"GROUP_NONE", "GROUP_FA_CLASS", "GROUP_FA_SUBCLASS", "GROUP_FA_LOCATION", "GROUP_MAIN_ASSET", "GROUP_DIMENSION_1", "GROUP_DIMENSION_2"}},
			{Name: "print_per_fixed_asset", Type: "bool", Default: "false"},
		},
		OnInitReport: func(run *Run) error { return defaultDeprBook(run, "depreciation_book") },
		OnPreReport:  r.onPreReport,
		DataItems: []*DataItem{{
			Name:                "FixedAsset",
			Table:               gtables.FixedAssetTableName,
			Key:                 []string{"no"},
			RequestFilterFields: []string{"no", "fa_class_code", "fa_subclass_code"},
			OnPreDataItem:       Trigger(r.faOnPreDataItem),
			OnAfterGetRecord:    Trigger(r.faOnAfterGetRecord),
			GroupFields:         []string{"group_head_line"},
			TotalValues:         []string{"amount_1", "amount_2", "amount_3"},
			OnGroupFooter:       GroupTrigger(r.faGroupFooter),
		}},
	}
}

// onPreReport: the book, the options, the dates (ValidateDates) and the head lines
func (r *maintenanceAnalysis) onPreReport(run *Run) error {
	deprBook, err := faReportDeprBook(run, "depreciation_book")
	if err != nil {
		return err
	}
	r.deprBookCode = deprBook.Code
	r.startingDate = run.OptionDate("starting_date")
	r.endingDate = run.OptionDate("ending_date")
	r.dateSelection = run.OptionInt("date_selection") - 1
	r.groupTotals = run.OptionInt("group_totals") - 1
	r.printDetails = run.OptionBool("print_per_fixed_asset")
	for i := 0; i < 3; i++ {
		r.maintenanceCode[i] = types.Code(system.UpperCase(strings.TrimSpace(run.Option("amount_field_" + itoa(i+1)))))
		if r.maintenanceCode[i] != "" {
			maintenance := gtables.NewMaintenance(run)
			err = maintenance.GetOrError(r.maintenanceCode[i])
			if err != nil {
				return err
			}
		}
		r.period[i] = run.OptionInt("period_"+itoa(i+1)) - 1
		if r.period[i] < 0 {
			r.period[i] = 0
		}
	}
	r.makeGroupTotalText(run)
	err = r.validateMaintenanceDates(run)
	if err != nil {
		return err
	}
	for i := 0; i < 3; i++ {
		err = r.makeAmountHeadLine(run, i)
		if err != nil {
			return err
		}
	}
	setReportHeadValues(run)
	run.SetValue("main_head_line", run.Text("TITLE"))
	run.SetValue("print_details", r.printDetails)
	run.SetValue("details_and_groups", r.printDetails && r.groupTotals > 0)
	return nil
}

// validateMaintenanceDates (ValidateDates): both dates, in order
func (r *maintenanceAnalysis) validateMaintenanceDates(run *Run) error {
	if r.endingDate.IsZero() || r.startingDate.IsZero() {
		return system.Error(run.Text("SPECIFY_DATES"))
	}
	if r.startingDate.After(r.endingDate) {
		return system.Error(run.Text("STARTING_DATE_LATER"))
	}
	return nil
}

// makeAmountHeadLine (MakeAmountHeadLine): "<maintenance code> <period>" (the period alone for
// all codes); before the starting date needs a starting date
func (r *maintenanceAnalysis) makeAmountHeadLine(run *Run, i int) error {
	periods := []string{"PERIOD_BEFORE_STARTING_DATE", "PERIOD_NET_CHANGE", "PERIOD_AT_ENDING_DATE"}
	if r.period[i] == faBeforeStartingDate {
		if r.startingDate.IsZero() {
			return system.Error(run.Text("STARTING_DATE_FOR_OPTION", run.Text(periods[r.period[i]])))
		}
	}
	if r.maintenanceCode[i] != "" {
		run.SetValue("head_"+itoa(i+1), r.maintenanceCode[i].String()+" "+run.Text(periods[r.period[i]]))
	} else {
		run.SetValue("head_"+itoa(i+1), run.Text(periods[r.period[i]]))
	}
	return nil
}

// faOnPreDataItem: the order of the group totals and the dates in the head (FA Posting Date
// Filter or Posting Date Filter)
func (r *maintenanceAnalysis) faOnPreDataItem(rec *gtables.FixedAsset, run *Run) error {
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
	if r.dateSelection == 0 {
		return appendFAPostingFilter(run, rec, r.startingDate, r.endingDate)
	}
	appendPostingDateFilter(run, rec.TableCaption(fcodeunits.CurrentLanguage()), r.startingDate, r.endingDate)
	return nil
}

// faOnAfterGetRecord: the asset's three amounts (inactive assets, those without the book or
// without amounts are left out) and its group
func (r *maintenanceAnalysis) faOnAfterGetRecord(fa *gtables.FixedAsset, run *Run) error {
	if fa.Inactive {
		return Skip()
	}
	faDeprBook := gtables.NewFADepreciationBook(run)
	if !faDeprBook.Get(map[string]interface{}{"fa_no": fa.No, "depreciation_book_code": r.deprBookCode}) {
		return Skip()
	}
	var amounts [3]types.Decimal
	for i := 0; i < 3; i++ {
		amount, err := r.calculateAmount(run, fa.No, r.maintenanceCode[i], r.period[i])
		if err != nil {
			return err
		}
		amounts[i] = amount
	}
	if amounts[0].IsZero() && amounts[1].IsZero() && amounts[2].IsZero() {
		return Skip()
	}
	groupHeadLine := ""
	if r.groupTotals > 0 {
		groupHeadLine = r.makeGroupHeadLine(run, fa)
	}
	run.SetValue("group_head_line", groupHeadLine)
	for i := 0; i < 3; i++ {
		run.SetValue("amount_"+itoa(i+1), amounts[i])
		r.total[i] = r.total[i].Add(amounts[i])
		run.SetValue("tot_amount_"+itoa(i+1), r.total[i])
	}
	return nil
}

// calculateAmount (CalculateAmount): the asset's maintenance in the book of the code (blank: all)
// before the starting date, in the period or up to the ending date, by the date selected
func (r *maintenanceAnalysis) calculateAmount(run *Run, faNo, maintenanceCode types.Code, period int) (types.Decimal, error) {
	endingDate2 := faEndOfTime(r.endingDate)
	maintenanceLedgEntry := gtables.NewMaintenanceLedgerEntry(run)
	dateField := "fa_posting_date"
	if r.dateSelection == 1 {
		dateField = "posting_date"
	}
	maintenanceLedgEntry.SetCurrentKey("fa_no", "depreciation_book_code", "maintenance_code", dateField)
	maintenanceLedgEntry.SetRange("depreciation_book_code", r.deprBookCode)
	maintenanceLedgEntry.SetRange("fa_no", faNo)
	switch period {
	case faBeforeStartingDate:
		maintenanceLedgEntry.SetRange(dateField, types.Date{}, r.startingDate.AddDays(-1))
	case faNetChange:
		maintenanceLedgEntry.SetRange(dateField, r.startingDate, endingDate2)
	case faAtEndingDate:
		maintenanceLedgEntry.SetRange(dateField, types.Date{}, endingDate2)
	}
	if maintenanceCode != "" {
		maintenanceLedgEntry.SetRange("maintenance_code", maintenanceCode)
	}
	err := maintenanceLedgEntry.CalcSums("amount")
	if err != nil {
		return types.Decimal{}, err
	}
	return maintenanceLedgEntry.Amount, nil
}

// faGroupFooter (the group footer): the group's amounts and text; none without group totals
func (r *maintenanceAnalysis) faGroupFooter(fa *gtables.FixedAsset, group string, run *Run) error {
	if r.groupTotals == 0 {
		return Skip()
	}
	for i := 1; i <= 3; i++ {
		run.SetValue("grp_amount_"+itoa(i), run.GroupTotal("amount_"+itoa(i)))
	}
	run.SetValue("group_total_text", run.Text("GROUP_TOTAL", run.Value("group_head_line")))
	return nil
}

// --- 5633 Maintenance Register ---

type maintenanceRegister struct {
	faRegister *gtables.FARegister
	total      types.Decimal
	regTotal   types.Decimal
}

func (r *maintenanceRegister) Definition() *Definition {
	return &Definition{
		OnPreReport: func(run *Run) error {
			setReportHeadValues(run)
			return nil
		},
		DataItems: []*DataItem{{
			Name:                "FARegister",
			Table:               gtables.FARegisterTableName,
			Key:                 []string{"no"},
			RequestFilterFields: []string{"no"},
			OnPreDataItem:       Trigger(r.faRegisterOnPreDataItem),
			OnAfterGetRecord:    Trigger(r.faRegisterOnAfterGetRecord),
			DataItems: []*DataItem{{
				Name:             "MaintenanceLedgerEntry",
				Table:            gtables.MaintenanceLedgerEntryTableName,
				Key:              []string{"entry_no"},
				OnPreDataItem:    Trigger(r.maintenanceLedgEntryOnPreDataItem),
				OnAfterGetRecord: Trigger(r.maintenanceLedgEntryOnAfterGetRecord),
			}},
		}},
	}
}

// faRegisterOnPreDataItem: the registers' filters in the head (MaintenanceRegFilter)
func (r *maintenanceRegister) faRegisterOnPreDataItem(rec *gtables.FARegister, run *Run) error {
	setFilterText(run, rec)
	return nil
}

// faRegisterOnAfterGetRecord: registers with maintenance entries (PrintOnlyIfDetail)
func (r *maintenanceRegister) faRegisterOnAfterGetRecord(rec *gtables.FARegister, run *Run) error {
	if rec.From_maintenance_entry_no == 0 && rec.To_maintenance_entry_no == 0 {
		return Skip()
	}
	r.faRegister = rec
	r.regTotal = types.Decimal{}
	return nil
}

// maintenanceLedgEntryOnPreDataItem: the register's maintenance entries
func (r *maintenanceRegister) maintenanceLedgEntryOnPreDataItem(rec *gtables.MaintenanceLedgerEntry, run *Run) error {
	rec.SetRange("entry_no", r.faRegister.From_maintenance_entry_no, r.faRegister.To_maintenance_entry_no)
	return nil
}

// maintenanceLedgEntryOnAfterGetRecord: the asset's description; the register's and the total
// (CREATETOTALS)
func (r *maintenanceRegister) maintenanceLedgEntryOnAfterGetRecord(rec *gtables.MaintenanceLedgerEntry, run *Run) error {
	fa := gtables.NewFixedAsset(run)
	if !fa.Get(rec.Fa_no) {
		fa.Init()
	}
	run.SetValue("fa_description", fa.Description.String())
	r.regTotal = r.regTotal.Add(rec.Amount)
	r.total = r.total.Add(rec.Amount)
	run.SetValue("reg_total", r.regTotal)
	run.SetValue("total", r.total)
	return nil
}

// --- 5634 Maintenance - Details ---

type maintenanceDetails struct {
	deprBookCode        types.Code
	printOnlyOnePerPage bool
	fa                  *gtables.FixedAsset
	printedFixedAssets  int
}

func (r *maintenanceDetails) Definition() *Definition {
	return &Definition{
		OptionsCaption: "OPTIONS",
		Options: []Option{
			{Name: "depreciation_book", Type: "text", Required: true, Relation: &OptionRelation{Table: "Depreciation_Book", Columns: []string{"code", "description"}}},
			{Name: "new_page_per_fa", Type: "bool", Default: "false"},
		},
		OnInitReport: func(run *Run) error { return defaultDeprBook(run, "depreciation_book") },
		OnPreReport:  r.onPreReport,
		DataItems: []*DataItem{{
			Name:                "FixedAsset",
			Table:               gtables.FixedAssetTableName,
			Key:                 []string{"no"},
			RequestFilterFields: []string{"no", "fa_class_code", "fa_subclass_code"},
			OnPreDataItem:       Trigger(faFilterOnPreDataItem),
			OnAfterGetRecord:    Trigger(r.faOnAfterGetRecord),
			DataItems: []*DataItem{{
				Name:          "MaintenanceLedgerEntry",
				Table:         gtables.MaintenanceLedgerEntryTableName,
				Key:           []string{"fa_no", "depreciation_book_code", "fa_posting_date"},
				OnPreDataItem: Trigger(r.maintenanceLedgEntryOnPreDataItem),
			}},
		}},
	}
}

func (r *maintenanceDetails) onPreReport(run *Run) error {
	deprBook, err := faReportDeprBook(run, "depreciation_book")
	if err != nil {
		return err
	}
	r.deprBookCode = deprBook.Code
	r.printOnlyOnePerPage = run.OptionBool("new_page_per_fa")
	setReportHeadValues(run)
	return nil
}

// faOnAfterGetRecord: not inactive assets with maintenance entries in the book
// (PrintOnlyIfDetail); New Page per FA
func (r *maintenanceDetails) faOnAfterGetRecord(rec *gtables.FixedAsset, run *Run) error {
	if rec.Inactive {
		return Skip()
	}
	r.fa = rec
	maintenanceLedgEntry := gtables.NewMaintenanceLedgerEntry(run)
	r.setEntryFilters(maintenanceLedgEntry)
	if maintenanceLedgEntry.IsEmpty() {
		return Skip()
	}
	if r.printOnlyOnePerPage && r.printedFixedAssets > 0 {
		run.NewPage()
	}
	r.printedFixedAssets++
	return nil
}

// maintenanceLedgEntryOnPreDataItem: the asset's entries in the book in its FA Posting Date
// Filter
func (r *maintenanceDetails) maintenanceLedgEntryOnPreDataItem(rec *gtables.MaintenanceLedgerEntry, run *Run) error {
	r.setEntryFilters(rec)
	return nil
}

func (r *maintenanceDetails) setEntryFilters(maintenanceLedgEntry *gtables.MaintenanceLedgerEntry) {
	maintenanceLedgEntry.SetCurrentKey("fa_no", "depreciation_book_code", "fa_posting_date")
	maintenanceLedgEntry.SetRange("fa_no", r.fa.No)
	maintenanceLedgEntry.SetRange("depreciation_book_code", r.deprBookCode)
	if r.fa.Fa_posting_date_filter != "" {
		maintenanceLedgEntry.SetFilter("fa_posting_date", r.fa.Fa_posting_date_filter)
	}
}

// --- 5635 Maintenance - Next Service ---

type maintenanceNextService struct {
	startingDate, endingDate types.Date
}

func (r *maintenanceNextService) Definition() *Definition {
	return &Definition{
		OptionsCaption: "OPTIONS",
		Options: []Option{
			{Name: "starting_date", Type: "date"},
			{Name: "ending_date", Type: "date"},
		},
		OnPreReport: r.onPreReport,
		DataItems: []*DataItem{{
			Name:                "FixedAsset",
			Table:               gtables.FixedAssetTableName,
			Key:                 []string{"no"},
			RequestFilterFields: []string{"no", "fa_class_code", "fa_subclass_code"},
			OnPreDataItem:       Trigger(r.faOnPreDataItem),
			OnAfterGetRecord:    Trigger(r.faOnAfterGetRecord),
		}},
	}
}

func (r *maintenanceNextService) onPreReport(run *Run) error {
	r.startingDate = run.OptionDate("starting_date")
	r.endingDate = run.OptionDate("ending_date")
	if !r.endingDate.IsZero() && r.startingDate.After(r.endingDate) {
		return system.Error(run.Text("STARTING_DATE_LATER"))
	}
	setReportHeadValues(run)
	return nil
}

// faOnPreDataItem: the filters and the dates (FA Posting Date Filter) in the head
func (r *maintenanceNextService) faOnPreDataItem(rec *gtables.FixedAsset, run *Run) error {
	setFilterText(run, rec)
	return appendFAPostingFilter(run, rec, r.startingDate, r.endingDate)
}

// faOnAfterGetRecord: assets that are not budgeted or inactive with a Next Service Date in the
// dates
func (r *maintenanceNextService) faOnAfterGetRecord(rec *gtables.FixedAsset, run *Run) error {
	if rec.Budgeted_asset || rec.Inactive || rec.Next_service_date.IsZero() {
		return Skip()
	}
	if !r.startingDate.IsZero() || !r.endingDate.IsZero() {
		if !r.startingDate.IsZero() && rec.Next_service_date.Before(r.startingDate) {
			return Skip()
		}
		if !r.endingDate.IsZero() && rec.Next_service_date.After(r.endingDate) {
			return Skip()
		}
	}
	return nil
}
