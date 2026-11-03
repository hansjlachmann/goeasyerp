package reports

import (
	fcodeunits "github.com/hansjlachmann/openerp/backend/foundation/codeunits"
	"github.com/hansjlachmann/openerp/backend/foundation/system"
	"github.com/hansjlachmann/openerp/backend/foundation/types"
	gtables "github.com/hansjlachmann/openerp/backend/generated/tables"
)

// Report 95 Date Compress Tax Entries: the tax entries of closed fiscal years become one entry
// per type, closed, tax posting groups, jurisdiction, use tax, document type, period and sign of
// the base — and per the retained Document No., Sell-to/Buy-from No., EU 3-Party Trade, Country
// Code and Internal Ref. No.; the G/L register of the run holds the tax entry range.
const DateCompressTaxEntriesID = 95

func init() {
	Register(DateCompressTaxEntriesID, "date-compress-tax-entries", func() Report { return &dateCompressTax{} })
}

type dateCompressTax struct {
	dateCompression
	glNumbered
	sourceCodeSetup *gtables.SourceCodeSetup
	newTaxEntry     *gtables.TaxEntry
	lastTaxEntryNo  int
	window          *system.Window
}

func (r *dateCompressTax) Definition() *Definition {
	r.tableID = gtables.TaxEntryTableID
	r.fields = []retainField{{name: "document_no"}, {name: "sell_to_buy_from_no"}, {name: "eu_3_party_trade"}, {name: "country_code"}, {name: "internal_ref_no"}}
	return &Definition{
		OptionsCaption: "OPTIONS",
		Options:        dateComprOptions(false, r.fields),
		Transaction:    true,
		OnInitReport:   r.onInitReport,
		OnPreReport: func(run *Run) error {
			r.read(run)
			r.filter = requestFilter(run, "Tax_Entry")
			return nil
		},
		DataItems: []*DataItem{{
			Name: "Tax_Entry", Table: gtables.TaxEntryTableName,
			Key:                 []string{"type", "closed", "tax_bus_posting_group", "tax_prod_posting_group", "tax_jurisdiction_code", "use_tax", "posting_date"},
			RequestFilterFields: []string{"type", "tax_bus_posting_group", "tax_prod_posting_group", "tax_jurisdiction_code", "use_tax", "closed"},
			OnPreDataItem:       Trigger(r.onPreDataItem),
			OnAfterGetRecord:    Trigger(r.onAfterGetRecord),
			OnPostDataItem:      Trigger(r.onPostDataItem),
		}},
	}
}

func (r *dateCompressTax) onPreDataItem(taxEntry *gtables.TaxEntry, run *Run) error {
	err := r.confirm(run)
	if err != nil {
		return err
	}
	r.window = system.OpenWindow(run.Text("WINDOW"))

	r.sourceCodeSetup = gtables.GetSourceCodeSetup(run)
	err = r.sourceCodeSetup.TestField("compress_tax_entries")
	if err != nil {
		return err
	}

	r.start(run, r.sourceCodeSetup.Compress_tax_entries, "", true)
	r.newTaxEntry = gtables.NewTaxEntry(run)
	r.newTaxEntry.LockTable()
	r.dateComprReg.LockTable()
	r.lastTaxEntryNo = 0
	if r.newTaxEntry.FindLast() {
		r.lastTaxEntryNo = r.newTaxEntry.Entry_no
	}
	taxEntry.SetRange("entry_no", 0, r.lastTaxEntryNo)
	taxEntry.SetRange("posting_date", r.entrdDateComprJnl.Starting_date, r.entrdDateComprJnl.Ending_date)

	r.initRegisters(run)
	return nil
}

