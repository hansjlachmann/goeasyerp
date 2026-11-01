// Package reminder makes reminders and finance charge memos: codeunit 392 Reminder-Make (the
// reminder of a customer's overdue entries, one per currency, or the lines of an existing
// reminder) and codeunit 394 FinChrgMemo-Make (finance charge memos the same way).
package reminder

import (
	"sort"

	"github.com/hansjlachmann/openerp/backend/business-logic/tables"
	ftables "github.com/hansjlachmann/openerp/backend/foundation/tables"
	"github.com/hansjlachmann/openerp/backend/foundation/types"
	gtables "github.com/hansjlachmann/openerp/backend/generated/tables"
)

// ReminderMake is codeunit 392 Reminder-Make (one run: Set or SuggestLines, then Run)
type ReminderMake struct {
	src                ftables.Source
	cust               *gtables.Customer
	custLedgEntry      *gtables.CustomerLedgerEntry // the request's entry filters (Document Type …)
	reminderHeaderReq  *gtables.ReminderHeader      // the request's posting and document date
	reminderHeader     *gtables.ReminderHeader
	reminderTerms      *gtables.ReminderTerms
	overdueEntriesOnly bool
	headerExists       bool
}

// New opens the codeunit in a database (the caller's transaction)
func New(src ftables.Source) *ReminderMake {
	return &ReminderMake{src: src, reminderHeader: gtables.NewReminderHeader(src)}
}

// Set (Set): make the reminders of a customer, with the entry filters and dates of the request
func (m *ReminderMake) Set(cust *gtables.Customer, custLedgEntry *gtables.CustomerLedgerEntry, reminderHeaderReq *gtables.ReminderHeader, overdueEntriesOnly bool) {
	m.cust = cust
	m.custLedgEntry = custLedgEntry
	m.reminderHeaderReq = reminderHeaderReq
	m.overdueEntriesOnly = overdueEntriesOnly
}

// SuggestLines (SuggestLines): fill an existing reminder
func (m *ReminderMake) SuggestLines(reminderHeader *gtables.ReminderHeader, custLedgEntry *gtables.CustomerLedgerEntry, overdueEntriesOnly bool) {
	m.reminderHeader = reminderHeader
	m.custLedgEntry = custLedgEntry
	m.overdueEntriesOnly = overdueEntriesOnly
}

// Run (OnRun): an existing reminder gets its lines again; otherwise one reminder per currency of
// the customer's open entries
func (m *ReminderMake) Run() error {
	reminderHeader := m.reminderHeader
	if reminderHeader.No != "" {
		m.headerExists = true
		err := reminderHeader.TestField("customer_no")
		if err != nil {
			return err
		}
		m.cust = gtables.NewCustomer(m.src)
		err = m.cust.GetOrError(reminderHeader.Customer_no)
		if err == nil {
			err = reminderHeader.TestField("document_date")
		}
		if err == nil {
			err = reminderHeader.TestField("reminder_terms_code")
		}
		if err != nil {
			return err
		}
		m.reminderHeaderReq = reminderHeader
		reminderLine := gtables.NewReminderLine(m.src)
		reminderLine.SetRange("reminder_no", reminderHeader.No)
		reminderLine.DeleteAll()
	}
	err := m.cust.TestField("reminder_terms_code")
	if err != nil {
		return err
	}
	m.reminderTerms = gtables.NewReminderTerms(m.src)
	err = m.reminderTerms.GetOrError(m.cust.Reminder_terms_code)
	if err != nil {
		return err
	}
	if m.headerExists {
		return m.makeReminder(reminderHeader.Currency_code)
	}
	var currencies []types.Code
	seen := map[types.Code]bool{}
	custLedgEntry := m.entries()
	custLedgEntry.SetRange("positive", true)
	for found := custLedgEntry.FindSet(); found; found = custLedgEntry.Next() {
		if custLedgEntry.On_hold == "" && !seen[custLedgEntry.Currency_code] {
			seen[custLedgEntry.Currency_code] = true
			currencies = append(currencies, custLedgEntry.Currency_code)
		}
	}
	for _, currencyCode := range sortedCodes(currencies) {
		err = m.makeReminder(currencyCode)
		if err != nil {
			return err
		}
	}
	return nil
}

// entries: the customer's open entries in the request's entry filters
func (m *ReminderMake) entries() *gtables.CustomerLedgerEntry {
	custLedgEntry := gtables.NewCustomerLedgerEntry(m.src)
	if m.custLedgEntry != nil {
		custLedgEntry.CopyFilters(m.custLedgEntry)
	}
	custLedgEntry.SetCurrentKey("customer_no", "open")
	custLedgEntry.SetRange("customer_no", m.cust.No)
	custLedgEntry.SetRange("open", true)
	return custLedgEntry
}

// lastReminderEntry: the last reminder of a customer entry (nil: none)
func (m *ReminderMake) lastReminderEntry(entryNo int) *gtables.ReminderFinChargeEntry {
	reminderEntry := gtables.NewReminderFinChargeEntry(m.src)
	reminderEntry.SetCurrentKey("customer_entry_no", "type")
	reminderEntry.SetRange("customer_entry_no", entryNo)
	reminderEntry.SetRange("type", gtables.ReminderFinChargeEntry_Type.Reminder)
	if reminderEntry.FindLast() {
		return reminderEntry
	}
	return nil
}

