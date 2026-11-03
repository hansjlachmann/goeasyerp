package reports

import (
	"sort"

	"github.com/hansjlachmann/openerp/backend/business-logic/genjnl"
	"github.com/hansjlachmann/openerp/backend/business-logic/tables"
	"github.com/hansjlachmann/openerp/backend/foundation/system"
	ftables "github.com/hansjlachmann/openerp/backend/foundation/tables"
	"github.com/hansjlachmann/openerp/backend/foundation/types"
	gtables "github.com/hansjlachmann/openerp/backend/generated/tables"
)

// Report 595 Adjust Exchange Rates: the LCY amounts of what is held in foreign currencies are
// brought to the adjustment exchange rates of the posting date — the bank accounts' balances in
// their currency (the difference to the bank account and the currency's realized gains or losses
// account), the open customer and vendor ledger entries of the period (their remaining LCY
// amounts changed in the entries; the total per posting group to the receivables or payables
// account, the unrealized gains and losses to the currency's accounts). Each currency gets its
// Last Date Adjusted; each adjustment per account type, posting group and currency an Exch. Rate
// Adjmt. Register (86). With Adjust G/L Accounts for Add.-Reporting Currency the G/L accounts and
// tax entries are adjusted to the additional reporting currency's rate as well
// (adjustexchratesaddcurr.go). A processing-only report (main menu General Ledger → Currency).
const AdjustExchangeRatesID = 595

func init() {
	Register(AdjustExchangeRatesID, "adjust-exchange-rates", func() Report { return &adjustExchangeRates{} })
}

type adjustExchangeRates struct {
	startDate, endDate types.Date
	postingDate        types.Date
	postingDocNo       types.Code
	postingDescription string
	sourceCode         types.Code
	genJnlPostLine     *genjnl.GenJnlPostLine
	exchRateAdjRegNo   int
	currencyFactors    map[types.Code]types.Decimal // the adjusted currencies (Currency2)
	adjBuffer          map[adjBufferKey]*adjBufferAmounts
	bankBuffer         map[types.Code]*adjBufferAmounts // a currency's bank accounts per posting group
	totalAdjAmount     types.Decimal
	window             *system.Window
	noCurrencies       bool
	currentCurrency    types.Code // the currency of the bank accounts being adjusted
	adjCustVendBank    bool
	adjGLAcc           bool
	addCurr            addCurrAdjustment
}

// adjBufferKey / adjBufferAmounts: table 331 Adjust Exchange Rate Buffer
type adjBufferKey struct {
	currencyCode, postingGroup types.Code
}

type adjBufferAmounts struct {
	adjBase, adjBaseLCY, adjAmount, totalGainsAmount, totalLossesAmount types.Decimal
}

