package reports

import (
	"github.com/hansjlachmann/openerp/backend/business-logic/consolidation"
	fcodeunits "github.com/hansjlachmann/openerp/backend/foundation/codeunits"
	"github.com/hansjlachmann/openerp/backend/foundation/system"
	ftables "github.com/hansjlachmann/openerp/backend/foundation/tables"
	"github.com/hansjlachmann/openerp/backend/foundation/types"
	gtables "github.com/hansjlachmann/openerp/backend/generated/tables"
)

// Report 90 Import Consolidation from DB: the business units to consolidate get their G/L of the
// consolidation period (or their own Starting and Ending Date) read from their companies — the
// earlier consolidation of the period taken out first, each account's entries posted on its
// consolidation accounts at the unit's share and currency factors, the opening balance sheet
// adjusted to a new balance factor, exchange rate differences and rounding residuals on the
// unit's accounts. All lines are posted at the end, in one transaction.
const ImportConsolidationDBID = 90

func init() {
	Register(ImportConsolidationDBID, "import-consolidation-from-db", func() Report { return &importConsolidationDB{} })
}

type importConsolidationDB struct {
	consolidationOptions
	docNo      types.Code
	sourceCode types.Code
	cleared    consolidation.Cleared
	lines      []*gtables.GenJournalLine
	window     *system.Window
}

func (r *importConsolidationDB) Definition() *Definition {
	return &Definition{
		OptionsCaption: "OPTIONS",
		Options:        append(consolidationPeriodOptions(), Option{Name: "document_no", Type: "text", Required: true}),
		Transaction:    true,
		DataItems: []*DataItem{{
			Name:                "Business_Unit",
			Table:               gtables.BusinessUnitTableName,
			Key:                 []string{"code"},
			TableView:           []ftables.FieldFilter{{Field: "consolidate", Expression: "true"}},
			RequestFilterFields: []string{"code"},
			OnPreDataItem:       Trigger(r.onPreDataItem),
			OnAfterGetRecord:    Trigger(r.onAfterGetRecord),
			OnPostDataItem:      Trigger(r.onPostDataItem),
		}},
	}
}

func (r *importConsolidationDB) onPreDataItem(busUnit *gtables.BusinessUnit, run *Run) error {
	r.read(run)
	r.docNo = types.Code(system.UpperCase(run.Option("document_no")))
	if r.docNo == "" {
		return system.Error(run.Text("ENTER_DOCUMENT_NO"))
	}
	r.sourceCode = gtables.GetSourceCodeSetup(run).Consolidation
	r.window = system.OpenWindow(run.Text("WINDOW"))
	if !r.perDay {
		r.window.Update(3, r.endDate)
	}
	return nil
}

