package reports

import (
	"github.com/hansjlachmann/openerp/backend/foundation/system"
	"github.com/hansjlachmann/openerp/backend/foundation/types"
	gtables "github.com/hansjlachmann/openerp/backend/generated/tables"
)

// Report 96 Copy G/L Budget: the G/L budget entries in the request filters copied as new entries
// — into another budget, account, dimension 1 or 2 when given (Copy to…), dated on by the
// Date Change Formula and multiplied by the Amount Change Factor; the budget copied to is created
// when it does not exist. A processing-only report (G/L Budget Names, the budget matrix).
const CopyGLBudgetID = 96

func init() {
	Register(CopyGLBudgetID, "copy-gl-budget", func() Report { return &copyGLBudget{} })
}

type copyGLBudget struct {
	toGLBudgetEntry      *gtables.GLBudgetEntry
	changeDateExpression string
	factor               types.Decimal
	lastEntryNo          int
	nextEntryNo          int
	copied               int
}

func (r *copyGLBudget) Definition() *Definition {
	return &Definition{
		OptionsCaption: "COPY_TO",
		Options: []Option{
			{Name: "budget_name", Type: "text", Relation: &OptionRelation{Table: gtables.GLBudgetNameTableName, Columns: []string{"name", "description"}}},
			{Name: "gl_account_no", Type: "text", Relation: &OptionRelation{Table: gtables.GLAccountTableName, Columns: []string{"no", "name"}}},
			{Name: "dimension_1_code", Type: "text", Relation: &OptionRelation{Table: gtables.Dimension1TableName, Columns: []string{"code", "name"}}},
			{Name: "dimension_2_code", Type: "text", Relation: &OptionRelation{Table: gtables.Dimension2TableName, Columns: []string{"code", "name"}}},
			{Name: "date_change_formula", Type: "text"},
			{Name: "amount_change_factor", Type: "number"},
		},
		Transaction:  true,
		ContextTable: gtables.GLBudgetNameTableName,
		OnInitReport: r.onInitReport,
		OnPreReport:  r.onPreReport,
		DataItems: []*DataItem{{
			Name:                "GLBudgetEntry",
			Table:               gtables.GLBudgetEntryTableName,
			Key:                 []string{"entry_no"},
			RequestFilterFields: []string{"budget_name", "gl_account_no", "dimension_1_code", "dimension_2_code", "date"},
			OnPreDataItem:       Trigger(r.onPreDataItem),
			OnAfterGetRecord:    Trigger(r.onAfterGetRecord),
			OnPostDataItem: Trigger(func(rec *gtables.GLBudgetEntry, run *Run) error {
				run.Message(run.Text("COPIED", r.copied))
				return nil
			}),
		}},
	}
}

// Run from a budget: its entries are copied
func (r *copyGLBudget) onInitReport(run *Run) error {
	glBudget, ok := run.Context().(*gtables.GLBudgetName)
	if ok && glBudget.Name != "" {
		run.SetRequestFilter("GLBudgetEntry", "budget_name", glBudget.Name.String())
	}
	return nil
}

func (r *copyGLBudget) onPreReport(run *Run) error {
	r.toGLBudgetEntry = gtables.NewGLBudgetEntry(run)
	r.toGLBudgetEntry.Budget_name = types.Code(system.UpperCase(run.Option("budget_name")))
	r.toGLBudgetEntry.Gl_account_no = types.Code(system.UpperCase(run.Option("gl_account_no")))
	r.toGLBudgetEntry.Dimension_1_code = types.Code(system.UpperCase(run.Option("dimension_1_code")))
	r.toGLBudgetEntry.Dimension_2_code = types.Code(system.UpperCase(run.Option("dimension_2_code")))
	r.changeDateExpression = system.UpperCase(run.Option("date_change_formula"))
	if r.changeDateExpression != "" {
		_, err := system.CalcDate(r.changeDateExpression, system.WorkDate())
		if err != nil {
			return err
		}
	}
	r.factor = run.OptionDecimal("amount_change_factor")
	return nil
}

// onPreDataItem: the last entry before copying (the copies are not copied again) and the budget
// copied to
func (r *copyGLBudget) onPreDataItem(rec *gtables.GLBudgetEntry, run *Run) error {
	if system.GuiAllowed() && !system.Confirm(run.Text("CONFIRM_COPY")) {
		return Quit()
	}
	glBudgetEntry2 := gtables.NewGLBudgetEntry(run)
	glBudgetEntry2.LockTable()
	if glBudgetEntry2.FindLast() {
		r.lastEntryNo = glBudgetEntry2.Entry_no
	}
	r.nextEntryNo = r.lastEntryNo + 1
	if r.toGLBudgetEntry.Budget_name != "" {
		glBudget := gtables.NewGLBudgetName(run)
		if !glBudget.Get(r.toGLBudgetEntry.Budget_name) {
			glBudget.Init()
			glBudget.Name = r.toGLBudgetEntry.Budget_name
			err := glBudget.InsertOrError(true)
			if err != nil {
				return err
			}
		}
	}
	return nil
}

func (r *copyGLBudget) onAfterGetRecord(rec *gtables.GLBudgetEntry, run *Run) error {
	if rec.Entry_no > r.lastEntryNo {
		return Break()
	}
	glBudgetEntry2 := gtables.NewGLBudgetEntry(run)
	glBudgetEntry2.Copy(rec)
	if r.toGLBudgetEntry.Budget_name != "" {
		glBudgetEntry2.Budget_name = r.toGLBudgetEntry.Budget_name
	}
	if r.toGLBudgetEntry.Gl_account_no != "" {
		glBudgetEntry2.Gl_account_no = r.toGLBudgetEntry.Gl_account_no
	}
	if r.toGLBudgetEntry.Dimension_1_code != "" {
		glBudgetEntry2.Dimension_1_code = r.toGLBudgetEntry.Dimension_1_code
	}
	if r.toGLBudgetEntry.Dimension_2_code != "" {
		glBudgetEntry2.Dimension_2_code = r.toGLBudgetEntry.Dimension_2_code
	}
	if r.changeDateExpression != "" {
		date, err := system.CalcDate(r.changeDateExpression, rec.Date)
		if err != nil {
			return err
		}
		glBudgetEntry2.Date = date
	}
	if !r.factor.IsZero() {
		glBudgetEntry2.Amount = system.Round(rec.Amount.Mul(r.factor))
	}
	glBudgetEntry2.Entry_no = r.nextEntryNo
	err := glBudgetEntry2.InsertOrError(true)
	if err != nil {
		return err
	}
	r.nextEntryNo++
	r.copied++
	return nil
}
