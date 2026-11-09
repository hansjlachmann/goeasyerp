package reports

import (
	"github.com/hansjlachmann/goeasyerp/backend/business-logic/formataddress"
	fcodeunits "github.com/hansjlachmann/goeasyerp/backend/foundation/codeunits"
	"github.com/hansjlachmann/goeasyerp/backend/foundation/types"
	gtables "github.com/hansjlachmann/goeasyerp/backend/generated/tables"
)

// The test reports of reminders and finance charge memos (Issuing › Test Report on the cards and
// lists; Report Selections usage Rem.Test / F.C.Test):
//   - 122 Reminder - Test and 123 Finance Charge Memo - Test: per document what issuing would
//     refuse — the customer (blocked, another currency), the posting date (allowed range), the
//     document and due dates, the customer posting group, lines of customer ledger entries with
//     a negative amount — with the beginning text, the lines, the totals (remaining amount,
//     interest, tax), the ending text and the tax per rate.
const (
	ReminderTestID          = 122
	FinanceChargeMemoTestID = 123
)

func init() {
	Register(ReminderTestID, "reminder-test", func() Report { return &reminderTest{reminders: true} })
	Register(FinanceChargeMemoTestID, "finance-charge-memo-test", func() Report { return &reminderTest{} })
	DefaultReports[gtables.ReportSelections_Usage.RemTest] = ReminderTestID
	DefaultReports[gtables.ReportSelections_Usage.FCTest] = FinanceChargeMemoTestID
}

// reminderTestHeader: the fields of a reminder or finance charge memo header the report checks
type reminderTestHeader struct {
	rec                                filteredRecord
	no, customerNo, currencyCode       types.Code
	custPostingGroup                   types.Code
	postingDate, documentDate, dueDate types.Date
	yourReference                      types.Text
}

// reminderTestLine is one line of the document as the report prints it
type reminderTestLine struct {
	lineNo                                  int
	entry                                   bool // Type Customer Ledger Entry
	blank                                   bool // Type blank
	description                             types.Text
	documentType                            string
	documentNo                              types.Code
	documentDate, dueDate                   types.Date
	originalAmount, remainingAmount, amount types.Decimal
	taxPct, taxAmount                       types.Decimal
	errors                                  []string
}

type reminderTest struct {
	documentTest
	reminders           bool
	header              reminderTestHeader
	lines, endLines     []reminderTestLine
	remaining, interest types.Decimal // reminder: Remaining Amount (G/L lines their amount) and the interest
	total, taxTotal     types.Decimal
}

