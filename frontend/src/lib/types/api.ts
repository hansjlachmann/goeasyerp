// API Response types
export interface ApiResponse<T = any> {
	success: boolean;
	data?: T;
	error?: string;
	captions?: CaptionData;
}

export interface CaptionData {
	table?: string;
	fields?: Record<string, string>;
	field_types?: Record<string, string>;
	options?: Record<string, Record<string, string>>;
	lookups?: Record<string, LookupData>;
}

// Lookup data structures (for table relations / dropdowns)
export interface LookupColumn {
	source: string;
	width: number;
}

export interface LookupData {
	columns?: LookupColumn[];
	rows?: Array<{ _key: string; [key: string]: any }>;
	simple?: Record<string, string>;
	search_timeout?: number;
	// Large related table: no rows are sent; the dropdown loads them on demand from this URL
	lazy_url?: string;
	total?: number;
	free_text?: boolean; // the relation is the lookup only: typed text is kept (e.g. a filter)
	new_page_id?: number; // the related table's list page: the lookup's New opens it with a new record
	key_field?: string; // the related table's field the relation matches (the rows' _key)
	variants?: Record<string, LookupData>; // a conditional relation: the lookup per value of variant_field
	variant_field?: string;
	filters?: { column: string; field?: string; const?: string }[]; // a relation with WHERE filters: rows matching the record
	new_path?: string; // New with the relation's filters (set by lookupFor): the new record gets them
}

// Table record type (generic)
export interface TableRecord {
	[key: string]: any;
}

// Result of a field validation (POST /tables/:table/validate)
export interface ValidateFieldResult {
	valid: boolean;
	error?: string;
	record?: TableRecord; // Record after the OnValidate trigger ran (may fill sibling fields)
}

// List response with pagination
export interface ListResponse<T = TableRecord> {
	records: T[];
	total: number;
	page: number;
	page_size: number;
	offset?: number; // position of the first record in the full list (windowed loads)
}

// Filter types
export interface TableFilter {
	field: string;
	expression: string; // filter expression: supports *, |, .., <, >, etc.
}

/** A fixed filter: an exact value the user cannot change (a subpage's link to its card) */
export interface FixedFilter {
	field: string;
	value: unknown;
	expression?: boolean; // value is a filter expression (a range from the page's OnOpenPage)
}

export interface ListOptions {
	filters?: TableFilter[];
	fixed_filters?: FixedFilter[]; // a subpage's link: filter group 2, under the user's filters
	sort_by?: string;
	sort_order?: 'asc' | 'desc';
	page?: number;
	page_size?: number;
	offset?: number; // window: skip this many records ...
	limit?: number; // ... and return at most this many (total = all matching records)
	search?: string; // case-insensitive "contains" over search_fields
	search_fields?: string[];
	flow_filters?: TableFilter[]; // FlowFilter fields (e.g. Date Filter) the FlowFields apply, ISO dates
	fields?: string[]; // Only load these fields (useful to skip expensive FlowFields)
	page_id?: number; // the page showing the list: its triggers fill its calculated fields
	page_params?: Record<string, string>; // the page's parameters (a page opened from code), read by its triggers
}

// Dialog result from codeunit
export interface DialogResult {
	title: string;
	message: string;
	type: 'info' | 'success' | 'warning' | 'error';
}

// Codeunit execution result
export interface CodeunitResult {
	success: boolean;
	message: string;
	data?: Record<string, any>;
	dialog?: DialogResult;
}
