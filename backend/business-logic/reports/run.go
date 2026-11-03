package reports

import (
	"database/sql"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"strings"

	fcodeunits "github.com/hansjlachmann/openerp/backend/foundation/codeunits"
	"github.com/hansjlachmann/openerp/backend/foundation/database"
	apperrors "github.com/hansjlachmann/openerp/backend/foundation/errors"
	"github.com/hansjlachmann/openerp/backend/foundation/i18n"
	"github.com/hansjlachmann/openerp/backend/foundation/pdf"
	"github.com/hansjlachmann/openerp/backend/foundation/system"
	ftables "github.com/hansjlachmann/openerp/backend/foundation/tables"
	"github.com/hansjlachmann/openerp/backend/foundation/types"
	gtables "github.com/hansjlachmann/openerp/backend/generated/tables"
)

// Run is a running report (CurrReport): its database, options and result
type Run struct {
	ID      int
	DB      database.Executor
	Company string
	DBType  database.DBType

	options map[string]string
	filters map[string][]ftables.FieldFilter // data item name → the user's filters
	message string
	context interface{}

	// A printed report: its printer, the values its sections print, the output chosen
	printer  *printer
	values   map[string]interface{}
	output   string
	fileName string

	// A report that writes a file (a declaration to send): its name and contents
	file    []byte
	fileOut string
	// The printouts of the reports a processing report printed (AddPrintouts) and the page it
	// opens afterwards (OpenPage)
	printouts []Result
	openPage  *fcodeunits.PageLink
}

// SetValue sets a value the layout prints (value, lines, image elements; print_if) — the
// classic report's global variables shown in its sections: texts, decimals, dates, whole
// numbers, booleans, lines (an address array) or a picture ([]byte)
func (r *Run) SetValue(name string, value interface{}) { r.values[name] = value }

// Value returns a value set with SetValue (nil when not set)
func (r *Run) Value(name string) interface{} { return r.values[name] }

// PageNo is the page number being printed (CurrReport.PAGENO); 0 for a report that does not
// print
func (r *Run) PageNo() int {
	if r.printer == nil {
		return 0
	}
	return r.printer.pageNumber()
}

// SetPageNo sets the page number (CurrReport.PAGENO := n): of the current page, or of the
// page about to start (a new copy starts on page 1)
func (r *Run) SetPageNo(n int) {
	if r.printer != nil {
		r.printer.setPageNo(n)
	}
}

// NewPage: the next section starts on a new page (CurrReport.NEWPAGE)
func (r *Run) NewPage() {
	if r.printer != nil && r.printer.pageOpen {
		r.printer.newPage = true
	}
}

// GroupTotal is the total of a field or value over the records of the group whose footer
// prints (in OnGroupFooter) — of a total or total_value element of the data item's layout; 0
// outside a group footer
func (r *Run) GroupTotal(name string) types.Decimal {
	if r.printer == nil || r.printer.footer == nil {
		return types.Decimal{}
	}
	sums := r.printer.footer
	total, ok := sums.values[name]
	if !ok {
		total = sums.fields[name]
	}
	return total
}

// Preview reports whether the printout is only shown (CurrReport.PREVIEW): a document is not
// counted as printed
func (r *Run) Preview() bool { return r.output == OutputPreview }

// Output is the output chosen on the request page: OutputPreview, OutputPrint or OutputPDF
func (r *Run) Output() string { return r.output }

// SetFileName sets the name of the PDF (without .pdf; default the report's caption) — also
// the document's title
func (r *Run) SetFileName(name string) { r.fileName = name }

// WriteFile makes the report's result a file (classic File.CREATE/WRITE of a processing report:
// a declaration for the authorities) — the browser downloads it under name
func (r *Run) WriteFile(name string, data []byte) {
	r.fileOut = name
	r.file = data
}

// AddPrintouts adds the printouts of reports the report printed (Make Orders printing the
// purchase orders it made): the browser shows, prints or downloads them after the report
func (r *Run) AddPrintouts(printouts ...Result) { r.printouts = append(r.printouts, printouts...) }

// OpenPage: the page the browser opens after the report (a worksheet the report moved the user
// to)
func (r *Run) OpenPage(link *fcodeunits.PageLink) { r.openPage = link }