func (r *reminderTest) Definition() *Definition {
	headerTable := gtables.ReminderHeaderTableName
	headerName := "Reminder_Header"
	if !r.reminders {
		headerTable = gtables.FinanceChargeMemoHeaderTableName
		headerName = "Finance_Charge_Memo_Header"
	}
	header := &DataItem{
		Name: headerName, Table: headerTable, Key: []string{"no"},
		RequestFilterFields: []string{"no"},
		DataItems: []*DataItem{
			r.errorLoop("Header_Error_Loop", &r.headerErrors),
			r.linesDataItem(),
			r.endTextDataItem(),
			r.taxCounter(),
		},
	}
	if r.reminders {
		header.OnPreDataItem = Trigger(func(rec *gtables.ReminderHeader, run *Run) error {
			setFilterText(run, rec)
			return nil
		})
		header.OnAfterGetRecord = Trigger(func(rec *gtables.ReminderHeader, run *Run) error {
			addr, err := formataddress.FormatAddr(rec, rec.Name.String(), rec.Name_2.String(), rec.Contact.String(), rec.Address.String(),
				rec.Address_2.String(), rec.City.String(), rec.Post_code, rec.State.String(), rec.Country_code)
			if err != nil {
				return err
			}
			run.SetValue("customer_address", addr)
			r.header = reminderTestHeader{rec: rec, no: rec.No, customerNo: rec.Customer_no, currencyCode: rec.Currency_code,
				custPostingGroup: rec.Customer_posting_group, postingDate: rec.Posting_date, documentDate: rec.Document_date,
				dueDate: rec.Due_date, yourReference: rec.Your_reference}
			r.readReminderLines(rec)
			return r.headerOnAfterGetRecord(run)
		})
	} else {
		header.OnPreDataItem = Trigger(func(rec *gtables.FinanceChargeMemoHeader, run *Run) error {
			setFilterText(run, rec)
			return nil
		})
		header.OnAfterGetRecord = Trigger(func(rec *gtables.FinanceChargeMemoHeader, run *Run) error {
			addr, err := formataddress.FormatAddr(rec, rec.Name.String(), rec.Name_2.String(), rec.Contact.String(), rec.Address.String(),
				rec.Address_2.String(), rec.City.String(), rec.Post_code, rec.State.String(), rec.Country_code)
			if err != nil {
				return err
			}
			run.SetValue("customer_address", addr)
			r.header = reminderTestHeader{rec: rec, no: rec.No, customerNo: rec.Customer_no, currencyCode: rec.Currency_code,
				custPostingGroup: rec.Customer_posting_group, postingDate: rec.Posting_date, documentDate: rec.Document_date,
				dueDate: rec.Due_date, yourReference: rec.Your_reference}
			r.readFinChrgMemoLines(rec)
			return r.headerOnAfterGetRecord(run)
		})
	}
	return &Definition{
		ContextTable: headerTable,
		OnInitReport: func(run *Run) error {
			switch rec := run.Context().(type) {
			case *gtables.ReminderHeader:
				if rec.No != "" {
					run.SetRequestFilter(headerName, "no", rec.No.String())
				}
			case *gtables.FinanceChargeMemoHeader:
				if rec.No != "" {
					run.SetRequestFilter(headerName, "no", rec.No.String())
				}
			}
			return nil
		},
		OnPreReport: func(run *Run) error {
			r.language = fcodeunits.CurrentLanguage()
			setReportHeadValues(run)
			return nil
		},
		DataItems: []*DataItem{header},
	}
}