func (r *adjustExchangeRates) Definition() *Definition {
	return &Definition{
		OptionsCaption: "ADJUSTMENT_PERIOD",
		Options: []Option{
			{Name: "starting_date", Type: "date"},
			{Name: "ending_date", Type: "date"},
			{Name: "posting_description", Type: "text"},
			{Name: "posting_date", Type: "date", Required: true},
			{Name: "document_no", Type: "text", Required: true},
			{Name: "adjust_cust_vend_bank", Type: "bool", Default: "true"},
			{Name: "adjust_gl_accounts", Type: "bool", Default: "false"},
		},
		Transaction:  true,
		OnInitReport: r.onInitReport,
		OnPreReport:  r.onPreReport,
		OnPostReport: r.onPostReport,
		DataItems: []*DataItem{
			{
				Name:                "currency",
				Table:               gtables.CurrencyTableName,
				Key:                 []string{"code"},
				RequestFilterFields: []string{"code"},
				OnPreDataItem:       Trigger(r.currencyOnPreDataItem),
				OnAfterGetRecord:    Trigger(r.currencyOnAfterGetRecord),
				OnPostDataItem:      Trigger(r.currencyOnPostDataItem),
				DataItems: []*DataItem{{
					Name:             "bank_account",
					Table:            gtables.BankAccountTableName,
					Key:              []string{"bank_acc_posting_group"},
					Link:             map[string]string{"currency_code": "code"},
					OnPreDataItem:    Trigger(r.bankAccOnPreDataItem),
					OnAfterGetRecord: Trigger(r.bankAccOnAfterGetRecord),
					OnPostDataItem:   Trigger(r.bankAccOnPostDataItem),
				}},
			},
			{
				Name:           "customer",
				Table:          gtables.CustomerTableName,
				Key:            []string{"no"},
				OnPreDataItem:  Trigger(r.customerOnPreDataItem),
				OnPostDataItem: Trigger(r.customerOnPostDataItem),
				DataItems: []*DataItem{{
					Name:             "cust_ledger_entry",
					Table:            "Customer_ledger_entry", // (the API name)
					Key:              []string{"customer_no", "open", "positive", "due_date", "currency_code"},
					TableView:        []ftables.FieldFilter{{Field: "open", Expression: "true"}, {Field: "currency_code", Expression: "<>''"}},
					Link:             map[string]string{"customer_no": "no"},
					OnPreDataItem:    Trigger(r.custLedgEntryOnPreDataItem),
					OnAfterGetRecord: Trigger(r.custLedgEntryOnAfterGetRecord),
				}},
			},
			{
				Name:           "vendor",
				Table:          gtables.VendorTableName,
				Key:            []string{"no"},
				OnPreDataItem:  Trigger(r.vendorOnPreDataItem),
				OnPostDataItem: Trigger(r.vendorOnPostDataItem),
				DataItems: []*DataItem{{
					Name:             "vendor_ledger_entry",
					Table:            "Vendor_ledger_entry",
					Key:              []string{"vendor_no", "open", "positive", "due_date", "currency_code"},
					TableView:        []ftables.FieldFilter{{Field: "open", Expression: "true"}, {Field: "currency_code", Expression: "<>''"}},
					Link:             map[string]string{"vendor_no": "no"},
					OnPreDataItem:    Trigger(r.vendLedgEntryOnPreDataItem),
					OnAfterGetRecord: Trigger(r.vendLedgEntryOnAfterGetRecord),
				}},
			},
			{
				Name:             "tax_posting_setup",
				Table:            gtables.TaxPostingSetupTableName,
				Key:              []string{"tax_bus_posting_group", "tax_prod_posting_group"},
				OnPreDataItem:    Trigger(r.taxPostingSetupOnPreDataItem),
				OnAfterGetRecord: Trigger(r.taxPostingSetupOnAfterGetRecord),
			},
			{
				Name:             "gl_account",
				Table:            gtables.GLAccountTableName,
				Key:              []string{"no"},
				TableView:        []ftables.FieldFilter{{Field: "exchange_rate_adjustment", Expression: "1..2"}},
				OnPreDataItem:    Trigger(r.glAccOnPreDataItem),
				OnAfterGetRecord: Trigger(r.glAccOnAfterGetRecord),
				OnPostDataItem:   Trigger(r.glAccOnPostDataItem),
			},
		},
	}
}

// The request page (OnOpenForm): the posting date is the work date, the description the
// standard text
func (r *adjustExchangeRates) onInitReport(run *Run) error {
	run.SetOption("posting_date", system.WorkDate().String())
	run.SetOption("posting_description", run.Text("POSTING_DESCRIPTION"))
	return nil
}

