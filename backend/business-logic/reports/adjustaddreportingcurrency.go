package reports

import (
	"github.com/hansjlachmann/openerp/backend/business-logic/tables"
	fcodeunits "github.com/hansjlachmann/openerp/backend/foundation/codeunits"
	"github.com/hansjlachmann/openerp/backend/foundation/system"
	ftables "github.com/hansjlachmann/openerp/backend/foundation/tables"
	"github.com/hansjlachmann/openerp/backend/foundation/types"
	gtables "github.com/hansjlachmann/openerp/backend/generated/tables"
)

// Report 86 Adjust Add. Reporting Currency: the General Ledger Setup's Additional Reporting
// Currency is changed here (the field is not edited directly). A new currency recalculates what
// is posted: the open tax entries and every G/L entry get their amounts in the new currency at
// the work date's rate; where a transaction's LCY amounts balance but the exchanged ones do not,
// the difference goes to the currency's residual gains or losses account (an entry of amount 0),
// and so do the closed income statements (the year's additional-currency balance of an account
// whose LCY balance the closing entries cleared goes to the retained earnings account) — in a G/L
// register of the run. Removing the currency only stops new entries from getting the amounts.
//
// Not migrated yet: the item ledger entries' and items' additional-currency costs (they come
// with the items' additional reporting currency), changing the rate on the request page.
const AdjustAddReportingCurrencyID = 86

func init() {
	Register(AdjustAddReportingCurrencyID, "adjust-add-reporting-currency", func() Report { return &adjustAddReportingCurrency{} })
}

type adjustAddReportingCurrency struct {
	addCurrency         types.Code
	currency            *gtables.Currency
	currencyFactor      types.Decimal
	docNo               types.Code
	retainedEarnings    types.Code
	sourceCode          types.Code
	glReg               *gtables.GLRegister
	nextEntryNo         int
	lastEntryNo         int
	nextTransactionNo   int
	fiscalYearStartDate types.Date
	totalAmount         types.Decimal
	totalAddCurrAmount  types.Decimal
	closings            []closingBufferKey
	closingSeen         map[closingBufferKey]bool
	window              *system.Window
}

// closingBufferKey is a closing-date entry's account and codes (Close Income Statement Buffer)
type closingBufferKey struct {
	closingDate         types.Date
	glAccountNo         types.Code
	busUnit, dept, proj types.Code
}

func (r *adjustAddReportingCurrency) Definition() *Definition {
	return &Definition{
		OptionsCaption: "OPTIONS",
		Options: []Option{
			{Name: "additional_reporting_currency", Type: "text", Relation: &OptionRelation{Table: gtables.CurrencyTableName, Columns: []string{"code", "description"}}},
			{Name: "document_no", Type: "text"},
			{Name: "retained_earnings_acc", Type: "text", Relation: &OptionRelation{Table: gtables.GLAccountTableName, Columns: []string{"no", "name"}}},
		},
		Transaction:  true,
		OnInitReport: r.onInitReport,
		OnPreReport:  r.onPreReport,
		OnPostReport: r.onPostReport,
		DataItems: []*DataItem{
			{
				Name:  "Tax_Entry",
				Table: gtables.TaxEntryTableName,
				Key:   []string{"type", "closed", "tax_bus_posting_group", "tax_prod_posting_group", "tax_jurisdiction_code", "use_tax", "posting_date"},
				TableView: []ftables.FieldFilter{{Field: "closed", Expression: "false"},
					{Field: "type", Expression: "1|2"}},
				OnAfterGetRecord: Trigger(r.taxEntryOnAfterGetRecord),
			},
			{
				Name:             "G_L_Entry",
				Table:            gtables.GLEntryTableName,
				Key:              []string{"entry_no"},
				OnPreDataItem:    Trigger(r.glEntryOnPreDataItem),
				OnAfterGetRecord: Trigger(r.glEntryOnAfterGetRecord),
				OnPostDataItem:   Trigger(r.glEntryOnPostDataItem),
			},
		},
	}
}

