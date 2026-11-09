package reports

import (
	"github.com/hansjlachmann/goeasyerp/backend/business-logic/fadepr"
	"github.com/hansjlachmann/goeasyerp/backend/business-logic/fajnl"
	"github.com/hansjlachmann/goeasyerp/backend/business-logic/tables"
	"github.com/hansjlachmann/goeasyerp/backend/foundation/system"
	"github.com/hansjlachmann/goeasyerp/backend/foundation/types"
	gtables "github.com/hansjlachmann/goeasyerp/backend/generated/tables"
)

// Report 5690 Index Fixed Assets: the fixed assets' amounts of the chosen posting types in a
// depreciation book (Allow Indexation) up to the FA posting date — acquisition cost,
// depreciation, write-down, appreciation, custom 1/2, maintenance, salvage value — are indexed:
// a line of amount × (Index Figure / 100 − 1), marked Index Entry, in the general journal of the
// book's FA Journal Setup when the book posts the type to the G/L (not for budgeted assets, with
// Insert Bal. Account the balancing line too), else in its FA journal; the user posts them.
// Inactive, blocked and disposed assets and those acquired after the date are left out.
const IndexFixedAssetsID = 5690

func init() {
	Register(IndexFixedAssetsID, "index-fixed-assets", func() Report { return &indexFixedAssets{} })
}

// indexChoices: the request page's Index check boxes in the classic order (IndexChoices 1–6, 8,
// 9), their FA journal posting type and the FA ledger entries' (maintenance: its own entries)
var indexChoices = []struct {
	option          string
	postingType     gtables.FAJournalLineFa_posting_type
	faLedgEntryType gtables.FALedgerEntryFa_posting_type
}{
	{"acquisition_cost", gtables.FAJournalLine_Fa_posting_type.AcquisitionCost, gtables.FALedgerEntry_Fa_posting_type.AcquisitionCost},
	{"depreciation", gtables.FAJournalLine_Fa_posting_type.Depreciation, gtables.FALedgerEntry_Fa_posting_type.Depreciation},
	{"write_down", gtables.FAJournalLine_Fa_posting_type.WriteDown, gtables.FALedgerEntry_Fa_posting_type.WriteDown},
	{"appreciation", gtables.FAJournalLine_Fa_posting_type.Appreciation, gtables.FALedgerEntry_Fa_posting_type.Appreciation},
	{"custom_1", gtables.FAJournalLine_Fa_posting_type.Custom1, gtables.FALedgerEntry_Fa_posting_type.Custom1},
	{"custom_2", gtables.FAJournalLine_Fa_posting_type.Custom2, gtables.FALedgerEntry_Fa_posting_type.Custom2},
	{"maintenance", gtables.FAJournalLine_Fa_posting_type.Maintenance, 0},
	{"salvage_value", gtables.FAJournalLine_Fa_posting_type.SalvageValue, gtables.FALedgerEntry_Fa_posting_type.SalvageValue},
}

type indexFixedAssets struct {
	deprBook                          *gtables.DepreciationBook
	faPostingDate, postingDate        types.Date
	indexFigure                       types.Decimal
	documentNo                        types.Code
	postingDescription                types.Text
	balAccount                        bool
	firstGenJnl, firstFAJnl           bool
	genJnlTemplate, genJnlBatch       types.Code
	faJnlTemplate, faJnlBatch         types.Code
	genJnlNextLineNo, faJnlNextLineNo int
	documentNo2, documentNo3          types.Code
	noSeries2, noSeries3              types.Code
	window                            *system.Window
}

func (r *indexFixedAssets) Definition() *Definition {
	options := []Option{
		{Name: "depreciation_book", Type: "text", Required: true, Relation: &OptionRelation{Table: "Depreciation_Book", Columns: []string{"code", "description"}}},
		{Name: "index_figure", Type: "number", Required: true},
		{Name: "fa_posting_date", Type: "date", Required: true},
		{Name: "posting_date", Type: "date"},
		{Name: "document_no", Type: "text"},
		{Name: "posting_description", Type: "text"},
		{Name: "insert_bal_account", Type: "bool", Default: "false"},
	}
	for _, c := range indexChoices {
		options = append(options, Option{Name: c.option, Type: "bool", Default: "false"})
	}
	return &Definition{
		Options:      options,
		Transaction:  true,
		OnInitReport: r.onInitReport,
		OnPreReport:  r.onPreReport,
		DataItems: []*DataItem{{
			Name: "Fixed_Asset", Table: gtables.FixedAssetTableName, Key: []string{"no"},
			RequestFilterFields: []string{"no", "fa_class_code", "fa_subclass_code", "budgeted_asset"},
			OnAfterGetRecord:    Trigger(r.onAfterGetRecord),
			OnPostDataItem: func(rec any, run *Run) error {
				r.window.Close()
				return nil
			},
		}},
	}
}