// Context is what the caller gave the report to work on (RunOptions.Context) — e.g. the item
// journal line whose batch Calculate Inventory fills (classic SetItemJnlLine before RUNMODAL)
func (r *Run) Context() interface{} { return r.context }

// Option returns an option of the request page as text
func (r *Run) Option(name string) string { return r.options[name] }

// OptionInt returns a number option (0 when blank or not a number)
func (r *Run) OptionInt(name string) int {
	n, _ := strconv.Atoi(strings.TrimSpace(r.options[name]))
	return n
}

// OptionDate returns a date option (blank when blank or not a date)
func (r *Run) OptionDate(name string) types.Date {
	d, _ := types.NewDateFromString(strings.TrimSpace(r.options[name]))
	return d
}

// OptionBool returns a "bool" option (true for true/yes/1)
func (r *Run) OptionBool(name string) bool {
	switch strings.ToLower(strings.TrimSpace(r.options[name])) {
	case "true", "yes", "1":
		return true
	}
	return false
}

// OptionFile returns the contents of a "file" option (the file the user picked, sent as base64;
// nil when none or not base64)
func (r *Run) OptionFile(name string) []byte {
	data, err := base64.StdEncoding.DecodeString(strings.TrimSpace(r.options[name]))
	if err != nil || len(data) == 0 {
		return nil
	}
	return data
}

// OptionDecimal returns a number option as a decimal (0 when blank or not a number)
func (r *Run) OptionDecimal(name string) types.Decimal {
	d, err := types.NewDecimalFromString(strings.TrimSpace(r.options[name]))
	if err != nil {
		return types.Decimal{}
	}
	return d
}

// SetOption sets an option's value (OnInitReport: a default that depends on the data)
func (r *Run) SetOption(name, value string) { r.options[name] = value }

// SetRequestFilter sets a data item's starting request filter on a field (OnInitReport: the
// document the report is run from — classic SETRANGE on the record given to REPORT.RUNMODAL)
func (r *Run) SetRequestFilter(dataItem, field, expression string) {
	filters := r.filters[dataItem]
	for i, f := range filters {
		if f.Field == field {
			filters[i].Expression = expression
			return
		}
	}
	r.filters[dataItem] = append(filters, ftables.FieldFilter{Field: field, Expression: expression})
}

// Message sets the message shown when the report is done (filled in with StrSubstNo; pass a
// translated text — Text gives the report's own)
func (r *Run) Message(text string, values ...any) {
	r.message = system.StrSubstNo(text, values...)
}

// Text returns a translated text of the report (reports.<id>.texts.<key>) in the user's
// language, filled in with StrSubstNo
func (r *Run) Text(key string, values ...any) string {
	return system.StrSubstNo(translate(r.ID, "texts."+key), values...)
}

// RunOptions say how a report runs
type RunOptions struct {
	// RequestPage: show the request page first (only with a dialog; a scheduled run never
	// shows it)
	RequestPage bool
	// Record: its filters (all filter groups) are the starting request filters of the data
	// items of its table (Report.Run with a record)
	Record ftables.Table
	// Options: starting values of the request page options
	Options map[string]string
	// Context: a value the report works on (Run.Context), set by the caller before the run
	Context interface{}
	// Output of a printed report run without a request page: OutputPreview, OutputPrint or
	// OutputPDF (default)
	Output string
}

// Result is how a report ended
type Result struct {
	Cancelled bool   // the user cancelled the request page: nothing ran
	Message   string // set by the report with Run.Message
	// A printed report's printout: the PDF, its file name, the output chosen, the pages and
	// the texts printed (what tests check)
	PDF      []byte
	FileName string
	Output   string
	Pages    int
	Texts    []pdf.PlacedText
	// A processing report's file (Run.WriteFile): its contents (FileName its name)
	File []byte
	// The printouts of the reports a processing report printed (Run.AddPrintouts) and the page
	// to open afterwards (Run.OpenPage)
	Printouts []Result
	OpenPage  *fcodeunits.PageLink
}

