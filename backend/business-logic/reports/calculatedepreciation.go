package reports

import (
	"github.com/hansjlachmann/openerp/backend/business-logic/fadepr"
	"github.com/hansjlachmann/openerp/backend/business-logic/fajnl"
	"github.com/hansjlachmann/openerp/backend/business-logic/tables"
	"github.com/hansjlachmann/openerp/backend/foundation/system"
	"github.com/hansjlachmann/openerp/backend/foundation/types"
	gtables "github.com/hansjlachmann/openerp/backend/generated/tables"
)

// Report 5692 Calculate Depreciation: the depreciation (and custom 1) of the fixed assets in a
// depreciation book up to an FA posting date (codeunit 5610), as journal lines — in the general
// journal batch of the book's FA Journal Setup when the book posts the type to the G/L (with Insert
// Bal. Account the balancing lines too), else in its FA journal batch; posted by the user. A
// processing-only report (main menu Fixed Assets → Activities).
const CalculateDepreciationID = 5692

func init() {
	Register(CalculateDepreciationID, "calculate-depreciation", func() Report { return &calculateDepreciation{} })
}

type calculateDepreciation struct {
	deprBook          *gtables.DepreciationBook
	deprUntilDate     types.Date
	postingDate       types.Date
	daysInPeriod      int
	documentNo        types.Code
	postingDesc       types.Text
	balAccount        bool
	faJnlLines        []deprLine // FAJnlLineTmp
	genJnlLines       []deprLine // GenJnlLineTmp
	window            *system.Window
	insertedLineCount int
}

// deprLine: a journal line to insert (the temporary journal lines of the classic report)
type deprLine struct {
	faNo         types.Code
	custom1      bool
	amount       types.Decimal
	numberOfDays int
}

func (r *calculateDepreciation) Definition() *Definition {
	return &Definition{
		OptionsCaption: "OPTIONS",
		Options: []Option{
			{Name: "depreciation_book", Type: "text", Required: true, Relation: &OptionRelation{Table: "Depreciation_Book", Columns: []string{"code", "description"}}},
			{Name: "fa_posting_date", Type: "date", Required: true},
			{Name: "use_force_no_of_days", Type: "bool", Default: "false"},
			{Name: "force_no_of_days", Type: "number", Default: "0"},
			{Name: "posting_date", Type: "date"},
			{Name: "document_no", Type: "text"},
			{Name: "posting_description", Type: "text"},
			{Name: "insert_bal_account", Type: "bool", Default: "false"},
		},
		Transaction:  true,
		OnInitReport: r.onInitReport,
		OnPreReport:  r.onPreReport,
		DataItems: []*DataItem{{
			Name:                "fixed_asset",
			Table:               gtables.FixedAssetTableName,
			Key:                 []string{"no"},
			RequestFilterFields: []string{"no", "fa_class_code", "fa_subclass_code", "budgeted_asset"},
			OnAfterGetRecord:    Trigger(r.faOnAfterGetRecord),
			OnPostDataItem:      Trigger(r.faOnPostDataItem),
		}},
	}
}

// The FA Setup's default depreciation book
func (r *calculateDepreciation) onInitReport(run *Run) error {
	faSetup := gtables.GetFASetup(run)
	run.SetOption("depreciation_book", faSetup.Default_depr_book.String())
	return nil
}

