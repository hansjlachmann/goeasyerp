package reports

import (
	"github.com/hansjlachmann/goeasyerp/backend/foundation/system"
	"github.com/hansjlachmann/goeasyerp/backend/foundation/types"
	gtables "github.com/hansjlachmann/goeasyerp/backend/generated/tables"
)

// Report 16 G/L Consolidation Eliminations: the eliminations of a general journal batch before
// they are posted — for each G/L account the consolidated amount of the other business units, the
// amount of the chosen one (blank: the posted eliminations, the entries without a business unit),
// their total, each journal line on the account and the total with them (a total account: with
// the lines on its Totaling).
const ConsolidationEliminationsID = 16

func init() {
	Register(ConsolidationEliminationsID, "g-l-consolidation-eliminations", func() Report { return &consolidationEliminations{} })
}

type consolidationEliminations struct {
	startDate, endDate        types.Date
	busUnit                   *gtables.BusinessUnit
	template, batch           types.Code
	balanceShown              bool
	consolidAmount, buAmount  types.Decimal
	eliminationAmount, amount types.Decimal
	firstLine                 bool
	glAcc                     *gtables.GLAccount
}

func (r *consolidationEliminations) Definition() *Definition {
	return &Definition{
		OptionsCaption: "CONSOLIDATION_PERIOD",
		Options: []Option{
			{Name: "starting_date", Type: "date", Required: true},
			{Name: "ending_date", Type: "date", Required: true},
			{Name: "business_unit", Type: "text", Relation: &OptionRelation{Table: gtables.BusinessUnitTableName, Columns: []string{"code", "name"}}},
			{Name: "journal_template", Type: "text", Required: true, Relation: &OptionRelation{Table: gtables.GenJournalTemplateTableName, Columns: []string{"name", "description"}}},
			{Name: "journal_batch", Type: "text", Required: true},
			{Name: "show", Type: "option", Default: "1", Options: []string{"NET_CHANGE", "BALANCE"}},
		},
		OnPreReport: r.onPreReport,
		DataItems: []*DataItem{{
			Name:                "G_L_Account",
			Table:               gtables.GLAccountTableName,
			Key:                 []string{"no"},
			RequestFilterFields: []string{"no", "dimension_1_filter", "dimension_2_filter"},
			OnPreDataItem: Trigger(func(rec *gtables.GLAccount, run *Run) error {
				setFilterText(run, rec)
				return nil
			}),
			OnAfterGetRecord: Trigger(r.glAccOnAfterGetRecord),
			DataItems: []*DataItem{
				{
					Name:             "Gen_Journal_Line",
					Table:            gtables.GenJournalLineTableName,
					Key:              []string{"journal_template_name", "journal_batch_name", "line_no"},
					Link:             map[string]string{"account_no": "no"},
					OnPreDataItem:    Trigger(r.lineOnPreDataItem),
					OnAfterGetRecord: Trigger(r.lineOnAfterGetRecord),
				},
				{
					Name:             "Account_Total",
					Table:            IntegerTable,
					OnPreDataItem:    Trigger(func(rec *Integer, run *Run) error { rec.SetRange("number", 1); return nil }),
					OnAfterGetRecord: Trigger(r.totalOnAfterGetRecord),
				},
			},
		}},
	}
}

func (r *consolidationEliminations) onPreReport(run *Run) error {
	var err error
	r.startDate, r.endDate, err = consolidatedPeriod(run)
	if err != nil {
		return err
	}
	r.template = types.Code(system.UpperCase(run.Option("journal_template")))
	r.batch = types.Code(system.UpperCase(run.Option("journal_batch")))
	genJnlBatch := gtables.NewGenJournalBatch(run)
	err = genJnlBatch.GetOrError(map[string]interface{}{"journal_template_name": r.template, "name": r.batch})
	if err != nil {
		return err
	}
	r.busUnit = gtables.NewBusinessUnit(run)
	busUnitCode := types.Code(system.UpperCase(run.Option("business_unit")))
	if busUnitCode != "" {
		err = r.busUnit.GetOrError(busUnitCode)
		if err != nil {
			return err
		}
		run.SetValue("bu_heading", busUnitCode.String())
	} else {
		run.SetValue("bu_heading", run.Text("POSTED_ELIMINATIONS"))
	}
	r.balanceShown = run.OptionInt("show") == 2
	if r.balanceShown {
		run.SetValue("amount_type", run.Text("BALANCE"))
	} else {
		run.SetValue("amount_type", run.Text("NET_CHANGE"))
	}
	run.SetValue("this_company", run.Company)
	run.SetValue("journal_text", run.Text("JOURNAL", r.template, r.batch))
	return nil
}

