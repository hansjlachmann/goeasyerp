package reports

import (
	"github.com/hansjlachmann/openerp/backend/business-logic/noseries"
	"github.com/hansjlachmann/openerp/backend/business-logic/tables"
	"github.com/hansjlachmann/openerp/backend/foundation/system"
	ftables "github.com/hansjlachmann/openerp/backend/foundation/tables"
	"github.com/hansjlachmann/openerp/backend/foundation/types"
	gtables "github.com/hansjlachmann/openerp/backend/generated/tables"
)

// Report 94 Close Income Statement: after the fiscal year is closed (Fiscal Year-Close), the
// balances of its income statement accounts — per business unit, department and project as
// asked — go to journal lines that clear them on the year's closing date (C31-12), with the
// total on the retained earnings account. The lines are written into a general journal batch;
// posting them closes the income statement, and reports up to the year's last day (..31-12)
// still show the year's result.
//
// Not migrated yet: the additional reporting currency (with one, the classic posts the lines
// at once).
const CloseIncomeStatementID = 94

func init() {
	Register(CloseIncomeStatementID, "close-income-statement", func() Report { return &closeIncomeStatement{} })
}

type closeIncomeStatement struct {
	fiscalYearStartDate, fiscYearClosingDate types.Date
	genJnlBatch                              *gtables.GenJournalBatch
	genJnlLine                               *gtables.GenJournalLine
	sourceCode                               types.Code
	retainedEarningsAcc                      types.Code
	perBusUnit, perDept, perProj             bool
	totalAmount                              types.Decimal
	window                                   *system.Window
}

func (r *closeIncomeStatement) Definition() *Definition {
	return &Definition{
		OptionsCaption: "OPTIONS",
		Options: []Option{
			{Name: "fiscal_year_ending_date", Type: "date", Required: true},
			{Name: "journal_template", Type: "text", Required: true, Relation: &OptionRelation{Table: gtables.GenJournalTemplateTableName, Columns: []string{"name", "description"}}},
			{Name: "journal_batch", Type: "text", Required: true},
			{Name: "document_no", Type: "text"},
			{Name: "retained_earnings_acc", Type: "text", Required: true, Relation: &OptionRelation{Table: gtables.GLAccountTableName, Columns: []string{"no", "name"}}},
			{Name: "posting_description", Type: "text"},
			{Name: "close_by_business_unit", Type: "bool", Default: "false"},
			{Name: "close_by_department", Type: "bool", Default: "false"},
			{Name: "close_by_project", Type: "bool", Default: "false"},
		},
		OnInitReport: r.onInitReport,
		OnPreReport:  r.onPreReport,
		OnPostReport: func(run *Run) error {
			r.window.Close()
			run.Message(run.Text("DONE", r.genJnlBatch.Journal_template_name, r.genJnlBatch.Name))
			return nil
		},
		Transaction: true,
		DataItems: []*DataItem{{
			Name:  "G_L_Account",
			Table: gtables.GLAccountTableName,
			Key:   []string{"no"},
			TableView: []ftables.FieldFilter{
				{Field: "account_type", Expression: "0"},
				{Field: "income_balance", Expression: "0"},
			},
			OnAfterGetRecord: Trigger(r.glAccOnAfterGetRecord),
			OnPostDataItem:   Trigger(r.glAccOnPostDataItem),
		}},
	}
}

// onInitReport (OnOpenForm): the description is the report's name; the ending date is the end
// of the last closed fiscal year
func (r *closeIncomeStatement) onInitReport(run *Run) error {
	run.SetOption("posting_description", run.Text("TITLE"))
	accountingPeriod := gtables.NewAccountingPeriod(run)
	accountingPeriod.SetRange("new_fiscal_year", true)
	accountingPeriod.SetRange("date_locked", true)
	if accountingPeriod.FindLast() {
		endDate := accountingPeriod.Starting_date.AddDays(-1)
		_, _, err := r.validateEndDate(run, endDate)
		if err == nil {
			run.SetOption("fiscal_year_ending_date", endDate.String())
		}
	}
	return nil
}

// validateEndDate (ValidateEndDate): the date ends a fiscal year (the next day starts a new
// one) that is closed; the year's first day and its closing date
func (r *closeIncomeStatement) validateEndDate(run *Run, endDate types.Date) (types.Date, types.Date, error) {
	accountingPeriod := gtables.NewAccountingPeriod(run)
	if !accountingPeriod.Get(endDate.AddDays(1)) || !accountingPeriod.New_fiscal_year {
		return types.Date{}, types.Date{}, system.Error(run.Text("NO_FISCAL_YEAR"))
	}
	if !accountingPeriod.Date_locked {
		return types.Date{}, types.Date{}, system.Error(run.Text("YEAR_NOT_CLOSED"))
	}
	accountingPeriod.SetRange("new_fiscal_year", true)
	if !accountingPeriod.Find("<") {
		return types.Date{}, types.Date{}, system.Error(run.Text("NO_FISCAL_YEAR"))
	}
	return accountingPeriod.Starting_date, endDate.Closing(), nil
}

