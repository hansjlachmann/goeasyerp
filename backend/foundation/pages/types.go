package pages

// PageDefinition represents a complete page definition from YAML
type PageDefinition struct {
	Page PageMetadata `yaml:"page" json:"page"`
}

// PageMetadata contains page metadata and structure
type PageMetadata struct {
	ID               int    `yaml:"id" json:"id"`
	Type             string `yaml:"type" json:"type"` // Card, List, Document, Worksheet
	Name             string `yaml:"name" json:"name"`
	SourceTable      string `yaml:"source_table" json:"source_table"`
	Caption          string `yaml:"caption" json:"caption"`
	CardPageID       int    `yaml:"card_page_id,omitempty" json:"card_page_id,omitempty"`
	ListPageID       int    `yaml:"list_page_id,omitempty" json:"list_page_id,omitempty"` // For card pages, the associated list page
	ModalCard        *bool  `yaml:"modal_card,omitempty" json:"modal_card,omitempty"`
	Editable         *bool  `yaml:"editable,omitempty" json:"editable,omitempty"`
	EnableNavigation *bool  `yaml:"enable_navigation,omitempty" json:"enable_navigation,omitempty"`
	// DelayedInsert (DelayedInsert): on editable lists, insert a new record only when
	// the user leaves the row, not on the first validated field. For pages where the user
	// types a composite key (e.g. User Members), so no half-entered key is ever saved.
	DelayedInsert *bool `yaml:"delayed_insert,omitempty" json:"delayed_insert,omitempty"`
	// CopyPasteAllowed (CopyPasteAllowed): on list pages, Ctrl+C copies the selected
	// records and Ctrl+V inserts them (same table; new keys from the list's filters, or
	// another company)
	CopyPasteAllowed *bool `yaml:"copy_paste_allowed,omitempty" json:"copy_paste_allowed,omitempty"`
	// AutoSplitKey (AutoSplitKey): on list pages whose last primary key field is a line number
	// (int), a new line inserted between two lines gets the number halfway between them (lines
	// are numbered in steps of 10000) — document lines
	AutoSplitKey *bool  `yaml:"auto_split_key,omitempty" json:"auto_split_key,omitempty"`
	FocusField   string `yaml:"focus_field,omitempty" json:"focus_field,omitempty"` // Field to focus when page opens
	// SourceTableTemporary (SourceTableTemporary, list pages): the page shows a temporary record
	// its OnFindRecords trigger fills from the page parameters on every load (Navigate); nothing
	// is read from or written to the table in the database
	SourceTableTemporary bool     `yaml:"source_table_temporary,omitempty" json:"source_table_temporary,omitempty"`
	Layout               Layout   `yaml:"layout" json:"layout"`
	Actions              []Action `yaml:"actions,omitempty" json:"actions,omitempty"`
	// HeaderFields (list pages): fields above the list that are not table fields (classic page
	// variables, e.g. a journal's Batch Name) — each edits a parameter of the page's OnOpenPage
	// trigger, which turns it into the list's filters
	HeaderFields []HeaderField `yaml:"header_fields,omitempty" json:"header_fields,omitempty"`
	// Footer (list pages): calculated fields shown under the list for the cursor row (a
	// journal's Account Name, Balance, Total Balance); OnAfterGetRecord fills them per row
	Footer []Field `yaml:"footer,omitempty" json:"footer,omitempty"`
	// HasOpenTrigger (populated at request time): the page has an OnOpenPage trigger — the
	// page asks POST /api/pages/:id/open for its filters before it loads its records
	HasOpenTrigger bool `yaml:"-" json:"has_open_trigger,omitempty"`
	// DataCaptionFields (populated at request time from the table's data_caption_fields): the
	// fields whose values a card shows in its title
	DataCaptionFields []string `yaml:"-" json:"data_caption_fields,omitempty"`
	// HasRecordTriggers (populated at request time): the page has OnAfterGetRecord triggers
	// (business-logic/pages) — the card asks for the field states of each record it shows
	HasRecordTriggers bool `yaml:"-" json:"has_record_triggers,omitempty"`
	// HasNewRecordTriggers (populated at request time): the page has OnNewRecord triggers — a
	// new record's initial values come from /init with the page and the neighbouring record
	HasNewRecordTriggers bool `yaml:"-" json:"has_new_record_triggers,omitempty"`
	// SetupTable (populated at request time): the source table holds a single record with a
	// blank key (setup); its card page opens on that record
	SetupTable bool `yaml:"-" json:"setup_table,omitempty"`
	// AutoKey (populated at request time): the source table's OnInsert assigns a blank key (the
	// next number of a series) — a new record is saved without one
	AutoKey bool `yaml:"-" json:"auto_key,omitempty"`
	// Posted (populated at request time): the source table is written by posting only (posted:
	// true — ledger entries, registers); the page starts no new records and has no New/Delete
	Posted bool `yaml:"-" json:"posted,omitempty"`
	// PrimaryKeyFields is populated at request time from table metadata so the
	// frontend knows the record key even when the PK is not shown (e.g. setup tables).
	PrimaryKeyFields []string `yaml:"-" json:"primary_key_fields,omitempty"`
	// FlowFields (populated at request time) are computed, not stored: list pages can
	// not sort or search on them server-side.
	FlowFields []string `yaml:"-" json:"flow_fields,omitempty"`
	// SortableFlowFields (populated at request time): the FlowFields lists can sort on
	// (Sum/Count, sorted on the server by their calculated value)
	SortableFlowFields []string `yaml:"-" json:"sortable_flow_fields,omitempty"`
	// FlowFilterFields (populated at request time): the source table's FlowFilter fields
	// (e.g. Date Filter), offered in the filter pane under "Filter totals by"
	FlowFilterFields []FlowFilterField `yaml:"-" json:"flow_filter_fields,omitempty"`
}

