package consolidation

import (
	"sort"

	"github.com/hansjlachmann/openerp/backend/business-logic/genjnl"
	"github.com/hansjlachmann/openerp/backend/business-logic/tables"
	fcodeunits "github.com/hansjlachmann/openerp/backend/foundation/codeunits"
	"github.com/hansjlachmann/openerp/backend/foundation/i18n"
	"github.com/hansjlachmann/openerp/backend/foundation/system"
	ftables "github.com/hansjlachmann/openerp/backend/foundation/tables"
	"github.com/hansjlachmann/openerp/backend/foundation/types"
	gtables "github.com/hansjlachmann/openerp/backend/generated/tables"
)

// MaxDays is the most days a consolidation per day takes in (classic ARRAYLEN(RoundResiduals))
const MaxDays = 500

// Method is how an account's amounts are translated from the business unit's currency
// (classic ',Current Rate,Historical Rate,Manual Rate')
type Method int

const (
	CurrentRate    Method = 1 // at the balance currency factor, the difference to the income factor an exchange rate adjustment
	HistoricalRate Method = 2 // the additional-currency amounts, as they are
	ManualRate     Method = 3 // at the income currency factor
)

// AccountMethod is the method of an account: its Consol. Currency Val. Method, Automatic being the
// current rate for the balance sheet and the (average) manual rate for the income statement
func AccountMethod(method gtables.GLAccountConsol_currency_val_method, incomeBalance gtables.GLAccountIncome_balance) Method {
	if method == gtables.GLAccount_Consol_currency_val_method.Automatic {
		if incomeBalance == gtables.GLAccount_Income_balance.BalanceSheet {
			return CurrentRate
		}
		return ManualRate
	}
	return Method(method)
}

// Group is the G/L entries of an account summed for one consolidation line: per day, per
// business unit and department, per project — as Transfer per Day/Department/Project ask
type Group struct {
	PostingDate      types.Date
	BusinessUnitCode types.Code
	DepartmentCode   types.Code
	ProjectCode      types.Code
	DebitAmount      types.Decimal
	CreditAmount     types.Decimal
	AcyDebitAmount   types.Decimal // Add.-Currency Debit Amount
	AcyCreditAmount  types.Decimal // Add.-Currency Credit Amount
}

// Groups sums an account's G/L entries in the filters of glEntry as the classic loop does
// (SETRANGE on the fields of the current entry, CALCSUMS, FIND('+')): by posting date with
// perDay, by business unit and department with perDept or perProj, by project with perProj or
// with perDept and perDay — in the order of the entries
func Groups(glEntry *gtables.GLEntry, perDay, perDept, perProj bool) []*Group {
	if perDept || perProj {
		glEntry.SetCurrentKey("gl_account_no", "business_unit_code", "department_code", "project_code", "posting_date")
	} else {
		glEntry.SetCurrentKey("gl_account_no", "posting_date")
	}
	byProject := perProj || perDept && perDay
	var groups []*Group
	index := map[Group]*Group{}
	for found := glEntry.FindSet(); found; found = glEntry.Next() {
		key := Group{}
		if perDay {
			key.PostingDate = glEntry.Posting_date
		}
		if perDept || perProj {
			key.BusinessUnitCode = glEntry.Business_unit_code
			key.DepartmentCode = glEntry.Department_code
		}
		if byProject {
			key.ProjectCode = glEntry.Project_code
		}
		group, ok := index[key]
		if !ok {
			group = &Group{PostingDate: key.PostingDate, BusinessUnitCode: key.BusinessUnitCode, DepartmentCode: key.DepartmentCode, ProjectCode: key.ProjectCode}
			index[key] = group
			groups = append(groups, group)
		}
		group.DebitAmount = group.DebitAmount.Add(glEntry.Debit_amount)
		group.CreditAmount = group.CreditAmount.Add(glEntry.Credit_amount)
		group.AcyDebitAmount = group.AcyDebitAmount.Add(glEntry.Add_currency_debit_amount)
		group.AcyCreditAmount = group.AcyCreditAmount.Add(glEntry.Add_currency_credit_amount)
	}
	return groups
}

