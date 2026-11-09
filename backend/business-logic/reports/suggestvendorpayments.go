package reports

import (
	"sort"

	"github.com/hansjlachmann/goeasyerp/backend/business-logic/noseries"
	"github.com/hansjlachmann/goeasyerp/backend/foundation/system"
	"github.com/hansjlachmann/goeasyerp/backend/foundation/types"
	gtables "github.com/hansjlachmann/goeasyerp/backend/generated/tables"
)

// Report 393 Suggest Vendor Payments (Payments › Suggest Vendor Payments on the payment
// journal): the vendors' open entries due up to the Last Payment Date — and, with Find Payment
// Discounts, the later ones whose payment discount date falls within the period — become payment
// lines of the journal, one per entry (or one per vendor and currency with Summarize per
// Vendor, applied by Applies-to ID), balanced on the bank or G/L account chosen. With an
// Available Amount the vendors are paid by priority until it is used up. A vendor's entries in a
// currency are paid only when they add up to a payment (a credit balance is left out).
const SuggestVendorPaymentsID = 393

func init() {
	Register(SuggestVendorPaymentsID, "suggest-vendor-payments", func() Report { return &suggestVendorPayments{} })
}

// payableVendLedgEntry is a payment found for an entry (classic temporary table 317 Payable
// Vendor Ledger Entry)
type payableVendLedgEntry struct {
	priority          int
	vendorNo          types.Code
	entryNo           int
	vendorLedgEntryNo int
	amount, amountLCY types.Decimal
	currencyCode      types.Code
	positive, future  bool
	deleted           bool
}

type suggestVendorPayments struct {
	genJnlLine                         *gtables.GenJournalLine
	genJnlTemplate                     *gtables.GenJournalTemplate
	genJnlBatch                        *gtables.GenJournalBatch
	payable                            []*payableVendLedgEntry
	usePaymentDisc, usePriority        bool
	summarizePerVend, docNoPerLine     bool
	postingDate, lastDueDateToPayReq   types.Date
	nextDocNo                          types.Code
	amountAvailable, originalAmtAvail  types.Decimal
	balAccountType                     gtables.GenJournalLineBal_account_type
	balAccountNo                       types.Code
	bankPaymentType                    gtables.GenJournalLineBank_payment_type
	nextEntryNo, firstLineNo, lastLine int
	stopPayments                       bool
	vend2                              *gtables.Vendor
}

func (r *suggestVendorPayments) Definition() *Definition {
	return &Definition{
		OptionsCaption: "OPTIONS",
		Options: []Option{
			{Name: "last_payment_date", Type: "date", Required: true},
			{Name: "find_payment_discounts", Type: "bool", Default: "false"},
			{Name: "use_vendor_priority", Type: "bool", Default: "false"},
			{Name: "available_amount", Type: "number"},
			{Name: "posting_date", Type: "date", Required: true},
			{Name: "starting_document_no", Type: "text"},
			{Name: "new_doc_no_per_line", Type: "bool", Default: "false"},
			{Name: "summarize_per_vendor", Type: "bool", Default: "false"},
			{Name: "bal_account_type", Type: "option", Default: "2", Options: []string{"GL_ACCOUNT", "BANK_ACCOUNT"}},
			{Name: "bal_account_no", Type: "text"},
			{Name: "bank_payment_type", Type: "option", Default: "1", Options: []string{"BLANK", "COMPUTER_CHECK", "MANUAL_CHECK"}},
		},
		ContextTable: gtables.GenJournalLineTableName,
		Transaction:  true,
		OnInitReport: r.onInitReport,
		DataItems: []*DataItem{{
			Name: "Vendor", Table: gtables.VendorTableName, Key: []string{"no"},
			RequestFilterFields: []string{"no", "payment_method_code"},
			OnPreDataItem:       Trigger(r.vendorOnPreDataItem),
			OnAfterGetRecord:    Trigger(r.vendorOnAfterGetRecord),
			OnPostDataItem:      Trigger(r.vendorOnPostDataItem),
		}},
	}
}