// HeaderField is a field above a list (a page variable): it shows and edits the page parameter
// Param of the OnOpenPage trigger (e.g. batch). Source names the list field it stands for (its
// caption and type); its lookup opens LookupPage filtered to the list's filters on the fields
// of LookupFilters (lookup field → list field) and takes the picked record's LookupField.
type HeaderField struct {
	Param         string            `yaml:"param" json:"param"`
	Source        string            `yaml:"source" json:"source"`
	Caption       string            `yaml:"caption,omitempty" json:"caption,omitempty"`
	LookupPage    int               `yaml:"lookup_page,omitempty" json:"lookup_page,omitempty"`
	LookupField   string            `yaml:"lookup_field,omitempty" json:"lookup_field,omitempty"`
	LookupFilters map[string]string `yaml:"lookup_filters,omitempty" json:"lookup_filters,omitempty"`
	// Options (optional): the header field is a choice of these values instead of typed text
	// (a classic option page variable: Report Selection's Usage) — the parameter is the chosen
	// option's Value; its Caption is translated as pages.<page>.options.<param>.<caption key>
	Options []HeaderOption `yaml:"options,omitempty" json:"options,omitempty"`
}

// HeaderOption is a value of a header field with options
type HeaderOption struct {
	Value   string `yaml:"value" json:"value"`
	Caption string `yaml:"caption" json:"caption"`
}

// FlowFilterField is a FlowFilter field as the page sends it: name, kind and caption
type FlowFilterField struct {
	Name    string `json:"name"`
	Kind    string `json:"kind"` // date, bool, int, text, code
	Caption string `json:"caption"`
}

// Layout defines the page layout structure
type Layout struct {
	Sections []Section `yaml:"sections,omitempty" json:"sections,omitempty"` // For Card pages
	Repeater *Repeater `yaml:"repeater,omitempty" json:"repeater,omitempty"` // For List pages
	// Matrix: a page that is a matrix of the source table's records against periods (classic
	// matrix forms on a table: Resource Capacity) — the page's matrix section "" (no card record)
	Matrix *Matrix `yaml:"matrix,omitempty" json:"matrix,omitempty"`
}

