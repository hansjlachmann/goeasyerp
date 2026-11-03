package reports

import (
	fcodeunits "github.com/hansjlachmann/openerp/backend/foundation/codeunits"
	"github.com/hansjlachmann/openerp/backend/foundation/system"
	"github.com/hansjlachmann/openerp/backend/foundation/types"
	gtables "github.com/hansjlachmann/openerp/backend/generated/tables"
)

// Report 1198 Date Compress Resource Ledger: the resource ledger entries of closed fiscal years
// become one entry per entry type, resource, resource group, period and sign of the quantity —
// and per the retained Document No., Work Type, Unit of Measure, Dimension 1, Dimension 2
// Code and Chargeable; a resource register of the run.
const DateCompressResourceLedgerID = 1198

func init() {
	Register(DateCompressResourceLedgerID, "date-compress-resource-ledger", func() Report { return &dateCompressResLedger{} })
}

type dateCompressResLedger struct {
	dateCompression
	ownRegister
	sourceCodeSetup *gtables.SourceCodeSetup
	newResLedgEntry *gtables.ResLedgerEntry
	lastEntryNo     int
	window          *system.Window
}

func (r *dateCompressResLedger) Definition() *Definition {
	r.tableID = gtables.ResLedgerEntryTableID
	r.fields = []retainField{{name: "document_no"}, {name: "work_type_code"}, {name: "unit_of_measure_code"},
		{name: "dimension_1_code"}, {name: "dimension_2_code"}, {name: "chargeable"}}
	return &Definition{
		OptionsCaption: "OPTIONS",
		Options:        dateComprOptions(true, r.fields),
		Transaction:    true,
		OnInitReport:   r.onInitReport,
		OnPreReport: func(run *Run) error {
			r.read(run)
			r.filter = requestFilter(run, "Res_Ledger_Entry")
			return nil
		},
		DataItems: []*DataItem{{
			Name: "Res_Ledger_Entry", Table: gtables.ResLedgerEntryTableName, Key: []string{"resource_no", "posting_date"},
			RequestFilterFields: []string{"entry_type", "resource_no", "resource_group_no"},
			OnPreDataItem:       Trigger(r.onPreDataItem),
			OnAfterGetRecord:    Trigger(r.onAfterGetRecord),
			OnPostDataItem:      Trigger(r.onPostDataItem),
		}},
	}
}

func (r *dateCompressResLedger) onPreDataItem(resLedgEntry *gtables.ResLedgerEntry, run *Run) error {
	err := r.confirm(run)
	if err != nil {
		return err
	}
	r.window = system.OpenWindow(run.Text("WINDOW"))

	r.sourceCodeSetup = gtables.GetSourceCodeSetup(run)
	err = r.sourceCodeSetup.TestField("compress_res_ledger")
	if err != nil {
		return err
	}

	r.newResLedgEntry = gtables.NewResLedgerEntry(run)
	r.newResLedgEntry.LockTable()
	r.reg = gtables.NewResourceRegister(run)
	r.reg.LockTable()
	r.dateComprReg.LockTable()

	resLedgEntry2 := gtables.NewResLedgerEntry(run)
	r.lastEntryNo = 0
	if resLedgEntry2.FindLast() {
		r.lastEntryNo = resLedgEntry2.Entry_no
	}
	resLedgEntry.SetRange("entry_no", 0, r.lastEntryNo)
	resLedgEntry.SetRange("posting_date", r.entrdDateComprJnl.Starting_date, r.entrdDateComprJnl.Ending_date)

	r.initRegisters(run)
	return nil
}

