package reports

import (
	fcodeunits "github.com/hansjlachmann/openerp/backend/foundation/codeunits"
	"github.com/hansjlachmann/openerp/backend/foundation/system"
	ftables "github.com/hansjlachmann/openerp/backend/foundation/tables"
	"github.com/hansjlachmann/openerp/backend/foundation/types"
	gtables "github.com/hansjlachmann/openerp/backend/generated/tables"
)

// Report 398 Date Compress Vendor Ledger: the closed vendor ledger entries of closed
// fiscal years become one entry per vendor, posting group, currency, document type, period
// and sign — and per the retained Document No., Buy-from Vendor No., Purchaser, Dimension 1
// and Dimension 2 Code. The new entries take G/L
// entry numbers (a G/L register of the run).
const DateCompressVendorLedgerID = 398

func init() {
	Register(DateCompressVendorLedgerID, "date-compress-vendor-ledger", func() Report { return &dateCompressVend{} })
}

type dateCompressVend struct {
	dateCompression
	glNumbered
	sourceCodeSetup  *gtables.SourceCodeSetup
	newVendLedgEntry *gtables.VendorLedgerEntry
	window           *system.Window
}

func (r *dateCompressVend) Definition() *Definition {
	r.tableID = gtables.VendorLedgerEntryTableID
	r.fields = []retainField{{name: "document_no"}, {name: "buy_from_vendor_no"}, {name: "purchaser_code"}, {name: "dimension_1_code"}, {name: "dimension_2_code"}}
	return &Definition{
		OptionsCaption: "OPTIONS",
		Options:        dateComprOptions(true, r.fields),
		Transaction:    true,
		OnInitReport:   r.onInitReport,
		OnPreReport: func(run *Run) error {
			r.read(run)
			r.filter = requestFilter(run, "Vend_Ledger_Entry")
			return nil
		},
		DataItems: []*DataItem{{
			Name: "Vend_Ledger_Entry", Table: "Vendor_ledger_entry", Key: []string{"vendor_no", "posting_date"},
			TableView:           []ftables.FieldFilter{{Field: "open", Expression: "false"}},
			RequestFilterFields: []string{"vendor_no", "vendor_posting_group", "currency_code"},
			OnPreDataItem:       Trigger(r.onPreDataItem),
			OnAfterGetRecord:    Trigger(r.onAfterGetRecord),
			OnPostDataItem:      Trigger(r.onPostDataItem),
		}},
	}
}

func (r *dateCompressVend) onPreDataItem(vendLedgEntry *gtables.VendorLedgerEntry, run *Run) error {
	err := r.confirm(run)
	if err != nil {
		return err
	}
	r.window = system.OpenWindow(run.Text("WINDOW"))

	r.sourceCodeSetup = gtables.GetSourceCodeSetup(run)
	err = r.sourceCodeSetup.TestField("compress_vend_ledger")
	if err != nil {
		return err
	}

	r.start(run, r.sourceCodeSetup.Compress_vend_ledger, r.description, false)
	r.newVendLedgEntry = gtables.NewVendorLedgerEntry(run)
	r.newVendLedgEntry.LockTable()
	r.dateComprReg.LockTable()
	vendLedgEntry.SetRange("entry_no", 0, r.lastEntryNo)
	vendLedgEntry.SetRange("posting_date", r.entrdDateComprJnl.Starting_date, r.entrdDateComprJnl.Ending_date)

	r.initRegisters(run)
	return nil
}