// headerOnAfterGetRecord: the head's checks, texts and totals
func (r *reminderTest) headerOnAfterGetRecord(run *Run) error {
	h := r.header
	r.headerErrors = nil
	run.SetPageNo(1)
	caption := func(field string) string { return h.rec.GetFieldCaption(field, r.language) }

	cust := gtables.NewCustomer(run)
	if h.customerNo == "" {
		r.headerErrors = append(r.headerErrors, run.Text("MUST_BE_SPECIFIED", caption("customer_no")))
	} else {
		if cust.Get(h.customerNo) {
			if cust.Blocked {
				r.headerErrors = append(r.headerErrors, r.mustBeFor(run, cust.GetFieldCaption("blocked", r.language), false, cust.TableCaption(r.language), h.customerNo))
			}
			if cust.Currency_code != h.currencyCode {
				r.headerErrors = append(r.headerErrors, run.Text("MUST_BE", caption("currency_code"), cust.Currency_code))
			}
		} else {
			r.headerErrors = append(r.headerErrors, run.Text("NOT_EXIST", cust.TableCaption(r.language), h.customerNo))
		}
	}
	if h.postingDate.IsZero() {
		r.headerErrors = append(r.headerErrors, run.Text("MUST_BE_SPECIFIED", caption("posting_date")))
	} else {
		r.checkPostingDate(run, h.postingDate, caption("posting_date"))
	}
	if h.documentDate.IsZero() {
		r.headerErrors = append(r.headerErrors, run.Text("MUST_BE_SPECIFIED", caption("document_date")))
	}
	if h.dueDate.IsZero() {
		r.headerErrors = append(r.headerErrors, run.Text("MUST_BE_SPECIFIED", caption("due_date")))
	}
	if h.custPostingGroup == "" {
		r.headerErrors = append(r.headerErrors, run.Text("MUST_BE_SPECIFIED", caption("customer_posting_group")))
	}
	if h.currencyCode == "" {
		run.SetValue("total_text", run.Text("TOTAL"))
		run.SetValue("total_incl_tax_text", run.Text("TOTAL_INCL_TAX"))
	} else {
		run.SetValue("total_text", run.Text("TOTAL_CURRENCY", h.currencyCode))
		run.SetValue("total_incl_tax_text", run.Text("TOTAL_CURRENCY_INCL_TAX", h.currencyCode))
	}
	run.SetValue("document_heading", h.no.String()+" "+cust.Name.String())
	if h.yourReference == "" {
		run.SetValue("reference_text", "")
	} else {
		run.SetValue("reference_text", caption("your_reference"))
	}

	// the lines' errors, totals and tax per rate
	r.taxLines = map[string]*taxAmountLine{}
	r.remaining, r.interest, r.total, r.taxTotal = types.ZeroDecimal(), types.ZeroDecimal(), types.ZeroDecimal(), types.ZeroDecimal()
	for i := range r.lines {
		line := &r.lines[i]
		r.addTaxLine(line.taxPct, line.amount, line.amount.Add(line.taxAmount))
		r.taxTotal = r.taxTotal.Add(line.taxAmount)
		r.total = r.total.Add(line.amount)
		if line.entry {
			if r.reminders {
				r.interest = r.interest.Add(line.amount)
			}
			if line.amount.IsNegative() {
				line.errors = append(line.errors, run.Text("MUST_BE_POSITIVE_OR_ZERO", r.lineCaption(run, "amount")))
			}
		} else {
			if r.reminders && !line.blank {
				line.remainingAmount = line.amount
			}
		}
		if r.reminders {
			r.remaining = r.remaining.Add(line.remainingAmount)
		}
	}
	totalAmount := r.total
	if r.reminders {
		totalAmount = r.remaining.Add(r.interest)
		run.SetValue("interest_amount", r.interest)
	}
	run.SetValue("total_amount", totalAmount)
	run.SetValue("tax_total", r.taxTotal)
	run.SetValue("total_incl_tax", totalAmount.Add(r.taxTotal))
	return nil
}

func (r *reminderTest) lineCaption(run *Run, field string) string {
	if r.reminders {
		return gtables.NewReminderLine(run).GetFieldCaption(field, r.language)
	}
	return gtables.NewFinanceChargeMemoLine(run).GetFieldCaption(field, r.language)
}

// splitTexts: the beginning text (blank lines before the first other line, up to the last
// empty one) and the ending text (blank lines after the last other line, from the last empty
// one) — classic StartLineNo/EndLineNo; the ending text is printed after the totals
func (r *reminderTest) splitTexts(all []reminderTestLine) {
	startLineNo := 0
	for _, line := range all {
		if !line.blank {
			break
		}
		if line.description == "" {
			startLineNo = line.lineNo
		}
	}
	endLineNo := 0
	if len(all) > 0 {
		endLineNo = all[len(all)-1].lineNo + 1
	}
	for i := len(all) - 1; i >= 0; i-- {
		if !all[i].blank {
			break
		}
		if all[i].description == "" {
			endLineNo = all[i].lineNo
		}
	}
	r.lines, r.endLines = nil, nil
	for _, line := range all {
		if line.lineNo >= endLineNo {
			r.endLines = append(r.endLines, line)
			continue
		}
		if line.lineNo <= startLineNo {
			line.entry = false
			line.blank = true
		}
		r.lines = append(r.lines, line)
	}
}

