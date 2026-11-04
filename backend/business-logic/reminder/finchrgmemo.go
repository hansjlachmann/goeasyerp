package reminder

import (
	"github.com/hansjlachmann/goeasyerp/backend/business-logic/tables"
	ftables "github.com/hansjlachmann/goeasyerp/backend/foundation/tables"
	"github.com/hansjlachmann/goeasyerp/backend/foundation/types"
	gtables "github.com/hansjlachmann/goeasyerp/backend/generated/tables"
)

// FinChrgMemoMake is codeunit 394 FinChrgMemo-Make (one run: Set or SuggestLines, then Run)
type FinChrgMemoMake struct {
	src                  ftables.Source
	cust                 *gtables.Customer
	custLedgEntry        *gtables.CustomerLedgerEntry // the request's entry filters
	finChrgMemoHeaderReq *gtables.FinanceChargeMemoHeader
	finChrgMemoHeader    *gtables.FinanceChargeMemoHeader
	finChrgTerms         *gtables.FinanceChargeTerms
	custAmountLCY        types.Decimal
	headerExists         bool
}

// NewFinChrgMemoMake opens the codeunit in a database (the caller's transaction)
func NewFinChrgMemoMake(src ftables.Source) *FinChrgMemoMake {
	return &FinChrgMemoMake{src: src, finChrgMemoHeader: gtables.NewFinanceChargeMemoHeader(src)}
}

// Set (Set): make the memos of a customer, with the request's entry filters and dates
func (m *FinChrgMemoMake) Set(cust *gtables.Customer, custLedgEntry *gtables.CustomerLedgerEntry, finChrgMemoHeaderReq *gtables.FinanceChargeMemoHeader) {
	m.cust = cust
	m.custLedgEntry = custLedgEntry
	m.finChrgMemoHeaderReq = finChrgMemoHeaderReq
}

// SuggestLines (SuggestLines): fill an existing memo
func (m *FinChrgMemoMake) SuggestLines(finChrgMemoHeader *gtables.FinanceChargeMemoHeader, custLedgEntry *gtables.CustomerLedgerEntry) {
	m.finChrgMemoHeader = finChrgMemoHeader
	m.custLedgEntry = custLedgEntry
}

// Run (OnRun): the customer's interest is checked first (all currencies); below the terms'
// Minimum Amount (LCY) nothing is made; then a memo per currency (or the existing memo's lines)
func (m *FinChrgMemoMake) Run() error {
	finChrgMemoHeader := m.finChrgMemoHeader
	if finChrgMemoHeader.No != "" {
		m.headerExists = true
		err := finChrgMemoHeader.TestField("customer_no")
		if err != nil {
			return err
		}
		m.cust = gtables.NewCustomer(m.src)
		err = m.cust.GetOrError(finChrgMemoHeader.Customer_no)
		if err == nil {
			err = finChrgMemoHeader.TestField("document_date")
		}
		if err == nil {
			err = finChrgMemoHeader.TestField("fin_charge_terms_code")
		}
		if err != nil {
			return err
		}
		m.finChrgMemoHeaderReq = finChrgMemoHeader
		finChrgMemoLine := gtables.NewFinanceChargeMemoLine(m.src)
		finChrgMemoLine.SetRange("finance_charge_memo_no", finChrgMemoHeader.No)
		finChrgMemoLine.DeleteAll()
	}
	err := m.cust.TestField("fin_charge_terms_code")
	if err != nil {
		return err
	}
	var currencies []types.Code
	if m.headerExists {
		currencies = []types.Code{finChrgMemoHeader.Currency_code}
	} else {
		seen := map[types.Code]bool{}
		custLedgEntry := gtables.NewCustomerLedgerEntry(m.src)
		custLedgEntry.SetRange("customer_no", m.cust.No)
		for found := custLedgEntry.FindSet(); found; found = custLedgEntry.Next() {
			if custLedgEntry.On_hold == "" && !seen[custLedgEntry.Currency_code] {
				seen[custLedgEntry.Currency_code] = true
				currencies = append(currencies, custLedgEntry.Currency_code)
			}
		}
		currencies = sortedCodes(currencies)
	}
	for _, currencyCode := range currencies {
		err = m.finChrgMemoCheck(currencyCode)
		if err != nil {
			return err
		}
	}
	if m.custAmountLCY.IsZero() || m.custAmountLCY.LessThan(m.finChrgTerms.Minimum_amount_lcy) {
		return nil
	}
	gtables.NewFinanceChargeMemoLine(m.src).LockTable()
	gtables.NewFinanceChargeMemoHeader(m.src).LockTable()
	for _, currencyCode := range currencies {
		err = m.makeFinChrgMemo(currencyCode)
		if err != nil {
			return err
		}
	}
	return nil
}

