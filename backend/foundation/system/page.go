package system

import (
	"strings"

	"github.com/hansjlachmann/goeasyerp/backend/foundation/codeunits"
	"github.com/hansjlachmann/goeasyerp/backend/foundation/tables"
)

// Action is how the user closed a page opened with RunModal
type Action int

const (
	ActionCancel       Action = iota // closed without OK (Esc), not in lookup mode
	ActionOK                         // closed with OK, not in lookup mode
	ActionLookupCancel               // lookup: closed without picking a record
	ActionLookupOK                   // lookup: OK or picked a record — read it with GetRecord
)

// PageRecord is a record a page is opened with (a generated record)
type PageRecord interface {
	FilterExpressions() []tables.FieldFilter
	KeyText() string
	GetFields() []tables.FieldInfo
	Get(primaryKey interface{}) bool
}

// Page is a page variable (Page): set it up, run it modally, and read the record the user
// picked — the classic pattern:
//
//	custList := system.NewPage(22) // Customer List
//	custList.SetTableView(&cust)   // the page shows cust's filters
//	custList.SetRecord(&cust)      // … positioned on cust
//	custList.LookupMode(true)      // OK and Cancel buttons
//	if custList.RunModal() == system.ActionLookupOK {
//		custList.GetRecord(&cust)
//	}
//
// The variable keeps its state after RunModal (it is not cleared); NewPage gives a fresh one.
type Page struct {
	id          int
	filters     []tables.FieldFilter
	flowFilters []tables.FieldFilter
	selectKey   string
	lookupMode  bool
	pickedKey   string
	pickedRow   string
	picked      bool
	params      map[string]string
}

// flowFilterRecord is a record with FlowFilters (generated): SetTableView passes them on
type flowFilterRecord interface {
	FlowFilterExpressions() []tables.FieldFilter
}

// NewPage returns a page variable for a page (by its number)
func NewPage(pageID int) *Page {
	return &Page{id: pageID}
}

// SetTableView applies a record's filters (all filter groups) and FlowFilters to the page
// (SetTableView): a card page's period grid starts with the FlowFilters (Location Filter,
// Variant Filter, Bin Filter; the Date Filter's end is the period it starts on)
func (p *Page) SetTableView(rec PageRecord) {
	p.filters = rec.FilterExpressions()
	p.flowFilters = nil
	ff, ok := rec.(flowFilterRecord)
	if ok {
		p.flowFilters = ff.FlowFilterExpressions()
	}
}

// SetRecord makes the page open on a record (SetRecord)
func (p *Page) SetRecord(rec PageRecord) {
	p.selectKey = rec.KeyText()
}

// LookupMode switches lookup mode on or off (LookupMode): in lookup mode the page has OK and
// Cancel buttons and RunModal returns ActionLookupOK or ActionLookupCancel
// SetParam gives the page a parameter its functions and triggers read with PageParam (classic
// SetGenJnlLine on a page: Apply Customer Entries gets the journal line's Applies-to ID)
func (p *Page) SetParam(name, value string) {
	if p.params == nil {
		p.params = map[string]string{}
	}
	p.params[name] = value
}

// PageParam returns a parameter of the page the running code was started from ("" when none)
func PageParam(name string) string {
	return codeunits.CurrentPageParam(name)
}

func (p *Page) LookupMode(on bool) {
	p.lookupMode = on
}

// RunModal opens the page over the running codeunit and waits until the user closes it
// (RunModal). In lookup mode: OK, or picking a record with Enter or a double-click, returns
// ActionLookupOK; Cancel, the close button or Esc returns ActionLookupCancel. Otherwise the
// page has no Cancel button: closing it returns ActionOK. Without a dialog (a scheduled run,
// a codeunit run from a card) it returns ActionLookupCancel / ActionCancel.
func (p *Page) RunModal() Action {
	p.picked, p.pickedKey, p.pickedRow = false, "", ""
	answer := codeunits.RunPage(p.id, p.filters, p.flowFilters, p.selectKey, p.lookupMode, p.params)
	switch {
	case p.lookupMode && answer["action"] == "LookupOK":
		p.picked, p.pickedKey, p.pickedRow = true, answer["key"], answer["row"]
		return ActionLookupOK
	case p.lookupMode:
		return ActionLookupCancel
	case answer["action"] == "OK":
		return ActionOK
	}
	return ActionCancel
}

// GetRecord reads the record the user picked (the page's current record when it was closed)
// into rec (GetRecord); false when no record was picked or it no longer exists
func (p *Page) GetRecord(rec PageRecord) bool {
	return p.picked && getByKeyText(rec, p.pickedKey)
}

// GetLastRow returns the row the user picked in a card page's period grid when it was closed
// with OK in lookup mode: a period's start date (ISO, classic GetLastDate) or the row's record
// key — a location, variant or bin (GetLastLocation, GetLastVariant, GetLastBin); "" when none
func (p *Page) GetLastRow() string {
	if !p.picked {
		return ""
	}
	return p.pickedRow
}

// RunModal opens a page with a record in one call (Page.RunModal(number, rec)): the record's
// filters and current record, and in lookup mode the picked record is read into rec when the
// result is ActionLookupOK. The same as a Page variable with SetTableView, SetRecord,
// LookupMode, RunModal and GetRecord.
func RunModal(pageID int, rec PageRecord, lookupMode bool) Action {
	page := NewPage(pageID)
	page.SetTableView(rec)
	page.SetRecord(rec)
	page.LookupMode(lookupMode)
	action := page.RunModal()
	if action == ActionLookupOK && !page.GetRecord(rec) {
		return ActionLookupCancel
	}
	return action
}

// getByKeyText reads the record with a key in KeyText form ("C01", or "A,B" for a composite
// primary key)
func getByKeyText(rec PageRecord, key string) bool {
	var pk []string
	for _, f := range rec.GetFields() {
		if f.PrimaryKey {
			pk = append(pk, f.Name)
		}
	}
	if len(pk) <= 1 {
		return rec.Get(key)
	}
	parts := strings.Split(key, ",")
	if len(parts) != len(pk) {
		return false
	}
	values := make(map[string]interface{}, len(pk))
	for i, name := range pk {
		values[name] = parts[i]
	}
	return rec.Get(values)
}
