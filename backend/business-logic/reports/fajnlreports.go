package reports

import (
	fcodeunits "github.com/hansjlachmann/goeasyerp/backend/foundation/codeunits"
	"github.com/hansjlachmann/goeasyerp/backend/foundation/system"
	"github.com/hansjlachmann/goeasyerp/backend/foundation/types"
	gtables "github.com/hansjlachmann/goeasyerp/backend/generated/tables"
)

// The fixed asset and insurance journals' reports:
//   - 5602 Fixed Asset Journal - Test (Test Report on the FA journals): per batch its lines with
//     what posting would refuse under each line — the FA posting date (closing date, the user's
//     allowed FA posting dates), document no., depreciation book, the asset (missing, blocked,
//     inactive) and its depreciation book, a posting type the book posts to the G/L, the fields
//     an FA Error Entry No. or the posting type does not allow, insurance integration, the
//     book's Use Same FA+G/L Posting Dates — and the batch's total;
//   - 5622 Insurance Journal - Test: per batch its lines with the posting date, the asset and
//     the document no. checked;
//   - 5623 Insurance Register (the insurance journal's posting report): per register its
//     coverage ledger entries with their total.
//
// Budgeted assets are not part of this application (their checks are left out).
const (
	FAJnlTestID          = 5602
	InsuranceJnlTestID   = 5622
	InsuranceRegisterID  = 5623
	faJnlTestName        = "fixed-asset-journal-test"
	insuranceJnlTestName = "insurance-journal-test"
	insuranceRegisterKey = "insurance-register"
)

func init() {
	Register(FAJnlTestID, faJnlTestName, func() Report { return &faJnlTest{} })
	Register(InsuranceJnlTestID, insuranceJnlTestName, func() Report { return &insuranceJnlTest{} })
	Register(InsuranceRegisterID, insuranceRegisterKey, func() Report { return &insuranceRegister{} })
}

// Report 5602 Fixed Asset Journal - Test
type faJnlTest struct {
	jnlTest
	batch                                *gtables.FAJournalBatch
	template                             *gtables.FAJournalTemplate
	allowFAPostingFrom, allowFAPostingTo types.Date
}

func (r *faJnlTest) Definition() *Definition {
	return &Definition{
		ContextTable: gtables.FAJournalLineTableName,
		// run from a journal (ReportPrint.PrintFAJnlLine): its template and batch
		OnInitReport: func(run *Run) error {
			line, ok := run.Context().(*gtables.FAJournalLine)
			if ok && line.Journal_template_name != "" {
				run.SetRequestFilter("FA_Journal_Batch", "journal_template_name", line.Journal_template_name.String())
				run.SetRequestFilter("FA_Journal_Batch", "name", line.Journal_batch_name.String())
			}
			return nil
		},
		OnPreReport: func(run *Run) error {
			setReportHeadValues(run)
			return nil
		},
		DataItems: []*DataItem{{
			Name: "FA_Journal_Batch", Table: gtables.FAJournalBatchTableName, Key: []string{"journal_template_name", "name"},
			RequestFilterFields: []string{"journal_template_name", "name"},
			OnAfterGetRecord: Trigger(func(batch *gtables.FAJournalBatch, run *Run) error {
				line := gtables.NewFAJournalLine(run)
				line.SetRange("journal_template_name", batch.Journal_template_name)
				line.SetRange("journal_batch_name", batch.Name)
				r.batch = batch
				return jnlTestBatchHasLines(run, line.IsEmpty())
			}),
			DataItems: []*DataItem{{
				Name: "FA_Journal_Line", Table: gtables.FAJournalLineTableName,
				Key:                 []string{"journal_template_name", "journal_batch_name", "line_no"},
				Link:                map[string]string{"journal_template_name": "journal_template_name", "journal_batch_name": "name"},
				RequestFilterFields: []string{"fa_posting_date"},
				OnPreDataItem:       Trigger(r.lineOnPreDataItem),
				OnAfterGetRecord:    Trigger(r.lineOnAfterGetRecord),
				DataItems:           []*DataItem{r.errorLoop()},
			}},
		}},
	}
}

