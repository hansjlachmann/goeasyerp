package demodata

import (
	"sort"

	"github.com/hansjlachmann/goeasyerp/backend/business-logic/reminder"
	ftables "github.com/hansjlachmann/goeasyerp/backend/foundation/tables"
	"github.com/hansjlachmann/goeasyerp/backend/foundation/types"
	gtables "github.com/hansjlachmann/goeasyerp/backend/generated/tables"
)

// The demo reminders and finance charge memos (Options.Journals, codeunit 901), of the domestic
// customers with overdue invoices (codeunits 392 and 394):
//   - the first customer got a reminder a month ago, issued (Options.ReminderIssue), and has
//     today's second-level reminder open; the second has a first reminder open;
//   - the third got a finance charge memo a month ago, issued (Options.FinChrgMemoIssue: the
//     interest and fee posted); the fourth has one open.

// ReminderIssuer issues a reminder (codeunit 393 Reminder-Issue)
type ReminderIssuer interface {
	Set(reminderHeader *gtables.ReminderHeader, replacePostingDate bool, postingDate types.Date)
	Run() error
}

// FinChrgMemoIssuer issues a finance charge memo (codeunit 395 FinChrgMemo-Issue)
type FinChrgMemoIssuer interface {
	Set(finChrgMemoHeader *gtables.FinanceChargeMemoHeader, replacePostingDate bool, postingDate types.Date)
	Run() error
}

func insertDemoReminders(src ftables.Source, today types.Date, opts Options, counts *Counts) error {
	monthAgo := today.AddMonths(-1)
	custs := overdueCustomers(src, monthAgo.AddDays(-15))
	next := func() *gtables.Customer {
		if len(custs) == 0 {
			return nil
		}
		cust := custs[0]
		custs = custs[1:]
		return cust
	}
	// reminders: the first customer's issued one and the open ones
	if cust := next(); cust != nil {
		if opts.ReminderIssue != nil {
			h, err := makeDemoReminder(src, cust, monthAgo)
			if err != nil {
				return err
			}
			if h != nil {
				issue := opts.ReminderIssue(src)
				issue.Set(h, false, types.Date{})
				err = issue.Run()
				if err != nil {
					return err
				}
				counts.IssuedReminders++
			}
		}
		for _, c := range []*gtables.Customer{cust, next()} {
			if c == nil {
				continue
			}
			h, err := makeDemoReminder(src, c, today)
			if err != nil {
				return err
			}
			if h != nil {
				counts.Reminders++
			}
		}
	}
	// finance charge memos: the third customer's issued one, the fourth's open one
	if cust := next(); cust != nil && opts.FinChrgMemoIssue != nil {
		h, err := makeDemoFinChrgMemo(src, cust, monthAgo)
		if err != nil {
			return err
		}
		if h != nil {
			issue := opts.FinChrgMemoIssue(src)
			issue.Set(h, false, types.Date{})
			err = issue.Run()
			if err != nil {
				return err
			}
			counts.IssuedFinChrgMemos++
		}
	}
	if cust := next(); cust != nil {
		h, err := makeDemoFinChrgMemo(src, cust, today)
		if err != nil {
			return err
		}
		if h != nil {
			counts.FinChrgMemos++
		}
	}
	return nil
}

// overdueCustomers: the domestic customers with an open invoice in LCY due before the date, by No.
func overdueCustomers(src ftables.Source, before types.Date) []*gtables.Customer {
	e := gtables.NewCustomerLedgerEntry(src)
	e.SetRange("open", true)
	e.SetRange("document_type", gtables.CustomerLedgerEntry_Document_type.Invoice)
	e.SetRange("customer_posting_group", types.Code("DOMESTIC"))
	e.SetRange("currency_code", types.Code(""))
	e.SetFilter("due_date", "<"+before.String())
	seen := map[types.Code]bool{}
	var nos []types.Code
	for found := e.FindSet(); found; found = e.Next() {
		if !seen[e.Customer_no] {
			seen[e.Customer_no] = true
			nos = append(nos, e.Customer_no)
		}
	}
	var custs []*gtables.Customer
	sort.Slice(nos, func(i, j int) bool { return nos[i] < nos[j] })
	for _, no := range nos {
		cust := gtables.NewCustomer(src)
		if cust.Get(no) {
			custs = append(custs, cust)
		}
	}
	return custs
}

// makeDemoReminder: the customer's reminder of the overdue entries on the date (Create Reminders),
// nil when nothing is due
func makeDemoReminder(src ftables.Source, cust *gtables.Customer, date types.Date) (*gtables.ReminderHeader, error) {
	req := gtables.NewReminderHeader(src)
	req.Posting_date = date
	req.Document_date = date
	m := reminder.New(src)
	m.Set(cust, nil, req, true)
	err := m.Run()
	if err != nil {
		return nil, err
	}
	h := gtables.NewReminderHeader(src)
	h.SetRange("customer_no", cust.No)
	h.SetRange("document_date", date)
	if !h.FindLast() {
		return nil, nil
	}
	return h, nil
}

// makeDemoFinChrgMemo: the customer's finance charge memo on the date (Create Finance Charge
// Memos), nil when below the terms' minimum
func makeDemoFinChrgMemo(src ftables.Source, cust *gtables.Customer, date types.Date) (*gtables.FinanceChargeMemoHeader, error) {
	req := gtables.NewFinanceChargeMemoHeader(src)
	req.Posting_date = date
	req.Document_date = date
	m := reminder.NewFinChrgMemoMake(src)
	m.Set(cust, nil, req)
	err := m.Run()
	if err != nil {
		return nil, err
	}
	h := gtables.NewFinanceChargeMemoHeader(src)
	h.SetRange("customer_no", cust.No)
	h.SetRange("document_date", date)
	if !h.FindLast() {
		return nil, nil
	}
	return h, nil
}