// glAccOnAfterGetRecord: the business unit's and the others' amounts on the account
func (r *consolidationEliminations) glAccOnAfterGetRecord(glAcc *gtables.GLAccount, run *Run) error {
	r.glAcc = glAcc
	r.firstLine = true
	r.eliminationAmount = types.Decimal{}
	r.amount = types.Decimal{}
	pick := func(netChange, balance types.Decimal) types.Decimal {
		if r.balanceShown {
			return balance
		}
		return netChange
	}
	busUnitFilter, otherFilter := blankBusUnit, "<>''"
	if r.busUnit.Code != "" {
		busUnitFilter, otherFilter = r.busUnit.Code.String(), "<>"+r.busUnit.Code.String()
	}
	startDate, endDate := busUnitPeriod(r.busUnit, r.startDate, r.endDate)
	netChange, balance, err := busUnitAmounts(glAcc, busUnitFilter, startDate, endDate)
	if err != nil {
		return err
	}
	r.buAmount = pick(netChange, balance)
	netChange, balance, err = busUnitAmounts(glAcc, otherFilter, r.startDate, r.endDate)
	if err != nil {
		return err
	}
	r.consolidAmount = pick(netChange, balance)
	run.SetValue("account_no", glAcc.No.String())
	run.SetValue("name", indentedName(glAcc))
	run.SetValue("is_total", isTotalAccount(glAcc))
	return nil
}

// lineOnPreDataItem: the batch's lines with an amount
func (r *consolidationEliminations) lineOnPreDataItem(line *gtables.GenJournalLine, run *Run) error {
	line.SetRange("journal_template_name", r.template)
	line.SetRange("journal_batch_name", r.batch)
	line.SetFilter("amount", "<>0")
	return nil
}

// lineOnAfterGetRecord: the account's amounts on its first line only
func (r *consolidationEliminations) lineOnAfterGetRecord(line *gtables.GenJournalLine, run *Run) error {
	run.SetValue("first_line", r.firstLine)
	r.firstLine = false
	r.amount = r.amount.Add(line.Amount)
	run.SetValue("consolid_amount", r.consolidAmount)
	run.SetValue("bu_amount", r.buAmount)
	run.SetValue("total_amount", r.consolidAmount.Add(r.buAmount))
	return nil
}

// totalOnAfterGetRecord: the eliminations on the account (a total account: on its Totaling)
// and the total with them; a posting account without amounts is not printed
func (r *consolidationEliminations) totalOnAfterGetRecord(rec *Integer, run *Run) error {
	glAcc := r.glAcc
	r.eliminationAmount = r.amount
	if isTotalAccount(glAcc) && glAcc.Totaling != "" {
		genJnlLine2 := gtables.NewGenJournalLine(run)
		genJnlLine2.SetRange("journal_template_name", r.template)
		genJnlLine2.SetRange("journal_batch_name", r.batch)
		genJnlLine2.SetFilter("account_no", glAcc.Totaling.String())
		err := genJnlLine2.CalcSums("amount")
		if err != nil {
			return err
		}
		r.eliminationAmount = genJnlLine2.Amount
	}
	if !isTotalAccount(glAcc) && r.firstLine && r.buAmount.IsZero() && r.consolidAmount.IsZero() {
		return Skip()
	}
	run.SetValue("show_account", r.firstLine)
	run.SetValue("consolid_amount", r.consolidAmount)
	run.SetValue("bu_amount", r.buAmount)
	run.SetValue("total_amount", r.consolidAmount.Add(r.buAmount))
	run.SetValue("elimination_amount", r.eliminationAmount)
	run.SetValue("total_incl", r.consolidAmount.Add(r.buAmount).Add(r.eliminationAmount))
	return nil
}