// onInitReport (SetGenJnlLine, request page OnOpenForm): the journal it runs from, the work date
// as posting date and the batch's next number as starting document no.
func (r *suggestVendorPayments) onInitReport(run *Run) error {
	genJnlLine, ok := run.Context().(*gtables.GenJournalLine)
	if !ok {
		return system.Error(run.Text("NO_JOURNAL"))
	}
	r.genJnlLine = genJnlLine
	if genJnlLine.Journal_template_name == "" {
		genJnlLine.Journal_template_name = genJnlLineFilter(genJnlLine, "journal_template_name")
	}
	if genJnlLine.Journal_batch_name == "" {
		genJnlLine.Journal_batch_name = genJnlLineFilter(genJnlLine, "journal_batch_name")
	}
	run.SetOption("posting_date", system.WorkDate().String())
	genJnlBatch := gtables.NewGenJournalBatch(run)
	err := genJnlBatch.GetOrError(map[string]interface{}{"journal_template_name": genJnlLine.Journal_template_name, "name": genJnlLine.Journal_batch_name})
	if err != nil {
		return err
	}
	if genJnlBatch.Bal_account_no != "" {
		run.SetOption("bal_account_no", genJnlBatch.Bal_account_no.String())
	}
	if genJnlBatch.No_series != "" {
		noSeriesMgt := noseries.New(run)
		nextDocNo, err := noSeriesMgt.GetNextNo(genJnlBatch.No_series, system.WorkDate(), false)
		if err != nil {
			return err
		}
		run.SetOption("starting_document_no", nextDocNo.String())
	}
	return nil
}

// genJnlLineFilter is the exact value a journal line is filtered on (the page's filters)
func genJnlLineFilter(genJnlLine *gtables.GenJournalLine, field string) types.Code {
	genJnlLine.FilterGroup(2)
	value := genJnlLine.GetFilter(field)
	genJnlLine.FilterGroup(0)
	return types.Code(value)
}

func (r *suggestVendorPayments) vendorOnPreDataItem(vendor *gtables.Vendor, run *Run) error {
	r.lastDueDateToPayReq = run.OptionDate("last_payment_date")
	r.postingDate = run.OptionDate("posting_date")
	r.usePaymentDisc = run.OptionBool("find_payment_discounts")
	r.usePriority = run.OptionBool("use_vendor_priority")
	r.amountAvailable = run.OptionDecimal("available_amount")
	r.nextDocNo = types.Code(run.Option("starting_document_no"))
	r.docNoPerLine = run.OptionBool("new_doc_no_per_line")
	r.summarizePerVend = run.OptionBool("summarize_per_vendor")
	r.balAccountType = gtables.GenJournalLine_Bal_account_type.GLAccount
	if run.OptionInt("bal_account_type") == 2 {
		r.balAccountType = gtables.GenJournalLine_Bal_account_type.BankAccount
	}
	r.balAccountNo = types.Code(run.Option("bal_account_no"))
	r.bankPaymentType = gtables.GenJournalLineBank_payment_type(run.OptionInt("bank_payment_type") - 1)

	if r.lastDueDateToPayReq.IsZero() {
		return system.Error(run.Text("ENTER_LAST_PAYMENT_DATE"))
	}
	if r.postingDate.IsZero() {
		return system.Error(run.Text("ENTER_POSTING_DATE"))
	}
	if r.balAccountType != gtables.GenJournalLine_Bal_account_type.BankAccount &&
		r.bankPaymentType != gtables.GenJournalLine_Bank_payment_type.Blank {
		return system.Error(run.Text("BANK_PAYMENT_TYPE_ONLY_BANK"))
	}
	if r.balAccountNo != "" {
		if r.balAccountType == gtables.GenJournalLine_Bal_account_type.BankAccount {
			bankAcc := gtables.NewBankAccount(run)
			err := bankAcc.GetOrError(r.balAccountNo)
			if err != nil {
				return err
			}
		} else {
			glAcc := gtables.NewGLAccount(run)
			err := glAcc.GetOrError(r.balAccountNo)
			if err != nil {
				return err
			}
		}
	}
	if (r.bankPaymentType == gtables.GenJournalLine_Bank_payment_type.Blank || r.summarizePerVend) && r.nextDocNo == "" {
		return system.Error(run.Text("ENTER_STARTING_DOC_NO"))
	}
	if r.nextDocNo != "" && system.IncStr(r.nextDocNo.String()) == "" {
		return system.Error(run.Text("STARTING_DOC_NO_NUMBER"))
	}
	if !r.amountAvailable.IsZero() {
		r.usePriority = true
	}
	if r.usePaymentDisc && r.lastDueDateToPayReq.Before(system.WorkDate()) {
		if !system.Confirm(run.Text("PAYMENT_DATE_EARLIER", formatDate(system.WorkDate(), "")), false) {
			return system.Error(run.Text("INTERRUPTED"))
		}
	}
	r.vend2 = gtables.NewVendor(run)
	r.vend2.CopyFilters(vendor)
	r.originalAmtAvail = r.amountAvailable
	if r.amountAvailable.IsPositive() {
		vendor.SetCurrentKey("priority")
		vendor.SetRange("priority", 1, 2147483647)
		r.usePriority = true
	}
	r.nextEntryNo = 1
	return nil
}