// Cleared is what removing the earlier consolidation of business units took out of the G/L,
// per posting date (classic DeletedAmounts/DeletedDates)
type Cleared struct {
	dates   []types.Date
	amounts map[types.Date]types.Decimal
}

// add (UpdateAmountArray) adds an entry's amount to its date
func (c *Cleared) add(postingDate types.Date, amount types.Decimal) error {
	if c.amounts == nil {
		c.amounts = map[types.Date]types.Decimal{}
	}
	_, ok := c.amounts[postingDate]
	if !ok {
		if len(c.dates) == MaxDays {
			ts := i18n.GetInstance()
			return system.Error(ts.Message("CONSOL_MAX_DELETED_DATES", fcodeunits.CurrentLanguage()), MaxDays)
		}
		c.dates = append(c.dates, postingDate)
	}
	c.amounts[postingDate] = c.amounts[postingDate].Add(amount)
	return nil
}

// Check (CheckAmountArray): the cleared entries add up to 0 on each date — otherwise someone
// posted across business units by hand and clearing would unbalance the G/L
func (c *Cleared) Check() error {
	for _, date := range c.dates {
		amount := c.amounts[date]
		if !amount.IsZero() {
			ts := i18n.GetInstance()
			return system.Error(ts.Message("CONSOL_CANNOT_ERASE", fcodeunits.CurrentLanguage()), amount.Neg(), date)
		}
	}
	return nil
}

// Consolidation posts the G/L of a business unit into the company: its entries on the
// consolidation accounts at the business unit's share and currency factors, the adjustment of
// the opening balance sheet to the new balance factor, exchange rate differences and rounding
// residuals. The journal lines are collected (Lines) and posted together at the end (Post).
type Consolidation struct {
	Src        ftables.Source
	BusUnit    *gtables.BusinessUnit
	DocNo      types.Code
	SourceCode types.Code
	// the consolidation period of the request page; the business unit's own Starting and
	// Ending Date (UseBusUnitDates) take precedence
	StartDate, EndDate       types.Date
	UseBusUnitDates          bool
	PerDay, PerDept, PerProj bool
	// MapDepartment / MapProject: the department or project an entry's code consolidates to
	// (the subsidiary's Consolidation Code, or the code of an import file checked here)
	MapDepartment func(code types.Code) (types.Code, error)
	MapProject    func(code types.Code) (types.Code, error)

	lines              []*gtables.GenJournalLine
	exchRateAdjAmount  types.Decimal
	exchRateAdjAmounts map[int]types.Decimal
	roundResiduals     map[int]types.Decimal
}

// ownDates: the business unit's Starting and Ending Date set the period
func (c *Consolidation) ownDates() bool {
	return c.UseBusUnitDates && (!c.BusUnit.Starting_date.IsZero() || !c.BusUnit.Ending_date.IsZero())
}

// PeriodStart is the first day consolidated
func (c *Consolidation) PeriodStart() types.Date {
	if c.ownDates() {
		return c.BusUnit.Starting_date
	}
	return c.StartDate
}

// PeriodEnd is the last day consolidated (the posting date of what is not posted per day)
func (c *Consolidation) PeriodEnd() types.Date {
	if c.ownDates() {
		return c.BusUnit.Ending_date
	}
	return c.EndDate
}

