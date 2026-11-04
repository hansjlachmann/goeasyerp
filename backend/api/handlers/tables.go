package handlers

import (
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"slices"
	"sort"
	"strconv"
	"strings"

	"github.com/gofiber/fiber/v2"
	apitypes "github.com/hansjlachmann/openerp/backend/api/types"
	"github.com/hansjlachmann/openerp/backend/business-logic/tables"
	fcodeunits "github.com/hansjlachmann/openerp/backend/foundation/codeunits"
	"github.com/hansjlachmann/openerp/backend/foundation/company"
	"github.com/hansjlachmann/openerp/backend/foundation/database"
	apperrors "github.com/hansjlachmann/openerp/backend/foundation/errors"
	"github.com/hansjlachmann/openerp/backend/foundation/flowfilter"
	"github.com/hansjlachmann/openerp/backend/foundation/i18n"
	"github.com/hansjlachmann/openerp/backend/foundation/pages"
	ftables "github.com/hansjlachmann/openerp/backend/foundation/tables"
	gtables "github.com/hansjlachmann/openerp/backend/generated/tables"
)

// CompanyInitializer can create company-scoped tables for a newly inserted company.
type CompanyInitializer interface {
	InitializeCompanyTablesWithDBType(db *sql.DB, companyName string, dbType database.DBType) error
}

// TablesHandler handles table-related API requests
type TablesHandler struct {
	db          *sql.DB
	dbType      database.DBType
	companyInit CompanyInitializer
	// onCompanyChanged is called after a company was renamed (newName set) or deleted
	// (newName ""), so sessions working in it can follow
	onCompanyChanged func(c *fiber.Ctx, oldName, newName string)
}

// OnCompanyChanged sets the function called after a company was renamed or deleted.
func (h *TablesHandler) OnCompanyChanged(fn func(c *fiber.Ctx, oldName, newName string)) {
	h.onCompanyChanged = fn
}

// NewTablesHandler creates a new tables handler (defaults to SQLite)
func NewTablesHandler(db *sql.DB) *TablesHandler {
	return NewTablesHandlerWithDBType(db, database.DBTypeSQLite)
}

// NewTablesHandlerWithDBType creates a new tables handler with explicit database type
func NewTablesHandlerWithDBType(db *sql.DB, dbType database.DBType) *TablesHandler {
	return &TablesHandler{db: db, dbType: dbType}
}

// NewTablesHandlerFull creates a tables handler with company initializer support
func NewTablesHandlerFull(db *sql.DB, dbType database.DBType, companyInit CompanyInitializer) *TablesHandler {
	return &TablesHandler{db: db, dbType: dbType, companyInit: companyInit}
}

// getTable creates a table instance by name using the registry
func (h *TablesHandler) getTable(tableName, company string) (ftables.Table, error) {
	return h.getTableOn(h.db, tableName, company)
}

// getTableOn is getTable on another executor (a transaction)
func (h *TablesHandler) getTableOn(exec database.Executor, tableName, company string) (ftables.Table, error) {
	factory, ok := gtables.GetTableFactory(tableName)
	if !ok {
		return nil, apperrors.TableNotFound(tableName)
	}
	table := factory()
	table.InitWithDBType(exec, company, h.dbType)
	return table, nil
}

// ensureSetupRecord makes sure a setup table's single blank-primary-key record
// exists, creating it if absent (the setup record is always there).
func (h *TablesHandler) ensureSetupRecord(tableName, company string) {
	probe, err := h.getTable(tableName, company)
	if err != nil || !probe.IsSetupTable() {
		return
	}
	if probe.Get("") {
		return // already exists
	}
	// Insert the single blank-primary-key record (fresh instance, no triggers).
	blank, err := h.getTable(tableName, company)
	if err != nil {
		return
	}
	_ = blank.Insert(false)
}

// parseRecordKey parses a URL record ID into the appropriate type for table.Get()
// For single PK tables: returns the string as-is
// For composite PK tables: splits comma-separated values and returns map[string]interface{}
// fullPrimaryKey returns the current record's complete primary key in the form Get expects
// (a string for a single-field key, a field->value map for a composite key — the same form
// parseRecordKey builds from a URL id), plus a comma-joined display value. GetPrimaryKeyValue
// only returns the first key field, so it cannot identify a composite-key record.
func fullPrimaryKey(table ftables.Table) (interface{}, string) {
	values := table.ToMap()
	var pkFields []string
	for _, f := range table.GetFields() {
		if f.PrimaryKey {
			pkFields = append(pkFields, f.Name)
		}
	}
	if len(pkFields) <= 1 {
		return table.GetPrimaryKeyValue(), table.GetPrimaryKeyValue()
	}
	pkMap := make(map[string]interface{}, len(pkFields))
	parts := make([]string, len(pkFields))
	for i, field := range pkFields {
		parts[i] = fmt.Sprint(values[field])
		pkMap[field] = parts[i]
	}
	return pkMap, strings.Join(parts, ",")
}

func parseRecordKey(id string, table ftables.Table) interface{} {
	// URL-decode the id (Fiber does not auto-decode route params)
	if decoded, err := url.PathUnescape(id); err == nil {
		id = decoded
	}

	fields := table.GetFields()
	var pkFields []string
	for _, f := range fields {
		if f.PrimaryKey {
			pkFields = append(pkFields, f.Name)
		}
	}

	if len(pkFields) <= 1 {
		return id
	}

	// Composite PK: split by comma and build map
	parts := strings.Split(id, ",")
	if len(parts) != len(pkFields) {
		return id // Mismatch — fall back to string
	}

	pkMap := make(map[string]interface{})
	for i, field := range pkFields {
		pkMap[field] = parts[i]
	}
	return pkMap
}

// LookupData represents structured lookup data for a field
type LookupData struct {
	Columns       []LookupColumn           `json:"columns,omitempty"`        // Column definitions (advanced mode)
	Rows          []map[string]interface{} `json:"rows"`                     // Row data
	Simple        map[string]string        `json:"simple,omitempty"`         // Simple key->display map (simple mode)
	SearchTimeout int                      `json:"search_timeout,omitempty"` // Auto-clear search timeout in ms (0 = default 1500)
	// LazyURL is set when the related table is too large to send along: the dropdown
	// loads its rows on demand from this URL (GET …/lookup/:field?search=), Rows and
	// Simple stay empty
	LazyURL string `json:"lazy_url,omitempty"`
	// FreeText: the relation is the lookup only (validate: false) — typed text is kept as it is
	FreeText bool `json:"free_text,omitempty"`
	// NewPageID: the related table's list page — the lookup's "New" opens it with a new record;
	// none for a read-only page or a system/posted table (no New in the dropdown)
	NewPageID int `json:"new_page_id,omitempty"`
	// KeyField: the related table's field the relation matches (the rows' _key) — a record
	// picked from the New page gives its value
	KeyField string `json:"key_field,omitempty"`
	// Variants (a conditional relation): the lookup per value of VariantField (the record's
	// type field, e.g. bal_account_type) — the page picks it by the record; types without a
	// variant have no lookup
	Variants     map[string]*LookupData `json:"variants,omitempty"`
	VariantField string                 `json:"variant_field,omitempty"`
	// Filters (a relation with WHERE filters): the rows matching the record — row[column]
	// equals the record's field (or const)
	Filters []LookupFilter `json:"filters,omitempty"`
	Total   int            `json:"total,omitempty"` // rows in the related table (lazy lookups)
}

// LookupFilter is a relation filter the page applies to the lookup rows
type LookupFilter struct {
	Column string `json:"column"`
	Field  string `json:"field,omitempty"`
	Const  string `json:"const,omitempty"`
}

// lookupInlineLimit: related tables with at most this many rows are sent along with every
// list/card response (instant dropdowns); larger ones are loaded on demand. Sending all
// 10,000 customers with each Customer Ledger Entries window cost ~0.4 s per request.
const lookupInlineLimit = 200

// lookupPageSize is how many rows an on-demand dropdown loads per search
const lookupPageSize = 50

// LookupColumn represents a column in the lookup dropdown
type LookupColumn struct {
	Source string `json:"source"`
	Width  int    `json:"width"`
}

// getLookupValues fetches lookup values for table relation fields
func (h *TablesHandler) getLookupValues(tableName string, table ftables.Table, company string) map[string]*LookupData {
	lookups := make(map[string]*LookupData)

	for fieldName, relInfo := range table.GetTableRelationFields() {
		if len(relInfo.Conditions) == 0 {
			if lookup := h.buildLookup(tableName, fieldName, relInfo, company, ""); lookup != nil {
				lookups[fieldName] = lookup
			}
			continue
		}
		// A conditional relation: one lookup per condition, picked by the record's field
		lookup := &LookupData{VariantField: relInfo.Conditions[0].Field, Variants: map[string]*LookupData{}}
		for _, cond := range relInfo.Conditions {
			variant := strconv.Itoa(cond.Value)
			if v := h.buildLookup(tableName, fieldName, cond.Relation, company, variant); v != nil {
				lookup.Variants[variant] = v
			}
		}
		lookups[fieldName] = lookup
	}

	return lookups
}