func (r *suggestVendorPayments) vendorOnAfterGetRecord(vendor *gtables.Vendor, run *Run) error {
	if r.stopPayments {
		return Break()
	}
	err := r.getVendLedgEntries(run, vendor, true, false)
	if err == nil {
		err = r.getVendLedgEntries(run, vendor, false, false)
	}
	if err != nil {
		return err
	}
	r.checkAmounts(vendor, false)
	return nil
}

// vendorOnPostDataItem: the vendors without priority, then the payment discounts of the later
// entries; then the payments become journal lines (by priority first)
func (r *suggestVendorPayments) vendorOnPostDataItem(vendor *gtables.Vendor, run *Run) error {
	if r.usePriority && !r.stopPayments {
		vendor2 := gtables.NewVendor(run)
		vendor2.CopyFilters(r.vend2)
		vendor2.SetCurrentKey("priority")
		vendor2.SetRange("priority", 0)
		for found := vendor2.FindSetBuffered(); found && !r.stopPayments; found = vendor2.Next() {
			err := r.getVendLedgEntries(run, vendor2, true, false)
			if err == nil {
				err = r.getVendLedgEntries(run, vendor2, false, false)
			}
			if err != nil {
				return err
			}
			r.checkAmounts(vendor2, false)
		}
	}
	if r.usePaymentDisc && !r.stopPayments {
		vendor2 := gtables.NewVendor(run)
		vendor2.CopyFilters(r.vend2)
		for found := vendor2.FindSetBuffered(); found && !r.stopPayments; found = vendor2.Next() {
			err := r.getVendLedgEntries(run, vendor2, true, true)
			if err == nil {
				err = r.getVendLedgEntries(run, vendor2, false, true)
			}
			if err != nil {
				return err
			}
			r.checkAmounts(vendor2, true)
		}
	}

	genJnlLine := gtables.NewGenJournalLine(run)
	genJnlLine.LockTable()
	r.genJnlTemplate = gtables.NewGenJournalTemplate(run)
	err := r.genJnlTemplate.GetOrError(r.genJnlLine.Journal_template_name)
	if err != nil {
		return err
	}
	r.genJnlBatch = gtables.NewGenJournalBatch(run)
	err = r.genJnlBatch.GetOrError(map[string]interface{}{"journal_template_name": r.genJnlLine.Journal_template_name, "name": r.genJnlLine.Journal_batch_name})
	if err != nil {
		return err
	}
	genJnlLine.SetRange("journal_template_name", r.genJnlLine.Journal_template_name)
	genJnlLine.SetRange("journal_batch_name", r.genJnlLine.Journal_batch_name)
	if genJnlLine.FindLast() {
		r.firstLineNo = genJnlLine.Line_no
		r.lastLine = genJnlLine.Line_no
	}
	sort.SliceStable(r.payable, func(i, j int) bool { return payableLess(r.payable[i], r.payable[j]) })
	err = r.makeGenJnlLines(run, true)
	if err == nil {
		err = r.makeGenJnlLines(run, false)
	}
	if err != nil {
		return err
	}
	r.payable = nil
	run.Refresh()
	return nil
}