// onInitReport: the current currency
func (r *adjustAddReportingCurrency) onInitReport(run *Run) error {
	run.SetOption("additional_reporting_currency", gtables.GetGeneralLedgerSetup(run).Additional_reporting_currency.String())
	return nil
}

// onPreReport (the setup field's OnValidate and OnPreReport): removing the currency only after a
// question; a new one needs its rounding and residual accounts, the source code, a document no.
// and the retained earnings account
func (r *adjustAddReportingCurrency) onPreReport(run *Run) error {
	glSetup := gtables.GetGeneralLedgerSetup(run)
	r.addCurrency = types.Code(system.UpperCase(run.Option("additional_reporting_currency")))
	if r.addCurrency == glSetup.Additional_reporting_currency {
		return system.Error(run.Text("UNCHANGED"))
	}
	if glSetup.Additional_reporting_currency != "" {
		question := "CONFIRM_CHANGE"
		if r.addCurrency == "" {
			question = "CONFIRM_DELETE"
		}
		if system.GuiAllowed() && !system.Confirm(run.Text(question)) {
			return Quit()
		}
	}
	if r.addCurrency == "" {
		glSetup.Additional_reporting_currency = ""
		err := glSetup.ModifyOrError(false)
		if err != nil {
			return err
		}
		run.Message(run.Text("DELETED"))
		return Quit()
	}

	r.currency = gtables.NewCurrency(run)
	err := r.currency.GetOrError(r.addCurrency)
	if err != nil {
		return err
	}
	for _, field := range []string{"amount_rounding_precision", "residual_gains_account", "residual_losses_account"} {
		err = r.currency.TestField(field)
		if err != nil {
			return err
		}
	}
	sourceCodeSetup := gtables.GetSourceCodeSetup(run)
	err = sourceCodeSetup.TestField("adjust_add_reporting_currency")
	if err != nil {
		return err
	}
	r.sourceCode = sourceCodeSetup.Adjust_add_reporting_currency
	r.docNo = types.Code(system.UpperCase(run.Option("document_no")))
	if r.docNo == "" {
		return system.Error(run.Text("ENTER_DOCUMENT_NO"))
	}
	r.retainedEarnings = types.Code(system.UpperCase(run.Option("retained_earnings_acc")))
	if r.retainedEarnings == "" {
		return system.Error(run.Text("ENTER_RETAINED_EARNINGS"))
	}
	retainedEarningsGLAcc := gtables.NewGLAccount(run)
	err = retainedEarningsGLAcc.GetOrError(r.retainedEarnings)
	if err != nil {
		return err
	}
	err = tables.CheckGLAcc(retainedEarningsGLAcc)
	if err != nil {
		return err
	}
	r.currencyFactor, err = tables.ExchangeRates(run).ExchangeRate(system.WorkDate(), r.addCurrency)
	if err != nil {
		return err
	}
	return nil
}

// exchangeAmtLCYToFCY (ExchangeAmtUSDToFCY): an LCY amount in the new currency at the work date's
// rate, rounded
func (r *adjustAddReportingCurrency) exchangeAmtLCYToFCY(run *Run, amount types.Decimal) (types.Decimal, error) {
	exchanged, err := tables.ExchangeRates(run).ExchangeAmtLCYToFCY(system.WorkDate(), r.addCurrency, amount, r.currencyFactor)
	if err != nil {
		return types.Decimal{}, err
	}
	return system.Round(exchanged, r.currency.Amount_rounding_precision), nil
}

// taxEntryOnAfterGetRecord: an open tax entry's amounts in the new currency
func (r *adjustAddReportingCurrency) taxEntryOnAfterGetRecord(taxEntry *gtables.TaxEntry, run *Run) error {
	var err error
	exchange := func(amount types.Decimal) types.Decimal {
		if err != nil {
			return types.Decimal{}
		}
		var v types.Decimal
		v, err = r.exchangeAmtLCYToFCY(run, amount)
		return v
	}
	taxEntry.Additional_currency_amount = exchange(taxEntry.Amount)
	taxEntry.Additional_currency_base = exchange(taxEntry.Base)
	taxEntry.Add_currency_unrealized_amt = exchange(taxEntry.Unrealized_amount)
	taxEntry.Add_currency_unrealized_base = exchange(taxEntry.Unrealized_base)
	taxEntry.Add_curr_rem_unreal_amount = exchange(taxEntry.Remaining_unrealized_amount)
	taxEntry.Add_curr_rem_unreal_base = exchange(taxEntry.Remaining_unrealized_base)
	if err != nil {
		return err
	}
	return taxEntry.ModifyOrError(false)
}