// onInitReport (OnOpenForm): the FA Setup's default depreciation book
func (r *indexFixedAssets) onInitReport(run *Run) error {
	faSetup := gtables.GetFASetup(run)
	run.SetOption("depreciation_book", faSetup.Default_depr_book.String())
	return nil
}

func (r *indexFixedAssets) onPreReport(run *Run) error {
	r.faPostingDate = run.OptionDate("fa_posting_date")
	if r.faPostingDate.IsZero() {
		return system.Error(run.Text("ENTER_FA_POSTING_DATE"))
	}
	if r.faPostingDate.IsClosing() {
		return system.Error(run.Text("FA_POSTING_DATE_CLOSING"))
	}
	r.postingDate = run.OptionDate("posting_date")
	if r.postingDate.IsZero() {
		r.postingDate = r.faPostingDate
	}
	r.deprBook = gtables.NewDepreciationBook(run)
	err := r.deprBook.GetOrError(types.Code(system.UpperCase(run.Option("depreciation_book"))))
	if err != nil {
		return err
	}
	err = r.deprBook.TestField("allow_indexation", true)
	if err != nil {
		return err
	}
	if r.deprBook.Use_same_fa_g_l_posting_dates && !r.faPostingDate.Equal(r.postingDate) {
		return system.Error(run.Text("SAME_DATES", r.deprBook.Code))
	}
	r.indexFigure = run.OptionDecimal("index_figure")
	if r.indexFigure.IsNegative() {
		return system.Error(run.Text("INDEX_FIGURE_NEGATIVE"))
	}
	r.documentNo = types.Code(system.UpperCase(run.Option("document_no")))
	r.postingDescription = types.Text(run.Option("posting_description"))
	r.balAccount = run.OptionBool("insert_bal_account")
	r.firstGenJnl = true
	r.firstFAJnl = true
	r.window = system.OpenWindow(run.Text("WINDOW"))
	return nil
}

func (r *indexFixedAssets) onAfterGetRecord(fa *gtables.FixedAsset, run *Run) error {
	faDeprBook := gtables.NewFADepreciationBook(run)
	if !faDeprBook.Get(map[string]interface{}{"fa_no": fa.No, "depreciation_book_code": r.deprBook.Code}) {
		return Skip()
	}
	r.window.Update(1, fa.No)
	if fa.Inactive || fa.Blocked || !faDeprBook.Disposal_date.IsZero() ||
		faDeprBook.Acquisition_date.IsZero() || faDeprBook.Acquisition_date.After(r.faPostingDate) {
		return Skip()
	}

	faLedgEntry := fadepr.SetFAFilter(run, fa.No, r.deprBook.Code, true)
	faLedgEntry.SetRange("fa_posting_category", gtables.FALedgerEntry_Fa_posting_category.Blank)
	faLedgEntry.SetRange("fa_posting_date", types.Date{}, r.faPostingDate)
	maintenanceLedgEntry := gtables.NewMaintenanceLedgerEntry(run)
	maintenanceLedgEntry.SetCurrentKey("fa_no", "depreciation_book_code", "fa_posting_date")
	maintenanceLedgEntry.SetRange("fa_no", fa.No)
	maintenanceLedgEntry.SetRange("depreciation_book_code", r.deprBook.Code)
	maintenanceLedgEntry.SetRange("fa_posting_date", types.Date{}, r.faPostingDate)

	hundred := types.NewDecimalFromInt(100)
	one := types.NewDecimalFromInt(1)
	for _, c := range indexChoices {
		if !run.OptionBool(c.option) {
			continue
		}
		var indexAmount types.Decimal
		var err error
		if c.postingType == gtables.FAJournalLine_Fa_posting_type.Maintenance {
			err = maintenanceLedgEntry.CalcSums("amount")
			indexAmount = maintenanceLedgEntry.Amount
		} else {
			faLedgEntry.SetRange("fa_posting_type", c.faLedgEntryType)
			err = faLedgEntry.CalcSums("amount")
			indexAmount = faLedgEntry.Amount
		}
		if err != nil {
			return err
		}
		indexAmount = fadepr.CalcRounding(r.deprBook, indexAmount.Mul(r.indexFigure.Div(hundred).Sub(one)))
		if !tables.DeprBookGLIntegration(r.deprBook, c.postingType) || fa.Budgeted_asset {
			err = r.insertFAJnlLine(run, fa.No, indexAmount, c.postingType)
		} else {
			err = r.insertGenJnlLine(run, fa.No, indexAmount, c.postingType)
		}
		if err != nil {
			return err
		}
	}
	return nil
}

