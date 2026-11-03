// Page type definitions matching the Go backend

export interface PageDefinition {
	page: PageMetadata;
}

// A field above a list page: shows and edits the OnOpenPage parameter `param`; its lookup opens
// lookup_page filtered by lookup_filters (lookup field → list field) and takes lookup_field
export interface HeaderField {
	param: string;
	source: string;
	caption?: string;
	lookup_page?: number;
	lookup_field?: string;
	lookup_filters?: Record<string, string>;
	// a choice of values instead of typed text (the parameter is the chosen value)
	options?: { value: string; caption: string }[];
}

export interface PageMetadata {
	id: number;
	type: 'Card' | 'List' | 'Document' | 'Worksheet';
	name: string;
	source_table: string;
	caption: string;
	card_page_id?: number;
	list_page_id?: number; // For card pages, the associated list page
	modal_card?: boolean;
	editable?: boolean;
	enable_navigation?: boolean;
	// DelayedInsert: insert a new list row only when the user leaves the row
	delayed_insert?: boolean;
	copy_paste_allowed?: boolean; // list Ctrl+C copies the selected records, Ctrl+V pastes them
	source_table_temporary?: boolean; // the list's rows are filled from the page parameters on every load (Navigate)
	auto_split_key?: boolean; // a new line between two lines gets the line number between them
	focus_field?: string; // Field to focus when page opens
	layout: Layout;
	actions?: Action[];
	// List pages: fields above the list (page variables, a journal's Batch Name) — each edits a
	// parameter of the page's OnOpenPage trigger
	header_fields?: HeaderField[];
	// List pages: calculated fields shown under the list for the cursor row
	footer?: Field[];
	has_open_trigger?: boolean; // OnOpenPage: ask /open for the page's filters before loading
	// Primary key field names from the source table (sent by the backend), used to
	// identify records even when the PK is not shown on the page (e.g. setup tables).
	primary_key_fields?: string[];
	has_record_triggers?: boolean;
	posted?: boolean; // written by posting only (ledger entries): no new records, no delete
	setup_table?: boolean; // a single record with a blank key (setup): the card opens on it
	auto_key?: boolean; // the table's OnInsert assigns a blank key (a number series): a new record is saved without one
	has_new_record_triggers?: boolean; // OnNewRecord: new records get values from their neighbour // OnAfterGetRecord: the card asks for the field states of each record
	flow_fields?: string[]; // computed fields: not searchable on the server
	sortable_flow_fields?: string[]; // FlowFields lists can sort on (Sum/Count, by their value)
	data_caption_fields?: string[]; // cards: the fields whose values follow the caption in the title
	// FlowFilter fields of the source table (e.g. Date Filter): "Filter totals by" in the filter pane
	flow_filter_fields?: Array<{ name: string; kind: string; caption: string }>;
}

export interface Layout {
	sections?: Section[];
	repeater?: Repeater;
	matrix?: Matrix; // a matrix page: the source table's records against periods
}

export interface Section {
	name: string;
	caption: string;
	fields: Field[];
	subpage?: Subpage; // instead of fields: a list page linked to the card's record
	period_grid?: PeriodGrid; // instead of fields: the record's FlowFields per period
	matrix?: Matrix; // instead of fields: a table's records against periods
}

/** A card section of a table's records (rows) against periods (columns): a FlowField per cell */
export interface Matrix {
	table: string;
	value: string;
	entry_table?: string;
	date_filter: string;
	record_filters?: Record<string, string>; // row FlowFilter → card record field
	filters?: Field[]; // FlowFilter fields of the rows the user filters by (captions set)
	fields: Field[]; // the row's fields before the cells (captions set)
	view_by: number; // 0 Day … 4 Year, 5 Accounting Period
	editable?: boolean;
	drilldown?: number;
	drilldown_row_field?: string;
	drilldown_row_value?: string;
	drilldown_date_field?: string;
	drilldown_record_filters?: Record<string, string>; // drilldown page field → card record field
	calculated?: boolean; // columns and cells from the page's trigger, for one period
	row_filters?: Record<string, string>; // row field → card record field
}

/** A period (column) of a matrix */
export interface MatrixColumn {
	start?: string; // a period (period columns)
	end?: string;
	name?: string;
	decimal_places?: string; // calculated columns: how the values show
	percent?: boolean;
}

/** A record (row) of a matrix: its key, fields, drilldown value and cells (decimals as text) */
export interface MatrixRow {
	key: string;
	fields: Record<string, any>;
	drilldown?: string;
	values: string[];
}

/** A card section of periods (or of a table's records) with the record's FlowFields per row */
export interface PeriodGrid {
	date_filter: string;
	filters: Field[]; // FlowFilter fields the grid is filtered by (Location Filter)
	columns: Field[];
	drilldown_date_field?: string; // the date field a column's drilldown filters on
	view_by: number; // 0 Day … 4 Year, 5 Accounting Period
	rows?: { table: string; field: string; caption_field: string; filter: string; filters?: Record<string, string>; drilldown_filter_field?: string };
}

/** A row of a period grid: its period (or record) and the columns' values */
export interface PeriodGridRow {
	period_start: string;
	period_end: string;
	period_name?: string;
	key?: string;
	caption?: string;
	values: Record<string, unknown>;
}

/** A list page shown in a card section, tied to the card's record */
export interface Subpage {
	page_id: number;
	link: Record<string, string>; // subpage field → card field (e.g. document_no: "no")
}