// lineOnPreDataItem: a recurring journal's lines up to the work date by their FA posting date,
// not expired (not filtered on these dates)
func (r *faJnlTest) lineOnPreDataItem(line *gtables.FAJournalLine, run *Run) error {
	language := fcodeunits.CurrentLanguage()
	r.template = gtables.NewFAJournalTemplate(run)
	if !r.template.Get(r.batch.Journal_template_name) {
		r.template.Init()
	}
	setFilterText(run, line)
	r.errors = nil
	if r.template.Recurring {
		if line.GetFilter("fa_posting_date") != "" {
			r.addError(run.Text("CANNOT_BE_FILTERED", line.GetFieldCaption("fa_posting_date", language)))
		}
		line.SetRange("fa_posting_date", types.Date{}, system.WorkDate())
		if line.GetFilter("expiration_date") != "" {
			r.addError(run.Text("CANNOT_BE_FILTERED", line.GetFieldCaption("expiration_date", language)))
		}
		line.SetFilter("expiration_date", "''|"+system.WorkDate().String()+"..")
	}
	return nil
}

func (r *faJnlTest) lineOnAfterGetRecord(line *gtables.FAJournalLine, run *Run) error {
	language := fcodeunits.CurrentLanguage()
	r.errors = nil
	caption := func(field string) string { return line.GetFieldCaption(field, language) }
	if line.Fa_no == "" {
		return nil
	}
	if line.Fa_posting_date.IsZero() {
		r.addError(run.Text("MUST_BE_SPECIFIED", caption("fa_posting_date")))
	} else {
		if line.Fa_posting_date.IsClosing() {
			r.addError(run.Text("NOT_CLOSING_DATE", caption("fa_posting_date")))
		}
		if r.faDateNotAllowed(run, line.Fa_posting_date) {
			r.addError(run.Text("DATE_NOT_ALLOWED", caption("fa_posting_date")))
		}
	}

	if line.Document_no == "" {
		r.addError(run.Text("MUST_BE_SPECIFIED", caption("document_no")))
	}

	if line.Depreciation_book_code == "" {
		r.addError(run.Text("MUST_BE_SPECIFIED", caption("depreciation_book_code")))
	}
	if line.Depreciation_book_code == line.Duplicate_in_depreciation_book {
		r.addError(run.Text("NOT_DIFFERENT", caption("depreciation_book_code"), caption("duplicate_in_depreciation_book")))
	}
	fa := gtables.NewFixedAsset(run)
	if !fa.Get(line.Fa_no) {
		r.addError(run.Text("NOT_EXIST", fa.TableCaption(language), line.Fa_no))
		fa.Init()
	}
	if fa.Blocked {
		r.addError(run.Text("MUST_BE_FOR", fa.GetFieldCaption("blocked", language), formatValue(false, "", language), fa.TableCaption(language), line.Fa_no))
	}
	if fa.Inactive {
		r.addError(run.Text("MUST_BE_FOR", fa.GetFieldCaption("inactive", language), formatValue(false, "", language), fa.TableCaption(language), line.Fa_no))
	}

	deprBook := gtables.NewDepreciationBook(run)
	if !deprBook.Get(line.Depreciation_book_code) {
		r.addError(run.Text("NOT_EXIST", deprBook.TableCaption(language), line.Depreciation_book_code))
		deprBook.Init()
	}
	faDeprBook := gtables.NewFADepreciationBook(run)
	if !faDeprBook.Get(map[string]interface{}{"fa_no": line.Fa_no, "depreciation_book_code": line.Depreciation_book_code}) {
		r.addError(run.Text("SETUP_NOT_EXIST", faDeprBook.TableCaption(language), line.Fa_no, line.Depreciation_book_code))
		faDeprBook.Init()
	}
	r.checkFAIntegration(run, line, deprBook)
	if line.Fa_error_entry_no > 0 {
		r.checkErrorNo(run, line)
	}
	r.checkConsistency(run, line, deprBook)
	return nil
}

