package tables

import (
	"fmt"
	"strings"

	"strconv"

	"github.com/hansjlachmann/openerp/backend/foundation/database"
)

// Table is the interface that all generated tables must implement.
// This enables generic API handlers without hardcoded switch statements.
type Table interface {
	// Identity
	GetTableID() int
	GetTableName() string
	// IsSetupTable reports whether this is a singleton setup table
	// (a single record identified by a blank primary key)
	IsSetupTable() bool

	// Initialization - must be called before any operations. There is no Init without the
	// database type: it decides the SQL placeholders (a SQLite default broke PostgreSQL)
	InitWithDBType(db database.Executor, company string, dbType database.DBType)
	// InitRecord initializes a new, not yet inserted record (OnNewRecord)
	InitRecord()

	// CRUD operations
	// Get retrieves a record by primary key, returns true if found
	Get(primaryKey interface{}) bool
	// Insert creates a new record, runTrigger controls OnInsert execution
	Insert(runTrigger bool) bool
	// Modify updates the current record, runTrigger controls OnModify execution
	Modify(runTrigger bool) bool
	// Delete removes the current record, runTrigger controls OnDelete execution
	Delete(runTrigger bool) bool
	// TriggerError returns the trigger error that made the last Insert/Modify/Delete
	// fail (a business-rule message for the user), or nil
	TriggerError() error
	// GetOrError / InsertOrError / ModifyOrError / DeleteOrError: Get, Insert, Modify and Delete
	// that fail like the classic GET/INSERT/MODIFY/DELETE without IF — the trigger's error or the
	// standard message ("<Table> <key> not found", "… already exists")
	GetOrError(primaryKey interface{}) error
	InsertOrError(runTrigger bool) error
	ModifyOrError(runTrigger bool) error
	DeleteOrError(runTrigger bool) error

	// Query operations
	// FindSet prepares iteration over filtered records
	FindSet() bool
	// Next moves to the next record, returns false when exhausted
	// Optional steps parameter allows skipping records (positive) or moving backward (negative)
	Next(steps ...int) bool
	// SetFilter applies a filter expression to a field
	SetFilter(field, expression string)
	// ClearFilters removes all filters
	ClearFilters()
	// SetCurrentKey sets the sort order by field(s)
	SetCurrentKey(fields ...string)
	// SetAscending sets the sort direction (default ascending). The primary key always
	// follows the current key, so the order is unique and pages never overlap.
	SetAscending(ascending bool)
	// SetSearch keeps the records where any of the fields contains text (case-insensitive)
	SetSearch(fields []string, text string)
	// HasColumn reports whether a field name is a stored column (allowed in filters/sort)
	HasColumn(field string) bool
	// SetPage sets a pagination window for FindSet: at most limit rows, skipping
	// the first offset rows. A limit of 0 returns all matching rows.
	SetPage(limit, offset int)
	// Count returns the number of records matching current filters (ignores pagination)
	Count() int

	// Field operations
	// CalcFields calculates FlowFields (computed fields)
	CalcFields(fields ...string)
	// CalcFieldsForRecords calculates FlowFields for many records at once (list pages):
	// one grouped query per FlowField. records are ToMap() results and get the values
	// set under the field names. With no fields, all FlowFields are calculated.
	CalcFieldsForRecords(records []map[string]interface{}, fields ...string)
	// SetFlowFilter sets a FlowFilter field's filter expression (e.g. Date Filter); FlowFields
	// applying it use it. "" clears it. Errors for unknown fields and invalid expressions.
	SetFlowFilter(field, expr string) error
	// GetFlowFilterFields returns the table's FlowFilter fields (not stored)
	GetFlowFilterFields() []FlowFilterFieldInfo
	// ValidateField validates a single field value
	ValidateField(field string, value interface{}) error

	// Serialization - for API JSON conversion
	// ToMap converts the current record to a map for JSON serialization
	ToMap() map[string]interface{}
	// FromMap populates the record fields from a map
	FromMap(data map[string]interface{})
	// UpdateFromMap updates only the provided fields (for PATCH-style updates)
	UpdateFromMap(data map[string]interface{})

	// Metadata
	// GetPrimaryKeyField returns the name of the primary key field
	GetPrimaryKeyField() string
	// GetPrimaryKeyValue returns the current primary key value as a string
	GetPrimaryKeyValue() string
	// GetFields returns metadata about all fields
	GetFields() []FieldInfo
	// GetFlowFields returns names of FlowFields that need CalcFields
	GetFlowFields() []string
	// GetOptionFields returns Option field names mapped to their option values
	// Returns map[fieldName][]string where each string is an option value (index = stored int value)
	GetOptionFields() map[string][]string
	// GetTableRelationFields returns fields that have table relations (foreign keys)
	// Returns map[fieldName]TableRelationInfo
	GetTableRelationFields() map[string]TableRelationInfo
}

// FlowFilterFieldInfo describes a FlowFilter field (FieldClass FlowFilter): its name and
// value kind ("date", "bool", "int", "text", "code")
type FlowFilterFieldInfo struct {
	Name string `json:"name"`
	Kind string `json:"kind"`
}