func (r *dateCompressVend) onAfterGetRecord(vendLedgEntry *gtables.VendorLedgerEntry, run *Run) error {
	// compressed with an entry before it
	stillThere := gtables.NewVendorLedgerEntry(run)
	if !stillThere.Get(vendLedgEntry.Entry_no) {
		return Skip()
	}

	vendLedgEntry2 := gtables.NewVendorLedgerEntry(run)
	vendLedgEntry2.Copy(vendLedgEntry)
	vendLedgEntry2.SetCurrentKey("vendor_no", "posting_date")
	vendLedgEntry2.CopyFilters(vendLedgEntry)
	vendLedgEntry2.SetRange("vendor_no", vendLedgEntry2.Vendor_no)
	dateFilter, periodStart, err := r.dateFilter(vendLedgEntry2.Posting_date, true)
	if err != nil {
		return err
	}
	vendLedgEntry2.SetFilter("posting_date", dateFilter)
	vendLedgEntry2.SetRange("vendor_posting_group", vendLedgEntry2.Vendor_posting_group)
	vendLedgEntry2.SetRange("currency_code", vendLedgEntry2.Currency_code)
	vendLedgEntry2.SetRange("document_type", vendLedgEntry2.Document_type)

	r.lastEntryNo++

	newVendLedgEntry := r.newVendLedgEntry
	newVendLedgEntry.Init()
	newVendLedgEntry.Entry_no = r.lastEntryNo
	newVendLedgEntry.Vendor_no = vendLedgEntry2.Vendor_no
	newVendLedgEntry.Posting_date = periodStart
	newVendLedgEntry.Description = r.description
	newVendLedgEntry.Vendor_posting_group = vendLedgEntry2.Vendor_posting_group
	newVendLedgEntry.Currency_code = vendLedgEntry2.Currency_code
	newVendLedgEntry.Document_type = vendLedgEntry2.Document_type
	newVendLedgEntry.Source_code = r.sourceCodeSetup.Compress_vend_ledger
	newVendLedgEntry.User_id = types.Code(fcodeunits.CurrentUserID())
	newVendLedgEntry.Transaction_no = r.nextTransactionNo
	r.window.Update(1, newVendLedgEntry.Vendor_no)
	r.window.Update(2, newVendLedgEntry.Posting_date)
	r.newRecord()
	r.window.Update(3, r.dateComprReg.No_of_new_records)

	if r.retainNo("document_no") {
		vendLedgEntry2.SetRange("document_no", vendLedgEntry2.Document_no)
		newVendLedgEntry.Document_no = vendLedgEntry2.Document_no
	}
	if r.retainNo("buy_from_vendor_no") {
		vendLedgEntry2.SetRange("buy_from_vendor_no", vendLedgEntry2.Buy_from_vendor_no)
		newVendLedgEntry.Buy_from_vendor_no = vendLedgEntry2.Buy_from_vendor_no
	}
	if r.retainNo("purchaser_code") {
		vendLedgEntry2.SetRange("purchaser_code", vendLedgEntry2.Purchaser_code)
		newVendLedgEntry.Purchaser_code = vendLedgEntry2.Purchaser_code
	}
	if r.retainNo("dimension_1_code") {
		vendLedgEntry2.SetRange("dimension_1_code", vendLedgEntry2.Dimension_1_code)
		newVendLedgEntry.Dimension_1_code = vendLedgEntry2.Dimension_1_code
	}
	if r.retainNo("dimension_2_code") {
		vendLedgEntry2.SetRange("dimension_2_code", vendLedgEntry2.Dimension_2_code)
		newVendLedgEntry.Dimension_2_code = vendLedgEntry2.Dimension_2_code
	}
	if !vendLedgEntry2.Amount.IsNegative() {
		vendLedgEntry2.SetFilter("amount", ">=0")
	} else {
		vendLedgEntry2.SetFilter("amount", "<0")
	}
	for found := vendLedgEntry2.FindFirst(); found; found = vendLedgEntry2.FindFirst() {
		newVendLedgEntry.Amount = newVendLedgEntry.Amount.Add(vendLedgEntry2.Amount)
		newVendLedgEntry.Remaining_amount = newVendLedgEntry.Remaining_amount.Add(vendLedgEntry2.Remaining_amount)
		newVendLedgEntry.Original_amount_lcy = newVendLedgEntry.Original_amount_lcy.Add(vendLedgEntry2.Original_amount_lcy)
		newVendLedgEntry.Remaining_amt_lcy = newVendLedgEntry.Remaining_amt_lcy.Add(vendLedgEntry2.Remaining_amt_lcy)
		newVendLedgEntry.Amount_lcy = newVendLedgEntry.Amount_lcy.Add(vendLedgEntry2.Amount_lcy)
		newVendLedgEntry.Purchase_lcy = newVendLedgEntry.Purchase_lcy.Add(vendLedgEntry2.Purchase_lcy)
		newVendLedgEntry.Inv_discount_lcy = newVendLedgEntry.Inv_discount_lcy.Add(vendLedgEntry2.Inv_discount_lcy)
		newVendLedgEntry.Pmt_disc_rcd_lcy = newVendLedgEntry.Pmt_disc_rcd_lcy.Add(vendLedgEntry2.Pmt_disc_rcd_lcy)
		newVendLedgEntry.Pmt_disc_possible = newVendLedgEntry.Pmt_disc_possible.Add(vendLedgEntry2.Pmt_disc_possible)
		newVendLedgEntry.Closed_by_amount_lcy = newVendLedgEntry.Closed_by_amount_lcy.Add(vendLedgEntry2.Closed_by_amount_lcy)
		newVendLedgEntry.Debit_amount = newVendLedgEntry.Debit_amount.Add(vendLedgEntry2.Debit_amount)
		newVendLedgEntry.Credit_amount = newVendLedgEntry.Credit_amount.Add(vendLedgEntry2.Credit_amount)
		newVendLedgEntry.Debit_amount_lcy = newVendLedgEntry.Debit_amount_lcy.Add(vendLedgEntry2.Debit_amount_lcy)
		newVendLedgEntry.Credit_amount_lcy = newVendLedgEntry.Credit_amount_lcy.Add(vendLedgEntry2.Credit_amount_lcy)
		err = vendLedgEntry2.DeleteOrError(false)
		if err != nil {
			return err
		}
		r.deleted()
		r.window.Update(4, r.dateComprReg.No_records_deleted)
	}
	err = newVendLedgEntry.InsertOrError(false)
	if err != nil {
		return err
	}

	if r.due() {
		return r.insertRegisters(run)
	}
	return nil
}