// faDateNotAllowed: the FA posting date is outside the user's allowed FA posting dates (User
// Setup, else FA Setup; no end: no limit)
func (r *faJnlTest) faDateNotAllowed(run *Run, faPostingDate types.Date) bool {
	if r.allowFAPostingFrom.IsZero() && r.allowFAPostingTo.IsZero() {
		if system.UserId() != "" {
			userSetup := gtables.NewUserSetup(run)
			if userSetup.Get(types.Code(system.UserId())) {
				r.allowFAPostingFrom = userSetup.Allow_fa_posting_from
				r.allowFAPostingTo = userSetup.Allow_fa_posting_to
			}
		}
		if r.allowFAPostingFrom.IsZero() && r.allowFAPostingTo.IsZero() {
			faSetup := gtables.GetFASetup(run)
			r.allowFAPostingFrom = faSetup.Allow_fa_posting_from
			r.allowFAPostingTo = faSetup.Allow_fa_posting_to
		}
		if r.allowFAPostingTo.IsZero() {
			r.allowFAPostingTo, _ = system.DMY2Date(31, 12, 9999)
		}
	}
	return faPostingDate.Before(r.allowFAPostingFrom) || r.allowFAPostingTo.Before(faPostingDate)
}

// checkFAIntegration (CheckFAIntegration): a posting type the book posts to the G/L belongs in
// the general journal, and so do Depr. until FA Posting Date and Depr. Acquisition Cost when the
// book posts depreciation to the G/L
func (r *faJnlTest) checkFAIntegration(run *Run, line *gtables.FAJournalLine, deprBook *gtables.DepreciationBook) {
	language := fcodeunits.CurrentLanguage()
	glIntegration := false
	switch line.Fa_posting_type {
	case gtables.FAJournalLine_Fa_posting_type.AcquisitionCost:
		glIntegration = deprBook.G_l_integration_acq_cost
	case gtables.FAJournalLine_Fa_posting_type.Depreciation:
		glIntegration = deprBook.G_l_integration_depreciation
	case gtables.FAJournalLine_Fa_posting_type.WriteDown:
		glIntegration = deprBook.G_l_integration_write_down
	case gtables.FAJournalLine_Fa_posting_type.Appreciation:
		glIntegration = deprBook.G_l_integration_appreciation
	case gtables.FAJournalLine_Fa_posting_type.Custom1:
		glIntegration = deprBook.G_l_integration_custom_1
	case gtables.FAJournalLine_Fa_posting_type.Custom2:
		glIntegration = deprBook.G_l_integration_custom_2
	case gtables.FAJournalLine_Fa_posting_type.Disposal:
		glIntegration = deprBook.G_l_integration_disposal
	case gtables.FAJournalLine_Fa_posting_type.Maintenance:
		glIntegration = deprBook.G_l_integration_maintenance
	case gtables.FAJournalLine_Fa_posting_type.SalvageValue:
		glIntegration = false
	}
	if glIntegration {
		r.addError(run.Text("GL_INTEGRATION", faPostingTypeCaption(line, language)))
	}
	if deprBook.G_l_integration_depreciation {
		if line.Depr_until_fa_posting_date {
			r.addError(run.Text("GL_INTEGRATION", line.GetFieldCaption("depr_until_fa_posting_date", language)))
		}
		if line.Depr_acquisition_cost {
			r.addError(run.Text("GL_INTEGRATION", line.GetFieldCaption("depr_acquisition_cost", language)))
		}
	}
}

// checkErrorNo (CheckErrorNo): a line canceling an entry (FA Error Entry No.) has none of the
// fields of a new entry
func (r *faJnlTest) checkErrorNo(run *Run, line *gtables.FAJournalLine) {
	language := fcodeunits.CurrentLanguage()
	caption := func(field string) string { return line.GetFieldCaption(field, language) }
	errorEntryNo := caption("fa_error_entry_no")
	if line.Depr_until_fa_posting_date {
		r.addError(run.Text("NOT_WITH_ERROR_ENTRY", caption("depr_until_fa_posting_date"), errorEntryNo))
	}
	if line.Depr_acquisition_cost {
		r.addError(run.Text("NOT_WITH_ERROR_ENTRY", caption("depr_acquisition_cost"), errorEntryNo))
	}
	if line.Duplicate_in_depreciation_book != "" {
		r.addError(run.Text("NOT_WITH_ERROR_ENTRY", caption("duplicate_in_depreciation_book"), errorEntryNo))
	}
	if line.Use_duplication_list {
		r.addError(run.Text("NOT_WITH_ERROR_ENTRY", caption("use_duplication_list"), errorEntryNo))
	}
	if !line.Salvage_value.IsZero() {
		r.addError(run.Text("NOT_WITH_ERROR_ENTRY", caption("salvage_value"), errorEntryNo))
	}
	if line.Insurance_no != "" {
		r.addError(run.Text("NOT_WITH_ERROR_ENTRY", caption("insurance_no"), errorEntryNo))
	}
	if line.Recurring_method != gtables.FAJournalLine_Recurring_method.Blank {
		r.addError(run.Text("NOT_WITH_ERROR_ENTRY", caption("recurring_method"), errorEntryNo))
	}
	if line.Fa_posting_type == gtables.FAJournalLine_Fa_posting_type.Maintenance {
		r.addError(run.Text("NOT_WITH_ERROR_ENTRY", faPostingTypeCaption(line, language), errorEntryNo))
	}
}