// payableLess is the order of the payments (the key Priority, Vendor No., Currency Code,
// Positive, Future, Entry No.)
func payableLess(a, b *payableVendLedgEntry) bool {
	if a.priority != b.priority {
		return a.priority < b.priority
	}
	if a.vendorNo != b.vendorNo {
		return a.vendorNo < b.vendorNo
	}
	if a.currencyCode != b.currencyCode {
		return a.currencyCode < b.currencyCode
	}
	if a.positive != b.positive {
		return !a.positive
	}
	if a.future != b.future {
		return !a.future
	}
	return a.entryNo < b.entryNo
}

// getVendLedgEntries: the vendor's open entries (not payments, not on hold) of one sign due up to
// the last payment date — or, Future, due later with a payment discount until then
func (r *suggestVendorPayments) getVendLedgEntries(run *Run, vendor *gtables.Vendor, positive, future bool) error {
	vendLedgEntry := gtables.NewVendorLedgerEntry(run)
	vendLedgEntry.SetRange("vendor_no", vendor.No)
	vendLedgEntry.SetRange("open", true)
	vendLedgEntry.SetRange("positive", positive)
	vendLedgEntry.SetFilter("document_type", "<>1") // not Payment
	if future {
		vendLedgEntry.SetFilter("due_date", r.lastDueDateToPayReq.AddDays(1).String()+"..")
		vendLedgEntry.SetRange("pmt_discount_date", r.postingDate, r.lastDueDateToPayReq)
		vendLedgEntry.SetFilter("pmt_disc_possible", "<0")
	} else {
		vendLedgEntry.SetRange("due_date", types.Date{}, r.lastDueDateToPayReq)
	}
	vendLedgEntry.SetRange("on_hold", "")
	for found := vendLedgEntry.FindSetBuffered(); found; found = vendLedgEntry.Next() {
		err := r.saveAmount(run, vendor, vendLedgEntry)
		if err != nil {
			return err
		}
	}
	return nil
}

// saveAmount: the entry's payment — its remaining amount, less the payment discount of an
// invoice paid in time — at the posting date's exchange rate
func (r *suggestVendorPayments) saveAmount(run *Run, vendor *gtables.Vendor, vendLedgEntry *gtables.VendorLedgerEntry) error {
	genJnlLine := gtables.NewGenJournalLine(run)
	genJnlLine.Journal_template_name = r.genJnlLine.Journal_template_name
	genJnlLine.Journal_batch_name = r.genJnlLine.Journal_batch_name
	err := genJnlLine.Validate("posting_date", r.postingDate)
	if err != nil {
		return err
	}
	genJnlLine.Document_type = gtables.GenJournalLine_Document_type.Payment
	genJnlLine.Account_type = gtables.GenJournalLine_Account_type.Vendor
	err = genJnlLine.Validate("account_no", vendLedgEntry.Vendor_no)
	if err == nil {
		err = genJnlLine.Validate("currency_code", vendLedgEntry.Currency_code)
	}
	if err != nil {
		return err
	}
	amount := vendLedgEntry.Remaining_amount.Neg()
	if vendLedgEntry.Document_type == gtables.VendorLedgerEntry_Document_type.Invoice && !r.postingDate.After(vendLedgEntry.Pmt_discount_date) {
		amount = vendLedgEntry.Remaining_amount.Sub(vendLedgEntry.Pmt_disc_possible).Neg()
	}
	err = genJnlLine.Validate("amount", amount)
	if err != nil {
		return err
	}
	priority := 0
	if r.usePriority {
		priority = vendor.Priority
	}
	r.payable = append(r.payable, &payableVendLedgEntry{
		priority:          priority,
		vendorNo:          vendLedgEntry.Vendor_no,
		entryNo:           r.nextEntryNo,
		vendorLedgEntryNo: vendLedgEntry.Entry_no,
		amount:            genJnlLine.Amount,
		amountLCY:         genJnlLine.Amount_lcy,
		positive:          genJnlLine.Amount.IsPositive(),
		future:            vendLedgEntry.Due_date.After(r.lastDueDateToPayReq),
		currencyCode:      vendLedgEntry.Currency_code,
	})
	r.nextEntryNo++
	return nil
}

