// Package reports runs processing-only reports: batch jobs that loop over data items (the
// records of tables, nested and linked), with a request page for options and filters, and
// triggers for each data item — the classic report structure without a printout.
//
// A report is a Go type registered with its number, like a codeunit:
//
//	func init() { Register(50100, "purge-log", func() Report { return &purgeLog{} }) }
//
//	func (r *purgeLog) Definition() *Definition {
//		return &Definition{
//			Options:   []Option{{Name: "older_than_days", Type: "number", Default: "30"}},
//			DataItems: []*DataItem{{
//				Name: "JobQueue", Table: "Job_Queue", RequestFilterFields: []string{"no"},
//				DataItems: []*DataItem{{
//					Name: "LogEntry", Table: "Job_Queue_Entry",
//					Link: map[string]string{"job_queue_no": "no"}, // DataItemLink
//					OnAfterGetRecord: Trigger(r.deleteEntry),
//				}},
//			}},
//		}
//	}
//
// The factory makes a new instance for every run, so a report keeps its counters and windows
// in its own fields. Triggers return Skip(), Break() or Quit() to leave the record, the data
// item or the report. Captions and labels come from translations/<lang>/reports.yaml
// (reports.<id>.caption, reports.<id>.options.<name>). A report with a layout
// (backend/definitions/reports/<id>-<name>.yaml, see layout.go) is printed: its data items
// print their sections into a PDF.
package reports

import (
	"errors"
	"sort"
	"sync"

	ftables "github.com/hansjlachmann/openerp/backend/foundation/tables"
)

// Report is a processing-only report
type Report interface {
	// Definition returns the report's data items, options and report triggers; it is
	// called once per run on a new instance (the triggers may use the instance's fields)
	Definition() *Definition
}

// Factory makes a new instance of a report for a run
type Factory func() Report

// Definition is the structure of a report
type Definition struct {
	// Options of the request page (the report's own fields, read with Run.Option), shown
	// below the data items' filters
	Options []Option
	// OptionsCaption: the heading of the options on the request page — a text of the report
	// (reports.<id>.texts.<key>); empty: the standard "Options"
	OptionsCaption string
	// Top-level data items, run one after another
	DataItems []*DataItem
	// OnInitReport runs before the request page, after the options got their defaults: it can
	// set other defaults (SetOption); options given to the run override them
	OnInitReport func(run *Run) error
	// OnPreReport runs after the request page, before the data items (Quit stops the report)
	OnPreReport func(run *Run) error
	// OnPostReport runs after the data items (not after Quit)
	OnPostReport func(run *Run) error
	// ContextTable: run from a page of this table, the report gets the page's record (with the
	// page's filters) as its Run.Context instead of request filters — e.g. Calculate Inventory
	// fills the journal batch it is run from (classic SetItemJnlLine before RUNMODAL)
	ContextTable string
	// Transaction: the report's writes are one unit (a posting) — after the request page it
	// runs in a transaction, committed at the end and rolled back when it fails
	Transaction bool
}

// Option is a field of the request page
type Option struct {
	Name     string   // key (read with Run.Option); the label is reports.<id>.options.<name>
	Type     string   // "text", "number", "date", "option", "bool" (a check box: true/false) or "file" (a file to upload: Run.OptionFile)
	Default  string   // initial value ("option": the number, 1-based)
	Options  []string // the choices of an "option" field: text keys of the report (reports.<id>.texts.<key>)
	Required bool
	// Relation: a "text" option picked from a table (classic request page control with a
	// TableRelation, e.g. Currency) — the dropdown shows its rows, Columns the key field
	// first and the columns shown; the report checks the value itself (classic FIND)
	Relation *OptionRelation
}

// OptionRelation is the table a request page option is picked from
type OptionRelation struct {
	Table   string   // the table's API name (Currency, Work_Type, Rounding_Method)
	Columns []string // the key field first, then the columns shown (code, description)
}

// TriggerFunc is a data item trigger; rec is the data item's record (a generated record, or
// *Integer for a data item of the Integer table)
type TriggerFunc func(rec any, run *Run) error