// Execute runs a report (Report.Run): request page, OnPreReport, the data items, OnPostReport
func Execute(id int, db database.Executor, company string, dbType database.DBType, opts RunOptions) (Result, error) {
	factory, ok := Get(id)
	if !ok {
		return Result{}, fmt.Errorf("report %d does not exist", id)
	}
	def := factory().Definition()
	layout, err := layoutOf(id)
	if err != nil {
		return Result{}, err
	}
	run := &Run{ID: id, DB: db, Company: company, DBType: dbType, options: map[string]string{}, filters: map[string][]ftables.FieldFilter{}, context: opts.Context, values: map[string]interface{}{}, output: opts.Output}
	if layout != nil && run.output == "" {
		run.output = OutputPDF
	}
	for _, o := range def.Options {
		run.options[o.Name] = o.Default
	}
	if def.OnInitReport != nil {
		err := def.OnInitReport(run)
		if err != nil {
			return Result{}, err
		}
	}
	for name, value := range opts.Options {
		run.options[name] = value
	}
	if opts.Record != nil {
		source, ok := opts.Record.(interface{ FilterExpressions() []ftables.FieldFilter })
		if ok {
			for _, item := range allDataItems(def.DataItems) {
				if sameTable(item.Table, opts.Record) {
					run.filters[item.Name] = append(run.filters[item.Name], source.FilterExpressions()...)
				}
			}
		}
	}

	if opts.RequestPage && fcodeunits.GetCurrentDialog() != nil {
		if !requestPage(run, def, layout != nil) {
			return Result{Cancelled: true}, nil
		}
	}
	if layout != nil {
		run.printer, err = newPrinter(run, layout)
		if err != nil {
			return Result{}, err
		}
	}

	// A report that writes as one unit (posting) runs in a transaction: committed at the end,
	// rolled back on an error (a caller's transaction is used as it is)
	var tx *sql.Tx
	if def.Transaction {
		if _, ok := run.DB.(*sql.DB); ok {
			var err error
			tx, err = ftables.BeginTransaction(run.DB)
			if err != nil {
				return Result{}, err
			}
			run.DB = tx
		}
	}

	err = func() error {
		if def.OnPreReport != nil {
			err := def.OnPreReport(run)
			if err != nil {
				return err
			}
		}
		for _, item := range def.DataItems {
			err := runDataItem(run, item, nil)
			if err != nil {
				return err
			}
		}
		if def.OnPostReport != nil {
			return def.OnPostReport(run)
		}
		return nil
	}()
	if errors.Is(err, errQuit) {
		err = nil
	}
	if tx != nil {
		if err != nil {
			_ = ftables.Rollback(tx)
			return Result{}, err
		}
		err = ftables.Commit(tx)
		if err != nil {
			return Result{}, err
		}
	}
	if err != nil {
		return Result{}, err
	}
	result := Result{Message: run.message, Printouts: run.printouts, OpenPage: run.openPage}
	if run.file != nil {
		result.File = run.file
		result.FileName = run.fileOut
	}
	if run.printer != nil {
		result, err = run.printout(result)
	}
	return result, err
}

// printout finishes a printed report's PDF
func (r *Run) printout(result Result) (Result, error) {
	name := r.fileName
	if name == "" {
		name = Caption(r.ID, fcodeunits.CurrentLanguage())
	}
	r.printer.doc.SetTitle(name)
	data, err := r.printer.doc.Bytes()
	if err != nil {
		return Result{}, err
	}
	result.PDF = data
	result.FileName = name + ".pdf"
	result.Output = r.output
	result.Pages = r.printer.doc.Pages()
	result.Texts = r.printer.doc.Texts()
	return result, nil
}

