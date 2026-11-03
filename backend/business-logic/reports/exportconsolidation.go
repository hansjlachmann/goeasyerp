package reports

import (
	"github.com/hansjlachmann/openerp/backend/business-logic/consolidation"
	"github.com/hansjlachmann/openerp/backend/foundation/system"
	ftables "github.com/hansjlachmann/openerp/backend/foundation/tables"
	"github.com/hansjlachmann/openerp/backend/foundation/types"
	gtables "github.com/hansjlachmann/openerp/backend/generated/tables"
)

// Report 91 Export Consolidation: the company's G/L of the consolidation period as a file for
// the consolidated company (Import Consolidation from File) — each posting account with its
// consolidation accounts and method, and its entries summed per day, dimension 1 and dimension 2 as
// asked (the dimension 1 and dimension 2 as their Consolidation Codes). The file format:
// consolidation/file.go.
const ExportConsolidationID = 91

func init() {
	Register(ExportConsolidationID, "export-consolidation", func() Report { return &exportConsolidation{} })
}

type exportConsolidation struct {
	consolidationOptions
	file   consolidation.File
	window *system.Window
}

// consolidationOptions are the options of the consolidation reports' request pages
type consolidationOptions struct {
	startDate, endDate       types.Date
	perDay, perDept, perProj bool
}

// consolidationPeriodOptions: the Consolidation Period, and the transfer per day, dimension 1
// and dimension 2
func consolidationPeriodOptions() []Option {
	return []Option{
		{Name: "starting_date", Type: "date"},
		{Name: "ending_date", Type: "date"},
		{Name: "transfer_per_day", Type: "bool", Default: "false"},
		{Name: "transfer_per_dimension_1", Type: "bool", Default: "false"},
		{Name: "transfer_per_dimension_2", Type: "bool", Default: "false"},
	}
}

// read reads the options
func (o *consolidationOptions) read(run *Run) {
	o.startDate = run.OptionDate("starting_date")
	o.endDate = run.OptionDate("ending_date")
	o.perDay = run.OptionBool("transfer_per_day")
	o.perDept = run.OptionBool("transfer_per_dimension_1")
	o.perProj = run.OptionBool("transfer_per_dimension_2")
}

func (r *exportConsolidation) Definition() *Definition {
	return &Definition{
		OptionsCaption: "OPTIONS",
		Options:        consolidationPeriodOptions(),
		OnPreReport:    r.onPreReport,
		OnPostReport:   r.onPostReport,
		DataItems: []*DataItem{{
			Name:             "G_L_Account",
			Table:            gtables.GLAccountTableName,
			Key:              []string{"no"},
			TableView:        []ftables.FieldFilter{{Field: "account_type", Expression: "0"}},
			OnAfterGetRecord: Trigger(r.onAfterGetRecord),
		}},
	}
}

func (r *exportConsolidation) onPreReport(run *Run) error {
	r.read(run)
	if r.startDate.IsZero() {
		return system.Error(run.Text("ENTER_STARTING_DATE"))
	}
	if r.endDate.IsZero() {
		return system.Error(run.Text("ENTER_ENDING_DATE"))
	}
	if r.perDay && r.startDate.DaysBetween(r.endDate)+1 > consolidation.MaxDays {
		return system.Error(run.Text("MAX_DAYS", consolidation.MaxDays))
	}
	r.file.WriteHeader(consolidation.FileHeader{CompanyName: run.Company, StartDate: r.startDate, EndDate: r.endDate,
		PerDay: r.perDay, PerDept: r.perDept, PerProj: r.perProj})
	r.window = system.OpenWindow(run.Text("WINDOW"))
	return nil
}

func (r *exportConsolidation) onAfterGetRecord(glAcc *gtables.GLAccount, run *Run) error {
	r.window.Update(1, glAcc.No)
	err := glAcc.TestField("consol_debit_acc")
	if err != nil {
		return err
	}
	err = glAcc.TestField("consol_credit_acc")
	if err != nil {
		return err
	}
	method := consolidation.AccountMethod(glAcc.Consol_currency_val_method, glAcc.Income_balance)
	r.file.WriteAccount(consolidation.SubsidAccount{No: glAcc.No, ConsolDebitAcc: glAcc.Consol_debit_acc, ConsolCreditAcc: glAcc.Consol_credit_acc, Method: method})

	glEntry := gtables.NewGLEntry(run)
	glEntry.SetRange("gl_account_no", glAcc.No)
	glEntry.SetRange("posting_date", r.startDate, r.endDate)
	for _, group := range consolidation.Groups(glEntry, r.perDay, r.perDept, r.perProj) {
		debit, credit := group.DebitAmount, group.CreditAmount
		if method == consolidation.HistoricalRate {
			debit, credit = group.AcyDebitAmount, group.AcyCreditAmount
		}
		if debit.IsZero() && credit.IsZero() {
			continue
		}
		postingDate := r.endDate
		if r.perDay {
			postingDate = group.PostingDate
		}
		r.window.Update(2, postingDate)
		var dimension1Code, dimension2Code types.Code
		if r.perDept && group.Dimension1Code != "" {
			dept := gtables.NewDimension1(run)
			err = dept.GetOrError(group.Dimension1Code)
			if err != nil {
				return err
			}
			dimension1Code = dept.Consolidation_code
		}
		if r.perProj && group.Dimension2Code != "" {
			proj := gtables.NewDimension2(run)
			err = proj.GetOrError(group.Dimension2Code)
			if err != nil {
				return err
			}
			dimension2Code = proj.Consolidation_code
		}
		r.file.WriteEntry(glAcc.No, postingDate, debit, credit, dimension1Code, dimension2Code)
	}
	return nil
}

func (r *exportConsolidation) onPostReport(run *Run) error {
	r.window.Close()
	run.WriteFile(run.Text("FILE_NAME", run.Company), r.file.Bytes())
	return nil
}