func (r *dateCompressResLedger) onAfterGetRecord(resLedgEntry *gtables.ResLedgerEntry, run *Run) error {
	// compressed with an entry before it
	stillThere := gtables.NewResLedgerEntry(run)
	if !stillThere.Get(resLedgEntry.Entry_no) {
		return Skip()
	}

	resLedgEntry2 := gtables.NewResLedgerEntry(run)
	resLedgEntry2.Copy(resLedgEntry)
	resLedgEntry2.SetCurrentKey("resource_no", "posting_date")
	resLedgEntry2.CopyFilters(resLedgEntry)
	resLedgEntry2.SetRange("entry_type", resLedgEntry2.Entry_type)
	resLedgEntry2.SetRange("resource_no", resLedgEntry2.Resource_no)
	resLedgEntry2.SetRange("resource_group_no", resLedgEntry2.Resource_group_no)
	dateFilter, periodStart, err := r.dateFilter(resLedgEntry2.Posting_date, true)
	if err != nil {
		return err
	}
	resLedgEntry2.SetFilter("posting_date", dateFilter)

	r.lastEntryNo++

	newResLedgEntry := r.newResLedgEntry
	newResLedgEntry.Init()
	newResLedgEntry.Entry_no = r.lastEntryNo
	newResLedgEntry.Entry_type = resLedgEntry2.Entry_type
	newResLedgEntry.Resource_no = resLedgEntry2.Resource_no
	newResLedgEntry.Resource_group_no = resLedgEntry2.Resource_group_no
	newResLedgEntry.Posting_date = periodStart
	newResLedgEntry.Description = r.description
	newResLedgEntry.Source_code = r.sourceCodeSetup.Compress_res_ledger
	newResLedgEntry.User_id = types.Code(fcodeunits.CurrentUserID())
	r.window.Update(1, newResLedgEntry.Resource_no)
	r.window.Update(2, newResLedgEntry.Posting_date)
	r.newRecord()
	r.window.Update(3, r.dateComprReg.No_of_new_records)

	if r.retainNo("document_no") {
		resLedgEntry2.SetRange("document_no", resLedgEntry2.Document_no)
		newResLedgEntry.Document_no = resLedgEntry2.Document_no
	}
	if r.retainNo("work_type_code") {
		resLedgEntry2.SetRange("work_type_code", resLedgEntry2.Work_type_code)
		newResLedgEntry.Work_type_code = resLedgEntry2.Work_type_code
	}
	if r.retainNo("unit_of_measure_code") {
		resLedgEntry2.SetRange("unit_of_measure_code", resLedgEntry2.Unit_of_measure_code)
		newResLedgEntry.Unit_of_measure_code = resLedgEntry2.Unit_of_measure_code
	}
	if r.retainNo("dimension_1_code") {
		resLedgEntry2.SetRange("dimension_1_code", resLedgEntry2.Dimension_1_code)
		newResLedgEntry.Dimension_1_code = resLedgEntry2.Dimension_1_code
	}
	if r.retainNo("dimension_2_code") {
		resLedgEntry2.SetRange("dimension_2_code", resLedgEntry2.Dimension_2_code)
		newResLedgEntry.Dimension_2_code = resLedgEntry2.Dimension_2_code
	}
	if r.retainNo("chargeable") {
		resLedgEntry2.SetRange("chargeable", resLedgEntry2.Chargeable)
		newResLedgEntry.Chargeable = resLedgEntry2.Chargeable
	}
	if !resLedgEntry2.Quantity.IsNegative() {
		resLedgEntry2.SetFilter("quantity", ">=0")
	} else {
		resLedgEntry2.SetFilter("quantity", "<0")
	}
	for found := resLedgEntry2.FindFirst(); found; found = resLedgEntry2.FindFirst() {
		newResLedgEntry.Quantity = newResLedgEntry.Quantity.Add(resLedgEntry2.Quantity)
		newResLedgEntry.Total_cost = newResLedgEntry.Total_cost.Add(resLedgEntry2.Total_cost)
		newResLedgEntry.Total_price = newResLedgEntry.Total_price.Add(resLedgEntry2.Total_price)
		err = resLedgEntry2.DeleteOrError(false)
		if err != nil {
			return err
		}
		r.deleted()
		r.window.Update(4, r.dateComprReg.No_records_deleted)
	}
	err = newResLedgEntry.InsertOrError(false)
	if err != nil {
		return err
	}

	if r.due() {
		return r.insertRegisters(run)
	}
	return nil
}

func (r *dateCompressResLedger) onPostDataItem(resLedgEntry *gtables.ResLedgerEntry, run *Run) error {
	r.window.Close()
	if r.dateComprReg.No_records_deleted > r.noOfDeleted {
		return r.insertRegisters(run)
	}
	return nil
}

// initRegisters (InitRegisters): the resource register of the new entries and the Date Compr.
// Register row
func (r *dateCompressResLedger) initRegisters(run *Run) {
	registerNo := r.init(r.sourceCodeSetup.Compress_res_ledger, r.lastEntryNo)
	resLedgEntry := gtables.NewResLedgerEntry(run)
	language := fcodeunits.CurrentLanguage()
	r.initRegister(registerNo, r.sourceCodeSetup.Compress_res_ledger, func(field string) string { return resLedgEntry.FieldCaption(field, language) })
}

// insertRegisters (InsertRegisters): the registers inserted the first time, modified after
func (r *dateCompressResLedger) insertRegisters(run *Run) error {
	err := r.save(r.newResLedgEntry.Entry_no)
	if err != nil {
		return err
	}
	err = r.saveRegister(r.regExists)
	if err != nil {
		return err
	}
	r.regExists = true

	resLedgEntry2 := gtables.NewResLedgerEntry(run)
	if resLedgEntry2.FindLast() && r.lastEntryNo != resLedgEntry2.Entry_no {
		r.lastEntryNo = resLedgEntry2.Entry_no
		r.initRegisters(run)
	}
	return nil
}