// documentNoOrNext: the Document No. option, else the batch series' next number (the batch must
// be empty), else an error
func (r *indexFixedAssets) documentNoOrNext(run *Run, next func() (types.Code, error)) (types.Code, error) {
	if r.documentNo != "" {
		return r.documentNo, nil
	}
	documentNo, err := next()
	if err != nil {
		return "", err
	}
	if documentNo == "" {
		return "", system.Error(run.Text("ENTER_DOCUMENT_NO"))
	}
	return documentNo, nil
}

// insertGenJnlLine (InsertGenJnlLine): the index line in the general journal of the book
func (r *indexFixedAssets) insertGenJnlLine(run *Run, faNo types.Code, indexAmount types.Decimal, postingType gtables.FAJournalLineFa_posting_type) error {
	if indexAmount.IsZero() {
		return nil
	}
	var err error
	if r.firstGenJnl {
		gtables.NewGenJournalLine(run).LockTable()
		r.genJnlTemplate, r.genJnlBatch, r.genJnlNextLineNo, err = tables.GenJnlName(run, r.deprBook.Code)
		if err != nil {
			return err
		}
		r.noSeries2 = tables.GetGenNoSeries(run, r.genJnlTemplate, r.genJnlBatch)
		r.documentNo2, err = r.documentNoOrNext(run, func() (types.Code, error) {
			return tables.GetGenJnlDocumentNo(run, r.genJnlTemplate, r.genJnlBatch, r.faPostingDate)
		})
		if err != nil {
			return err
		}
		r.firstGenJnl = false
	}
	genJnlLine := gtables.NewGenJournalLine(run)
	genJnlLine.Init()
	genJnlLine.Journal_template_name = r.genJnlTemplate
	genJnlLine.Journal_batch_name = r.genJnlBatch
	genJnlLine.Fa_posting_date = r.faPostingDate
	genJnlLine.Posting_date = r.postingDate
	if genJnlLine.Posting_date.Equal(genJnlLine.Fa_posting_date) {
		genJnlLine.Fa_posting_date = types.Date{}
	}
	genJnlLine.Fa_posting_type = gtables.GenJournalLineFa_posting_type(int(postingType) + 1)
	genJnlLine.Account_type = gtables.GenJournalLine_Account_type.FixedAsset
	err = genJnlLine.Validate("account_no", faNo)
	if err != nil {
		return err
	}
	genJnlLine.Document_no = r.documentNo2
	genJnlLine.Posting_no_series = r.noSeries2
	genJnlLine.Description = r.postingDescription
	err = genJnlLine.Validate("depreciation_book_code", r.deprBook.Code)
	if err != nil {
		return err
	}
	err = genJnlLine.Validate("amount", indexAmount)
	if err != nil {
		return err
	}
	genJnlLine.Index_entry = true
	r.genJnlNextLineNo += 10000
	genJnlLine.Line_no = r.genJnlNextLineNo
	err = tables.SetGenJnlTrailCodes(genJnlLine)
	if err != nil {
		return err
	}
	err = genJnlLine.InsertOrError(true)
	if err != nil {
		return err
	}
	if r.balAccount {
		err = fajnl.GetBalAcc(genJnlLine)
		if err != nil {
			return err
		}
		last := gtables.NewGenJournalLine(run)
		last.SetRange("journal_template_name", r.genJnlTemplate)
		last.SetRange("journal_batch_name", r.genJnlBatch)
		if last.FindLast() {
			r.genJnlNextLineNo = last.Line_no
		}
	}
	return nil
}