func (r *adjustExchangeRates) onPreReport(run *Run) error {
	r.startDate = run.OptionDate("starting_date")
	r.endDate = run.OptionDate("ending_date")
	if r.endDate.IsZero() {
		r.endDate = types.NewDate(9999, 12, 31)
	}
	r.postingDate = run.OptionDate("posting_date")
	if r.postingDate.IsZero() {
		return system.Error(run.Text("ENTER_POSTING_DATE"))
	}
	r.postingDocNo = types.Code(system.UpperCase(run.Option("document_no")))
	if r.postingDocNo == "" {
		return system.Error(run.Text("ENTER_DOCUMENT_NO"))
	}
	r.postingDescription = run.Option("posting_description")
	r.adjCustVendBank = run.OptionBool("adjust_cust_vend_bank")
	r.adjGLAcc = run.OptionBool("adjust_gl_accounts")
	if !r.adjCustVendBank && r.adjGLAcc {
		if system.GuiAllowed() && !system.Confirm(run.Text("CONFIRM_GL_ONLY")) {
			return system.Error(run.Text("CANCELED"))
		}
	}
	r.sourceCode = gtables.GetSourceCodeSetup(run).Exchange_rate_adjmt
	exchRateAdjReg := gtables.NewExchRateAdjmtReg(run)
	if exchRateAdjReg.FindLast() {
		r.exchRateAdjRegNo = exchRateAdjReg.No
	}
	if r.adjGLAcc {
		err := r.checkAddCurrAdjustment(run)
		if err != nil {
			return err
		}
	}
	r.genJnlPostLine = genjnl.NewGenJnlPostLine(run)
	r.currencyFactors = map[types.Code]types.Decimal{}
	r.adjBuffer = map[adjBufferKey]*adjBufferAmounts{}
	r.noCurrencies = true
	r.window = system.OpenWindow(run.Text("WINDOW"))
	return nil
}

func (r *adjustExchangeRates) onPostReport(run *Run) error {
	r.window.Close()
	run.Message(run.Text("DONE", r.totalAdjAmount.String()))
	return nil
}

// currencyOnPreDataItem: currencies, bank accounts, customers and vendors only with Adjust
// Customer, Vendor and Bank Accounts
func (r *adjustExchangeRates) currencyOnPreDataItem(currency *gtables.Currency, run *Run) error {
	if !r.adjCustVendBank {
		return Break()
	}
	return nil
}

// currencyOnAfterGetRecord: the currency's Last Date Adjusted and its adjustment exchange rate
func (r *adjustExchangeRates) currencyOnAfterGetRecord(currency *gtables.Currency, run *Run) error {
	r.noCurrencies = false
	currency.Last_date_adjusted = r.postingDate
	err := currency.ModifyOrError(false)
	if err != nil {
		return err
	}
	factor, err := tables.ExchangeRates(run).ExchangeRateAdjmt(r.postingDate, currency.Code)
	if err != nil {
		return err
	}
	r.currencyFactors[currency.Code] = factor
	r.currentCurrency = currency.Code
	r.window.Update(1, currency.Code)
	return nil
}

func (r *adjustExchangeRates) currencyOnPostDataItem(currency *gtables.Currency, run *Run) error {
	if r.noCurrencies {
		return system.Error(run.Text("NO_CURRENCIES"))
	}
	return nil
}

func (r *adjustExchangeRates) bankAccOnPreDataItem(bankAcc *gtables.BankAccount, run *Run) error {
	r.bankBuffer = map[types.Code]*adjBufferAmounts{}
	return bankAcc.SetFlowFilter("date_filter", r.startDate.String()+".."+r.endDate.String())
}