// checkConsistency (CheckConsistency): the fields the line's FA posting type does not use,
// insurance integration and the book's Use Same FA+G/L Posting Dates
func (r *faJnlTest) checkConsistency(run *Run, line *gtables.FAJournalLine, deprBook *gtables.DepreciationBook) {
	language := fcodeunits.CurrentLanguage()
	caption := func(field string) string { return line.GetFieldCaption(field, language) }
	postingType := faPostingTypeCaption(line, language)
	notTogether := func(field string) {
		r.addError(run.Text("NOT_TOGETHER_WITH", caption(field), caption("fa_posting_type"), postingType))
	}
	if line.Fa_posting_type != gtables.FAJournalLine_Fa_posting_type.AcquisitionCost {
		if line.Depr_acquisition_cost {
			notTogether("depr_acquisition_cost")
		}
		if !line.Salvage_value.IsZero() {
			notTogether("salvage_value")
		}
		if !line.Quantity.IsZero() && line.Fa_posting_type != gtables.FAJournalLine_Fa_posting_type.Maintenance {
			notTogether("quantity")
		}
		if line.Insurance_no != "" {
			notTogether("insurance_no")
		}
	}
	if line.Fa_posting_type == gtables.FAJournalLine_Fa_posting_type.Maintenance && line.Depr_until_fa_posting_date {
		notTogether("depr_until_fa_posting_date")
	}
	if line.Fa_posting_type != gtables.FAJournalLine_Fa_posting_type.Maintenance && line.Maintenance_code != "" {
		notTogether("maintenance_code")
	}
	if line.Fa_posting_type != gtables.FAJournalLine_Fa_posting_type.Depreciation &&
		line.Fa_posting_type != gtables.FAJournalLine_Fa_posting_type.Custom1 &&
		line.No_of_depreciation_days != 0 {
		notTogether("no_of_depreciation_days")
	}
	if line.Fa_posting_type == gtables.FAJournalLine_Fa_posting_type.Disposal && line.Fa_reclassification_entry {
		notTogether("fa_reclassification_entry")
	}

	faSetup := gtables.GetFASetup(run)
	if line.Fa_posting_type == gtables.FAJournalLine_Fa_posting_type.AcquisitionCost &&
		line.Insurance_no != "" && deprBook.Code != faSetup.Insurance_depr_book {
		r.addError(run.Text("NO_INSURANCE_INTEGRATION", caption("depreciation_book_code"), line.Depreciation_book_code))
	}

	if deprBook.Use_same_fa_g_l_posting_dates && line.Posting_date != line.Fa_posting_date && !line.Posting_date.IsZero() {
		r.addError(run.Text("MUST_BE_IDENTICAL", caption("posting_date"), caption("fa_posting_date")))
	}
}

// Report 5622 Insurance Journal - Test
type insuranceJnlTest struct {
	jnlTest
}