// buildLookup is the lookup of one relation (of a conditional relation: one variant, which
// the on-demand URL names); nil when the related table is not registered
func (h *TablesHandler) buildLookup(tableName, fieldName string, relInfo ftables.TableRelationInfo, company, variant string) *LookupData {
	relFactory, ok := gtables.GetTableFactory(relInfo.Table)
	if !ok {
		return nil
	}
	relTable := relFactory()
	relTable.InitWithDBType(h.db, company, h.dbType)

	lookup := &LookupData{
		Rows:          []map[string]interface{}{},
		Simple:        make(map[string]string),
		SearchTimeout: relInfo.SearchTimeout,
		FreeText:      relInfo.NoValidate,
		NewPageID:     pages.GetRegistry().NewRecordPage(relInfo.Table),
		KeyField:      relInfo.Field,
	}
	for _, f := range relInfo.Filters {
		lookup.Filters = append(lookup.Filters, LookupFilter{Column: f.Column, Field: f.Field, Const: f.Const})
	}

	// Add column definitions if lookup_columns is specified (advanced mode)
	for _, col := range relInfo.LookupColumns {
		lookup.Columns = append(lookup.Columns, LookupColumn{Source: col.Source, Width: col.Width})
	}

	// Large related table: no rows, the dropdown loads them on demand
	if total := relTable.Count(); total > lookupInlineLimit {
		if len(lookup.Columns) == 0 {
			lookup.Columns = []LookupColumn{{Source: relInfo.Field, Width: 150}}
		}
		lookup.Rows = nil
		lookup.Simple = nil
		lookup.LazyURL = fmt.Sprintf("/api/tables/%s/lookup/%s", url.PathEscape(tableName), url.PathEscape(fieldName))
		if variant != "" {
			lookup.LazyURL += "?variant=" + url.QueryEscape(variant)
		}
		lookup.Total = total
		return lookup
	}

	// Fetch all records from related table
	if relTable.FindSet() {
		for {
			keyValue := relTable.GetPrimaryKeyValue()
			recordMap := relTable.ToMap()

			// For advanced mode: add full row data
			if len(relInfo.LookupColumns) > 0 {
				row := make(map[string]interface{})
				row["_key"] = keyValue // Always include the key
				for _, col := range relInfo.LookupColumns {
					if val, ok := recordMap[col.Source]; ok {
						row[col.Source] = val
					}
				}
				for _, f := range relInfo.Filters {
					row[f.Column] = recordMap[f.Column] // the page shows the rows matching the record
				}
				if len(relInfo.Filters) > 0 {
					row["_key"] = fmt.Sprint(recordMap[relInfo.Field]) // the related field, not the composite key
				}
				lookup.Rows = append(lookup.Rows, row)
			}

			// For simple mode: add key->display mapping
			displayValue := keyValue
			if relInfo.DisplayField != "" {
				if dv, ok := recordMap[relInfo.DisplayField]; ok && dv != nil {
					displayValue = fmt.Sprintf("%v", dv)
				}
			}
			lookup.Simple[keyValue] = displayValue

			if !relTable.Next() {
				break
			}
		}
	}
	return lookup
}

// LookupRows returns rows for an on-demand dropdown (lookups with lazy_url): rows of the
// table related to :field whose key or lookup columns contain search (case-insensitive),
// ordered by key, at most lookupPageSize. key=<value> returns the row with that key, if any.
// GET /api/tables/:table/lookup/:field?search=&offset=&key=
func (h *TablesHandler) LookupRows(c *fiber.Ctx) error {
	tableName := c.Params("table")
	fieldName := c.Params("field")
	sess := getSession(c)
	if sess == nil {
		return c.Status(fiber.StatusUnauthorized).JSON(apitypes.NewErrorResponse(apperrors.NoActiveSession().Message("en-US")))
	}
	company := sess.GetCompany()
	language := sess.GetLanguage()

	table, err := h.getTable(tableName, company)
	if err != nil {
		return c.Status(404).JSON(apitypes.NewErrorResponse(apperrors.TableNotFound(tableName).Message(language)))
	}
	relInfo, ok := table.GetTableRelationFields()[fieldName]
	if !ok {
		return c.Status(404).JSON(apitypes.NewErrorResponse(apperrors.InvalidFields().Message(language)))
	}
	if len(relInfo.Conditions) > 0 {
		// A conditional relation: the variant (the type's value) names the related table
		record := map[string]interface{}{relInfo.Conditions[0].Field: c.Query("variant", "")}
		if relInfo, ok = relInfo.For(record); !ok {
			return c.Status(404).JSON(apitypes.NewErrorResponse(apperrors.InvalidFields().Message(language)))
		}
	}
	relFactory, ok := gtables.GetTableFactory(relInfo.Table)
	if !ok {
		return c.Status(404).JSON(apitypes.NewErrorResponse(apperrors.TableNotFound(relInfo.Table).Message(language)))
	}
	relTable := relFactory()
	relTable.InitWithDBType(h.db, company, h.dbType)

	// Columns shown in the dropdown (stored columns only), the key first
	columns := []string{relInfo.Field}
	for _, col := range relInfo.LookupColumns {
		if col.Source != relInfo.Field && ftables.IsQueryableColumn(relTable, col.Source) {
			columns = append(columns, col.Source)
		}
	}

	// The relation's constant filters (WHERE Account Type=CONST(Posting)): only those rows
	for _, f := range relInfo.Filters {
		if f.Field == "" {
			relTable.SetFilter(f.Column, f.Const)
		}
	}
	if key := c.Query("key", ""); key != "" {
		relTable.SetFilter(relInfo.Field, strings.ToUpper(key))
	} else if search := strings.TrimSpace(c.Query("search", "")); search != "" {
		relTable.SetSearch(columns, search)
	}
	relTable.SetCurrentKey(relInfo.Field)
	total := relTable.Count()
	relTable.SetPage(lookupPageSize, max(c.QueryInt("offset", 0), 0))

	rows := make([]map[string]interface{}, 0)
	if relTable.FindSet() {
		for {
			record := relTable.ToMap()
			row := map[string]interface{}{"_key": fmt.Sprint(record[relInfo.Field])}
			for _, col := range columns {
				row[col] = record[col]
			}
			rows = append(rows, row)
			if !relTable.Next() {
				break
			}
		}
	}
	return c.JSON(apitypes.NewSuccessResponse(map[string]interface{}{"rows": rows, "total": total}))
}

// getLookupValuesAsInterface converts lookup data to interface{} map for JSON serialization
func (h *TablesHandler) getLookupValuesAsInterface(tableName string, table ftables.Table, company string) map[string]interface{} {
	lookups := h.getLookupValues(tableName, table, company)
	result := make(map[string]interface{})
	for k, v := range lookups {
		result[k] = v
	}
	return result
}

// optionCaptions returns the option values of the table's Option fields in the user's
// language: field → option index ("0", "1", …) → caption, from tables.<table>.options.
// <field>.<value in snake case> ("On Hold" → on_hold); the YAML value when not translated.
// Records carry the index, so only the displayed text changes with the language.
func optionCaptions(tableName string, table ftables.Table, language string) map[string]map[string]string {
	ts := i18n.GetInstance()
	lang := normalizeLanguageCode(language)
	options := make(map[string]map[string]string)
	for fieldName, values := range table.GetOptionFields() {
		optionMap := make(map[string]string, len(values))
		keys := optionKeys(values)
		for i, value := range values {
			caption := value
			key := keys[i]
			if key != "" {
				if translated := ts.OptionCaption(tableName, fieldName, key, lang); !strings.HasPrefix(translated, "tables.") {
					caption = translated
				}
			}
			optionMap[strconv.Itoa(i)] = caption
		}
		options[fieldName] = optionMap
	}
	return options
}

// GetOptions returns only the option field metadata (no records)
// GET /api/tables/:table/options
func (h *TablesHandler) GetOptions(c *fiber.Ctx) error {
	tableName := c.Params("table")
	sess := getSession(c)

	if sess == nil {
		return c.Status(fiber.StatusUnauthorized).JSON(apitypes.NewErrorResponse(apperrors.NoActiveSession().Message("en-US")))
	}

	company := sess.GetCompany()
	language := sess.GetLanguage()

	// Create table instance (doesn't fetch any data)
	table, err := h.getTable(tableName, company)
	if err != nil {
		return c.Status(404).JSON(apitypes.NewErrorResponse(apperrors.TableNotFound(tableName).Message(language)))
	}

	options := optionCaptions(tableName, table, language)

	// Build lookups map for table relation fields
	lookups := h.getLookupValues(tableName, table, company)

	response := apitypes.NewSuccessResponse(map[string]interface{}{
		"options": options,
		"lookups": lookups,
	})
	return c.JSON(response)
}