func (r *adjustAddReportingCurrency) glEntryOnPreDataItem(glEntry *gtables.GLEntry, run *Run) error {
	r.window = system.OpenWindow(run.Text("WINDOW"))
	r.closingSeen = map[closingBufferKey]bool{}
	last := gtables.NewGLEntry(run)
	if last.FindLast() {
		r.lastEntryNo = last.Entry_no
		r.nextEntryNo = last.Entry_no
		r.nextTransactionNo = last.Transaction_no + 1
	}
	glEntry.SetRange("entry_no", 0, r.lastEntryNo)
	accountingPeriod := gtables.NewAccountingPeriod(run)
	accountingPeriod.SetCurrentKey("closed")
	accountingPeriod.SetRange("closed", false)
	if accountingPeriod.FindFirst() {
		r.fiscalYearStartDate = accountingPeriod.Starting_date
	}
	return nil
}

// glEntryOnAfterGetRecord: the entry's amount in the new currency; when the entries so far
// balance in LCY but not in the new currency, a residual entry; closing-date entries noted for
// the closed income statements
func (r *adjustAddReportingCurrency) glEntryOnAfterGetRecord(glEntry *gtables.GLEntry, run *Run) error {
	r.window.Update(1, glEntry.Posting_date)
	acyAmount, err := r.exchangeAmtLCYToFCY(run, glEntry.Amount)
	if err != nil {
		return err
	}
	glEntry.Additional_currency_amount = acyAmount
	if !glEntry.Debit_amount.IsZero() {
		glEntry.Add_currency_debit_amount = acyAmount
		glEntry.Add_currency_credit_amount = types.Decimal{}
	} else {
		glEntry.Add_currency_debit_amount = types.Decimal{}
		glEntry.Add_currency_credit_amount = acyAmount.Neg()
	}
	err = glEntry.ModifyOrError(false)
	if err != nil {
		return err
	}
	r.totalAddCurrAmount = r.totalAddCurrAmount.Add(acyAmount)
	r.totalAmount = r.totalAmount.Add(glEntry.Amount)
	if r.totalAmount.IsZero() && !r.totalAddCurrAmount.IsZero() {
		account := r.currency.Residual_gains_account
		if r.totalAddCurrAmount.IsNegative() {
			account = r.currency.Residual_losses_account
		}
		// in the transaction it completes (the classic gives it the run's own), so every
		// transaction balances in the new currency too, as codeunit 12 posts them
		err = r.insertGLEntry(run, glEntry.Transaction_no, glEntry.Posting_date, glEntry.Document_date, glEntry.Document_type, glEntry.Document_no, account,
			glEntry.Business_unit_code, "", "", glEntry.Journal_batch_name, glEntry.Reason_code, r.totalAddCurrAmount.Neg())
		if err != nil {
			return err
		}
		r.totalAddCurrAmount = types.Decimal{}
	}
	if glEntry.Posting_date.IsClosing() {
		key := closingBufferKey{closingDate: glEntry.Posting_date, glAccountNo: glEntry.Gl_account_no,
			busUnit: glEntry.Business_unit_code, dept: glEntry.Department_code, proj: glEntry.Project_code}
		if !r.closingSeen[key] {
			r.closingSeen[key] = true
			r.closings = append(r.closings, key)
		}
	}
	return nil
}

