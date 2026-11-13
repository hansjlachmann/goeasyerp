package reports

import (
	"github.com/hansjlachmann/goeasyerp/backend/foundation/types"
	gtables "github.com/hansjlachmann/goeasyerp/backend/generated/tables"
)

// The insurance printouts (main menu Assets & Depreciations → Reports → Insurance):
//   - 5620 Insurance - Analysis: per insurance its annual premium, policy coverage, the total
//     value insured and the over/under insured amount (U: under insured), with totals; Print per
//     Insurance No. prints the lines, else only the totals.
//   - 5621 Insurance - List: the insurances with effective date, policy no., premium, coverage,
//     type and vendor.
//   - 5624 Insurance - Coverage Details: per insurance its coverage ledger entries (Date Filter:
//     those posted in the dates); insurances without entries are not printed; New Page per
//     Insurance No.
//   - 5625 Insurance - Tot. Value Insured: per fixed asset (not disposed in the FA Setup's
//     Insurance Depr. Book, not inactive) the value insured by each insurance — the asset's
//     coverage entries per insurance, not those of a disposed asset; assets without coverage are
//     not printed.
//   - 5626 Insurance - Uninsured FAs: the fixed assets (not disposed, not inactive) without
//     coverage, with their acquisition cost, depreciation and book value in the Insurance Depr.
//     Book, and the totals.
const (
	InsuranceAnalysisID        = 5620
	InsuranceListID            = 5621
	InsuranceCoverageDetailsID = 5624
	InsuranceTotValueInsuredID = 5625
	InsuranceUninsuredFAsID    = 5626
)

func init() {
	Register(InsuranceAnalysisID, "insurance-analysis", func() Report { return &insuranceAnalysis{} })
	Register(InsuranceListID, "insurance-list", func() Report { return &insuranceList{} })
	Register(InsuranceCoverageDetailsID, "insurance-coverage-details", func() Report { return &insuranceCoverageDetails{} })
	Register(InsuranceTotValueInsuredID, "insurance-tot-value-insured", func() Report { return &insuranceTotValueInsured{} })
	Register(InsuranceUninsuredFAsID, "insurance-uninsured-fas", func() Report { return &insuranceUninsuredFAs{} })
}

// insuranceOnPreDataItem: the insurances' filters in the head (InsuranceFilter)
func insuranceOnPreDataItem(rec *gtables.Insurance, run *Run) error {
	setFilterText(run, rec)
	return nil
}

// insuranceHead (OnPreReport): the head of an insurance printout
func insuranceHead(run *Run) error {
	setReportHeadValues(run)
	return nil
}

// --- 5620 Insurance - Analysis ---

type insuranceAnalysis struct {
	printDetails bool
	totalAmounts [4]types.Decimal
}

func (r *insuranceAnalysis) Definition() *Definition {
	return &Definition{
		OptionsCaption: "OPTIONS",
		Options:        []Option{{Name: "print_per_insurance_no", Type: "bool", Default: "false"}},
		OnPreReport: func(run *Run) error {
			r.printDetails = run.OptionBool("print_per_insurance_no")
			run.SetValue("print_details", r.printDetails)
			return insuranceHead(run)
		},
		DataItems: []*DataItem{{
			Name:                "Insurance",
			Table:               gtables.InsuranceTableName,
			Key:                 []string{"no"},
			RequestFilterFields: []string{"no", "fa_class_code", "fa_subclass_code"},
			OnPreDataItem:       Trigger(insuranceOnPreDataItem),
			OnAfterGetRecord:    Trigger(r.insuranceOnAfterGetRecord),
		}},
	}
}

// insuranceOnAfterGetRecord: the over/under insured amount (U when under insured) and the totals
func (r *insuranceAnalysis) insuranceOnAfterGetRecord(rec *gtables.Insurance, run *Run) error {
	rec.CalcFields("total_value_insured")
	overUnderInsured := rec.Policy_coverage.Sub(rec.Total_value_insured)
	underInsured := ""
	if overUnderInsured.IsNegative() {
		underInsured = "U"
	}
	run.SetValue("over_under_insured", overUnderInsured)
	run.SetValue("under_insured", underInsured)
	r.totalAmounts[0] = r.totalAmounts[0].Add(rec.Annual_premium)
	r.totalAmounts[1] = r.totalAmounts[1].Add(rec.Policy_coverage)
	r.totalAmounts[2] = r.totalAmounts[2].Add(rec.Total_value_insured)
	r.totalAmounts[3] = r.totalAmounts[3].Add(overUnderInsured)
	for i, amount := range r.totalAmounts {
		run.SetValue("tot_amount_"+itoa(i+1), amount)
	}
	return nil
}

// --- 5621 Insurance - List ---

type insuranceList struct{}

func (r *insuranceList) Definition() *Definition {
	return &Definition{
		OnPreReport: insuranceHead,
		DataItems: []*DataItem{{
			Name:                "Insurance",
			Table:               gtables.InsuranceTableName,
			Key:                 []string{"no"},
			RequestFilterFields: []string{"no"},
			OnPreDataItem:       Trigger(insuranceOnPreDataItem),
		}},
	}
}