// insertFAJnlLine (InsertFAJnlLine): the index line in the FA journal of the book
func (r *indexFixedAssets) insertFAJnlLine(run *Run, faNo types.Code, indexAmount types.Decimal, postingType gtables.FAJournalLineFa_posting_type) error {
	if indexAmount.IsZero() {
		return nil
	}
	var err error
	if r.firstFAJnl {
		gtables.NewFAJournalLine(run).LockTable()
		r.faJnlTemplate, r.faJnlBatch, r.faJnlNextLineNo, err = tables.FAJnlName(run, r.deprBook.Code)
		if err != nil {
			return err
		}
		r.noSeries3 = tables.GetFANoSeries(run, r.faJnlTemplate, r.faJnlBatch)
		r.documentNo3, err = r.documentNoOrNext(run, func() (types.Code, error) {
			return tables.GetFAJnlDocumentNo(run, r.faJnlTemplate, r.faJnlBatch, r.faPostingDate)
		})
		if err != nil {
			return err
		}
		r.firstFAJnl = false
	}
	faJnlLine := gtables.NewFAJournalLine(run)
	faJnlLine.Init()
	faJnlLine.Journal_template_name = r.faJnlTemplate
	faJnlLine.Journal_batch_name = r.faJnlBatch
	faJnlLine.Posting_date = r.postingDate
	faJnlLine.Fa_posting_date = r.faPostingDate
	if faJnlLine.Posting_date.Equal(faJnlLine.Fa_posting_date) {
		faJnlLine.Posting_date = types.Date{}
	}
	faJnlLine.Fa_posting_type = postingType
	err = faJnlLine.Validate("fa_no", faNo)
	if err != nil {
		return err
	}
	faJnlLine.Document_no = r.documentNo3
	faJnlLine.Posting_no_series = r.noSeries3
	faJnlLine.Description = r.postingDescription
	err = faJnlLine.Validate("depreciation_book_code", r.deprBook.Code)
	if err != nil {
		return err
	}
	err = faJnlLine.Validate("amount", indexAmount)
	if err != nil {
		return err
	}
	faJnlLine.Index_entry = true
	r.faJnlNextLineNo += 10000
	faJnlLine.Line_no = r.faJnlNextLineNo
	return faJnlLine.InsertOrError(true)
}

// Report 5691 Index Insurance: each fixed asset's coverage per insurance up to the posting date
// (the sum of its Ins. Coverage Ledger Entries) is indexed: a line of amount × (Index Figure /
// 100 − 1), marked Index Entry, in the insurance journal of the FA Setup's Insurance Depr.
// Book; the user posts them. Blocked and inactive assets and blocked insurances are left out.
const IndexInsuranceID = 5691

func init() {
	Register(IndexInsuranceID, "index-insurance", func() Report { return &indexInsurance{} })
}

type indexInsurance struct {
	postingDate             types.Date
	indexFigure             types.Decimal
	documentNo              types.Code
	noSeries                types.Code
	postingDescription      types.Text
	templateName, batchName types.Code
	nextLineNo              int
	window                  *system.Window
}

func (r *indexInsurance) Definition() *Definition {
	return &Definition{
		Options: []Option{
			{Name: "index_figure", Type: "number", Required: true},
			{Name: "posting_date", Type: "date", Required: true},
			{Name: "document_no", Type: "text"},
			{Name: "posting_description", Type: "text"},
		},
		Transaction: true,
		OnPreReport: r.onPreReport,
		DataItems: []*DataItem{{
			Name: "Fixed_Asset", Table: gtables.FixedAssetTableName, Key: []string{"no"},
			RequestFilterFields: []string{"no", "fa_class_code", "fa_subclass_code"},
			OnAfterGetRecord:    Trigger(r.onAfterGetRecord),
			OnPostDataItem: func(rec any, run *Run) error {
				r.window.Close()
				return nil
			},
		}},
	}
}