func (r *insuranceJnlTest) Definition() *Definition {
	return &Definition{
		ContextTable: gtables.InsuranceJournalLineTableName,
		// run from a journal (ReportPrint.PrintInsuranceJnlLine): its template and batch
		OnInitReport: func(run *Run) error {
			line, ok := run.Context().(*gtables.InsuranceJournalLine)
			if ok && line.Journal_template_name != "" {
				run.SetRequestFilter("Insurance_Journal_Batch", "journal_template_name", line.Journal_template_name.String())
				run.SetRequestFilter("Insurance_Journal_Batch", "name", line.Journal_batch_name.String())
			}
			return nil
		},
		OnPreReport: func(run *Run) error {
			setReportHeadValues(run)
			return nil
		},
		DataItems: []*DataItem{{
			Name: "Insurance_Journal_Batch", Table: gtables.InsuranceJournalBatchTableName, Key: []string{"journal_template_name", "name"},
			RequestFilterFields: []string{"journal_template_name", "name"},
			OnAfterGetRecord: Trigger(func(batch *gtables.InsuranceJournalBatch, run *Run) error {
				line := gtables.NewInsuranceJournalLine(run)
				line.SetRange("journal_template_name", batch.Journal_template_name)
				line.SetRange("journal_batch_name", batch.Name)
				return jnlTestBatchHasLines(run, line.IsEmpty())
			}),
			DataItems: []*DataItem{{
				Name: "Insurance_Journal_Line", Table: gtables.InsuranceJournalLineTableName,
				Key:                 []string{"journal_template_name", "journal_batch_name", "line_no"},
				Link:                map[string]string{"journal_template_name": "journal_template_name", "journal_batch_name": "name"},
				RequestFilterFields: []string{"posting_date"},
				OnPreDataItem: Trigger(func(line *gtables.InsuranceJournalLine, run *Run) error {
					setFilterText(run, line)
					return nil
				}),
				OnAfterGetRecord: Trigger(r.lineOnAfterGetRecord),
				DataItems:        []*DataItem{r.errorLoop()},
			}},
		}},
	}
}

func (r *insuranceJnlTest) lineOnAfterGetRecord(line *gtables.InsuranceJournalLine, run *Run) error {
	language := fcodeunits.CurrentLanguage()
	r.errors = nil
	caption := func(field string) string { return line.GetFieldCaption(field, language) }
	if line.Insurance_no == "" {
		return nil
	}
	if line.Posting_date.IsZero() {
		r.addError(run.Text("MUST_BE_SPECIFIED", caption("posting_date")))
	}
	if line.Fa_no == "" {
		r.addError(run.Text("MUST_BE_SPECIFIED", caption("fa_no")))
	} else {
		fa := gtables.NewFixedAsset(run)
		if !fa.Get(line.Fa_no) {
			r.addError(run.Text("NOT_EXIST", fa.TableCaption(language), line.Fa_no))
		}
	}
	if line.Document_no == "" {
		r.addError(run.Text("MUST_BE_SPECIFIED", caption("document_no")))
	}
	return nil
}

// Report 5623 Insurance Register
type insuranceRegister struct {
	insuranceRegister *gtables.InsuranceRegister
}

func (r *insuranceRegister) Definition() *Definition {
	return &Definition{
		OnPreReport: func(run *Run) error {
			setReportHeadValues(run)
			return nil
		},
		DataItems: []*DataItem{{
			Name: "Insurance_Register", Table: gtables.InsuranceRegisterTableName, Key: []string{"no"},
			RequestFilterFields: []string{"no"},
			OnPreDataItem:       headWithFilters[*gtables.InsuranceRegister](nil),
			OnAfterGetRecord: Trigger(func(insuranceReg *gtables.InsuranceRegister, run *Run) error {
				r.insuranceRegister = insuranceReg
				return nil
			}),
			DataItems: []*DataItem{{
				Name: "Ins_Coverage_Ledger_Entry", Table: gtables.InsCoverageLedgerEntryTableName, Key: []string{"entry_no"},
				OnPreDataItem: Trigger(func(insCoverageLedgEntry *gtables.InsCoverageLedgerEntry, run *Run) error {
					insCoverageLedgEntry.SetRange("entry_no", r.insuranceRegister.From_entry_no, r.insuranceRegister.To_entry_no)
					return nil
				}),
				OnAfterGetRecord: Trigger(func(insCoverageLedgEntry *gtables.InsCoverageLedgerEntry, run *Run) error {
					fa := gtables.NewFixedAsset(run)
					if !fa.Get(insCoverageLedgEntry.Fa_no) {
						fa.Init()
					}
					insurance := gtables.NewInsurance(run)
					if !insurance.Get(insCoverageLedgEntry.Insurance_no) {
						insurance.Init()
					}
					run.SetValue("fa_description", fa.Description.String())
					run.SetValue("insurance_description", insurance.Description.String())
					return nil
				}),
			}},
		}},
	}
}

// faPostingTypeCaption: the line's FA posting type in the user's language
func faPostingTypeCaption(line *gtables.FAJournalLine, language string) string {
	return optionCaption(gtables.FAJournalLineTableName, "fa_posting_type", line.Fa_posting_type.String(), language)
}