func (r *dateCompressTax) onAfterGetRecord(taxEntry *gtables.TaxEntry, run *Run) error {
	// compressed with an entry before it
	stillThere := gtables.NewTaxEntry(run)
	if !stillThere.Get(taxEntry.Entry_no) {
		return Skip()
	}

	taxEntry2 := gtables.NewTaxEntry(run)
	taxEntry2.Copy(taxEntry)
	taxEntry2.SetCurrentKey("type", "closed", "tax_bus_posting_group", "tax_prod_posting_group", "tax_jurisdiction_code", "use_tax", "posting_date")
	taxEntry2.CopyFilters(taxEntry)
	taxEntry2.SetRange("type", taxEntry2.Type)
	taxEntry2.SetRange("closed", taxEntry2.Closed)
	taxEntry2.SetRange("tax_bus_posting_group", taxEntry2.Tax_bus_posting_group)
	taxEntry2.SetRange("tax_prod_posting_group", taxEntry2.Tax_prod_posting_group)
	taxEntry2.SetRange("tax_jurisdiction_code", taxEntry2.Tax_jurisdiction_code)
	taxEntry2.SetRange("use_tax", taxEntry2.Use_tax)
	dateFilter, periodStart, err := r.dateFilter(taxEntry2.Posting_date, true)
	if err != nil {
		return err
	}
	taxEntry2.SetFilter("posting_date", dateFilter)
	taxEntry2.SetRange("document_type", taxEntry2.Document_type)

	r.lastTaxEntryNo++

	newTaxEntry := r.newTaxEntry
	newTaxEntry.Init()
	newTaxEntry.Entry_no = r.lastTaxEntryNo
	newTaxEntry.Type = taxEntry2.Type
	newTaxEntry.Closed = taxEntry2.Closed
	newTaxEntry.Tax_bus_posting_group = taxEntry2.Tax_bus_posting_group
	newTaxEntry.Tax_prod_posting_group = taxEntry2.Tax_prod_posting_group
	newTaxEntry.Tax_jurisdiction_code = taxEntry2.Tax_jurisdiction_code
	newTaxEntry.Use_tax = taxEntry2.Use_tax
	newTaxEntry.Posting_date = periodStart
	newTaxEntry.Document_type = taxEntry2.Document_type
	newTaxEntry.Source_code = r.sourceCodeSetup.Compress_tax_entries
	newTaxEntry.User_id = types.Code(fcodeunits.CurrentUserID())
	newTaxEntry.Transaction_no = r.nextTransactionNo
	r.window.Update(1, newTaxEntry.Type)
	r.window.Update(2, newTaxEntry.Tax_bus_posting_group)
	r.window.Update(3, newTaxEntry.Tax_prod_posting_group)
	r.window.Update(4, newTaxEntry.Tax_jurisdiction_code)
	r.window.Update(5, newTaxEntry.Use_tax)
	r.window.Update(6, newTaxEntry.Posting_date)
	r.newRecord()
	r.window.Update(7, r.dateComprReg.No_of_new_records)

	if r.retainNo("document_no") {
		taxEntry2.SetRange("document_no", taxEntry2.Document_no)
		newTaxEntry.Document_no = taxEntry2.Document_no
	}
	if r.retainNo("sell_to_buy_from_no") {
		taxEntry2.SetRange("sell_to_buy_from_no", taxEntry2.Sell_to_buy_from_no)
		newTaxEntry.Sell_to_buy_from_no = taxEntry2.Sell_to_buy_from_no
	}
	if r.retainNo("eu_3_party_trade") {
		taxEntry2.SetRange("eu_3_party_trade", taxEntry2.Eu_3_party_trade)
		newTaxEntry.Eu_3_party_trade = taxEntry2.Eu_3_party_trade
	}
	if r.retainNo("country_code") {
		taxEntry2.SetRange("country_code", taxEntry2.Country_code)
		newTaxEntry.Country_code = taxEntry2.Country_code
	}
	if r.retainNo("internal_ref_no") {
		taxEntry2.SetRange("internal_ref_no", taxEntry2.Internal_ref_no)
		newTaxEntry.Internal_ref_no = taxEntry2.Internal_ref_no
	}
	if !taxEntry2.Base.IsNegative() {
		taxEntry2.SetFilter("base", ">=0")
	} else {
		taxEntry2.SetFilter("base", "<0")
	}
	for found := taxEntry2.FindFirst(); found; found = taxEntry2.FindFirst() {
		newTaxEntry.Base = newTaxEntry.Base.Add(taxEntry2.Base)
		newTaxEntry.Amount = newTaxEntry.Amount.Add(taxEntry2.Amount)
		newTaxEntry.Unrealized_amount = newTaxEntry.Unrealized_amount.Add(taxEntry2.Unrealized_amount)
		newTaxEntry.Unrealized_base = newTaxEntry.Unrealized_base.Add(taxEntry2.Unrealized_base)
		newTaxEntry.Additional_currency_base = newTaxEntry.Additional_currency_base.Add(taxEntry2.Additional_currency_base)
		newTaxEntry.Additional_currency_amount = newTaxEntry.Additional_currency_amount.Add(taxEntry2.Additional_currency_amount)
		newTaxEntry.Add_currency_unrealized_amt = newTaxEntry.Add_currency_unrealized_amt.Add(taxEntry2.Add_currency_unrealized_amt)
		newTaxEntry.Add_currency_unrealized_base = newTaxEntry.Add_currency_unrealized_base.Add(taxEntry2.Add_currency_unrealized_base)
		newTaxEntry.Remaining_unrealized_amount = newTaxEntry.Remaining_unrealized_amount.Add(taxEntry2.Remaining_unrealized_amount)
		newTaxEntry.Remaining_unrealized_base = newTaxEntry.Remaining_unrealized_base.Add(taxEntry2.Remaining_unrealized_base)
		err = taxEntry2.DeleteOrError(false)
		if err != nil {
			return err
		}
		r.deleted()
		r.window.Update(8, r.dateComprReg.No_records_deleted)
	}
	err = newTaxEntry.InsertOrError(false)
	if err != nil {
		return err
	}

	if r.due() {
		return r.insertRegisters(run)
	}
	return nil
}

func (r *dateCompressTax) onPostDataItem(taxEntry *gtables.TaxEntry, run *Run) error {
	r.window.Close()
	if r.dateComprReg.No_records_deleted > r.noOfDeleted {
		return r.insertRegisters(run)
	}
	return nil
}

// initRegisters (InitRegisters): the G/L register (with the tax entry range) and the Date
// Compr. Register row
func (r *dateCompressTax) initRegisters(run *Run) {
	r.initGLReg()
	r.glReg.From_tax_entry_no = r.lastTaxEntryNo + 1
	taxEntry := gtables.NewTaxEntry(run)
	language := fcodeunits.CurrentLanguage()
	r.initRegister(r.glReg.No, r.sourceCodeSetup.Compress_tax_entries, func(field string) string { return taxEntry.FieldCaption(field, language) })
}

// insertRegisters (InsertRegisters): a G/L entry of 0 and the registers
func (r *dateCompressTax) insertRegisters(run *Run) error {
	err := r.insertGLEntry(run)
	if err != nil {
		return err
	}
	r.glReg.To_tax_entry_no = r.newTaxEntry.Entry_no
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
	taxEntry := gtables.NewTaxEntry(run)
	glEntry.FindLast()
	taxEntry.FindLast()
	if r.lastEntryNo != glEntry.Entry_no || r.lastTaxEntryNo != taxEntry.Entry_no {
		r.lastEntryNo = glEntry.Entry_no
		r.lastTaxEntryNo = taxEntry.Entry_no
		r.nextTransactionNo = glEntry.Transaction_no + 1
		r.initRegisters(run)
	}
	return nil
}
