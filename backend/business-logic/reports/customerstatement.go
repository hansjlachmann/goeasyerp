package reports

import (
	"github.com/hansjlachmann/goeasyerp/backend/business-logic/formataddress"
	fcodeunits "github.com/hansjlachmann/goeasyerp/backend/foundation/codeunits"
	"github.com/hansjlachmann/goeasyerp/backend/foundation/i18n"
	"github.com/hansjlachmann/goeasyerp/backend/foundation/system"
	"github.com/hansjlachmann/goeasyerp/backend/foundation/types"
	gtables "github.com/hansjlachmann/goeasyerp/backend/generated/tables"
)

// Report 116 Customer Statement (from the classic US Customer Statement): a page per customer
// with the company and customer addresses and the statement date (the end of the Date Filter,
// else the work date) — as Open Item the customer's open entries up to that date with their
// remaining amounts, as Balance the balance brought forward from before the period and the
// period's entries — with the running balance, the balance due and how much of it is current,
// 1–30, 31–60 and over 60 days past due. Customers are printed with a balance (Print All with
// Balance) or with entries in the period (Print All with Entries); Update Statement No. raises
// the customer's Last Statement No. (not in a preview). Amounts in LCY.
const CustomerStatementID = 116

func init() {
	Register(CustomerStatementID, "customer-statement", func() Report { return &customerStatement{} })
}

type customerStatement struct {
	openItem    bool
	withBalance bool
	withEntries bool
	updateNo    bool
	fromDate    types.Date
	toDate      types.Date
	customer    *gtables.Customer
	lines       []statementLine
	balance     types.Decimal
	agingDue    [4]types.Decimal // current, 1–30, 31–60, over 60 days past due
}

// statementLine is a line of the statement: an entry, or the balance brought forward
type statementLine struct {
	date        types.Date
	docType     string
	docNo       types.Code
	description string
	dueDate     types.Date
	amount      types.Decimal
	forward     bool
}

func (r *customerStatement) Definition() *Definition {
	return &Definition{
		OptionsCaption: "OPTIONS",
		Options: []Option{
			{Name: "statement_style", Type: "option", Default: "1", Options: []string{"OPEN_ITEM", "BALANCE"}},
			{Name: "print_all_with_balance", Type: "bool", Default: "true"},
			{Name: "print_all_with_entries", Type: "bool", Default: "false"},
			{Name: "print_company_address", Type: "bool", Default: "true"},
			{Name: "update_statement_no", Type: "bool", Default: "false"},
		},
		OnPreReport: r.onPreReport,
		DataItems: []*DataItem{{
			Name:                "Customer",
			Table:               gtables.CustomerTableName,
			Key:                 []string{"no"},
			RequestFilterFields: []string{"no", "print_statements", "date_filter"},
			OnAfterGetRecord:    Trigger(r.customerOnAfterGetRecord),
			DataItems: []*DataItem{{
				Name:             "StatementLine",
				Table:            IntegerTable,
				OnPreDataItem:    Trigger(r.lineOnPreDataItem),
				OnAfterGetRecord: Trigger(r.lineOnAfterGetRecord),
				OnPostDataItem:   Trigger(r.lineOnPostDataItem),
			}},
		}},
	}
}

func (r *customerStatement) onPreReport(run *Run) error {
	r.openItem = run.OptionInt("statement_style") == 1
	r.withBalance = run.OptionBool("print_all_with_balance")
	r.withEntries = run.OptionBool("print_all_with_entries")
	r.updateNo = run.OptionBool("update_statement_no")
	setCurrencyValue(run, run, "")
	return setCompanyValues(run)
}