// bankAccOnAfterGetRecord: the balance at the date at the adjustment rate less its LCY balance
// goes to the bank account (an LCY amount only) and to the currency's realized gains or losses
func (r *adjustExchangeRates) bankAccOnAfterGetRecord(bankAcc *gtables.BankAccount, run *Run) error {
	r.window.Update(2, bankAcc.No)
	bankAcc.CalcFields("balance_at_date", "balance_at_date_lcy")
	currency := gtables.NewCurrency(run)
	err := currency.GetOrError(bankAcc.Currency_code)
	if err != nil {
		return err
	}
	adjBase := bankAcc.Balance_at_date
	adjBaseLCY := bankAcc.Balance_at_date_lcy
	amountLCY, err := tables.ExchangeRates(run).ExchangeAmtFCYToLCYAdjmt(r.postingDate, currency.Code, adjBase, r.currencyFactors[currency.Code])
	if err != nil {
		return err
	}
	adjAmount := system.Round(amountLCY).Sub(adjBaseLCY)
	if adjAmount.IsZero() {
		return nil
	}
	genJnlLine := gtables.NewGenJournalLine(run)
	genJnlLine.Init()
	err = genJnlLine.Validate("posting_date", r.postingDate)
	if err != nil {
		return err
	}
	genJnlLine.Document_no = r.postingDocNo
	genJnlLine.Account_type = gtables.GenJournalLine_Account_type.BankAccount
	err = genJnlLine.Validate("account_no", bankAcc.No)
	if err != nil {
		return err
	}
	genJnlLine.Description = types.Text(system.CopyStr(system.StrSubstNo(r.postingDescription, currency.Code, adjBase), 1, genJnlLine.MaxStrLen("description")))
	err = genJnlLine.Validate("amount", types.Decimal{})
	if err != nil {
		return err
	}
	genJnlLine.Amount_lcy = adjAmount
	genJnlLine.Source_currency_code = currency.Code
	genJnlLine.Source_code = r.sourceCode
	genJnlLine.System_created_entry = true
	err = r.genJnlPostLine.Run(genJnlLine)
	if err != nil {
		return err
	}
	if adjAmount.IsPositive() {
		err = currency.TestField("realized_gains_acc")
		if err == nil {
			err = r.postAdjmt(run, currency.Realized_gains_acc, adjAmount.Neg(), adjBase, currency.Code)
		}
	} else {
		err = currency.TestField("realized_losses_acc")
		if err == nil {
			err = r.postAdjmt(run, currency.Realized_losses_acc, adjAmount.Neg(), adjBase, currency.Code)
		}
	}
	if err != nil {
		return err
	}
	r.totalAdjAmount = r.totalAdjAmount.Add(adjAmount)
	r.window.Update(4, r.totalAdjAmount)
	b := r.bankBuffer[bankAcc.Bank_acc_posting_group]
	if b == nil {
		b = &adjBufferAmounts{}
		r.bankBuffer[bankAcc.Bank_acc_posting_group] = b
	}
	b.adjBase = b.adjBase.Add(adjBase)
	b.adjBaseLCY = b.adjBaseLCY.Add(adjBaseLCY)
	b.adjAmount = b.adjAmount.Add(adjAmount)
	return nil
}

// bankAccOnPostDataItem: a register per bank account posting group of the currency
func (r *adjustExchangeRates) bankAccOnPostDataItem(bankAcc *gtables.BankAccount, run *Run) error {
	for _, group := range sortedGroups(r.bankBuffer) {
		err := r.insertExchRateAdjmtReg(run, gtables.ExchRateAdjmtReg_Account_type.BankAccount, group, r.currentCurrency, r.bankBuffer[group])
		if err != nil {
			return err
		}
	}
	return nil
}

func sortedGroups(m map[types.Code]*adjBufferAmounts) []types.Code {
	groups := make([]types.Code, 0, len(m))
	for g := range m {
		groups = append(groups, g)
	}
	sort.Slice(groups, func(i, j int) bool { return groups[i] < groups[j] })
	return groups
}

// ledgerAmounts are the fields of a customer or vendor ledger entry the adjustment changes
type ledgerAmounts struct {
	currencyCode, postingGroup                 types.Code
	amount, remainingAmount, originalAmountLCY types.Decimal
	debitAmount, creditAmount                  types.Decimal
	amountLCY, remainingAmtLCY                 *types.Decimal
	debitAmountLCY, creditAmountLCY            *types.Decimal
}