// requestPage asks for the options and the data items' filters (any field, as on a list
// page; the request filter fields to start with); false when cancelled
func requestPage(run *Run, def *Definition, printed bool) bool {
	language := fcodeunits.CurrentLanguage()
	var fields []fcodeunits.InputField
	for _, o := range def.Options {
		fields = append(fields, fcodeunits.InputField{
			Name:     "option:" + o.Name,
			Label:    translate(run.ID, "options."+o.Name),
			Type:     o.Type,
			Required: o.Required,
			Default:  run.options[o.Name],
			Options:  translatedChoices(run, o.Options),
			Lookup:   optionLookup(run, o.Relation, language),
		})
	}
	var items []fcodeunits.RequestDataItem
	for _, entry := range indentedDataItems(def.DataItems, 0) {
		item := entry.item
		rec, ok := newRecord(run, item)
		if !ok {
			continue
		}
		requestItem := fcodeunits.RequestDataItem{Name: item.Name, Caption: rec.GetTableName(), Indent: entry.indent, Filters: []fcodeunits.RequestFilter{}}
		c, ok := rec.(interface{ GetCaption(string) string })
		if ok {
			requestItem.Caption = c.GetCaption(language)
		}
		caption := func(field string) string {
			c, ok := rec.(interface{ GetFieldCaption(string, string) string })
			if ok {
				return c.GetFieldCaption(field, language)
			}
			return field
		}
		for _, f := range ftables.FilterableFields(rec) {
			requestItem.Fields = append(requestItem.Fields, fcodeunits.RequestField{Name: f.Name, Caption: caption(f.Name), Type: string(f.Type)})
		}
		// FlowFilter fields too (Date Filter, Location Filter …): they limit the FlowFields
		for _, f := range rec.GetFlowFilterFields() {
			requestItem.Fields = append(requestItem.Fields, fcodeunits.RequestField{Name: f.Name, Caption: caption(f.Name), Type: string(flowFilterFieldType[f.Kind])})
		}
		// The request filter fields first (blank or with the starting filter), then the other
		// starting filters
		start := map[string]string{}
		for _, f := range run.filters[item.Name] {
			start[f.Field] = f.Expression
		}
		listed := map[string]bool{}
		for _, field := range item.RequestFilterFields {
			requestItem.Filters = append(requestItem.Filters, fcodeunits.RequestFilter{Field: field, Expression: start[field]})
			listed[field] = true
		}
		for _, f := range run.filters[item.Name] {
			if !listed[f.Field] {
				requestItem.Filters = append(requestItem.Filters, fcodeunits.RequestFilter{Field: f.Field, Expression: f.Expression})
			}
		}
		items = append(items, requestItem)
	}
	optionsCaption := ""
	if def.OptionsCaption != "" {
		optionsCaption = run.Text(def.OptionsCaption)
	}
	output := ""
	if printed {
		output = PrinterOutput(run, run.ID, OutputPreview)
	}
	values := fcodeunits.RequestPage(translate(run.ID, "caption"), fields, items, optionsCaption, output)
	if len(values) == 0 {
		return false
	}
	if printed {
		switch values[fcodeunits.OutputKey] {
		case OutputPreview, OutputPrint, OutputPDF:
			run.output = values[fcodeunits.OutputKey]
		}
	}
	for key, value := range values {
		name, ok := strings.CutPrefix(key, "option:")
		if ok {
			run.options[name] = value
		} else {
			name, ok := strings.CutPrefix(key, "filters:")
			if ok {
				var filters []ftables.FieldFilter
				err := json.Unmarshal([]byte(value), &filters)
				if err == nil {
					run.filters[name] = filters
				}
			}
		}
	}
	return true
}

// indented is a data item with its level (0 top level)
type indented struct {
	item   *DataItem
	indent int
}

// indentedDataItems returns the data items and their nested ones, in order, with their level
func indentedDataItems(items []*DataItem, level int) []indented {
	var all []indented
	for _, item := range items {
		all = append(all, indented{item, level})
		all = append(all, indentedDataItems(item.DataItems, level+1)...)
	}
	return all
}

// reportRecord: what the runner needs of a generated record
type reportRecord interface {
	ftables.Table
	SetRange(fieldName string, values ...interface{})
	FilterGroup(group ...int) int
	FindSetBuffered() bool
}

// newRecord makes a data item's record, initialized for the run
func newRecord(run *Run, item *DataItem) (reportRecord, bool) {
	factory, ok := gtables.GetTableFactory(item.Table)
	if !ok {
		return nil, false
	}
	rec, ok := factory().(reportRecord)
	if !ok {
		return nil, false
	}
	rec.InitWithDBType(run.DB, run.Company, run.DBType)
	return rec, true
}