// checkAmounts (CheckAmounts): the vendor's payments per currency are kept when they add up to a
// payment and, with an available amount, while it lasts; a credit balance in a currency drops
// its payments
func (r *suggestVendorPayments) checkAmounts(vendor *gtables.Vendor, future bool) {
	var entries []*payableVendLedgEntry
	for _, p := range r.payable {
		if !p.deleted && p.vendorNo == vendor.No && p.future == future {
			entries = append(entries, p)
		}
	}
	sort.SliceStable(entries, func(i, j int) bool { return payableLess(entries[i], entries[j]) })
	if len(entries) == 0 {
		return
	}
	currencyBalance := types.ZeroDecimal()
	prevCurrency := entries[0].currencyCode
	dropCurrency := func(currency types.Code) {
		for _, p := range entries {
			if p.currencyCode == currency {
				p.deleted = true
			}
		}
	}
	for _, p := range entries {
		if p.currencyCode != prevCurrency {
			if currencyBalance.IsNegative() {
				dropCurrency(prevCurrency)
			} else {
				r.amountAvailable = r.amountAvailable.Sub(currencyBalance)
			}
			currencyBalance = types.ZeroDecimal()
			prevCurrency = p.currencyCode
		}
		if r.originalAmtAvail.IsZero() || !r.amountAvailable.LessThan(currencyBalance.Add(p.amountLCY)) {
			currencyBalance = currencyBalance.Add(p.amountLCY)
		} else {
			p.deleted = true
		}
	}
	if currencyBalance.IsNegative() {
		dropCurrency(prevCurrency)
	} else {
		if r.originalAmtAvail.IsPositive() {
			r.amountAvailable = r.amountAvailable.Sub(currencyBalance)
		}
	}
	if r.originalAmtAvail.IsPositive() && !r.amountAvailable.IsPositive() {
		r.stopPayments = true
	}
}

