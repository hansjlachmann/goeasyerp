package pages

import (
	"fmt"
	"github.com/hansjlachmann/openerp/backend/foundation/types"
	"strings"
	"sync"
)

// Page triggers: a page's business logic (business-logic/pages) switches fields per record —
// OnAfterGetRecord runs for the record a card shows and sets fields editable or visible:
//
//	pages.OnAfterGetRecord(674, func(rec *gtables.JobQueue, page *pages.Control) {
//		page.SetEditable("minutes_between_run", rec.Recurrence == gtables.JobQueue_Recurrence.Minutes)
//	})
//
// The API returns the field states with the record (POST /api/pages/:id/field-states) and the
// card applies them; the page definition says whether a page has such a trigger.

// FieldState is what a page trigger set for a field (nil: as in the page definition)
type FieldState struct {
	Editable *bool `json:"editable,omitempty"`
	Visible  *bool `json:"visible,omitempty"`
}

// Control is the page as a trigger sees it: it switches fields for the current record and
// fills its calculated fields (page fields with calculated: true, classic page variables)
type Control struct {
	states map[string]FieldState
	values map[string]interface{}
}

// SetValue fills a calculated page field for the current record (e.g. the No. Series list's
// Last No. Used of the series' current line)
func (c *Control) SetValue(field string, value interface{}) {
	if c.values == nil {
		c.values = map[string]interface{}{}
	}
	// as the records carry their fields (ToMap): dates, codes, texts and decimals as text — a
	// blank date ""
	switch v := value.(type) {
	case types.Date:
		value = v.String()
	case types.DateTime:
		value = v.String()
	case types.Code:
		value = v.String()
	case types.Text:
		value = v.String()
	case types.Decimal:
		value = v.String()
	}
	c.values[strings.ToLower(field)] = value
}

// Values returns the calculated fields the triggers filled, by field name
func (c *Control) Values() map[string]interface{} {
	if c.values == nil {
		return map[string]interface{}{}
	}
	return c.values
}

// SetEditable makes a field editable or read-only for the current record
func (c *Control) SetEditable(field string, editable bool) {
	s := c.state(field)
	s.Editable = &editable
	c.states[strings.ToLower(field)] = s
}

// SetVisible shows or hides a field for the current record
func (c *Control) SetVisible(field string, visible bool) {
	s := c.state(field)
	s.Visible = &visible
	c.states[strings.ToLower(field)] = s
}

func (c *Control) state(field string) FieldState {
	if c.states == nil {
		c.states = map[string]FieldState{}
	}
	return c.states[strings.ToLower(field)]
}

// States returns the field states the triggers set, by field name
func (c *Control) States() map[string]FieldState {
	if c.states == nil {
		return map[string]FieldState{}
	}
	return c.states
}

var (
	triggersMu     sync.RWMutex
	afterGetRecord = map[int][]func(rec any, page *Control) error{}
	newRecord      = map[int][]func(rec, xRec any, belowXRec bool) error{}
)

// OnAfterGetRecord registers a trigger of a page that runs for each record the page shows
// (OnAfterGetRecord); rec is the page's source table record (*gtables.<Table>). Several
// triggers of a page run in registration order.
func OnAfterGetRecord[T any](pageID int, fn func(rec T, page *Control)) {
	triggersMu.Lock()
	defer triggersMu.Unlock()
	afterGetRecord[pageID] = append(afterGetRecord[pageID], func(rec any, page *Control) error {
		r, ok := rec.(T)
		if !ok {
			var want T
			return fmt.Errorf("page %d: OnAfterGetRecord takes %T, the page's record is %T", pageID, want, rec)
		}
		fn(r, page)
		return nil
	})
}

// HasRecordTriggers reports whether a page has OnAfterGetRecord triggers
func HasRecordTriggers(pageID int) bool {
	triggersMu.RLock()
	defer triggersMu.RUnlock()
	return len(afterGetRecord[pageID]) > 0
}

// RunAfterGetRecord runs a page's OnAfterGetRecord triggers for a record and returns the
// field states they set
func RunAfterGetRecord(pageID int, rec any) (map[string]FieldState, error) {
	page, err := RunAfterGetRecordControl(pageID, rec)
	if err != nil {
		return nil, err
	}
	return page.States(), nil
}