// glEntryOnPostDataItem: the closed income statements — for each closing date of a fiscal year
// and account (with its codes), an account whose year balances in LCY but not in the new
// currency gets the difference cleared against the retained earnings account; then the
// residual accounts of the closing dates
func (r *adjustAddReportingCurrency) glEntryOnPostDataItem(glEntry *gtables.GLEntry, run *Run) error {
	residualAccounts := map[closingBufferKey]bool{}
	var residualKeys []closingBufferKey
	for _, key := range r.closings {
		yearStart, ok := r.fiscalYearOfClosingDate(run, key.closingDate)
		if !ok {
			continue
		}
		usedBusUnit, usedDept, usedProj := r.usedCodes(key)
		for _, combo := range [][3]bool{{true, true, true}, {true, true, false}, {true, false, true}, {true, false, false},
			{false, true, true}, {false, true, false}, {false, false, true}, {false, false, false}} {
			err := r.checkCombination(run, combo, usedBusUnit, usedDept, usedProj, key, yearStart)
			if err != nil {
				return err
			}
		}
		for _, account := range []types.Code{r.currency.Residual_gains_account, r.currency.Residual_losses_account} {
			residual := closingBufferKey{closingDate: key.closingDate, glAccountNo: account}
			if !residualAccounts[residual] {
				residualAccounts[residual] = true
				residualKeys = append(residualKeys, residual)
			}
		}
	}
	for _, key := range residualKeys {
		yearStart, _ := r.fiscalYearOfClosingDate(run, key.closingDate)
		err := r.checkCombination(run, [3]bool{false, false, false}, false, false, false, key, yearStart)
		if err != nil {
			return err
		}
	}
	if r.glReg != nil && r.glReg.To_entry_no != 0 {
		return r.glReg.InsertOrError(false)
	}
	return nil
}

// usedCodes: whether the closing entries of the key's date and account carry business units,
// departments, projects
func (r *adjustAddReportingCurrency) usedCodes(key closingBufferKey) (bool, bool, bool) {
	var busUnit, dept, proj bool
	for _, other := range r.closings {
		if other.closingDate.Equal(key.closingDate) && other.glAccountNo == key.glAccountNo {
			busUnit = busUnit || other.busUnit != ""
			dept = dept || other.dept != ""
			proj = proj || other.proj != ""
		}
	}
	return busUnit, dept, proj
}

// fiscalYearOfClosingDate (IsAccountingPeriodClosingDate): a closing date that ends a fiscal year
// (the next day starts a new one) and the year's first day
func (r *adjustAddReportingCurrency) fiscalYearOfClosingDate(run *Run, closingDate types.Date) (types.Date, bool) {
	accountingPeriod := gtables.NewAccountingPeriod(run)
	if !accountingPeriod.Get(closingDate.Normal().AddDays(1)) || !accountingPeriod.New_fiscal_year {
		return types.Date{}, false
	}
	accountingPeriod.SetRange("new_fiscal_year", true)
	if !accountingPeriod.Find("<") {
		return types.Date{}, false
	}
	return accountingPeriod.Starting_date, true
}

// checkCombination (CheckCombination): the account's entries of the year (filtered by the codes
// asked) — zero in LCY but not in the new currency: cleared against the retained earnings
func (r *adjustAddReportingCurrency) checkCombination(run *Run, combo [3]bool, usedBusUnit, usedDept, usedProj bool, key closingBufferKey, yearStart types.Date) error {
	useBusUnit, useDept, useProj := combo[0], combo[1], combo[2]
	if !useBusUnit && usedBusUnit || !useDept && usedDept || !useProj && usedProj {
		return nil
	}
	glEntry := gtables.NewGLEntry(run)
	if useBusUnit || useDept || useProj {
		glEntry.SetCurrentKey("gl_account_no", "business_unit_code", "department_code", "project_code", "posting_date")
	} else {
		glEntry.SetCurrentKey("gl_account_no", "posting_date")
	}
	glEntry.SetRange("gl_account_no", key.glAccountNo)
	glEntry.SetRange("posting_date", yearStart, key.closingDate)
	if useBusUnit {
		glEntry.SetRange("business_unit_code", key.busUnit)
	}
	if useDept {
		glEntry.SetRange("department_code", key.dept)
	}
	if useProj {
		glEntry.SetRange("project_code", key.proj)
	}
	err := glEntry.CalcSums("amount", "additional_currency_amount")
	if err != nil {
		return err
	}
	if !glEntry.Amount.Round(2).IsZero() || glEntry.Additional_currency_amount.Round(2).IsZero() {
		return nil
	}
	acyAmount := glEntry.Additional_currency_amount.Round(2)
	err = r.insertGLEntry(run, r.nextTransactionNo, key.closingDate, key.closingDate, gtables.GLEntry_Document_type.Blank, r.docNo, key.glAccountNo,
		key.busUnit, key.dept, key.proj, "", "", acyAmount.Neg())
	if err != nil {
		return err
	}
	return r.insertGLEntry(run, r.nextTransactionNo, key.closingDate, key.closingDate, gtables.GLEntry_Document_type.Blank, r.docNo, r.retainedEarnings,
		key.busUnit, key.dept, key.proj, "", "", acyAmount)
}