// GetRecordIDs returns only the IDs from a table (lightweight for navigation)
// GET /api/tables/:table/ids
func (h *TablesHandler) GetRecordIDs(c *fiber.Ctx) error {
	tableName := c.Params("table")
	sess := getSession(c)

	if sess == nil {
		return c.Status(fiber.StatusUnauthorized).JSON(apitypes.NewErrorResponse(apperrors.NoActiveSession().Message("en-US")))
	}

	company := sess.GetCompany()
	language := sess.GetLanguage()

	// Create table instance
	table, err := h.getTable(tableName, company)
	if err != nil {
		return c.Status(404).JSON(apitypes.NewErrorResponse(apperrors.TableNotFound(tableName).Message(language)))
	}

	// A page on a temporary source table: its rows (Navigate)
	if err := h.fillTemporaryPage(c, table, tableName, sess); err != nil {
		return c.Status(500).JSON(apitypes.NewErrorResponse(errorMessage(err, language)))
	}
	// A list opened from a posting preview: the previewed entries
	if err := h.fillPreview(c, table, tableName, sess); err != nil {
		return c.Status(500).JSON(apitypes.NewErrorResponse(errorMessage(err, language)))
	}

	// The list's filters, search and sort: card navigation (First/Previous/Next/Last) moves
	// through the records in the order and selection the list shows
	if err := applyFlowFilters(c, table); err != nil {
		return c.Status(400).JSON(apitypes.NewErrorResponse(err.Error()))
	}
	if err := applyListQuery(c, table, language); err != nil {
		return c.Status(400).JSON(apitypes.NewErrorResponse(err.Error()))
	}

	// Collect IDs (the full key, comma-separated for composite keys, as the frontend's getRecordId)
	ids := make([]string, 0)
	if table.FindSet() {
		for {
			_, id := fullPrimaryKey(table)
			ids = append(ids, id)
			if !table.Next() {
				break
			}
		}
	}

	response := apitypes.NewSuccessResponse(map[string]interface{}{
		"ids": ids,
	})
	return c.JSON(response)
}

// flowFieldFilterer is implemented by generated tables with Sum/Count FlowFields: lists
// can filter on them (the filter applies to a subquery computing the value)
type flowFieldFilterer interface {
	FlowFieldFilterKind(field string) (flowfilter.Kind, bool)
}

// applyListQuery applies the list query parameters shared by /list, /ids and
// /delete-many: sort_by and sort_order, filters (JSON [{field, expression}], filter
// expressions) and search with search_fields (case-insensitive "contains"). Field names end
// up in SQL text: only real, non-sensitive columns of the table are accepted.
func applyListQuery(c *fiber.Ctx, table ftables.Table, language string) error {
	if sortBy := c.Query("sort_by", ""); sortBy != "" {
		// A stored column, or a Sum/Count FlowField (sorted on its calculated value)
		sortable := ftables.IsQueryableColumn(table, sortBy)
		if ff, ok := table.(flowFieldFilterer); ok && !sortable {
			_, sortable = ff.FlowFieldFilterKind(sortBy)
		}
		if !sortable {
			return errors.New(apperrors.InvalidSortField().Message(language))
		}
		table.SetCurrentKey(sortBy)
	}
	switch c.Query("sort_order", "asc") {
	case "asc":
	case "desc":
		table.SetAscending(false)
	default:
		return errors.New(apperrors.InvalidSortField().Message(language))
	}

	// fixed_filters: [{"field","value"}] — a subpage's link to its card's record: exact values
	// in filter group 2, so the user's filters (group 0) narrow them and cannot replace them
	if fixedParam := c.Query("fixed_filters", ""); fixedParam != "" {
		var fixed []fixedFilterParam
		if err := json.Unmarshal([]byte(fixedParam), &fixed); err != nil {
			return errors.New(apperrors.InvalidFilters().Message(language))
		}
		if err := applyFixedFilters(table, fixed, language); err != nil {
			return err
		}
	}

	if filtersParam := c.Query("filters", ""); filtersParam != "" {
		var apiFilters []filterParam
		if err := json.Unmarshal([]byte(filtersParam), &apiFilters); err != nil {
			return errors.New(apperrors.InvalidFilters().Message(language))
		}
		if err := applyFieldFilters(table, apiFilters, c.Params("table"), language); err != nil {
			return err
		}
	}

	if search := strings.TrimSpace(c.Query("search", "")); search != "" {
		var searchFields []string
		if err := json.Unmarshal([]byte(c.Query("search_fields", "[]")), &searchFields); err != nil {
			return errors.New(apperrors.InvalidFilters().Message(language))
		}
		for _, f := range searchFields {
			if !ftables.IsQueryableColumn(table, f) {
				return errors.New(apperrors.InvalidFilters().Message(language))
			}
		}
		table.SetSearch(searchFields, search)
	}
	return nil
}

// fixedFilterParam is an exact-value filter of filter group 2 (a subpage's link, a journal's
// template and batch); filterParam a user filter (group 0) with a filter expression
type fixedFilterParam struct {
	Field string      `json:"field"`
	Value interface{} `json:"value"`
	// Expression: Value is a filter expression (a page's OnOpenPage range), applied as such
	Expression bool `json:"expression,omitempty"`
}

type filterParam struct {
	Field      string `json:"field"`
	Expression string `json:"expression"`
}

// applyFixedFilters puts exact-value filters on stored fields into filter group 2
func applyFixedFilters(table ftables.Table, fixed []fixedFilterParam, language string) error {
	if len(fixed) == 0 {
		return nil
	}
	grouped, ok := table.(interface{ FilterGroup(group ...int) int })
	if !ok {
		return errors.New(apperrors.InvalidFilters().Message(language))
	}
	ranger, ok := table.(interface {
		SetRange(fieldName string, values ...interface{})
	})
	if !ok {
		return errors.New(apperrors.InvalidFilters().Message(language))
	}
	grouped.FilterGroup(2)
	defer grouped.FilterGroup(0)
	for _, f := range fixed {
		if !ftables.IsQueryableColumn(table, f.Field) {
			return errors.New(apperrors.InvalidFilters().Message(language))
		}
		value := ""
		expression := f.Expression
		switch v := f.Value.(type) {
		case string:
			value = v
		case float64:
			value = strconv.FormatFloat(v, 'f', -1, 64)
		case bool:
			// a Yes/No filter: the filter expression reads true/false (SetRange would compare text)
			value, expression = strconv.FormatBool(v), true
		case nil:
		default:
			value = fmt.Sprint(v)
		}
		if expression {
			if err := ftables.ApplyFilter(table, ftables.FieldFilter{Field: f.Field, Expression: value}); err != nil {
				return errors.New(apperrors.InvalidFilters().Message(language))
			}
			continue
		}
		ranger.SetRange(f.Field, value)
	}
	return nil
}

// applyFieldFilters applies user filters (filter group 0): stored fields, and Sum/Count
// FlowFields on their calculated value (e.g. Balance)
func applyFieldFilters(table ftables.Table, filters []filterParam, tableName, language string) error {
	for _, f := range filters {
		switch err := ftables.ApplyFilter(table, ftables.FieldFilter{Field: f.Field, Expression: f.Expression}); {
		case errors.Is(err, ftables.ErrFilterExpression):
			return errors.New(apperrors.FilterInvalid(tableName, f.Field, f.Expression).Message(language))
		case err != nil:
			return errors.New(apperrors.InvalidFilters().Message(language))
		}
	}
	return nil
}