// CheckPeriod: the period is given and, per day, not longer than MaxDays
func CheckPeriod(busUnit *gtables.BusinessUnit, startDate, endDate types.Date, useBusUnitDates, perDay bool) error {
	ts := i18n.GetInstance()
	lang := fcodeunits.CurrentLanguage()
	if useBusUnitDates && (!busUnit.Starting_date.IsZero() || !busUnit.Ending_date.IsZero()) {
		err := busUnit.TestField("starting_date")
		if err != nil {
			return err
		}
		err = busUnit.TestField("ending_date")
		if err != nil {
			return err
		}
		if perDay && busUnit.Starting_date.DaysBetween(busUnit.Ending_date)+1 > MaxDays {
			return system.Error(ts.Message("CONSOL_BU_MAX_DAYS", lang), busUnit.Code, MaxDays)
		}
		return nil
	}
	if startDate.IsZero() {
		return system.Error(ts.Message("CONSOL_ENTER_STARTING_DATE", lang))
	}
	if endDate.IsZero() {
		return system.Error(ts.Message("CONSOL_ENTER_ENDING_DATE", lang))
	}
	if perDay && startDate.DaysBetween(endDate)+1 > MaxDays {
		return system.Error(ts.Message("CONSOL_MAX_DAYS", lang), MaxDays)
	}
	return nil
}

// ClearPrevious takes the earlier consolidation of the business unit in the period out of the
// G/L: its entries get amount 0 and a description saying so; the amounts go to cleared
func (c *Consolidation) ClearPrevious(cleared *Cleared) error {
	ts := i18n.GetInstance()
	lang := fcodeunits.CurrentLanguage()
	consolidGLEntry := gtables.NewGLEntry(c.Src)
	consolidGLEntry.SetCurrentKey("gl_account_no", "business_unit_code", "department_code", "project_code", "posting_date")
	consolidGLEntry.SetRange("business_unit_code", c.BusUnit.Code)
	consolidGLEntry.SetRange("posting_date", c.PeriodStart(), c.PeriodEnd())
	for found := consolidGLEntry.FindSetBuffered(); found; found = consolidGLEntry.Next() {
		err := cleared.add(consolidGLEntry.Posting_date, consolidGLEntry.Amount)
		if err != nil {
			return err
		}
		changed := gtables.NewGLEntry(c.Src)
		changed.Copy(consolidGLEntry)
		changed.Description = types.Text(system.CopyStr(system.StrSubstNo(ts.Message("CONSOL_CHANGED_BY", lang), system.WorkDate()), 1, changed.MaxStrLen("description")))
		changed.Amount = types.Decimal{}
		changed.Debit_amount = types.Decimal{}
		changed.Credit_amount = types.Decimal{}
		changed.Additional_currency_amount = types.Decimal{}
		changed.Add_currency_debit_amount = types.Decimal{}
		changed.Add_currency_credit_amount = types.Decimal{}
		err = changed.ModifyOrError(false)
		if err != nil {
			return err
		}
	}
	return nil
}

// newLine is a journal line of the business unit's consolidation
func (c *Consolidation) newLine() *gtables.GenJournalLine {
	genJnlLine := gtables.NewGenJournalLine(c.Src)
	genJnlLine.Init()
	genJnlLine.Account_type = gtables.GenJournalLine_Account_type.GLAccount
	genJnlLine.Business_unit_code = c.BusUnit.Code
	genJnlLine.Document_no = c.DocNo
	genJnlLine.Source_code = c.SourceCode
	genJnlLine.System_created_entry = true
	return genJnlLine
}

// setAmount (VALIDATE(Amount)) sets the line's amount: in the local currency, with its debit and
// credit
func setAmount(genJnlLine *gtables.GenJournalLine, amount types.Decimal) error {
	return genJnlLine.Validate("amount", amount)
}

// add (GenJnlPostLineTmp) collects a line for Post
func (c *Consolidation) add(genJnlLine *gtables.GenJournalLine) {
	c.lines = append(c.lines, genJnlLine)
}

// NeedsBalanceAdjustment: the balance factor changed since the last consolidation, so the
// balance sheet accounts at the current rate are adjusted (classic: else CurrReport.BREAK)
func (c *Consolidation) NeedsBalanceAdjustment() bool {
	last := c.BusUnit.Last_balance_currency_factor
	return !last.IsZero() && !last.Equal(c.BusUnit.Balance_currency_factor)
}

