package reports

import (
	"github.com/hansjlachmann/openerp/backend/business-logic/formataddress"
	"github.com/hansjlachmann/openerp/backend/foundation/types"
	gtables "github.com/hansjlachmann/openerp/backend/generated/tables"
)

// The printouts of the issued documents — report 117 Reminder and 118 Finance Charge Memo: the
// company, the customer's address, the document's dates, the lines (texts, the customer entries
// with their remaining amounts and interest, the additional fee) and the totals — remaining
// amount, interest, additional fee, tax and the total —, in copies; a print that is not a
// preview counts in No. Printed (IncrNoPrinted). Run from an issued document it prints that
// document. Layouts: backend/definitions/reports/117-reminder.yaml, 118-finance-charge-memo.yaml.
const (
	ReminderPrintID          = 117
	FinanceChargeMemoPrintID = 118
)

func init() {
	Register(ReminderPrintID, "reminder", func() Report { return &reminderPrint{} })
	Register(FinanceChargeMemoPrintID, "finance-charge-memo", func() Report { return &finChrgMemoPrint{} })
}

// setReminderTotals: the document's totals (remaining amount, interest, fee, tax, total)
func setReminderTotals(run *Run, remaining, interest, fee, tax types.Decimal) {
	run.SetValue("remaining_amount", remaining)
	run.SetValue("interest_amount", interest)
	run.SetValue("additional_fee", fee)
	run.SetValue("tax_amount", tax)
	run.SetValue("total", remaining.Add(interest).Add(fee).Add(tax))
}

// setReminderLineValues: what a line prints — a text line its description only
func setReminderLineValues(run *Run, isText bool, hasEntry bool) {
	run.SetValue("text_line", isText)
	run.SetValue("entry_line", hasEntry)
	run.SetValue("amount_line", !isText && !hasEntry)
}

// Report 117 Reminder

type reminderPrint struct {
	postedDocument
	header *gtables.IssuedReminderHeader
}

func (r *reminderPrint) Definition() *Definition {
	return &Definition{
		Options:      postedDocumentOptions(),
		ContextTable: gtables.IssuedReminderHeaderTableName,
		OnInitReport: func(run *Run) error {
			h, ok := run.Context().(*gtables.IssuedReminderHeader)
			if ok && h.No != "" {
				run.SetRequestFilter("IssuedReminderHeader", "no", h.No.String())
			}
			return nil
		},
		OnPreReport: setCompanyValues,
		DataItems: []*DataItem{{
			Name:                "IssuedReminderHeader",
			Table:               gtables.IssuedReminderHeaderTableName,
			Key:                 []string{"no"},
			RequestFilterFields: []string{"no", "customer_no", "no_printed"},
			OnAfterGetRecord:    Trigger(r.headerOnAfterGetRecord),
			DataItems: []*DataItem{{
				Name:             "CopyLoop",
				Table:            IntegerTable,
				OnPreDataItem:    Trigger(r.copyLoopOnPreDataItem),
				OnAfterGetRecord: Trigger(r.copyLoopOnAfterGetRecord),
				OnPostDataItem: Trigger(func(rec *Integer, run *Run) error {
					if !run.Preview() {
						return incrReminderNoPrinted(r.header)
					}
					return nil
				}),
				DataItems: []*DataItem{{
					Name:  "IssuedReminderLine",
					Table: gtables.IssuedReminderLineTableName,
					Key:   []string{"reminder_no", "line_no"},
					OnPreDataItem: Trigger(func(rec *gtables.IssuedReminderLine, run *Run) error {
						rec.SetRange("reminder_no", r.header.No)
						return nil
					}),
					OnAfterGetRecord: Trigger(func(rec *gtables.IssuedReminderLine, run *Run) error {
						setReminderLineValues(run, rec.Type == gtables.IssuedReminderLine_Type.Blank, rec.Entry_no != 0)
						return nil
					}),
				}},
			}},
		}},
	}
}