// ListRecords returns a list of records from a table
// GET /api/tables/:table/list
func (h *TablesHandler) ListRecords(c *fiber.Ctx) error {
	tableName := c.Params("table")
	sess := getSession(c)

	if sess == nil {
		return c.Status(fiber.StatusUnauthorized).JSON(apitypes.NewErrorResponse(apperrors.NoActiveSession().Message("en-US")))
	}

	company := sess.GetCompany()
	language := sess.GetLanguage()

	// Create table instance
	table, err := h.getTable(tableName, company)
	if err != nil {
		return c.Status(404).JSON(apitypes.NewErrorResponse(apperrors.TableNotFound(tableName).Message(language)))
	}

	// A page on a temporary source table: its rows (Navigate)
	if err := h.fillTemporaryPage(c, table, tableName, sess); err != nil {
		return c.Status(500).JSON(apitypes.NewErrorResponse(errorMessage(err, language)))
	}
	// A list opened from a posting preview: the previewed entries
	if err := h.fillPreview(c, table, tableName, sess); err != nil {
		return c.Status(500).JSON(apitypes.NewErrorResponse(errorMessage(err, language)))
	}

	// Sort, filters and search
	if err := applyListQuery(c, table, language); err != nil {
		return c.Status(400).JSON(apitypes.NewErrorResponse(err.Error()))
	}

	// Setup tables always have their single record present
	if table.IsSetupTable() {
		h.ensureSetupRecord(tableName, company)
	}

	// FlowFilters (e.g. Date Filter) the FlowFields apply
	if err := applyFlowFilters(c, table); err != nil {
		return c.Status(400).JSON(apitypes.NewErrorResponse(err.Error()))
	}

	// Parse fields parameter (JSON array of field names) - for future FlowField optimization
	var requestedFields []string
	fieldsParam := c.Query("fields", "")
	if fieldsParam != "" {
		if err := json.Unmarshal([]byte(fieldsParam), &requestedFields); err != nil {
			return c.Status(400).JSON(apitypes.NewErrorResponse(apperrors.InvalidFields().Message(language)))
		}
	}

	// Pagination window (opt-in): offset/limit (list page windows) or page/page_size.
	// Then total is the full filtered count. Without either the entire result set is
	// returned (small tables, callers that need every row).
	const maxPageSize = 1000
	offset := max(c.QueryInt("offset", 0), 0)
	limit := min(max(c.QueryInt("limit", 0), 0), maxPageSize)
	page := c.QueryInt("page", 1)
	if page < 1 {
		page = 1
	}
	pageSize := min(max(c.QueryInt("page_size", 0), 0), maxPageSize)
	if limit == 0 && pageSize > 0 {
		limit, offset = pageSize, (page-1)*pageSize
	}

	total := 0
	if limit > 0 {
		total = table.Count() // full filtered count, before applying the window
		table.SetPage(limit, offset)
	}

	// Collect records (non-nil so an empty result serializes as [] rather than null,
	// e.g. when a requested page is past the end of the data)
	records := make([]map[string]interface{}, 0)
	if table.FindSet() {
		records = append(records, ftables.PublicMap(table))
		for table.Next() {
			records = append(records, ftables.PublicMap(table))
		}
	}

	// FlowFields for all rows at once (one grouped query per FlowField, not one query
	// per row and FlowField); only those requested when the client names its fields
	if calc := flowFieldsToCalc(table.GetFlowFields(), requestedFields); len(calc) > 0 {
		table.CalcFieldsForRecords(records, calc...)
	}

	// The calculated fields of the page showing the list (page_id: its OnAfterGetRecord triggers;
	// page_params: the page's parameters, which its triggers read with system.PageParam)
	if pageID := c.QueryInt("page_id", 0); pageID > 0 {
		var pageParams map[string]string
		if raw := c.Query("page_params", ""); raw != "" {
			_ = json.Unmarshal([]byte(raw), &pageParams)
		}
		fcodeunits.SetCurrentPageParams(pageParams)
		if err := calculatedValues(pageID, tableName, records, func() ftables.Table {
			rec, _ := h.getTable(tableName, company)
			return rec
		}); err != nil {
			return c.Status(500).JSON(apitypes.NewErrorResponse(errorMessage(err, language)))
		}
	}

	// Get captions
	ts := i18n.GetInstance()
	captions := &apitypes.CaptionData{
		Table:      ts.TableCaption(tableName, language),
		Fields:     make(map[string]string),
		FieldTypes: make(map[string]string),
		Options:    make(map[string]map[string]string),
		Lookups:    h.getLookupValuesAsInterface(tableName, table, company),
	}

	// Add field captions and types from metadata
	for _, field := range table.GetFields() {
		if field.Sensitive {
			continue
		}
		captions.Fields[field.Name] = ts.FieldCaption(tableName, field.Name, language)
		captions.FieldTypes[field.Name] = string(field.Type)
	}

	// Option field values in the user's language
	captions.Options = optionCaptions(tableName, table, language)

	// Pagination metadata: real values when a window was requested, otherwise the
	// legacy single-page shape (all records reported as page 1).
	respPage, respPageSize, respTotal, respOffset := 1, len(records), len(records), 0
	if limit > 0 {
		respPageSize, respTotal, respOffset = limit, total, offset
		if pageSize > 0 {
			respPage = page
		}
	}

	response := apitypes.NewSuccessResponseWithCaptions(map[string]interface{}{
		"records":   records,
		"total":     respTotal,
		"page":      respPage,
		"page_size": respPageSize,
		"offset":    respOffset,
	}, captions)
	return c.JSON(response)
}

// GetRecord returns a single record by ID
// GET /api/tables/:table/card/:id
func (h *TablesHandler) GetRecord(c *fiber.Ctx) error {
	tableName := c.Params("table")
	id := c.Params("id")
	sess := getSession(c)

	if sess == nil {
		return c.Status(fiber.StatusUnauthorized).JSON(apitypes.NewErrorResponse(apperrors.NoActiveSession().Message("en-US")))
	}

	company := sess.GetCompany()
	language := sess.GetLanguage()
	tableCaption := i18n.GetInstance().TableCaption(tableName, language)

	// Create table instance
	table, err := h.getTable(tableName, company)
	if err != nil {
		return c.Status(404).JSON(apitypes.NewErrorResponse(apperrors.TableNotFound(tableName).Message(language)))
	}

	// A setup table's single record is always there (its card opens on it)
	if id == "" && table.IsSetupTable() {
		h.ensureSetupRecord(tableName, company)
	}

	// Get record by primary key (supports composite keys via comma-separated values)
	if !table.Get(parseRecordKey(id, table)) {
		return c.Status(404).JSON(apitypes.NewErrorResponse(apperrors.RecordNotFound(tableCaption, id).Message(language)))
	}

	// FlowFilters (e.g. Date Filter) the FlowFields apply
	if err := applyFlowFilters(c, table); err != nil {
		return c.Status(400).JSON(apitypes.NewErrorResponse(err.Error()))
	}

	// Calculate FlowFields
	table.CalcFields(table.GetFlowFields()...)

	// Get captions
	ts := i18n.GetInstance()
	captions := &apitypes.CaptionData{
		Table:      ts.TableCaption(tableName, language),
		Fields:     make(map[string]string),
		FieldTypes: make(map[string]string),
		Options:    make(map[string]map[string]string),
		Lookups:    h.getLookupValuesAsInterface(tableName, table, company),
	}

	for _, field := range table.GetFields() {
		if field.Sensitive {
			continue
		}
		captions.Fields[field.Name] = ts.FieldCaption(tableName, field.Name, language)
		captions.FieldTypes[field.Name] = string(field.Type)
	}

	// Option field values in the user's language
	captions.Options = optionCaptions(tableName, table, language)

	response := apitypes.NewSuccessResponseWithCaptions(ftables.PublicMap(table), captions)
	return c.JSON(response)
}

// InsertRecord inserts a new record
// POST /api/tables/:table/insert
func (h *TablesHandler) InsertRecord(c *fiber.Ctx) error {
	tableName := c.Params("table")
	sess := getSession(c)

	if sess == nil {
		return c.Status(fiber.StatusUnauthorized).JSON(apitypes.NewErrorResponse(apperrors.NoActiveSession().Message("en-US")))
	}

	company := sess.GetCompany()
	language := sess.GetLanguage()

	// Parse request body
	var data map[string]interface{}
	if err := c.BodyParser(&data); err != nil {
		return c.Status(400).JSON(apitypes.NewErrorResponse(apperrors.InvalidRequestBody().Message(language)))
	}

	// One transaction: what the triggers wrote (e.g. the number series' Last No. Used) is
	// undone when the insert fails
	tx, err := h.db.Begin()
	if err != nil {
		return c.Status(500).JSON(apitypes.NewErrorResponse(apperrors.InsertFailed(tableName).Message(language)))
	}
	table, status, msg := h.insertOne(tx, tableName, company, language, data, c.QueryInt("page_id", 0))
	if msg != "" {
		_ = ftables.Rollback(tx)
		return c.Status(status).JSON(apitypes.NewErrorResponse(msg))
	}
	if err := ftables.Commit(tx); err != nil {
		return c.Status(500).JSON(apitypes.NewErrorResponse(commitErrorMessage(err, apperrors.InsertFailed(tableName), language)))
	}
	if setter, ok := table.(interface{ SetDB(database.Executor) }); ok {
		setter.SetDB(h.db) // the transaction is over
	}

	// Initialize company-scoped tables when a new Company is created
	if tableName == "Company" && h.companyInit != nil {
		companyName := table.GetPrimaryKeyValue()
		if err := h.companyInit.InitializeCompanyTablesWithDBType(h.db, companyName, h.dbType); err != nil {
			fmt.Printf("Warning: Failed to initialize tables for company '%s': %v\n", companyName, err)
		}
	}

	return c.JSON(apitypes.NewSuccessResponse(h.recordWithCalculatedValues(c, table)))
}

