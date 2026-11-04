package reports

import (
	fcodeunits "github.com/hansjlachmann/goeasyerp/backend/foundation/codeunits"
	"github.com/hansjlachmann/goeasyerp/backend/foundation/system"
	ftables "github.com/hansjlachmann/goeasyerp/backend/foundation/tables"
	"github.com/hansjlachmann/goeasyerp/backend/foundation/types"
	gtables "github.com/hansjlachmann/goeasyerp/backend/generated/tables"
)

// Report 198 Date Compress Customer Ledger: the closed customer ledger entries of closed
// fiscal years become one entry per customer, posting group, currency, document type, period
// and sign — and per the retained Document No., Sell-to Customer No., Salesperson, Dimension 1
// and Dimension 2 Code; their reminder/finance charge entries are deleted. The new entries take G/L
// entry numbers (a G/L register of the run).
const DateCompressCustomerLedgerID = 198

func init() {
	Register(DateCompressCustomerLedgerID, "date-compress-customer-ledger", func() Report { return &dateCompressCust{} })
}

type dateCompressCust struct {
	dateCompression
	glNumbered
	sourceCodeSetup  *gtables.SourceCodeSetup
	newCustLedgEntry *gtables.CustomerLedgerEntry
	window           *system.Window
}

func (r *dateCompressCust) Definition() *Definition {
	r.tableID = gtables.CustomerLedgerEntryTableID
	r.fields = []retainField{{name: "document_no"}, {name: "sell_to_customer_no"}, {name: "salesperson_code"}, {name: "dimension_1_code"}, {name: "dimension_2_code"}}
	return &Definition{
		OptionsCaption: "OPTIONS",
		Options:        dateComprOptions(true, r.fields),
		Transaction:    true,
		OnInitReport:   r.onInitReport,
		OnPreReport: func(run *Run) error {
			r.read(run)
			r.filter = requestFilter(run, "Cust_Ledger_Entry")
			return nil
		},
		DataItems: []*DataItem{{
			Name: "Cust_Ledger_Entry", Table: "Customer_ledger_entry", Key: []string{"customer_no", "posting_date"},
			TableView:           []ftables.FieldFilter{{Field: "open", Expression: "false"}},
			RequestFilterFields: []string{"customer_no", "customer_posting_group", "currency_code"},
			OnPreDataItem:       Trigger(r.onPreDataItem),
			OnAfterGetRecord:    Trigger(r.onAfterGetRecord),
			OnPostDataItem:      Trigger(r.onPostDataItem),
		}},
	}
}

func (r *dateCompressCust) onPreDataItem(custLedgEntry *gtables.CustomerLedgerEntry, run *Run) error {
	err := r.confirm(run)
	if err != nil {
		return err
	}
	r.window = system.OpenWindow(run.Text("WINDOW"))

	r.sourceCodeSetup = gtables.GetSourceCodeSetup(run)
	err = r.sourceCodeSetup.TestField("compress_cust_ledger")
	if err != nil {
		return err
	}

	r.start(run, r.sourceCodeSetup.Compress_cust_ledger, r.description, false)
	r.newCustLedgEntry = gtables.NewCustomerLedgerEntry(run)
	r.newCustLedgEntry.LockTable()
	r.dateComprReg.LockTable()
	custLedgEntry.SetRange("entry_no", 0, r.lastEntryNo)
	custLedgEntry.SetRange("posting_date", r.entrdDateComprJnl.Starting_date, r.entrdDateComprJnl.Ending_date)

	r.initRegisters(run)
	return nil
}