func (r *closeIncomeStatement) onPreReport(run *Run) error {
	endDate := run.OptionDate("fiscal_year_ending_date")
	if endDate.IsZero() {
		return system.Error(run.Text("ENTER_ENDING_DATE"))
	}
	var err error
	r.fiscalYearStartDate, r.fiscYearClosingDate, err = r.validateEndDate(run, endDate.Normal())
	if err != nil {
		return err
	}
	r.genJnlBatch = gtables.NewGenJournalBatch(run)
	err = r.genJnlBatch.GetOrError(map[string]interface{}{
		"journal_template_name": types.Code(system.UpperCase(run.Option("journal_template"))),
		"name":                  types.Code(system.UpperCase(run.Option("journal_batch")))})
	if err != nil {
		return err
	}
	docNo := types.Code(system.UpperCase(run.Option("document_no")))
	if docNo == "" && r.genJnlBatch.No_series != "" {
		// ValidateJnl: the batch's next number
		docNo = noseries.New(run).TryGetNextNo(r.genJnlBatch.No_series, endDate)
	}
	if docNo == "" {
		return system.Error(run.Text("ENTER_DOCUMENT_NO"))
	}
	r.retainedEarningsAcc = types.Code(system.UpperCase(run.Option("retained_earnings_acc")))
	retainedEarningsGLAcc := gtables.NewGLAccount(run)
	err = retainedEarningsGLAcc.GetOrError(r.retainedEarningsAcc)
	if err != nil {
		return err
	}
	err = tables.CheckGLAcc(retainedEarningsGLAcc)
	if err != nil {
		return err
	}
	r.sourceCode = gtables.GetSourceCodeSetup(run).Close_income_statement
	r.perBusUnit = run.OptionBool("close_by_business_unit")
	r.perDept = run.OptionBool("close_by_department")
	r.perProj = run.OptionBool("close_by_project")

	// the batch's last line: the new ones follow it
	r.genJnlLine = gtables.NewGenJournalLine(run)
	r.genJnlLine.SetRange("journal_template_name", r.genJnlBatch.Journal_template_name)
	r.genJnlLine.SetRange("journal_batch_name", r.genJnlBatch.Name)
	if !r.genJnlLine.FindLast() {
		r.genJnlLine.Journal_template_name = r.genJnlBatch.Journal_template_name
		r.genJnlLine.Journal_batch_name = r.genJnlBatch.Name
		r.genJnlLine.Line_no = 0
	}
	r.genJnlLine.Init()
	r.genJnlLine.Posting_date = r.fiscYearClosingDate
	r.genJnlLine.Document_no = docNo
	r.genJnlLine.Description = types.Text(system.CopyStr(run.Option("posting_description"), 1, r.genJnlLine.MaxStrLen("description")))
	r.window = system.OpenWindow(run.Text("WINDOW"))
	return nil
}

// closingGroup is an account's entries closed by one line
type closingGroup struct {
	busUnit, dept, proj types.Code
	amount              types.Decimal
}

// glAccOnAfterGetRecord: the account's entries of the fiscal year (up to its closing date) per
// business unit, department and project as asked, each balance cleared by a line
func (r *closeIncomeStatement) glAccOnAfterGetRecord(glAcc *gtables.GLAccount, run *Run) error {
	glEntry := gtables.NewGLEntry(run)
	if r.perBusUnit || r.perDept || r.perProj {
		glEntry.SetCurrentKey("gl_account_no", "business_unit_code", "department_code", "project_code", "posting_date")
	} else {
		glEntry.SetCurrentKey("gl_account_no", "posting_date")
	}
	glEntry.SetRange("gl_account_no", glAcc.No)
	glEntry.SetRange("posting_date", r.fiscalYearStartDate, r.fiscYearClosingDate)
	var groups []*closingGroup
	index := map[closingGroup]*closingGroup{}
	for found := glEntry.FindSet(); found; found = glEntry.Next() {
		key := closingGroup{}
		if r.perBusUnit {
			key.busUnit = glEntry.Business_unit_code
		}
		if r.perDept {
			key.dept = glEntry.Department_code
		}
		if r.perProj {
			key.proj = glEntry.Project_code
		}
		group, ok := index[key]
		if !ok {
			group = &closingGroup{busUnit: key.busUnit, dept: key.dept, proj: key.proj}
			index[key] = group
			groups = append(groups, group)
		}
		group.amount = group.amount.Add(glEntry.Amount)
	}
	for _, group := range groups {
		r.totalAmount = r.totalAmount.Add(group.amount)
		if group.amount.IsZero() {
			continue
		}
		r.genJnlLine.Business_unit_code = group.busUnit
		r.genJnlLine.Department_code = group.dept
		r.genJnlLine.Project_code = group.proj
		err := r.insertLine(run, glAcc.No, group.amount.Neg())
		if err != nil {
			return err
		}
	}
	return nil
}

// glAccOnPostDataItem: the total goes to the retained earnings account
func (r *closeIncomeStatement) glAccOnPostDataItem(glAcc *gtables.GLAccount, run *Run) error {
	if r.totalAmount.IsZero() {
		return nil
	}
	r.genJnlLine.Business_unit_code = ""
	r.genJnlLine.Department_code = ""
	r.genJnlLine.Project_code = ""
	r.genJnlLine.Currency_code = ""
	return r.insertLine(run, r.retainedEarningsAcc, r.totalAmount)
}

// insertLine (HandleGenJnlLine) writes the next line of the batch
func (r *closeIncomeStatement) insertLine(run *Run, accountNo types.Code, amount types.Decimal) error {
	genJnlLine := r.genJnlLine
	genJnlLine.Line_no += 10000
	genJnlLine.Account_type = gtables.GenJournalLine_Account_type.GLAccount
	genJnlLine.Account_no = accountNo
	genJnlLine.Source_code = r.sourceCode
	genJnlLine.Reason_code = r.genJnlBatch.Reason_code
	err := genJnlLine.Validate("amount", amount)
	if err != nil {
		return err
	}
	r.window.Update(1, genJnlLine.Account_no)
	r.window.Update(2, genJnlLine.Amount)
	r.window.Update(3, genJnlLine.Business_unit_code)
	r.window.Update(4, genJnlLine.Department_code)
	r.window.Update(5, genJnlLine.Project_code)
	return genJnlLine.InsertOrError(false)
}