// ModifyRecord updates an existing record
// PUT /api/tables/:table/modify/:id
func (h *TablesHandler) ModifyRecord(c *fiber.Ctx) error {
	tableName := c.Params("table")
	id := c.Params("id")
	sess := getSession(c)

	if sess == nil {
		return c.Status(fiber.StatusUnauthorized).JSON(apitypes.NewErrorResponse(apperrors.NoActiveSession().Message("en-US")))
	}

	company := sess.GetCompany()
	language := sess.GetLanguage()
	tableCaption := i18n.GetInstance().TableCaption(tableName, language)

	// Create table instance
	table, err := h.getTable(tableName, company)
	if err != nil {
		return c.Status(404).JSON(apitypes.NewErrorResponse(apperrors.TableNotFound(tableName).Message(language)))
	}

	// An empty ID is only valid for a setup table (its single record has a blank primary key)
	if id == "" && !table.IsSetupTable() {
		return c.Status(400).JSON(apitypes.NewErrorResponse(apperrors.EmptyPrimaryKey(tableCaption).Message(language)))
	}

	// Get existing record (supports composite keys via comma-separated values)
	if !table.Get(parseRecordKey(id, table)) {
		return c.Status(404).JSON(apitypes.NewErrorResponse(apperrors.RecordNotFound(tableCaption, id).Message(language)))
	}
	_, oldKey := fullPrimaryKey(table)

	// Parse request body
	var data map[string]interface{}
	if err := c.BodyParser(&data); err != nil {
		return c.Status(400).JSON(apitypes.NewErrorResponse(apperrors.InvalidRequestBody().Message(language)))
	}

	// Sensitive fields (e.g. password_hash) cannot be set through the API; a masked field
	// sent back as its placeholder keeps its stored value
	ftables.DropMaskedPlaceholders(table, data)
	ftables.DropReadOnlyFields(table, data, false) // editable: false — only business logic sets them
	dropPostedFields(tableName, data)
	if err := rejectSensitiveFields(table, data); err != nil {
		return c.Status(400).JSON(apitypes.NewErrorResponse(err.Error()))
	}

	// Validate and set each changed field (runs OnValidate triggers for table relations, etc.)
	if err := validateChangedFields(table, tableName, data); err != nil {
		return c.Status(400).JSON(apitypes.NewErrorResponse(errorMessage(err, language)))
	}

	// Special handling for User table password
	if tableName == "User" {
		if password, ok := data["password"].(string); ok && password != "" {
			if userTable, ok := table.(*gtables.User); ok {
				if err := tables.SetPassword(userTable, password); err != nil {
					return c.Status(400).JSON(apitypes.NewErrorResponse(errorMessage(err, language)))
				}
			}
		}
	}

	// A rename to a key that another record already has ("The record already exists"); a
	// posted record is never renamed (dropPostedFields keeps its key fields out)
	if newLookup, newKey := fullPrimaryKey(table); newKey != oldKey {
		if existing, err := h.getTable(tableName, company); err == nil && existing.Get(newLookup) {
			return c.Status(409).JSON(apitypes.NewErrorResponse(apperrors.DuplicateRecord(tableCaption, newKey).Message(language)))
		}
	}

	// Modify record — in a transaction, so a renamed key and the fields of other tables that
	// refer to it (Rename, done by the generated Modify) change together or not at all
	if ok, err := h.modifyInTransaction(table); !ok {
		if err != nil {
			return c.Status(500).JSON(apitypes.NewErrorResponse(err.Error()))
		}
		// A failed OnModify trigger is a business rule the user broke: show its message
		if trigErr := table.TriggerError(); trigErr != nil {
			return c.Status(400).JSON(apitypes.NewErrorResponse(errorMessage(trigErr, language)))
		}
		return c.Status(500).JSON(apitypes.NewErrorResponse(apperrors.ModifyFailed(tableCaption, id).Message(language)))
	}

	// A renamed company: sessions working in it must follow (cookie, session cache)
	if tableName == "Company" && h.onCompanyChanged != nil {
		if oldName, newName := fmt.Sprint(parseRecordKey(id, table)), table.GetPrimaryKeyValue(); oldName != newName {
			h.onCompanyChanged(c, oldName, newName)
		}
	}

	// Calculate FlowFields for response
	table.CalcFields(table.GetFlowFields()...)

	return c.JSON(apitypes.NewSuccessResponse(h.recordWithCalculatedValues(c, table)))
}

// recordWithCalculatedValues returns a saved record for the response; with page_id (a list
// saving its row) also the page's calculated fields of the row, so a footer (a journal's
// balance) follows the edit
func (h *TablesHandler) recordWithCalculatedValues(c *fiber.Ctx, table ftables.Table) map[string]interface{} {
	data := ftables.PublicMap(table)
	pageID := c.QueryInt("page_id", 0)
	if pageID <= 0 {
		return data
	}
	sess := getSession(c)
	tableName := c.Params("table")
	err := calculatedValues(pageID, tableName, []map[string]interface{}{data}, func() ftables.Table {
		rec, _ := h.getTable(tableName, sess.GetCompany())
		return rec
	})
	if err != nil {
		fmt.Printf("Warning: calculated fields of page %d: %v\n", pageID, err)
	}
	return data
}

// DeleteRecord deletes a record
// DELETE /api/tables/:table/delete/:id
func (h *TablesHandler) DeleteRecord(c *fiber.Ctx) error {
	tableName := c.Params("table")
	id := c.Params("id")
	sess := getSession(c)

	if sess == nil {
		return c.Status(fiber.StatusUnauthorized).JSON(apitypes.NewErrorResponse(apperrors.NoActiveSession().Message("en-US")))
	}

	company := sess.GetCompany()
	language := sess.GetLanguage()
	tableCaption := i18n.GetInstance().TableCaption(tableName, language)

	// Create table instance
	table, err := h.getTable(tableName, company)
	if err != nil {
		return c.Status(404).JSON(apitypes.NewErrorResponse(apperrors.TableNotFound(tableName).Message(language)))
	}

	// An empty ID is only valid for a setup table (its single record has a blank primary key)
	if id == "" && !table.IsSetupTable() {
		return c.Status(400).JSON(apitypes.NewErrorResponse(apperrors.EmptyPrimaryKey(tableCaption).Message(language)))
	}

	// Get existing record (supports composite keys via comma-separated values)
	if !table.Get(parseRecordKey(id, table)) {
		return c.Status(404).JSON(apitypes.NewErrorResponse(apperrors.RecordNotFound(tableCaption, id).Message(language)))
	}

	if msg := refusePosted(tableName, language); msg != "" {
		return c.Status(403).JSON(apitypes.NewErrorResponse(msg))
	}

	// Delete record (a Company with all its tables and SIFT objects, in one transaction)
	deleted, txErr := h.deleteInTransaction(tableName, table)
	if txErr != nil {
		return c.Status(500).JSON(apitypes.NewErrorResponse(txErr.Error()))
	}
	if !deleted {
		// A failed OnDelete trigger is a business rule the user broke: show its message
		if trigErr := table.TriggerError(); trigErr != nil {
			return c.Status(400).JSON(apitypes.NewErrorResponse(errorMessage(trigErr, language)))
		}
		return c.Status(500).JSON(apitypes.NewErrorResponse(apperrors.DeleteFailed(tableCaption, id).Message(language)))
	}

	if tableName == "Company" && h.onCompanyChanged != nil {
		h.onCompanyChanged(c, table.GetPrimaryKeyValue(), "")
	}

	response := apitypes.NewSuccessResponse(nil)
	return c.JSON(response)
}