export interface Repeater {
	fields: Field[];
}

export interface Field {
	drilldown_date_field?: string; // in a period grid: the drilldown page's date field the row's dates filter
	source: string;
	caption?: string;
	editable?: boolean;
	visible?: boolean;
	importance?: 'Promoted' | 'Standard' | 'Additional';
	style?: 'Strong' | 'Attention' | 'Favorable' | 'Unfavorable';
	table_relation?: string;
	width?: number;
	drilldown?: number;
	drilldown_filter_field?: string;
	drilldown_filter_value?: string;
	drilldown_page_field?: string; // the drilldown opens the page whose number is in this record's field
	check_box?: boolean; // a Yes/No FlowFilter above a matrix shown as a check box
	drilldown_filters?: RunPageFilter[]; // with these filters (as an action's run_page_filters)
	primary_key?: boolean;
	required?: boolean;
	masked?: boolean; // write-only secret (Masked): password input, value never shown
	decimal_places?: string; // decimals: "min:max" digits after the point (table YAML decimal_places)
	indent_field?: string; // lists: indent the cell by this field's value (levels)
	style_expr?: StyleExpr; // lists: a style when the record's field (does not) equal a value
	assist_edit?: string; // AssistEdit: run_object of the field's "…" function (table:Customer.AssistEdit)
	on_lookup?: string; // OnLookup: run_object of the field's own lookup function, giving the record back (table:Sales_Line.SelectItemEntry)
	on_after_validate?: string; // OnAfterValidate: run_object of a function run after the field is saved (table:Sales_Line.ReserveAlways)
	calculated?: boolean; // not a table field: the page's triggers fill it per record (read-only)
}

/** A conditional cell style: style when record[field] equals `equal` (or differs from `not_equal`) */
export interface StyleExpr {
	field: string;
	equal?: string | number | boolean;
	not_equal?: string | number | boolean;
	style: 'Strong' | 'Attention' | 'Favorable' | 'Unfavorable';
}

/** A filter of a page an action opens: field = the record's value field, a constant, or the range from..to (optional: none when blank) */
export interface RunPageFilter {
	field: string;
	value?: string;
	const?: string;
	from?: string;
	to?: string;
	optional?: boolean;
}

export interface Action {
	name: string;
	caption: string;
	shortcut?: string;
	promoted?: boolean;
	run_page?: number;
	run_page_filter_field?: string; // open run_page filtered: filter_field = this record's
	run_page_filter_value?: string; // filter_value field (e.g. the customer's ledger entries)
	run_page_filters?: RunPageFilter[]; // several filters (record fields, constants, or the range from..to of two fields)
	run_page_field?: string; // the page to open is the number in this record's field (instead of run_page)
	run_page_params?: Record<string, string>; // page parameters of the opened page from this record's fields (parameter → field)
	apply_header_fields?: boolean; // the header fields as typed become the page's parameters and the list loads again (Navigate's Find)
	run_object?: string; // "codeunit:455", "report:93", a function "table:Job_Queue_Entry.ShowLineNo"
	enabled?: boolean;
	items?: Action[]; // a menu button: its menu items (each an action of its own)
	image?: string; // the button's icon (actionIcons.ts names, e.g. "New", "Post"); a bullet by default
}

export interface MenuDefinition {
	menu: MenuGroup[];
}

export interface MenuGroup {
	name: string;
	icon?: string;
	items?: MenuItem[]; // For grouped menus
	page_id?: number; // For flat menus (direct link)
}

export interface MenuItem {
	name?: string;
	page_id?: number;
	run_object?: string; // runs a report or codeunit ("report:93", "codeunit:6")
	items?: MenuItem[]; // a sub-menu (Setup, Activities …)
	icon?: string;
	description?: string;
	separator?: boolean;
	enabled?: boolean; // false: its object is not built yet (shown greyed out)
}

export interface PageResponse {
	success: boolean;
	data: PageDefinition;
	captions?: {
		table_name: string;
		fields: Record<string, string>;
	};
	navigation?: Record<string, string>; // Translated navigation strings (e.g., "home")
}

export interface MenuResponse {
	success: boolean;
	data: MenuDefinition;
}

/** Fields a page trigger switched for the current record (OnAfterGetRecord) */
export interface FieldState {
	editable?: boolean;
	visible?: boolean;
}

/** A field of an input dialog or a report's request page (options) */
export interface RequestInputField {
	name: string;
	label: string;
	type: string; // "text", "number", "date", "option", "bool", "file" (the contents as base64)
	required?: boolean;
	default?: string;
	options?: string[]; // type "option": the choices; the value is the chosen number (1-based)
	// A code picked from a table (an option with a table relation): the dropdown's rows
	lookup?: {
		columns: { source: string; width: number }[];
		rows: Array<{ _key: string; [key: string]: unknown }>;
		captions?: Record<string, string>;
	};
}

/** A data item of a report's request page: its fields can be filtered as on a list page */
export interface RequestDataItem {
	name: string;
	caption: string;
	indent: number; // 0 top level, 1 under its parent, …
	fields: RequestField[];
	filters: RequestFilter[]; // to start with (the request filter fields)
}

export interface RequestField {
	name: string;
	caption: string;
	type?: string;
}

export interface RequestFilter {
	field: string;
	expression: string;
}