// Section represents a section/group on a Card page
type Section struct {
	Name    string  `yaml:"name" json:"name"`
	Caption string  `yaml:"caption" json:"caption"`
	Fields  []Field `yaml:"fields" json:"fields"`
	// Subpage: instead of fields, the section shows a list page with the records linked to
	// the card's record (e.g. a document's lines)
	Subpage *Subpage `yaml:"subpage,omitempty" json:"subpage,omitempty"`
	// PeriodGrid: instead of fields, the section shows the record's FlowFields per period
	// (classic matrix-by-periods forms: Item Availability by Periods, Item Turnover)
	PeriodGrid *PeriodGrid `yaml:"period_grid,omitempty" json:"period_grid,omitempty"`
	// Matrix: instead of fields, the section shows a table's records against periods (classic
	// matrix forms: G/L Budget by Periods)
	Matrix *Matrix `yaml:"matrix,omitempty" json:"matrix,omitempty"`
}

// Matrix is a card section of a table's records (rows) against periods (columns): each cell is
// the row record's FlowField Value with the column's period in its FlowFilter DateFilter (View
// as Net Change: the period, Balance at Date: up to its end), RecordFilters (a FlowFilter of
// the row → a field of the card record: the budget) and the Filters the user sets above the
// matrix. With Editable (and the page's OnMatrixValidate trigger) a value typed into a cell is
// handed to the trigger with the row's record so filtered.
type Matrix struct {
	Table string `yaml:"table" json:"table"`
	Value string `yaml:"value" json:"value"`
	// EntryTable: the table a value entered writes to (the user needs Insert permission on it)
	EntryTable    string            `yaml:"entry_table,omitempty" json:"entry_table,omitempty"`
	DateFilter    string            `yaml:"date_filter" json:"date_filter"`
	RecordFilters map[string]string `yaml:"record_filters,omitempty" json:"record_filters,omitempty"`
	// Filters: FlowFilter fields of the rows the user filters the matrix by (Department Filter);
	// drilldown_filter_field: the field of the drilldown page they filter
	Filters []Field `yaml:"filters,omitempty" json:"filters,omitempty"`
	// Fields: the row's fields before the cells (indent_field / style_expr as on lists)
	Fields []Field `yaml:"fields" json:"fields"`
	// ViewBy: the period length the matrix starts with (0 Day … 4 Year, 5 Accounting Period)
	ViewBy   int  `yaml:"view_by" json:"view_by"`
	Editable bool `yaml:"editable,omitempty" json:"editable,omitempty"`
	// Drilldown: a cell opens this page filtered to the row (DrilldownRowField = the row's
	// DrilldownRowValue field — when blank its key: a total account's Totaling), the column's
	// dates (DrilldownDateField), the card record (DrilldownRecordFilters: drilldown page field
	// → card field) and the user's filters
	// Calculated: the columns and the cells come from the page's OnMatrixCalculate trigger
	// (an account schedule's column layout) for one period chosen above the matrix (View by,
	// previous/next) — the period goes into DateFilter; RowFilters (a field of the rows → a
	// field of the card record) select the rows (the schedule's lines)
	Calculated             bool              `yaml:"calculated,omitempty" json:"calculated,omitempty"`
	RowFilters             map[string]string `yaml:"row_filters,omitempty" json:"row_filters,omitempty"`
	Drilldown              int               `yaml:"drilldown,omitempty" json:"drilldown,omitempty"`
	DrilldownRowField      string            `yaml:"drilldown_row_field,omitempty" json:"drilldown_row_field,omitempty"`
	DrilldownRowValue      string            `yaml:"drilldown_row_value,omitempty" json:"drilldown_row_value,omitempty"`
	DrilldownDateField     string            `yaml:"drilldown_date_field,omitempty" json:"drilldown_date_field,omitempty"`
	DrilldownRecordFilters map[string]string `yaml:"drilldown_record_filters,omitempty" json:"drilldown_record_filters,omitempty"`
}

