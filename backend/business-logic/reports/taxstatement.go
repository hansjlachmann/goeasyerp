package reports

import (
	"github.com/hansjlachmann/goeasyerp/backend/business-logic/taxstatement"
	ftables "github.com/hansjlachmann/goeasyerp/backend/foundation/tables"
	"github.com/hansjlachmann/goeasyerp/backend/foundation/types"
	gtables "github.com/hansjlachmann/goeasyerp/backend/generated/tables"
)

// Report 12 Tax Statement: the printed tax statements in the filters — each row to print with its
// amount (taxstatement.Calculator: the Date Filter of the lines, the tax entries chosen by Include
// Tax Entries and the period choice, Round to Whole Numbers, Show Amounts in Add. Reporting
// Currency), with Print With's sign. Run from the Tax Statement page (317) it starts on that
// page's statement.
const TaxStatementID = 12

func init() {
	Register(TaxStatementID, "tax-statement", func() Report { return &taxStatementReport{} })
}

type taxStatementReport struct {
	calc *taxstatement.Calculator
}

func (r *taxStatementReport) Definition() *Definition {
	return &Definition{
		OptionsCaption: "OPTIONS",
		Options: []Option{
			{Name: "include_tax_entries", Type: "option", Default: "1", Options: []string{"OPEN", "CLOSED", "OPEN_AND_CLOSED"}},
			{Name: "period", Type: "option", Default: "1", Options: []string{"BEFORE_AND_WITHIN_PERIOD", "WITHIN_PERIOD"}},
			{Name: "round_to_whole_numbers", Type: "bool", Default: "false"},
			{Name: "show_amounts_in_add_currency", Type: "bool", Default: "false"},
		},
		ContextTable: gtables.TaxStatementLineTableName,
		OnInitReport: r.onInitReport,
		OnPreReport:  r.onPreReport,
		DataItems: []*DataItem{{
			Name:                "TaxStatementName",
			Table:               gtables.TaxStatementNameTableName,
			Key:                 []string{"statement_template_name", "name"},
			RequestFilterFields: []string{"statement_template_name", "name"},
			DataItems: []*DataItem{{
				Name:                "TaxStatementLine",
				Table:               gtables.TaxStatementLineTableName,
				Key:                 []string{"statement_template_name", "statement_name", "line_no"},
				Link:                map[string]string{"statement_template_name": "statement_template_name", "statement_name": "name"},
				TableView:           []ftables.FieldFilter{{Field: "print", Expression: "true"}},
				RequestFilterFields: []string{"date_filter"},
				OnPreDataItem:       Trigger(r.lineOnPreDataItem),
				OnAfterGetRecord:    Trigger(r.lineOnAfterGetRecord),
			}},
		}},
	}
}

// The statement of the Tax Statement page it runs from, or of the Tax Statement Names line it
// is printed from (Print on page 320)
func (r *taxStatementReport) onInitReport(run *Run) error {
	taxStmtName, ok := run.Context().(*gtables.TaxStatementName)
	if ok {
		run.SetRequestFilter("TaxStatementName", "statement_template_name", taxStmtName.Statement_template_name.String())
		run.SetRequestFilter("TaxStatementName", "name", taxStmtName.Name.String())
		return nil
	}
	line, ok := run.Context().(*gtables.TaxStatementLine)
	if !ok {
		return nil
	}
	template, name := line.Statement_template_name, line.Statement_name
	line.FilterGroup(2)
	if template == "" {
		template = types.Code(line.GetFilter("statement_template_name"))
	}
	if name == "" {
		name = types.Code(line.GetFilter("statement_name"))
	}
	line.FilterGroup(0)
	if template != "" {
		run.SetRequestFilter("TaxStatementName", "statement_template_name", template.String())
	}
	if name != "" {
		run.SetRequestFilter("TaxStatementName", "name", name.String())
	}
	return nil
}

func (r *taxStatementReport) onPreReport(run *Run) error {
	setReportHeadValues(run)
	r.calc = &taxstatement.Calculator{Src: run,
		Selection:        taxstatement.Selection(run.OptionInt("include_tax_entries") - 1),
		PeriodSelection:  taxstatement.PeriodSelection(run.OptionInt("period") - 1),
		PrintInIntegers:  run.OptionBool("round_to_whole_numbers"),
		UseAmtsInAddCurr: run.OptionBool("show_amounts_in_add_currency")}
	glSetup := gtables.GetGeneralLedgerSetup(run)
	if r.calc.UseAmtsInAddCurr {
		run.SetValue("amounts_in", run.Text("AMOUNTS_IN", glSetup.Additional_reporting_currency))
		run.SetValue("whole_add_curr", r.calc.PrintInIntegers)
	} else {
		err := glSetup.TestField("lcy_code")
		if err != nil {
			return err
		}
		run.SetValue("amounts_in", run.Text("AMOUNTS_IN", glSetup.Lcy_code))
		run.SetValue("whole_numbers", r.calc.PrintInIntegers)
	}
	run.SetValue("whole_add_curr_text", run.Text("WHOLE_ADD_CURR", glSetup.Additional_reporting_currency))
	run.SetValue("all_entries", r.calc.Selection == taxstatement.SelectionOpenAndClosed)
	run.SetValue("closed_entries", r.calc.Selection == taxstatement.SelectionClosed)
	return nil
}

// lineOnPreDataItem: the period of the lines (their Date Filter) in the head
func (r *taxStatementReport) lineOnPreDataItem(rec *gtables.TaxStatementLine, run *Run) error {
	r.calc.DateFilter = rec.Date_filter
	setPeriodText(run, rec.Date_filter)
	return nil
}

func (r *taxStatementReport) lineOnAfterGetRecord(rec *gtables.TaxStatementLine, run *Run) error {
	amount, err := r.calc.LineTotal(rec)
	if err != nil {
		return err
	}
	if rec.Print_with == gtables.TaxStatementLine_Print_with.OppositeSign {
		amount = amount.Neg()
	}
	run.SetValue("amount", amount)
	run.SetValue("is_description", rec.Type == gtables.TaxStatementLine_Type.Description)
	return nil
}