func (r *reminderPrint) headerOnAfterGetRecord(rec *gtables.IssuedReminderHeader, run *Run) error {
	r.header = rec
	custAddress, err := formataddress.IssuedReminder(rec)
	if err != nil {
		return err
	}
	run.SetValue("customer_address", custAddress)
	run.SetValue("your_reference", rec.Your_reference.String())
	setCurrencyValue(run, rec, rec.Currency_code)
	rec.CalcFields("remaining_amount", "interest_amount", "additional_fee", "tax_amount")
	setReminderTotals(run, rec.Remaining_amount, rec.Interest_amount, rec.Additional_fee, rec.Tax_amount)
	r.headerPrinted(run, rec.No)
	return nil
}

// incrReminderNoPrinted (codeunit 393 IncrNoPrinted)
func incrReminderNoPrinted(rec *gtables.IssuedReminderHeader) error {
	h := gtables.NewIssuedReminderHeader(rec)
	err := h.GetOrError(rec.No)
	if err != nil {
		return err
	}
	h.No_printed = h.No_printed + 1
	return h.ModifyOrError(false)
}

// Report 118 Finance Charge Memo

type finChrgMemoPrint struct {
	postedDocument
	header *gtables.IssuedFinChargeMemoHeader
}

func (r *finChrgMemoPrint) Definition() *Definition {
	return &Definition{
		Options:      postedDocumentOptions(),
		ContextTable: gtables.IssuedFinChargeMemoHeaderTableName,
		OnInitReport: func(run *Run) error {
			h, ok := run.Context().(*gtables.IssuedFinChargeMemoHeader)
			if ok && h.No != "" {
				run.SetRequestFilter("IssuedFinChargeMemoHeader", "no", h.No.String())
			}
			return nil
		},
		OnPreReport: setCompanyValues,
		DataItems: []*DataItem{{
			Name:                "IssuedFinChargeMemoHeader",
			Table:               gtables.IssuedFinChargeMemoHeaderTableName,
			Key:                 []string{"no"},
			RequestFilterFields: []string{"no", "customer_no", "no_printed"},
			OnAfterGetRecord:    Trigger(r.headerOnAfterGetRecord),
			DataItems: []*DataItem{{
				Name:             "CopyLoop",
				Table:            IntegerTable,
				OnPreDataItem:    Trigger(r.copyLoopOnPreDataItem),
				OnAfterGetRecord: Trigger(r.copyLoopOnAfterGetRecord),
				OnPostDataItem: Trigger(func(rec *Integer, run *Run) error {
					if !run.Preview() {
						return incrFinChrgMemoNoPrinted(r.header)
					}
					return nil
				}),
				DataItems: []*DataItem{{
					Name:  "IssuedFinChargeMemoLine",
					Table: gtables.IssuedFinChargeMemoLineTableName,
					Key:   []string{"finance_charge_memo_no", "line_no"},
					OnPreDataItem: Trigger(func(rec *gtables.IssuedFinChargeMemoLine, run *Run) error {
						rec.SetRange("finance_charge_memo_no", r.header.No)
						return nil
					}),
					OnAfterGetRecord: Trigger(func(rec *gtables.IssuedFinChargeMemoLine, run *Run) error {
						setReminderLineValues(run, rec.Type == gtables.IssuedFinChargeMemoLine_Type.Blank, rec.Entry_no != 0)
						return nil
					}),
				}},
			}},
		}},
	}
}

func (r *finChrgMemoPrint) headerOnAfterGetRecord(rec *gtables.IssuedFinChargeMemoHeader, run *Run) error {
	r.header = rec
	custAddress, err := formataddress.IssuedFinChrgMemo(rec)
	if err != nil {
		return err
	}
	run.SetValue("customer_address", custAddress)
	run.SetValue("your_reference", rec.Your_reference.String())
	setCurrencyValue(run, rec, rec.Currency_code)
	rec.CalcFields("remaining_amount", "interest_amount", "additional_fee", "tax_amount")
	setReminderTotals(run, rec.Remaining_amount, rec.Interest_amount, rec.Additional_fee, rec.Tax_amount)
	r.headerPrinted(run, rec.No)
	return nil
}

// incrFinChrgMemoNoPrinted (codeunit 395 IncrNoPrinted)
func incrFinChrgMemoNoPrinted(rec *gtables.IssuedFinChargeMemoHeader) error {
	h := gtables.NewIssuedFinChargeMemoHeader(rec)
	err := h.GetOrError(rec.No)
	if err != nil {
		return err
	}
	h.No_printed = h.No_printed + 1
	return h.ModifyOrError(false)
}