// AdjustBalance (PostBalanceAdjustment): an amount of the business unit's balance on a
// consolidated account, taken in at the last balance factor, is adjusted to the current one
func (c *Consolidation) AdjustBalance(accountNo, departmentCode, projectCode types.Code, amountToPost types.Decimal) error {
	ts := i18n.GetInstance()
	lang := fcodeunits.CurrentLanguage()
	err := c.BusUnit.TestField("balance_currency_factor")
	if err != nil {
		return err
	}
	err = c.BusUnit.TestField("last_balance_currency_factor")
	if err != nil {
		return err
	}
	genJnlLine := c.newLine()
	genJnlLine.Account_no = accountNo
	genJnlLine.Posting_date = c.PeriodEnd()
	genJnlLine.Department_code = departmentCode
	genJnlLine.Project_code = projectCode
	amount := amountToPost.Mul(c.BusUnit.Last_balance_currency_factor).Div(c.BusUnit.Balance_currency_factor).Sub(amountToPost).Round(2)
	genJnlLine.Description = types.Text(system.CopyStr(system.StrSubstNo(ts.Message("CONSOL_ADJUSTED", lang),
		amountToPost, c.BusUnit.Last_balance_currency_factor, c.BusUnit.Balance_currency_factor, system.WorkDate()), 1, genJnlLine.MaxStrLen("description")))
	err = setAmount(genJnlLine, amount)
	if err != nil {
		return err
	}
	if !genJnlLine.Amount.IsZero() {
		c.add(genJnlLine)
	}
	c.exchRateAdjAmount = c.exchRateAdjAmount.Add(genJnlLine.Amount)
	return nil
}

// FinishBalanceAdjustment: the adjustments go against the exchange rate gains or losses
// account; the business unit's last balance factor becomes the current one
func (c *Consolidation) FinishBalanceAdjustment() error {
	ts := i18n.GetInstance()
	lang := fcodeunits.CurrentLanguage()
	c.BusUnit.Last_balance_currency_factor = c.BusUnit.Balance_currency_factor
	err := c.BusUnit.ModifyOrError(false)
	if err != nil {
		return err
	}
	if c.exchRateAdjAmount.IsZero() {
		return nil
	}
	genJnlLine := c.newLine()
	err = setAmount(genJnlLine, c.exchRateAdjAmount.Neg())
	if err != nil {
		return err
	}
	err = c.gainsOrLossesAccount(genJnlLine)
	if err != nil {
		return err
	}
	genJnlLine.Posting_date = c.PeriodEnd()
	genJnlLine.Description = types.Text(system.CopyStr(system.StrSubstNo(ts.Message("CONSOL_OPENING_ADJUSTMENT", lang), system.WorkDate()), 1, genJnlLine.MaxStrLen("description")))
	c.add(genJnlLine)
	c.exchRateAdjAmount = types.Decimal{}
	return nil
}

// gainsOrLossesAccount: a credit goes to the exchange rate gains account, a debit to the losses
func (c *Consolidation) gainsOrLossesAccount(genJnlLine *gtables.GenJournalLine) error {
	if genJnlLine.Amount.IsNegative() {
		err := c.BusUnit.TestField("exch_rate_gains_acc")
		if err != nil {
			return err
		}
		genJnlLine.Account_no = c.BusUnit.Exch_rate_gains_acc
		return nil
	}
	err := c.BusUnit.TestField("exch_rate_loss_acc")
	if err != nil {
		return err
	}
	genJnlLine.Account_no = c.BusUnit.Exch_rate_loss_acc
	return nil
}

// SubsidAccount is an account of the business unit: where it consolidates to and how
type SubsidAccount struct {
	No              types.Code
	ConsolDebitAcc  types.Code
	ConsolCreditAcc types.Code
	Method          Method
}

// dayIndex is the day of the period a date is (1 …), 1 when not per day
func (c *Consolidation) dayIndex(postingDate types.Date) int {
	if !c.PerDay {
		return 1
	}
	return c.PeriodStart().DaysBetween(postingDate) + 1
}

// dayDate is the posting date of a day of the period (the period's end when not per day)
func (c *Consolidation) dayDate(index int) types.Date {
	if !c.PerDay {
		return c.PeriodEnd()
	}
	return c.PeriodStart().AddDays(index - 1)
}