func (r *calculateDepreciation) onPreReport(run *Run) error {
	r.deprBook = gtables.NewDepreciationBook(run)
	err := r.deprBook.GetOrError(types.Code(system.UpperCase(run.Option("depreciation_book"))))
	if err != nil {
		return err
	}
	r.deprUntilDate = run.OptionDate("fa_posting_date")
	if r.deprUntilDate.IsZero() {
		return system.Error(run.Text("ENTER_FA_POSTING_DATE"))
	}
	r.postingDate = run.OptionDate("posting_date")
	if r.postingDate.IsZero() {
		r.postingDate = r.deprUntilDate
	}
	useForceNoOfDays := run.OptionBool("use_force_no_of_days")
	r.daysInPeriod = run.OptionInt("force_no_of_days")
	if !useForceNoOfDays && r.daysInPeriod != 0 {
		return system.Error(run.Text("USE_FORCE_NO_OF_DAYS"))
	}
	if useForceNoOfDays && r.daysInPeriod == 0 {
		return system.Error(run.Text("FORCE_NO_OF_DAYS_ACTIVATED"))
	}
	if r.deprBook.Use_same_fa_g_l_posting_dates && !r.deprUntilDate.Equal(r.postingDate) {
		return system.Error(run.Text("SAME_DATES", r.deprBook.Code))
	}
	r.documentNo = types.Code(system.UpperCase(run.Option("document_no")))
	r.postingDesc = types.Text(run.Option("posting_description"))
	r.balAccount = run.OptionBool("insert_bal_account")
	r.window = system.OpenWindow(run.Text("WINDOW"))
	return nil
}

// faOnAfterGetRecord: the asset's depreciation and custom 1 — a line for the FA journal, or the
// general journal when the book posts the type to the G/L (a budgeted asset: the FA journal)
func (r *calculateDepreciation) faOnAfterGetRecord(fa *gtables.FixedAsset, run *Run) error {
	if fa.Inactive || fa.Blocked {
		return Skip()
	}
	result, err := fadepr.Calculate(run, fa.No, r.deprBook.Code, r.deprUntilDate, fadepr.EntryAmounts{}, types.Date{}, r.daysInPeriod)
	if err != nil {
		return err
	}
	if !result.DeprAmount.IsZero() || !result.Custom1Amount.IsZero() {
		r.window.Update(1, fa.No)
	} else {
		r.window.Update(2, fa.No)
	}
	if !result.Custom1Amount.IsZero() {
		line := deprLine{faNo: fa.No, custom1: true, amount: result.Custom1Amount, numberOfDays: result.Custom1NumberOfDays}
		if !r.deprBook.G_l_integration_custom_1 || fa.Budgeted_asset {
			r.faJnlLines = append(r.faJnlLines, line)
		} else {
			r.genJnlLines = append(r.genJnlLines, line)
		}
	}
	if !result.DeprAmount.IsZero() {
		line := deprLine{faNo: fa.No, amount: result.DeprAmount, numberOfDays: result.NumberOfDays}
		if !r.deprBook.G_l_integration_depreciation || fa.Budgeted_asset {
			r.faJnlLines = append(r.faJnlLines, line)
		} else {
			r.genJnlLines = append(r.genJnlLines, line)
		}
	}
	return nil
}

// faOnPostDataItem: the lines inserted in the journals of the book's FA Journal Setup
func (r *calculateDepreciation) faOnPostDataItem(fa *gtables.FixedAsset, run *Run) error {
	defer r.window.Close()
	if len(r.faJnlLines) > 0 {
		err := r.insertFAJnlLines(run)
		if err != nil {
			return err
		}
	}
	if len(r.genJnlLines) > 0 {
		err := r.insertGenJnlLines(run)
		if err != nil {
			return err
		}
	}
	run.Message(run.Text("LINES_CREATED", r.insertedLineCount))
	return nil
}