// PeriodGrid is a card section listing periods (days … years, accounting periods) — or, with
// Rows, the records of a table — with the card record's FlowFields calculated for each: the
// row's period goes into the FlowFilter DateFilter (View as Net Change: the period, Balance at
// Date: up to its end). Rows from a table (Item Availability by Location) set their key into
// the FlowFilter Rows.Filter and share the one period chosen above the grid.
type PeriodGrid struct {
	DateFilter string `yaml:"date_filter" json:"date_filter"`
	// Filters: FlowFilter fields of the record the user filters the grid by (Location Filter)
	Filters []Field `yaml:"filters" json:"filters"`
	// Columns: FlowFields of the record, or calculated fields its page's OnAfterGetRecord
	// trigger fills for each row; drilldown opens a page filtered to the record (Drilldown
	// filter field/value) and the row's dates (DrilldownDateField)
	Columns            []Field `yaml:"columns" json:"columns"`
	DrilldownDateField string  `yaml:"drilldown_date_field,omitempty" json:"drilldown_date_field,omitempty"`
	// ViewBy: the period length the grid starts with (0 Day … 4 Year, 5 Accounting Period)
	ViewBy int `yaml:"view_by" json:"view_by"`
	// Rows: the rows are the records of a table instead of periods
	Rows *PeriodGridRows `yaml:"rows,omitempty" json:"rows,omitempty"`
}

// PeriodGridRows are the records a period grid lists (locations, variants): Table's records —
// those matching Filters (a field of the row table → a field of the card record, the variants
// of the item) — keyed by Field and named by CaptionField; each row's key goes into the record's
// FlowFilter Filter
type PeriodGridRows struct {
	Table        string            `yaml:"table" json:"table"`
	Field        string            `yaml:"field" json:"field"`
	CaptionField string            `yaml:"caption_field" json:"caption_field"`
	Filter       string            `yaml:"filter" json:"filter"`
	Filters      map[string]string `yaml:"filters,omitempty" json:"filters,omitempty"`
	// DrilldownFilterField: the field of a column's drilldown page the row's key filters
	// (location_code)
	DrilldownFilterField string `yaml:"drilldown_filter_field,omitempty" json:"drilldown_filter_field,omitempty"`
}

// Subpage is a list page shown in a card, tied to the card's record by Link (SubPageLink)
type Subpage struct {
	PageID int `yaml:"page_id" json:"page_id"` // the list page
	// Link: field of the subpage's table → field of the card's table (e.g. document_no: no);
	// the subpage shows only the records with these values (filter group 2, the user cannot
	// change it), and new records get them
	Link map[string]string `yaml:"link" json:"link"`
}

// Repeater represents a repeater (table) on a List page
type Repeater struct {
	Fields []Field `yaml:"fields" json:"fields"`
}

