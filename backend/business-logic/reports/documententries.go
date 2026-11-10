package reports

import (
	"github.com/hansjlachmann/goeasyerp/backend/business-logic/navigate"
	"github.com/hansjlachmann/goeasyerp/backend/foundation/system"
	ftables "github.com/hansjlachmann/goeasyerp/backend/foundation/tables"
	"github.com/hansjlachmann/goeasyerp/backend/foundation/types"
	gtables "github.com/hansjlachmann/goeasyerp/backend/generated/tables"
)

// Report 35 Document Entries (Print on page 344 Navigate): what Navigate finds for the Document
// No. and Posting Date filters — the tables with their number of records, and with Show
// Entries each table's entries (entry or document no., posting date, document no., account,
// description, amount). Run from Navigate it takes the page's filters (the external search's
// too: the documents found by the external document no.).
const DocumentEntriesID = 35

func init() {
	Register(DocumentEntriesID, "document-entries", func() Report { return &documentEntries{} })
}

// docEntryLine is a table Navigate found
type docEntryLine struct {
	tableID                                  int
	tableName                                string
	noOfRecords                              int
	docNoFilter, noFilter, postingDateFilter string
}

// docEntryRow is an entry of a table found, its fields as printed
type docEntryRow struct {
	no, documentNo, account, description string
	postingDate                          types.Date
	amount                               types.Decimal
	hasAmount                            bool
}

type documentEntries struct {
	lines   []docEntryLine
	rows    []docEntryRow
	records map[int]ftables.Table
}

func (r *documentEntries) Definition() *Definition {
	return &Definition{
		ContextTable: gtables.DocumentEntryTableName,
		Options: []Option{
			{Name: "document_no", Type: "text"},
			{Name: "posting_date", Type: "text"},
			{Name: "show_entries", Type: "bool", Default: "true"},
		},
		OnInitReport: r.onInitReport,
		OnPreReport:  r.onPreReport,
		DataItems: []*DataItem{{
			Name: "DocEntry", Table: IntegerTable,
			OnPreDataItem: Trigger(func(rec *Integer, run *Run) error {
				rec.SetRange("number", 1, len(r.lines))
				return nil
			}),
			OnAfterGetRecord: Trigger(r.docEntryOnAfterGetRecord),
			DataItems: []*DataItem{{
				Name: "Entry", Table: IntegerTable,
				OnPreDataItem: Trigger(func(rec *Integer, run *Run) error {
					rec.SetRange("number", 1, len(r.rows))
					return nil
				}),
				OnAfterGetRecord: Trigger(r.entryOnAfterGetRecord),
			}},
		}},
	}
}

// onInitReport: run from Navigate, the page's search (its parameters)
func (r *documentEntries) onInitReport(run *Run) error {
	docNoFilter := system.PageParam("document_no")
	postingDateFilter := system.PageParam("posting_date")
	if docNoFilter == "" && postingDateFilter == "" {
		docEntry, ok := run.Context().(*gtables.DocumentEntry)
		if ok {
			docNoFilter = docEntry.Document_no_filter.String()
			if docNoFilter == "" {
				docNoFilter = docEntry.No_filter.String()
			}
			postingDateFilter = docEntry.Posting_date_filter.String()
		}
	}
	run.SetOption("document_no", docNoFilter)
	run.SetOption("posting_date", postingDateFilter)
	return nil
}