// DataItem is a loop over the records of a table
type DataItem struct {
	Name  string // name in the report (filter keys, messages)
	Table string // the table (its API name, e.g. "Job_Queue_Entry")
	// Key and Descending: the sort order (DataItemTableView SORTING)
	Key        []string
	Descending bool
	// TableView: fixed filters (DataItemTableView WHERE), in filter group 2 — the user's
	// request filters cannot remove them
	TableView []ftables.FieldFilter
	// RequestFilterFields: fields the request page offers filters for (ReqFilterFields)
	RequestFilterFields []string
	// Link: field of this data item → field of the parent data item (DataItemLink): the
	// records of the current parent record only (filter group 4)
	Link map[string]string
	// Triggers
	OnPreDataItem    TriggerFunc // before the loop (filters can still be set); Break skips the data item
	OnAfterGetRecord TriggerFunc // for each record; Skip: the next record, Break: end of the loop
	OnPostDataItem   TriggerFunc // after the loop
	// GroupFields: the data item's groups, outermost first (classic GroupTotalFields; the key
	// sorts by them) — where a field's value changes, the groups from it inwards end, their
	// group_footer sections print, and new ones start with their group_header sections. A name
	// the table has no field of is a report value OnAfterGetRecord sets (the groups of rows a
	// report computes, in an Integer data item).
	GroupFields []string
	// OnGroupFooter runs before a group's footer sections print (classic OnPreSection of a
	// GroupFooter with CurrReport.TOTALSCAUSEDBY): group is the group field that ends, rec and
	// the values are the group's last record's, Run.GroupTotal gives the group's totals; Skip
	// leaves the footer out
	OnGroupFooter GroupTriggerFunc
	// Nested data items: run for each record of this one (DataItemIndent)
	DataItems []*DataItem
}

// GroupTriggerFunc is a trigger of a data item's group; group is the group field
type GroupTriggerFunc func(rec any, group string, run *Run) error

// GroupTrigger makes a group trigger of a function on the typed record
func GroupTrigger[T any](fn func(rec T, group string, run *Run) error) GroupTriggerFunc {
	return func(rec any, group string, run *Run) error {
		r, ok := rec.(T)
		if !ok {
			return errors.New("report group trigger: wrong record type")
		}
		return fn(r, group, run)
	}
}

// Trigger makes a data item trigger of a function on the typed record:
//
//	OnAfterGetRecord: Trigger(func(e *gtables.JobQueueEntry, run *Run) error { … })
func Trigger[T any](fn func(rec T, run *Run) error) TriggerFunc {
	return func(rec any, run *Run) error {
		r, ok := rec.(T)
		if !ok {
			return errors.New("report trigger: wrong record type")
		}
		return fn(r, run)
	}
}

var (
	errSkip  = errors.New("report: skip")
	errBreak = errors.New("report: break")
	errQuit  = errors.New("report: quit")
)

// Skip leaves the current record (CurrReport.Skip): the next record follows, nested data
// items do not run for this one
func Skip() error { return errSkip }

// Break ends the current data item (CurrReport.Break): in OnPreDataItem it is skipped, in
// OnAfterGetRecord the loop ends (OnPostDataItem still runs)
func Break() error { return errBreak }

// Quit ends the report (CurrReport.Quit): nothing more runs, OnPostReport neither
func Quit() error { return errQuit }

var (
	registryMu sync.RWMutex
	registry   = map[int]Factory{}
	names      = map[int]string{}
)

// Register adds a report with its number and a name
func Register(id int, name string, factory Factory) {
	registryMu.Lock()
	defer registryMu.Unlock()
	registry[id] = factory
	names[id] = name
}

// Get returns a report's factory
func Get(id int) (Factory, bool) {
	registryMu.RLock()
	defer registryMu.RUnlock()
	f, ok := registry[id]
	return f, ok
}

// List returns the registered reports: number → name
func List() map[int]string {
	registryMu.RLock()
	defer registryMu.RUnlock()
	result := make(map[int]string, len(names))
	for id, name := range names {
		result[id] = name
	}
	return result
}

// IDs returns the registered report numbers, sorted
func IDs() []int {
	var ids []int
	for id := range List() {
		ids = append(ids, id)
	}
	sort.Ints(ids)
	return ids
}