// insertOne runs a record insert as the API does it (InsertRecord, and each record of a
// paste): masked placeholders dropped, sensitive fields refused, table relations checked,
// changed fields validated (OnValidate), key not empty and not existing, Insert(true) with
// OnInsert. exec is the database or the paste's transaction. pageID (0: none) is the page the
// record was entered on: its read-only fields take the values the page's OnNewRecord gives
// them (a journal line's Source Code), as the record had them when the user started it. On
// failure it returns the HTTP status and the message for the user; msg is "" on success.
func (h *TablesHandler) insertOne(exec database.Executor, tableName, company, language string, data map[string]interface{}, pageID int) (table ftables.Table, status int, msg string) {
	tableCaption := i18n.GetInstance().TableCaption(tableName, language)
	table, err := h.getTableOn(exec, tableName, company)
	if err != nil {
		return nil, 404, apperrors.TableNotFound(tableName).Message(language)
	}
	if msg := refusePosted(tableName, language); msg != "" {
		return nil, 403, msg
	}

	// Sensitive fields (e.g. password_hash) cannot be set through the API; a masked field
	// sent back as its placeholder keeps its stored value
	ftables.DropMaskedPlaceholders(table, data)
	ftables.DropReadOnlyFields(table, data, true) // editable: false — only business logic sets them
	if err := rejectSensitiveFields(table, data); err != nil {
		return nil, 400, err.Error()
	}

	// Read-only fields of a record entered on a page with OnNewRecord triggers: the values the
	// server's own init gives them (the payload's are dropped above)
	if pageID > 0 && pages.HasNewRecordTriggers(pageID) {
		newRecordValues, err := h.newRecordReadOnlyValues(exec, tableName, company, data, pageID)
		if err != nil {
			return nil, 400, errorMessage(err, language)
		}
		// before the user's fields are validated: their triggers may set read-only fields too
		table.UpdateFromMap(newRecordValues)
	}

	// Validate and set each changed field (runs OnValidate triggers for table relations, etc.)
	if err := validateChangedFields(table, tableName, data); err != nil {
		return nil, 400, errorMessage(err, language)
	}

	// Special handling for User table password
	if tableName == "User" {
		if password, ok := data["password"].(string); ok && password != "" {
			if userTable, ok := table.(*gtables.User); ok {
				if err := tables.SetPassword(userTable, password); err != nil {
					return nil, 400, errorMessage(err, language)
				}
			}
		}
	}

	// Check for empty primary key — unless the table's OnInsert assigns it (auto_key: the next
	// number of a series; the key's required check after OnInsert still applies). A composite
	// key may have blank fields, as classic (a posting setup for a blank business group)
	metadata := pages.GetTableMetadata()
	if table.GetPrimaryKeyValue() == "" && !metadata.IsAutoKey(tableName) && len(metadata.GetPrimaryKeyFields(tableName)) <= 1 {
		return nil, 400, apperrors.EmptyPrimaryKey(tableCaption).Message(language)
	}

	// Insert record: a taken key (also one OnInsert assigned) is refused by Insert itself
	if !table.Insert(true) {
		// A failed OnInsert trigger is a business rule the user broke, a taken key "<Table>
		// <key> already exists": show its message
		if trigErr := table.TriggerError(); trigErr != nil {
			if appErr, ok := trigErr.(*apperrors.AppError); ok && appErr.Code == apperrors.ErrDuplicateRecord {
				return nil, 409, errorMessage(trigErr, language)
			}
			return nil, 400, errorMessage(trigErr, language)
		}
		return nil, 500, apperrors.InsertFailed(tableCaption).Message(language)
	}
	return table, 200, ""
}

// maxCopyRecords caps one copy/paste on a list (Ctrl+C / Ctrl+V)
const maxCopyRecords = 1000

// copyableMap is a record as copied (list copy/paste): its stored fields without
// FlowFields, FlowFilter fields and secrets (sensitive, masked/encrypted)
func copyableMap(table ftables.Table) map[string]interface{} {
	m := table.ToMap()
	for _, f := range table.GetFields() {
		if f.Sensitive || f.Masked || f.FlowField {
			delete(m, f.Name)
		}
	}
	for _, ff := range table.GetFlowFilterFields() {
		delete(m, ff.Name)
	}
	return m
}

// selectedKeys returns the record keys of a multi-select request: the given keys, or with
// all every record the list shows (filters, flow_filters, search as /list). At most max+1
// keys are collected, so the caller can tell there are too many.
func selectedKeys(c *fiber.Ctx, table ftables.Table, language string, keys []string, all bool, max int) ([]string, error) {
	if !all {
		return keys, nil
	}
	if err := applyFlowFilters(c, table); err != nil {
		return nil, err
	}
	if err := applyListQuery(c, table, language); err != nil {
		return nil, err
	}
	keys = nil
	if table.FindSet() {
		for {
			_, id := fullPrimaryKey(table)
			keys = append(keys, id)
			if len(keys) > max || !table.Next() {
				break
			}
		}
	}
	return keys, nil
}

// CopyRecords returns the records to copy on a list (Ctrl+C, list multi-select): body
// {"keys": [...]} or {"all": true} with the list's query parameters. Stored fields only
// (copyableMap); at most maxCopyRecords.
// POST /api/tables/:table/copy
func (h *TablesHandler) CopyRecords(c *fiber.Ctx) error {
	tableName := c.Params("table")
	sess := getSession(c)
	if sess == nil {
		return c.Status(fiber.StatusUnauthorized).JSON(apitypes.NewErrorResponse(apperrors.NoActiveSession().Message("en-US")))
	}
	company := sess.GetCompany()
	language := sess.GetLanguage()
	tableCaption := i18n.GetInstance().TableCaption(tableName, language)

	var req struct {
		Keys []string `json:"keys"`
		All  bool     `json:"all"`
	}
	if err := c.BodyParser(&req); err != nil {
		return c.Status(400).JSON(apitypes.NewErrorResponse(apperrors.InvalidRequestBody().Message(language)))
	}
	table, err := h.getTable(tableName, company)
	if err != nil {
		return c.Status(404).JSON(apitypes.NewErrorResponse(apperrors.TableNotFound(tableName).Message(language)))
	}
	keys, err := selectedKeys(c, table, language, req.Keys, req.All, maxCopyRecords)
	if err != nil {
		return c.Status(400).JSON(apitypes.NewErrorResponse(err.Error()))
	}
	if len(keys) > maxCopyRecords {
		return c.Status(400).JSON(apitypes.NewErrorResponse(apperrors.CopyTooMany(maxCopyRecords).Message(language)))
	}
	records := make([]map[string]interface{}, 0, len(keys))
	for _, id := range keys {
		record, _ := h.getTable(tableName, company)
		if !record.Get(parseRecordKey(id, record)) {
			return c.Status(404).JSON(apitypes.NewErrorResponse(apperrors.RecordNotFound(tableCaption, id).Message(language)))
		}
		records = append(records, copyableMap(record))
	}
	return c.JSON(apitypes.NewSuccessResponse(map[string]interface{}{"records": records}))
}

// InsertRecords inserts several records at once (list paste, Ctrl+V): body {"records":
// [...]}. One transaction: each record goes through insertOne (validation, relations,
// OnInsert); the first one that fails rolls back all and is named. Returns the keys of
// the inserted records.
// POST /api/tables/:table/insert-many
func (h *TablesHandler) InsertRecords(c *fiber.Ctx) error {
	tableName := c.Params("table")
	sess := getSession(c)
	if sess == nil {
		return c.Status(fiber.StatusUnauthorized).JSON(apitypes.NewErrorResponse(apperrors.NoActiveSession().Message("en-US")))
	}
	company := sess.GetCompany()
	language := sess.GetLanguage()

	var req struct {
		Records []map[string]interface{} `json:"records"`
	}
	// A company is more than a record (its tables are created with it): never pasted
	if err := c.BodyParser(&req); err != nil || tableName == "Company" {
		return c.Status(400).JSON(apitypes.NewErrorResponse(apperrors.InvalidRequestBody().Message(language)))
	}
	if _, err := h.getTable(tableName, company); err != nil {
		return c.Status(404).JSON(apitypes.NewErrorResponse(apperrors.TableNotFound(tableName).Message(language)))
	}
	if len(req.Records) > maxCopyRecords {
		return c.Status(400).JSON(apitypes.NewErrorResponse(apperrors.CopyTooMany(maxCopyRecords).Message(language)))
	}
	keys := make([]string, 0, len(req.Records))
	if len(req.Records) == 0 {
		return c.JSON(apitypes.NewSuccessResponse(map[string]interface{}{"inserted": 0, "keys": keys}))
	}

	tx, err := h.db.Begin()
	if err != nil {
		return c.Status(500).JSON(apitypes.NewErrorResponse(err.Error()))
	}
	for i, data := range req.Records {
		table, status, msg := h.insertOne(tx, tableName, company, language, data, 0)
		if msg != "" {
			_ = ftables.Rollback(tx)
			return c.Status(status).JSON(apitypes.NewErrorResponse(
				apperrors.PasteRecordFailed(i+1, len(req.Records), msg).Message(language)))
		}
		_, id := fullPrimaryKey(table)
		keys = append(keys, id)
	}
	if err := ftables.Commit(tx); err != nil {
		return c.Status(500).JSON(apitypes.NewErrorResponse(errorMessage(err, language)))
	}
	return c.JSON(apitypes.NewSuccessResponse(map[string]interface{}{"inserted": len(keys), "keys": keys}))
}

// maxDeleteMany caps one bulk delete: it runs in one transaction, with each record's
// OnDelete trigger.
const maxDeleteMany = 10000