// Field represents a single field on a page
type Field struct {
	Source               string `yaml:"source" json:"source"`                                                     // Field name from table
	Caption              string `yaml:"caption,omitempty" json:"caption,omitempty"`                               // Override caption
	Editable             *bool  `yaml:"editable,omitempty" json:"editable,omitempty"`                             // false: read-only (also from the table field's editable: false)
	Visible              *bool  `yaml:"visible,omitempty" json:"visible,omitempty"`                               // Field is visible (default true)
	Importance           string `yaml:"importance,omitempty" json:"importance,omitempty"`                         // Promoted, Standard, Additional
	Style                string `yaml:"style,omitempty" json:"style,omitempty"`                                   // Strong, Attention, Favorable, Unfavorable
	TableRelation        string `yaml:"table_relation,omitempty" json:"table_relation,omitempty"`                 // Lookup table
	Width                int    `yaml:"width,omitempty" json:"width,omitempty"`                                   // Column width (for List pages)
	Drilldown            int    `yaml:"drilldown,omitempty" json:"drilldown,omitempty"`                           // Page ID to navigate to on click
	DrilldownFilterField string `yaml:"drilldown_filter_field,omitempty" json:"drilldown_filter_field,omitempty"` // Field on target table to filter
	DrilldownFilterValue string `yaml:"drilldown_filter_value,omitempty" json:"drilldown_filter_value,omitempty"` // Field on current record for filter value
	// DrilldownPageField / DrilldownFilters: the drilldown opens the page whose number is in
	// this record's field, filtered like an action's run_page_filters (Navigate's No. of Records)
	DrilldownPageField string          `yaml:"drilldown_page_field,omitempty" json:"drilldown_page_field,omitempty"`
	DrilldownFilters   []RunPageFilter `yaml:"drilldown_filters,omitempty" json:"drilldown_filters,omitempty"`
	// DrilldownDateField: in a period grid, the date field of the drilldown page the row's dates
	// filter (else the grid's)
	DrilldownDateField string `yaml:"drilldown_date_field,omitempty" json:"drilldown_date_field,omitempty"`
	PrimaryKey         bool   `yaml:"-" json:"primary_key,omitempty"` // Is this the primary key field (populated at runtime)
	Required           bool   `yaml:"-" json:"required,omitempty"`    // Is this field required (populated at runtime)
	// Masked (ExtendedDatatype Masked): shown as a password input, never in clear text.
	// Set in the page YAML (e.g. the User card's virtual password field) or taken from the
	// table field's masked: true
	Masked bool `yaml:"masked,omitempty" json:"masked,omitempty"`
	// DecimalPlaces: the "min:max" digits after the point the page shows — the page field's
	// own (decimal_places in the page YAML; a calculated field's), else the table field's
	// (filled in at request time)
	DecimalPlaces string `yaml:"decimal_places,omitempty" json:"decimal_places,omitempty"`
	// IndentField (lists): indent the cell by this field's value of the record, one step per
	// level (the chart of accounts indents names by Indentation)
	IndentField string `yaml:"indent_field,omitempty" json:"indent_field,omitempty"`
	// StyleExpr (lists): the cell takes Style when the record's field equals / does not equal a
	// value (an option by its number) — bold heading and total accounts
	StyleExpr *StyleExpr `yaml:"style_expr,omitempty" json:"style_expr,omitempty"`
	// Calculated: not a table field — the page's OnAfterGetRecord trigger fills it per record
	// (page.SetValue; classic page variables shown in a column); read-only, not searchable or
	// sortable. Type (e.g. types.Date) formats it; its caption is tables.<table>.fields.<source>.
	Calculated bool `yaml:"calculated,omitempty" json:"calculated,omitempty"`
	// AssistEdit: the field's "…" (classic OnAssistEdit) runs this registered function
	// (run_object "table:Customer.AssistEdit", registered WithAssistEdit) with the card's record
	AssistEdit string `yaml:"assist_edit,omitempty" json:"assist_edit,omitempty"`
	Type       string `yaml:"type,omitempty" json:"-"`
	// CheckBox: a Yes/No FlowFilter above a matrix shown as a check box (checked: the filter
	// Yes, else none) — a page variable switching the matrix, such as Use Add. Reporting Currency
	CheckBox bool `yaml:"check_box,omitempty" json:"check_box,omitempty"`
}

// StyleExpr is a conditional style of a list cell: Style when Field = Equal (or <> NotEqual)
type StyleExpr struct {
	Field    string      `yaml:"field" json:"field"`
	Equal    interface{} `yaml:"equal,omitempty" json:"equal,omitempty"`
	NotEqual interface{} `yaml:"not_equal,omitempty" json:"not_equal,omitempty"`
	Style    string      `yaml:"style" json:"style"` // Strong, Attention, Favorable, Unfavorable
}