// --- 5624 Insurance - Coverage Details ---

type insuranceCoverageDetails struct {
	printOnlyOnePerPage bool
	insurance           *gtables.Insurance
	printedInsurances   int
}

func (r *insuranceCoverageDetails) Definition() *Definition {
	return &Definition{
		OptionsCaption: "OPTIONS",
		Options:        []Option{{Name: "new_page_per_insurance_no", Type: "bool", Default: "false"}},
		OnPreReport: func(run *Run) error {
			r.printOnlyOnePerPage = run.OptionBool("new_page_per_insurance_no")
			return insuranceHead(run)
		},
		DataItems: []*DataItem{{
			Name:                "Insurance",
			Table:               gtables.InsuranceTableName,
			Key:                 []string{"no"},
			RequestFilterFields: []string{"no", "fa_class_code", "fa_subclass_code"},
			OnPreDataItem:       Trigger(insuranceOnPreDataItem),
			OnAfterGetRecord:    Trigger(r.insuranceOnAfterGetRecord),
			DataItems: []*DataItem{{
				Name:          "InsCoverageLedgerEntry",
				Table:         gtables.InsCoverageLedgerEntryTableName,
				Key:           []string{"insurance_no", "disposed_fa", "posting_date"},
				OnPreDataItem: Trigger(r.insCoverageLedgEntryOnPreDataItem),
			}},
		}},
	}
}

// insuranceOnAfterGetRecord: insurances with entries (PrintOnlyIfDetail); New Page per Insurance
// No. (NEWPAGEPERRECORD)
func (r *insuranceCoverageDetails) insuranceOnAfterGetRecord(rec *gtables.Insurance, run *Run) error {
	r.insurance = rec
	insCoverageLedgEntry := gtables.NewInsCoverageLedgerEntry(run)
	r.setEntryFilters(insCoverageLedgEntry)
	if insCoverageLedgEntry.IsEmpty() {
		return Skip()
	}
	if r.printOnlyOnePerPage && r.printedInsurances > 0 {
		run.NewPage()
	}
	r.printedInsurances++
	return nil
}

// insCoverageLedgEntryOnPreDataItem: the insurance's entries in its Date Filter
func (r *insuranceCoverageDetails) insCoverageLedgEntryOnPreDataItem(rec *gtables.InsCoverageLedgerEntry, run *Run) error {
	r.setEntryFilters(rec)
	return nil
}

func (r *insuranceCoverageDetails) setEntryFilters(insCoverageLedgEntry *gtables.InsCoverageLedgerEntry) {
	insCoverageLedgEntry.SetCurrentKey("insurance_no", "disposed_fa", "posting_date")
	insCoverageLedgEntry.SetRange("insurance_no", r.insurance.No)
	if r.insurance.Date_filter != "" {
		insCoverageLedgEntry.SetFilter("posting_date", r.insurance.Date_filter)
	}
}

// --- 5625 Insurance - Tot. Value Insured ---

type insuranceTotValueInsured struct {
	deprBook     *gtables.DepreciationBook
	fa           *gtables.FixedAsset
	firstTime    bool
	insuranceTmp map[types.Code]bool // InsuranceTmp: the insurances printed for the asset
}

func (r *insuranceTotValueInsured) Definition() *Definition {
	return &Definition{
		OnPreReport: r.onPreReport,
		DataItems: []*DataItem{{
			Name:                "FixedAsset",
			Table:               gtables.FixedAssetTableName,
			Key:                 []string{"no"},
			RequestFilterFields: []string{"no", "fa_class_code", "fa_subclass_code"},
			OnPreDataItem:       Trigger(faFilterOnPreDataItem),
			OnAfterGetRecord:    Trigger(r.faOnAfterGetRecord),
			DataItems: []*DataItem{{
				Name:             "InsCoverageLedgerEntry",
				Table:            gtables.InsCoverageLedgerEntryTableName,
				Key:              []string{"fa_no", "insurance_no", "disposed_fa"},
				OnPreDataItem:    Trigger(r.insCoverageLedgEntryOnPreDataItem),
				OnAfterGetRecord: Trigger(r.insCoverageLedgEntryOnAfterGetRecord),
			}},
		}},
	}
}

// faFilterOnPreDataItem: the assets' filters in the head (FAFilter)
func faFilterOnPreDataItem(rec *gtables.FixedAsset, run *Run) error {
	setFilterText(run, rec)
	return nil
}

// insuranceDeprBook: the FA Setup's Insurance Depr. Book (TESTFIELD, GET)
func insuranceDeprBook(run *Run) (*gtables.DepreciationBook, error) {
	faSetup := gtables.GetFASetup(run)
	err := faSetup.TestField("insurance_depr_book")
	if err != nil {
		return nil, err
	}
	deprBook := gtables.NewDepreciationBook(run)
	err = deprBook.GetOrError(faSetup.Insurance_depr_book)
	if err != nil {
		return nil, err
	}
	return deprBook, nil
}