// PostEntry (PostGenJnlLine): a group of the account's entries as a line on its consolidation
// account (credit: the Consol. Credit Acc., debit: the Consol. Debit Acc.) at the business
// unit's share, translated by the account's method
func (c *Consolidation) PostEntry(acc SubsidAccount, group *Group, amountToPost types.Decimal) error {
	ts := i18n.GetInstance()
	lang := fcodeunits.CurrentLanguage()
	if c.exchRateAdjAmounts == nil {
		c.exchRateAdjAmounts = map[int]types.Decimal{}
		c.roundResiduals = map[int]types.Decimal{}
	}
	genJnlLine := c.newLine()
	subsidGLAcc := gtables.NewGLAccount(c.Src)
	subsidGLAcc.No = acc.No
	if amountToPost.IsNegative() {
		if acc.ConsolCreditAcc == "" {
			return subsidGLAcc.TestField("consol_credit_acc")
		}
		genJnlLine.Account_no = acc.ConsolCreditAcc
	} else {
		if acc.ConsolDebitAcc == "" {
			return subsidGLAcc.TestField("consol_debit_acc")
		}
		genJnlLine.Account_no = acc.ConsolDebitAcc
	}
	consolidGLAcc := gtables.NewGLAccount(c.Src)
	err := consolidGLAcc.GetOrError(genJnlLine.Account_no)
	if err != nil {
		return err
	}
	pctCaption := c.BusUnit.FieldCaption("consolidation_pct", lang)
	consolidAmount := amountToPost.Mul(c.BusUnit.Consolidation_pct).Div(types.NewDecimalFromInt(100))
	exchRates := tables.ExchangeRates(c.Src)
	var amount, incomeAmount types.Decimal
	var description string
	switch acc.Method {
	case ManualRate:
		if c.BusUnit.Currency_code == "" {
			amount = consolidAmount.Round(2)
			description = system.StrSubstNo(ts.Message("CONSOL_AT_PCT", lang), amountToPost, c.BusUnit.Consolidation_pct, pctCaption)
		} else {
			amount, err = exchRates.ExchangeAmtFCYToLCY(system.WorkDate(), c.BusUnit.Currency_code, consolidAmount, c.BusUnit.Income_currency_factor)
			if err != nil {
				return err
			}
			amount = amount.Round(2)
			description = system.StrSubstNo(ts.Message("CONSOL_AT_AVERAGE_RATE", lang), consolidAmount, c.BusUnit.Income_currency_factor, system.WorkDate())
		}
	case CurrentRate:
		if c.BusUnit.Currency_code == "" {
			amount = consolidAmount.Round(2)
			incomeAmount = amount
			description = system.StrSubstNo(ts.Message("CONSOL_AT_PCT", lang), amountToPost, c.BusUnit.Consolidation_pct, pctCaption)
		} else {
			amount, err = exchRates.ExchangeAmtFCYToLCY(system.WorkDate(), c.BusUnit.Currency_code, consolidAmount, c.BusUnit.Balance_currency_factor)
			if err != nil {
				return err
			}
			amount = amount.Round(2)
			incomeAmount, err = exchRates.ExchangeAmtFCYToLCY(system.WorkDate(), c.BusUnit.Currency_code, consolidAmount, c.BusUnit.Income_currency_factor)
			if err != nil {
				return err
			}
			incomeAmount = incomeAmount.Round(2)
			description = system.StrSubstNo(ts.Message("CONSOL_AT_CURRENT_RATE", lang), consolidAmount, c.BusUnit.Balance_currency_factor, system.WorkDate())
		}
	case HistoricalRate:
		amount = consolidAmount.Round(2)
		description = system.StrSubstNo(ts.Message("CONSOL_AT_HISTORICAL_RATE", lang), amountToPost, c.BusUnit.Consolidation_pct, pctCaption)
	}
	genJnlLine.Description = types.Text(system.CopyStr(description, 1, genJnlLine.MaxStrLen("description")))
	err = setAmount(genJnlLine, amount)
	if err != nil {
		return err
	}
	i := c.dayIndex(group.PostingDate)
	genJnlLine.Posting_date = c.dayDate(i)
	if c.PerDept && group.DepartmentCode != "" {
		genJnlLine.Department_code, err = c.MapDepartment(group.DepartmentCode)
		if err != nil {
			return err
		}
	}
	if c.PerProj && group.ProjectCode != "" {
		genJnlLine.Project_code, err = c.MapProject(group.ProjectCode)
		if err != nil {
			return err
		}
	}
	if !genJnlLine.Amount.IsZero() {
		c.add(genJnlLine)
		c.roundResiduals[i] = c.roundResiduals[i].Add(genJnlLine.Amount)
	}
	// balance including exchange rate differences
	difference := incomeAmount.Sub(genJnlLine.Amount)
	if acc.Method == CurrentRate && !difference.IsZero() {
		c.exchRateAdjAmounts[i] = c.exchRateAdjAmounts[i].Add(difference)
	}
	return nil
}