// RunPageFilter is one filter of an action's run_page: Field of the opened page = this record's
// Value field, or Const, or the range From..To of two of its fields
type RunPageFilter struct {
	Field string `yaml:"field" json:"field"`
	Value string `yaml:"value,omitempty" json:"value,omitempty"`
	Const string `yaml:"const,omitempty" json:"const,omitempty"`
	// From / To: a range of this record's two fields (from..to), e.g. a G/L Register's
	// From Entry No. .. To Entry No.
	From string `yaml:"from,omitempty" json:"from,omitempty"`
	To   string `yaml:"to,omitempty" json:"to,omitempty"`
	// Optional: a blank value sets no filter (otherwise the action needs the value)
	Optional bool `yaml:"optional,omitempty" json:"optional,omitempty"`
}

// Action represents a page action/button
type Action struct {
	Name     string `yaml:"name" json:"name"`
	Caption  string `yaml:"caption" json:"caption"`
	Shortcut string `yaml:"shortcut,omitempty" json:"shortcut,omitempty"`
	Promoted bool   `yaml:"promoted,omitempty" json:"promoted,omitempty"`
	RunPage  int    `yaml:"run_page,omitempty" json:"run_page,omitempty"` // Open another page
	// RunPageFilterField / RunPageFilterValue (optional): open run_page filtered to
	// filter_field = this record's filter_value field (e.g. the customer's ledger entries)
	RunPageFilterField string `yaml:"run_page_filter_field,omitempty" json:"run_page_filter_field,omitempty"`
	RunPageFilterValue string `yaml:"run_page_filter_value,omitempty" json:"run_page_filter_value,omitempty"`
	// RunPageFilters (optional): several filters of run_page — each a field of the opened page
	// set to this record's field (value) or a constant (const, an option by its number);
	// classic RunFormLink=A=FIELD(x),B=CONST(y)
	RunPageFilters []RunPageFilter `yaml:"run_page_filters,omitempty" json:"run_page_filters,omitempty"`
	// RunPageField (optional): the page to open is the number in this record's field (a
	// Navigate line's entries page) instead of run_page
	RunPageField string `yaml:"run_page_field,omitempty" json:"run_page_field,omitempty"`
	// RunPageParams (optional): page parameters of the opened page (its OnOpenPage / header
	// fields) from this record's fields: parameter → field (Navigate's document_no, posting_date)
	RunPageParams map[string]string `yaml:"run_page_params,omitempty" json:"run_page_params,omitempty"`
	// ApplyHeaderFields (optional): the page's header fields as typed become its parameters and
	// the list loads again — a temporary source page searches again (Navigate's Find)
	ApplyHeaderFields bool   `yaml:"apply_header_fields,omitempty" json:"apply_header_fields,omitempty"`
	RunObject         string `yaml:"run_object,omitempty" json:"run_object,omitempty"` // Run codeunit, report, etc.
	// Enabled: actions are enabled unless the YAML says enabled: false (a missing value is
	// omitted from the JSON; a plain bool sent false and disabled such actions and their shortcuts)
	Enabled *bool `yaml:"enabled,omitempty" json:"enabled,omitempty"`
	// Items: the action is a menu button (a dropdown) with these menu items; each item is an
	// action of its own (run_object, run_page, shortcut). run_object can call a registered
	// function with the current record: "table:Job_Queue_Entry.ShowLineNo" (foundation/functions)
	Items []Action `yaml:"items,omitempty" json:"items,omitempty"`
	// Image: the button's icon — a name of the frontend's action icon set
	// (frontend/src/lib/components/icons/actionIcons.ts: New, Edit, Delete, Post, Print, …);
	// without one the button shows a bullet point
	Image string `yaml:"image,omitempty" json:"image,omitempty"`
}

// MenuDefinition represents the menu structure
type MenuDefinition struct {
	Menu []MenuGroup `yaml:"menu" json:"menu"`
}

