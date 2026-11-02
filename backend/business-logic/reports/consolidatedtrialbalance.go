package reports

import (
	"strings"

	"github.com/hansjlachmann/openerp/backend/foundation/system"
	"github.com/hansjlachmann/openerp/backend/foundation/types"
	gtables "github.com/hansjlachmann/openerp/backend/generated/tables"
)

// Report 17 Consolidated Trial Balance: for each G/L account the net change and balance of each
// business unit in the consolidation period (or its own Starting and Ending Date), their total,
// the eliminations (the entries without a business unit) and the total with them.
//
// Report 18 Consolidated Trial Balance (4): the same with the business units in columns (at most
// four), as net change or balance.
const (
	ConsolidatedTrialBalanceID  = 17
	ConsolidatedTrialBalance4ID = 18
)

func init() {
	Register(ConsolidatedTrialBalanceID, "consolidated-trial-balance", func() Report { return &consolidatedTrialBalance{} })
	Register(ConsolidatedTrialBalance4ID, "consolidated-trial-balance-4", func() Report { return &consolidatedTrialBalance4{} })
}

// consolidatedPeriod reads the Consolidation Period (both dates required) and sets the head's
// period and filters
func consolidatedPeriod(run *Run) (types.Date, types.Date, error) {
	startDate := run.OptionDate("starting_date")
	endDate := run.OptionDate("ending_date")
	if startDate.IsZero() {
		return startDate, endDate, system.Error(run.Text("ENTER_STARTING_DATE"))
	}
	if endDate.IsZero() {
		return startDate, endDate, system.Error(run.Text("ENTER_ENDING_DATE"))
	}
	setReportHeadValues(run)
	setPeriodText(run, startDate.String()+".."+endDate.String())
	run.SetValue("in_thousands", run.OptionBool("in_thousands"))
	return startDate, endDate, nil
}

// busUnitAmounts: an account's net change and balance at date of a business unit filter in a
// period
func busUnitAmounts(glAcc *gtables.GLAccount, busUnitFilter string, startDate, endDate types.Date) (netChange, balance types.Decimal, err error) {
	err = glAcc.SetFlowFilter("business_unit_filter", busUnitFilter)
	if err != nil {
		return
	}
	err = glAcc.SetFlowFilter("date_filter", startDate.String()+".."+endDate.String())
	if err != nil {
		return
	}
	glAcc.CalcFields("net_change", "balance_at_date")
	return glAcc.Net_change, glAcc.Balance_at_date, nil
}

// busUnitPeriod: the business unit's own Starting and Ending Date, else the consolidation period
func busUnitPeriod(busUnit *gtables.BusinessUnit, startDate, endDate types.Date) (types.Date, types.Date) {
	if !busUnit.Starting_date.IsZero() || !busUnit.Ending_date.IsZero() {
		return busUnit.Starting_date, busUnit.Ending_date
	}
	return startDate, endDate
}

// blankBusUnit is the filter of the entries without a business unit (SETRANGE(…, ”))
const blankBusUnit = "''"

// thousands divides an amount by 1000 with Amounts in whole 1000s
func thousands(run *Run, amount types.Decimal) types.Decimal {
	if run.OptionBool("in_thousands") {
		return amount.Div(types.NewDecimalFromInt(1000))
	}
	return amount
}

type busUnitLine struct {
	code               string
	netChange, balance types.Decimal
}

type consolidatedTrialBalance struct {
	startDate, endDate types.Date
	lines              []busUnitLine
}

func (r *consolidatedTrialBalance) Definition() *Definition {
	return &Definition{
		OptionsCaption: "CONSOLIDATION_PERIOD",
		Options: []Option{
			{Name: "starting_date", Type: "date", Required: true},
			{Name: "ending_date", Type: "date", Required: true},
			{Name: "in_thousands", Type: "bool", Default: "false"},
		},
		OnPreReport: func(run *Run) error {
			var err error
			r.startDate, r.endDate, err = consolidatedPeriod(run)
			return err
		},
		DataItems: []*DataItem{{
			Name:                "G_L_Account",
			Table:               gtables.GLAccountTableName,
			Key:                 []string{"no"},
			RequestFilterFields: []string{"no", "department_filter", "project_filter"},
			OnPreDataItem: Trigger(func(rec *gtables.GLAccount, run *Run) error {
				setFilterText(run, rec)
				return nil
			}),
			OnAfterGetRecord: Trigger(r.glAccOnAfterGetRecord),
			DataItems: []*DataItem{
				{
					Name:  "Business_Unit_Line",
					Table: IntegerTable,
					OnPreDataItem: Trigger(func(rec *Integer, run *Run) error {
						if len(r.lines) == 0 {
							return Break()
						}
						rec.SetRange("number", 1, len(r.lines))
						return nil
					}),
					OnAfterGetRecord: Trigger(func(rec *Integer, run *Run) error {
						line := r.lines[rec.Number-1]
						run.SetValue("bu_code", line.code)
						run.SetValue("bu_net_change", line.netChange)
						run.SetValue("bu_balance", line.balance)
						return nil
					}),
				},
				{
					Name:          "Consolidated_Total",
					Table:         IntegerTable,
					OnPreDataItem: Trigger(func(rec *Integer, run *Run) error { rec.SetRange("number", 1); return nil }),
				},
			},
		}},
	}
}