func (r *dateCompressCust) onAfterGetRecord(custLedgEntry *gtables.CustomerLedgerEntry, run *Run) error {
	// compressed with an entry before it
	stillThere := gtables.NewCustomerLedgerEntry(run)
	if !stillThere.Get(custLedgEntry.Entry_no) {
		return Skip()
	}

	reminderEntry := gtables.NewReminderFinChargeEntry(run)
	reminderEntry.SetCurrentKey("customer_entry_no")
	custLedgEntry2 := gtables.NewCustomerLedgerEntry(run)
	custLedgEntry2.Copy(custLedgEntry)
	custLedgEntry2.SetCurrentKey("customer_no", "posting_date")
	custLedgEntry2.CopyFilters(custLedgEntry)
	custLedgEntry2.SetRange("customer_no", custLedgEntry2.Customer_no)
	dateFilter, periodStart, err := r.dateFilter(custLedgEntry2.Posting_date, true)
	if err != nil {
		return err
	}
	custLedgEntry2.SetFilter("posting_date", dateFilter)
	custLedgEntry2.SetRange("customer_posting_group", custLedgEntry2.Customer_posting_group)
	custLedgEntry2.SetRange("currency_code", custLedgEntry2.Currency_code)
	custLedgEntry2.SetRange("document_type", custLedgEntry2.Document_type)

	r.lastEntryNo++

	newCustLedgEntry := r.newCustLedgEntry
	newCustLedgEntry.Init()
	newCustLedgEntry.Entry_no = r.lastEntryNo
	newCustLedgEntry.Customer_no = custLedgEntry2.Customer_no
	newCustLedgEntry.Posting_date = periodStart
	newCustLedgEntry.Description = r.description
	newCustLedgEntry.Customer_posting_group = custLedgEntry2.Customer_posting_group
	newCustLedgEntry.Currency_code = custLedgEntry2.Currency_code
	newCustLedgEntry.Document_type = custLedgEntry2.Document_type
	newCustLedgEntry.Source_code = r.sourceCodeSetup.Compress_cust_ledger
	newCustLedgEntry.User_id = types.Code(fcodeunits.CurrentUserID())
	newCustLedgEntry.Transaction_no = r.nextTransactionNo
	r.window.Update(1, newCustLedgEntry.Customer_no)
	r.window.Update(2, newCustLedgEntry.Posting_date)
	r.newRecord()
	r.window.Update(3, r.dateComprReg.No_of_new_records)

	if r.retainNo("document_no") {
		custLedgEntry2.SetRange("document_no", custLedgEntry2.Document_no)
		newCustLedgEntry.Document_no = custLedgEntry2.Document_no
	}
	if r.retainNo("sell_to_customer_no") {
		custLedgEntry2.SetRange("sell_to_customer_no", custLedgEntry2.Sell_to_customer_no)
		newCustLedgEntry.Sell_to_customer_no = custLedgEntry2.Sell_to_customer_no
	}
	if r.retainNo("salesperson_code") {
		custLedgEntry2.SetRange("salesperson_code", custLedgEntry2.Salesperson_code)
		newCustLedgEntry.Salesperson_code = custLedgEntry2.Salesperson_code
	}
	if r.retainNo("dimension_1_code") {
		custLedgEntry2.SetRange("dimension_1_code", custLedgEntry2.Dimension_1_code)
		newCustLedgEntry.Dimension_1_code = custLedgEntry2.Dimension_1_code
	}
	if r.retainNo("dimension_2_code") {
		custLedgEntry2.SetRange("dimension_2_code", custLedgEntry2.Dimension_2_code)
		newCustLedgEntry.Dimension_2_code = custLedgEntry2.Dimension_2_code
	}
	if !custLedgEntry2.Amount.IsNegative() {
		custLedgEntry2.SetFilter("amount", ">=0")
	} else {
		custLedgEntry2.SetFilter("amount", "<0")
	}
	for found := custLedgEntry2.FindFirst(); found; found = custLedgEntry2.FindFirst() {
		newCustLedgEntry.Amount = newCustLedgEntry.Amount.Add(custLedgEntry2.Amount)
		newCustLedgEntry.Remaining_amount = newCustLedgEntry.Remaining_amount.Add(custLedgEntry2.Remaining_amount)
		newCustLedgEntry.Original_amount_lcy = newCustLedgEntry.Original_amount_lcy.Add(custLedgEntry2.Original_amount_lcy)
		newCustLedgEntry.Remaining_amt_lcy = newCustLedgEntry.Remaining_amt_lcy.Add(custLedgEntry2.Remaining_amt_lcy)
		newCustLedgEntry.Amount_lcy = newCustLedgEntry.Amount_lcy.Add(custLedgEntry2.Amount_lcy)
		newCustLedgEntry.Sales_lcy = newCustLedgEntry.Sales_lcy.Add(custLedgEntry2.Sales_lcy)
		newCustLedgEntry.Profit_lcy = newCustLedgEntry.Profit_lcy.Add(custLedgEntry2.Profit_lcy)
		newCustLedgEntry.Inv_discount_lcy = newCustLedgEntry.Inv_discount_lcy.Add(custLedgEntry2.Inv_discount_lcy)
		newCustLedgEntry.Pmt_disc_given_lcy = newCustLedgEntry.Pmt_disc_given_lcy.Add(custLedgEntry2.Pmt_disc_given_lcy)
		newCustLedgEntry.Pmt_disc_possible = newCustLedgEntry.Pmt_disc_possible.Add(custLedgEntry2.Pmt_disc_possible)
		newCustLedgEntry.Closed_by_amount_lcy = newCustLedgEntry.Closed_by_amount_lcy.Add(custLedgEntry2.Closed_by_amount_lcy)
		newCustLedgEntry.Debit_amount = newCustLedgEntry.Debit_amount.Add(custLedgEntry2.Debit_amount)
		newCustLedgEntry.Credit_amount = newCustLedgEntry.Credit_amount.Add(custLedgEntry2.Credit_amount)
		newCustLedgEntry.Debit_amount_lcy = newCustLedgEntry.Debit_amount_lcy.Add(custLedgEntry2.Debit_amount_lcy)
		newCustLedgEntry.Credit_amount_lcy = newCustLedgEntry.Credit_amount_lcy.Add(custLedgEntry2.Credit_amount_lcy)
		reminderEntry.SetRange("customer_entry_no", custLedgEntry2.Entry_no)
		reminderEntry.DeleteAll()
		err = custLedgEntry2.DeleteOrError(false)
		if err != nil {
			return err
		}
		r.deleted()
		r.window.Update(4, r.dateComprReg.No_records_deleted)
	}
	err = newCustLedgEntry.InsertOrError(false)
	if err != nil {
		return err
	}

	if r.due() {
		return r.insertRegisters(run)
	}
	return nil
}

