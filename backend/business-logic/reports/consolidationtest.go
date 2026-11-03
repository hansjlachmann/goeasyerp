package reports

import (
	"github.com/hansjlachmann/openerp/backend/business-logic/consolidation"
	fcodeunits "github.com/hansjlachmann/openerp/backend/foundation/codeunits"
	"github.com/hansjlachmann/openerp/backend/foundation/system"
	ftables "github.com/hansjlachmann/openerp/backend/foundation/tables"
	"github.com/hansjlachmann/openerp/backend/foundation/types"
	gtables "github.com/hansjlachmann/openerp/backend/generated/tables"
)

// Report 14 Consolidation - Test Database and report 15 Consolidation - Test File: what the
// consolidation of a business unit would post to that is not in this company — the
// consolidation accounts of its accounts with entries in the period, the consolidation codes
// of its dimension 1 and 2 values — listed per business unit (one without problems is not
// printed). Report 14 reads the business units' companies, report 15 a consolidation file.
const (
	ConsolidationTestDatabaseID = 14
	ConsolidationTestFileID     = 15
)

func init() {
	Register(ConsolidationTestDatabaseID, "consolidation-test-database", func() Report { return &consolidationTest{} })
	Register(ConsolidationTestFileID, "consolidation-test-file", func() Report { return &consolidationTest{fromFile: true} })
}

type consolidationTest struct {
	consolidationOptions
	fromFile bool
	header   consolidation.FileHeader
	lines    []consolidation.FileLine
	fileUnit types.Code // the business unit of the file
	missing  []consolidation.Missing
	check    *consolidation.Check
	window   *system.Window
}

func (r *consolidationTest) Definition() *Definition {
	def := &Definition{
		OptionsCaption: "OPTIONS",
		Options:        consolidationPeriodOptions(),
		OnPreReport:    r.onPreReport,
		OnPostReport: func(run *Run) error {
			r.window.Close()
			return nil
		},
	}
	if r.fromFile {
		def.Options = []Option{{Name: "file", Type: "file", Required: true}}
	}
	busUnit := &DataItem{
		Name:             "Business_Unit",
		Table:            gtables.BusinessUnitTableName,
		Key:              []string{"code"},
		TableView:        []ftables.FieldFilter{{Field: "consolidate", Expression: "true"}},
		OnPreDataItem:    Trigger(r.busUnitOnPreDataItem),
		OnAfterGetRecord: Trigger(r.busUnitOnAfterGetRecord),
		DataItems:        []*DataItem{r.missingLoop("Missing_Account", (*consolidation.Check).Accounts), r.missingLoop("Missing_Dimension_1", (*consolidation.Check).Dimension1Values), r.missingLoop("Missing_Dimension_2", (*consolidation.Check).Dimension2Values)},
	}
	if !r.fromFile {
		busUnit.RequestFilterFields = []string{"code"}
	}
	def.DataItems = []*DataItem{busUnit}
	return def
}

// missingLoop is the list of one kind of what is missing
func (r *consolidationTest) missingLoop(name string, list func(*consolidation.Check) []consolidation.Missing) *DataItem {
	return &DataItem{
		Name:  name,
		Table: IntegerTable,
		OnPreDataItem: Trigger(func(rec *Integer, run *Run) error {
			r.missing = list(r.check)
			if len(r.missing) == 0 {
				return Break()
			}
			rec.SetRange("number", 1, len(r.missing))
			return nil
		}),
		OnAfterGetRecord: Trigger(func(rec *Integer, run *Run) error {
			missing := r.missing[rec.Number-1]
			run.SetValue("missing_code", missing.Code.String())
			from := ""
			if missing.From != "" {
				from = "(" + missing.From.String() + ")"
			}
			run.SetValue("missing_from", from)
			return nil
		}),
	}
}

func (r *consolidationTest) onPreReport(run *Run) error {
	setReportHeadValues(run)
	run.SetValue("this_company", run.Company)
	if r.fromFile {
		header, lines, busUnit, err := readConsolidationFile(run)
		if err != nil {
			return err
		}
		r.header, r.lines, r.fileUnit = header, lines, busUnit.Code
		r.startDate, r.endDate = header.StartDate, header.EndDate
		r.perDay, r.perDept, r.perProj = header.PerDay, header.PerDept, header.PerProj
		run.SetValue("file_company", header.CompanyName)
	} else {
		r.read(run)
	}
	run.SetValue("per_day", r.perDay)
	run.SetValue("per_dept", r.perDept)
	run.SetValue("per_proj", r.perProj)
	r.window = system.OpenWindow(run.Text("TEST_WINDOW"))
	return nil
}

func (r *consolidationTest) busUnitOnPreDataItem(busUnit *gtables.BusinessUnit, run *Run) error {
	if r.fromFile {
		busUnit.SetRange("code", r.fileUnit)
		return nil
	}
	setFilterText(run, busUnit)
	return nil
}