// glAccOnAfterGetRecord: the business units' amounts on the account (those with any), their
// total and the eliminations
func (r *consolidatedTrialBalance) glAccOnAfterGetRecord(glAcc *gtables.GLAccount, run *Run) error {
	run.SetValue("name", indentedName(glAcc))
	run.SetValue("is_total", isTotalAccount(glAcc))
	r.lines = nil
	var netChange, balance types.Decimal
	busUnit := gtables.NewBusinessUnit(run)
	busUnit.SetRange("consolidate", true)
	for found := busUnit.FindSet(); found; found = busUnit.Next() {
		startDate, endDate := busUnitPeriod(busUnit, r.startDate, r.endDate)
		buNetChange, buBalance, err := busUnitAmounts(glAcc, busUnit.Code.String(), startDate, endDate)
		if err != nil {
			return err
		}
		if buNetChange.IsZero() && buBalance.IsZero() {
			continue
		}
		buNetChange, buBalance = thousands(run, buNetChange), thousands(run, buBalance)
		r.lines = append(r.lines, busUnitLine{code: strings.Repeat("  ", glAcc.Indentation+1) + busUnit.Code.String(), netChange: buNetChange, balance: buBalance})
		netChange = netChange.Add(buNetChange)
		balance = balance.Add(buBalance)
	}
	eliminations, _, err := busUnitAmounts(glAcc, blankBusUnit, r.startDate, r.endDate)
	if err != nil {
		return err
	}
	eliminations = thousands(run, eliminations)
	run.SetValue("net_change", netChange)
	run.SetValue("balance", balance)
	run.SetValue("eliminations", eliminations)
	run.SetValue("net_change_incl", netChange.Add(eliminations))
	run.SetValue("balance_incl", balance.Add(eliminations))
	return nil
}

type consolidatedTrialBalance4 struct {
	startDate, endDate types.Date
	busUnits           []*gtables.BusinessUnit
}

// maxBusUnitColumns: the business units the report has columns for
const maxBusUnitColumns = 4

func (r *consolidatedTrialBalance4) Definition() *Definition {
	return &Definition{
		OptionsCaption: "CONSOLIDATION_PERIOD",
		Options: []Option{
			{Name: "starting_date", Type: "date", Required: true},
			{Name: "ending_date", Type: "date", Required: true},
			{Name: "show", Type: "option", Default: "1", Options: []string{"NET_CHANGE", "BALANCE"}},
			{Name: "in_thousands", Type: "bool", Default: "false"},
		},
		OnPreReport: r.onPreReport,
		DataItems: []*DataItem{{
			Name:                "G_L_Account",
			Table:               gtables.GLAccountTableName,
			Key:                 []string{"no"},
			RequestFilterFields: []string{"no", "department_filter", "project_filter"},
			OnPreDataItem: Trigger(func(rec *gtables.GLAccount, run *Run) error {
				setFilterText(run, rec)
				if len(r.busUnits) == 0 {
					return Break()
				}
				return nil
			}),
			OnAfterGetRecord: Trigger(r.glAccOnAfterGetRecord),
		}},
	}
}

// onPreReport: the business units to consolidate in the columns
func (r *consolidatedTrialBalance4) onPreReport(run *Run) error {
	var err error
	r.startDate, r.endDate, err = consolidatedPeriod(run)
	if err != nil {
		return err
	}
	if run.OptionInt("show") == 2 {
		run.SetValue("amount_type", run.Text("BALANCE"))
	} else {
		run.SetValue("amount_type", run.Text("NET_CHANGE"))
	}
	busUnit := gtables.NewBusinessUnit(run)
	busUnit.SetRange("consolidate", true)
	for found := busUnit.FindSet(); found; found = busUnit.Next() {
		if len(r.busUnits) == maxBusUnitColumns {
			return system.Error(run.Text("MAX_BUSINESS_UNITS", maxBusUnitColumns))
		}
		column := gtables.NewBusinessUnit(run)
		column.Copy(busUnit)
		r.busUnits = append(r.busUnits, column)
	}
	for i := 0; i < maxBusUnitColumns; i++ {
		code := ""
		if i < len(r.busUnits) {
			code = r.busUnits[i].Code.String()
		}
		run.SetValue("bu_code_"+string(rune('1'+i)), code)
	}
	return nil
}

// glAccOnAfterGetRecord: the business units' amounts in their columns, the eliminations; a
// posting account without amounts is not printed
func (r *consolidatedTrialBalance4) glAccOnAfterGetRecord(glAcc *gtables.GLAccount, run *Run) error {
	balanceShown := run.OptionInt("show") == 2
	pick := func(netChange, balance types.Decimal) types.Decimal {
		if balanceShown {
			return balance
		}
		return netChange
	}
	var total types.Decimal
	anyAmount := false
	for i := 0; i < maxBusUnitColumns; i++ {
		var amount types.Decimal
		if i < len(r.busUnits) {
			startDate, endDate := busUnitPeriod(r.busUnits[i], r.startDate, r.endDate)
			netChange, balance, err := busUnitAmounts(glAcc, r.busUnits[i].Code.String(), startDate, endDate)
			if err != nil {
				return err
			}
			amount = thousands(run, pick(netChange, balance))
		}
		run.SetValue("amount_"+string(rune('1'+i)), amount)
		total = total.Add(amount)
		anyAmount = anyAmount || !amount.IsZero()
	}
	netChange, balance, err := busUnitAmounts(glAcc, blankBusUnit, r.startDate, r.endDate)
	if err != nil {
		return err
	}
	eliminations := thousands(run, pick(netChange, balance))
	if !isTotalAccount(glAcc) && !anyAmount && eliminations.IsZero() {
		return Skip()
	}
	run.SetValue("name", indentedName(glAcc))
	run.SetValue("is_total", isTotalAccount(glAcc))
	run.SetValue("total", total)
	run.SetValue("eliminations", eliminations)
	run.SetValue("total_incl", total.Add(eliminations))
	return nil
}