// runDataItem runs a data item: filters, OnPreDataItem, the loop with OnAfterGetRecord and
// the nested data items, OnPostDataItem. parent is the parent data item's current record. A
// printed report prints the records by the data item's layout.
func runDataItem(run *Run, item *DataItem, parent ftables.Table) error {
	if item.Table == IntegerTable {
		return runIntegerDataItem(run, item)
	}
	rec, ok := newRecord(run, item)
	if !ok {
		return fmt.Errorf("report %d: data item %s: table %q does not exist", run.ID, item.Name, item.Table)
	}
	if len(item.Key) > 0 {
		rec.SetCurrentKey(item.Key...)
	}
	rec.SetAscending(!item.Descending)
	// Fixed filters of the table view (group 2), the parent link (group 4), the user's
	// request filters (group 0)
	if len(item.TableView) > 0 {
		rec.FilterGroup(2)
		for _, f := range item.TableView {
			rec.SetFilter(f.Field, f.Expression)
		}
	}
	if parent != nil && len(item.Link) > 0 {
		values := parent.ToMap()
		rec.FilterGroup(4)
		for field, parentField := range item.Link {
			rec.SetRange(field, values[parentField])
		}
	}
	// The user's filters as on a list page (stored fields, Sum/Count FlowFields)
	rec.FilterGroup(0)
	for _, f := range run.filters[item.Name] {
		if strings.TrimSpace(f.Expression) == "" {
			continue
		}
		switch err := ftables.ApplyFilter(rec, ftables.FieldFilter{Field: f.Field, Expression: strings.TrimSpace(f.Expression)}); {
		case errors.Is(err, ftables.ErrFilterExpression):
			return apperrors.FilterInvalid(item.Table, f.Field, f.Expression)
		case err != nil:
			return apperrors.InvalidFilters()
		}
	}

	if item.OnPreDataItem != nil {
		err := item.OnPreDataItem(rec, run)
		if err != nil {
			if errors.Is(err, errBreak) {
				return nil
			}
			return err
		}
	}

	frame := run.beginItem(item, rec)
	// SQLite reads all at once: its connection cannot read and write the table at once
	var found bool
	if run.DBType == database.DBTypeSQLite {
		found = rec.FindSetBuffered()
	} else {
		found = rec.FindSet()
	}
	for ; found; found = rec.Next() {
		next, err := runRecord(run, item, rec, frame)
		if err != nil {
			return err
		}
		if !next {
			break
		}
	}
	return endDataItem(run, item, rec, frame)
}

// runIntegerDataItem runs a data item of the Integer table: the numbers of the range its
// OnPreDataItem sets
func runIntegerDataItem(run *Run, item *DataItem) error {
	rec := &Integer{}
	if item.OnPreDataItem != nil {
		err := item.OnPreDataItem(rec, run)
		if err != nil {
			if errors.Is(err, errBreak) {
				return nil
			}
			return err
		}
	}
	frame := run.beginItem(item, rec)
	if rec.ranged {
		for number := rec.from; number <= rec.to; number++ {
			rec.Number = number
			next, err := runRecord(run, item, rec, frame)
			if err != nil {
				return err
			}
			if !next {
				break
			}
		}
	}
	return endDataItem(run, item, rec, frame)
}

// runRecord runs a data item's record: OnAfterGetRecord, its printout, the nested data items;
// false when the loop ends (Break)
func runRecord(run *Run, item *DataItem, rec interface{}, frame *frame) (bool, error) {
	undoNewPage := func() {}
	if run.printer != nil {
		undoNewPage = run.printer.beforeRecord(frame)
	}
	if item.OnAfterGetRecord != nil {
		err := item.OnAfterGetRecord(rec, run)
		if errors.Is(err, errSkip) {
			undoNewPage()
			return true, nil
		}
		if errors.Is(err, errBreak) {
			undoNewPage()
			return false, nil
		}
		if err != nil {
			return false, err
		}
	}
	if run.printer != nil {
		err := run.printer.record(frame)
		if err != nil {
			return false, err
		}
	}
	table, _ := rec.(ftables.Table) // the parent of the nested data items' links
	for _, child := range item.DataItems {
		err := runDataItem(run, child, table)
		if err != nil {
			return false, err
		}
	}
	return true, nil
}

// endDataItem ends a data item's loop: its footers, OnPostDataItem
func endDataItem(run *Run, item *DataItem, rec interface{}, frame *frame) error {
	if run.printer != nil {
		err := run.printer.endItem(frame)
		if err != nil {
			return err
		}
	}
	if item.OnPostDataItem != nil {
		err := item.OnPostDataItem(rec, run)
		if err != nil && !errors.Is(err, errBreak) {
			return err
		}
	}
	return nil
}