// FieldInfo contains metadata about a table field
type FieldInfo struct {
	No         int       // Field number (field ID), unique in the table
	Name       string    // Field name (snake_case)
	Type       FieldType // Field type
	Length     int       // Max length for Code/Text fields
	Required   bool      // Whether the field is required
	Editable   bool      // Whether the field can be edited
	PrimaryKey bool      // Whether this is the primary key
	FlowField  bool      // Whether this is a computed FlowField
	Sensitive  bool      // Never sent by the API, not filterable/sortable/searchable (YAML sensitive: true)
	Masked     bool      // Write-only secret: sent as MaskedValue, settable (YAML masked: true)
	// ReadOnly (YAML editable: false, classic Editable=No): pages show it read-only and the API
	// keeps the stored value; business logic can still set it
	ReadOnly bool
	// DecimalPlaces (YAML decimal_places, classic DecimalPlaces): "min:max" digits shown
	DecimalPlaces string
}

// LookupColumnInfo defines a column to display in the lookup dropdown
type LookupColumnInfo struct {
	Source string // Field name to display
	Width  int    // Column width in pixels (0 = auto)
}

// TableRelationInfo contains metadata about a field's table relation
type TableRelationInfo struct {
	Table         string             // Related table name (e.g., "PaymentTerms")
	Field         string             // Field in related table to match (usually primary key)
	DisplayField  string             // Field to display in dropdown (simple mode)
	LookupColumns []LookupColumnInfo // Columns to show in dropdown (advanced mode)
	SearchTimeout int                // Auto-clear search after N milliseconds (0 = use default 1500)
	// NoValidate (YAML validate: false, classic ValidateTableRelation=No): the relation is the
	// field's lookup only — any value is accepted (e.g. Totaling holds a filter of accounts)
	NoValidate bool
	// Conditions (YAML table_relation.conditions, classic IF (Type=CONST(x)) Table ELSE IF …):
	// the relation depends on another field of the record — the condition whose value the
	// record's field has applies; a value without a condition has no relation (its table is not
	// migrated yet). Table and Field are blank then.
	Conditions []ConditionalRelationInfo
	// Filters (YAML table_relation.filters, classic … WHERE (x=FIELD(y))): the related records
	// whose Column equals this record's Field (or Const) — e.g. a level of the record's terms
	Filters []RelationFilterInfo
}

// RelationFilterInfo is a WHERE condition of a table relation: the related table's Column
// equals this record's Field, or Const when Field is blank
type RelationFilterInfo struct {
	Column string
	Field  string
	Const  string
}

// ConditionalRelationInfo is one branch of a conditional relation: Relation applies when the
// record's Field (an option) has Value
type ConditionalRelationInfo struct {
	Field    string
	Value    int
	Relation TableRelationInfo
}

// For returns the relation that applies to a record (field name → value, e.g. ToMap); ok is
// false when a conditional relation has no branch for the record's value
func (ri TableRelationInfo) For(record map[string]interface{}) (TableRelationInfo, bool) {
	if len(ri.Conditions) == 0 {
		return ri, ri.Table != ""
	}
	for _, c := range ri.Conditions {
		if v, ok := intValue(record[c.Field]); ok && v == c.Value {
			return c.Relation, true
		}
	}
	return TableRelationInfo{}, false
}

// intValue reads an option or integer value as the API and records carry it
func intValue(v interface{}) (int, bool) {
	switch x := v.(type) {
	case int:
		return x, true
	case int64:
		return int(x), true
	case float64:
		return int(x), true
	case string:
		n, err := strconv.Atoi(x)
		return n, err == nil
	}
	return 0, false
}

// FieldType represents the field types
type FieldType string

const (
	FieldTypeCode     FieldType = "Code"
	FieldTypeText     FieldType = "Text"
	FieldTypeInteger  FieldType = "Integer"
	FieldTypeDecimal  FieldType = "Decimal"
	FieldTypeBoolean  FieldType = "Boolean"
	FieldTypeDate     FieldType = "Date"
	FieldTypeDateTime FieldType = "DateTime"
	FieldTypeOption   FieldType = "Option"
	FieldTypeBlob     FieldType = "Blob"
)

// TableFactory is a function that creates a new instance of a table
type TableFactory func() Table

// Source is anything that knows a database, company and database type — a record (every
// generated record) whose database and company another record is opened in:
// gtables.NewGLAccount(rec), gtables.GetGeneralLedgerSetup(rec), noseries.New(rec)
type Source interface {
	GetDB() database.Executor
	GetCompany() string
	GetDBType() database.DBType
}

// DBSource is a Source for code without a record at hand (a codeunit, a test):
// gtables.NewGLAccount(tables.DBSource(db, company, dbType))
func DBSource(db database.Executor, company string, dbType database.DBType) Source {
	return dbSource{db, company, dbType}
}

type dbSource struct {
	db      database.Executor
	company string
	dbType  database.DBType
}

func (s dbSource) GetDB() database.Executor   { return s.db }
func (s dbSource) GetCompany() string         { return s.company }
func (s dbSource) GetDBType() database.DBType { return s.dbType }

// KeyString is a record's whole primary key as text: the key fields' values joined by commas
// (the form of the API's record ids)
func KeyString(table Table) string {
	values := table.ToMap()
	var parts []string
	for _, f := range table.GetFields() {
		if f.PrimaryKey {
			parts = append(parts, fmt.Sprint(values[f.Name]))
		}
	}
	return strings.Join(parts, ",")
}