// adjustEntry: the entry's remaining amount at the adjustment rate less its remaining LCY amount
// is added to its (remaining) LCY amounts; the part up to the original rate's value is a gain or
// loss reversed, beyond it a new one (OnAfterGetRecord of the ledger entries); it reports whether
// the entry changed
func (r *adjustExchangeRates) adjustEntry(run *Run, e ledgerAmounts) (bool, error) {
	factor, ok := r.currencyFactors[e.currencyCode]
	if !ok || e.amount.IsZero() {
		return false, nil // a currency not adjusted (the request filter)
	}
	adjBase := e.remainingAmount
	adjBaseLCY := *e.remainingAmtLCY
	amountLCY, err := tables.ExchangeRates(run).ExchangeAmtFCYToLCYAdjmt(r.postingDate, e.currencyCode, e.remainingAmount, factor)
	if err != nil {
		return false, err
	}
	adjAmount := system.Round(amountLCY).Sub(*e.remainingAmtLCY)
	if adjAmount.IsZero() {
		return false, nil
	}
	*e.amountLCY = e.amountLCY.Add(adjAmount)
	oldRemainAmount := *e.remainingAmtLCY
	orgRemainAmount := system.Round(e.remainingAmount.Mul(e.originalAmountLCY.Div(e.amount)))
	*e.remainingAmtLCY = e.remainingAmtLCY.Add(adjAmount)
	correction := e.debitAmount.IsNegative() || e.creditAmount.IsNegative() || e.debitAmountLCY.IsNegative() || e.creditAmountLCY.IsNegative()
	if (e.amount.IsPositive() || e.amountLCY.IsPositive()) && !correction || (e.amount.IsNegative() || e.amountLCY.IsNegative()) && correction {
		*e.debitAmountLCY = *e.amountLCY
		*e.creditAmountLCY = types.Decimal{}
	} else {
		*e.debitAmountLCY = types.Decimal{}
		*e.creditAmountLCY = e.amountLCY.Neg()
	}
	newRemain := *e.remainingAmtLCY
	var gainsAmount, lossesAmount types.Decimal
	switch {
	case !oldRemainAmount.LessThan(orgRemainAmount) && !newRemain.LessThan(orgRemainAmount):
		gainsAmount = newRemain.Sub(oldRemainAmount)
	case !oldRemainAmount.LessThan(orgRemainAmount) && !newRemain.GreaterThan(orgRemainAmount):
		gainsAmount = orgRemainAmount.Sub(oldRemainAmount)
		lossesAmount = newRemain.Sub(orgRemainAmount)
	case !oldRemainAmount.GreaterThan(orgRemainAmount) && !newRemain.LessThan(orgRemainAmount):
		gainsAmount = newRemain.Sub(orgRemainAmount)
		lossesAmount = orgRemainAmount.Sub(oldRemainAmount)
	default:
		lossesAmount = newRemain.Sub(oldRemainAmount)
	}
	r.totalAdjAmount = r.totalAdjAmount.Add(adjAmount)
	r.window.Update(4, r.totalAdjAmount)
	key := adjBufferKey{e.currencyCode, e.postingGroup}
	b := r.adjBuffer[key]
	if b == nil {
		b = &adjBufferAmounts{}
		r.adjBuffer[key] = b
	}
	b.adjBase = b.adjBase.Add(adjBase)
	b.adjBaseLCY = b.adjBaseLCY.Add(adjBaseLCY)
	b.adjAmount = b.adjAmount.Add(adjAmount)
	b.totalGainsAmount = b.totalGainsAmount.Add(gainsAmount)
	b.totalLossesAmount = b.totalLossesAmount.Add(lossesAmount)
	return true, nil
}

func (r *adjustExchangeRates) custLedgEntryOnPreDataItem(e *gtables.CustomerLedgerEntry, run *Run) error {
	e.SetRange("posting_date", r.startDate, r.endDate)
	e.LockTable()
	return nil
}

func (r *adjustExchangeRates) custLedgEntryOnAfterGetRecord(e *gtables.CustomerLedgerEntry, run *Run) error {
	r.window.Update(2, e.Customer_no)
	changed, err := r.adjustEntry(run, ledgerAmounts{
		currencyCode: e.Currency_code, postingGroup: e.Customer_posting_group,
		amount: e.Amount, remainingAmount: e.Remaining_amount, originalAmountLCY: e.Original_amount_lcy,
		debitAmount: e.Debit_amount, creditAmount: e.Credit_amount,
		amountLCY: &e.Amount_lcy, remainingAmtLCY: &e.Remaining_amt_lcy, debitAmountLCY: &e.Debit_amount_lcy, creditAmountLCY: &e.Credit_amount_lcy,
	})
	if err != nil || !changed {
		return err
	}
	return e.ModifyOrError(false)
}