func (r *dateCompressCust) onPostDataItem(custLedgEntry *gtables.CustomerLedgerEntry, run *Run) error {
	r.window.Close()
	if r.dateComprReg.No_records_deleted > r.noOfDeleted {
		return r.insertRegisters(run)
	}
	return nil
}

// initRegisters (InitRegisters): the G/L register and the Date Compr. Register row
func (r *dateCompressCust) initRegisters(run *Run) {
	r.initGLReg()
	custLedgEntry := gtables.NewCustomerLedgerEntry(run)
	language := fcodeunits.CurrentLanguage()
	r.initRegister(r.glReg.No, r.sourceCodeSetup.Compress_cust_ledger, func(field string) string { return custLedgEntry.FieldCaption(field, language) })
}

// insertRegisters (InsertRegisters): a G/L entry of 0 and the registers
func (r *dateCompressCust) insertRegisters(run *Run) error {
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
	newCustLedgEntry := gtables.NewCustomerLedgerEntry(run)
	glEntry.FindLast()
	newCustLedgEntry.FindLast()
	if r.lastEntryNo != glEntry.Entry_no || r.lastEntryNo != newCustLedgEntry.Entry_no+1 {
		r.lastEntryNo = glEntry.Entry_no
		r.nextTransactionNo = glEntry.Transaction_no + 1
		r.initRegisters(run)
	}
	return nil
}