// RunAfterGetRecordControl runs a page's OnAfterGetRecord triggers for a record and returns
// the page as they left it (field states and calculated values)
func RunAfterGetRecordControl(pageID int, rec any) (*Control, error) {
	triggersMu.RLock()
	triggers := afterGetRecord[pageID]
	triggersMu.RUnlock()
	page := &Control{}
	for _, trigger := range triggers {
		if err := trigger(rec, page); err != nil {
			return nil, err
		}
	}
	return page, nil
}

// OnNewRecord registers a trigger of a page that runs when the user starts a new record on it
// (OnNewRecord): rec is the new record (with the table's initial values), xRec the record the
// new one is inserted next to — below it when belowXRec, above it otherwise; an empty record
// when the page has none. Used for values taken from the neighbour (SetupNewGLAcc).
func OnNewRecord[T any](pageID int, fn func(rec, xRec T, belowXRec bool)) {
	triggersMu.Lock()
	defer triggersMu.Unlock()
	newRecord[pageID] = append(newRecord[pageID], func(rec, xRec any, below bool) error {
		r, ok1 := rec.(T)
		x, ok2 := xRec.(T)
		if !ok1 || !ok2 {
			var want T
			return fmt.Errorf("page %d: OnNewRecord takes %T, the page's record is %T", pageID, want, rec)
		}
		fn(r, x, below)
		return nil
	})
}

// HasNewRecordTriggers reports whether a page has OnNewRecord triggers
func HasNewRecordTriggers(pageID int) bool {
	triggersMu.RLock()
	defer triggersMu.RUnlock()
	return len(newRecord[pageID]) > 0
}

// RunNewRecord runs a page's OnNewRecord triggers on a new record
func RunNewRecord(pageID int, rec, xRec any, belowXRec bool) error {
	triggersMu.RLock()
	triggers := newRecord[pageID]
	triggersMu.RUnlock()
	for _, trigger := range triggers {
		if err := trigger(rec, xRec, belowXRec); err != nil {
			return err
		}
	}
	return nil
}

// FixedFilter is an exact-value filter the user cannot remove (filter group 2): a journal's
// template and batch
type FixedFilter struct {
	Field string      `json:"field"`
	Value interface{} `json:"value"`
	// Expression: Value is a filter expression (a range: ..2026-10-20, <0), not one value
	Expression bool `json:"expression,omitempty"`
}

// SelectRequest asks the user to pick a record before the page opens (classic RUNMODAL of a
// list in lookup mode in OnOpenForm): the page PageID shown with Filters; the picked record's
// Field becomes the page parameter Param, and the page opens again
type SelectRequest struct {
	PageID  int           `json:"page_id"`
	Param   string        `json:"param"`
	Field   string        `json:"field"`
	Filters []FixedFilter `json:"filters,omitempty"`
}

// Open is the page as its OnOpenPage trigger sees it: the page's parameters (from the URL,
// e.g. template and batch) and the filters it sets on the page's records
//
//	pages.OnOpenPage(39, func(rec *gtables.GenJournalLine, page *pages.Open) error {
//		page.SetFilter("journal_template_name", "GENERAL")
//		return nil
//	})
type Open struct {
	params    map[string]string
	filters   []FixedFilter
	selection *SelectRequest
}

// NewOpen returns the page for an OnOpenPage trigger with the given parameters
func NewOpen(params map[string]string) *Open {
	o := &Open{params: map[string]string{}}
	for name, value := range params {
		o.params[strings.ToLower(name)] = value
	}
	return o
}

// Param returns a page parameter ("" when not given)
func (o *Open) Param(name string) string {
	return o.params[strings.ToLower(name)]
}

// SetParam sets a page parameter: the page keeps it (URL), so it opens the same way again
func (o *Open) SetParam(name, value string) {
	o.params[strings.ToLower(name)] = value
}

// Params returns the page parameters
func (o *Open) Params() map[string]string {
	return o.params
}