func (r *adjustExchangeRates) vendLedgEntryOnPreDataItem(e *gtables.VendorLedgerEntry, run *Run) error {
	e.SetRange("posting_date", r.startDate, r.endDate)
	e.LockTable()
	return nil
}

func (r *adjustExchangeRates) vendLedgEntryOnAfterGetRecord(e *gtables.VendorLedgerEntry, run *Run) error {
	r.window.Update(3, e.Vendor_no)
	changed, err := r.adjustEntry(run, ledgerAmounts{
		currencyCode: e.Currency_code, postingGroup: e.Vendor_posting_group,
		amount: e.Amount, remainingAmount: e.Remaining_amount, originalAmountLCY: e.Original_amount_lcy,
		debitAmount: e.Debit_amount, creditAmount: e.Credit_amount,
		amountLCY: &e.Amount_lcy, remainingAmtLCY: &e.Remaining_amt_lcy, debitAmountLCY: &e.Debit_amount_lcy, creditAmountLCY: &e.Credit_amount_lcy,
	})
	if err != nil || !changed {
		return err
	}
	return e.ModifyOrError(false)
}

func (r *adjustExchangeRates) customerOnPreDataItem(cust *gtables.Customer, run *Run) error {
	if !r.adjCustVendBank {
		return Break()
	}
	return nil
}

func (r *adjustExchangeRates) vendorOnPreDataItem(vend *gtables.Vendor, run *Run) error {
	if !r.adjCustVendBank {
		return Break()
	}
	return nil
}

func (r *adjustExchangeRates) customerOnPostDataItem(cust *gtables.Customer, run *Run) error {
	return r.handlePostAdjmt(run, gtables.ExchRateAdjmtReg_Account_type.Customer)
}

func (r *adjustExchangeRates) vendorOnPostDataItem(vend *gtables.Vendor, run *Run) error {
	return r.handlePostAdjmt(run, gtables.ExchRateAdjmtReg_Account_type.Vendor)
}