// documentNo2: the Document No. option, else the batch series' next number (the batch must be
// empty), else an error
func (r *calculateDepreciation) documentNo2(run *Run, next func() (types.Code, error)) (types.Code, error) {
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

func (r *calculateDepreciation) insertFAJnlLines(run *Run) error {
	templateName, batchName, nextLineNo, err := tables.FAJnlName(run, r.deprBook.Code)
	if err != nil {
		return err
	}
	noSeries := tables.GetFANoSeries(run, templateName, batchName)
	documentNo, err := r.documentNo2(run, func() (types.Code, error) {
		return tables.GetFAJnlDocumentNo(run, templateName, batchName, r.deprUntilDate)
	})
	if err != nil {
		return err
	}
	for _, l := range r.faJnlLines {
		r.insertedLineCount++
		r.window.Update(3, r.insertedLineCount)
		faJnlLine := gtables.NewFAJournalLine(run)
		faJnlLine.Init()
		faJnlLine.Journal_template_name = templateName
		faJnlLine.Journal_batch_name = batchName
		faJnlLine.Posting_date = r.postingDate
		faJnlLine.Fa_posting_date = r.deprUntilDate
		if faJnlLine.Posting_date.Equal(faJnlLine.Fa_posting_date) {
			faJnlLine.Posting_date = types.Date{}
		}
		faJnlLine.Fa_posting_type = gtables.FAJournalLine_Fa_posting_type.Depreciation
		if l.custom1 {
			faJnlLine.Fa_posting_type = gtables.FAJournalLine_Fa_posting_type.Custom1
		}
		err = faJnlLine.Validate("fa_no", l.faNo)
		if err != nil {
			return err
		}
		faJnlLine.Document_no = documentNo
		faJnlLine.Posting_no_series = noSeries
		if r.postingDesc != "" {
			faJnlLine.Description = r.postingDesc
		}
		err = faJnlLine.Validate("depreciation_book_code", r.deprBook.Code)
		if err != nil {
			return err
		}
		err = faJnlLine.Validate("amount", l.amount)
		if err != nil {
			return err
		}
		faJnlLine.No_of_depreciation_days = l.numberOfDays
		nextLineNo += 10000
		faJnlLine.Line_no = nextLineNo
		err = faJnlLine.InsertOrError(true)
		if err != nil {
			return err
		}
	}
	return nil
}

func (r *calculateDepreciation) insertGenJnlLines(run *Run) error {
	templateName, batchName, nextLineNo, err := tables.GenJnlName(run, r.deprBook.Code)
	if err != nil {
		return err
	}
	noSeries := tables.GetGenNoSeries(run, templateName, batchName)
	documentNo, err := r.documentNo2(run, func() (types.Code, error) {
		return tables.GetGenJnlDocumentNo(run, templateName, batchName, r.deprUntilDate)
	})
	if err != nil {
		return err
	}
	for _, l := range r.genJnlLines {
		r.insertedLineCount++
		r.window.Update(3, r.insertedLineCount)
		genJnlLine := gtables.NewGenJournalLine(run)
		genJnlLine.Init()
		genJnlLine.Journal_template_name = templateName
		genJnlLine.Journal_batch_name = batchName
		genJnlLine.Posting_date = r.postingDate
		genJnlLine.Fa_posting_date = r.deprUntilDate
		if genJnlLine.Posting_date.Equal(genJnlLine.Fa_posting_date) {
			genJnlLine.Fa_posting_date = types.Date{}
		}
		genJnlLine.Fa_posting_type = gtables.GenJournalLine_Fa_posting_type.Depreciation
		if l.custom1 {
			genJnlLine.Fa_posting_type = gtables.GenJournalLine_Fa_posting_type.Custom1
		}
		genJnlLine.Account_type = gtables.GenJournalLine_Account_type.FixedAsset
		err = genJnlLine.Validate("account_no", l.faNo)
		if err != nil {
			return err
		}
		if r.postingDesc != "" {
			genJnlLine.Description = r.postingDesc
		}
		genJnlLine.Document_no = documentNo
		genJnlLine.Posting_no_series = noSeries
		err = genJnlLine.Validate("depreciation_book_code", r.deprBook.Code)
		if err != nil {
			return err
		}
		err = genJnlLine.Validate("amount", l.amount)
		if err != nil {
			return err
		}
		genJnlLine.No_of_depreciation_days = l.numberOfDays
		nextLineNo += 10000
		genJnlLine.Line_no = nextLineNo
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
			last.SetRange("journal_template_name", templateName)
			last.SetRange("journal_batch_name", batchName)
			if last.FindLast() {
				nextLineNo = last.Line_no
			}
		}
	}
	return nil
}