// SetFilter limits the page to records whose field has the value (filter group 2: the user's
// filters narrow it, they cannot remove it); new records get the value. Setting a field again
// replaces its filter.
func (o *Open) SetFilter(field string, value interface{}) {
	field = strings.ToLower(field)
	for i, f := range o.filters {
		if f.Field == field {
			o.filters[i].Value = value
			return
		}
	}
	o.filters = append(o.filters, FixedFilter{Field: field, Value: value})
}

// SetFilterExpression limits the page with a filter expression on a field (filter group 2: a
// range or a comparison — ..2026-10-20, <0); new records get no value from it
func (o *Open) SetFilterExpression(field, expression string) {
	field = strings.ToLower(field)
	for i, f := range o.filters {
		if f.Field == field {
			o.filters[i].Value, o.filters[i].Expression = expression, true
			return
		}
	}
	o.filters = append(o.filters, FixedFilter{Field: field, Value: expression, Expression: true})
}

// Filters returns the filters the trigger set
func (o *Open) Filters() []FixedFilter {
	return o.filters
}

// SelectRecord asks the user to pick a record of a list page (in lookup mode, filtered) before
// the page opens; the picked record's field becomes the parameter param and the page's
// OnOpenPage runs again. The trigger returns after calling it.
func (o *Open) SelectRecord(pageID int, param, field string, filters ...FixedFilter) {
	o.selection = &SelectRequest{PageID: pageID, Param: strings.ToLower(param), Field: strings.ToLower(field), Filters: filters}
}

// Selection returns the pick the trigger asked for (nil: none)
func (o *Open) Selection() *SelectRequest {
	return o.selection
}

var openPage = map[int][]func(rec any, page *Open) error{}

// OnOpenPage registers a trigger that runs when a page opens (classic OnOpenForm): it reads the
// page's parameters and sets the filters of the records the page shows, or asks the user to
// pick a record first (SelectRecord); an error stops the page from opening and is shown.
func OnOpenPage[T any](pageID int, fn func(rec T, page *Open) error) {
	triggersMu.Lock()
	defer triggersMu.Unlock()
	openPage[pageID] = append(openPage[pageID], func(rec any, page *Open) error {
		r, ok := rec.(T)
		if !ok {
			var want T
			return fmt.Errorf("page %d: OnOpenPage takes %T, the page's record is %T", pageID, want, rec)
		}
		return fn(r, page)
	})
}

// HasOpenTrigger reports whether a page has OnOpenPage triggers
func HasOpenTrigger(pageID int) bool {
	triggersMu.RLock()
	defer triggersMu.RUnlock()
	return len(openPage[pageID]) > 0
}

// RunOpenPage runs a page's OnOpenPage triggers with its parameters; it stops after a trigger
// that asked the user to pick a record
func RunOpenPage(pageID int, rec any, params map[string]string) (*Open, error) {
	triggersMu.RLock()
	triggers := openPage[pageID]
	triggersMu.RUnlock()
	page := NewOpen(params)
	for _, trigger := range triggers {
		if err := trigger(rec, page); err != nil {
			return nil, err
		}
		if page.selection != nil {
			break
		}
	}
	return page, nil
}

var findRecords = map[int][]func(rec any, page *Open) error{}

// OnFindRecords registers the trigger of a page on a temporary source table
// (source_table_temporary: true): it fills the page's temporary record from the page's
// parameters (page.Param) before the list reads it — on every load, so the rows are always
// up to date (Navigate counts the entries of a document)
//
//	pages.OnFindRecords(344, func(rec *gtables.DocumentEntry, page *pages.Open) error {
//		return navigate.FindRecords(rec, page.Param("document_no"), page.Param("posting_date"))
//	})
func OnFindRecords[T any](pageID int, fn func(rec T, page *Open) error) {
	triggersMu.Lock()
	defer triggersMu.Unlock()
	findRecords[pageID] = append(findRecords[pageID], func(rec any, page *Open) error {
		r, ok := rec.(T)
		if !ok {
			var want T
			return fmt.Errorf("page %d: OnFindRecords takes %T, the page's record is %T", pageID, want, rec)
		}
		return fn(r, page)
	})
}