// onAfterGetRecord: the business unit's company is read and its consolidation collected
func (r *importConsolidationDB) onAfterGetRecord(busUnit *gtables.BusinessUnit, run *Run) error {
	r.window.Update(1, busUnit.Code)
	r.window.Update(2, "")
	err := busUnit.TestField("company_name")
	if err != nil {
		return err
	}
	subsidiary := busUnit.Company_name.String()
	subsidGLAcc := gtables.NewGLAccount(run)
	if !subsidGLAcc.ChangeCompany(subsidiary) {
		return system.Error(run.Text("NO_COMPANY", subsidiary, busUnit.Code))
	}
	for _, table := range []string{gtables.GLAccountTableName, gtables.GLEntryTableName, gtables.DepartmentTableName, gtables.ProjectTableName} {
		if !fcodeunits.CanReadInCompany(run.DB, run.DBType, subsidiary, table) {
			return system.Error(run.Text("NO_READ", table, subsidiary))
		}
	}
	err = consolidation.CheckPeriod(busUnit, r.startDate, r.endDate, true, r.perDay)
	if err != nil {
		return err
	}
	c := &consolidation.Consolidation{Src: run, BusUnit: busUnit, DocNo: r.docNo, SourceCode: r.sourceCode,
		StartDate: r.startDate, EndDate: r.endDate, UseBusUnitDates: true, PerDay: r.perDay, PerDept: r.perDept, PerProj: r.perProj,
		MapDepartment: func(code types.Code) (types.Code, error) {
			return subsidiaryConsolidationCode(run, subsidiary, gtables.NewDepartment(run), gtables.NewDepartment(run), code)
		},
		MapProject: func(code types.Code) (types.Code, error) {
			return subsidiaryConsolidationCode(run, subsidiary, gtables.NewProject(run), gtables.NewProject(run), code)
		},
	}
	err = c.ClearPrevious(&r.cleared)
	if err != nil {
		return err
	}

	// the opening balance sheet at the new balance factor
	if c.NeedsBalanceAdjustment() {
		err = r.adjustBalances(run, c, subsidiary)
		if err != nil {
			return err
		}
		err = c.FinishBalanceAdjustment()
		if err != nil {
			return err
		}
	}

	// the business unit's entries of the period on the consolidation accounts
	subsidGLAcc.SetCurrentKey("no")
	subsidGLAcc.SetRange("account_type", gtables.GLAccount_Account_type.Posting)
	for found := subsidGLAcc.FindSetBuffered(); found; found = subsidGLAcc.Next() {
		r.window.Update(2, subsidGLAcc.No)
		acc := consolidation.SubsidAccount{No: subsidGLAcc.No, ConsolDebitAcc: subsidGLAcc.Consol_debit_acc, ConsolCreditAcc: subsidGLAcc.Consol_credit_acc,
			Method: consolidation.AccountMethod(subsidGLAcc.Consol_currency_val_method, subsidGLAcc.Income_balance)}
		glEntry := gtables.NewGLEntry(run)
		glEntry.ChangeCompany(subsidiary)
		glEntry.SetRange("gl_account_no", subsidGLAcc.No)
		glEntry.SetRange("posting_date", c.PeriodStart(), c.PeriodEnd())
		for _, group := range consolidation.Groups(glEntry, r.perDay, r.perDept, r.perProj) {
			if r.perDay {
				r.window.Update(3, group.PostingDate)
			}
			err = postGroup(c, acc, group)
			if err != nil {
				return err
			}
		}
	}
	err = c.FinishEntries()
	if err != nil {
		return err
	}
	r.lines = append(r.lines, c.Lines()...)
	return nil
}

// postGroup posts a group's debit and credit (Historical Rate: in the additional currency)
func postGroup(c *consolidation.Consolidation, acc consolidation.SubsidAccount, group *consolidation.Group) error {
	debit, credit := group.DebitAmount, group.CreditAmount
	if acc.Method == consolidation.HistoricalRate {
		debit, credit = group.AcyDebitAmount, group.AcyCreditAmount
	}
	if !debit.IsZero() {
		err := c.PostEntry(acc, group, debit)
		if err != nil {
			return err
		}
	}
	if !credit.IsZero() {
		return c.PostEntry(acc, group, credit.Neg())
	}
	return nil
}

// adjustBalances: the business unit's balance on each consolidation account valued at the
// current rate (as the subsidiary's accounts consolidating to it say) — per department and
// project when the consolidation is
func (r *importConsolidationDB) adjustBalances(run *Run, c *consolidation.Consolidation, subsidiary string) error {
	consolidGLAcc := gtables.NewGLAccount(run)
	consolidGLAcc.SetCurrentKey("no")
	consolidGLAcc.SetRange("account_type", gtables.GLAccount_Account_type.Posting)
	for found := consolidGLAcc.FindSetBuffered(); found; found = consolidGLAcc.Next() {
		method, err := valuationMethod(run, c.BusUnit, subsidiary, consolidGLAcc.No)
		if err != nil {
			return err
		}
		if method != consolidation.CurrentRate {
			continue
		}
		err = adjustAccountBalance(run, c, consolidGLAcc, r.perDept, r.perProj)
		if err != nil {
			return err
		}
	}
	return nil
}