func (r *dateCompressVend) onPostDataItem(vendLedgEntry *gtables.VendorLedgerEntry, run *Run) error {
	r.window.Close()
	if r.dateComprReg.No_records_deleted > r.noOfDeleted {
		return r.insertRegisters(run)
	}
	return nil
}

// initRegisters (InitRegisters): the G/L register and the Date Compr. Register row
func (r *dateCompressVend) initRegisters(run *Run) {
	r.initGLReg()
	vendLedgEntry := gtables.NewVendorLedgerEntry(run)
	language := fcodeunits.CurrentLanguage()
	r.initRegister(r.glReg.No, r.sourceCodeSetup.Compress_vend_ledger, func(field string) string { return vendLedgEntry.FieldCaption(field, language) })
}

// insertRegisters (InsertRegisters): a G/L entry of 0 and the registers
func (r *dateCompressVend) insertRegisters(run *Run) error {
	err := r.insertGLEntry(run)
	if err != nil {
		return err
	}
	err = r.saveGLReg()
	if err != nil {
		return err
	}
	err = r.saveRegister(r.glRegExists)
	if err != nil {
		return err
	}
	r.glRegExists = true

	glEntry := gtables.NewGLEntry(run)
	newVendLedgEntry := gtables.NewVendorLedgerEntry(run)
	glEntry.FindLast()
	newVendLedgEntry.FindLast()
	if r.lastEntryNo != glEntry.Entry_no || r.lastEntryNo != newVendLedgEntry.Entry_no+1 {
		r.lastEntryNo = glEntry.Entry_no
		r.nextTransactionNo = glEntry.Transaction_no + 1
		r.initRegisters(run)
	}
	return nil
}