// RunFindRecords runs a page's OnFindRecords triggers on its temporary record
func RunFindRecords(pageID int, rec any, params map[string]string) error {
	triggersMu.RLock()
	triggers := findRecords[pageID]
	triggersMu.RUnlock()
	page := NewOpen(params)
	for _, trigger := range triggers {
		if err := trigger(rec, page); err != nil {
			return err
		}
	}
	return nil
}

var matrixValidate = map[int]func(rec any, value types.Decimal) error{}

// OnMatrixValidate registers what a value typed into a cell of a page's matrix section does
// (classic: a value entered into the FlowField of a matrix): rec is the row's record with the
// row's FlowFilters set (the column's period, the card record's and the user's filters), value
// the amount typed. One per page.
func OnMatrixValidate[T any](pageID int, fn func(rec T, value types.Decimal) error) {
	triggersMu.Lock()
	defer triggersMu.Unlock()
	matrixValidate[pageID] = func(rec any, value types.Decimal) error {
		r, ok := rec.(T)
		if !ok {
			var want T
			return fmt.Errorf("page %d: OnMatrixValidate takes %T, the matrix row is %T", pageID, want, rec)
		}
		return fn(r, value)
	}
}

// HasMatrixValidate reports whether a page's matrix cells can be entered
func HasMatrixValidate(pageID int) bool {
	triggersMu.RLock()
	defer triggersMu.RUnlock()
	return matrixValidate[pageID] != nil
}

// RunMatrixValidate runs a page's OnMatrixValidate trigger on a matrix row's record
func RunMatrixValidate(pageID int, rec any, value types.Decimal) error {
	triggersMu.RLock()
	fn := matrixValidate[pageID]
	triggersMu.RUnlock()
	if fn == nil {
		return fmt.Errorf("page %d has no OnMatrixValidate trigger", pageID)
	}
	return fn(rec, value)
}

// MatrixColumn is a column of a calculated matrix: its caption and how its values show
// (DecimalPlaces "min:max"; Percent: a percent sign after the value)
type MatrixColumn struct {
	Name          string `json:"name"`
	DecimalPlaces string `json:"decimal_places,omitempty"`
	Percent       bool   `json:"percent,omitempty"`
}

var matrixCalculate = map[int]func(card, rows any) ([]MatrixColumn, map[string][]string, error){}

// OnMatrixCalculate registers how a page's calculated matrix gets its columns and cells: card
// is the card's record (as shown), rows the rows' table record with the matrix's filters and
// FlowFilters set (the period, the user's filters); it returns the columns and, by the rows'
// key (ftables.KeyString), each row's values (decimals as text, or a text shown as it is). One
// per page.
func OnMatrixCalculate[C, R any](pageID int, fn func(card C, rows R) ([]MatrixColumn, map[string][]string, error)) {
	triggersMu.Lock()
	defer triggersMu.Unlock()
	matrixCalculate[pageID] = func(card, rows any) ([]MatrixColumn, map[string][]string, error) {
		c, ok1 := card.(C)
		r, ok2 := rows.(R)
		if !ok1 || !ok2 {
			var wantC C
			var wantR R
			return nil, nil, fmt.Errorf("page %d: OnMatrixCalculate takes %T and %T, got %T and %T", pageID, wantC, wantR, card, rows)
		}
		return fn(c, r)
	}
}

// HasMatrixCalculate reports whether a page has an OnMatrixCalculate trigger
func HasMatrixCalculate(pageID int) bool {
	triggersMu.RLock()
	defer triggersMu.RUnlock()
	return matrixCalculate[pageID] != nil
}

// RunMatrixCalculate runs a page's OnMatrixCalculate trigger
func RunMatrixCalculate(pageID int, card, rows any) ([]MatrixColumn, map[string][]string, error) {
	triggersMu.RLock()
	fn := matrixCalculate[pageID]
	triggersMu.RUnlock()
	if fn == nil {
		return nil, nil, fmt.Errorf("page %d has no OnMatrixCalculate trigger", pageID)
	}
	return fn(card, rows)
}