// DeleteRecords deletes several records at once (list multi-select: delete the selected
// lines). Body {"keys": [...]} with record IDs as in /delete/:id, or {"all": true} for every
// record the list shows, with the list's query parameters (filters, flow_filters, search,
// search_fields). One transaction: each record's OnDelete trigger runs, and the first record
// that cannot be deleted rolls back everything; the error names it.
// DELETE /api/tables/:table/delete-many
func (h *TablesHandler) DeleteRecords(c *fiber.Ctx) error {
	tableName := c.Params("table")
	sess := getSession(c)
	if sess == nil {
		return c.Status(fiber.StatusUnauthorized).JSON(apitypes.NewErrorResponse(apperrors.NoActiveSession().Message("en-US")))
	}
	sessCompany := sess.GetCompany()
	language := sess.GetLanguage()
	tableCaption := i18n.GetInstance().TableCaption(tableName, language)

	var req struct {
		Keys []string `json:"keys"`
		All  bool     `json:"all"`
	}
	if err := c.BodyParser(&req); err != nil {
		return c.Status(400).JSON(apitypes.NewErrorResponse(apperrors.InvalidRequestBody().Message(language)))
	}
	table, err := h.getTable(tableName, sessCompany)
	if err != nil {
		return c.Status(404).JSON(apitypes.NewErrorResponse(apperrors.TableNotFound(tableName).Message(language)))
	}
	if msg := refusePosted(tableName, language); msg != "" {
		return c.Status(403).JSON(apitypes.NewErrorResponse(msg))
	}

	keys := req.Keys
	if req.All {
		// Every record the list shows: the same filters and search as /list
		if err := applyFlowFilters(c, table); err != nil {
			return c.Status(400).JSON(apitypes.NewErrorResponse(err.Error()))
		}
		if err := applyListQuery(c, table, language); err != nil {
			return c.Status(400).JSON(apitypes.NewErrorResponse(err.Error()))
		}
		keys = nil
		if table.FindSet() {
			for {
				_, id := fullPrimaryKey(table)
				keys = append(keys, id)
				if len(keys) > maxDeleteMany || !table.Next() {
					break
				}
			}
		}
	}
	if len(keys) > maxDeleteMany {
		return c.Status(400).JSON(apitypes.NewErrorResponse(apperrors.DeleteTooMany(maxDeleteMany).Message(language)))
	}
	if len(keys) == 0 {
		return c.JSON(apitypes.NewSuccessResponse(map[string]interface{}{"deleted": 0}))
	}

	tx, err := h.db.Begin()
	if err != nil {
		return c.Status(500).JSON(apitypes.NewErrorResponse(err.Error()))
	}
	var companies []string
	for _, id := range keys {
		record, err := h.getTable(tableName, sessCompany)
		if err != nil {
			_ = ftables.Rollback(tx)
			return c.Status(500).JSON(apitypes.NewErrorResponse(err.Error()))
		}
		setter, ok := record.(interface{ SetDB(database.Executor) })
		if !ok {
			_ = ftables.Rollback(tx)
			return c.Status(500).JSON(apitypes.NewErrorResponse(apperrors.DeleteFailed(tableCaption, id).Message(language)))
		}
		setter.SetDB(tx)
		if (id == "" && !record.IsSetupTable()) || !record.Get(parseRecordKey(id, record)) {
			_ = ftables.Rollback(tx)
			return c.Status(404).JSON(apitypes.NewErrorResponse(apperrors.RecordNotFound(tableCaption, id).Message(language)))
		}
		if !record.Delete(true) {
			_ = ftables.Rollback(tx)
			// A failed OnDelete trigger is a business rule: name the record and the reason
			if trigErr := record.TriggerError(); trigErr != nil {
				return c.Status(400).JSON(apitypes.NewErrorResponse(
					apperrors.DeleteRecordFailed(tableCaption, id, errorMessage(trigErr, language)).Message(language)))
			}
			return c.Status(500).JSON(apitypes.NewErrorResponse(apperrors.DeleteFailed(tableCaption, id).Message(language)))
		}
		if tableName == "Company" {
			// A company takes its tables and SIFT objects along (as DeleteRecord)
			name := record.GetPrimaryKeyValue()
			if err := company.DropObjects(tx, h.dbType, name); err != nil {
				_ = ftables.Rollback(tx)
				return c.Status(500).JSON(apitypes.NewErrorResponse(fmt.Sprintf("delete company %s: %v", name, err)))
			}
			companies = append(companies, name)
		}
	}
	if err := ftables.Commit(tx); err != nil {
		return c.Status(500).JSON(apitypes.NewErrorResponse(errorMessage(err, language)))
	}
	if h.onCompanyChanged != nil {
		for _, name := range companies {
			h.onCompanyChanged(c, name, "")
		}
	}
	return c.JSON(apitypes.NewSuccessResponse(map[string]interface{}{"deleted": len(keys)}))
}

// rejectSensitiveFields returns an error when data sets a sensitive field (YAML sensitive:
// true). Such fields are set by the server only (User.password_hash from the virtual
// "password" field). A null value (an unchanged field) is allowed.
func rejectSensitiveFields(table ftables.Table, data map[string]interface{}) error {
	for name, value := range data {
		if value != nil && ftables.IsSensitive(table, name) {
			return fmt.Errorf("field %s cannot be set", name)
		}
	}
	return nil
}

// validateChangedFields runs ValidateField (VALIDATE) for each field in data whose
// value differs from the table's current value, in table field order. Unchanged fields are
// skipped so a stale payload value cannot overwrite a sibling that another field's
// OnValidate trigger filled in, and so are values an earlier field's trigger already set (a
// new line's unit cost from its No.): only the user's changes are validated, as on a page.
// Unknown fields are still validated so they are reported.
func validateChangedFields(table ftables.Table, tableName string, data map[string]interface{}) error {
	current := table.ToMap()

	skip := make(map[string]bool)
	for _, ff := range table.GetFlowFields() {
		skip[ff] = true // FlowFields are calculated, not validated
	}
	for field := range pages.GetRegistry().CalculatedFields(tableName) {
		if !table.HasColumn(field) {
			skip[field] = true // a page's calculated field (e.g. the No. Series list's Ending No.)
		}
	}
	if tableName == "User" {
		skip["password"] = true // virtual field, handled separately by the caller
	}

	// The type field of a conditional relation goes first (Bal. Account Type before Bal.
	// Account No.): Validate checks a relation against the table of the record's type
	ordered := make([]string, 0, len(data))
	seen := make(map[string]bool, len(data))
	for _, rel := range table.GetTableRelationFields() {
		for _, cond := range rel.Conditions {
			if _, ok := data[cond.Field]; ok && !seen[cond.Field] {
				ordered = append(ordered, cond.Field)
				seen[cond.Field] = true
			}
		}
	}
	for _, f := range table.GetFields() {
		if _, ok := data[f.Name]; ok && !seen[f.Name] {
			ordered = append(ordered, f.Name)
			seen[f.Name] = true
		}
	}
	extra := make([]string, 0)
	for name := range data {
		if !seen[name] {
			extra = append(extra, name)
		}
	}
	sort.Strings(extra)
	ordered = append(ordered, extra...)

	for _, fieldName := range ordered {
		value := data[fieldName]
		// Skip nil values - frontend may send null for unchanged fields
		if value == nil || skip[fieldName] {
			continue
		}
		if cur, ok := current[fieldName]; ok && fmt.Sprint(cur) == fmt.Sprint(value) {
			continue
		}
		// a value a trigger of an earlier field already set (No. fills the unit cost) is not the
		// user's change: validating it again would run checks meant for typing it
		if now, ok := table.ToMap()[fieldName]; ok && fmt.Sprint(now) == fmt.Sprint(value) {
			continue
		}
		if err := table.ValidateField(fieldName, value); err != nil {
			return err
		}
	}
	return nil
}

// newRecordReadOnlyValues are the read-only (editable: false), non-key fields of a new record
// as the page's OnNewRecord triggers set them (InitRecord, the record's values, OnNewRecord
// without a record above): only business logic sets read-only fields, so the API takes them
// from its own init instead of the payload
func (h *TablesHandler) newRecordReadOnlyValues(exec database.Executor, tableName, company string, data map[string]interface{}, pageID int) (map[string]interface{}, error) {
	rec, err := h.getTableOn(exec, tableName, company)
	if err != nil {
		return nil, err
	}
	xRec, err := h.getTableOn(exec, tableName, company)
	if err != nil {
		return nil, err
	}
	rec.InitRecord()
	rec.UpdateFromMap(data)
	err = pages.RunNewRecord(pageID, rec, xRec, false)
	if err != nil {
		return nil, err
	}
	values := rec.ToMap()
	readOnly := map[string]interface{}{}
	for _, f := range rec.GetFields() {
		if f.ReadOnly && !f.PrimaryKey && f.Type != ftables.FieldTypeBlob {
			readOnly[f.Name] = values[f.Name]
		}
	}
	return readOnly, nil
}