// customerOnAfterGetRecord: the customer's lines, balance and aging — skipped when the options
// leave it out
func (r *customerStatement) customerOnAfterGetRecord(cust *gtables.Customer, run *Run) error {
	r.customer = cust
	r.fromDate, r.toDate = dateFilterRange(cust.Date_filter)
	if r.toDate.IsZero() {
		r.toDate = system.WorkDate()
	}
	r.lines = nil
	r.agingDue = [4]types.Decimal{}
	hasEntries := false

	entries := gtables.NewCustomerLedgerEntry(run)
	entries.SetCurrentKey("customer_no", "posting_date")
	entries.SetRange("customer_no", cust.No)
	if r.openItem {
		entries.SetRange("open", true)
		entries.SetRange("posting_date", types.Date{}, r.toDate)
		for found := entries.FindSetBuffered(); found; found = entries.Next() {
			r.lines = append(r.lines, r.entryLine(entries, entries.Remaining_amt_lcy))
			if !r.fromDate.IsZero() && !entries.Posting_date.Before(r.fromDate) {
				hasEntries = true
			}
		}
	} else {
		if !r.fromDate.IsZero() {
			forward := gtables.NewCustomerLedgerEntry(run)
			forward.SetRange("customer_no", cust.No)
			forward.SetRange("posting_date", types.Date{}, r.fromDate.AddDays(-1))
			err := forward.CalcSums("amount_lcy")
			if err != nil {
				return err
			}
			r.lines = append(r.lines, statementLine{date: r.fromDate.AddDays(-1), description: run.Text("BALANCE_FORWARD"), amount: forward.Amount_lcy, forward: true})
		}
		entries.SetRange("posting_date", r.fromDate, r.toDate)
		for found := entries.FindSetBuffered(); found; found = entries.Next() {
			r.lines = append(r.lines, r.entryLine(entries, entries.Amount_lcy))
			hasEntries = true
		}
	}
	r.balance = types.Decimal{}
	for _, l := range r.lines {
		r.balance = r.balance.Add(l.amount)
	}
	if !(r.withBalance && !r.balance.IsZero()) && !(r.withEntries && hasEntries) {
		return Skip()
	}

	// the aging of the balance due: the open entries by days past due on the statement date
	open := gtables.NewCustomerLedgerEntry(run)
	open.SetRange("customer_no", cust.No)
	open.SetRange("open", true)
	open.SetRange("posting_date", types.Date{}, r.toDate)
	for found := open.FindSetBuffered(); found; found = open.Next() {
		days := open.Due_date.DaysBetween(r.toDate)
		bucket := 0
		switch {
		case open.Due_date.IsZero() || days <= 0:
			bucket = 0
		case days <= 30:
			bucket = 1
		case days <= 60:
			bucket = 2
		default:
			bucket = 3
		}
		r.agingDue[bucket] = r.agingDue[bucket].Add(open.Remaining_amt_lcy)
	}

	// the footer's values (it prints after the last line, before OnPostDataItem)
	var debitTotal, creditTotal types.Decimal
	for _, l := range r.lines {
		if l.forward {
			continue
		}
		debit, credit := debitCredit(l.amount)
		debitTotal, creditTotal = debitTotal.Add(debit), creditTotal.Add(credit)
	}
	run.SetValue("total_debit", debitTotal)
	run.SetValue("total_credit", creditTotal)
	run.SetValue("balance_due", r.balance)
	for i, name := range []string{"current", "due_1_30", "due_31_60", "due_over_60"} {
		run.SetValue(name, r.agingDue[i])
	}

	custAddress, err := formataddress.Customer(cust)
	if err != nil {
		return err
	}
	run.SetValue("customer_address", custAddress)
	run.SetValue("statement_date", r.toDate)
	return nil
}

func (r *customerStatement) entryLine(e *gtables.CustomerLedgerEntry, amount types.Decimal) statementLine {
	docType := ""
	if e.Document_type != gtables.CustomerLedgerEntry_Document_type.Blank {
		docType = e.GetOptionCaption("document_type", i18n.Key(e.Document_type.String()), fcodeunits.CurrentLanguage())
	}
	return statementLine{date: e.Posting_date, docType: docType, docNo: e.Document_no, description: e.Description.String(), dueDate: e.Due_date, amount: amount}
}

func (r *customerStatement) lineOnPreDataItem(rec *Integer, run *Run) error {
	if len(r.lines) == 0 {
		return Break()
	}
	rec.SetRange("number", 1, len(r.lines))
	r.balance = types.Decimal{}
	return nil
}

// lineOnAfterGetRecord: a line with its debit or credit and the running balance
func (r *customerStatement) lineOnAfterGetRecord(rec *Integer, run *Run) error {
	l := r.lines[rec.Number-1]
	r.balance = r.balance.Add(l.amount)
	debit, credit := debitCredit(l.amount)
	if l.forward {
		debit, credit = types.Decimal{}, types.Decimal{}
	}
	run.SetValue("line_date", l.date)
	run.SetValue("line_document_type", l.docType)
	run.SetValue("line_document_no", l.docNo.String())
	run.SetValue("line_description", l.description)
	run.SetValue("line_description_only", l.forward)
	run.SetValue("line_due_date", l.dueDate)
	run.SetValue("line_debit", debit)
	run.SetValue("line_credit", credit)
	run.SetValue("line_balance", r.balance)
	return nil
}

// lineOnPostDataItem: Update Statement No. (not in a preview)
func (r *customerStatement) lineOnPostDataItem(rec *Integer, run *Run) error {
	if r.updateNo && !run.Preview() {
		cust := gtables.NewCustomer(run)
		err := cust.GetOrError(r.customer.No)
		if err != nil {
			return err
		}
		cust.Last_statement_no++
		return cust.ModifyOrError(false)
	}
	return nil
}