// MenuGroup is a module of the main menu (General Ledger, Sales & Receivables …): its menu
// entries, or a direct page
type MenuGroup struct {
	Name   string     `yaml:"name" json:"name"`
	Icon   string     `yaml:"icon,omitempty" json:"icon,omitempty"`
	Items  []MenuItem `yaml:"items,omitempty" json:"items,omitempty"`     // the module's menu
	PageID int        `yaml:"page_id,omitempty" json:"page_id,omitempty"` // For flat menus (direct item)
}

// MenuItem is an entry of a module's menu: it opens a page (PageID), runs a report or codeunit
// (RunObject "report:93", "codeunit:6"), holds a sub-menu (Items, e.g. Setup, Activities) or is
// a separator. Enabled is set when the menu is sent: the entry's object exists (a sub-menu:
// one of its entries is available); an entry without one is shown greyed out (not built yet).
type MenuItem struct {
	Name        string     `yaml:"name,omitempty" json:"name,omitempty"`
	PageID      int        `yaml:"page_id,omitempty" json:"page_id,omitempty"`
	RunObject   string     `yaml:"run_object,omitempty" json:"run_object,omitempty"`
	Items       []MenuItem `yaml:"items,omitempty" json:"items,omitempty"`
	Icon        string     `yaml:"icon,omitempty" json:"icon,omitempty"`
	Description string     `yaml:"description,omitempty" json:"description,omitempty"`
	Separator   bool       `yaml:"separator,omitempty" json:"separator,omitempty"`
	Enabled     bool       `yaml:"-" json:"enabled"`
}

// cloneActions copies actions and their menu items (captions are translated per request)
func cloneActions(actions []Action) []Action {
	if actions == nil {
		return nil
	}
	out := make([]Action, len(actions))
	for i, a := range actions {
		a.Items = cloneActions(a.Items)
		out[i] = a
	}
	return out
}

// Clone returns a copy of the page definition that can be changed per request (translated
// captions, primary key and required flags) without touching the registry's shared one.
func (p *PageDefinition) Clone() *PageDefinition {
	c := *p
	m := &c.Page
	m.Layout.Sections = make([]Section, len(p.Page.Layout.Sections))
	for i, s := range p.Page.Layout.Sections {
		s.Fields = append([]Field{}, s.Fields...) // never null (a subpage section has none)
		if s.PeriodGrid != nil {
			g := *s.PeriodGrid
			g.Filters = append([]Field{}, g.Filters...)
			g.Columns = append([]Field{}, g.Columns...)
			s.PeriodGrid = &g
		}
		if s.Matrix != nil {
			mx := *s.Matrix
			mx.Filters = append([]Field{}, mx.Filters...)
			mx.Fields = append([]Field{}, mx.Fields...)
			s.Matrix = &mx
		}
		m.Layout.Sections[i] = s
	}
	if p.Page.Layout.Matrix != nil {
		mx := *p.Page.Layout.Matrix
		mx.Filters = append([]Field{}, mx.Filters...)
		mx.Fields = append([]Field{}, mx.Fields...)
		m.Layout.Matrix = &mx
	}
	if p.Page.Layout.Repeater != nil {
		r := *p.Page.Layout.Repeater
		r.Fields = append([]Field(nil), r.Fields...)
		m.Layout.Repeater = &r
	}
	m.Actions = cloneActions(p.Page.Actions)
	m.HeaderFields = append([]HeaderField(nil), p.Page.HeaderFields...)
	for i := range m.HeaderFields {
		m.HeaderFields[i].Options = append([]HeaderOption(nil), m.HeaderFields[i].Options...)
	}
	m.Footer = append([]Field(nil), p.Page.Footer...)
	m.PrimaryKeyFields = append([]string(nil), p.Page.PrimaryKeyFields...)
	m.FlowFields = append([]string(nil), p.Page.FlowFields...)
	m.SortableFlowFields = append([]string(nil), p.Page.SortableFlowFields...)
	m.FlowFilterFields = append([]FlowFilterField(nil), p.Page.FlowFilterFields...)
	return &c
}