// handlePostAdjmt: per currency and posting group the adjustment to the receivables or payables
// account (and a register), per currency the unrealized gains and losses (HandlePostAdjmt)
func (r *adjustExchangeRates) handlePostAdjmt(run *Run, accountType gtables.ExchRateAdjmtRegAccount_type) error {
	keys := make([]adjBufferKey, 0, len(r.adjBuffer))
	for k := range r.adjBuffer {
		keys = append(keys, k)
	}
	sort.Slice(keys, func(i, j int) bool {
		if keys[i].currencyCode != keys[j].currencyCode {
			return keys[i].currencyCode < keys[j].currencyCode
		}
		return keys[i].postingGroup < keys[j].postingGroup
	})
	// summarize per currency
	perCurrency := map[types.Code]*adjBufferAmounts{}
	var currencies []types.Code
	for _, k := range keys {
		c := perCurrency[k.currencyCode]
		if c == nil {
			c = &adjBufferAmounts{}
			perCurrency[k.currencyCode] = c
			currencies = append(currencies, k.currencyCode)
		}
		b := r.adjBuffer[k]
		c.adjBase = c.adjBase.Add(b.adjBase)
		c.totalGainsAmount = c.totalGainsAmount.Add(b.totalGainsAmount)
		c.totalLossesAmount = c.totalLossesAmount.Add(b.totalLossesAmount)
	}
	// post per posting group and per currency
	for _, currencyCode := range currencies {
		for _, k := range keys {
			if k.currencyCode != currencyCode {
				continue
			}
			b := r.adjBuffer[k]
			if b.adjAmount.IsZero() {
				continue
			}
			var accNo types.Code
			if accountType == gtables.ExchRateAdjmtReg_Account_type.Customer {
				custPostingGr := gtables.NewCustomerPostingGroup(run)
				err := custPostingGr.GetOrError(k.postingGroup)
				if err == nil {
					err = custPostingGr.TestField("receivables_account")
				}
				if err != nil {
					return err
				}
				accNo = custPostingGr.Receivables_account
			} else {
				vendPostingGr := gtables.NewVendorPostingGroup(run)
				err := vendPostingGr.GetOrError(k.postingGroup)
				if err == nil {
					err = vendPostingGr.TestField("payables_account")
				}
				if err != nil {
					return err
				}
				accNo = vendPostingGr.Payables_account
			}
			err := r.postAdjmt(run, accNo, b.adjAmount, b.adjBase, currencyCode)
			if err == nil {
				err = r.insertExchRateAdjmtReg(run, accountType, k.postingGroup, currencyCode, b)
			}
			if err != nil {
				return err
			}
		}
		c := perCurrency[currencyCode]
		currency := gtables.NewCurrency(run)
		err := currency.GetOrError(currencyCode)
		if err != nil {
			return err
		}
		if !c.totalGainsAmount.IsZero() {
			err = currency.TestField("unrealized_gains_acc")
			if err == nil {
				err = r.postAdjmt(run, currency.Unrealized_gains_acc, c.totalGainsAmount.Neg(), c.adjBase, currencyCode)
			}
			if err != nil {
				return err
			}
		}
		if !c.totalLossesAmount.IsZero() {
			err = currency.TestField("unrealized_losses_acc")
			if err == nil {
				err = r.postAdjmt(run, currency.Unrealized_losses_acc, c.totalLossesAmount.Neg(), c.adjBase, currencyCode)
			}
			if err != nil {
				return err
			}
		}
	}
	r.adjBuffer = map[adjBufferKey]*adjBufferAmounts{}
	return nil
}

// postAdjmt: a system-created G/L account line of the adjustment (PostAdjmt)
func (r *adjustExchangeRates) postAdjmt(run *Run, glAccNo types.Code, postingAmount, adjBase types.Decimal, currencyCode types.Code) error {
	if postingAmount.IsZero() {
		return nil
	}
	genJnlLine := gtables.NewGenJournalLine(run)
	genJnlLine.Init()
	err := genJnlLine.Validate("posting_date", r.postingDate)
	if err != nil {
		return err
	}
	genJnlLine.Document_no = r.postingDocNo
	genJnlLine.Account_type = gtables.GenJournalLine_Account_type.GLAccount
	err = genJnlLine.Validate("account_no", glAccNo)
	if err != nil {
		return err
	}
	genJnlLine.Source_currency_code = currencyCode
	genJnlLine.Description = types.Text(system.CopyStr(system.StrSubstNo(r.postingDescription, currencyCode, adjBase), 1, genJnlLine.MaxStrLen("description")))
	err = genJnlLine.Validate("amount", postingAmount)
	if err != nil {
		return err
	}
	genJnlLine.Source_code = r.sourceCode
	genJnlLine.System_created_entry = true
	return r.genJnlPostLine.Run(genJnlLine)
}

// insertExchRateAdjmtReg: the register of an account type, posting group and currency
func (r *adjustExchangeRates) insertExchRateAdjmtReg(run *Run, accountType gtables.ExchRateAdjmtRegAccount_type, postingGroup, currencyCode types.Code, b *adjBufferAmounts) error {
	r.exchRateAdjRegNo++
	reg := gtables.NewExchRateAdjmtReg(run)
	reg.No = r.exchRateAdjRegNo
	reg.Creation_date = r.postingDate
	reg.Account_type = accountType
	reg.Posting_group = postingGroup
	reg.Currency_code = currencyCode
	reg.Currency_factor = r.currencyFactors[currencyCode]
	reg.Adjusted_base = b.adjBase
	reg.Adjusted_base_lcy = b.adjBaseLCY
	reg.Adjusted_amt_lcy = b.adjAmount
	return reg.InsertOrError(false)
}