// InitRecord returns a new, not yet inserted record with its defaults applied
// (OnNewRecord). Nothing is persisted.
// POST /api/tables/:table/init
func (h *TablesHandler) InitRecord(c *fiber.Ctx) error {
	tableName := c.Params("table")
	sess := getSession(c)

	if sess == nil {
		return c.Status(fiber.StatusUnauthorized).JSON(apitypes.NewErrorResponse(apperrors.NoActiveSession().Message("en-US")))
	}

	company := sess.GetCompany()
	language := sess.GetLanguage()

	table, err := h.getTable(tableName, company)
	if err != nil {
		return c.Status(404).JSON(apitypes.NewErrorResponse(apperrors.TableNotFound(tableName).Message(language)))
	}

	table.InitRecord()

	// A page's OnNewRecord triggers (optional body): the record the new one is inserted next
	// to (x_rec, below it when below_x_rec) gives values (e.g. Income/Balance of the account);
	// values are what the new record already has (the page's filters and link values)
	var req struct {
		PageID    int                    `json:"page_id"`
		XRec      map[string]interface{} `json:"x_rec"`
		BelowXRec bool                   `json:"below_x_rec"`
		Values    map[string]interface{} `json:"values"`
	}
	if len(c.Body()) > 0 {
		if err := c.BodyParser(&req); err != nil {
			return c.Status(400).JSON(apitypes.NewErrorResponse(apperrors.InvalidRequestBody().Message(language)))
		}
	}
	if req.PageID != 0 && pages.HasNewRecordTriggers(req.PageID) {
		xRec, err := h.getTable(tableName, company)
		if err != nil {
			return c.Status(404).JSON(apitypes.NewErrorResponse(apperrors.TableNotFound(tableName).Message(language)))
		}
		if req.XRec != nil {
			ftables.DropMaskedPlaceholders(xRec, req.XRec)
			xRec.FromMap(req.XRec)
		}
		if req.Values != nil {
			table.UpdateFromMap(req.Values)
		}
		if err := pages.RunNewRecord(req.PageID, table, xRec, req.BelowXRec); err != nil {
			return c.Status(400).JSON(apitypes.NewErrorResponse(errorMessage(err, language)))
		}
	}

	return c.JSON(apitypes.NewSuccessResponse(ftables.PublicMap(table)))
}

// ValidateField validates a single field value (VALIDATE).
// When the in-progress record is supplied, the table is hydrated from it first so the
// field's OnValidate trigger can see (and fill in) sibling fields; the resulting record
// is returned in data.
// POST /api/tables/:table/validate
func (h *TablesHandler) ValidateField(c *fiber.Ctx) error {
	tableName := c.Params("table")
	sess := getSession(c)

	if sess == nil {
		return c.Status(fiber.StatusUnauthorized).JSON(apitypes.NewErrorResponse(apperrors.NoActiveSession().Message("en-US")))
	}

	company := sess.GetCompany()
	language := sess.GetLanguage()

	// Parse request body
	var req struct {
		Field  string                 `json:"field"`
		Value  interface{}            `json:"value"`
		Record map[string]interface{} `json:"record"`
	}
	if err := c.BodyParser(&req); err != nil {
		return c.Status(400).JSON(apitypes.NewErrorResponse(apperrors.InvalidRequestBody().Message(language)))
	}

	// Create table instance
	table, err := h.getTable(tableName, company)
	if err != nil {
		return c.Status(404).JSON(apitypes.NewErrorResponse(apperrors.TableNotFound(tableName).Message(language)))
	}

	// Sensitive fields can neither be validated (set) nor hydrated through the API
	if ftables.IsSensitive(table, req.Field) || rejectSensitiveFields(table, req.Record) != nil {
		return c.Status(400).JSON(apitypes.NewErrorResponse(apperrors.InvalidRequestBody().Message(language)))
	}

	// Hydrate from the in-progress record (plain assignment, no triggers)
	if req.Record != nil {
		ftables.DropMaskedPlaceholders(table, req.Record)
		table.FromMap(req.Record)
	}

	// A read-only field (editable: false, or a posted table's field that is not editable: true)
	// is not changed from a page
	if ftables.IsReadOnly(table, req.Field) || !pages.GetTableMetadata().IsPostedFieldEditable(tableName, req.Field) {
		return c.Status(400).JSON(apitypes.NewErrorResponse(apperrors.FieldNotEditable(tableName, req.Field).Message(language)))
	}

	// A masked field sent back as its placeholder is unchanged: nothing to validate
	if ftables.IsMasked(table, req.Field) && req.Value == ftables.MaskedValue {
		return c.JSON(apitypes.NewSuccessResponse(ftables.PublicMap(table)))
	}

	// Validate field (runs OnValidate trigger)
	if err := table.ValidateField(req.Field, req.Value); err != nil {
		return c.JSON(apitypes.APIResponse{
			Success: false,
			Error:   errorMessage(err, language),
		})
	}

	return c.JSON(apitypes.NewSuccessResponse(ftables.PublicMap(table)))
}

// modifyInTransaction runs table.Modify(true) in a database transaction (committed when it
// succeeds, rolled back otherwise) and then points the record back at the handler's
// connection. err is set only when the transaction itself fails.
func (h *TablesHandler) modifyInTransaction(table ftables.Table) (bool, error) {
	setter, ok := table.(interface{ SetDB(database.Executor) })
	if !ok {
		return table.Modify(true), nil
	}
	tx, err := h.db.Begin()
	if err != nil {
		return false, err
	}
	setter.SetDB(tx)
	defer setter.SetDB(h.db)
	if !table.Modify(true) {
		_ = ftables.Rollback(tx)
		return false, nil
	}
	if err := ftables.Commit(tx); err != nil {
		return false, err
	}
	return true, nil
}

// deleteInTransaction deletes the record like table.Delete(true); deleting a Company also
// drops all its tables and SIFT objects, in the same transaction, so a failure keeps the
// company and its data. err is set only when the transaction itself fails.
func (h *TablesHandler) deleteInTransaction(tableName string, table ftables.Table) (bool, error) {
	setter, ok := table.(interface{ SetDB(database.Executor) })
	if tableName != "Company" || !ok {
		return table.Delete(true), nil
	}
	companyName := table.GetPrimaryKeyValue()
	tx, err := h.db.Begin()
	if err != nil {
		return false, err
	}
	setter.SetDB(tx)
	defer setter.SetDB(h.db)
	if !table.Delete(true) {
		_ = ftables.Rollback(tx)
		return false, nil
	}
	if err := company.DropObjects(tx, h.dbType, companyName); err != nil {
		_ = ftables.Rollback(tx)
		return false, fmt.Errorf("delete company %s: %w", companyName, err)
	}
	return true, ftables.Commit(tx)
}

// errorMessage is an error as shown to the user: AppErrors (e.g. from table triggers) in
// the user's language, others as they are.
func errorMessage(err error, language string) string {
	var appErr *apperrors.AppError
	if errors.As(err, &appErr) {
		return appErr.Message(language)
	}
	return err.Error()
}

// refusePosted is the message refusing an insert, delete or rename on a posted table (posted:
// true — ledger entries and registers, written by posting only; classic: users have only
// indirect write permission on them), "" for other tables. Applies to every user, SUPER too.
func refusePosted(tableName, language string) string {
	if !pages.GetTableMetadata().IsPosted(tableName) {
		return ""
	}
	return apperrors.PostedTable(i18n.GetInstance().TableCaption(tableName, language)).Message(language)
}

// dropPostedFields leaves only the fields of a posted record the user may change (the table
// YAML's editable: true — a customer entry's Due Date, On Hold, Applies-to ID); the amounts,
// accounts, dates and keys of a posted entry are written by posting only
func dropPostedFields(tableName string, data map[string]interface{}) {
	metadata := pages.GetTableMetadata()
	if !metadata.IsPosted(tableName) {
		return
	}
	for field := range data {
		if !metadata.IsPostedFieldEditable(tableName, field) {
			delete(data, field)
		}
	}
}

// commitErrorMessage is a refused commit as shown to the user: an AppError (the balance check,
// ERR_INCONSISTENT) in the user's language, other errors as fallback
func commitErrorMessage(err error, fallback *apperrors.AppError, language string) string {
	var appErr *apperrors.AppError
	if errors.As(err, &appErr) {
		return appErr.Message(language)
	}
	return fallback.Message(language)
}

// applyFlowFilters sets the FlowFilter fields (FieldClass FlowFilter, e.g. Date Filter)
// from the flow_filters query parameter, JSON [{field, expression}]. The expression is
// validated per the field's kind (dates in ISO); unknown fields and invalid expressions
// are errors.
func applyFlowFilters(c *fiber.Ctx, table ftables.Table) error {
	param := c.Query("flow_filters", "")
	if param == "" {
		return nil
	}
	var filters []struct {
		Field      string `json:"field"`
		Expression string `json:"expression"`
	}
	if err := json.Unmarshal([]byte(param), &filters); err != nil {
		return fmt.Errorf("invalid flow_filters parameter")
	}
	for _, f := range filters {
		if err := table.SetFlowFilter(f.Field, f.Expression); err != nil {
			return fmt.Errorf("%s: %v", f.Field, err)
		}
	}
	return nil
}

// flowFieldsToCalc returns the FlowFields a list request needs: all of them when the
// client sends no field list, otherwise only the requested ones.
func flowFieldsToCalc(flowFields, requestedFields []string) []string {
	if len(requestedFields) == 0 {
		return flowFields
	}
	var calc []string
	for _, f := range flowFields {
		if slices.Contains(requestedFields, f) {
			calc = append(calc, f)
		}
	}
	return calc
}