// adjustAccountBalance: the business unit's debit and credit on the account up to the period's
// end, adjusted (per department, and project, when the consolidation is)
func adjustAccountBalance(run *Run, c *consolidation.Consolidation, consolidGLAcc *gtables.GLAccount, perDept, perProj bool) error {
	if !perDept && !perProj {
		err := consolidGLAcc.SetFlowFilter("business_unit_filter", c.BusUnit.Code.String())
		if err != nil {
			return err
		}
		err = consolidGLAcc.SetFlowFilter("date_filter", ".."+c.PeriodEnd().String())
		if err != nil {
			return err
		}
		consolidGLAcc.CalcFields("debit_amount", "credit_amount")
		if !consolidGLAcc.Debit_amount.IsZero() {
			err = c.AdjustBalance(consolidGLAcc.No, "", "", consolidGLAcc.Debit_amount)
			if err != nil {
				return err
			}
		}
		if !consolidGLAcc.Credit_amount.IsZero() {
			return c.AdjustBalance(consolidGLAcc.No, "", "", consolidGLAcc.Credit_amount.Neg())
		}
		return nil
	}
	glEntry := gtables.NewGLEntry(run)
	glEntry.SetRange("gl_account_no", consolidGLAcc.No)
	glEntry.SetRange("business_unit_code", c.BusUnit.Code)
	glEntry.SetFilter("posting_date", ".."+c.PeriodEnd().String())
	for _, group := range consolidation.Groups(glEntry, false, true, perProj) {
		departmentCode := group.DepartmentCode
		if !perDept {
			departmentCode = ""
		}
		projectCode := group.ProjectCode
		if !perProj {
			projectCode = ""
		}
		if !group.DebitAmount.IsZero() {
			err := c.AdjustBalance(consolidGLAcc.No, departmentCode, projectCode, group.DebitAmount)
			if err != nil {
				return err
			}
		}
		if !group.CreditAmount.IsZero() {
			err := c.AdjustBalance(consolidGLAcc.No, departmentCode, projectCode, group.CreditAmount.Neg())
			if err != nil {
				return err
			}
		}
	}
	return nil
}

// valuationMethod (GetValuationMethod): the method of the subsidiary's accounts that consolidate
// to an account (0 when none does) — they must all have the same
func valuationMethod(run *Run, busUnit *gtables.BusinessUnit, subsidiary string, consolAccNo types.Code) (consolidation.Method, error) {
	var method consolidation.Method
	prevAccNo := types.Code("")
	for _, field := range []string{"consol_debit_acc", "consol_credit_acc"} {
		subsidGLAcc := gtables.NewGLAccount(run)
		subsidGLAcc.ChangeCompany(subsidiary)
		subsidGLAcc.SetRange(field, consolAccNo)
		for found := subsidGLAcc.FindSet(); found; found = subsidGLAcc.Next() {
			valMethod := consolidation.AccountMethod(subsidGLAcc.Consol_currency_val_method, subsidGLAcc.Income_balance)
			if method == 0 {
				method = valMethod
				prevAccNo = subsidGLAcc.No
				continue
			}
			if valMethod != method {
				language := fcodeunits.CurrentLanguage()
				return 0, system.Error(run.Text("DIFFERENT_METHODS", subsidGLAcc.TableCaption(language), subsidGLAcc.FieldCaption("consol_currency_val_method", language),
					busUnit.Company_name, prevAccNo, subsidGLAcc.No, consolAccNo))
			}
		}
	}
	return method, nil
}

// consolidationCoder is a department or project
type consolidationCoder interface {
	ChangeCompany(company string) bool
	GetOrError(primaryKey interface{}) error
}

// subsidiaryConsolidationCode: the code a department or project of the subsidiary consolidates
// to (its Consolidation Code; blank when it has none) — which must be here
func subsidiaryConsolidationCode(run *Run, subsidiary string, subsid, consolid consolidationCoder, code types.Code) (types.Code, error) {
	subsid.ChangeCompany(subsidiary)
	err := subsid.GetOrError(code)
	if err != nil {
		return "", err
	}
	var consolidationCode types.Code
	switch rec := subsid.(type) {
	case *gtables.Department:
		consolidationCode = rec.Consolidation_code
	case *gtables.Project:
		consolidationCode = rec.Consolidation_code
	}
	if consolidationCode == "" {
		return "", nil
	}
	err = consolid.GetOrError(consolidationCode)
	if err != nil {
		return "", err
	}
	return consolidationCode, nil
}

// onPostDataItem: when clearing the earlier consolidation keeps the G/L balanced, all lines
// are posted
func (r *importConsolidationDB) onPostDataItem(busUnit *gtables.BusinessUnit, run *Run) error {
	r.window.Close()
	err := r.cleared.Check()
	if err != nil {
		return err
	}
	return consolidation.Post(run, r.lines)
}