func (r *reminderTest) readReminderLines(rec *gtables.ReminderHeader) {
	var all []reminderTestLine
	reminderLine := gtables.NewReminderLine(rec)
	reminderLine.SetRange("reminder_no", rec.No)
	for found := reminderLine.FindSetBuffered(); found; found = reminderLine.Next() {
		all = append(all, reminderTestLine{
			lineNo: reminderLine.Line_no, entry: reminderLine.Type == gtables.ReminderLine_Type.CustomerLedgerEntry,
			blank: reminderLine.Type == gtables.ReminderLine_Type.Blank, description: reminderLine.Description,
			documentType: optionCaption(gtables.ReminderLineTableName, "document_type", reminderLine.Document_type.String(), r.language),
			documentNo:   reminderLine.Document_no, documentDate: reminderLine.Document_date, dueDate: reminderLine.Due_date,
			originalAmount: reminderLine.Original_amount, remainingAmount: reminderLine.Remaining_amount, amount: reminderLine.Amount,
			taxPct: reminderLine.Tax_percent, taxAmount: reminderLine.Tax_amount,
		})
	}
	r.splitTexts(all)
}

func (r *reminderTest) readFinChrgMemoLines(rec *gtables.FinanceChargeMemoHeader) {
	var all []reminderTestLine
	finChrgMemoLine := gtables.NewFinanceChargeMemoLine(rec)
	finChrgMemoLine.SetRange("finance_charge_memo_no", rec.No)
	for found := finChrgMemoLine.FindSetBuffered(); found; found = finChrgMemoLine.Next() {
		all = append(all, reminderTestLine{
			lineNo: finChrgMemoLine.Line_no, entry: finChrgMemoLine.Type == gtables.FinanceChargeMemoLine_Type.CustomerLedgerEntry,
			blank: finChrgMemoLine.Type == gtables.FinanceChargeMemoLine_Type.Blank, description: finChrgMemoLine.Description,
			documentType: optionCaption(gtables.FinanceChargeMemoLineTableName, "document_type", finChrgMemoLine.Document_type.String(), r.language),
			documentNo:   finChrgMemoLine.Document_no, documentDate: finChrgMemoLine.Document_date, dueDate: finChrgMemoLine.Due_date,
			originalAmount: finChrgMemoLine.Original_amount, remainingAmount: finChrgMemoLine.Remaining_amount, amount: finChrgMemoLine.Amount,
			taxPct: finChrgMemoLine.Tax_percent, taxAmount: finChrgMemoLine.Tax_amount,
		})
	}
	r.splitTexts(all)
}

// linesDataItem: the beginning text and the lines, each followed by its warnings
func (r *reminderTest) linesDataItem() *DataItem {
	return &DataItem{
		Name: "Lines", Table: IntegerTable,
		OnPreDataItem: Trigger(func(rec *Integer, run *Run) error {
			rec.SetRange("number", 1, len(r.lines))
			return nil
		}),
		OnAfterGetRecord: Trigger(func(rec *Integer, run *Run) error {
			line := r.lines[rec.Number-1]
			r.lineErrors = line.errors
			run.SetValue("is_text", line.blank)
			run.SetValue("is_entry", line.entry)
			run.SetValue("is_other", !line.blank && !line.entry)
			run.SetValue("description", line.description)
			run.SetValue("document_date", line.documentDate)
			run.SetValue("document_type", line.documentType)
			run.SetValue("document_no", line.documentNo)
			run.SetValue("due_date", line.dueDate)
			run.SetValue("original_amount", line.originalAmount)
			run.SetValue("remaining_amount", line.remainingAmount)
			run.SetValue("amount", line.amount)
			return nil
		}),
		DataItems: []*DataItem{r.errorLoop("Line_Error_Loop", &r.lineErrors)},
	}
}

// endTextDataItem: the ending text after the totals
func (r *reminderTest) endTextDataItem() *DataItem {
	return &DataItem{
		Name: "End_Text", Table: IntegerTable,
		OnPreDataItem: Trigger(func(rec *Integer, run *Run) error {
			rec.SetRange("number", 1, len(r.endLines))
			return nil
		}),
		OnAfterGetRecord: Trigger(func(rec *Integer, run *Run) error {
			run.SetValue("end_description", r.endLines[rec.Number-1].description)
			return nil
		}),
	}
}