// busUnitOnAfterGetRecord checks the business unit; one without problems is not printed
// (PrintOnlyIfDetail)
func (r *consolidationTest) busUnitOnAfterGetRecord(busUnit *gtables.BusinessUnit, run *Run) error {
	r.window.Update(1, busUnit.Code)
	r.check = consolidation.NewCheck()
	var err error
	if r.fromFile {
		err = r.checkFile(run)
	} else {
		err = r.checkDatabase(run, busUnit)
	}
	if err != nil {
		return err
	}
	if len(r.check.Accounts()) == 0 && len(r.check.Dimension1Values()) == 0 && len(r.check.Dimension2Values()) == 0 {
		return Skip()
	}
	run.SetValue("accounts_heading", run.Text("ACCOUNTS_NOT_HERE", run.Company, busUnit.Code))
	run.SetValue("dimension_1_heading", run.Text("DIMENSION_1_NOT_HERE", run.Company, busUnit.Code))
	run.SetValue("dimension_2_heading", run.Text("DIMENSION_2_NOT_HERE", run.Company, busUnit.Code))
	run.SetValue("period_start", r.startDate)
	run.SetValue("period_end", r.endDate)
	if !r.fromFile && (!busUnit.Starting_date.IsZero() || !busUnit.Ending_date.IsZero()) {
		run.SetValue("period_start", busUnit.Starting_date)
		run.SetValue("period_end", busUnit.Ending_date)
	}
	return nil
}

// checkDatabase (CheckConsistency): the business unit's company, its accounts' groups of entries
// in the period that consolidate
func (r *consolidationTest) checkDatabase(run *Run, busUnit *gtables.BusinessUnit) error {
	err := busUnit.TestField("company_name")
	if err != nil {
		return err
	}
	subsidiary := busUnit.Company_name.String()
	subsidGLAcc := gtables.NewGLAccount(run)
	if !subsidGLAcc.ChangeCompany(subsidiary) {
		return system.Error(run.Text("NO_COMPANY", subsidiary, busUnit.Code))
	}
	for _, table := range []string{gtables.GLAccountTableName, gtables.GLEntryTableName, gtables.Dimension1TableName, gtables.Dimension2TableName} {
		if !fcodeunits.CanReadInCompany(run.DB, run.DBType, subsidiary, table) {
			return system.Error(run.Text("NO_READ", table, subsidiary))
		}
	}
	err = consolidation.CheckPeriod(busUnit, r.startDate, r.endDate, true, false)
	if err != nil {
		return err
	}
	c := &consolidation.Consolidation{BusUnit: busUnit, StartDate: r.startDate, EndDate: r.endDate, UseBusUnitDates: true}
	subsidGLAcc.SetCurrentKey("no")
	for found := subsidGLAcc.FindSet(); found; found = subsidGLAcc.Next() {
		r.window.Update(2, subsidGLAcc.No)
		glEntry := gtables.NewGLEntry(run)
		glEntry.ChangeCompany(subsidiary)
		glEntry.SetRange("gl_account_no", subsidGLAcc.No)
		glEntry.SetRange("posting_date", c.PeriodStart(), c.PeriodEnd())
		for _, group := range consolidation.Groups(glEntry, r.perDay, r.perDept, r.perProj) {
			amount := group.DebitAmount.Sub(group.CreditAmount)
			if amount.IsZero() {
				continue
			}
			field, consolAccNo := "consol_debit_acc", subsidGLAcc.Consol_debit_acc
			if amount.IsNegative() {
				field, consolAccNo = "consol_credit_acc", subsidGLAcc.Consol_credit_acc
			}
			err = subsidGLAcc.TestField(field)
			if err != nil {
				return err
			}
			if !gtables.NewGLAccount(run).Get(consolAccNo) {
				r.check.Account(consolAccNo, subsidGLAcc.No)
			}
			if r.perDept && group.Dimension1Code != "" {
				dept := gtables.NewDimension1(run)
				dept.ChangeCompany(subsidiary)
				err = dept.GetOrError(group.Dimension1Code)
				if err != nil {
					return err
				}
				if dept.Consolidation_code != "" && !gtables.NewDimension1(run).Get(dept.Consolidation_code) {
					r.check.Dimension1(dept.Consolidation_code, dept.Code)
				}
			}
			if r.perProj && group.Dimension2Code != "" {
				proj := gtables.NewDimension2(run)
				proj.ChangeCompany(subsidiary)
				err = proj.GetOrError(group.Dimension2Code)
				if err != nil {
					return err
				}
				if proj.Consolidation_code != "" && !gtables.NewDimension2(run).Get(proj.Consolidation_code) {
					r.check.Dimension2(proj.Consolidation_code, proj.Code)
				}
			}
		}
	}
	return nil
}

// checkFile (CheckConsistency): the file's entries — their consolidation accounts, dimension 1 values
// and dimension 2 values
func (r *consolidationTest) checkFile(run *Run) error {
	var acc *consolidation.SubsidAccount
	for i, line := range r.lines {
		if line.Account != nil {
			acc = line.Account
			r.window.Update(2, acc.No)
			continue
		}
		entry := line.Entry
		if acc == nil || entry.AccountNo != acc.No {
			return system.Error(run.Text("E_LINE_INCONSISTENT", entry.AccountNo, i+2))
		}
		if !entry.DebitAmount.IsZero() && !gtables.NewGLAccount(run).Get(acc.ConsolDebitAcc) {
			r.check.Account(acc.ConsolDebitAcc, acc.No)
		}
		if !entry.CreditAmount.IsZero() && !gtables.NewGLAccount(run).Get(acc.ConsolCreditAcc) {
			r.check.Account(acc.ConsolCreditAcc, acc.No)
		}
		if entry.Dimension1Code != "" && !gtables.NewDimension1(run).Get(entry.Dimension1Code) {
			r.check.Dimension1(entry.Dimension1Code, "")
		}
		if entry.Dimension2Code != "" && !gtables.NewDimension2(run).Get(entry.Dimension2Code) {
			r.check.Dimension2(entry.Dimension2Code, "")
		}
	}
	return nil
}