func (r *insuranceTotValueInsured) onPreReport(run *Run) error {
	var err error
	r.deprBook, err = insuranceDeprBook(run)
	if err != nil {
		return err
	}
	return insuranceHead(run)
}

// faOnAfterGetRecord: assets in the book not disposed and not inactive; the asset's no. and
// description print on its first line only
func (r *insuranceTotValueInsured) faOnAfterGetRecord(rec *gtables.FixedAsset, run *Run) error {
	faDeprBook := gtables.NewFADepreciationBook(run)
	if !faDeprBook.Get(map[string]interface{}{"fa_no": rec.No, "depreciation_book_code": r.deprBook.Code}) {
		return Skip()
	}
	if !faDeprBook.Disposal_date.IsZero() || rec.Inactive {
		return Skip()
	}
	r.fa = rec
	r.firstTime = true
	return nil
}

// insCoverageLedgEntryOnPreDataItem: the asset's entries; no insurance printed yet
func (r *insuranceTotValueInsured) insCoverageLedgEntryOnPreDataItem(rec *gtables.InsCoverageLedgerEntry, run *Run) error {
	rec.SetRange("fa_no", r.fa.No)
	r.insuranceTmp = map[types.Code]bool{}
	return nil
}

// insCoverageLedgEntryOnAfterGetRecord: each insurance once, with the asset's coverage by it
// (not of a disposed asset), skipped when 0
func (r *insuranceTotValueInsured) insCoverageLedgEntryOnAfterGetRecord(rec *gtables.InsCoverageLedgerEntry, run *Run) error {
	insurance := gtables.NewInsurance(run)
	if !insurance.Get(rec.Insurance_no) {
		insurance.Init()
	}
	if r.insuranceTmp[rec.Insurance_no] {
		return Skip()
	}
	r.insuranceTmp[rec.Insurance_no] = true
	insCoverageLedgEntry := gtables.NewInsCoverageLedgerEntry(run)
	insCoverageLedgEntry.SetCurrentKey("fa_no", "insurance_no", "disposed_fa")
	insCoverageLedgEntry.SetRange("disposed_fa", false)
	insCoverageLedgEntry.SetRange("fa_no", r.fa.No)
	insCoverageLedgEntry.SetRange("insurance_no", rec.Insurance_no)
	err := insCoverageLedgEntry.CalcSums("amount")
	if err != nil {
		return err
	}
	if insCoverageLedgEntry.Amount.IsZero() {
		return Skip()
	}
	faNo := ""
	faDescription := ""
	if r.firstTime {
		faNo = r.fa.No.String()
		faDescription = r.fa.Description.String()
	}
	r.firstTime = false
	run.SetValue("fa_no", faNo)
	run.SetValue("fa_description", faDescription)
	run.SetValue("insurance_description", insurance.Description.String())
	run.SetValue("total_value_insured", insCoverageLedgEntry.Amount)
	return nil
}

// --- 5626 Insurance - Uninsured FAs ---

type insuranceUninsuredFAs struct {
	deprBook     *gtables.DepreciationBook
	totalAmounts [3]types.Decimal
}

func (r *insuranceUninsuredFAs) Definition() *Definition {
	return &Definition{
		OnPreReport: r.onPreReport,
		DataItems: []*DataItem{{
			Name:                "FixedAsset",
			Table:               gtables.FixedAssetTableName,
			Key:                 []string{"no"},
			RequestFilterFields: []string{"no", "fa_class_code", "fa_subclass_code"},
			OnPreDataItem:       Trigger(faFilterOnPreDataItem),
			OnAfterGetRecord:    Trigger(r.faOnAfterGetRecord),
		}},
	}
}

func (r *insuranceUninsuredFAs) onPreReport(run *Run) error {
	var err error
	r.deprBook, err = insuranceDeprBook(run)
	if err != nil {
		return err
	}
	return insuranceHead(run)
}

// faOnAfterGetRecord: assets in the book that are not disposed, insured or inactive, with their
// acquisition cost, depreciation and book value
func (r *insuranceUninsuredFAs) faOnAfterGetRecord(rec *gtables.FixedAsset, run *Run) error {
	faDeprBook := gtables.NewFADepreciationBook(run)
	if !faDeprBook.Get(map[string]interface{}{"fa_no": rec.No, "depreciation_book_code": r.deprBook.Code}) {
		return Skip()
	}
	rec.CalcFields("insured")
	if !faDeprBook.Disposal_date.IsZero() || rec.Insured || rec.Inactive {
		return Skip()
	}
	faDeprBook.CalcFields("acquisition_cost", "depreciation", "book_value")
	amounts := [3]types.Decimal{faDeprBook.Acquisition_cost, faDeprBook.Depreciation, faDeprBook.Book_value}
	for i := range amounts {
		r.totalAmounts[i] = r.totalAmounts[i].Add(amounts[i])
		run.SetValue("amount_"+itoa(i+1), amounts[i])
		run.SetValue("tot_amount_"+itoa(i+1), r.totalAmounts[i])
	}
	return nil
}