// makeGenJnlLines (MakeGenJnlLines): the payments of the vendors with a priority (or, the second
// time, without) as journal lines — a line per entry applied to it, or one per vendor, currency
// and dimensions applied by Applies-to ID
func (r *suggestVendorPayments) makeGenJnlLines(run *Run, withPriority bool) error {
	for _, p := range r.payable {
		if p.deleted || (p.priority > 0) != withPriority {
			continue
		}
		vendLedgEntry := gtables.NewVendorLedgerEntry(run)
		err := vendLedgEntry.GetOrError(p.vendorLedgEntryNo)
		if err != nil {
			return err
		}
		genJnlLine := gtables.NewGenJournalLine(run)
		genJnlLine3 := gtables.NewGenJournalLine(run)
		foundLine := false
		if r.summarizePerVend || !r.docNoPerLine {
			genJnlLine3.SetRange("journal_template_name", r.genJnlLine.Journal_template_name)
			genJnlLine3.SetRange("journal_batch_name", r.genJnlLine.Journal_batch_name)
			genJnlLine3.SetRange("line_no", r.firstLineNo+1, r.lastLine)
			genJnlLine3.SetRange("account_type", gtables.GenJournalLine_Account_type.Vendor)
			genJnlLine3.SetRange("account_no", vendLedgEntry.Vendor_no)
			genJnlLine3.SetRange("currency_code", vendLedgEntry.Currency_code)
			genJnlLine3.SetRange("dimension_1_code", vendLedgEntry.Dimension_1_code)
			genJnlLine3.SetRange("dimension_2_code", vendLedgEntry.Dimension_2_code)
			foundLine = genJnlLine3.FindFirst()
		}

		if r.summarizePerVend && foundLine {
			err = genJnlLine.GetOrError(map[string]interface{}{"journal_template_name": genJnlLine3.Journal_template_name,
				"journal_batch_name": genJnlLine3.Journal_batch_name, "line_no": genJnlLine3.Line_no})
			if err == nil {
				err = genJnlLine.Validate("amount", genJnlLine.Amount.Add(p.amount))
			}
			if err != nil {
				return err
			}
		} else {
			r.lastLine += 10000
			genJnlLine.Journal_template_name = r.genJnlLine.Journal_template_name
			genJnlLine.Journal_batch_name = r.genJnlLine.Journal_batch_name
			genJnlLine.Line_no = r.lastLine
			err = genJnlLine.Validate("posting_date", r.postingDate)
			if err != nil {
				return err
			}
			genJnlLine.Document_type = gtables.GenJournalLine_Document_type.Payment
			if !r.docNoPerLine && foundLine {
				genJnlLine.Document_no = genJnlLine3.Document_no
			} else {
				genJnlLine.Document_no = r.nextDocNo
				r.nextDocNo = types.Code(system.IncStr(r.nextDocNo.String()))
			}
			genJnlLine.Account_type = gtables.GenJournalLine_Account_type.Vendor
			err = genJnlLine.Validate("account_no", vendLedgEntry.Vendor_no)
			if err != nil {
				return err
			}
			genJnlLine.Bal_account_type = r.balAccountType
			err = genJnlLine.Validate("bal_account_no", r.balAccountNo)
			if err == nil {
				err = genJnlLine.Validate("currency_code", vendLedgEntry.Currency_code)
			}
			if err != nil {
				return err
			}
			genJnlLine.Bank_payment_type = r.bankPaymentType
			if r.summarizePerVend {
				genJnlLine.Applies_to_id = genJnlLine.Document_no
				genJnlLine.Description = types.Text(run.Text("PAYMENT_TO_VENDOR", vendLedgEntry.Vendor_no))
			} else {
				genJnlLine.Description = types.Text(run.Text("PAYMENT_OF",
					optionCaption(gtables.VendorLedgerEntryTableName, "document_type", vendLedgEntry.Document_type.String(), ""),
					vendLedgEntry.Document_no))
			}
			genJnlLine.Dimension_1_code = vendLedgEntry.Dimension_1_code
			genJnlLine.Dimension_2_code = vendLedgEntry.Dimension_2_code
			genJnlLine.Source_code = r.genJnlTemplate.Source_code
			genJnlLine.Reason_code = r.genJnlBatch.Reason_code
			err = genJnlLine.Validate("amount", p.amount)
			if err != nil {
				return err
			}
		}

		if r.summarizePerVend {
			err = vendLedgEntry.TestField("applies_to_id", types.Code(""))
			if err != nil {
				return err
			}
			vendLedgEntry.Applies_to_id = genJnlLine.Applies_to_id
			err = vendLedgEntry.ModifyOrError(false)
			if err != nil {
				return err
			}
		} else {
			genJnlLine4 := gtables.NewGenJournalLine(run)
			genJnlLine4.SetRange("account_type", gtables.GenJournalLine_Account_type.Vendor)
			genJnlLine4.SetRange("account_no", vendLedgEntry.Vendor_no)
			genJnlLine4.SetRange("applies_to_doc_type", int(vendLedgEntry.Document_type))
			genJnlLine4.SetRange("applies_to_doc_no", vendLedgEntry.Document_no)
			if genJnlLine4.FindFirst() {
				return system.Error(run.Text("ALREADY_APPLIED",
					optionCaption(gtables.VendorLedgerEntryTableName, "document_type", vendLedgEntry.Document_type.String(), ""),
					vendLedgEntry.Document_no, vendLedgEntry.Vendor_no))
			}
			genJnlLine.Applies_to_doc_type = gtables.GenJournalLineApplies_to_doc_type(vendLedgEntry.Document_type)
			genJnlLine.Applies_to_doc_no = vendLedgEntry.Document_no
		}

		if r.summarizePerVend && foundLine {
			err = genJnlLine.ModifyOrError(false)
		} else {
			err = genJnlLine.InsertOrError(false)
		}
		if err != nil {
			return err
		}
	}
	return nil
}