// FinishEntries: with another currency or share, the exchange rate differences of each day go
// to the gains or losses account and the rounding residuals to the residual account
func (c *Consolidation) FinishEntries() error {
	ts := i18n.GetInstance()
	lang := fcodeunits.CurrentLanguage()
	one := types.NewDecimalFromInt(1)
	if c.BusUnit.Income_currency_factor.Equal(one) && c.BusUnit.Balance_currency_factor.Equal(one) &&
		c.BusUnit.Consolidation_pct.Equal(types.NewDecimalFromInt(100)) {
		return nil
	}
	endDateIndex := 1
	if c.PerDay {
		endDateIndex = c.PeriodStart().DaysBetween(c.PeriodEnd()) + 1
	}
	for i := 1; i <= endDateIndex; i++ {
		if !c.exchRateAdjAmounts[i].IsZero() {
			genJnlLine := c.newLine()
			err := setAmount(genJnlLine, c.exchRateAdjAmounts[i])
			if err != nil {
				return err
			}
			err = c.gainsOrLossesAccount(genJnlLine)
			if err != nil {
				return err
			}
			genJnlLine.Posting_date = c.dayDate(i)
			genJnlLine.Description = types.Text(system.CopyStr(system.StrSubstNo(ts.Message("CONSOL_EXCH_RATE_ADJUSTMENT", lang), system.WorkDate()), 1, genJnlLine.MaxStrLen("description")))
			c.add(genJnlLine)
			c.roundResiduals[i] = c.roundResiduals[i].Add(genJnlLine.Amount)
		}
		if !c.roundResiduals[i].IsZero() {
			genJnlLine := c.newLine()
			err := setAmount(genJnlLine, c.roundResiduals[i].Neg())
			if err != nil {
				return err
			}
			err = c.BusUnit.TestField("residual_account")
			if err != nil {
				return err
			}
			genJnlLine.Account_no = c.BusUnit.Residual_account
			genJnlLine.Posting_date = c.dayDate(i)
			genJnlLine.Description = types.Text(system.CopyStr(system.StrSubstNo(ts.Message("CONSOL_RESIDUAL", lang), genJnlLine.Posting_date, system.WorkDate()), 1, genJnlLine.MaxStrLen("description")))
			c.add(genJnlLine)
		}
	}
	return nil
}

// Lines are the journal lines collected so far
func (c *Consolidation) Lines() []*gtables.GenJournalLine {
	return c.lines
}

// Post (GenJnlPostLineFinally) posts lines in the order of their posting dates
func Post(src ftables.Source, lines []*gtables.GenJournalLine) error {
	sort.SliceStable(lines, func(i, j int) bool { return lines[i].Posting_date.Before(lines[j].Posting_date) })
	genJnlPostLine := genjnl.NewGenJnlPostLine(src)
	for _, genJnlLine := range lines {
		err := genJnlPostLine.Run(genJnlLine)
		if err != nil {
			return err
		}
	}
	return nil
}