// beginItem: a data item's loop starts (its frame in the printout; nil when nothing prints)
func (r *Run) beginItem(item *DataItem, rec interface{}) *frame {
	if r.printer == nil {
		return nil
	}
	return r.printer.beginItem(item, rec)
}

// allDataItems returns the data items and their nested ones, in order
func allDataItems(items []*DataItem) []*DataItem {
	var all []*DataItem
	for _, item := range items {
		all = append(all, item)
		all = append(all, allDataItems(item.DataItems)...)
	}
	return all
}

// sameTable reports whether a data item's table (API name) is the record's table
func sameTable(table string, rec ftables.Table) bool {
	factory, ok := gtables.GetTableFactory(table)
	return ok && factory().GetTableName() == rec.GetTableName()
}

// Caption returns the report's caption in the language (reports.<id>.caption), else its
// registered name
func Caption(id int, language string) string {
	key := fmt.Sprintf("reports.%d.caption", id)
	caption := i18n.GetInstance().Translate(key, language)
	if caption == key {
		return List()[id]
	}
	return caption
}

// translate returns reports.<id>.<key> in the user's language — else reports.shared.<key>, the
// texts and options several reports share (the date compression reports' request page)
func translate(id int, key string) string {
	language := fcodeunits.CurrentLanguage()
	full := fmt.Sprintf("reports.%d.%s", id, key)
	text := i18n.GetInstance().Translate(full, language)
	if text == full {
		shared := "reports.shared." + key
		sharedText := i18n.GetInstance().Translate(shared, language)
		if sharedText != shared {
			return sharedText
		}
	}
	return text
}

// GetDB, GetCompany and GetDBType: records a report opens are in its database and company
// (gtables.NewAccountingPeriod(run))
func (r *Run) GetDB() database.Executor   { return r.DB }
func (r *Run) GetCompany() string         { return r.Company }
func (r *Run) GetDBType() database.DBType { return r.DBType }

// flowFilterFieldType: the field type a FlowFilter's kind is shown as on the request page
var flowFilterFieldType = map[string]ftables.FieldType{
	"date": ftables.FieldTypeDate,
	"bool": ftables.FieldTypeBoolean,
	"int":  ftables.FieldTypeInteger,
	"text": ftables.FieldTypeText,
	"code": ftables.FieldTypeCode,
}

// translatedChoices: the choices of an option field in the user's language (text keys of the
// report)
func translatedChoices(run *Run, keys []string) []string {
	var choices []string
	for _, key := range keys {
		choices = append(choices, run.Text(key))
	}
	return choices
}

// optionLookupLimit: an option's dropdown shows at most this many rows of its table
const optionLookupLimit = 200

// optionLookup is the dropdown of an option with a table relation: the table's rows (at most
// optionLookupLimit) with the relation's columns; nil without a relation
func optionLookup(run *Run, relation *OptionRelation, language string) *fcodeunits.InputLookup {
	if relation == nil || len(relation.Columns) == 0 {
		return nil
	}
	factory, ok := gtables.GetTableFactory(relation.Table)
	if !ok {
		return nil
	}
	rec := factory()
	rec.InitWithDBType(run.DB, run.Company, run.DBType)
	lookup := &fcodeunits.InputLookup{Rows: []map[string]interface{}{}, Captions: map[string]string{}}
	for i, col := range relation.Columns {
		width := 250
		if i == 0 {
			width = 100
		}
		lookup.Columns = append(lookup.Columns, fcodeunits.InputLookupColumn{Source: col, Width: width})
		c, ok := rec.(interface{ GetFieldCaption(string, string) string })
		if ok {
			lookup.Captions[col] = c.GetFieldCaption(col, language)
		}
	}
	rec.SetCurrentKey(relation.Columns[0])
	seen := map[string]bool{} // a code once (Rounding Method has a row per minimum amount)
	for found := rec.FindSet(); found && len(lookup.Rows) < optionLookupLimit; found = rec.Next() {
		values := rec.ToMap()
		key := fmt.Sprint(values[relation.Columns[0]])
		if seen[key] {
			continue
		}
		seen[key] = true
		row := map[string]interface{}{"_key": key}
		for _, col := range relation.Columns {
			row[col] = values[col]
		}
		lookup.Rows = append(lookup.Rows, row)
	}
	return lookup
}