// insertGLEntry (InsertGLEntry): an entry of an additional-currency amount alone (a residual), in
// the run's G/L register
func (r *adjustAddReportingCurrency) insertGLEntry(run *Run, transactionNo int, postingDate, documentDate types.Date, documentType gtables.GLEntryDocument_type, documentNo, glAccountNo,
	busUnit, dept, proj, jnlBatchName, reasonCode types.Code, addCurrAmount types.Decimal) error {
	if r.glReg == nil {
		r.glReg = gtables.NewGLRegister(run)
		r.glReg.LockTable()
		no := 1
		if r.glReg.FindLast() {
			no = r.glReg.No + 1
		}
		r.glReg.Init()
		r.glReg.No = no
		r.glReg.From_entry_no = r.nextEntryNo + 1
		r.glReg.Creation_date = system.Today()
		r.glReg.Source_code = r.sourceCode
		r.glReg.User_id = types.Code(system.UserId())
	}
	r.nextEntryNo++
	language := fcodeunits.CurrentLanguage()
	glEntry := gtables.NewGLEntry(run)
	glEntry.Init()
	glEntry.Posting_date = postingDate
	glEntry.Document_date = documentDate
	glEntry.Document_type = documentType
	glEntry.Document_no = documentNo
	glEntry.Description = types.Text(system.CopyStr(run.Text("RESIDUAL", glEntry.FieldCaption("additional_currency_amount", language)), 1, glEntry.MaxStrLen("description")))
	glEntry.Business_unit_code = busUnit
	glEntry.Department_code = dept
	glEntry.Project_code = proj
	glEntry.Source_code = r.sourceCode
	glEntry.Journal_batch_name = jnlBatchName
	glEntry.Reason_code = reasonCode
	glEntry.Entry_no = r.nextEntryNo
	glEntry.Transaction_no = transactionNo
	glEntry.Gl_account_no = glAccountNo
	glEntry.User_id = types.Code(system.UserId())
	glEntry.System_created_entry = true
	glEntry.Prior_year_entry = glEntry.Posting_date.Before(r.fiscalYearStartDate)
	glEntry.Additional_currency_amount = addCurrAmount
	if addCurrAmount.IsPositive() {
		glEntry.Add_currency_debit_amount = addCurrAmount
	} else {
		glEntry.Add_currency_credit_amount = addCurrAmount.Neg()
	}
	err := glEntry.InsertOrError(false)
	if err != nil {
		return err
	}
	r.glReg.To_entry_no = glEntry.Entry_no
	return nil
}

// onPostReport: the setup has the new currency
func (r *adjustAddReportingCurrency) onPostReport(run *Run) error {
	r.window.Close()
	glSetup := gtables.GetGeneralLedgerSetup(run)
	glSetup.Additional_reporting_currency = r.addCurrency
	err := glSetup.ModifyOrError(false)
	if err != nil {
		return err
	}
	run.Message(run.Text("DONE", r.addCurrency))
	return nil
}