func (r *indexInsurance) onPreReport(run *Run) error {
	r.postingDate = run.OptionDate("posting_date")
	if r.postingDate.IsZero() {
		return system.Error(run.Text("ENTER_POSTING_DATE"))
	}
	if r.postingDate.IsClosing() {
		return system.Error(run.Text("POSTING_DATE_CLOSING"))
	}
	r.indexFigure = run.OptionDecimal("index_figure")
	if r.indexFigure.Equal(types.NewDecimalFromInt(100)) {
		return system.Error(run.Text("INDEX_FIGURE_100"))
	}
	if !r.indexFigure.IsPositive() {
		return system.Error(run.Text("INDEX_FIGURE_POSITIVE"))
	}
	faSetup := gtables.GetFASetup(run)
	err := faSetup.TestField("insurance_depr_book")
	if err != nil {
		return err
	}
	deprBook := gtables.NewDepreciationBook(run)
	err = deprBook.GetOrError(faSetup.Insurance_depr_book)
	if err != nil {
		return err
	}
	gtables.NewInsuranceJournalLine(run).LockTable()
	r.templateName, r.batchName, r.nextLineNo, err = tables.InsuranceJnlName(run, deprBook.Code)
	if err != nil {
		return err
	}
	r.noSeries = tables.GetInsuranceNoSeries(run, r.templateName, r.batchName)
	r.documentNo = types.Code(system.UpperCase(run.Option("document_no")))
	if r.documentNo == "" {
		r.documentNo, err = tables.GetInsuranceJnlDocumentNo(run, r.templateName, r.batchName, r.postingDate)
		if err != nil {
			return err
		}
		if r.documentNo == "" {
			return system.Error(run.Text("ENTER_DOCUMENT_NO"))
		}
	}
	r.postingDescription = types.Text(run.Option("posting_description"))
	r.window = system.OpenWindow(run.Text("WINDOW"))
	return nil
}

func (r *indexInsurance) onAfterGetRecord(fa *gtables.FixedAsset, run *Run) error {
	if fa.Blocked || fa.Inactive {
		return Skip()
	}
	r.window.Update(1, fa.No)
	insCoverageLedgEntry := gtables.NewInsCoverageLedgerEntry(run)
	insCoverageLedgEntry.SetCurrentKey("fa_no", "insurance_no", "disposed_fa", "posting_date")
	insCoverageLedgEntry.SetRange("fa_no", fa.No)
	insCoverageLedgEntry.SetRange("posting_date", types.Date{}, r.postingDate)
	insuranceNos := map[types.Code]bool{} // InsuranceTmp
	var insNos []types.Code
	for found := insCoverageLedgEntry.FindSet(); found; found = insCoverageLedgEntry.Next() {
		if !insuranceNos[insCoverageLedgEntry.Insurance_no] {
			insuranceNos[insCoverageLedgEntry.Insurance_no] = true
			insNos = append(insNos, insCoverageLedgEntry.Insurance_no)
		}
	}
	hundred := types.NewDecimalFromInt(100)
	one := types.NewDecimalFromInt(1)
	for _, insuranceNo := range insNos {
		insCoverageLedgEntry.SetRange("insurance_no", insuranceNo)
		err := insCoverageLedgEntry.CalcSums("amount")
		if err != nil {
			return err
		}
		insCoverageLedgEntry.SetRange("insurance_no")
		if insCoverageLedgEntry.Amount.IsZero() {
			continue
		}
		insurance := gtables.NewInsurance(run)
		if !insurance.Get(insuranceNo) || insurance.Blocked {
			return Skip()
		}
		insuranceJnlLine := gtables.NewInsuranceJournalLine(run)
		insuranceJnlLine.Init()
		insuranceJnlLine.Journal_template_name = r.templateName
		insuranceJnlLine.Journal_batch_name = r.batchName
		insuranceJnlLine.Posting_date = r.postingDate
		err = insuranceJnlLine.Validate("insurance_no", insuranceNo)
		if err != nil {
			return err
		}
		err = insuranceJnlLine.Validate("fa_no", fa.No)
		if err != nil {
			return err
		}
		err = insuranceJnlLine.Validate("amount", system.Round(insCoverageLedgEntry.Amount.Mul(r.indexFigure.Div(hundred).Sub(one)), types.MustDecimal("0.01")))
		if err != nil {
			return err
		}
		insuranceJnlLine.Document_no = r.documentNo
		insuranceJnlLine.Posting_no_series = r.noSeries
		insuranceJnlLine.Description = r.postingDescription
		insuranceJnlLine.Index_entry = true
		r.nextLineNo += 10000
		insuranceJnlLine.Line_no = r.nextLineNo
		err = insuranceJnlLine.InsertOrError(true)
		if err != nil {
			return err
		}
	}
	return nil
}