// makeReminder (MakeReminder): the reminder level (the highest of the overdue entries), the
// reminder when an entry is past the level's grace period, a line per (overdue) entry, the texts
// and fee
func (m *ReminderMake) makeReminder(currencyCode types.Code) error {
	cust := m.cust
	custLedgEntry := m.entries()
	custLedgEntry.SetRange("currency_code", currencyCode)
	maxLineLevel := 0
	var custAmount types.Decimal
	// Calculate reminder level and remaining amount
	for found := custLedgEntry.FindSet(); found; found = custLedgEntry.Next() {
		if custLedgEntry.On_hold == "" && custLedgEntry.Positive {
			var reminderDate types.Date
			lineLevel := 1
			reminderEntry := m.lastReminderEntry(custLedgEntry.Entry_no)
			if reminderEntry != nil {
				reminderDate = reminderEntry.Document_date
				lineLevel = reminderEntry.Reminder_level + 1
			} else {
				reminderDate = custLedgEntry.Due_date
			}
			if (reminderDate.Before(m.reminderHeaderReq.Document_date) || !m.overdueEntriesOnly) && lineLevel > maxLineLevel {
				maxLineLevel = lineLevel
			}
		}
		custAmount = custAmount.Add(custLedgEntry.Remaining_amount)
	}
	if maxLineLevel > m.reminderTerms.Max_no_of_reminders && m.reminderTerms.Max_no_of_reminders != 0 || custAmount.IsNegative() {
		return nil
	}
	reminderLevel := gtables.NewReminderLevel(m.src)
	reminderLevel.SetRange("reminder_terms_code", m.reminderTerms.Code)
	reminderLevel.SetRange("no", 1, maxLineLevel)
	if !reminderLevel.FindLast() {
		reminderLevel.Init()
	}
	// Determine whether to make lines
	makeDoc := false
	for found := custLedgEntry.FindSet(); found && !makeDoc; found = custLedgEntry.Next() {
		if custLedgEntry.On_hold == "" && custLedgEntry.Positive {
			reminderDate := custLedgEntry.Due_date
			reminderEntry := m.lastReminderEntry(custLedgEntry.Entry_no)
			if reminderEntry != nil && reminderEntry.Document_date.After(custLedgEntry.Due_date) {
				reminderDate = reminderEntry.Document_date
			}
			graceEnd, err := tables.CalcDateFormula(reminderLevel.Grace_period, reminderDate)
			if err != nil {
				return err
			}
			if graceEnd.Before(m.reminderHeaderReq.Document_date) {
				makeDoc = true
			}
		}
	}
	if !makeDoc {
		return nil
	}
	reminderHeader := m.reminderHeader
	gtables.NewReminderLine(m.src).LockTable()
	reminderHeader.LockTable()
	if !m.headerExists {
		existing := gtables.NewReminderHeader(m.src)
		existing.SetCurrentKey("customer_no", "currency_code")
		existing.SetRange("customer_no", cust.No)
		existing.SetRange("currency_code", currencyCode)
		if existing.FindFirst() {
			return nil
		}
		reminderHeader = gtables.NewReminderHeader(m.src)
		reminderHeader.Init()
		reminderHeader.No = ""
		reminderHeader.Posting_date = m.reminderHeaderReq.Posting_date
		err := reminderHeader.InsertOrError(true)
		if err == nil {
			err = reminderHeader.Validate("customer_no", cust.No)
		}
		if err == nil {
			err = reminderHeader.Validate("currency_code", currencyCode)
		}
		if err != nil {
			return err
		}
		reminderHeader.Document_date = m.reminderHeaderReq.Document_date
	}
	reminderHeader.Reminder_level = maxLineLevel
	err := reminderHeader.ModifyOrError(false)
	if err != nil {
		return err
	}
	nextLineNo := 0
	for found := custLedgEntry.FindSet(); found; found = custLedgEntry.Next() {
		var reminderDate types.Date
		reminderEntry := m.lastReminderEntry(custLedgEntry.Entry_no)
		if reminderEntry != nil {
			reminderDate = reminderEntry.Document_date
		} else {
			reminderDate = custLedgEntry.Due_date
		}
		if reminderDate.Before(m.reminderHeaderReq.Document_date) || !m.overdueEntriesOnly {
			nextLineNo += 10000
			reminderLine := gtables.NewReminderLine(m.src)
			reminderLine.Init()
			reminderLine.Reminder_no = reminderHeader.No
			reminderLine.Line_no = nextLineNo
			reminderLine.Type = gtables.ReminderLine_Type.CustomerLedgerEntry
			err = reminderLine.Validate("entry_no", custLedgEntry.Entry_no)
			if err == nil {
				err = reminderLine.InsertOrError(false)
			}
			if err != nil {
				return err
			}
		}
	}
	err = reminderHeader.Validate("reminder_level", reminderHeader.Reminder_level)
	if err == nil {
		err = tables.InsertReminderLines(reminderHeader)
	}
	return err
}

// sortedCodes: the currencies in code order (the classic's temporary Currency table)
func sortedCodes(codes []types.Code) []types.Code {
	sort.Slice(codes, func(i, j int) bool { return codes[i] < codes[j] })
	return codes
}