// onPreReport: Navigate's FindRecords in a temporary Document Entry, its lines kept
func (r *documentEntries) onPreReport(run *Run) error {
	setReportHeadValues(run)
	docNoFilter := run.Option("document_no")
	postingDateFilter := run.Option("posting_date")
	run.SetValue("document_no_filter", docNoFilter)
	run.SetValue("posting_date_filter", postingDateFilter)
	docEntry := gtables.NewDocumentEntry(run)
	err := docEntry.InitTemporary(run.GetDB(), run.GetCompany(), run.GetDBType())
	if err != nil {
		return err
	}
	err = navigate.FindRecords(docEntry, docNoFilter, postingDateFilter)
	if err != nil {
		return err
	}
	docEntry.Reset()
	for found := docEntry.FindSet(); found; found = docEntry.Next() {
		r.lines = append(r.lines, docEntryLine{tableID: docEntry.Table_id, tableName: docEntry.Table_name.String(),
			noOfRecords: docEntry.No_of_records, docNoFilter: docEntry.Document_no_filter.String(),
			noFilter: docEntry.No_filter.String(), postingDateFilter: docEntry.Posting_date_filter.String()})
	}
	if run.OptionBool("show_entries") {
		r.records = gtables.NewRecords()
	}
	return nil
}

// docEntryOnAfterGetRecord: a table found; with Show Entries its entries for the Entry loop
func (r *documentEntries) docEntryOnAfterGetRecord(rec *Integer, run *Run) error {
	line := r.lines[rec.Number-1]
	run.SetValue("table_id", types.NewDecimalFromInt(int64(line.tableID)))
	run.SetValue("table_name", line.tableName)
	run.SetValue("no_of_records", types.NewDecimalFromInt(int64(line.noOfRecords)))
	r.rows = nil
	if r.records == nil {
		return nil
	}
	table, ok := r.records[line.tableID]
	if !ok {
		return nil
	}
	factory, ok := gtables.GetTableFactory(table.GetTableName())
	if !ok {
		return nil
	}
	entryRec := factory()
	entryRec.InitWithDBType(run.GetDB(), run.GetCompany(), run.GetDBType())
	if line.docNoFilter != "" {
		entryRec.SetFilter("document_no", line.docNoFilter)
	}
	if line.noFilter != "" {
		entryRec.SetFilter("no", line.noFilter)
	}
	if line.postingDateFilter != "" {
		entryRec.SetFilter("posting_date", line.postingDateFilter)
	}
	fields := map[string]bool{}
	for _, f := range entryRec.GetFields() {
		if !f.FlowField {
			fields[f.Name] = true
		}
	}
	for found := entryRec.FindSet(); found; found = entryRec.Next() {
		r.rows = append(r.rows, docEntryRowOf(entryRec.ToMap(), fields))
	}
	return nil
}

// docEntryRowOf: an entry's printed fields — the first of the fields each column may show that
// the table has
func docEntryRowOf(values map[string]interface{}, fields map[string]bool) docEntryRow {
	first := func(names ...string) (interface{}, bool) {
		for _, name := range names {
			if fields[name] {
				return values[name], true
			}
		}
		return nil, false
	}
	text := func(names ...string) string {
		value, ok := first(names...)
		if !ok || value == nil {
			return ""
		}
		return system.Format(value)
	}
	row := docEntryRow{
		no:         text("entry_no", "no"),
		documentNo: text("document_no", "no"),
		account: text("g_l_account_no", "gl_account_no", "customer_no", "vendor_no", "bank_account_no", "item_no", "resource_no",
			"project_no", "fa_no", "insurance_no", "sell_to_customer_no", "buy_from_vendor_no", "sell_to_buy_from_no"),
		description: text("description", "sell_to_customer_name", "buy_from_vendor_name"),
	}
	date, ok := first("posting_date")
	if ok {
		row.postingDate, _ = date.(types.Date)
	}
	amount, ok := first("amount", "base", "quantity")
	if ok {
		row.amount, row.hasAmount = amount.(types.Decimal)
	}
	return row
}

func (r *documentEntries) entryOnAfterGetRecord(rec *Integer, run *Run) error {
	row := r.rows[rec.Number-1]
	run.SetValue("entry_no", row.no)
	run.SetValue("entry_posting_date", row.postingDate)
	run.SetValue("entry_document_no", row.documentNo)
	run.SetValue("entry_account", row.account)
	run.SetValue("entry_description", row.description)
	run.SetValue("entry_has_amount", row.hasAmount)
	run.SetValue("entry_amount", row.amount)
	return nil
}