// finChrgMemoCheck (FinChrgMemoCheck): the interest the customer's entries in a currency would
// get, added up in LCY (MakeLines with Checking — nothing inserted)
func (m *FinChrgMemoMake) finChrgMemoCheck(currencyCode types.Code) error {
	termsCode := m.cust.Fin_charge_terms_code
	documentDate := m.finChrgMemoHeaderReq.Document_date
	postingDate := m.finChrgMemoHeaderReq.Posting_date
	if m.headerExists {
		termsCode = m.finChrgMemoHeader.Fin_charge_terms_code
	}
	m.finChrgTerms = gtables.NewFinanceChargeTerms(m.src)
	err := m.finChrgTerms.GetOrError(termsCode)
	if err != nil {
		return err
	}
	return m.entries(currencyCode, func(custLedgEntry *gtables.CustomerLedgerEntry) error {
		amount, _, err := tables.CalcEntryInterest(m.src, custLedgEntry, termsCode, custLedgEntry.Due_date, custLedgEntry.Remaining_amount, documentDate, true)
		if err != nil {
			return err
		}
		if currencyCode != "" {
			currExchRate := tables.ExchangeRates(m.src)
			rate, err := currExchRate.ExchangeRate(postingDate, currencyCode)
			if err == nil {
				amount, err = currExchRate.ExchangeAmtFCYToLCY(postingDate, currencyCode, amount, rate)
			}
			if err != nil {
				return err
			}
		}
		m.custAmountLCY = m.custAmountLCY.Add(amount)
		return nil
	})
}

// entries (MakeLines): the customer's entries the terms charge in a currency — open ones (Open
// Entries, All Entries), closed ones still earning interest (Closed Entries, All Entries); not
// on hold
func (m *FinChrgMemoMake) entries(currencyCode types.Code, fn func(*gtables.CustomerLedgerEntry) error) error {
	run := func(open bool) error {
		custLedgEntry := gtables.NewCustomerLedgerEntry(m.src)
		if m.custLedgEntry != nil {
			custLedgEntry.CopyFilters(m.custLedgEntry)
		}
		custLedgEntry.SetCurrentKey("customer_no", "open")
		custLedgEntry.SetRange("customer_no", m.cust.No)
		custLedgEntry.SetRange("open", open)
		custLedgEntry.SetRange("on_hold", types.Code(""))
		custLedgEntry.SetRange("currency_code", currencyCode)
		if !open {
			custLedgEntry.SetRange("calculate_interest", true)
		}
		for found := custLedgEntry.FindSet(); found; found = custLedgEntry.Next() {
			err := fn(custLedgEntry)
			if err != nil {
				return err
			}
		}
		return nil
	}
	interestCalculation := m.finChrgTerms.Interest_calculation
	if interestCalculation == gtables.FinanceChargeTerms_Interest_calculation.OpenEntries || interestCalculation == gtables.FinanceChargeTerms_Interest_calculation.AllEntries {
		err := run(true)
		if err != nil {
			return err
		}
	}
	if interestCalculation == gtables.FinanceChargeTerms_Interest_calculation.ClosedEntries || interestCalculation == gtables.FinanceChargeTerms_Interest_calculation.AllEntries {
		return run(false)
	}
	return nil
}

// makeFinChrgMemo (MakeFinChrgMemo, MakeHeader, MakeLines): the memo of a currency (not when the
// customer has one), a line per entry with interest, the texts and fee
func (m *FinChrgMemoMake) makeFinChrgMemo(currencyCode types.Code) error {
	finChrgMemoHeader := m.finChrgMemoHeader
	if !m.headerExists {
		existing := gtables.NewFinanceChargeMemoHeader(m.src)
		existing.SetCurrentKey("customer_no", "currency_code")
		existing.SetRange("customer_no", m.cust.No)
		existing.SetRange("currency_code", currencyCode)
		if existing.FindFirst() {
			return nil
		}
		finChrgMemoHeader = gtables.NewFinanceChargeMemoHeader(m.src)
		finChrgMemoHeader.Init()
		finChrgMemoHeader.No = ""
		finChrgMemoHeader.Posting_date = m.finChrgMemoHeaderReq.Posting_date
		err := finChrgMemoHeader.InsertOrError(true)
		if err == nil {
			err = finChrgMemoHeader.Validate("customer_no", m.cust.No)
		}
		if err == nil {
			err = finChrgMemoHeader.Validate("document_date", m.finChrgMemoHeaderReq.Document_date)
		}
		if err == nil {
			err = finChrgMemoHeader.Validate("currency_code", currencyCode)
		}
		if err == nil {
			err = finChrgMemoHeader.ModifyOrError(false)
		}
		if err != nil {
			return err
		}
	}
	nextLineNo := 0
	err := m.entries(currencyCode, func(custLedgEntry *gtables.CustomerLedgerEntry) error {
		nextLineNo += 10000
		finChrgMemoLine := gtables.NewFinanceChargeMemoLine(m.src)
		finChrgMemoLine.Init()
		finChrgMemoLine.Finance_charge_memo_no = finChrgMemoHeader.No
		finChrgMemoLine.Line_no = nextLineNo
		finChrgMemoLine.Type = gtables.FinanceChargeMemoLine_Type.CustomerLedgerEntry
		err := finChrgMemoLine.Validate("entry_no", custLedgEntry.Entry_no)
		if err != nil {
			return err
		}
		if !finChrgMemoLine.Amount.IsZero() {
			return finChrgMemoLine.InsertOrError(false)
		}
		return nil
	})
	if err != nil {
		return err
	}
	return tables.InsertFinChrgMemoLines(finChrgMemoHeader)
}
