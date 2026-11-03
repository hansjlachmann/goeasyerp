package main

import (
	"bytes"
	"flag"
	"fmt"
	"go/format"
	"go/token"
	"os"
	"path/filepath"
	"slices"
	"sort"
	"strconv"
	"strings"
	"text/template"
	"unicode"

	"gopkg.in/yaml.v3"
)

// TableDef represents a table definition from YAML
type TableDef struct {
	Table struct {
		ID         int     `yaml:"id"`
		Name       string  `yaml:"name"`
		APIName    string  `yaml:"api_name"`    // Name in the API and the pages' source_table (default: name)
		App        string  `yaml:"app"`         // Set by extmerge on a table of an extension: the extension's app id
		Global     bool    `yaml:"global"`      // If true, table is global (no company prefix)
		SetupTable bool    `yaml:"setup_table"` // If true, singleton setup table (single blank-PK record)
		Fields     []Field `yaml:"fields"`
		Keys       []Key   `yaml:"keys"`
		// DataCaptionFields (classic DataCaptionFields): fields whose values the pages show in
		// the title of a card ("Finance Charge Terms – STD Standard finance charge")
		DataCaptionFields []string `yaml:"data_caption_fields"`
	} `yaml:"table"`

	// FlowFilterFields (derived): fields with flow_filter: true, taken out of Fields after
	// parsing — they are not stored columns, the record only keeps their filter expression
	FlowFilterFields []Field `yaml:"-"`

	// References (derived): per primary key field (db name), the fields of other tables
	// whose table_relation points to it — updated when the record is renamed (Rename)
	References map[string][]Reference `yaml:"-"`
}

// Reference is a field of a table that refers to another table's key field
type Reference struct {
	Table  string // referencing table name (e.g. "Customer Ledger Entry")
	Global bool   // referencing table has no company prefix
	Column string // referencing column (e.g. "customer_no")
	// CondColumn / CondValue: a conditional relation refers only from records whose
	// CondColumn has CondValue (balancing accounts of type Customer)
	CondColumn string
	CondValue  int
}

// Key represents an index/key on a table
type Key struct {
	Name      string   `yaml:"name"`      // Key name (e.g., "customer_open")
	Fields    []string `yaml:"fields"`    // Fields in the key (e.g., ["customer_no", "open"])
	Unique    bool     `yaml:"unique"`    // Whether this is a UNIQUE index
	Clustered bool     `yaml:"clustered"` // Primary key-like behavior (concept)
	// SumIndexFields (SIFT): the database keeps a totals table for this key with
	// these fields summed, and FlowFields over the key's fields read the totals
	SumIndexFields []string `yaml:"sum_index_fields"`
}

// Field represents a single field in a table
type Field struct {
	No            int         `yaml:"no"`  // Field number (field ID): unique in the table, never changed or reused
	App           string      `yaml:"app"` // Set by extmerge on fields an extension adds: the extension's app id
	Name          string      `yaml:"name"`
	Type          string      `yaml:"type"`
	DBName        string      `yaml:"db_name"`
	PrimaryKey    bool        `yaml:"primary_key"`
	Length        int         `yaml:"length"`
	Required      bool        `yaml:"required"`
	Default       interface{} `yaml:"default"`
	AutoTimestamp bool        `yaml:"auto_timestamp"`
	Validation    *Validation `yaml:"validation"`
	// HasFieldFilter / FieldFilterColumn (derived, FlowFields): a "field_filter" flow filter
	// applies the filter held in the record's FieldFilterColumn (Totaling) — such FlowFields are
	// calculated per record for those records and cannot be filtered or sorted on in lists
	HasFieldFilter    bool           `yaml:"-"`
	FieldFilterColumn string         `yaml:"-"`
	TableRelation     *TableRelation `yaml:"table_relation"`
	Options           []string       `yaml:"options"`      // For Option type fields (enum values)
	Precision         int            `yaml:"precision"`    // For Decimal type (total digits)
	Scale             int            `yaml:"scale"`        // For Decimal type (decimal places)
	FlowField         bool           `yaml:"flow_field"`   // For FlowFields (calculated fields)
	CalcFormula       string         `yaml:"calc_formula"` // Sum, Count, Lookup, Exist, Average, Min, Max
	SourceTable       string         `yaml:"source_table"` // Table to calculate from
	SourceField       string         `yaml:"source_field"` // Field to aggregate
	FlowFilters       []FlowFilter   `yaml:"flow_filters"` // Filter conditions
	// ReverseSign (Sum FlowFields): the total with the opposite sign (classic CalcFormula=-Sum,
	// e.g. Customer Payments)
	ReverseSign bool `yaml:"reverse_sign"`
	// FlowFilter (FieldClass FlowFilter): not stored; holds a filter expression the
	// user sets (e.g. Date Filter), applied by FlowFields with a flow filter of type "filter"
	FlowFilter bool `yaml:"flow_filter"`
	// Sensitive: never sent by the table API (e.g. User.password_hash), and not usable in
	// filters, sorting, search or lookup columns; the API cannot set it either
	Sensitive bool `yaml:"sensitive"`
	// Masked (ExtendedDatatype Masked): a secret that can be set through the API but is
	// never read back — responses carry tables.MaskedValue when it has a value (e.g.
	// SMTP_Setup.password); not usable in filters, sorting, search or lookup columns
	Masked bool `yaml:"masked"`
	// Encrypted: a masked secret the server has to read back in clear (e.g. the SMTP
	// password it logs in with) is stored encrypted (backend/foundation/secrets, AES-256-GCM)
	// — Insert/Modify/InsertAll encrypt it, Plain<Field>() decrypts it. Requires masked and
	// types.Text. Not for values that are only compared (user passwords: hash them, sensitive).
	Encrypted bool `yaml:"encrypted"`
	// DateFormula: a types.Code field that holds a date formula ("30D", "1M+CM"); the field
	// checks and ValidateField reject a value that is not one (foundation/dateformula), and
	// business logic calculates with it: system.CalcDate(string(rec.Due_date_calculation), d)
	DateFormula bool `yaml:"date_formula"`
	// Editable: false (classic Editable=No) — the pages show the field read-only and the API
	// keeps its stored value; business logic (Go) can still set it
	Editable *bool `yaml:"editable"`
	// DecimalPlaces (classic DecimalPlaces "min:max"): the pages show a decimal with at least
	// min and at most max digits after the point ("2:5" → 0.01, 0.00001); storage keeps scale
	DecimalPlaces string `yaml:"decimal_places"`

	// Derived (not in YAML): for a FlowField with exactly one "field" flow filter, the
	// source column to group by and the record map key holding its value. Set by
	// prepareTemplateData; empty means CalcFieldsForRecords calculates per record.
	BatchKeyField string `yaml:"-"`
	BatchKeyValue string `yaml:"-"`
	// SIFTKey (derived): the source table's SIFT key this FlowField reads its totals from
	// ("" = sums the entries)
	SIFTKey string `yaml:"-"`
	// SIFTKeyFiltered (derived): the SIFT key to read when a FlowFilter of this FlowField is
	// set (its key must contain the filtered fields too); "" = sums the entries then
	SIFTKeyFiltered string `yaml:"-"`
	// FilterRefs (derived): the flow filters of type "filter" (FlowFilter fields applied)
	FilterRefs []FlowFilter `yaml:"-"`
}

// FlowFilter represents a filter condition for FlowField calculation
type FlowFilter struct {
	Field string `yaml:"field"` // Field name in source table
	Type  string `yaml:"type"`  // "const", "field", "filter" (apply a FlowFilter field of this table) or "field_filter" (a stored field of this table holding a filter, e.g. Totaling)
	Value string `yaml:"value"` // Constant value, or field / FlowFilter field name of the current table
	// UpperLimit (type "filter"): only the upper end of the FlowFilter applies — up to the
	// filter's last date, from the start ("Balance at Date": Posting Date = UpperLimit(Date Filter))
	UpperLimit bool `yaml:"upper_limit"`

	// Derived for type "filter": the FlowFilter field's struct field and flowfilter kind
	FilterField string `yaml:"-"`
	Kind        string `yaml:"-"`
	// Derived for type "field": the database column of the current table's field
	ValueColumn string `yaml:"-"`
	// Derived for type "field" when a "field_filter" flow filter on the same source field
	// exists: the struct field holding the filter (e.g. Totaling) and the source field's
	// flowfilter kind — a filter in it replaces the equality (classic FIELD(FILTER(Totaling)))
	OverrideField string `yaml:"-"`
	OverrideKind  string `yaml:"-"`
}

// LookupColumn defines a column to display in the lookup dropdown
type LookupColumn struct {
	Source string `yaml:"source"` // Field name to display
	Width  int    `yaml:"width"`  // Column width in pixels (optional)
}

// TableRelation represents a foreign key relationship to another table
type TableRelation struct {
	Table         string         `yaml:"table"`
	Field         string         `yaml:"field"`
	DisplayField  string         `yaml:"display_field"`  // Field to show in dropdown (e.g., "description") - simple mode
	LookupColumns []LookupColumn `yaml:"lookup_columns"` // Columns to show in dropdown - advanced mode
	SearchTimeout int            `yaml:"search_timeout"` // Auto-clear search after N milliseconds (default: 1500)
	Validate      *bool          `yaml:"validate"`       // Whether to validate the relation (default: true)
	// Conditions (classic IF (Type=CONST(x)) Table ELSE IF …): the relation depends on another
	// field of the record — each condition is a relation of its own (Table, Field, lookup
	// columns) that applies when the record's field equals the value; Table/Field stay blank
	Conditions []RelationCondition `yaml:"conditions"`
	// Filters (classic TableRelation … WHERE (Field=FIELD(x)) / CONST(y)): the related records
	// are those whose column matches this record's field (field) or a constant (const) — for a
	// key part of a composite key (Reminder Level No. of the record's Reminder Terms)
	Filters map[string]RelationFilter `yaml:"filters"`
}

// RelationFilter is one WHERE condition of a table relation: a field of this record or a constant
type RelationFilter struct {
	Field string      `yaml:"field"`
	Const interface{} `yaml:"const"`
}

// SortedFilters returns the relation's filters in column order (stable generated code)
func (tr *TableRelation) SortedFilters() []struct {
	Column string
	RelationFilter
} {
	var out []struct {
		Column string
		RelationFilter
	}
	cols := make([]string, 0, len(tr.Filters))
	for col := range tr.Filters {
		cols = append(cols, col)
	}
	sort.Strings(cols)
	for _, col := range cols {
		out = append(out, struct {
			Column string
			RelationFilter
		}{col, tr.Filters[col]})
	}
	return out
}

// RelationCondition is one branch of a conditional table relation
type RelationCondition struct {
	When struct {
		Field string `yaml:"field"` // an option field of the same table (e.g. bal_account_type)
		Equal int    `yaml:"equal"` // the option's number
	} `yaml:"when"`
	TableRelation `yaml:",inline"`
}

// Branches returns the relations of a field: its conditions, or itself
func (tr *TableRelation) Branches() []RelationCondition {
	if len(tr.Conditions) > 0 {
		return tr.Conditions
	}
	return []RelationCondition{{TableRelation: *tr}}
}

// ShouldValidate returns whether this table relation should be validated
func (tr *TableRelation) ShouldValidate() bool {
	if tr.Validate == nil {
		return true // default is true
	}
	return *tr.Validate
}

// Validation represents field validation rules
// Validation is a field's range (classic MinValue / MaxValue): min, max or both; on whole
// numbers and decimals
type Validation struct {
	Min interface{} `yaml:"min"`
	Max interface{} `yaml:"max"`
}

// TemplateData is the data passed to templates
type TemplateData struct {
	TableDef
	StructName       string
	PackageName      string
	GeneratedPkg     string // Import alias for generated package (e.g., "gtables")
	HasSumFields     bool   // a stored decimal or integer field (CalcSums)
	HasTimeField     bool
	HasCodeField     bool
	HasTextField     bool
	HasOptionField   bool
	HasDecimalField  bool
	HasDateField     bool
	HasDateTimeField bool
	HasFlowField     bool
	HasBlobField     bool
	HasIntField      bool
	FirstPrimaryKey  *Field        // First primary key field (for GetPrimaryKeyField/Value)
	SIFTKeys         []SIFTKeyData // this table's keys with sum_index_fields
	UsesSIFT         bool          // imports the sift package (own SIFT keys or FlowFields reading totals)
	UsesFlowFilter   bool          // imports the flowfilter package (table has FlowFilter fields)
	EncryptedFields  []Field       // fields stored encrypted (encrypted: true)
	// HasFilterableFlowField: Sum/Count FlowFields lists can filter on (FlowFieldFilterKind)
	HasFilterableFlowField bool
}

// SIFTKeyData is a key with sum index fields, as the templates need it
type SIFTKeyData struct {
	Name   string
	Fields []SIFTColumn
	Sums   []SIFTColumn
}

// SIFTColumn is a key or sum column of a SIFT key (column name and sift.Kind)
type SIFTColumn struct {
	Name string
	Kind string
}

func main() {
	// Run by go:generate in backend/generated/tables (generate.go): the definitions, the
	// generated code and the business logic are found relative to it
	defsDir := flag.String("defs", "../../definitions/tables", "directory of the table definitions (*.yaml)")
	outDir := flag.String("out", ".", "directory of the generated code (package tables)")
	logicDir := flag.String("logic", "../../business-logic/tables", "directory of the business logic: a new table gets a starting file here")
	appFile := flag.String("app", "../../definitions/application.yaml", "the base application's application.yaml (number series)")
	flag.Parse()

	app, err := loadApplication(*appFile)
	if err != nil {
		fmt.Printf("Error reading the application: %v\n", err)
		os.Exit(1)
	}

	if err := os.MkdirAll(*outDir, 0755); err != nil {
		fmt.Printf("Error creating output directory: %v\n", err)
		os.Exit(1)
	}

	// Find all YAML definition files
	yamlFiles, err := filepath.Glob(filepath.Join(*defsDir, "*.yaml"))
	if err != nil {
		fmt.Printf("Error finding YAML files: %v\n", err)
		os.Exit(1)
	}

	if len(yamlFiles) == 0 {
		fmt.Println("No YAML definition files found in", *defsDir)
		os.Exit(1)
	}
	sort.Strings(yamlFiles)

	fmt.Printf("Found %d table definition(s)\n", len(yamlFiles))

	// Pass 1: read every definition. A FlowField reads the totals of a SIFT key on its
	// source table, so FlowFields can only be resolved when all tables are known.
	type parsed struct {
		file string
		def  *TableDef
	}
	var defs []parsed
	byStruct := map[string]*TableDef{}
	failed := false
	for _, yamlFile := range yamlFiles {
		tableDef, err := parseYAML(yamlFile)
		if err != nil {
			fmt.Printf("✗ Error parsing %s: %v\n", filepath.Base(yamlFile), err)
			failed = true
			continue
		}
		if err := validateNumberSeries(tableDef, app); err != nil {
			fmt.Printf("✗ %s: %v\n", filepath.Base(yamlFile), err)
			failed = true
			continue
		}
		if err := validateFieldNumbers(tableDef); err != nil {
			fmt.Printf("✗ %s: %v\n", filepath.Base(yamlFile), err)
			failed = true
			continue
		}
		if err := validateRelationConditions(tableDef); err != nil {
			fmt.Printf("✗ %s: %v\n", filepath.Base(yamlFile), err)
			failed = true
			continue
		}
		if err := validateFieldProperties(tableDef); err != nil {
			fmt.Printf("✗ %s: %v\n", filepath.Base(yamlFile), err)
			failed = true
			continue
		}
		if err := validateDateFormulaFields(tableDef); err != nil {
			fmt.Printf("✗ %s: %v\n", filepath.Base(yamlFile), err)
			failed = true
			continue
		}
		splitFlowFilterFields(tableDef)
		defs = append(defs, parsed{yamlFile, tableDef})
		byStruct[toPascalCase(tableDef.Table.Name)] = tableDef
	}
	for _, p := range defs {
		if err := validateSIFTKeys(p.def); err != nil {
			fmt.Printf("✗ %s: %v\n", filepath.Base(p.file), err)
			failed = true
		}
		if err := validateFlowFilters(p.def, byStruct); err != nil {
			fmt.Printf("✗ %s: %v\n", filepath.Base(p.file), err)
			failed = true
		}
		if err := validateSensitiveFields(p.def, byStruct); err != nil {
			fmt.Printf("✗ %s: %v\n", filepath.Base(p.file), err)
			failed = true
		}
	}
	if failed {
		os.Exit(1)
	}
	for _, p := range defs {
		resolveFlowFieldSIFT(p.def, byStruct)
	}
	resolveReferences(defs, func(d parsed) *TableDef { return d.def })

	// The output directory holds generated code only: remove the files of the last run,
	// so a table removed from the definitions leaves nothing behind
	old, _ := filepath.Glob(filepath.Join(*outDir, "*_gen.go"))
	for _, f := range old {
		if err := os.Remove(f); err != nil {
			fmt.Printf("Error removing %s: %v\n", f, err)
			os.Exit(1)
		}
	}

	// Pass 2: generate
	var all []TemplateData
	for _, p := range defs {
		data := prepareTemplateData(p.def)
		all = append(all, data)

		genFile := filepath.Join(*outDir, strings.ToLower(data.StructName)+"_gen.go")
		if err := generateFile(genFile, "gen", boilerplateTemplate, data); err != nil {
			fmt.Printf("✗ %s: %v\n", filepath.Base(p.file), err)
			os.Exit(1)
		}

		// A new table gets a starting business logic file (never overwritten)
		logicFile := filepath.Join(*logicDir, strings.ToLower(data.StructName)+".go")
		if !fileExists(logicFile) {
			if err := generateFile(logicFile, "logic", businessTemplate, data); err != nil {
				fmt.Printf("✗ %s: %v\n", filepath.Base(logicFile), err)
				os.Exit(1)
			}
			fmt.Printf("  ✓ Created business logic: %s\n", logicFile)
		}
	}
	if err := generateFile(filepath.Join(*outDir, "tables_gen.go"), "tables", tablesTemplate, all); err != nil {
		fmt.Printf("✗ tables_gen.go: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("✓ Generated %d table(s) in %s\n", len(all), *outDir)
}

// parseYAML reads and parses a YAML definition file
func parseYAML(filename string) (*TableDef, error) {
	data, err := os.ReadFile(filename)
	if err != nil {
		return nil, err
	}

	var def TableDef
	if err := yaml.Unmarshal(data, &def); err != nil {
		return nil, err
	}

	// Auto-fill db_name if not specified
	for i := range def.Table.Fields {
		if def.Table.Fields[i].DBName == "" {
			def.Table.Fields[i].DBName = toSnakeCase(def.Table.Fields[i].Name)
		}
	}

	return &def, nil
}

// prepareTemplateData creates template data from table definition
func prepareTemplateData(def *TableDef) TemplateData {
	structName := toPascalCase(def.Table.Name)
	data := TemplateData{
		TableDef:     *def,
		StructName:   structName,
		PackageName:  "tables",
		GeneratedPkg: "gtables",
	}

	// Check which imports are needed and find first primary key
	for i := range def.Table.Fields {
		field := &def.Table.Fields[i]
		if field.Type == "time.Time" {
			data.HasTimeField = true
		}
		if field.Type == "types.Code" {
			data.HasCodeField = true
		}
		if field.Type == "types.Text" {
			data.HasTextField = true
		}
		if field.Type == "Option" {
			data.HasOptionField = true
		}
		if field.Type == "types.Decimal" {
			data.HasDecimalField = true
		}
		if field.Type == "types.Date" {
			data.HasDateField = true
		}
		if field.Type == "types.DateTime" {
			data.HasDateTimeField = true
		}
		if field.FlowField {
			data.HasFlowField = true
		}
		if field.Type == "[]byte" || field.Type == "BLOB" {
			data.HasBlobField = true
		}
		// an int field, or an option in the primary key (its key value is parsed): strconv
		if (field.Type == "int" || field.Type == "Option" && field.PrimaryKey) && !field.FlowField {
			data.HasIntField = true
		}
		if !field.FlowField && (field.Type == "int" || field.Type == "types.Decimal") {
			data.HasSumFields = true
		}
		if field.Encrypted {
			data.EncryptedFields = append(data.EncryptedFields, *field)
		}
		// Track first primary key field (for tables with composite keys)
		if field.PrimaryKey && data.FirstPrimaryKey == nil {
			data.FirstPrimaryKey = field
		}
	}

	// FlowFields with a single "field" flow filter can be calculated for many records
	// with one grouped query (CalcFieldsForRecords)
	dbNames := make(map[string]string, len(def.Table.Fields))
	for _, f := range def.Table.Fields {
		dbNames[f.Name] = f.DBName
	}
	for i := range def.Table.Fields {
		field := &def.Table.Fields[i]
		if !field.FlowField {
			continue
		}
		var keyFilters []FlowFilter
		for _, ff := range field.FlowFilters {
			if ff.Type == "field" {
				keyFilters = append(keyFilters, ff)
			}
		}
		if len(keyFilters) == 1 {
			field.BatchKeyField = keyFilters[0].Field
			field.BatchKeyValue = dbNames[keyFilters[0].Value]
			if field.BatchKeyValue == "" {
				field.BatchKeyValue = keyFilters[0].Value
			}
		}
	}
	data.TableDef = *def

	// SIFT keys of this table, and whether the sift package is needed
	fieldsByName := map[string]Field{}
	for _, f := range def.Table.Fields {
		fieldsByName[f.Name] = f
	}
	for _, k := range def.Table.Keys {
		if len(k.SumIndexFields) == 0 {
			continue
		}
		kd := SIFTKeyData{Name: k.Name}
		for _, name := range k.Fields {
			f := fieldsByName[name]
			kd.Fields = append(kd.Fields, SIFTColumn{Name: f.DBName, Kind: siftKind(f)})
		}
		for _, name := range k.SumIndexFields {
			f := fieldsByName[name]
			kd.Sums = append(kd.Sums, SIFTColumn{Name: f.DBName, Kind: siftKind(f)})
		}
		data.SIFTKeys = append(data.SIFTKeys, kd)
	}
	data.UsesSIFT = true // SyncKeys always calls sift.Sync (drops totals of removed keys)
	hasFieldFilter := false
	for _, f := range def.Table.Fields {
		if f.FlowField && (f.CalcFormula == "Sum" || f.CalcFormula == "Count") && !f.HasFieldFilter {
			data.HasFilterableFlowField = true
		}
		hasFieldFilter = hasFieldFilter || f.HasFieldFilter
	}
	data.UsesFlowFilter = len(def.FlowFilterFields) > 0 || data.HasFilterableFlowField || hasFieldFilter

	return data
}

// siftKind maps a field type to the sift package's column kind
func siftKind(f Field) string {
	switch f.Type {
	case "int", "int64", "Option":
		return "KindInt"
	case "bool":
		return "KindBool"
	case "types.Date":
		return "KindDate"
	case "types.DateTime", "time.Time":
		return "KindDateTime"
	case "types.Decimal", "float64":
		return "KindDecimal"
	default:
		return "KindText"
	}
}

// normalizedTableName compares table names written as registry names ("Payment_terms") and
// as display names ("Payment Terms")
func normalizedTableName(name string) string {
	return strings.ToLower(strings.ReplaceAll(strings.TrimSpace(name), " ", "_"))
}

// resolveReferences records, for each table's primary key fields, the fields of other tables
// whose table_relation points to them (renaming a record updates those, Rename).
func resolveReferences[T any](items []T, defOf func(T) *TableDef) {
	byName := map[string]*TableDef{}
	for _, it := range items {
		d := defOf(it)
		byName[normalizedTableName(d.Table.Name)] = d
		d.References = nil
	}
	for _, it := range items {
		d := defOf(it)
		for _, f := range d.Table.Fields {
			if f.TableRelation == nil || f.FlowField {
				continue
			}
			if hasFieldFilter(f.TableRelation.Filters) {
				continue // a relation to part of a composite key: no rename cascade (yet)
			}
			for _, branch := range f.TableRelation.Branches() {
				if hasFieldFilter(branch.Filters) {
					continue
				}
				target, ok := byName[normalizedTableName(branch.Table)]
				if !ok {
					continue
				}
				condColumn := ""
				if branch.When.Field != "" {
					condColumn = dbNameOf(d, branch.When.Field)
				}
				for _, tf := range target.Table.Fields {
					if tf.PrimaryKey && (tf.Name == branch.Field || tf.DBName == branch.Field) {
						if target.References == nil {
							target.References = map[string][]Reference{}
						}
						target.References[tf.DBName] = append(target.References[tf.DBName], Reference{
							Table: d.Table.Name, Global: d.Table.Global, Column: f.DBName,
							CondColumn: condColumn, CondValue: branch.When.Equal,
						})
					}
				}
			}
		}
	}
}

// hasFieldFilter: a relation filter on another field of the record (WHERE x=FIELD(y)) — the
// relation points to part of a composite key. Constant filters (WHERE Account Type=CONST(Posting))
// still point to the whole key, so a rename cascades.
func hasFieldFilter(filters map[string]RelationFilter) bool {
	for _, f := range filters {
		if f.Field != "" {
			return true
		}
	}
	return false
}

// Highest field number: the base application uses its field series (application.yaml, 1–999),
// extension fields (merged in by extmerge, marked with app) the series of their app
const maxExtensionFieldNo = 99999

// numberRange and application are the parts of application.yaml tablegen checks against
// (the server reads the whole file: backend/foundation/apps)
type numberRange struct {
	From int `yaml:"from"`
	To   int `yaml:"to"`
}

type application struct {
	Application struct {
		Name         string `yaml:"name"`
		NumberSeries struct {
			Objects []numberRange `yaml:"objects"`
			Fields  []numberRange `yaml:"fields"`
		} `yaml:"number_series"`
		SystemTables []numberRange `yaml:"system_tables"`
	} `yaml:"application"`
}

func inRanges(ranges []numberRange, n int) bool {
	for _, r := range ranges {
		if n >= r.From && n <= r.To {
			return true
		}
	}
	return false
}

func loadApplication(path string) (*application, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var app application
	if err := yaml.Unmarshal(data, &app); err != nil {
		return nil, fmt.Errorf("%s: %w", path, err)
	}
	if len(app.Application.NumberSeries.Objects) == 0 || len(app.Application.NumberSeries.Fields) == 0 {
		return nil, fmt.Errorf("%s: number_series needs objects and fields", path)
	}
	return &app, nil
}

// validateNumberSeries checks a table of the base application against its application.yaml:
// the table number in its object series or system tables, every field (except those an
// extension added, marked with app) in its field series
func validateNumberSeries(def *TableDef, app *application) error {
	a := app.Application
	if def.Table.App != "" {
		return nil // a table of an extension app: extmerge checked it against the app's series
	}
	if !inRanges(a.NumberSeries.Objects, def.Table.ID) && !inRanges(a.SystemTables, def.Table.ID) {
		return fmt.Errorf("table number %d is outside the object series and system tables of %s", def.Table.ID, a.Name)
	}
	var outside []string
	for _, f := range def.Table.Fields {
		if f.App == "" && f.No > 0 && !inRanges(a.NumberSeries.Fields, f.No) {
			outside = append(outside, fmt.Sprintf("%s (%d)", f.Name, f.No))
		}
	}
	if len(outside) > 0 {
		return fmt.Errorf("fields outside the field series of %s: %s", a.Name, strings.Join(outside, ", "))
	}
	return nil
}

// validateFieldNumbers checks that every field (also FlowFields and FlowFilters) has a field
// number (field ID) in the allowed ranges, unique in the table.
func validateFieldNumbers(def *TableDef) error {
	var problems []string
	byNo := map[int]string{}
	for _, f := range def.Table.Fields {
		switch {
		case f.No == 0:
			problems = append(problems, fmt.Sprintf("field %s has no field number (no:)", f.Name))
		case f.No < 0 || f.No > maxExtensionFieldNo:
			problems = append(problems, fmt.Sprintf("field %s: number %d outside 1–%d", f.Name, f.No, maxExtensionFieldNo))
		case byNo[f.No] != "":
			problems = append(problems, fmt.Sprintf("fields %s and %s have the same number %d", byNo[f.No], f.Name, f.No))
		default:
			byNo[f.No] = f.Name
		}
	}
	if len(problems) > 0 {
		return fmt.Errorf("field numbers: %s", strings.Join(problems, "; "))
	}
	return nil
}

// validateDateFormulaFields checks that date formula fields (date_formula: true) are stored
// types.Code fields
func validateDateFormulaFields(def *TableDef) error {
	for _, f := range def.Table.Fields {
		if f.DateFormula && (f.Type != "types.Code" || f.FlowField || f.FlowFilter) {
			return fmt.Errorf("field %s: date_formula needs a stored types.Code field", f.Name)
		}
	}
	return nil
}

// splitFlowFilterFields moves FlowFilter fields out of Fields (they are not stored).
func splitFlowFilterFields(def *TableDef) {
	stored := def.Table.Fields[:0]
	for _, f := range def.Table.Fields {
		if f.FlowFilter {
			def.FlowFilterFields = append(def.FlowFilterFields, f)
		} else {
			stored = append(stored, f)
		}
	}
	def.Table.Fields = stored
}

// flowFilterKind maps a FlowFilter field's type to the flowfilter package kind
func flowFilterKind(f Field) (string, bool) {
	switch f.Type {
	case "types.Date":
		return "KindDate", true
	case "bool":
		return "KindBool", true
	case "int", "int64":
		return "KindInt", true
	case "types.Text":
		return "KindText", true
	case "types.Code":
		return "KindCode", true
	}
	return "", false
}

// validateFlowFilters checks FlowFilter field types and the "filter" flow filters of
// FlowFields (FlowFilter field of this table, stored field of the source table), and fills
// their derived struct field / kind.
func validateFlowFilters(def *TableDef, byStruct map[string]*TableDef) error {
	for _, f := range def.Table.Fields {
		if f.FlowField && f.CalcFormula == "Lookup" && f.Type != "types.Text" && f.Type != "types.Code" {
			return fmt.Errorf("Lookup FlowField %s must be types.Text or types.Code, not %s", f.Name, f.Type)
		}
		if f.ReverseSign && !(f.FlowField && f.CalcFormula == "Sum") {
			return fmt.Errorf("field %s: reverse_sign is only for Sum FlowFields", f.Name)
		}
		if f.FlowField && f.CalcFormula == "Exist" && f.Type != "bool" {
			return fmt.Errorf("field %s: an Exist FlowField is a bool, not %s", f.Name, f.Type)
		}
	}
	filters := map[string]Field{}
	for _, f := range def.FlowFilterFields {
		if _, ok := flowFilterKind(f); !ok {
			return fmt.Errorf("FlowFilter field %q has type %s; supported: types.Date, bool, int, types.Text, types.Code", f.Name, f.Type)
		}
		filters[f.Name] = f
	}
	for i := range def.Table.Fields {
		f := &def.Table.Fields[i]
		if !f.FlowField {
			continue
		}
		for j := range f.FlowFilters {
			ff := &f.FlowFilters[j]
			if ff.Type == "field" {
				for _, own := range def.Table.Fields {
					if own.Name == ff.Value && !own.FlowField && !own.FlowFilter {
						ff.ValueColumn = own.DBName
					}
				}
				if ff.ValueColumn == "" {
					return fmt.Errorf("FlowField %s: flow filter on %s uses %q, which is not a stored field of %s", f.Name, ff.Field, ff.Value, def.Table.Name)
				}
			}
			if ff.Type == "field_filter" {
				var own *Field
				for k := range def.Table.Fields {
					if def.Table.Fields[k].Name == ff.Value && !def.Table.Fields[k].FlowField {
						own = &def.Table.Fields[k]
					}
				}
				if own == nil || (own.Type != "types.Text" && own.Type != "types.Code") {
					return fmt.Errorf("FlowField %s: field_filter on %s uses %q, which is not a stored Text/Code field of %s", f.Name, ff.Field, ff.Value, def.Table.Name)
				}
				kind := "KindCode"
				if source, ok := byStruct[toPascalCase(f.SourceTable)]; ok {
					for _, sf := range source.Table.Fields {
						if sf.Name == ff.Field {
							if k, ok := flowFilterKind(sf); ok {
								kind = k
							}
						}
					}
				}
				found := false
				for k := range f.FlowFilters {
					if f.FlowFilters[k].Type == "field" && f.FlowFilters[k].Field == ff.Field {
						f.FlowFilters[k].OverrideField = upperFirst(own.Name)
						f.FlowFilters[k].OverrideKind = kind
						found = true
					}
				}
				if !found {
					return fmt.Errorf("FlowField %s: field_filter on %s needs a flow filter of type field on the same field", f.Name, ff.Field)
				}
				f.HasFieldFilter = true
				f.FieldFilterColumn = own.DBName
				continue
			}
			if ff.UpperLimit && ff.Type != "filter" {
				return fmt.Errorf("FlowField %s: upper_limit on %s needs a flow filter of type filter", f.Name, ff.Field)
			}
			if ff.Type != "filter" {
				continue
			}
			filter, ok := filters[ff.Value]
			if !ok {
				return fmt.Errorf("FlowField %s: flow filter on %s uses %q, which is not a FlowFilter field of %s", f.Name, ff.Field, ff.Value, def.Table.Name)
			}
			if source, ok := byStruct[toPascalCase(f.SourceTable)]; ok {
				found := false
				for _, sf := range source.Table.Fields {
					if sf.Name == ff.Field && !sf.FlowField {
						found = true
					}
				}
				if !found {
					return fmt.Errorf("FlowField %s: %q is not a stored field of %s", f.Name, ff.Field, source.Table.Name)
				}
			}
			ff.FilterField = upperFirst(filter.Name)
			ff.Kind, _ = flowFilterKind(filter)
		}
	}
	return nil
}

// encryptedLength is the stored length of a secret of n bytes: "enc:v1:" + base64 without
// padding of nonce (12) + secret + GCM tag (16). Must match secrets.EncryptedLength.
func encryptedLength(n int) int {
	return len("enc:v1:") + (4*(12+n+16)+2)/3
}

// maxEncryptedPlain is the longest secret (bytes) whose encrypted form fits a column of
// the given length
func maxEncryptedPlain(length int) int {
	n := 0
	for encryptedLength(n+1) <= length {
		n++
	}
	return n
}

// validateSensitiveFields checks that sensitive and masked fields are plain stored fields
// (not a key, FlowField or FlowFilter; masked ones Text/Code) and that no table relation
// shows one of another table in its dropdown — the dropdown rows would send it to the client.
func validateSensitiveFields(def *TableDef, byStruct map[string]*TableDef) error {
	for _, f := range def.Table.Fields {
		if f.Sensitive && (f.PrimaryKey || f.FlowField || f.FlowFilter) {
			return fmt.Errorf("sensitive field %q cannot be a primary key, FlowField or FlowFilter", f.Name)
		}
		if f.Masked && (f.PrimaryKey || f.FlowField || f.FlowFilter || f.Sensitive) {
			return fmt.Errorf("masked field %q cannot be a primary key, FlowField, FlowFilter or sensitive", f.Name)
		}
		if f.Masked && f.Type != "types.Text" && f.Type != "types.Code" {
			return fmt.Errorf("masked field %q must be types.Text or types.Code, not %s", f.Name, f.Type)
		}
		if f.Encrypted && (!f.Masked || f.Type != "types.Text") {
			return fmt.Errorf("encrypted field %q must be masked and types.Text (an encrypted value is never sent by the API)", f.Name)
		}
		if f.Encrypted && maxEncryptedPlain(f.Length) < 16 {
			return fmt.Errorf("encrypted field %q: length %d is too short for an encrypted value (at least %d)", f.Name, f.Length, encryptedLength(16))
		}
	}
	for _, f := range def.Table.Fields {
		if f.TableRelation == nil {
			continue
		}
		for _, branch := range f.TableRelation.Branches() {
			rel := branch.TableRelation
			related, ok := byStruct[toPascalCase(rel.Table)]
			if !ok {
				continue
			}
			shown := []string{rel.Field, rel.DisplayField}
			for _, col := range rel.LookupColumns {
				shown = append(shown, col.Source)
			}
			for _, rf := range related.Table.Fields {
				if !rf.Sensitive && !rf.Masked {
					continue
				}
				for _, name := range shown {
					if name == rf.Name {
						return fmt.Errorf("table relation of %s shows secret (sensitive/masked) field %s.%s", f.Name, related.Table.Name, rf.Name)
					}
				}
			}
		}
	}
	return nil
}

// validateSIFTKeys checks keys with sum_index_fields: key and sum fields must be stored
// fields of the table, sum fields numeric.
func validateSIFTKeys(def *TableDef) error {
	fields := map[string]Field{}
	for _, f := range def.Table.Fields {
		fields[f.Name] = f
	}
	for _, k := range def.Table.Keys {
		if len(k.SumIndexFields) == 0 {
			continue
		}
		for _, name := range k.Fields {
			f, ok := fields[name]
			if !ok || f.FlowField {
				return fmt.Errorf("key %s: field %q is not a stored field of %s", k.Name, name, def.Table.Name)
			}
		}
		for _, name := range k.SumIndexFields {
			f, ok := fields[name]
			if !ok || f.FlowField {
				return fmt.Errorf("key %s: sum index field %q is not a stored field of %s", k.Name, name, def.Table.Name)
			}
			switch f.Type {
			case "types.Decimal", "int", "int64", "float64":
			default:
				return fmt.Errorf("key %s: sum index field %q has type %s; only numbers can be summed", k.Name, name, f.Type)
			}
		}
	}
	return nil
}

// resolveFlowFieldSIFT picks, for each Sum/Count FlowField, a SIFT key on its source table
// whose fields contain every flow filter field and (Sum) whose sum index fields contain
// the summed field — the key with the fewest fields. Without one the FlowField sums the
// entries; when the source table has SIFT keys that is reported, as it is likely a
// removed sum index field.
func resolveFlowFieldSIFT(def *TableDef, byStruct map[string]*TableDef) {
	for i := range def.Table.Fields {
		f := &def.Table.Fields[i]
		f.SIFTKey, f.SIFTKeyFiltered, f.FilterRefs = "", "", nil
		for _, ff := range f.FlowFilters {
			if ff.Type == "filter" {
				f.FilterRefs = append(f.FilterRefs, ff)
			}
		}
		if !f.FlowField || (f.CalcFormula != "Sum" && f.CalcFormula != "Count" && f.CalcFormula != "Exist") {
			continue
		}
		source, ok := byStruct[toPascalCase(f.SourceTable)]
		if !ok {
			continue
		}
		// Without FlowFilters set: the fixed (const/field) filters; with: all filter fields
		var fixed, all []string
		for _, ff := range f.FlowFilters {
			all = append(all, ff.Field)
			if ff.Type != "filter" {
				fixed = append(fixed, ff.Field)
			}
		}
		var hasSIFT bool
		f.SIFTKey, hasSIFT = pickSIFTKey(source, f, fixed)
		if f.SIFTKey == "" && hasSIFT {
			fmt.Printf("⚠ %s.%s: no SIFT key of %s covers it, sums the entries\n", def.Table.Name, f.Name, source.Table.Name)
		}
		if len(f.FilterRefs) > 0 {
			f.SIFTKeyFiltered, _ = pickSIFTKey(source, f, all)
			if f.SIFTKeyFiltered == "" && hasSIFT {
				fmt.Printf("⚠ %s.%s: no SIFT key of %s covers it with its FlowFilters, sums the entries when one is set\n", def.Table.Name, f.Name, source.Table.Name)
			}
		}
	}
}

// pickSIFTKey returns the SIFT key of source with the fewest fields that contains all
// filterFields and (Sum) sums f's source field; hasSIFT reports whether source has any.
func pickSIFTKey(source *TableDef, f *Field, filterFields []string) (best string, hasSIFT bool) {
	bestLen := 0
	for _, k := range source.Table.Keys {
		if len(k.SumIndexFields) == 0 {
			continue
		}
		hasSIFT = true
		if f.CalcFormula == "Sum" && !slices.Contains(k.SumIndexFields, f.SourceField) {
			continue
		}
		covers := true
		for _, field := range filterFields {
			if !slices.Contains(k.Fields, field) {
				covers = false
				break
			}
		}
		if covers && (best == "" || len(k.Fields) < bestLen) {
			best, bestLen = k.Name, len(k.Fields)
		}
	}
	return best, hasSIFT
}

// rangeCheck is the Go call checking a field's range (validation: min/max): decimals compare
// as decimals; one bound may be missing
func rangeCheck(structName string, f Field) string {
	bound := func(v interface{}) string {
		if v == nil {
			return ""
		}
		return defaultText(v)
	}
	min, max := bound(f.Validation.Min), bound(f.Validation.Max)
	target := fmt.Sprintf("%sTableName, %q, t.%s", structName, f.Name, upperFirst(f.Name))
	switch {
	case f.Type == "types.Decimal":
		return fmt.Sprintf("tables.CheckDecimalRange(%s, %q, %q)", target, min, max)
	case min != "" && max != "":
		return fmt.Sprintf("tables.CheckRange(%s, %s, %s)", target, min, max)
	case min != "":
		return fmt.Sprintf("tables.CheckMin(%s, %s)", target, min)
	default:
		return fmt.Sprintf("tables.CheckMax(%s, %s)", target, max)
	}
}

// sqlRangeCheck is the SQL CHECK of a whole-number field with both bounds (decimals are
// checked in Go only: SQLite stores them as text, where "5" <= "100" is false)
func sqlRangeCheck(f Field) string {
	if f.Validation == nil || f.Validation.Min == nil || f.Validation.Max == nil || f.Type == "types.Decimal" {
		return ""
	}
	return fmt.Sprintf(" CHECK (%s >= %s AND %s <= %s)", f.DBName, defaultText(f.Validation.Min), f.DBName, defaultText(f.Validation.Max))
}

// validateFieldProperties checks the field properties of the classic table: validation
// (ranges) on numbers only, decimal_places "min:max" on decimals, editable: false not on the
// primary key, data_caption_fields naming stored fields
func validateFieldProperties(def *TableDef) error {
	byName := map[string]Field{}
	for _, f := range def.Table.Fields {
		byName[f.Name] = f
		if v := f.Validation; v != nil {
			if v.Min == nil && v.Max == nil {
				return fmt.Errorf("field %s: validation needs min, max or both", f.Name)
			}
			switch f.Type {
			case "int", "int64", "float64", "types.Decimal":
			default:
				return fmt.Errorf("field %s: validation (min/max) is for numbers, not %s", f.Name, f.Type)
			}
			if v.Min != nil && v.Max != nil {
				lo, err1 := strconv.ParseFloat(defaultText(v.Min), 64)
				hi, err2 := strconv.ParseFloat(defaultText(v.Max), 64)
				if err1 != nil || err2 != nil || lo > hi {
					return fmt.Errorf("field %s: validation min %v / max %v", f.Name, v.Min, v.Max)
				}
			}
		}
		if f.DecimalPlaces != "" {
			lo, hi, ok := strings.Cut(f.DecimalPlaces, ":")
			a, err1 := strconv.Atoi(lo)
			b, err2 := strconv.Atoi(hi)
			if f.Type != "types.Decimal" || !ok || err1 != nil || err2 != nil || a < 0 || a > b {
				return fmt.Errorf("field %s: decimal_places %q needs a decimal field and \"min:max\" (e.g. \"2:5\")", f.Name, f.DecimalPlaces)
			}
		}
	}
	for _, name := range def.Table.DataCaptionFields {
		if f, ok := byName[name]; !ok || f.FlowField {
			return fmt.Errorf("data_caption_fields: %q is not a stored field", name)
		}
	}
	return nil
}

// dbNameOf is the column of a field of a table (its db_name, else its name)
func dbNameOf(def *TableDef, name string) string {
	for _, f := range def.Table.Fields {
		if f.Name == name {
			if f.DBName != "" {
				return f.DBName
			}
			return f.Name
		}
	}
	return name
}

// validateRelationConditions checks conditional table relations: each condition names an
// option or integer field of the table, a value, and a related table and field; no two
// conditions share a value
func validateRelationConditions(def *TableDef) error {
	byName := map[string]Field{}
	for _, f := range def.Table.Fields {
		byName[f.Name] = f
	}
	for _, f := range def.Table.Fields {
		if f.TableRelation == nil || len(f.TableRelation.Conditions) == 0 {
			continue
		}
		if f.TableRelation.Table != "" {
			return fmt.Errorf("field %s: table_relation has conditions and a table — the table goes into each condition", f.Name)
		}
		seen := map[int]bool{}
		for _, c := range f.TableRelation.Conditions {
			cf, ok := byName[c.When.Field]
			if !ok || cf.FlowField || cf.FlowFilter || (cf.Type != "Option" && cf.Type != "int") {
				return fmt.Errorf("field %s: relation condition on %q, which is not an option or integer field of the table", f.Name, c.When.Field)
			}
			if c.Table == "" || c.Field == "" {
				return fmt.Errorf("field %s: relation condition %s = %d needs a table and a field", f.Name, c.When.Field, c.When.Equal)
			}
			if seen[c.When.Equal] {
				return fmt.Errorf("field %s: two relation conditions for %s = %d", f.Name, c.When.Field, c.When.Equal)
			}
			seen[c.When.Equal] = true
		}
	}
	return nil
}

// defaultText is a YAML default as text: decimals without exponent (0.00001, not 1e-05)
func defaultText(v interface{}) string {
	switch x := v.(type) {
	case float64:
		return strconv.FormatFloat(x, 'f', -1, 64)
	case float32:
		return strconv.FormatFloat(float64(x), 'f', -1, 32)
	}
	return fmt.Sprint(v)
}

// goDefault is the Go expression of a field's YAML default (the classic InitValue):
// decimals, text and code values need their types' constructors
func goDefault(f Field) string {
	switch f.Type {
	case "types.Decimal":
		return fmt.Sprintf("types.MustDecimal(%q)", defaultText(f.Default))
	case "types.Text":
		return fmt.Sprintf("types.NewText(%q)", defaultText(f.Default))
	case "types.Code":
		return fmt.Sprintf("types.NewCode(%q)", defaultText(f.Default))
	case "string":
		return fmt.Sprintf("%q", defaultText(f.Default))
	}
	return defaultText(f.Default)
}

// sqlDefault is the SQL DEFAULT value of a field's YAML default: text and codes quoted
func sqlDefault(f Field) string {
	switch f.Type {
	case "types.Text", "types.Code", "string":
		return "'" + strings.ReplaceAll(defaultText(f.Default), "'", "''") + "'"
	}
	return defaultText(f.Default)
}

// fileExists checks if a file exists
func fileExists(filename string) bool {
	_, err := os.Stat(filename)
	return !os.IsNotExist(err)
}

// generateFile writes a template's output to filename, formatted by go/format
func generateFile(filename, name, text string, data any) error {
	tmpl, err := template.New(name).Funcs(templateFuncs()).Parse(text)
	if err != nil {
		return err
	}
	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, data); err != nil {
		return err
	}
	src, err := format.Source(buf.Bytes())
	if err != nil {
		return fmt.Errorf("format: %w", err)
	}
	return os.WriteFile(filename, src, 0644)
}

// getTableNameExpr returns the Go code expression for getting the table name
// For global tables: just the table name constant
// For company tables: company prefix + table name
func getTableNameExpr(isGlobal bool, structName, companyVar string) string {
	if isGlobal {
		return fmt.Sprintf("%sTableName", structName)
	}
	return fmt.Sprintf(`fmt.Sprintf("%%s$%%s", %s, %sTableName)`, companyVar, structName)
}

// templateFuncs returns template helper functions
func templateFuncs() template.FuncMap {
	return template.FuncMap{
		"upperFirst": upperFirst,
		// keyArg: the SQL argument of a field's value — a primary key date goes through
		// types.KeyDate (a blank key date is stored as types.BlankKeyDate, never NULL)
		"keyArg": func(f Field, expr string) string {
			if f.PrimaryKey && f.Type == "types.Date" {
				return "types.KeyDate(" + expr + ")"
			}
			return expr
		},
		"lowerFirst":    lowerFirst,
		"sqlType":       getSQLType,
		"goDefault":     goDefault,
		"rangeCheck":    rangeCheck,
		"sqlRangeCheck": sqlRangeCheck,
		"readOnly":      func(f Field) bool { return f.Editable != nil && !*f.Editable },
		"dbFieldName": func(fields []Field, name string) string {
			for _, f := range fields {
				if f.Name == name {
					if f.DBName != "" {
						return f.DBName
					}
					return f.Name
				}
			}
			return name
		},
		"sqlDefault":         sqlDefault,
		"postgresSqlType":    getPostgresSQLType,
		"tableName":          getTableNameExpr,
		"isLast":             isLast,
		"isLastPK":           isLastPK,
		"isLastDBField":      isLastDBField,
		"hasSuffix":          strings.HasSuffix,
		"join":               strings.Join,
		"sub":                func(a, b int) int { return a - b },
		"sanitizeIdentifier": sanitizeIdentifier,
		// optionIdent: the constant name of option i; a name an earlier option already has
		// gets the option's number ("Qty. Disc. + …" and "Qty. Disc. * …")
		"optionIdent": func(options []string, i int) string {
			name := sanitizeIdentifier(options[i])
			for j := 0; j < i; j++ {
				if sanitizeIdentifier(options[j]) == name {
					return fmt.Sprintf("%s%d", name, i)
				}
			}
			return name
		},
		// fieldGoType: the Go type of a field (an option field has its own named type)
		"fieldGoType": func(structName string, f Field) string {
			if f.Type == "Option" {
				return structName + upperFirst(f.Name)
			}
			return f.Type
		},
		"pkCount":           countPrimaryKeys,
		"firstPK":           getFirstPK,
		"toPascalCase":      toPascalCase,
		"lower":             strings.ToLower,
		"tableNameVar":      getTableNameVarCode,
		"filterSetExpr":     filterSetExpr,
		"flowFilterKind":    func(f Field) string { k, _ := flowFilterKind(f); return k },
		"maxEncryptedPlain": maxEncryptedPlain,
	}
}

// filterSetExpr is the Go condition "one of these FlowFilters has a value"
func filterSetExpr(refs []FlowFilter) string {
	var conds []string
	for _, r := range refs {
		conds = append(conds, fmt.Sprintf("t.%s != \"\"", r.FilterField))
	}
	return strings.Join(conds, " || ")
}

// getTableNameVarCode returns the Go code to set the tableName variable
// For global tables: tableName := StructNameTableName
// For company tables: tableName := fmt.Sprintf("%s$%s", t.company, StructNameTableName)
func getTableNameVarCode(isGlobal bool, structName, companyVar string) string {
	if isGlobal {
		return fmt.Sprintf("tableName := %sTableName", structName)
	}
	return fmt.Sprintf(`tableName := fmt.Sprintf("%%s$%%s", %s, %sTableName)`, companyVar, structName)
}

// countPrimaryKeys counts the number of primary key fields
func countPrimaryKeys(fields []Field) int {
	count := 0
	for _, f := range fields {
		if f.PrimaryKey {
			count++
		}
	}
	return count
}

// getFirstPK returns the first primary key field (or nil if none)
func getFirstPK(fields []Field) *Field {
	for i := range fields {
		if fields[i].PrimaryKey {
			return &fields[i]
		}
	}
	return nil
}

// Helper functions

func toPascalCase(s string) string {
	// Remove special characters and split by spaces
	s = strings.ReplaceAll(s, "-", " ")
	s = strings.ReplaceAll(s, "_", " ")
	words := strings.Fields(s)

	for i, word := range words {
		words[i] = upperFirst(word)
	}

	return strings.Join(words, "")
}

func sanitizeIdentifier(s string) string {
	// Convert option values like "Credit Memo", "G/L Account", "Post Code+City" into valid Go
	// identifiers: every character that is not a letter or digit separates words
	// ("G/L Account" -> "GLAccount", "Post Code+City" -> "PostCodeCity")
	words := strings.FieldsFunc(s, func(r rune) bool { return !unicode.IsLetter(r) && !unicode.IsDigit(r) })
	for i, word := range words {
		words[i] = upperFirst(word)
	}

	result := strings.Join(words, "")

	// Handle blank option (empty string or single space)
	if result == "" || s == " " {
		return "Blank"
	}
	// An identifier cannot start with a digit ("30 Days" -> "N30Days")
	if unicode.IsDigit(rune(result[0])) {
		result = "N" + result
	}

	return result
}

func toSnakeCase(s string) string {
	var result strings.Builder
	for i, r := range s {
		if i > 0 && r >= 'A' && r <= 'Z' {
			result.WriteRune('_')
		}
		result.WriteRune(r)
	}
	return strings.ToLower(result.String())
}

func upperFirst(s string) string {
	if s == "" {
		return ""
	}
	return strings.ToUpper(s[:1]) + s[1:]
}

// lowerFirst is a variable or parameter name for a field or type: a Go keyword (a key field
// named Type) gets a trailing underscore
func lowerFirst(s string) string {
	if s == "" {
		return ""
	}
	name := strings.ToLower(s[:1]) + s[1:]
	if token.IsKeyword(name) {
		name += "_"
	}
	return name
}

func isLast(index int, slice []Field) bool {
	return index == len(slice)-1
}

func isLastPK(index int, slice []Field) bool {
	// Check if this field is a PK and if there are any more PK fields after it
	if !slice[index].PrimaryKey {
		return false
	}

	// Look for any PK fields after this index
	for i := index + 1; i < len(slice); i++ {
		if slice[i].PrimaryKey {
			return false // Found another PK after this one
		}
	}

	return true // This is the last PK field
}

func isLastDBField(index int, slice []Field) bool {
	// Check if this is the last non-FlowField
	// Look for any more non-FlowFields after this index
	for i := index + 1; i < len(slice); i++ {
		if !slice[i].FlowField {
			return false // Found another DB field after this one
		}
	}

	return true // This is the last DB field
}

func getSQLType(f Field) string {
	switch f.Type {
	case "types.Code", "types.Text", "string":
		if f.Length > 0 {
			return fmt.Sprintf("TEXT(%d)", f.Length)
		}
		return "TEXT"
	case "int", "int64":
		return "INTEGER"
	case "float64":
		return "REAL"
	case "bool":
		return "INTEGER"
	case "time.Time":
		return "TEXT"
	case "Option":
		return "INTEGER"
	case "types.Decimal":
		return "TEXT" // Store as TEXT for exact decimal representation
	case "types.Date":
		return "TEXT" // Store as TEXT: "YYYY-MM-DD", a closing date "YYYY-MM-DD 23:59:59"
	case "types.DateTime":
		return "TEXT" // Store as TEXT in ISO 8601 format
	case "BLOB", "[]byte":
		return "BLOB"
	default:
		return "TEXT"
	}
}

func getPostgresSQLType(f Field) string {
	switch f.Type {
	case "types.Code", "types.Text", "string":
		if f.Length > 0 {
			return fmt.Sprintf("VARCHAR(%d)", f.Length)
		}
		return "TEXT"
	case "int", "int64":
		return "INTEGER"
	case "float64":
		return "DOUBLE PRECISION"
	case "bool":
		return "BOOLEAN"
	case "time.Time":
		return "TIMESTAMP"
	case "Option":
		return "INTEGER"
	case "types.Decimal":
		return "NUMERIC" // PostgreSQL NUMERIC for exact decimal representation
	case "types.Date":
		return "TIMESTAMP" // a closing date is the date at 23:59:59 (types.Date)
	case "types.DateTime":
		return "TIMESTAMP"
	case "BLOB", "[]byte":
		return "BYTEA"
	default:
		return "TEXT"
	}
}

func getSQLConstraints(f Field) string {
	var constraints []string

	if f.PrimaryKey {
		constraints = append(constraints, "PRIMARY KEY")
	}

	if f.Required && !f.PrimaryKey {
		constraints = append(constraints, "NOT NULL")
	}

	if f.Validation != nil {
		if f.Validation.Min != nil && f.Validation.Max != nil {
			constraints = append(constraints, fmt.Sprintf("CHECK (%s >= %v AND %s <= %v)",
				f.DBName, f.Validation.Min, f.DBName, f.Validation.Max))
		}
	}

	// Option fields need CHECK constraint for valid range
	if f.Type == "Option" && len(f.Options) > 0 {
		maxValue := len(f.Options) - 1
		constraints = append(constraints, fmt.Sprintf("CHECK (%s >= 0 AND %s <= %d)",
			f.DBName, f.DBName, maxValue))
	}

	if f.Default != nil {
		constraints = append(constraints, "DEFAULT "+sqlDefault(f))
	}

	if f.AutoTimestamp {
		constraints = append(constraints, "DEFAULT CURRENT_TIMESTAMP")
	}

	if len(constraints) > 0 {
		return " " + strings.Join(constraints, " ")
	}
	return ""
}

// Templates

const boilerplateTemplate = `// Code generated by tablegen. DO NOT EDIT.

// Table {{ .Table.ID }} {{ .Table.Name }}, generated from definitions/tables.
//
// The business logic is in business-logic/tables: a type registered with
// Register{{ .StructName }}Logic, with the triggers it needs as methods. Add a trigger when it
// is needed. Available triggers:
//
//	OnInsert(rec *gtables.{{ .StructName }}) error
//	OnModify(rec *gtables.{{ .StructName }}) error
//	OnDelete(rec *gtables.{{ .StructName }}) error
//	OnRename(rec *gtables.{{ .StructName }}) error  — a primary key field changed (rec.OldValue: xRec)
{{- range .Table.Fields }}
{{- if not .FlowField }}
//	OnValidate_{{ upperFirst .Name }}(rec *gtables.{{ $.StructName }}) error
{{- end }}
{{- end }}
{{- define "logic" }}
// ========================================
// Business logic (triggers)
// ========================================

// Triggers of {{ .Table.Name }}: a business logic type implements the ones it needs
type (
	{{ .StructName }}OnInsert interface{ OnInsert(rec *{{ .StructName }}) error }
	{{ .StructName }}OnModify interface{ OnModify(rec *{{ .StructName }}) error }
	{{ .StructName }}OnDelete interface{ OnDelete(rec *{{ .StructName }}) error }
	{{ .StructName }}OnRename interface{ OnRename(rec *{{ .StructName }}) error }
{{- range .Table.Fields }}
{{- if not .FlowField }}
	{{ $.StructName }}OnValidate_{{ upperFirst .Name }} interface{ OnValidate_{{ upperFirst .Name }}(rec *{{ $.StructName }}) error }
{{- end }}
{{- end }}
)

// {{ lowerFirst .StructName }}Logic is the registered business logic of {{ .Table.Name }}, in
// registration order (the base application first, then extensions)
var {{ lowerFirst .StructName }}Logic []any

// Register{{ .StructName }}Logic adds business logic to {{ .Table.Name }}: a value whose methods are
// triggers (OnInsert, OnModify, OnDelete, OnRename, OnValidate_<Field>). Every registered logic
// runs, in registration order; the first error stops the operation. Call it from an init
// function. It panics on a method named like a trigger that the table does not have (a
// misspelled trigger, or a field renamed or removed in the definition), so the program
// does not start with a trigger that would never run.
func Register{{ .StructName }}Logic(logic any) {
	checkLogic({{ .StructName }}TableName, "{{ .StructName }}", logic, {{ lowerFirst .StructName }}Trigger)
	{{ lowerFirst .StructName }}Logic = append({{ lowerFirst .StructName }}Logic, logic)
}

// {{ lowerFirst .StructName }}Trigger reports whether name is a trigger of {{ .Table.Name }} and
// whether logic implements it with the right signature.
func {{ lowerFirst .StructName }}Trigger(name string, logic any) (known, ok bool) {
	switch name {
	case "OnInsert":
		_, ok = logic.({{ .StructName }}OnInsert)
	case "OnModify":
		_, ok = logic.({{ .StructName }}OnModify)
	case "OnDelete":
		_, ok = logic.({{ .StructName }}OnDelete)
	case "OnRename":
		_, ok = logic.({{ .StructName }}OnRename)
{{- range .Table.Fields }}
{{- if not .FlowField }}
	case "OnValidate_{{ upperFirst .Name }}":
		_, ok = logic.({{ $.StructName }}OnValidate_{{ upperFirst .Name }})
{{- end }}
{{- end }}
	default:
		return false, false
	}
	return true, ok
}

// SetLogic replaces the registered business logic for this record only (tests); no
// arguments runs no triggers at all.
func (t *{{ .StructName }}) SetLogic(logic ...any) {
	for _, l := range logic {
		checkLogic({{ .StructName }}TableName, "{{ .StructName }}", l, {{ lowerFirst .StructName }}Trigger)
	}
	if logic == nil {
		logic = []any{}
	}
	t.logic = logic
}

func (t *{{ .StructName }}) registeredLogic() []any {
	if t.logic != nil {
		return t.logic
	}
	return {{ lowerFirst .StructName }}Logic
}

// runOnInsert runs the OnInsert triggers, then the field checks of the definition
func (t *{{ .StructName }}) runOnInsert() error {
{{- range .Table.Fields }}
{{- if .AutoTimestamp }}
	t.{{ upperFirst .Name }} = time.Now()
{{- end }}
{{- end }}
	for _, l := range t.registeredLogic() {
		if trg, ok := l.({{ .StructName }}OnInsert); ok {
			if err := trg.OnInsert(t); err != nil {
				return err
			}
		}
	}
	return t.CheckFields()
}

// runOnModify runs the OnModify triggers, then the field checks of the definition
func (t *{{ .StructName }}) runOnModify() error {
{{- range .Table.Fields }}
{{- if .AutoTimestamp }}
	t.{{ upperFirst .Name }} = time.Now()
{{- end }}
{{- end }}
	for _, l := range t.registeredLogic() {
		if trg, ok := l.({{ .StructName }}OnModify); ok {
			if err := trg.OnModify(t); err != nil {
				return err
			}
		}
	}
	return t.CheckFields()
}

func (t *{{ .StructName }}) runOnDelete() error {
	for _, l := range t.registeredLogic() {
		if trg, ok := l.({{ .StructName }}OnDelete); ok {
			if err := trg.OnDelete(t); err != nil {
				return err
			}
		}
	}
	return nil
}

func (t *{{ .StructName }}) runOnRename() error {
	for _, l := range t.registeredLogic() {
		if trg, ok := l.({{ .StructName }}OnRename); ok {
			if err := trg.OnRename(t); err != nil {
				return err
			}
		}
	}
	return nil
}

// CheckFields checks the field rules of the definition (required, length, validation
// range). Insert and Modify run it after the OnInsert/OnModify triggers.
func (t *{{ .StructName }}) CheckFields() error {
{{- range .Table.Fields }}
{{- if not (or .FlowField .Encrypted) }}
{{- if .Required }}
	{{- if or (eq .Type "types.Code") (eq .Type "types.Text") }}
	if err := tables.CheckRequired({{ $.StructName }}TableName, "{{ .Name }}", t.{{ upperFirst .Name }}.IsEmpty()); err != nil {
		return err
	}
	{{- else if eq .Type "string" }}
	if err := tables.CheckRequired({{ $.StructName }}TableName, "{{ .Name }}", t.{{ upperFirst .Name }} == ""); err != nil {
		return err
	}
	{{- end }}
{{- end }}
{{- if and .Length (or (eq .Type "types.Code") (eq .Type "types.Text") (eq .Type "string")) }}
	if err := tables.CheckMaxLength({{ $.StructName }}TableName, "{{ .Name }}", string(t.{{ upperFirst .Name }}), {{ .Length }}); err != nil {
		return err
	}
{{- end }}
{{- if .DateFormula }}
	if err := tables.CheckDateFormula({{ $.StructName }}TableName, "{{ .Name }}", string(t.{{ upperFirst .Name }})); err != nil {
		return err
	}
{{- end }}
{{- if .Validation }}
	if err := {{ rangeCheck $.StructName . }}; err != nil {
		return err
	}
{{- end }}
{{- end }}
{{- end }}
	return nil
}
{{- end }}
{{- define "flowSource" }}
	// Source: the SIFT totals table of a key covering the filters, else the entries
	tableName, useSIFT := fmt.Sprintf("%s$%s", t.company, {{ toPascalCase .SourceTable }}TableName), false
{{- if .SIFTKey }}
	tableName, useSIFT = sift.TableName(t.company, {{ toPascalCase .SourceTable }}TableName, "{{ .SIFTKey }}"), true
{{- end }}
{{- if .FilterRefs }}
	if {{ filterSetExpr .FilterRefs }} {
		// A FlowFilter is set: the key must contain the filtered fields too
{{- if .SIFTKeyFiltered }}
		tableName, useSIFT = sift.TableName(t.company, {{ toPascalCase .SourceTable }}TableName, "{{ .SIFTKeyFiltered }}"), true
{{- else }}
		tableName, useSIFT = fmt.Sprintf("%s$%s", t.company, {{ toPascalCase .SourceTable }}TableName), false
{{- end }}
	}
{{- end }}
	_ = useSIFT
{{- end }}
{{- define "flowFilterWhere" }}
{{- range .FlowFilters }}
{{- if eq .Type "filter" }}
	if t.{{ .FilterField }} != "" {
{{- if .UpperLimit }}
		// The upper end of FlowFilter {{ .Value }} applied to {{ .Field }} (UpperLimit)
		if clause, filterArgs, _ := flowfilter.Clause("{{ .Field }}", flowfilter.{{ .Kind }}, flowfilter.UpperLimit(flowfilter.{{ .Kind }}, t.{{ .FilterField }})); clause != "" {
{{- else }}
		// FlowFilter {{ .Value }} applied to {{ .Field }} (validated by SetFlowFilter)
		if clause, filterArgs, _ := flowfilter.Clause("{{ .Field }}", flowfilter.{{ .Kind }}, t.{{ .FilterField }}); clause != "" {
{{- end }}
			whereClauses = append(whereClauses, clause)
			args = append(args, filterArgs...)
		}
	}
{{- end }}
{{- end }}
{{- end }}

package {{ .PackageName }}

import (
	"database/sql"
{{- if .HasBlobField }}
	"encoding/base64"
{{- end }}
{{- if .EncryptedFields }}
	"errors"
{{- end }}
	"fmt"
	"slices"
{{- if .HasIntField }}
	"strconv"
{{- end }}
	"strings"
{{- if or .HasTimeField .HasDateField .HasDateTimeField }}
	"time"
{{- end }}

	"github.com/hansjlachmann/openerp/backend/foundation/database"
	apperrors "github.com/hansjlachmann/openerp/backend/foundation/errors"
	"github.com/hansjlachmann/openerp/backend/foundation/i18n"
{{- if .UsesFlowFilter }}
	"github.com/hansjlachmann/openerp/backend/foundation/flowfilter"
{{- end }}
{{- if .EncryptedFields }}
	"github.com/hansjlachmann/openerp/backend/foundation/secrets"
{{- end }}
{{- if .UsesSIFT }}
	"github.com/hansjlachmann/openerp/backend/foundation/sift"
{{- end }}
	"github.com/hansjlachmann/openerp/backend/foundation/tables"
{{- if or .HasCodeField .HasTextField .HasDecimalField .HasDateField .HasDateTimeField }}
	"github.com/hansjlachmann/openerp/backend/foundation/types"
{{- end }}
)

{{- if .HasOptionField }}

// ========================================
// Option Field Type Definitions
// ========================================
{{- range .Table.Fields }}
{{- if eq .Type "Option" }}

// {{ $.StructName }}{{ upperFirst .Name }} represents the {{ .Name }} option field
type {{ $.StructName }}{{ upperFirst .Name }} int

// String returns the text representation of {{ $.StructName }}{{ upperFirst .Name }}
func (o {{ $.StructName }}{{ upperFirst .Name }}) String() string {
	options := []string{ {{- range $i, $opt := .Options }}{{- if $i }}, {{ end }}"{{ $opt }}"{{- end }} }
	if o >= 0 && int(o) < len(options) {
		return options[o]
	}
	return ""
}

// IsValid checks if the {{ $.StructName }}{{ upperFirst .Name }} value is within valid range
func (o {{ $.StructName }}{{ upperFirst .Name }}) IsValid() bool {
	return o >= 0 && o < {{ len .Options }}
}
{{- end }}
{{- end }}
{{- end }}

// {{ .StructName }} represents Table {{ .Table.ID }}: {{ .Table.Name }}
// (generated from the table definition). Its business logic — the table and field
// triggers — is in business-logic/tables, registered with Register{{ .StructName }}Logic.
type {{ .StructName }} struct {
{{- range .Table.Fields }}
{{- if .FlowField }}
	// FlowField: {{ .CalcFormula }}({{ .SourceTable }}.{{ .SourceField }})
	{{ upperFirst .Name }} {{ .Type }}
{{- else if eq .Type "Option" }}
	{{ upperFirst .Name }} {{ $.StructName }}{{ upperFirst .Name }} ` + "`db:\"{{ .DBName }}{{if .PrimaryKey}},pk{{end}}\"`" + `
{{- else }}
	{{ upperFirst .Name }} {{ .Type }} ` + "`db:\"{{ .DBName }}{{if .PrimaryKey}},pk{{end}}\"`" + `
{{- end }}
{{- end }}

{{- range .FlowFilterFields }}
	// FlowFilter {{ .Name }} ({{ .Type }}): the filter expression FlowFields apply — not stored
	{{ upperFirst .Name }} string
{{- end }}

	// Internal context (set by Init)
	db      database.Executor
	company string
	dbType  database.DBType

	// Field tracking for optimal Modify() operations
	oldValues map[string]interface{} // Stores original values from Get()

	// Filter state for SetRange/FindFirst/FindLast: the current filter group's filters
	filters map[string]*{{ lowerFirst .StructName }}FilterCondition
	// FilterGroup: the current filter group and the filters of the other groups; the
	// filters of all groups apply together
	filterGroup  int
	otherFilters map[int]map[string]*{{ lowerFirst .StructName }}FilterCondition
	// Find(">") / Find("<"): the condition for the record after/before the current one
	seek     string
	seekArgs []interface{}
	// The records selected on the list a codeunit was run from (SetSelectionFilter)
	selection map[string][]interface{}
	// Temporary: the record's own table is in a private in-memory database (InitTemporary)
	temp *tables.TemporaryStore

	// Iteration state for FindSet/Next
	currentRows *sql.Rows

	// LockTable: the following reads lock the rows they read (SELECT … FOR UPDATE, Postgres)
	lockForUpdate bool

	// Mark / MarkedOnly: the marked records (primary key values by key text)
	marks      map[string][]interface{}
	markedOnly bool
	orderByFields []string
	descending    bool // sort order of the current key (SetAscending)

	// Free-text search (SetSearch): rows where any of the columns contains the text
	searchColumns []string
	searchText    string

	// Pagination window for FindSet/FindSetBuffered (0 limit = all rows)
	limit  int
	offset int

	// Buffered recordset for bidirectional navigation
	bufferedRecords []*{{ .StructName }}
	currentBufferPos int

	// Business logic of this record only (SetLogic, tests); nil = the registered logic
	// (Register{{ .StructName }}Logic)
	logic []any

	// Error returned by the OnInsert/OnModify/OnDelete trigger of the last
	// Insert/Modify/Delete call (nil if the trigger passed or did not run)
	triggerErr error
}

const {{ .StructName }}TableID = {{ .Table.ID }}
const {{ .StructName }}TableName = "{{ .Table.Name }}"

{{- if .HasOptionField }}

// ========================================
// Option Field Namespaces
// ========================================
{{- range .Table.Fields }}
{{- if eq .Type "Option" }}

// {{ $.StructName }}_{{ upperFirst .Name }} provides named constants for the {{ .Name }} option field (FieldName.OptionValue syntax)
var {{ $.StructName }}_{{ upperFirst .Name }} = struct {
{{- $fieldName := .Name }}
{{- $opts := .Options }}
{{- range $i, $opt := .Options }}
	{{ optionIdent $opts $i }}    {{ $.StructName }}{{ upperFirst $fieldName }}
{{- end }}
}{
{{- range $i, $opt := .Options }}
	{{ optionIdent $opts $i }}:    {{ $i }},
{{- end }}
}
{{- end }}
{{- end }}
{{- end }}

// ========================================
// Field numbers (field IDs)
// ========================================

// {{ .StructName }}_FieldNo holds the field numbers of {{ .Table.Name }} (FieldNo):
// {{ .StructName }}_FieldNo.{{ upperFirst (index .Table.Fields 0).Name }} is {{ (index .Table.Fields 0).No }}
var {{ .StructName }}_FieldNo = struct {
{{- range .Table.Fields }}
	{{ upperFirst .Name }} int
{{- end }}
{{- range .FlowFilterFields }}
	{{ upperFirst .Name }} int
{{- end }}
}{
{{- range .Table.Fields }}
	{{ upperFirst .Name }}: {{ .No }},
{{- end }}
{{- range .FlowFilterFields }}
	{{ upperFirst .Name }}: {{ .No }},
{{- end }}
}

// FieldNo returns the number of a field (FieldNo), 0 for an unknown field
func (t *{{ .StructName }}) FieldNo(fieldName string) int {
	switch strings.ToLower(fieldName) {
{{- range .Table.Fields }}
	case "{{ .DBName }}":
		return {{ .No }}
{{- end }}
{{- range .FlowFilterFields }}
	case "{{ .DBName }}":
		return {{ .No }}
{{- end }}
	}
	return 0
}

// FieldName returns the name of the field with a number, "" for an unknown number
func (t *{{ .StructName }}) FieldName(fieldNo int) string {
	switch fieldNo {
{{- range .Table.Fields }}
	case {{ .No }}:
		return "{{ .DBName }}"
{{- end }}
{{- range .FlowFilterFields }}
	case {{ .No }}:
		return "{{ .DBName }}"
{{- end }}
	}
	return ""
}

// GetTableID returns the table ID (for Object Registry)
func (t *{{ .StructName }}) GetTableID() int {
	return {{ .StructName }}TableID
}

// GetTableName returns the table name
func (t *{{ .StructName }}) GetTableName() string {
	return {{ .StructName }}TableName
}

// GetTableSchema returns the CREATE TABLE schema (SQLite)
func (t *{{ .StructName }}) GetTableSchema() string {
	return Get{{ .StructName }}TableSchema()
}

// GetPostgresTableSchema returns the CREATE TABLE schema (PostgreSQL)
func (t *{{ .StructName }}) GetPostgresTableSchema() string {
	return Get{{ .StructName }}PostgresTableSchema()
}


// TriggerError returns the error that made the last Insert/Modify/Delete fail and is meant
// for the user: the OnInsert/OnModify/OnDelete trigger's, or "<Table> <key> already exists"
// when Insert found the key taken (also a key OnInsert assigned, e.g. from a number series);
// nil if it failed for another reason (database) or succeeded.
func (t *{{ .StructName }}) TriggerError() error {
	return t.triggerErr
}

// GetOrError reads a record by its key (Get) and fails like a classic GET without IF when it
// does not exist: "<Table> <key> not found" in the user's language
func (t *{{ .StructName }}) GetOrError(primaryKey interface{}) error {
	if t.Get(primaryKey) {
		return nil
	}
	key := fmt.Sprint(primaryKey)
	if m, ok := primaryKey.(map[string]interface{}); ok {
		key = strings.Join([]string{ {{- range $i, $f := .Table.Fields }}{{ if $f.PrimaryKey }}fmt.Sprint(m["{{ $f.DBName }}"]){{ if not (isLastPK $i $.Table.Fields) }}, {{ end }}{{ end }}{{ end -}} }, ",")
	}
	return apperrors.TableRecordNotFound({{ .StructName }}TableName, key)
}

// InsertOrError inserts the record (Insert) and fails like a classic INSERT without IF: the
// trigger's error, "<Table> <key> already exists", or "Failed to insert <Table>"
func (t *{{ .StructName }}) InsertOrError(runTrigger bool) error {
	if t.Insert(runTrigger) {
		return nil
	}
	if err := t.TriggerError(); err != nil {
		return err
	}
	return apperrors.TableInsertFailed({{ .StructName }}TableName)
}

// ModifyOrError modifies the record (Modify) and fails like a classic MODIFY without IF: the
// trigger's error or "Failed to modify <Table> <key>"
func (t *{{ .StructName }}) ModifyOrError(runTrigger bool) error {
	if t.Modify(runTrigger) {
		return nil
	}
	if err := t.TriggerError(); err != nil {
		return err
	}
	_, key := t.recordKey()
	return apperrors.TableModifyFailed({{ .StructName }}TableName, key)
}

// DeleteOrError deletes the record (Delete) and fails like a classic DELETE without IF: the
// trigger's error or "Failed to delete <Table> <key>"
func (t *{{ .StructName }}) DeleteOrError(runTrigger bool) error {
	if t.Delete(runTrigger) {
		return nil
	}
	if err := t.TriggerError(); err != nil {
		return err
	}
	_, key := t.recordKey()
	return apperrors.TableDeleteFailed({{ .StructName }}TableName, key)
}

{{ template "logic" . }}

// GetDB returns the database executor (for wrapper access)
func (t *{{ .StructName }}) GetDB() database.Executor {
	return t.db
}

// GetCompany returns the company name (for wrapper access)
func (t *{{ .StructName }}) GetCompany() string {
	return t.company
}

// GetDBType returns the database type (for wrapper access)
func (t *{{ .StructName }}) GetDBType() database.DBType {
	return t.dbType
}

// IsSetupTable reports whether this is a singleton setup table
// (a single record identified by a blank primary key).
func (t *{{ .StructName }}) IsSetupTable() bool {
	return {{ .Table.SetupTable }}
}
{{- if .Table.SetupTable }}

// Get{{ .StructName }} reads the single {{ .Table.Name }} record (classic SETUP.GET) in the database
// and company of another record (from, e.g. the record a trigger works on) — a blank one, with
// the definition's defaults, when it does not exist yet
func Get{{ .StructName }}(from tables.Source) *{{ .StructName }} {
	setup := &{{ .StructName }}{}
	setup.InitWithDBType(from.GetDB(), from.GetCompany(), from.GetDBType())
	if !setup.Get("") {
		setup.InitRecord()
	}
	return setup
}
{{- end }}

// Get{{ .StructName }}TableSchema returns the SQLite schema
func Get{{ .StructName }}TableSchema() string {
	return ` + "`" + `
{{- range $i, $f := .Table.Fields }}
		{{ $f.DBName }} {{ sqlType $f }}{{ if and $f.PrimaryKey (eq (pkCount $.Table.Fields) 1) }} PRIMARY KEY{{ end }}{{ if $f.Required }}{{ if not $f.PrimaryKey }} NOT NULL{{ end }}{{ end }}{{ sqlRangeCheck $f }}{{ if eq $f.Type "Option" }} CHECK ({{ $f.DBName }} >= 0 AND {{ $f.DBName }} <= {{ sub (len $f.Options) 1 }}){{ end }}{{ if $f.Default }} DEFAULT {{ sqlDefault $f }}{{ end }}{{ if $f.AutoTimestamp }} DEFAULT CURRENT_TIMESTAMP{{ end }}{{ if or (not (isLast $i $.Table.Fields)) (gt (pkCount $.Table.Fields) 1) }},{{ end }}
{{- end }}
{{- if gt (pkCount .Table.Fields) 1 }}
		PRIMARY KEY ({{ range $i, $f := .Table.Fields }}{{ if $f.PrimaryKey }}{{ $f.DBName }}{{ if not (isLastPK $i $.Table.Fields) }}, {{ end }}{{ end }}{{ end }})
{{- end }}
	` + "`" + `
}

// Get{{ .StructName }}PostgresTableSchema returns the PostgreSQL schema
func Get{{ .StructName }}PostgresTableSchema() string {
	return ` + "`" + `
{{- range $i, $f := .Table.Fields }}
		{{ $f.DBName }} {{ postgresSqlType $f }}{{ if and $f.PrimaryKey (eq (pkCount $.Table.Fields) 1) }} PRIMARY KEY{{ end }}{{ if $f.Required }}{{ if not $f.PrimaryKey }} NOT NULL{{ end }}{{ end }}{{ sqlRangeCheck $f }}{{ if eq $f.Type "Option" }} CHECK ({{ $f.DBName }} >= 0 AND {{ $f.DBName }} <= {{ sub (len $f.Options) 1 }}){{ end }}{{ if $f.Default }} DEFAULT {{ sqlDefault $f }}{{ end }}{{ if $f.AutoTimestamp }} DEFAULT CURRENT_TIMESTAMP{{ end }}{{ if or (not (isLast $i $.Table.Fields)) (gt (pkCount $.Table.Fields) 1) }},{{ end }}
{{- end }}
{{- if gt (pkCount .Table.Fields) 1 }}
		PRIMARY KEY ({{ range $i, $f := .Table.Fields }}{{ if $f.PrimaryKey }}{{ $f.DBName }}{{ if not (isLastPK $i $.Table.Fields) }}, {{ end }}{{ end }}{{ end }})
{{- end }}
	` + "`" + `
}

// ========================================
// Translation Support (CaptionML)
// ========================================

// GetCaption returns the table caption in the specified language
func (t *{{ .StructName }}) GetCaption(language string) string {
	ts := i18n.GetInstance()
	return ts.TableCaption("{{ .Table.Name }}", language)
}

// GetFieldCaption returns the field caption in the specified language
func (t *{{ .StructName }}) GetFieldCaption(fieldName, language string) string {
	ts := i18n.GetInstance()
	return ts.FieldCaption("{{ .Table.Name }}", fieldName, language)
}
{{- if .HasOptionField }}

// GetOptionCaption returns the option field value caption in the specified language
func (t *{{ .StructName }}) GetOptionCaption(fieldName, optionValue, language string) string {
	ts := i18n.GetInstance()
	return ts.OptionCaption("{{ .Table.Name }}", fieldName, optionValue, language)
}
{{- end }}

// CreateTable creates the {{ .Table.Name }} table for the specified company (SQLite)
// The db parameter can be either *sql.DB or *sql.Tx
func (t *{{ .StructName }}) CreateTable(db database.Executor, company string) error {
	return t.CreateTableWithDBType(db, company, database.DBTypeSQLite)
}

// CreateTableWithDBType creates the {{ .Table.Name }} table for the specified company with the given database type
// The db parameter can be either *sql.DB or *sql.Tx
func (t *{{ .StructName }}) CreateTableWithDBType(db database.Executor, company string, dbType database.DBType) error {
{{- if .Table.Global }}
	tableName := {{ .StructName }}TableName
{{- else }}
	tableName := fmt.Sprintf("%s$%s", company, {{ .StructName }}TableName)
{{- end }}
	var schema string
	if dbType == database.DBTypePostgres {
		schema = Get{{ .StructName }}PostgresTableSchema()
	} else {
		schema = Get{{ .StructName }}TableSchema()
	}

	createSQL := fmt.Sprintf(` + "`CREATE TABLE IF NOT EXISTS \"%s\" (%s)`" + `, tableName, schema)
	_, err := db.Exec(createSQL)
	if err != nil {
		return fmt.Errorf("failed to create {{ .Table.Name }} table: %w", err)
	}

	// Indexes (Keys) and SIFT totals
	return t.SyncKeys(db, company, dbType)
}

// SyncKeys brings an existing table's keys up to date (table sync at startup): creates
// missing indexes and builds, rebuilds or drops the SIFT totals of keys with
// sum_index_fields (sift.Sync; unchanged keys cost one query).
func (t *{{ .StructName }}) SyncKeys(db database.Executor, company string, dbType database.DBType) error {
{{- if .Table.Global }}
	tableName := {{ .StructName }}TableName
	siftCompany := ""
{{- else }}
	tableName := fmt.Sprintf("%s$%s", company, {{ .StructName }}TableName)
	siftCompany := company
{{- end }}
{{- range .Table.Keys }}
	// Index "company$Table$key"; a global table's indexes are shared by all companies:
	// "Table$key" (a company prefix created a copy of every index per company)
	if _, err := db.Exec(fmt.Sprintf(` + "`CREATE INDEX IF NOT EXISTS \"%s\" ON \"%s\" ({{ join .Fields \", \" }})`" + `,
{{- if $.Table.Global }}
		"{{ $.Table.Name }}${{ .Name }}", tableName)); err != nil {
{{- else }}
		fmt.Sprintf("%s${{ $.Table.Name }}${{ .Name }}", company), tableName)); err != nil {
{{- end }}
		return fmt.Errorf("failed to create index {{ .Name }}: %w", err)
	}
{{- end }}
{{- if not .Table.Keys }}
	_ = tableName // no keys: nothing to index
{{- end }}
	var keys []sift.Key
	for _, spec := range {{ lowerFirst .StructName }}SIFTSpecs() {
		keys = append(keys, sift.BuildKey(dbType, siftCompany, {{ .StructName }}TableName, tableName, spec))
	}
	return sift.Sync(db, dbType, siftCompany, {{ .StructName }}TableName, keys)
}

// {{ lowerFirst .StructName }}SIFTSpecs are the table's keys with sum_index_fields
func {{ lowerFirst .StructName }}SIFTSpecs() []sift.KeySpec {
	return []sift.KeySpec{
{{- range .SIFTKeys }}
		{
			Name: "{{ .Name }}",
			Fields: []sift.Column{ {{- range .Fields }}{Name: "{{ .Name }}", Kind: sift.{{ .Kind }}}, {{ end -}} },
			Sums: []sift.Column{ {{- range .Sums }}{Name: "{{ .Name }}", Kind: sift.{{ .Kind }}}, {{ end -}} },
		},
{{- end }}
	}
}

// VerifySIFT compares the table's SIFT totals with its entries (Verify SIFT codeunit); with
// repair it rebuilds the keys whose totals differ.
func (t *{{ .StructName }}) VerifySIFT(repair bool) ([]sift.VerifyResult, error) {
{{- if .Table.Global }}
	tableName, siftCompany := {{ .StructName }}TableName, ""
{{- else }}
	tableName, siftCompany := fmt.Sprintf("%s$%s", t.company, {{ .StructName }}TableName), t.company
{{- end }}
	var results []sift.VerifyResult
	for _, spec := range {{ lowerFirst .StructName }}SIFTSpecs() {
		n, err := sift.VerifyKey(t.db, t.dbType, siftCompany, {{ .StructName }}TableName, tableName, spec)
		if err != nil {
			return results, fmt.Errorf("%s: %w", spec.Name, err)
		}
		result := sift.VerifyResult{Table: {{ .StructName }}TableName, Key: spec.Name, Differences: n}
		if n > 0 && repair {
			if err := sift.RebuildKey(t.db, t.dbType, siftCompany, {{ .StructName }}TableName, tableName, spec); err != nil {
				return results, fmt.Errorf("%s: %w", spec.Name, err)
			}
			result.Rebuilt = true
		}
		results = append(results, result)
	}
	return results, nil
}

// ========================================
// Record Methods
// ========================================

// InitWithDBType initializes a new {{ .StructName }} record with database context and type.
// db can be *sql.DB or *sql.Tx. There is no Init without the type: the type decides the SQL
// placeholders, and a default (SQLite) silently broke code on PostgreSQL.
func (t *{{ .StructName }}) InitWithDBType(db database.Executor, company string, dbType database.DBType) {
	t.db = db
	t.company = company
	t.dbType = dbType
	t.oldValues = nil // Fresh record, no old values
	t.applyDefaults()
}

// InitRecord initializes a new, not yet inserted record (OnNewRecord).
// The base implementation applies the YAML default values; wrappers can override
// it to supply further defaults (call the base implementation first).
func (t *{{ .StructName }}) InitRecord() {
	t.applyDefaults()
}

// applyDefaults assigns the YAML default values and auto timestamps
func (t *{{ .StructName }}) applyDefaults() {
{{- range .Table.Fields }}
{{- if .AutoTimestamp }}
	t.{{ upperFirst .Name }} = time.Now()
{{- else if .Default }}
	t.{{ upperFirst .Name }} = {{ goDefault . }}
{{- end }}
{{- end }}
}

// StoreOldValues stores current field values for change detection
// Call this after loading a record from the database
func (t *{{ .StructName }}) StoreOldValues() {
	t.oldValues = make(map[string]interface{})
{{- range .Table.Fields }}
{{- if not .FlowField }}
	t.oldValues["{{ .DBName }}"] = t.{{ upperFirst .Name }}
{{- end }}
{{- end }}
}

// OldValue returns a field's value as last read from or written to the database (
// xRec), e.g. the old key in OnRename; nil when the record was not loaded.
func (t *{{ .StructName }}) OldValue(fieldName string) interface{} {
	return t.oldValues[fieldName]
}

// XRec returns the record as it was last read from or written to the database (classic xRec),
// in the same database and company: xRec := rec.XRec(); if rec.No != xRec.No { … }. For a new
// record (never read) it is a blank record with the definition's defaults.
func (t *{{ .StructName }}) XRec() *{{ .StructName }} {
	x := &{{ .StructName }}{db: t.db, company: t.company, dbType: t.dbType, temp: t.temp}
	x.InitRecord()
{{- range .Table.Fields }}
{{- if not .FlowField }}
	if v, ok := t.oldValues["{{ .DBName }}"].({{ fieldGoType $.StructName . }}); ok {
		x.{{ upperFirst .Name }} = v
	}
{{- end }}
{{- end }}
	return x
}

// New{{ .StructName }} returns a {{ .Table.Name }} record in the database and company of another
// record (from) — a classic Record variable: glAcc := gtables.NewGLAccount(rec)
func New{{ .StructName }}(from tables.Source) *{{ .StructName }} {
	rec := &{{ .StructName }}{}
	rec.InitWithDBType(from.GetDB(), from.GetCompany(), from.GetDBType())
	return rec
}

// IsGlobal reports whether the table is global (no company prefix, e.g. User, Company)
func (t *{{ .StructName }}) IsGlobal() bool {
	return {{ .Table.Global }}
}

// convertPlaceholders converts SQLite-style ? placeholders to PostgreSQL-style $1, $2, etc.
// when running on PostgreSQL
func (t *{{ .StructName }}) convertPlaceholders(sql string, count int) string {
	if t.dbType != database.DBTypePostgres {
		return sql
	}
	result := sql
	for i := 1; i <= count; i++ {
		result = strings.Replace(result, "?", fmt.Sprintf("$%d", i), 1)
	}
	return result
}

// Get retrieves a record from the database by primary key (interface{} for generic API)
// For single primary key: pass the value directly (string, int, etc.)
// For composite keys: pass a map[string]interface{} with field names as keys
func (t *{{ .StructName }}) Get(primaryKey interface{}) bool {
	// Handle composite primary key (map[string]interface{})
	if pkMap, ok := primaryKey.(map[string]interface{}); ok {
{{- range $i, $f := .Table.Fields }}{{- if $f.PrimaryKey }}
		var {{ lowerFirst $f.Name }}Val {{ fieldGoType $.StructName $f }}
		if v, exists := pkMap["{{ $f.DBName }}"]; exists {
{{- if eq $f.Type "types.Code" }}
			switch val := v.(type) {
			case types.Code:
				{{ lowerFirst $f.Name }}Val = val
			case string:
				{{ lowerFirst $f.Name }}Val = types.NewCode(val)
			}
{{- else if eq $f.Type "types.Text" }}
			switch val := v.(type) {
			case types.Text:
				{{ lowerFirst $f.Name }}Val = val
			case string:
				{{ lowerFirst $f.Name }}Val = types.NewText(val)
			}
{{- else if eq $f.Type "int" }}
			switch val := v.(type) {
			case int:
				{{ lowerFirst $f.Name }}Val = val
			case float64:
				{{ lowerFirst $f.Name }}Val = int(val)
			case string:
				// the key part as text: the API's "CUSTOMER1,10000"
				if n, err := strconv.Atoi(strings.TrimSpace(val)); err == nil {
					{{ lowerFirst $f.Name }}Val = n
				}
			}
{{- else if eq $f.Type "int64" }}
			switch val := v.(type) {
			case int64:
				{{ lowerFirst $f.Name }}Val = val
			case int:
				{{ lowerFirst $f.Name }}Val = int64(val)
			case float64:
				{{ lowerFirst $f.Name }}Val = int64(val)
			case string:
				if n, err := strconv.ParseInt(strings.TrimSpace(val), 10, 64); err == nil {
					{{ lowerFirst $f.Name }}Val = n
				}
			}
{{- else if eq $f.Type "string" }}
			if val, ok := v.(string); ok {
				{{ lowerFirst $f.Name }}Val = val
			}
{{- else if eq $f.Type "Option" }}
			switch val := v.(type) {
			case {{ fieldGoType $.StructName $f }}:
				{{ lowerFirst $f.Name }}Val = val
			case int:
				{{ lowerFirst $f.Name }}Val = {{ fieldGoType $.StructName $f }}(val)
			case float64:
				{{ lowerFirst $f.Name }}Val = {{ fieldGoType $.StructName $f }}(int(val))
			case string:
				if n, err := strconv.Atoi(val); err == nil {
					{{ lowerFirst $f.Name }}Val = {{ fieldGoType $.StructName $f }}(n)
				}
			}
{{- else if eq $f.Type "types.Decimal" }}
			switch val := v.(type) {
			case types.Decimal:
				{{ lowerFirst $f.Name }}Val = val
			case int:
				{{ lowerFirst $f.Name }}Val = types.NewDecimalFromInt(int64(val))
			case float64:
				{{ lowerFirst $f.Name }}Val = types.NewDecimal(val)
			case string:
				// the key part as text: the API's "CODE,10"
				{{ lowerFirst $f.Name }}Val, _ = types.NewDecimalFromString(strings.TrimSpace(val))
			}
{{- else if eq $f.Type "types.Date" }}
			switch val := v.(type) {
			case types.Date:
				{{ lowerFirst $f.Name }}Val = val
			case time.Time:
				{{ lowerFirst $f.Name }}Val = types.NewDate(val.Year(), val.Month(), val.Day())
			case string:
				{{ lowerFirst $f.Name }}Val, _ = types.NewDateFromString(val)
			}
{{- end }}
		}
{{- end }}{{- end }}
		return t.GetByPK({{- range $i, $f := .Table.Fields }}{{- if $f.PrimaryKey }}{{ lowerFirst $f.Name }}Val{{ if not (isLastPK $i $.Table.Fields) }}, {{ end }}{{- end }}{{- end }})
	}

{{- if eq (pkCount .Table.Fields) 1 }}
	// Handle single primary key (for tables with only one PK field)
{{- $pk := firstPK .Table.Fields }}
{{- if eq $pk.Type "types.Code" }}
	switch pk := primaryKey.(type) {
	case types.Code:
		return t.GetByPK(pk)
	case string:
		return t.GetByPK(types.NewCode(pk))
	}
{{- else if eq $pk.Type "types.Text" }}
	switch pk := primaryKey.(type) {
	case types.Text:
		return t.GetByPK(pk)
	case string:
		return t.GetByPK(types.NewText(pk))
	}
{{- else if eq $pk.Type "int" }}
	switch pk := primaryKey.(type) {
	case int:
		return t.GetByPK(pk)
	case float64:
		return t.GetByPK(int(pk))
	case string:
		var intVal int
		if _, err := fmt.Sscanf(pk, "%d", &intVal); err == nil {
			return t.GetByPK(intVal)
		}
	}
{{- else if eq $pk.Type "int64" }}
	switch pk := primaryKey.(type) {
	case int64:
		return t.GetByPK(pk)
	case int:
		return t.GetByPK(int64(pk))
	case float64:
		return t.GetByPK(int64(pk))
	case string:
		var intVal int64
		if _, err := fmt.Sscanf(pk, "%d", &intVal); err == nil {
			return t.GetByPK(intVal)
		}
	}
{{- else if eq $pk.Type "string" }}
	if pk, ok := primaryKey.(string); ok {
		return t.GetByPK(pk)
	}
{{- else if eq $pk.Type "types.Date" }}
	switch pk := primaryKey.(type) {
	case types.Date:
		return t.GetByPK(pk)
	case time.Time:
		return t.GetByPK(types.NewDate(pk.Year(), pk.Month(), pk.Day()))
	case string:
		if d, err := types.NewDateFromString(pk); err == nil {
			return t.GetByPK(d)
		}
	}
{{- end }}
{{- else }}
	// For tables with composite keys, direct value is not supported
	// Use a map[string]interface{} with field names as keys
{{- end }}

	fmt.Printf("Error: Invalid primary key type for {{ .Table.Name }}.Get: %T (use map for composite keys)\n", primaryKey)
	return false
}

// GetByPK retrieves a record by its typed primary key(s) - for direct typed access
func (t *{{ .StructName }}) GetByPK({{- range $i, $f := .Table.Fields }}{{- if $f.PrimaryKey }}{{ lowerFirst $f.Name }} {{ fieldGoType $.StructName $f }}{{ if not (isLastPK $i $.Table.Fields) }}, {{ end }}{{- end }}{{- end }}) bool {
{{- if .Table.Global }}
	tableName := {{ .StructName }}TableName
{{- else }}
	tableName := fmt.Sprintf("%s$%s", t.company, {{ .StructName }}TableName)
{{- end }}

	{{- range .Table.Fields }}
	{{- if not .FlowField }}
	{{- if eq .Type "types.Code" }}
	var {{ lowerFirst .Name }}Null sql.NullString
	{{- else if eq .Type "types.Text" }}
	var {{ lowerFirst .Name }}Null sql.NullString
	{{- else if eq .Type "types.Decimal" }}
	var {{ lowerFirst .Name }}Null sql.NullString
	{{- else if eq .Type "types.Date" }}
	var {{ lowerFirst .Name }}Null sql.NullString
	{{- else if eq .Type "types.DateTime" }}
	var {{ lowerFirst .Name }}Null sql.NullString
	{{- else if eq .Type "bool" }}
	var {{ lowerFirst .Name }}Bool sql.NullBool
	{{- else if eq .Type "Option" }}
	var {{ lowerFirst .Name }}Int int
	{{- else }}
	var {{ lowerFirst .Name }}Val {{ .Type }}
	{{- end }}
	{{- end }}
	{{- end }}

	// Collect arguments for query
	args := []interface{}{
		{{- range $i, $f := .Table.Fields }}{{- if $f.PrimaryKey }}
		{{ keyArg $f (lowerFirst $f.Name) }},
		{{- end }}{{- end }}
	}

	// Build SQL with placeholders
	sqlStr := fmt.Sprintf(` + "`SELECT {{ range $i, $f := .Table.Fields }}{{ if not $f.FlowField }}{{ $f.DBName }}{{ if not (isLastDBField $i $.Table.Fields) }}, {{ end }}{{ end }}{{ end }} FROM \"%s\" WHERE 1=1{{ range .Table.Fields }}{{ if .PrimaryKey }} AND {{ .DBName }} = ?{{ end }}{{ end }}`" + `, tableName)

	// Convert placeholders for PostgreSQL
	sqlStr = t.convertPlaceholders(sqlStr, len(args))
	sqlStr += t.forUpdate()

	err := t.exec().QueryRow(sqlStr, args...).Scan(
{{- range $i, $f := .Table.Fields }}
		{{- if not $f.FlowField }}
		{{- if eq $f.Type "types.Code" }}
		&{{ lowerFirst $f.Name }}Null,
		{{- else if eq $f.Type "types.Text" }}
		&{{ lowerFirst $f.Name }}Null,
		{{- else if eq $f.Type "types.Decimal" }}
		&{{ lowerFirst $f.Name }}Null,
		{{- else if eq $f.Type "types.Date" }}
		&{{ lowerFirst $f.Name }}Null,
		{{- else if eq $f.Type "types.DateTime" }}
		&{{ lowerFirst $f.Name }}Null,
		{{- else if eq $f.Type "bool" }}
		&{{ lowerFirst $f.Name }}Bool,
		{{- else if eq $f.Type "Option" }}
		&{{ lowerFirst $f.Name }}Int,
		{{- else }}
		&{{ lowerFirst $f.Name }}Val,
		{{- end }}
		{{- end }}
{{- end }}
	)

	if err != nil {
		if err == sql.ErrNoRows {
			// Record not found - this is not an error, just return false
			return false
		}
		// Actual database error
		fmt.Printf("Error: Failed to get {{ .Table.Name }}: %v\n", err)
		return false
	}

	// Populate fields
{{- range .Table.Fields }}
{{- if not .FlowField }}
{{- if eq .Type "types.Code" }}
	t.{{ upperFirst .Name }} = types.NewCode({{ lowerFirst .Name }}Null.String)
{{- else if eq .Type "types.Text" }}
	t.{{ upperFirst .Name }} = types.NewText({{ lowerFirst .Name }}Null.String)
{{- else if eq .Type "types.Decimal" }}
	t.{{ upperFirst .Name }}, _ = types.NewDecimalFromString({{ lowerFirst .Name }}Null.String)
{{- else if eq .Type "types.Date" }}
	t.{{ upperFirst .Name }}, _ = types.NewDateFromString({{ lowerFirst .Name }}Null.String)
{{- else if eq .Type "types.DateTime" }}
	t.{{ upperFirst .Name }}, _ = types.NewDateTimeFromString({{ lowerFirst .Name }}Null.String)
{{- else if eq .Type "bool" }}
	t.{{ upperFirst .Name }} = {{ lowerFirst .Name }}Bool.Bool
{{- else if eq .Type "Option" }}
	t.{{ upperFirst .Name }} = {{ $.StructName }}{{ upperFirst .Name }}({{ lowerFirst .Name }}Int)
{{- else }}
	t.{{ upperFirst .Name }} = {{ lowerFirst .Name }}Val
{{- end }}
{{- end }}
{{- end }}

	// Store old values for field tracking
	t.StoreOldValues()

	return true
}

// Insert inserts the record into the database
func (t *{{ .StructName }}) Insert(runTrigger bool) bool {
	// OnInsert trigger of the business logic, then the field checks of the definition
	t.triggerErr = nil
	if runTrigger {
		if err := t.runOnInsert(); err != nil {
			fmt.Printf("Error: OnInsert trigger failed: %v\n", err)
			t.triggerErr = err
			return false
		}
	}
{{- if .EncryptedFields }}
	if err := t.encryptSecrets(); err != nil {
		fmt.Printf("Error: %v\n", err)
		t.triggerErr = err
		return false
	}
{{- end }}

{{- if .Table.Global }}
	tableName := {{ .StructName }}TableName
{{- else }}
	tableName := fmt.Sprintf("%s$%s", t.company, {{ .StructName }}TableName)
{{- end }}

	// Collect arguments for INSERT
	args := []interface{}{
{{- range .Table.Fields }}
{{- if not .FlowField }}
		{{ keyArg . (printf "t.%s" (upperFirst .Name)) }},
{{- end }}
{{- end }}
	}

	// Build SQL with placeholders. A taken key inserts nothing (ON CONFLICT … DO NOTHING) instead
	// of failing: on PostgreSQL a failed INSERT would abort the transaction
	sqlStr := fmt.Sprintf(` + "`INSERT INTO \"%s\" ({{ range $i, $f := .Table.Fields }}{{ if not $f.FlowField }}{{ $f.DBName }}{{ if not (isLastDBField $i $.Table.Fields) }}, {{ end }}{{ end }}{{ end }}) VALUES ({{ range $i, $f := .Table.Fields }}{{ if not $f.FlowField }}?{{ if not (isLastDBField $i $.Table.Fields) }}, {{ end }}{{ end }}{{ end }}) ON CONFLICT ({{ range $i, $f := .Table.Fields }}{{ if $f.PrimaryKey }}{{ $f.DBName }}{{ if not (isLastPK $i $.Table.Fields) }}, {{ end }}{{ end }}{{ end }}) DO NOTHING`" + `, tableName)

	// Convert placeholders for PostgreSQL
	sqlStr = t.convertPlaceholders(sqlStr, len(args))

	result, err := t.exec().Exec(sqlStr, args...)
	if err != nil {
		fmt.Printf("Error: Failed to insert {{ .Table.Name }}: %v\n", err)
		return false
	}
	// The key is taken (INSERT fails in the classic way: "<Table> <key> already exists")
	if n, err := result.RowsAffected(); err == nil && n == 0 {
		_, key := t.recordKey()
		t.triggerErr = apperrors.TableRecordExists({{ .StructName }}TableName, key)
		return false
	}
	// The record as written: its xRec from now on (a following Modify or Rename compares with it)
	t.StoreOldValues()
	return true
}

// InsertAll inserts records in bulk (demo data, imports): multi-row INSERT statements, one
// round trip per batch instead of one per record. With runTrigger every record's OnInsert
// trigger runs first, as Insert(true) would; the first failure stops before anything is
// written. Records are written with the receiver's database and company. Errors are
// *tables.BatchInsertError (Index = the record in records). Use it in a transaction: a
// failed batch leaves the earlier batches written.
func (t *{{ .StructName }}) InsertAll(records []*{{ .StructName }}, runTrigger bool) error {
	rows := make([][]interface{}, 0, len(records))
	for i, r := range records {
		r.triggerErr = nil
		if runTrigger {
			if err := r.runOnInsert(); err != nil {
				r.triggerErr = err
				return &tables.BatchInsertError{Index: i, Trigger: true, Err: err}
			}
		}
{{- if .EncryptedFields }}
		if err := r.encryptSecrets(); err != nil {
			return &tables.BatchInsertError{Index: i, Trigger: true, Err: err}
		}
{{- end }}
		rows = append(rows, []interface{}{
{{- range .Table.Fields }}
{{- if not .FlowField }}
			{{ keyArg . (printf "r.%s" (upperFirst .Name)) }},
{{- end }}
{{- end }}
		})
	}
{{- if .Table.Global }}
	tableName := {{ .StructName }}TableName
{{- else }}
	tableName := fmt.Sprintf("%s$%s", t.company, {{ .StructName }}TableName)
{{- end }}
	columns := []string{ {{- range $i, $f := .Table.Fields }}{{ if not $f.FlowField }}"{{ $f.DBName }}"{{ if not (isLastDBField $i $.Table.Fields) }}, {{ end }}{{ end }}{{ end -}} }
	return tables.InsertRows(t.exec(), t.dbType, tableName, columns, rows)
}

{{ if .EncryptedFields -}}
// ========================================
// Encrypted fields (encrypted: true)
// ========================================

// encryptSecrets stores the encrypted fields encrypted (secrets.Encrypt) when they hold a
// new value as typed. Without an encryption key (neither OPENERP_ENCRYPTION_KEY nor
// JWT_SECRET set — development) the value stays as typed; the backend warns at startup.
func (t *{{ .StructName }}) encryptSecrets() error {
{{- range .EncryptedFields }}
	if v := t.{{ upperFirst .Name }}.String(); v != "" && !secrets.IsEncrypted(v) {
		// Encrypted, the value must still fit the {{ .Length }}-character column
		if len(v) > {{ maxEncryptedPlain .Length }} {
			return apperrors.FieldTooLong({{ $.StructName }}TableName, "{{ .Name }}", {{ maxEncryptedPlain .Length }})
		}
		stored, err := secrets.Encrypt(v)
		if err != nil && !errors.Is(err, secrets.ErrNoKey) {
			return err
		}
		if err == nil {
			t.{{ upperFirst .Name }} = types.NewText(stored)
		}
	}
{{- end }}
	return nil
}
{{- range .EncryptedFields }}

// Plain{{ upperFirst .Name }} returns {{ .Name }} decrypted, for the server code that uses the
// secret (never send it to the client). secrets.ErrDecrypt means it was encrypted with
// another key: it has to be entered again.
func (t *{{ $.StructName }}) Plain{{ upperFirst .Name }}() (string, error) {
	return secrets.Decrypt(t.{{ upperFirst .Name }}.String())
}
{{- end }}

// EncryptStoredSecrets encrypts the values of encrypted fields that are stored as typed
// (saved before the field was encrypted, or without a key) in the table of the receiver's
// company. Run at startup; does nothing without a key. Returns how many it encrypted.
func (t *{{ .StructName }}) EncryptStoredSecrets() (int, error) {
	if secrets.Source() == "" {
		return 0, nil
	}
{{- if .Table.Global }}
	tableName := {{ .StructName }}TableName
{{- else }}
	tableName := fmt.Sprintf("%s$%s", t.company, {{ .StructName }}TableName)
{{- end }}
	done := 0
{{- range .EncryptedFields }}
	{
		rows, err := t.db.Query(fmt.Sprintf(` + "`SELECT {{ range $.Table.Fields }}{{ if .PrimaryKey }}{{ .DBName }}, {{ end }}{{ end }}{{ .DBName }} FROM \"%s\" WHERE {{ .DBName }} <> ''`" + `, tableName))
		if err != nil {
			return done, err
		}
		type plainRow struct {
			key   []interface{}
			value string
		}
		var plain []plainRow
		for rows.Next() {
			key := make([]interface{}, {{ pkCount $.Table.Fields }})
			dest := make([]interface{}, 0, len(key)+1)
			for i := range key {
				dest = append(dest, &key[i])
			}
			var value string
			dest = append(dest, &value)
			if err := rows.Scan(dest...); err != nil {
				_ = rows.Close()
				return done, err
			}
			if !secrets.IsEncrypted(value) {
				plain = append(plain, plainRow{key, value})
			}
		}
		if err := rows.Close(); err != nil {
			return done, err
		}
		for _, r := range plain {
			if len(r.value) > {{ maxEncryptedPlain .Length }} {
				return done, apperrors.FieldTooLong({{ $.StructName }}TableName, "{{ .Name }}", {{ maxEncryptedPlain .Length }})
			}
			stored, err := secrets.Encrypt(r.value)
			if err != nil {
				return done, err
			}
			sqlStr := t.convertPlaceholders(fmt.Sprintf(` + "`UPDATE \"%s\" SET {{ .DBName }} = ? WHERE {{ range $i, $f := $.Table.Fields }}{{ if $f.PrimaryKey }}{{ $f.DBName }} = ?{{ if not (isLastPK $i $.Table.Fields) }} AND {{ end }}{{ end }}{{ end }}`" + `, tableName), 1+len(r.key))
			if _, err := t.db.Exec(sqlStr, append([]interface{}{stored}, r.key...)...); err != nil {
				return done, err
			}
			done++
		}
	}
{{- end }}
	return done, nil
}

{{ end -}}
// Modify updates the record in the database
func (t *{{ .StructName }}) Modify(runTrigger bool) bool {
	// OnModify trigger of the business logic, then the field checks of the definition
	t.triggerErr = nil
	if runTrigger {
		if err := t.runOnModify(); err != nil {
			fmt.Printf("Error: OnModify trigger failed: %v\n", err)
			t.triggerErr = err
			return false
		}
	}
	// A changed primary key is a rename: run the OnRename trigger
	// before it is written; xRec values are available through OldValue
	keyChanged := false
	if t.oldValues != nil {
{{- range .Table.Fields }}
{{- if .PrimaryKey }}
		if t.hasFieldChanged("{{ .DBName }}") {
			keyChanged = true
		}
{{- end }}
{{- end }}
	}
	if runTrigger && keyChanged {
		if err := t.runOnRename(); err != nil {
			fmt.Printf("Error: OnRename trigger failed: %v\n", err)
			t.triggerErr = err
			return false
		}
	}
{{- if .EncryptedFields }}
	if err := t.encryptSecrets(); err != nil {
		fmt.Printf("Error: %v\n", err)
		t.triggerErr = err
		return false
	}
{{- end }}

{{- if .Table.Global }}
	tableName := {{ .StructName }}TableName
{{- else }}
	tableName := fmt.Sprintf("%s$%s", t.company, {{ .StructName }}TableName)
{{- end }}

	// Build dynamic SQL based on field tracking
	var setClauses []string
	var values []interface{}

	// If we have old values (loaded from Get), only update changed fields.
	// Changed primary key fields are renamed in place (Rename): they are SET to the
	// new value while the WHERE clause matches the old one.
	if t.oldValues != nil {
{{- range .Table.Fields }}
{{- if not .FlowField }}
		if t.hasFieldChanged("{{ .DBName }}") {
			setClauses = append(setClauses, "{{ .DBName }} = ?")
			values = append(values, {{ keyArg . (printf "t.%s" (upperFirst .Name)) }})
		}
{{- end }}
{{- end }}

		// If nothing changed, skip update
		if len(setClauses) == 0 {
			return true // No changes, success
		}
	} else {
		// No old values (fresh record), update all fields
{{- range .Table.Fields }}
{{- if and (not .PrimaryKey) (not .FlowField) }}
		setClauses = append(setClauses, "{{ .DBName }} = ?")
		values = append(values, t.{{ upperFirst .Name }})
{{- end }}
{{- end }}
	}

	// Add WHERE clause value (primary key as loaded, so a renamed key still matches)
{{- range .Table.Fields }}
{{- if .PrimaryKey }}
	if old, ok := t.oldValues["{{ .DBName }}"]; ok {
		values = append(values, {{ keyArg . "old" }})
	} else {
		values = append(values, {{ keyArg . (printf "t.%s" (upperFirst .Name)) }})
	}
{{- end }}
{{- end }}

{{- if .References }}
	// Renamed key fields that other tables refer to (Rename): carried over below
	type renamedKey struct {
		field    string
		old, new interface{}
	}
	var renamed []renamedKey
	if t.oldValues != nil {
{{- range .Table.Fields }}
{{- if and .PrimaryKey (index $.References .DBName) }}
		if t.hasFieldChanged("{{ .DBName }}") {
			renamed = append(renamed, renamedKey{"{{ .DBName }}", t.oldValues["{{ .DBName }}"], t.{{ upperFirst .Name }}})
		}
{{- end }}
{{- end }}
	}
{{- end }}

	// Build and execute SQL
	sqlStr := fmt.Sprintf(` + "`UPDATE \"%s\" SET %s WHERE 1=1{{ range .Table.Fields }}{{ if .PrimaryKey }} AND {{ .DBName }} = ?{{ end }}{{ end }}`" + `,
		tableName,
		strings.Join(setClauses, ", "),
	)

	// Convert placeholders for PostgreSQL
	sqlStr = t.convertPlaceholders(sqlStr, len(values))

	_, err := t.exec().Exec(sqlStr, values...)
	if err != nil {
		fmt.Printf("Error: Failed to modify {{ .Table.Name }}: %v\n", err)
		return false
	}
{{- if .References }}
	// A temporary record's rename stays in the record: other tables are not changed
	for _, r := range renamed {
		if t.temp != nil {
			break
		}
		if err := t.renameReferences(r.field, r.old, r.new); err != nil {
			fmt.Printf("Error: Failed to rename references to {{ .Table.Name }}: %v\n", err)
			t.triggerErr = err
			return false
		}
	}
{{- end }}
	// The stored record now matches the current values (including a renamed key)
	if t.oldValues != nil {
		t.StoreOldValues()
	}
	return true
}

// hasFieldChanged checks if a field value has changed from oldValues
func (t *{{ .StructName }}) hasFieldChanged(fieldName string) bool {
	if t.oldValues == nil {
		return true // No old values, assume changed
	}

	oldValue, exists := t.oldValues[fieldName]
	if !exists {
		return true // Field not in old values, assume changed
	}
	_ = oldValue // Suppress unused variable when all fields are PKs

	// Compare old vs new value based on field name (with type assertion)
	switch fieldName {
{{- range .Table.Fields }}
{{- if not .FlowField }}
	case "{{ .DBName }}":
{{- if eq .Type "Option" }}
		if old, ok := oldValue.({{ $.StructName }}{{ upperFirst .Name }}); ok {
			return t.{{ upperFirst .Name }} != old
		}
{{- else if eq .Type "types.Code" }}
		if old, ok := oldValue.(types.Code); ok {
			return !t.{{ upperFirst .Name }}.Equal(old)
		}
{{- else if eq .Type "types.Text" }}
		if old, ok := oldValue.(types.Text); ok {
			return !t.{{ upperFirst .Name }}.Equal(old)
		}
{{- else if eq .Type "types.Decimal" }}
		if old, ok := oldValue.(types.Decimal); ok {
			return !t.{{ upperFirst .Name }}.Equal(old)
		}
{{- else if eq .Type "types.Date" }}
		if old, ok := oldValue.(types.Date); ok {
			return !t.{{ upperFirst .Name }}.Equal(old)
		}
{{- else if eq .Type "types.DateTime" }}
		if old, ok := oldValue.(types.DateTime); ok {
			return !t.{{ upperFirst .Name }}.Equal(old)
		}
{{- else if eq .Type "[]byte" }}
		// Skip comparison for BLOB fields (too large, use always modified)
		return true
{{- else }}
		if old, ok := oldValue.({{ .Type }}); ok {
			return t.{{ upperFirst .Name }} != old
		}
		return true // Type mismatch, assume changed
{{- end }}
{{- end }}
{{- end }}
	}

	return false
}

// SetDB changes the database executor of the record without touching its values — e.g. to
// run a Modify (with its rename cascade) inside a transaction.
func (t *{{ .StructName }}) SetDB(db database.Executor) {
	t.db = db
}
{{- if .References }}

// renameReferences carries a renamed key over to every field of another table that refers
// to it (Rename). References from company tables are updated in this company — in
// every company when this table is global. Runs on the record's executor, so inside the
// caller's transaction.
func (t *{{ .StructName }}) renameReferences(field string, oldValue, newValue interface{}) error {
	type reference struct {
		table      string
		global     bool
		column     string
		condColumn string // a conditional relation: only rows whose condColumn = condValue
		condValue  int
	}
	references := map[string][]reference{
{{- range $field, $refs := .References }}
		"{{ $field }}": { {{- range $refs }}{"{{ .Table }}", {{ .Global }}, "{{ .Column }}", "{{ .CondColumn }}", {{ .CondValue }}}, {{ end -}} },
{{- end }}
	}
{{- if .Table.Global }}
	var companies []string
	rows, err := t.db.Query(` + "`SELECT name FROM \"Company\"`" + `)
	if err != nil {
		return err
	}
	for rows.Next() {
		var name string
		if err := rows.Scan(&name); err != nil {
			rows.Close()
			return err
		}
		companies = append(companies, name)
	}
	rows.Close()
{{- else }}
	companies := []string{t.company}
{{- end }}
	for _, ref := range references[field] {
		tables := []string{ref.table}
		if !ref.global {
			tables = tables[:0]
			for _, c := range companies {
				tables = append(tables, c+"$"+ref.table)
			}
		}
		for _, tbl := range tables {
			stmt := fmt.Sprintf(` + "`UPDATE \"%s\" SET %s = ? WHERE %s = ?`" + `, tbl, ref.column, ref.column)
			args := []interface{}{newValue, oldValue}
			if ref.condColumn != "" {
				stmt += fmt.Sprintf(" AND %s = ?", ref.condColumn)
				args = append(args, ref.condValue)
			}
			query := t.convertPlaceholders(stmt, len(args))
			if _, err := t.db.Exec(query, args...); err != nil {
				return fmt.Errorf("%s.%s: %w", tbl, ref.column, err)
			}
		}
	}
	return nil
}
{{- end }}

// Delete removes the record from the database
func (t *{{ .StructName }}) Delete(runTrigger bool) bool {
	// OnDelete trigger of the business logic
	t.triggerErr = nil
	if runTrigger {
		if err := t.runOnDelete(); err != nil {
			fmt.Printf("Error: OnDelete trigger failed: %v\n", err)
			t.triggerErr = err
			return false
		}
	}

{{- if .Table.Global }}
	tableName := {{ .StructName }}TableName
{{- else }}
	tableName := fmt.Sprintf("%s$%s", t.company, {{ .StructName }}TableName)
{{- end }}

	// Collect arguments for DELETE
	args := []interface{}{
{{- range .Table.Fields }}
{{- if .PrimaryKey }}
		{{ keyArg . (printf "t.%s" (upperFirst .Name)) }},
{{- end }}
{{- end }}
	}

	// Build SQL with placeholders
	sqlStr := fmt.Sprintf(` + "`DELETE FROM \"%s\" WHERE 1=1{{ range .Table.Fields }}{{ if .PrimaryKey }} AND {{ .DBName }} = ?{{ end }}{{ end }}`" + `, tableName)

	// Convert placeholders for PostgreSQL
	sqlStr = t.convertPlaceholders(sqlStr, len(args))

	_, err := t.exec().Exec(sqlStr, args...)
	if err != nil {
		fmt.Printf("Error: Failed to delete {{ .Table.Name }}: %v\n", err)
		return false
	}
	return true
}

{{- if .HasFlowField }}

// ========================================
// FlowField Calculations
// ========================================

// CalcFields calculates FlowField values
// Usage:
//   customer.CalcFields("balance", "balance_lcy") - Calculate specific fields
//   customer.CalcFields() - Calculate all FlowFields
func (t *{{ .StructName }}) CalcFields(fieldNames ...string) {
	// If no field names specified, calculate all FlowFields
	if len(fieldNames) == 0 {
		{{- range .Table.Fields }}
		{{- if .FlowField }}
		t.calcFlowField_{{ .Name }}()
		{{- end }}
		{{- end }}
		return
	}

	// Calculate only specified fields
	for _, fieldName := range fieldNames {
		switch fieldName {
		{{- range .Table.Fields }}
		{{- if .FlowField }}
		case "{{ .Name }}":
			t.calcFlowField_{{ .Name }}()
		{{- end }}
		{{- end }}
		}
	}
}

{{- range .Table.Fields }}
{{- if .FlowField }}

// calcFlowField_{{ .Name }} calculates the {{ .Name }} FlowField
// CalcFormula: {{ .CalcFormula }}({{ .SourceTable }}.{{ .SourceField }})
func (t *{{ $.StructName }}) calcFlowField_{{ .Name }}() {
	{{- if eq .CalcFormula "Sum" }}
	t.{{ upperFirst .Name }} = t.calcSum_{{ .Name }}()
	{{- else if eq .CalcFormula "Count" }}
	t.{{ upperFirst .Name }} = t.calcCount_{{ .Name }}()
	{{- else if eq .CalcFormula "Average" }}
	t.{{ upperFirst .Name }} = t.calcAverage{{ upperFirst .SourceTable }}{{ upperFirst .SourceField }}()
	{{- else if eq .CalcFormula "Min" }}
	t.{{ upperFirst .Name }} = t.calcMin{{ upperFirst .SourceTable }}{{ upperFirst .SourceField }}()
	{{- else if eq .CalcFormula "Max" }}
	t.{{ upperFirst .Name }} = t.calcMax{{ upperFirst .SourceTable }}{{ upperFirst .SourceField }}()
	{{- else if eq .CalcFormula "Lookup" }}
	t.{{ upperFirst .Name }} = t.calcLookup_{{ .Name }}()
	{{- else if eq .CalcFormula "Exist" }}
	t.{{ upperFirst .Name }} = t.calcCount_{{ .Name }}() > 0
	{{- end }}
}
{{- end }}
{{- end }}

// CalcFieldsForRecords calculates FlowFields for many records at once (list pages):
// one grouped query per FlowField instead of one query per record. records are
// ToMap() results; each gets its FlowField values under the field name. With no
// field names, all FlowFields are calculated.
func (t *{{ .StructName }}) CalcFieldsForRecords(records []map[string]interface{}, fieldNames ...string) {
	if len(records) == 0 {
		return
	}
	if len(fieldNames) == 0 {
		fieldNames = t.GetFlowFields()
	}
	for _, fieldName := range fieldNames {
		switch fieldName {
		{{- range .Table.Fields }}
		{{- if .FlowField }}
		case "{{ .DBName }}":
			t.calcForRecords_{{ .Name }}(records)
		{{- end }}
		{{- end }}
		}
	}
}

{{- range .Table.Fields }}
{{- if .FlowField }}

// calcForRecords_{{ .Name }} calculates the {{ .Name }} FlowField for a set of records
// CalcFormula: {{ .CalcFormula }}({{ .SourceTable }}.{{ .SourceField }})
func (t *{{ $.StructName }}) calcForRecords_{{ .Name }}(records []map[string]interface{}) {
{{- if .HasFieldFilter }}
	// Records whose {{ .FieldFilterColumn }} holds a filter (total accounts) are calculated one by
	// one, with this record's FlowFilters; the others in the grouped query below
	var grouped []map[string]interface{}
	for _, rec := range records {
		if s := strings.TrimSpace(fmt.Sprint(rec["{{ .FieldFilterColumn }}"])); s == "" || s == "<nil>" {
			grouped = append(grouped, rec)
			continue
		}
		one := &{{ $.StructName }}{}
		one.InitWithDBType(t.db, t.company, t.dbType)
{{- range $.FlowFilterFields }}
		one.{{ upperFirst .Name }} = t.{{ upperFirst .Name }}
{{- end }}
		one.FromMap(rec)
{{- if (or (eq .CalcFormula "Count") (eq .CalcFormula "Exist")) }}
		rec["{{ .DBName }}"] = one.calcCount_{{ .Name }}(){{ if eq .CalcFormula "Exist" }} > 0{{ end }}
{{- else }}
		rec["{{ .DBName }}"] = one.calcSum_{{ .Name }}().String()
{{- end }}
	}
	records = grouped
	if len(records) == 0 {
		return
	}
{{- end }}
{{- if and .BatchKeyField (or (eq .CalcFormula "Sum") (eq .CalcFormula "Count") (eq .CalcFormula "Exist")) }}
{{- template "flowSource" . }}

	// Distinct key values of the records
	seen := make(map[string]bool, len(records))
	var keys []interface{}
	for _, rec := range records {
		k := fmt.Sprint(rec["{{ .BatchKeyValue }}"])
		if !seen[k] {
			seen[k] = true
			keys = append(keys, rec["{{ .BatchKeyValue }}"])
		}
	}

	// One grouped query for the keys (IN list). With more keys than chunkSize (a long
	// list) one grouped query over the whole source table instead: much cheaper than
	// many IN queries, and it keeps the bind parameters bounded.
	{{- if (or (eq .CalcFormula "Count") (eq .CalcFormula "Exist")) }}
	values := make(map[string]int, len(keys))
	{{- else }}
	values := make(map[string]{{ .Type }}, len(keys))
	{{- end }}
	const chunkSize = 500
	chunks := [][]interface{}{nil} // nil chunk: no IN filter, all keys
	if len(keys) <= chunkSize {
		chunks = [][]interface{}{keys}
	}
	for _, chunk := range chunks {

		var whereClauses []string
		var args []interface{}
		{{- range .FlowFilters }}
		{{- if eq .Type "const" }}
		whereClauses = append(whereClauses, "{{ .Field }} = ?")
		args = append(args, {{ .Value }})
		{{- end }}
		{{- end }}
		{{- template "flowFilterWhere" . }}
		if chunk != nil {
			whereClauses = append(whereClauses, "{{ .BatchKeyField }} IN ("+strings.TrimSuffix(strings.Repeat("?, ", len(chunk)), ", ")+")")
			args = append(args, chunk...)
		}
		whereClause := "1=1"
		if len(whereClauses) > 0 {
			whereClause = strings.Join(whereClauses, " AND ")
		}

		{{- if (or (eq .CalcFormula "Count") (eq .CalcFormula "Exist")) }}
		agg := "COUNT(*)"
		if useSIFT {
			agg = "COALESCE(SUM(cnt), 0)" // the totals hold the entry count per key value
		}
		query := fmt.Sprintf(` + "`SELECT {{ .BatchKeyField }}, %s FROM \"%s\" WHERE %s GROUP BY {{ .BatchKeyField }}`" + `, agg, tableName, whereClause)
		{{- else }}
		query := fmt.Sprintf(` + "`SELECT {{ .BatchKeyField }}, {{ if .ReverseSign }}-{{ end }}COALESCE(SUM({{ .SourceField }}), 0) FROM \"%s\" WHERE %s GROUP BY {{ .BatchKeyField }}`" + `, tableName, whereClause)
		{{- end }}
		query = t.convertPlaceholders(query, len(args))

		rows, err := t.db.Query(query, args...)
		if err != nil {
			fmt.Printf("Error: Failed to calculate {{ .Name }}: %v\n", err)
			return
		}
		for rows.Next() {
			var key string
			{{- if (or (eq .CalcFormula "Count") (eq .CalcFormula "Exist")) }}
			var count int
			if err := rows.Scan(&key, &count); err == nil {
				values[key] = count
			}
			{{- else }}
			var sumStr string
			if err := rows.Scan(&key, &sumStr); err == nil {
				values[key], _ = types.NewDecimalFromString(sumStr)
			}
			{{- end }}
		}
		_ = rows.Close()
	}

	// Records without matching entries get zero
	for _, rec := range records {
		{{- if (or (eq .CalcFormula "Count") (eq .CalcFormula "Exist")) }}
		rec["{{ .DBName }}"] = values[fmt.Sprint(rec["{{ .BatchKeyValue }}"])]{{ if eq .CalcFormula "Exist" }} > 0{{ end }}
		{{- else }}
		v, ok := values[fmt.Sprint(rec["{{ .BatchKeyValue }}"])]
		if !ok {
			v = types.ZeroDecimal()
		}
		rec["{{ .DBName }}"] = v.String()
		{{- end }}
	}
{{- else }}
	// No single key field to group by: calculate per record, with this record's FlowFilters
	for _, rec := range records {
		r := &{{ $.StructName }}{db: t.db, company: t.company, dbType: t.dbType}
{{- range $.FlowFilterFields }}
		r.{{ upperFirst .Name }} = t.{{ upperFirst .Name }}
{{- end }}
		r.FromMap(rec)
		r.calcFlowField_{{ .Name }}()
		{{- if eq .Type "types.Decimal" }}
		rec["{{ .DBName }}"] = r.{{ upperFirst .Name }}.String()
		{{- else }}
		rec["{{ .DBName }}"] = r.{{ upperFirst .Name }}
		{{- end }}
	}
{{- end }}
}
{{- end }}
{{- end }}

// Helper methods for FlowField calculations
{{- range .Table.Fields }}
{{- if and .FlowField (eq .CalcFormula "Sum") }}

func (t *{{ $.StructName }}) calcSum_{{ .Name }}() {{ .Type }} {
{{- template "flowSource" . }}

	// Build WHERE clause from FlowFilters
	var whereClauses []string
	var args []interface{}

	{{- range .FlowFilters }}
	{{- if eq .Type "const" }}
	whereClauses = append(whereClauses, "{{ .Field }} = ?")
	args = append(args, {{ .Value }})
	{{- else if and (eq .Type "field") .OverrideField }}
	if s := strings.TrimSpace(t.{{ .OverrideField }}.String()); s != "" {
		// {{ .OverrideField }} holds a filter of {{ .Field }} (a total account sums its range)
		if clause, filterArgs, err := flowfilter.Clause("{{ .Field }}", flowfilter.{{ .OverrideKind }}, s); err == nil && clause != "" {
			whereClauses = append(whereClauses, clause)
			args = append(args, filterArgs...)
		} else {
			whereClauses = append(whereClauses, "1=0")
		}
	} else {
		whereClauses = append(whereClauses, "{{ .Field }} = ?")
		args = append(args, t.{{ upperFirst .Value }})
	}
	{{- else if eq .Type "field" }}
	whereClauses = append(whereClauses, "{{ .Field }} = ?")
	args = append(args, t.{{ upperFirst .Value }})
	{{- end }}
	{{- end }}
	{{- template "flowFilterWhere" . }}

	whereClause := "1=1"
	if len(whereClauses) > 0 {
		whereClause = strings.Join(whereClauses, " AND ")
	}

	query := fmt.Sprintf(` + "`SELECT {{ if .ReverseSign }}-{{ end }}COALESCE(SUM({{ .SourceField }}), 0) FROM \"%s\" WHERE %s`" + `, tableName, whereClause)

	// Convert placeholders for PostgreSQL
	query = t.convertPlaceholders(query, len(args))

	var sumStr string
	err := t.db.QueryRow(query, args...).Scan(&sumStr)
	if err != nil {
		fmt.Printf("Error: Failed to calculate sum for {{ .Name }}: %v\n", err)
		return types.ZeroDecimal()
	}

	sum, _ := types.NewDecimalFromString(sumStr)
	return sum
}
{{- end }}
{{- if and .FlowField ((or (eq .CalcFormula "Count") (eq .CalcFormula "Exist"))) }}

func (t *{{ $.StructName }}) calcCount_{{ .Name }}() int {
{{- template "flowSource" . }}

	// Build WHERE clause from FlowFilters
	var whereClauses []string
	var args []interface{}

	{{- range .FlowFilters }}
	{{- if eq .Type "const" }}
	whereClauses = append(whereClauses, "{{ .Field }} = ?")
	args = append(args, {{ .Value }})
	{{- else if and (eq .Type "field") .OverrideField }}
	if s := strings.TrimSpace(t.{{ .OverrideField }}.String()); s != "" {
		// {{ .OverrideField }} holds a filter of {{ .Field }} (a total account sums its range)
		if clause, filterArgs, err := flowfilter.Clause("{{ .Field }}", flowfilter.{{ .OverrideKind }}, s); err == nil && clause != "" {
			whereClauses = append(whereClauses, clause)
			args = append(args, filterArgs...)
		} else {
			whereClauses = append(whereClauses, "1=0")
		}
	} else {
		whereClauses = append(whereClauses, "{{ .Field }} = ?")
		args = append(args, t.{{ upperFirst .Value }})
	}
	{{- else if eq .Type "field" }}
	whereClauses = append(whereClauses, "{{ .Field }} = ?")
	args = append(args, t.{{ upperFirst .Value }})
	{{- end }}
	{{- end }}
	{{- template "flowFilterWhere" . }}

	whereClause := "1=1"
	if len(whereClauses) > 0 {
		whereClause = strings.Join(whereClauses, " AND ")
	}

	agg := "COUNT(*)"
	if useSIFT {
		agg = "COALESCE(SUM(cnt), 0)" // the totals hold the entry count per key value
	}
	query := fmt.Sprintf(` + "`SELECT %s FROM \"%s\" WHERE %s`" + `, agg, tableName, whereClause)

	// Convert placeholders for PostgreSQL
	query = t.convertPlaceholders(query, len(args))

	var count int
	err := t.db.QueryRow(query, args...).Scan(&count)
	if err != nil {
		fmt.Printf("Error: Failed to calculate count for {{ .Name }}: %v\n", err)
		return 0
	}

	return count
}
{{- end }}
{{- if and .FlowField (eq .CalcFormula "Lookup") }}

// calcLookup_{{ .Name }} reads {{ .SourceTable }}.{{ .SourceField }} of the first record matching the
// flow filters (CalcFormula Lookup); blank when there is none
func (t *{{ $.StructName }}) calcLookup_{{ .Name }}() {{ .Type }} {
	tableName := fmt.Sprintf("%s$%s", t.company, {{ toPascalCase .SourceTable }}TableName)
	var whereClauses []string
	var args []interface{}
	{{- range .FlowFilters }}
	{{- if eq .Type "const" }}
	whereClauses = append(whereClauses, "{{ .Field }} = ?")
	args = append(args, {{ .Value }})
	{{- else if eq .Type "field" }}
	whereClauses = append(whereClauses, "{{ .Field }} = ?")
	args = append(args, t.{{ upperFirst .Value }})
	{{- end }}
	{{- end }}
	{{- template "flowFilterWhere" . }}
	whereClause := "1=1"
	if len(whereClauses) > 0 {
		whereClause = strings.Join(whereClauses, " AND ")
	}
	query := t.convertPlaceholders(fmt.Sprintf(` + "`SELECT {{ .SourceField }} FROM \"%s\" WHERE %s LIMIT 1`" + `, tableName, whereClause), len(args))
	var value sql.NullString
	if err := t.db.QueryRow(query, args...).Scan(&value); err != nil && err != sql.ErrNoRows {
		fmt.Printf("Error: Failed to look up {{ .Name }}: %v\n", err)
	}
{{- if eq .Type "types.Code" }}
	return types.NewCode(value.String)
{{- else if eq .Type "types.Text" }}
	return types.NewText(value.String)
{{- else }}
	return value.String
{{- end }}
}
{{- end }}
{{- end }}

{{- else }}

// CalcFields is a no-op for tables without FlowFields
// Implemented for tables.Table interface compliance
func (t *{{ .StructName }}) CalcFields(fieldNames ...string) {
	// This table has no FlowFields to calculate
}

// CalcFieldsForRecords is a no-op for tables without FlowFields
func (t *{{ .StructName }}) CalcFieldsForRecords(records []map[string]interface{}, fieldNames ...string) {
}

{{- end }}

// ========================================
// Filtering and Search
// ========================================

// {{ lowerFirst .StructName }}FilterCondition represents a filter on a field
type {{ lowerFirst .StructName }}FilterCondition struct {
	fieldName    string
	minValue     interface{}
	maxValue     interface{}
	filterExpr   string        // For complex SetFilter expressions
	isExpression bool          // True if using filterExpr instead of min/max
	invalidField bool          // Filter on an unknown field: matches no rows (fail closed)
	blankDate    bool          // A date range from the blank date: blank dates (NULL) match too
{{- if .HasFilterableFlowField }}
	flowField    string        // Filter on a FlowField (filterExpr applied to its subquery)
{{- end }}
}

// SetRange sets a range filter on a field
// Usage:
//   SetRange("No", "10000") - exact match (No = "10000")
//   SetRange("No", "10000", "20000") - range (No between "10000" and "20000")
//   SetRange("No") - removes the filter on the field
func (t *{{ .StructName }}) SetRange(fieldName string, values ...interface{}) {
	if t.filters == nil {
		t.filters = make(map[string]*{{ lowerFirst .StructName }}FilterCondition)
	}

	var minValue, maxValue interface{}

	switch len(values) {
	case 0:
		if column, ok := t.columnName(fieldName); ok {
			delete(t.filters, column)
		}
		delete(t.filters, fieldName)
		return
	case 1:
		// Exact match: SetRange("No", "10000")
		minValue = values[0]
		maxValue = values[0]
	case 2:
		// Range: SetRange("No", "10000", "20000")
		minValue = values[0]
		maxValue = values[1]
	default:
		fmt.Printf("Error: SetRange requires 1 or 2 values, got %d\n", len(values))
		return
	}

	column, ok := t.columnName(fieldName)
	if !ok {
		// Unknown field: fail closed (no rows) rather than drop the filter or put
		// the name into SQL
		fmt.Printf("Error: SetRange on unknown field %q of {{ .Table.Name }}\n", fieldName)
		t.filters[fieldName] = &{{ lowerFirst .StructName }}FilterCondition{invalidField: true}
		return
	}

	// Blank dates are stored as NULL and come before every date: a range from the blank date
	// (or the blank date itself) matches them; a range up to the blank date (0D..0D) is the
	// blank date only
	blankDate := false
	if d, ok := values[0].(types.Date); ok && d.IsZero() {
		blankDate = true
		minValue = nil
		if len(values) == 1 {
			maxValue = nil
		}
		if to, ok := maxValue.(types.Date); ok && to.IsZero() {
			maxValue = nil
		}
	}

	t.filters[column] = &{{ lowerFirst .StructName }}FilterCondition{
		fieldName: column,
		minValue:  minValue,
		maxValue:  maxValue,
		blankDate: blankDate,
	}
}

// SetFilter sets a complex filter expression on a field
// Supports filter syntax: "100..200|500" (range OR exact value)
// Operators: .. (range), | (OR), & (AND), * (wildcard), <> (not equal)
// Example: customer.SetFilter("No", "001..003|005")
func (t *{{ .StructName }}) SetFilter(fieldName, filterExpr string) {
	if t.filters == nil {
		t.filters = make(map[string]*{{ lowerFirst .StructName }}FilterCondition)
	}
{{- if .HasFilterableFlowField }}
	if _, ok := t.FlowFieldFilterKind(fieldName); ok {
		// A FlowField is not a column: the filter applies to its subquery (flowFieldExpr)
		name := strings.ToLower(fieldName)
		t.filters["flowfield:"+name] = &{{ lowerFirst .StructName }}FilterCondition{
			flowField:    name,
			filterExpr:   filterExpr,
			isExpression: true,
		}
		return
	}
{{- end }}
	column, ok := t.columnName(fieldName)
	if !ok {
		// Unknown field: fail closed (no rows) rather than drop the filter or put
		// the name into SQL
		fmt.Printf("Error: SetFilter on unknown field %q of {{ .Table.Name }}\n", fieldName)
		t.filters[fieldName] = &{{ lowerFirst .StructName }}FilterCondition{invalidField: true}
		return
	}
	t.filters[column] = &{{ lowerFirst .StructName }}FilterCondition{
		fieldName:    column,
		filterExpr:   filterExpr,
		isExpression: true,
	}
}

{{ if .HasFilterableFlowField -}}
// FlowFieldFilterKind reports whether a list can be filtered on FlowField field (Sum and
// Count FlowFields: SETFILTER on a FlowField) and the kind of its values.
func (t *{{ .StructName }}) FlowFieldFilterKind(field string) (flowfilter.Kind, bool) {
	switch strings.ToLower(field) {
{{- range .Table.Fields }}
{{- if and .FlowField (or (eq .CalcFormula "Sum") (eq .CalcFormula "Count")) (not .HasFieldFilter) }}
	case "{{ .DBName }}":
{{- if eq .CalcFormula "Count" }}
		return flowfilter.KindInt, true
{{- else }}
		return flowfilter.KindDecimal, true
{{- end }}
{{- end }}
{{- end }}
	}
	return "", false
}

// flowFieldExpr is the SQL expression computing FlowField field for each row of a query
// on this table: a correlated subquery reading the source like CalcFieldsForRecords (the
// SIFT totals of a covering key, else the entries), with the FlowFilters set on t.
func (t *{{ .StructName }}) flowFieldExpr(field string) (string, []interface{}) {
{{- if .Table.Global }}
	outer := "\"" + {{ .StructName }}TableName + "\""
{{- else }}
	outer := "\"" + fmt.Sprintf("%s$%s", t.company, {{ .StructName }}TableName) + "\""
{{- end }}
	_ = outer
	switch strings.ToLower(field) {
{{- range .Table.Fields }}
{{- if and .FlowField (or (eq .CalcFormula "Sum") (eq .CalcFormula "Count")) (not .HasFieldFilter) }}
	case "{{ .DBName }}":
{{- template "flowSource" . }}
		var whereClauses []string
		var args []interface{}
		{{- range .FlowFilters }}
		{{- if eq .Type "const" }}
		whereClauses = append(whereClauses, "{{ .Field }} = ?")
		args = append(args, {{ .Value }})
		{{- else if eq .Type "field" }}
		whereClauses = append(whereClauses, "{{ .Field }} = "+outer+".{{ .ValueColumn }}")
		{{- end }}
		{{- end }}
		{{- template "flowFilterWhere" . }}
		where := "1=1"
		if len(whereClauses) > 0 {
			where = strings.Join(whereClauses, " AND ")
		}
		{{- if eq .CalcFormula "Count" }}
		agg := "COUNT(*)"
		if useSIFT {
			agg = "COALESCE(SUM(cnt), 0)"
		}
		{{- else }}
		agg := "{{ if .ReverseSign }}-{{ end }}COALESCE(SUM({{ .SourceField }}), 0)"
		{{- end }}
		return "(SELECT " + agg + " FROM \"" + tableName + "\" WHERE " + where + ")", args
{{- end }}
{{- end }}
	}
	return "NULL", nil
}

{{ end -}}

// SetCurrentKey sets the sort order for queries
// Unknown fields are ignored (the primary key order is used if none remain).
// Example: customer.SetCurrentKey("City", "Name")
func (t *{{ .StructName }}) SetCurrentKey(fields ...string) {
	t.orderByFields = nil
	for _, field := range fields {
{{- if .HasFilterableFlowField }}
		// A Sum/Count FlowField sorts on its calculated value (flowFieldExpr, see getOrderByClause)
		if _, ok := t.FlowFieldFilterKind(field); ok {
			t.orderByFields = append(t.orderByFields, "flowfield:"+strings.ToLower(field))
			continue
		}
{{- end }}
		column, ok := t.columnName(field)
		if !ok {
			fmt.Printf("Error: SetCurrentKey on unknown field %q of {{ .Table.Name }}\n", field)
			continue
		}
		t.orderByFields = append(t.orderByFields, column)
	}
}

// SetAscending sets the sort direction of the current key (Ascending).
// The default is ascending.
func (t *{{ .StructName }}) SetAscending(ascending bool) {
	t.descending = !ascending
}

// SetSearch limits the records to those where any of the given fields contains text,
// case-insensitive (the list page search box). Unknown fields are ignored; an empty
// text clears the search.
func (t *{{ .StructName }}) SetSearch(fields []string, text string) {
	t.searchColumns = nil
	t.searchText = strings.ToLower(strings.TrimSpace(text))
	if t.searchText == "" {
		return
	}
	for _, field := range fields {
		if column, ok := t.columnName(field); ok {
			t.searchColumns = append(t.searchColumns, column)
		}
	}
}

// HasColumn reports whether fieldName (case-insensitive) is a stored column of this
// table. Only such names may be used in filters and sort keys.
func (t *{{ .StructName }}) HasColumn(fieldName string) bool {
	_, ok := t.columnName(fieldName)
	return ok
}

// columnName maps a field name (case-insensitive) to its database column. Field names
// end up in SQL text (WHERE / ORDER BY / SET), so only names from this allowlist are used.
func (t *{{ .StructName }}) columnName(fieldName string) (string, bool) {
	switch strings.ToLower(fieldName) {
{{- range .Table.Fields }}
{{- if not .FlowField }}
	case strings.ToLower("{{ .DBName }}"):
		return "{{ .DBName }}", true
{{- end }}
{{- end }}
	}
	return "", false
}

// Reset clears all filters (of all filter groups; back to group 0)
func (t *{{ .StructName }}) Reset() {
	t.filters = nil
	t.otherFilters = nil
	t.filterGroup = 0
	t.seek, t.seekArgs = "", nil
	t.oldValues = nil
	t.orderByFields = nil
	t.descending = false
	t.searchColumns = nil
	t.searchText = ""
	if t.currentRows != nil {
		t.currentRows.Close()
		t.currentRows = nil
	}
}

// buildWhereClause builds WHERE clause from current filters and search
func (t *{{ .StructName }}) buildWhereClause() (string, []interface{}) {
	filters := t.allFilters()
	if len(filters) == 0 && t.searchText == "" && !t.markedOnly && t.seek == "" {
		return "1=1", nil
	}

	var conditions []string
	var args []interface{}

	if t.searchText != "" {
		if len(t.searchColumns) == 0 {
			conditions = append(conditions, "1=0") // searching, but in no valid column
		} else {
			// LIKE wildcards in the text are matched literally
			pattern := "%" + strings.NewReplacer("\\", "\\\\", "%", "\\%", "_", "\\_").Replace(t.searchText) + "%"
			var ors []string
			for _, column := range t.searchColumns {
				ors = append(ors, "LOWER(CAST("+column+" AS TEXT)) LIKE ? ESCAPE '\\'")
				args = append(args, pattern)
			}
			conditions = append(conditions, "("+strings.Join(ors, " OR ")+")")
		}
	}

	for _, filter := range filters {
		if filter.invalidField {
			conditions = append(conditions, "1=0")
{{- if .HasFilterableFlowField }}
		} else if filter.flowField != "" {
			// filter syntax on the FlowField's value; an invalid expression matches no rows
			// (the API validates it first and reports it)
			kind, _ := t.FlowFieldFilterKind(filter.flowField)
			expr, exprArgs := t.flowFieldExpr(filter.flowField)
			clause, clauseArgs, err := flowfilter.ClauseFor(expr, exprArgs, kind, filter.filterExpr)
			if err != nil {
				conditions = append(conditions, "1=0")
			} else if clause != "" {
				conditions = append(conditions, clause)
				args = append(args, clauseArgs...)
			}
{{- end }}
		} else if filter.isExpression {
			// Parse filter expression
			clause, exprArgs := t.parseFilterExpression(filter.fieldName, filter.filterExpr)
			conditions = append(conditions, clause)
			args = append(args, exprArgs...)
		} else {
			// Simple range filter
			if filter.blankDate && filter.maxValue != nil {
				conditions = append(conditions, fmt.Sprintf("(%s <= ? OR %s IS NULL)", filter.fieldName, filter.fieldName))
				args = append(args, filter.maxValue)
			} else if filter.blankDate {
				conditions = append(conditions, fmt.Sprintf("(%s IS NULL OR %s = '%s')", filter.fieldName, filter.fieldName, types.BlankKeyDate))
			} else if filter.minValue != nil && filter.maxValue != nil {
				conditions = append(conditions, fmt.Sprintf("%s BETWEEN %s AND %s", t.cmpColumn(filter.fieldName), t.cmpParam(filter.fieldName), t.cmpParam(filter.fieldName)))
				args = append(args, filter.minValue, filter.maxValue)
			} else if filter.minValue != nil {
				conditions = append(conditions, fmt.Sprintf("%s >= %s", t.cmpColumn(filter.fieldName), t.cmpParam(filter.fieldName)))
				args = append(args, filter.minValue)
			} else if filter.maxValue != nil {
				conditions = append(conditions, fmt.Sprintf("%s <= %s", t.cmpColumn(filter.fieldName), t.cmpParam(filter.fieldName)))
				args = append(args, filter.maxValue)
			}
		}
	}

	// MarkedOnly: only the marked records
	if t.markedOnly {
		if len(t.marks) == 0 {
			conditions = append(conditions, "1=0")
		} else {
			var keys []string
			for _, key := range t.marks {
				keys = append(keys, "({{ range $i, $f := .Table.Fields }}{{ if $f.PrimaryKey }}{{ $f.DBName }} = ?{{ if not (isLastPK $i $.Table.Fields) }} AND {{ end }}{{ end }}{{ end }})")
				args = append(args, key...)
			}
			conditions = append(conditions, "("+strings.Join(keys, " OR ")+")")
		}
	}

	// Find(">") / Find("<"): after/before the current record in the current key
	if t.seek != "" {
		conditions = append(conditions, t.seek)
		args = append(args, t.seekArgs...)
	}

	where := strings.Join(conditions, " AND ")
	if where == "" {
		where = "1=1"
	}

	return where, args
}

// allFilters returns the filters of all filter groups, the current group's first
func (t *{{ .StructName }}) allFilters() []*{{ lowerFirst .StructName }}FilterCondition {
	filters := make([]*{{ lowerFirst .StructName }}FilterCondition, 0, len(t.filters))
	for _, f := range t.filters {
		filters = append(filters, f)
	}
	groups := make([]int, 0, len(t.otherFilters))
	for g := range t.otherFilters {
		groups = append(groups, g)
	}
	slices.Sort(groups)
	for _, g := range groups {
		for _, f := range t.otherFilters[g] {
			filters = append(filters, f)
		}
	}
	return filters
}

// FilterGroup switches to filter group group and returns the current one; without an
// argument it only returns it (FilterGroup). Each group has its own filters — SetRange,
// SetFilter, GetFilter and CopyFilter work on the current group — and the filters of all
// groups apply together. Code sets filters the user must keep in group 2 and goes back to 0.
func (t *{{ .StructName }}) FilterGroup(group ...int) int {
	if len(group) == 0 || group[0] == t.filterGroup {
		return t.filterGroup
	}
	if t.otherFilters == nil {
		t.otherFilters = map[int]map[string]*{{ lowerFirst .StructName }}FilterCondition{}
	}
	if len(t.filters) > 0 {
		t.otherFilters[t.filterGroup] = t.filters
	} else {
		delete(t.otherFilters, t.filterGroup)
	}
	t.filterGroup = group[0]
	t.filters = t.otherFilters[t.filterGroup]
	delete(t.otherFilters, t.filterGroup)
	return t.filterGroup
}

// isKeyDateColumn: a Date column of the primary key (a blank date is types.BlankKeyDate)
func (t *{{ .StructName }}) isKeyDateColumn(column string) bool {
	switch column {
{{- range .Table.Fields }}
{{- if and .PrimaryKey (eq .Type "types.Date") }}
	case "{{ .DBName }}":
		return true
{{- end }}
{{- end }}
	}
	return false
}

// isBoolColumn: a Boolean column
func (t *{{ .StructName }}) isBoolColumn(column string) bool {
	switch column {
{{- range .Table.Fields }}
{{- if and (not .FlowField) (not .FlowFilter) (eq .Type "bool") }}
	case "{{ .DBName }}":
		return true
{{- end }}
{{- end }}
	}
	return false
}

// isDateColumn: a Date column (blank dates are NULL, in the primary key types.BlankKeyDate)
func (t *{{ .StructName }}) isDateColumn(column string) bool {
	switch column {
{{- range .Table.Fields }}
{{- if and (not .FlowField) (not .FlowFilter) (eq .Type "types.Date") }}
	case "{{ .DBName }}":
		return true
{{- end }}
{{- end }}
	}
	return false
}

// isDecimalColumn: a Decimal column
func (t *{{ .StructName }}) isDecimalColumn(column string) bool {
	switch column {
{{- range .Table.Fields }}
{{- if and (not .FlowField) (not .FlowFilter) (eq .Type "types.Decimal") }}
	case "{{ .DBName }}":
		return true
{{- end }}
{{- end }}
	}
	return false
}

// sqliteDecimal: decimals are stored as text on SQLite (exact), so comparisons and the order
// on them must be numeric there — as text 100 lies between 0 and 16.2
func (t *{{ .StructName }}) sqliteDecimal(column string) bool {
	return t.isDecimalColumn(column) && (t.dbType != database.DBTypePostgres || t.temp != nil)
}

// cmpColumn is a column as filters, keys and the order compare it (a decimal on SQLite as a number)
func (t *{{ .StructName }}) cmpColumn(column string) string {
	if t.sqliteDecimal(column) {
		return "CAST(" + column + " AS REAL)"
	}
	return column
}

// cmpParam is the placeholder of a value compared with a column (a decimal on SQLite as a number)
func (t *{{ .StructName }}) cmpParam(column string) string {
	if t.sqliteDecimal(column) {
		return "CAST(? AS REAL)"
	}
	return "?"
}

// parseFilterExpression parses filter expressions into SQL
// Supports: "100..200" (range), "100|200|300" (OR), "100..200|500" (combined)
func (t *{{ .StructName }}) parseFilterExpression(fieldName, expr string) (string, []interface{}) {
	var conditions []string
	var args []interface{}
	// Blank dates are stored as NULL and come before every date
	isDate := t.isDateColumn(fieldName)
	orBlankDate := func(condition string) string {
		if isDate {
			return fmt.Sprintf("(%s OR %s IS NULL)", condition, fieldName)
		}
		return condition
	}

	// a decimal on SQLite compares as a number
	col, param := t.cmpColumn(fieldName), t.cmpParam(fieldName)

	// Split by | (OR operator)
	orParts := strings.Split(expr, "|")

	for _, part := range orParts {
		part = strings.TrimSpace(part)

		// Check for range (..); an open end ("..x", "x..") has one bound
		if strings.Contains(part, "..") {
			rangeParts := strings.Split(part, "..")
			if len(rangeParts) == 2 {
				min := strings.TrimSpace(rangeParts[0])
				max := strings.TrimSpace(rangeParts[1])
				switch {
				case min == "" && max == "":
					conditions = append(conditions, "1=1")
				case min == "":
					conditions = append(conditions, orBlankDate(fmt.Sprintf("%s <= %s", col, param)))
					args = append(args, max)
				case max == "":
					conditions = append(conditions, fmt.Sprintf("%s >= %s", col, param))
					args = append(args, min)
				default:
					conditions = append(conditions, fmt.Sprintf("%s BETWEEN %s AND %s", col, param, param))
					args = append(args, min, max)
				}
			}
		} else if part == "''" {
			// '' is the blank value
			if isDate {
				conditions = append(conditions, fmt.Sprintf("(%s IS NULL OR %s = '%s')", fieldName, fieldName, types.BlankKeyDate))
			} else {
				conditions = append(conditions, fmt.Sprintf("%s = ?", fieldName))
				args = append(args, "")
			}
		} else if op := tables.FilterComparison(part); op != "" {
			// Comparison: <=, >=, <, > (a classic filter such as <3000)
			if op == "<" || op == "<=" {
				conditions = append(conditions, orBlankDate(fmt.Sprintf("%s %s %s", col, op, param)))
			} else {
				conditions = append(conditions, fmt.Sprintf("%s %s %s", col, op, param))
			}
			args = append(args, strings.TrimSpace(strings.TrimPrefix(part, op)))
		} else if strings.Contains(part, "*") {
			// Wildcard support: convert * to %
			likePattern := strings.ReplaceAll(part, "*", "%")
			conditions = append(conditions, fmt.Sprintf("%s LIKE ?", fieldName))
			args = append(args, likePattern)
		} else if strings.HasPrefix(part, "<>") {
			// Not equal
			value := strings.TrimSpace(strings.TrimPrefix(part, "<>"))
			if isDate && (value == "''" || value == "") {
				conditions = append(conditions, fmt.Sprintf("(%s IS NOT NULL AND %s <> '%s')", fieldName, fieldName, types.BlankKeyDate))
			} else {
				if value == "''" {
					value = ""
				}
				conditions = append(conditions, orBlankDate(fmt.Sprintf("%s <> %s", col, param)))
				args = append(args, value)
			}
		} else {
			// Exact match (a boolean column takes Yes/No, true/false, 1/0)
			conditions = append(conditions, fmt.Sprintf("%s = %s", col, param))
			if t.isBoolColumn(fieldName) {
				args = append(args, tables.FilterBool(part))
			} else {
				args = append(args, part)
			}
		}
	}

	// A date column compares its dates as the database keeps them (a closing date "C2026-12-31"
	// as "2026-12-31 23:59:59")
	if isDate {
		for i, a := range args {
			if v, ok := a.(string); ok {
				args[i] = types.FilterArg(v)
			}
		}
	}

	// Join with OR
	whereClause := "(" + strings.Join(conditions, " OR ") + ")"
	return whereClause, args
}

// getOrderByClause builds ORDER BY clause from current key and direction, with the
// arguments of its placeholders (a FlowField key sorts on its subquery, which may have
// some; they follow the WHERE arguments). The primary key columns always follow the
// current key, so the order is unique and paging (SetPage) never repeats or skips rows
// when the key has duplicate values.
func (t *{{ .StructName }}) getOrderByClause() (string, []interface{}) {
	pk := []string{ {{- range $i, $f := .Table.Fields }}{{ if $f.PrimaryKey }}"{{ $f.DBName }}", {{ end }}{{ end -}} }
	var columns []string
	var args []interface{}
	for _, column := range t.orderByFields {
{{- if .HasFilterableFlowField }}
		if name, ok := strings.CutPrefix(column, "flowfield:"); ok {
			expr, exprArgs := t.flowFieldExpr(name)
			columns = append(columns, expr)
			args = append(args, exprArgs...)
			continue
		}
{{- end }}
		columns = append(columns, t.cmpColumn(column))
	}
	for _, column := range pk {
		if !slices.Contains(t.orderByFields, column) {
			columns = append(columns, t.cmpColumn(column))
		}
	}
	if t.descending {
		for i := range columns {
			columns[i] += " DESC"
		}
	}
	return strings.Join(columns, ", "), args
}

// SetPage sets a pagination window for FindSet/FindSetBuffered: return at most
// limit rows, skipping the first offset rows. A limit of 0 disables pagination
// (all matching rows are returned). Negative values are treated as 0.
func (t *{{ .StructName }}) SetPage(limit, offset int) {
	if limit < 0 {
		limit = 0
	}
	if offset < 0 {
		offset = 0
	}
	t.limit = limit
	t.offset = offset
}

// getLimitClause builds the LIMIT/OFFSET clause from the pagination window.
// Returns an empty string when no limit is set. LIMIT/OFFSET take integer
// literals (not placeholders), which both SQLite and PostgreSQL accept.
func (t *{{ .StructName }}) getLimitClause() string {
	if t.limit <= 0 {
		return ""
	}
	if t.offset > 0 {
		return fmt.Sprintf(" LIMIT %d OFFSET %d", t.limit, t.offset)
	}
	return fmt.Sprintf(" LIMIT %d", t.limit)
}

// FindFirst finds the first record matching current filters
// Returns true if found, false if not found
func (t *{{ .StructName }}) FindFirst() bool {
{{- if .Table.Global }}
	tableName := {{ .StructName }}TableName
{{- else }}
	tableName := fmt.Sprintf("%s$%s", t.company, {{ .StructName }}TableName)
{{- end }}
	where, args := t.buildWhereClause()
	// The first record in the current key and direction (classic FIND('-'))
	orderBy, orderArgs := t.getOrderByClause()
	args = append(args, orderArgs...)

	// Build SELECT with all fields
	query := fmt.Sprintf(` + "`SELECT {{ range $i, $f := .Table.Fields }}{{ if not $f.FlowField }}{{ $f.DBName }}{{ if not (isLastDBField $i $.Table.Fields) }}, {{ end }}{{ end }}{{ end }} FROM \"%s\" WHERE %s ORDER BY %s LIMIT 1`" + `, tableName, where, orderBy)

	// Convert placeholders for PostgreSQL
	query = t.convertPlaceholders(query, len(args))
	query += t.forUpdate()

{{- range .Table.Fields }}
{{- if not .FlowField }}
{{- if eq .Type "types.Code" }}
	var {{ .Name }}Null sql.NullString
{{- else if eq .Type "types.Text" }}
	var {{ .Name }}Null sql.NullString
{{- else if eq .Type "types.Decimal" }}
	var {{ .Name }}Null sql.NullString
{{- else if eq .Type "types.Date" }}
	var {{ .Name }}Null sql.NullString
{{- else if eq .Type "types.DateTime" }}
	var {{ .Name }}Null sql.NullString
{{- else if eq .Type "bool" }}
	var {{ .Name }}Bool sql.NullBool
{{- else if eq .Type "Option" }}
	var {{ .Name }}Int int
{{- else if eq .Type "time.Time" }}
	var {{ .Name }}Time time.Time
{{- end }}
{{- end }}
{{- end }}

	err := t.exec().QueryRow(query, args...).Scan(
{{- range $i, $f := .Table.Fields }}
{{- if not $f.FlowField }}
{{- if eq $f.Type "types.Code" }}
		&{{ $f.Name }}Null,
{{- else if eq $f.Type "types.Text" }}
		&{{ $f.Name }}Null,
{{- else if eq $f.Type "types.Decimal" }}
		&{{ $f.Name }}Null,
{{- else if eq $f.Type "types.Date" }}
		&{{ $f.Name }}Null,
{{- else if eq $f.Type "types.DateTime" }}
		&{{ $f.Name }}Null,
{{- else if eq $f.Type "bool" }}
		&{{ $f.Name }}Bool,
{{- else if eq $f.Type "Option" }}
		&{{ $f.Name }}Int,
{{- else if eq $f.Type "time.Time" }}
		&{{ $f.Name }}Time,
{{- else }}
		&t.{{ upperFirst $f.Name }},
{{- end }}
{{- end }}
{{- end }}
	)

	if err != nil {
		if err == sql.ErrNoRows {
			return false
		}
		fmt.Printf("Error: Failed to find first {{ .Table.Name }}: %v\n", err)
		return false
	}

	// Populate fields
{{- range .Table.Fields }}
{{- if not .FlowField }}
{{- if eq .Type "types.Code" }}
	t.{{ upperFirst .Name }} = types.NewCode({{ .Name }}Null.String)
{{- else if eq .Type "types.Text" }}
	t.{{ upperFirst .Name }} = types.NewText({{ .Name }}Null.String)
{{- else if eq .Type "types.Decimal" }}
	t.{{ upperFirst .Name }}, _ = types.NewDecimalFromString({{ .Name }}Null.String)
{{- else if eq .Type "types.Date" }}
	t.{{ upperFirst .Name }}, _ = types.NewDateFromString({{ .Name }}Null.String)
{{- else if eq .Type "types.DateTime" }}
	t.{{ upperFirst .Name }}, _ = types.NewDateTimeFromString({{ .Name }}Null.String)
{{- else if eq .Type "bool" }}
	t.{{ upperFirst .Name }} = {{ .Name }}Bool.Bool
{{- else if eq .Type "Option" }}
	t.{{ upperFirst .Name }} = {{ $.StructName }}{{ upperFirst .Name }}({{ .Name }}Int)
{{- else if eq .Type "time.Time" }}
	t.{{ upperFirst .Name }} = {{ .Name }}Time
{{- end }}
{{- end }}
{{- end }}

	// Store old values for field tracking
	t.StoreOldValues()

	return true
}

// FindLast finds the last record matching current filters
// Returns true if found, false if not found
func (t *{{ .StructName }}) FindLast() bool {
{{- if .Table.Global }}
	tableName := {{ .StructName }}TableName
{{- else }}
	tableName := fmt.Sprintf("%s$%s", t.company, {{ .StructName }}TableName)
{{- end }}
	where, args := t.buildWhereClause()
	// The last record in the current key and direction (classic FIND('+')): the key reversed,
	// every column of it
	t.descending = !t.descending
	orderBy, orderArgs := t.getOrderByClause()
	t.descending = !t.descending
	args = append(args, orderArgs...)

	// Build SELECT with all fields
	query := fmt.Sprintf(` + "`SELECT {{ range $i, $f := .Table.Fields }}{{ if not $f.FlowField }}{{ $f.DBName }}{{ if not (isLastDBField $i $.Table.Fields) }}, {{ end }}{{ end }}{{ end }} FROM \"%s\" WHERE %s ORDER BY %s LIMIT 1`" + `, tableName, where, orderBy)

	// Convert placeholders for PostgreSQL
	query = t.convertPlaceholders(query, len(args))
	query += t.forUpdate()

{{- range .Table.Fields }}
{{- if not .FlowField }}
{{- if eq .Type "types.Code" }}
	var {{ .Name }}Null sql.NullString
{{- else if eq .Type "types.Text" }}
	var {{ .Name }}Null sql.NullString
{{- else if eq .Type "types.Decimal" }}
	var {{ .Name }}Null sql.NullString
{{- else if eq .Type "types.Date" }}
	var {{ .Name }}Null sql.NullString
{{- else if eq .Type "types.DateTime" }}
	var {{ .Name }}Null sql.NullString
{{- else if eq .Type "bool" }}
	var {{ .Name }}Bool sql.NullBool
{{- else if eq .Type "Option" }}
	var {{ .Name }}Int int
{{- else if eq .Type "time.Time" }}
	var {{ .Name }}Time time.Time
{{- end }}
{{- end }}
{{- end }}

	err := t.exec().QueryRow(query, args...).Scan(
{{- range $i, $f := .Table.Fields }}
{{- if not $f.FlowField }}
{{- if eq $f.Type "types.Code" }}
		&{{ $f.Name }}Null,
{{- else if eq $f.Type "types.Text" }}
		&{{ $f.Name }}Null,
{{- else if eq $f.Type "types.Decimal" }}
		&{{ $f.Name }}Null,
{{- else if eq $f.Type "types.Date" }}
		&{{ $f.Name }}Null,
{{- else if eq $f.Type "types.DateTime" }}
		&{{ $f.Name }}Null,
{{- else if eq $f.Type "bool" }}
		&{{ $f.Name }}Bool,
{{- else if eq $f.Type "Option" }}
		&{{ $f.Name }}Int,
{{- else if eq $f.Type "time.Time" }}
		&{{ $f.Name }}Time,
{{- else }}
		&t.{{ upperFirst $f.Name }},
{{- end }}
{{- end }}
{{- end }}
	)

	if err != nil {
		if err == sql.ErrNoRows {
			return false
		}
		fmt.Printf("Error: Failed to find last {{ .Table.Name }}: %v\n", err)
		return false
	}

	// Populate fields
{{- range .Table.Fields }}
{{- if not .FlowField }}
{{- if eq .Type "types.Code" }}
	t.{{ upperFirst .Name }} = types.NewCode({{ .Name }}Null.String)
{{- else if eq .Type "types.Text" }}
	t.{{ upperFirst .Name }} = types.NewText({{ .Name }}Null.String)
{{- else if eq .Type "types.Decimal" }}
	t.{{ upperFirst .Name }}, _ = types.NewDecimalFromString({{ .Name }}Null.String)
{{- else if eq .Type "types.Date" }}
	t.{{ upperFirst .Name }}, _ = types.NewDateFromString({{ .Name }}Null.String)
{{- else if eq .Type "types.DateTime" }}
	t.{{ upperFirst .Name }}, _ = types.NewDateTimeFromString({{ .Name }}Null.String)
{{- else if eq .Type "bool" }}
	t.{{ upperFirst .Name }} = {{ .Name }}Bool.Bool
{{- else if eq .Type "Option" }}
	t.{{ upperFirst .Name }} = {{ $.StructName }}{{ upperFirst .Name }}({{ .Name }}Int)
{{- else if eq .Type "time.Time" }}
	t.{{ upperFirst .Name }} = {{ .Name }}Time
{{- end }}
{{- end }}
{{- end }}

	// Store old values for field tracking
	t.StoreOldValues()

	return true
}

// Count returns the number of records matching current filters
func (t *{{ .StructName }}) Count() int {
{{- if .Table.Global }}
	tableName := {{ .StructName }}TableName
{{- else }}
	tableName := fmt.Sprintf("%s$%s", t.company, {{ .StructName }}TableName)
{{- end }}
	where, args := t.buildWhereClause()

	query := fmt.Sprintf(` + "`SELECT COUNT(*) FROM \"%s\" WHERE %s`" + `, tableName, where)

	// Convert placeholders for PostgreSQL
	query = t.convertPlaceholders(query, len(args))

	var count int
	err := t.exec().QueryRow(query, args...).Scan(&count)
	if err != nil {
		fmt.Printf("Error: Failed to count {{ .Table.Name }}: %v\n", err)
		return 0
	}

	return count
}

// FindSet opens a result set matching current filters
// Call Next() to iterate through the results
// Returns true if at least one record found, false otherwise
func (t *{{ .StructName }}) FindSet() bool {
	// A temporary record, and a record in a transaction, reads all at once: its in-memory
	// database or the transaction has one connection, which an open result set would block for
	// the statements of the loop (Modify while looping; on PostgreSQL a query while the rows are
	// read breaks the connection — "unexpected Parse response")
	if _, inTx := t.db.(*sql.Tx); t.temp != nil || inTx {
		return t.FindSetBuffered()
	}
	// Close any existing result set
	if t.currentRows != nil {
		t.currentRows.Close()
		t.currentRows = nil
	}
	t.bufferedRecords = nil

{{- if .Table.Global }}
	tableName := {{ .StructName }}TableName
{{- else }}
	tableName := fmt.Sprintf("%s$%s", t.company, {{ .StructName }}TableName)
{{- end }}
	where, args := t.buildWhereClause()
	orderBy, orderArgs := t.getOrderByClause()
	args = append(args, orderArgs...)

	// Build SELECT with all fields
	query := fmt.Sprintf(` + "`SELECT {{ range $i, $f := .Table.Fields }}{{ if not $f.FlowField }}{{ $f.DBName }}{{ if not (isLastDBField $i $.Table.Fields) }}, {{ end }}{{ end }}{{ end }} FROM \"%s\" WHERE %s ORDER BY %s%s`" + `, tableName, where, orderBy, t.getLimitClause())

	// Convert placeholders for PostgreSQL
	query = t.convertPlaceholders(query, len(args))
	query += t.forUpdate()

	rows, err := t.exec().Query(query, args...)
	if err != nil {
		fmt.Printf("Error: Failed to execute FindSet for {{ .Table.Name }}: %v\n", err)
		return false
	}

	t.currentRows = rows

	// Load first record
	return t.Next()
}

// Next advances to the next record: in the result set after FindSet() or FindSetBuffered(),
// else from the current record in the current key (after Get, Find, FindFirst, FindLast)
// Optional steps parameter:
//   - Next() or Next(1): Move forward 1 record (default)
//   - Next(5): Skip forward 5 records
//   - Next(-1): Move backward 1 record (FindSetBuffered, or after Get/Find)
//   - Next(-3): Skip backward 3 records (FindSetBuffered, or after Get/Find)
// Returns true if a record was loaded, false if no more records or out of bounds
func (t *{{ .StructName }}) Next(steps ...int) bool {
	// Default to 1 step forward
	step := 1
	if len(steps) > 0 {
		step = steps[0]
	}

	// BUFFERED MODE: Bidirectional navigation with in-memory records
	if t.bufferedRecords != nil {
		// Calculate new position
		newPos := t.currentBufferPos + step

		// Check bounds
		if newPos < 0 || newPos >= len(t.bufferedRecords) {
			return false // Out of bounds
		}

		// Move to new position
		t.currentBufferPos = newPos
		t.copyFromBuffered(t.bufferedRecords[t.currentBufferPos])
		return true
	}

	// FORWARD-ONLY MODE: Streaming with sql.Rows (only positive steps allowed)
	if t.currentRows != nil {
		// Validate: only forward movement allowed
		if step < 1 {
			fmt.Printf("Error: Backward navigation (Next(%d)) requires FindSetBuffered()\n", step)
			return false
		}

		// Advance 'step' times (1 = next record, 2 = skip 1 record, etc.)
		for i := 0; i < step; i++ {
			if !t.currentRows.Next() {
				// No more rows - close result set
				t.currentRows.Close()
				t.currentRows = nil
				return false
			}
		}

		// Scan the row
{{- range .Table.Fields }}
{{- if not .FlowField }}
{{- if eq .Type "types.Code" }}
		var {{ .Name }}Null sql.NullString
{{- else if eq .Type "types.Text" }}
		var {{ .Name }}Null sql.NullString
{{- else if eq .Type "types.Decimal" }}
		var {{ .Name }}Null sql.NullString
{{- else if eq .Type "types.Date" }}
		var {{ .Name }}Null sql.NullString
{{- else if eq .Type "types.DateTime" }}
		var {{ .Name }}Null sql.NullString
{{- else if eq .Type "bool" }}
		var {{ .Name }}Bool sql.NullBool
{{- else if eq .Type "Option" }}
		var {{ .Name }}Int int
{{- else if eq .Type "time.Time" }}
		var {{ .Name }}Time time.Time
{{- end }}
{{- end }}
{{- end }}

		err := t.currentRows.Scan(
{{- range $i, $f := .Table.Fields }}
{{- if not $f.FlowField }}
{{- if eq $f.Type "types.Code" }}
			&{{ $f.Name }}Null,
{{- else if eq $f.Type "types.Text" }}
			&{{ $f.Name }}Null,
{{- else if eq $f.Type "types.Decimal" }}
			&{{ $f.Name }}Null,
{{- else if eq $f.Type "types.Date" }}
			&{{ $f.Name }}Null,
{{- else if eq $f.Type "types.DateTime" }}
			&{{ $f.Name }}Null,
{{- else if eq $f.Type "bool" }}
			&{{ $f.Name }}Bool,
{{- else if eq $f.Type "Option" }}
			&{{ $f.Name }}Int,
{{- else if eq $f.Type "time.Time" }}
			&{{ $f.Name }}Time,
{{- else }}
			&t.{{ upperFirst $f.Name }},
{{- end }}
{{- end }}
{{- end }}
		)

		if err != nil {
			fmt.Printf("Error: Failed to scan {{ .Table.Name }} record: %v\n", err)
			t.currentRows.Close()
			t.currentRows = nil
			return false
		}

		// Populate fields
{{- range .Table.Fields }}
{{- if not .FlowField }}
{{- if eq .Type "types.Code" }}
		t.{{ upperFirst .Name }} = types.NewCode({{ .Name }}Null.String)
{{- else if eq .Type "types.Text" }}
		t.{{ upperFirst .Name }} = types.NewText({{ .Name }}Null.String)
{{- else if eq .Type "types.Decimal" }}
		t.{{ upperFirst .Name }}, _ = types.NewDecimalFromString({{ .Name }}Null.String)
{{- else if eq .Type "types.Date" }}
		t.{{ upperFirst .Name }}, _ = types.NewDateFromString({{ .Name }}Null.String)
{{- else if eq .Type "types.DateTime" }}
		t.{{ upperFirst .Name }}, _ = types.NewDateTimeFromString({{ .Name }}Null.String)
{{- else if eq .Type "bool" }}
		t.{{ upperFirst .Name }} = {{ .Name }}Bool.Bool
{{- else if eq .Type "Option" }}
		t.{{ upperFirst .Name }} = {{ $.StructName }}{{ upperFirst .Name }}({{ .Name }}Int)
{{- else if eq .Type "time.Time" }}
		t.{{ upperFirst .Name }} = {{ .Name }}Time
{{- end }}
{{- end }}
{{- end }}

		// Store old values for field tracking
		t.StoreOldValues()

		return true
	}

	// No record set (after Get, Find, FindFirst, FindLast): the next record in the current key
	// and direction within the filters, one step at a time (classic NEXT after FIND); a step
	// that finds none leaves the record on the last one reached
	if step == 0 {
		return false
	}
	for i := 0; i != step; {
		if step > 0 {
			if !t.findInKey(">", false) {
				return false
			}
			i++
		} else {
			if !t.findInKey("<", true) {
				return false
			}
			i--
		}
	}
	return true
}

// FindSetBuffered loads all filtered records into memory for bidirectional navigation
// Use this when you need to move backward/forward with Next(steps)
// Filters (SetRange/SetFilter) are applied in SQL before buffering to minimize memory usage
// Returns true if at least one record found, false otherwise
func (t *{{ .StructName }}) FindSetBuffered() bool {
	// Close any existing forward-only result set
	if t.currentRows != nil {
		t.currentRows.Close()
		t.currentRows = nil
	}

	// Clear any existing buffer
	t.bufferedRecords = nil
	t.currentBufferPos = -1

{{- if .Table.Global }}
	tableName := {{ .StructName }}TableName
{{- else }}
	tableName := fmt.Sprintf("%s$%s", t.company, {{ .StructName }}TableName)
{{- end }}
	where, args := t.buildWhereClause()
	orderBy, orderArgs := t.getOrderByClause()
	args = append(args, orderArgs...)

	// Build SELECT with all fields
	query := fmt.Sprintf(` + "`SELECT {{ range $i, $f := .Table.Fields }}{{ if not $f.FlowField }}{{ $f.DBName }}{{ if not (isLastDBField $i $.Table.Fields) }}, {{ end }}{{ end }}{{ end }} FROM \"%s\" WHERE %s ORDER BY %s%s`" + `, tableName, where, orderBy, t.getLimitClause())

	// Convert placeholders for PostgreSQL
	query = t.convertPlaceholders(query, len(args))

	rows, err := t.exec().Query(query, args...)
	if err != nil {
		fmt.Printf("Error: Failed to execute FindSetBuffered for {{ .Table.Name }}: %v\n", err)
		return false
	}
	defer rows.Close()

	// Load all records into memory
	for rows.Next() {
		// Create a new record instance
		record := &{{ .StructName }}{}
		record.db = t.db
		record.temp = t.temp
		record.company = t.company
		record.dbType = t.dbType

		// Scan the row
{{- range .Table.Fields }}
{{- if not .FlowField }}
{{- if eq .Type "types.Code" }}
		var {{ .Name }}Null sql.NullString
{{- else if eq .Type "types.Text" }}
		var {{ .Name }}Null sql.NullString
{{- else if eq .Type "types.Decimal" }}
		var {{ .Name }}Null sql.NullString
{{- else if eq .Type "types.Date" }}
		var {{ .Name }}Null sql.NullString
{{- else if eq .Type "types.DateTime" }}
		var {{ .Name }}Null sql.NullString
{{- else if eq .Type "bool" }}
		var {{ .Name }}Bool sql.NullBool
{{- else if eq .Type "Option" }}
		var {{ .Name }}Int int
{{- else if eq .Type "time.Time" }}
		var {{ .Name }}Time time.Time
{{- end }}
{{- end }}
{{- end }}

		err := rows.Scan(
{{- range $i, $f := .Table.Fields }}
{{- if not $f.FlowField }}
{{- if eq $f.Type "types.Code" }}
			&{{ $f.Name }}Null,
{{- else if eq $f.Type "types.Text" }}
			&{{ $f.Name }}Null,
{{- else if eq $f.Type "types.Decimal" }}
			&{{ $f.Name }}Null,
{{- else if eq $f.Type "types.Date" }}
			&{{ $f.Name }}Null,
{{- else if eq $f.Type "types.DateTime" }}
			&{{ $f.Name }}Null,
{{- else if eq $f.Type "bool" }}
			&{{ $f.Name }}Bool,
{{- else if eq $f.Type "Option" }}
			&{{ $f.Name }}Int,
{{- else if eq $f.Type "time.Time" }}
			&{{ $f.Name }}Time,
{{- else }}
			&record.{{ upperFirst $f.Name }},
{{- end }}
{{- end }}
{{- end }}
		)

		if err != nil {
			fmt.Printf("Error: Failed to scan {{ .Table.Name }} record: %v\n", err)
			return false
		}

		// Populate special type fields
{{- range .Table.Fields }}
{{- if not .FlowField }}
{{- if eq .Type "types.Code" }}
		record.{{ upperFirst .Name }} = types.NewCode({{ .Name }}Null.String)
{{- else if eq .Type "types.Text" }}
		record.{{ upperFirst .Name }} = types.NewText({{ .Name }}Null.String)
{{- else if eq .Type "types.Decimal" }}
		record.{{ upperFirst .Name }}, _ = types.NewDecimalFromString({{ .Name }}Null.String)
{{- else if eq .Type "types.Date" }}
		record.{{ upperFirst .Name }}, _ = types.NewDateFromString({{ .Name }}Null.String)
{{- else if eq .Type "types.DateTime" }}
		record.{{ upperFirst .Name }}, _ = types.NewDateTimeFromString({{ .Name }}Null.String)
{{- else if eq .Type "bool" }}
		record.{{ upperFirst .Name }} = {{ .Name }}Bool.Bool
{{- else if eq .Type "Option" }}
		record.{{ upperFirst .Name }} = {{ $.StructName }}{{ upperFirst .Name }}({{ .Name }}Int)
{{- else if eq .Type "time.Time" }}
		record.{{ upperFirst .Name }} = {{ .Name }}Time
{{- end }}
{{- end }}
{{- end }}

		// Store old values
		record.StoreOldValues()

		// Add to buffer
		t.bufferedRecords = append(t.bufferedRecords, record)
	}

	// Check for errors during iteration
	if err := rows.Err(); err != nil {
		fmt.Printf("Error: Failed to iterate {{ .Table.Name }} records: %v\n", err)
		return false
	}

	// If no records found, return false
	if len(t.bufferedRecords) == 0 {
		return false
	}

	// Load first record into current instance
	t.currentBufferPos = 0
	t.copyFromBuffered(t.bufferedRecords[0])

	return true
}

// copyFromBuffered copies field values from a buffered record to the current instance
func (t *{{ .StructName }}) copyFromBuffered(record *{{ .StructName }}) {
{{- range .Table.Fields }}
	t.{{ upperFirst .Name }} = record.{{ upperFirst .Name }}
{{- end }}
	t.StoreOldValues()
}

// ========================================
// Phase 3: Advanced Methods
// ========================================

// IsEmpty returns true if no records match current filters
func (t *{{ .StructName }}) IsEmpty() bool {
	return t.Count() == 0
}

// ModifyAll updates a field for all records matching current filters
// Returns the number of records modified
func (t *{{ .StructName }}) ModifyAll(fieldName string, newValue interface{}) int {
{{- if .Table.Global }}
	tableName := {{ .StructName }}TableName
{{- else }}
	tableName := fmt.Sprintf("%s$%s", t.company, {{ .StructName }}TableName)
{{- end }}
	column, ok := t.columnName(fieldName)
	if !ok {
		fmt.Printf("Error: ModifyAll on unknown field %q of {{ .Table.Name }}\n", fieldName)
		return 0
	}
	where, args := t.buildWhereClause()

	// Build UPDATE SQL
	updateSQL := fmt.Sprintf(` + "`UPDATE \"%s\" SET %s = ? WHERE %s`" + `, tableName, column, where)

	// Prepend newValue to args
	allArgs := append([]interface{}{newValue}, args...)

	// Convert placeholders for PostgreSQL
	updateSQL = t.convertPlaceholders(updateSQL, len(allArgs))

	result, err := t.exec().Exec(updateSQL, allArgs...)
	if err != nil {
		fmt.Printf("Error: Failed to modify all {{ .Table.Name }}: %v\n", err)
		return 0
	}

	rowsAffected, _ := result.RowsAffected()
	return int(rowsAffected)
}

// DeleteAll deletes all records matching current filters
// Returns the number of records deleted
func (t *{{ .StructName }}) DeleteAll() int {
{{- if .Table.Global }}
	tableName := {{ .StructName }}TableName
{{- else }}
	tableName := fmt.Sprintf("%s$%s", t.company, {{ .StructName }}TableName)
{{- end }}
	where, args := t.buildWhereClause()

	// Build DELETE SQL
	deleteSQL := fmt.Sprintf(` + "`DELETE FROM \"%s\" WHERE %s`" + `, tableName, where)

	// Convert placeholders for PostgreSQL
	deleteSQL = t.convertPlaceholders(deleteSQL, len(args))

	result, err := t.exec().Exec(deleteSQL, args...)
	if err != nil {
		fmt.Printf("Error: Failed to delete all {{ .Table.Name }}: %v\n", err)
		return 0
	}

	rowsAffected, _ := result.RowsAffected()
	return int(rowsAffected)
}

// CopyFilters copies filters from another record variable (all filter groups)
func (t *{{ .StructName }}) CopyFilters(from *{{ .StructName }}) {
	t.filters = copy{{ .StructName }}Filters(from.filters)
	t.filterGroup = from.filterGroup
	t.otherFilters = nil
	for g, filters := range from.otherFilters {
		if t.otherFilters == nil {
			t.otherFilters = map[int]map[string]*{{ lowerFirst .StructName }}FilterCondition{}
		}
		t.otherFilters[g] = copy{{ .StructName }}Filters(filters)
	}

	// Also copy order by fields
	if len(from.orderByFields) > 0 {
		t.orderByFields = make([]string, len(from.orderByFields))
		copy(t.orderByFields, from.orderByFields)
	} else {
		t.orderByFields = nil
	}
}

// copy{{ .StructName }}Filters returns a deep copy of a filter group's filters (nil for none)
func copy{{ .StructName }}Filters(from map[string]*{{ lowerFirst .StructName }}FilterCondition) map[string]*{{ lowerFirst .StructName }}FilterCondition {
	if from == nil {
		return nil
	}
	filters := make(map[string]*{{ lowerFirst .StructName }}FilterCondition, len(from))
	for key, filter := range from {
		c := *filter
		filters[key] = &c
	}
	return filters
}

// GetFilters returns a string representation of current filters
// Useful for debugging and logging
func (t *{{ .StructName }}) GetFilters() string {
	if len(t.filters) == 0 {
		return ""
	}

	var parts []string
	for _, filter := range t.filters {
		if filter.invalidField {
			parts = append(parts, "<invalid field>")
		} else if filter.isExpression {
			parts = append(parts, fmt.Sprintf("%s: %s", filter.fieldName, filter.filterExpr))
		} else if filter.minValue != nil && filter.maxValue != nil {
			parts = append(parts, fmt.Sprintf("%s: %v..%v", filter.fieldName, filter.minValue, filter.maxValue))
		} else if filter.minValue != nil {
			parts = append(parts, fmt.Sprintf("%s: >=%v", filter.fieldName, filter.minValue))
		} else if filter.maxValue != nil {
			parts = append(parts, fmt.Sprintf("%s: <=%v", filter.fieldName, filter.maxValue))
		}
	}

	return strings.Join(parts, ", ")
}

// ========================================
// Field Validation
// ========================================

// ValidateField validates a field and calls its OnValidate trigger
// This is equivalent to the VALIDATE function
// Usage: customer.ValidateField("Payment_terms_code", types.NewCode("30DAYS"))
func (t *{{ .StructName }}) ValidateField(fieldName string, value interface{}) error {
	fieldNameLower := strings.ToLower(fieldName)

	switch fieldNameLower {
{{- range .Table.Fields }}
{{- if not .FlowField }}
	case "{{ .DBName }}":
		// Set field value
{{- if eq .Type "types.Code" }}
		if v, ok := value.(types.Code); ok {
			t.{{ upperFirst .Name }} = v
		} else if v, ok := value.(string); ok {
			t.{{ upperFirst .Name }} = types.NewCode(v)
		} else {
			return fmt.Errorf("invalid type for field {{ .Name }}")
		}
{{- if .DateFormula }}
		if err := tables.CheckDateFormula({{ $.StructName }}TableName, "{{ .Name }}", string(t.{{ upperFirst .Name }})); err != nil {
			return err
		}
{{- end }}
{{- else if eq .Type "types.Text" }}
		if v, ok := value.(types.Text); ok {
			t.{{ upperFirst .Name }} = v
		} else if v, ok := value.(string); ok {
			t.{{ upperFirst .Name }} = types.NewText(v)
		} else {
			return fmt.Errorf("invalid type for field {{ .Name }}")
		}
{{- else if eq .Type "types.Decimal" }}
		if v, ok := value.(types.Decimal); ok {
			t.{{ upperFirst .Name }} = v
		} else if v, ok := value.(string); ok {
			var err error
			t.{{ upperFirst .Name }}, err = types.NewDecimalFromString(v)
			if err != nil {
				return fmt.Errorf("invalid decimal value for field {{ .Name }}: %w", err)
			}
		} else if v, ok := value.(float64); ok {
			t.{{ upperFirst .Name }} = types.NewDecimal(v)
		} else if v, ok := value.(int); ok {
			t.{{ upperFirst .Name }} = types.NewDecimalFromInt(int64(v))
		} else if v, ok := value.(int64); ok {
			t.{{ upperFirst .Name }} = types.NewDecimalFromInt(v)
		} else {
			return fmt.Errorf("invalid type for field {{ .Name }} (expected Decimal, string, float64, int, or int64)")
		}
{{- else if eq .Type "types.Date" }}
		if v, ok := value.(types.Date); ok {
			t.{{ upperFirst .Name }} = v
		} else if v, ok := value.(string); ok {
			var err error
			t.{{ upperFirst .Name }}, err = types.NewDateFromString(v)
			if err != nil {
				return fmt.Errorf("invalid date value for field {{ .Name }}: %w", err)
			}
		} else if v, ok := value.(time.Time); ok {
			t.{{ upperFirst .Name }} = types.NewDateFromTime(v)
		} else {
			return fmt.Errorf("invalid type for field {{ .Name }} (expected Date, string, or time.Time)")
		}
{{- else if eq .Type "types.DateTime" }}
		if v, ok := value.(types.DateTime); ok {
			t.{{ upperFirst .Name }} = v
		} else if v, ok := value.(string); ok {
			var err error
			t.{{ upperFirst .Name }}, err = types.NewDateTimeFromString(v)
			if err != nil {
				return fmt.Errorf("invalid datetime value for field {{ .Name }}: %w", err)
			}
		} else if v, ok := value.(time.Time); ok {
			t.{{ upperFirst .Name }} = types.NewDateTimeFromTime(v)
		} else {
			return fmt.Errorf("invalid type for field {{ .Name }} (expected DateTime, string, or time.Time)")
		}
{{- else if eq .Type "[]byte" }}
		// Accept nil, []byte, or base64-encoded string
		if value == nil {
			t.{{ upperFirst .Name }} = nil
		} else if v, ok := value.([]byte); ok {
			t.{{ upperFirst .Name }} = v
		} else if s, ok := value.(string); ok {
			if s == "" {
				t.{{ upperFirst .Name }} = nil
			} else {
				// Try base64 decode
				decoded, err := base64.StdEncoding.DecodeString(s)
				if err != nil {
					return fmt.Errorf("invalid base64 value for field {{ .Name }}: %w", err)
				}
				t.{{ upperFirst .Name }} = decoded
			}
		} else {
			return fmt.Errorf("invalid type for field {{ .Name }} (expected []byte, string, or nil)")
		}
{{- else if eq .Type "bool" }}
		if v, ok := value.(bool); ok {
			t.{{ upperFirst .Name }} = v
		} else if v, ok := value.(string); ok {
			// Handle string boolean values from JSON/frontend
			t.{{ upperFirst .Name }} = v == "true" || v == "1"
		} else {
			return fmt.Errorf("invalid type for field {{ .Name }}")
		}
{{- else if eq .Type "Option" }}
		// Accept enum type directly
		if v, ok := value.({{ $.StructName }}{{ upperFirst .Name }}); ok {
			t.{{ upperFirst .Name }} = v
		// Accept int (convert to enum)
		} else if v, ok := value.(int); ok {
			if v < 0 || v >= {{ len .Options }} {
				return fmt.Errorf("invalid option value %d for field {{ .Name }} (valid range: 0-%d)", v, {{ len .Options }}-1)
			}
			t.{{ upperFirst .Name }} = {{ $.StructName }}{{ upperFirst .Name }}(v)
		// Accept float64 (JSON numbers decode as float64)
		} else if v, ok := value.(float64); ok {
			intVal := int(v)
			if intVal < 0 || intVal >= {{ len .Options }} {
				return fmt.Errorf("invalid option value %d for field {{ .Name }} (valid range: 0-%d)", intVal, {{ len .Options }}-1)
			}
			t.{{ upperFirst .Name }} = {{ $.StructName }}{{ upperFirst .Name }}(intVal)
		// Accept string (lookup in options and convert)
		} else if v, ok := value.(string); ok {
			if v == "" {
				// Empty string defaults to first option (behavior)
				t.{{ upperFirst .Name }} = 0
			} else {
				options := []string{ {{- range $i, $opt := .Options }}{{- if $i }}, {{ end }}"{{ $opt }}"{{- end }} }
				found := false
				for i, opt := range options {
					if opt == v {
						t.{{ upperFirst .Name }} = {{ $.StructName }}{{ upperFirst .Name }}(i)
						found = true
						break
					}
				}
				if !found {
					return fmt.Errorf("invalid option '%s' for field {{ .Name }} (valid options: %v)", v, options)
				}
			}
		} else {
			return fmt.Errorf("invalid type for field {{ .Name }} (expected {{ $.StructName }}{{ upperFirst .Name }}, int, or string)")
		}
{{- else if eq .Type "int" }}
		switch v := value.(type) {
		case int:
			t.{{ upperFirst .Name }} = v
		case float64:
			t.{{ upperFirst .Name }} = int(v)
		case string:
			if v == "" {
				t.{{ upperFirst .Name }} = 0
			} else if i, err := strconv.Atoi(v); err == nil {
				t.{{ upperFirst .Name }} = i
			} else {
				return fmt.Errorf("invalid integer value for field {{ .Name }}: %s", v)
			}
		default:
			return fmt.Errorf("invalid type for field {{ .Name }}")
		}
{{- else if eq .Type "time.Time" }}
		if v, ok := value.(time.Time); ok {
			t.{{ upperFirst .Name }} = v
		} else {
			return fmt.Errorf("invalid type for field {{ .Name }}")
		}
{{- end }}
{{- if .TableRelation }}
		// TableRelation: the value must exist in the related table (before OnValidate)
		if err := tables.CheckRelation(t, t.db, t.company, t.dbType, "{{ .DBName }}"); err != nil {
			return err
		}
{{- end }}
		// OnValidate trigger of the business logic (OnValidate of the field)
		for _, l := range t.registeredLogic() {
			if trg, ok := l.({{ $.StructName }}OnValidate_{{ upperFirst .Name }}); ok {
				if err := trg.OnValidate_{{ upperFirst .Name }}(t); err != nil {
					return err
				}
			}
		}
		return nil
{{- end }}
{{- end }}
	}

	return fmt.Errorf("field '%s' not found", fieldName)
}


// ========================================
// Classic record functions
// ========================================

// FieldValue returns the value of a stored field by its number (FlowFields are not stored)
func (t *{{ .StructName }}) FieldValue(no int) (any, bool) {
	switch no {
{{- range .Table.Fields }}
{{- if not .FlowField }}
	case {{ .No }}:
		return t.{{ upperFirst .Name }}, true
{{- end }}
{{- end }}
	}
	return nil, false
}

// flowFieldValue returns the value of a FlowField by its number, as CalcFields left it
func (t *{{ .StructName }}) flowFieldValue(no int) (any, bool) {
	switch no {
{{- range .Table.Fields }}
{{- if .FlowField }}
	case {{ .No }}:
		return t.{{ upperFirst .Name }}, true
{{- end }}
{{- end }}
	}
	return nil, false
}

// SetFieldValue sets a stored field by its number when the value has the field's type
// (options: any option value or int); it reports whether the field was set
func (t *{{ .StructName }}) SetFieldValue(no int, value any) bool {
	switch no {
{{- range .Table.Fields }}
{{- if not .FlowField }}
	case {{ .No }}:
{{- if eq .Type "Option" }}
		if i, ok := tables.OptionIndex(value); ok && i >= 0 && i < {{ len .Options }} {
			t.{{ upperFirst .Name }} = {{ $.StructName }}{{ upperFirst .Name }}(i)
			return true
		}
{{- else if eq .Type "BLOB" }}
		if v, ok := value.([]byte); ok {
			t.{{ upperFirst .Name }} = v
			return true
		}
{{- else }}
		if v, ok := value.({{ .Type }}); ok {
			t.{{ upperFirst .Name }} = v
			return true
		}
{{- if eq .Type "types.Code" }}
		if v, ok := value.(string); ok {
			t.{{ upperFirst .Name }} = types.NewCode(v)
			return true
		}
{{- else if eq .Type "types.Text" }}
		if v, ok := value.(string); ok {
			t.{{ upperFirst .Name }} = types.NewText(v)
			return true
		}
{{- end }}
{{- end }}
{{- end }}
{{- end }}
	}
	return false
}

// TransferFields copies the fields of another record that have the same field number and
// type (TransferFields): FlowFields are not copied, and with initPrimaryKey false the
// primary key fields keep their values (default true: they are copied too). No trigger runs.
func (t *{{ .StructName }}) TransferFields(from tables.FieldValuer, initPrimaryKey ...bool) {
	copyKey := len(initPrimaryKey) == 0 || initPrimaryKey[0]
	for _, f := range t.GetFields() {
		if f.FlowField || f.PrimaryKey && !copyKey {
			continue
		}
		if v, ok := from.FieldValue(f.No); ok {
			t.SetFieldValue(f.No, v)
		}
	}
}

// CompanyName returns the company the record reads and writes (the session's company)
func (t *{{ .StructName }}) CompanyName() string {
	return t.company
}

// ChangeCompany makes the record read and write another company's table (filters are kept);
// it reports false when the company does not exist.
{{- if .Table.Global }} {{ .Table.Name }} is a global table, the same in every company.{{ end }}
func (t *{{ .StructName }}) ChangeCompany(company string) bool {
	if !tables.CompanyExists(t.db, t.dbType, company) {
		return false
	}
	t.company = company
	return true
}

// TestField returns an error unless the field has a value — or, with a value, that value
// (TestField): the user's messages "X is required" / "X must be …"
func (t *{{ .StructName }}) TestField(fieldName string, value ...any) error {
	no := t.FieldNo(fieldName)
	current, ok := t.FieldValue(no)
	if !ok {
		// a FlowField: its value as CalcFields left it (classic TESTFIELD after CALCFIELDS)
		current, ok = t.flowFieldValue(no)
	}
	if !ok {
		return fmt.Errorf("TestField: %s is not a field of {{ .Table.Name }}", fieldName)
	}
	field := t.FieldName(no)
	if len(value) == 0 {
		if tables.IsEmptyValue(current) {
			return apperrors.FieldRequired({{ .StructName }}TableName, field)
		}
		return nil
	}
	if !tables.SameValue(current, value[0]) {
		return apperrors.FieldMustBeValue({{ .StructName }}TableName, field, value[0])
	}
	return nil
}

// FieldError returns an error about a field (FieldError): "<caption> <text>"; pass a
// translated text
func (t *{{ .StructName }}) FieldError(fieldName string, text string) error {
	return apperrors.FieldErrorText({{ .StructName }}TableName, strings.ToLower(fieldName), text)
}

// CalcSums sets each of the decimal or integer fields to its total over the records matching
// the current filters (CalcSums)
func (t *{{ .StructName }}) CalcSums(fieldNames ...string) error {
{{- if not .HasSumFields }}
	if len(fieldNames) > 0 {
		return fmt.Errorf("CalcSums: %s is not a decimal or integer field of {{ .Table.Name }}", fieldNames[0])
	}
	return nil
}
{{- else }}
{{- if .Table.Global }}
	tableName := {{ .StructName }}TableName
{{- else }}
	tableName := fmt.Sprintf("%s$%s", t.company, {{ .StructName }}TableName)
{{- end }}
	where, args := t.buildWhereClause()
	for _, name := range fieldNames {
		var column string
		switch strings.ToLower(name) {
{{- range .Table.Fields }}
{{- if and (not .FlowField) (or (eq .Type "types.Decimal") (eq .Type "int")) }}
		case "{{ .DBName }}":
			column = "{{ .DBName }}"
{{- end }}
{{- end }}
		default:
			return fmt.Errorf("CalcSums: %s is not a decimal or integer field of {{ .Table.Name }}", name)
		}
		query := t.convertPlaceholders(fmt.Sprintf(` + "`SELECT CAST(COALESCE(SUM(%s), 0) AS TEXT) FROM \"%s\" WHERE %s`" + `, column, tableName, where), len(args))
		var total string
		if err := t.exec().QueryRow(query, args...).Scan(&total); err != nil {
			return fmt.Errorf("CalcSums {{ .Table.Name }}.%s: %w", column, err)
		}
		switch column {
{{- range .Table.Fields }}
{{- if and (not .FlowField) (eq .Type "types.Decimal") }}
		case "{{ .DBName }}":
			d, err := types.NewDecimalFromString(total)
			if err != nil {
				return fmt.Errorf("CalcSums {{ $.Table.Name }}.%s: %w", column, err)
			}
			t.{{ upperFirst .Name }} = d
{{- else if and (not .FlowField) (eq .Type "int") }}
		case "{{ .DBName }}":
			f, err := strconv.ParseFloat(total, 64)
			if err != nil {
				return fmt.Errorf("CalcSums {{ $.Table.Name }}.%s: %w", column, err)
			}
			t.{{ upperFirst .Name }} = int(f)
{{- end }}
{{- end }}
		}
	}
	return nil
}
{{- end }}

// LockTable locks the table until the transaction ends (classic LOCKTABLE): on Postgres a
// table lock (tables.LockTable) and the following reads (Get, FindFirst, FindLast, FindSet)
// lock the rows they read (SELECT … FOR UPDATE); SQLite locks the whole database on write
// anyway. Use it inside a transaction.
func (t *{{ .StructName }}) LockTable() {
	if !t.lockForUpdate {
		tables.LockTable(t.db, t.dbType, t.company, {{ .StructName }}TableName)
	}
	t.lockForUpdate = true
}

func (t *{{ .StructName }}) forUpdate() string {
	if t.lockForUpdate && t.dbType == database.DBTypePostgres {
		return " FOR UPDATE"
	}
	return ""
}

// MaxStrLen returns the maximum length of a Code or Text field (0 for other fields)
func (t *{{ .StructName }}) MaxStrLen(fieldName string) int {
	for _, f := range t.GetFields() {
		if strings.EqualFold(f.Name, fieldName) {
			return f.Length
		}
	}
	return 0
}

// GetFilter returns the filter on a field as text ("C1", "10..20", or the expression), ""
// without one
func (t *{{ .StructName }}) GetFilter(fieldName string) string {
	column, ok := t.columnName(fieldName)
	if !ok {
		return ""
	}
	filter, ok := t.filters[column]
	switch {
	case !ok || filter.invalidField:
		return ""
	case filter.isExpression:
		return filter.filterExpr
	case filter.minValue != nil && filter.maxValue != nil && fmt.Sprint(filter.minValue) == fmt.Sprint(filter.maxValue):
		return fmt.Sprint(filter.minValue)
	case filter.minValue != nil && filter.maxValue != nil:
		return fmt.Sprintf("%v..%v", filter.minValue, filter.maxValue)
	}
	return ""
}

// exec is the database of the record's own table: the temporary record's in-memory
// database, else the record's database
func (t *{{ .StructName }}) exec() database.Executor {
	if t.temp != nil {
		return t.temp.DB
	}
	return t.db
}

// InitTemporary makes the record temporary (Temporary): its table lives in a private
// in-memory database — Insert, Modify, Delete, Find, filters, keys and CalcSums work as usual
// and nothing reaches the real database. Triggers (Insert(true)) and FlowFields still use the
// real database (db) for other tables; a rename does not change other tables. The records
// are gone when the record is no longer used.
func (t *{{ .StructName }}) InitTemporary(db database.Executor, company string, dbType database.DBType) error {
	t.InitWithDBType(db, company, dbType)
	store, err := tables.NewTemporaryStore()
	if err != nil {
		return err
	}
	if err := t.CreateTableWithDBType(store.DB, company, database.DBTypeSQLite); err != nil {
		return err
	}
	t.temp = store
	return nil
}

// IsTemporary reports whether the record is temporary (InitTemporary)
func (t *{{ .StructName }}) IsTemporary() bool {
	return t.temp != nil
}

// Consistent marks the table consistent or inconsistent in the record's transaction
// (Consistent): tables.Commit refuses to commit while a table is inconsistent — e.g. G/L
// entries while the posting does not balance yet
func (t *{{ .StructName }}) Consistent(ok bool) {
	tables.SetConsistent(t.db, {{ .StructName }}TableName, ok)
}

// FilterExpressions returns the filters of all filter groups as field and expression (the
// filter syntax: "C1", "10..20", the SetFilter expression) — for a page opened from code
// (system.RunModal)
func (t *{{ .StructName }}) FilterExpressions() []tables.FieldFilter {
	var result []tables.FieldFilter
	for _, f := range t.allFilters() {
		expr := ""
		switch {
		case f.invalidField:
			continue
		case f.isExpression:
			expr = f.filterExpr
		case f.minValue != nil && f.maxValue != nil && fmt.Sprint(f.minValue) == fmt.Sprint(f.maxValue):
			expr = fmt.Sprint(f.minValue)
		case f.minValue != nil && f.maxValue != nil:
			expr = fmt.Sprintf("%v..%v", f.minValue, f.maxValue)
		default:
			continue
		}
		field := f.fieldName
{{- if .HasFilterableFlowField }}
		if f.flowField != "" {
			field = f.flowField
		}
{{- end }}
		result = append(result, tables.FieldFilter{Field: field, Expression: expr})
	}
	return result
}

// KeyText returns the primary key of the current record as text, the values joined by ","
// (the record id of the API and the pages)
func (t *{{ .StructName }}) KeyText() string {
	_, text := t.recordKey()
	return text
}

// CopyFilter copies the filter on one field to a field of another record, also of another
// table: cust.CopyFilter("no", &entry, "customer_no"). No filter on the field removes the
// filter on the other record's field.
func (t *{{ .StructName }}) CopyFilter(fieldName string, to tables.FilterTarget, toFieldName string) {
	column, ok := t.columnName(fieldName)
	if !ok {
		fmt.Printf("Error: CopyFilter from unknown field %q of {{ .Table.Name }}\n", fieldName)
		return
	}
	filter, ok := t.filters[column]
	switch {
	case !ok:
		to.SetRange(toFieldName)
	case filter.isExpression:
		to.SetFilter(toFieldName, filter.filterExpr)
	default:
		to.SetRange(toFieldName, filter.minValue, filter.maxValue)
	}
}

// GetRangeMin returns the lower end of the SetRange filter on a field (nil without one)
func (t *{{ .StructName }}) GetRangeMin(fieldName string) interface{} {
	if column, ok := t.columnName(fieldName); ok {
		if filter, ok := t.filters[column]; ok && !filter.isExpression {
			return filter.minValue
		}
	}
	return nil
}

// GetRangeMax returns the upper end of the SetRange filter on a field (nil without one)
func (t *{{ .StructName }}) GetRangeMax(fieldName string) interface{} {
	if column, ok := t.columnName(fieldName); ok {
		if filter, ok := t.filters[column]; ok && !filter.isExpression {
			return filter.maxValue
		}
	}
	return nil
}

// SetRecFilter filters on the primary key of the current record (SetRecFilter)
func (t *{{ .StructName }}) SetRecFilter() {
{{- range .Table.Fields }}
{{- if .PrimaryKey }}
	t.SetRange("{{ .DBName }}", t.{{ upperFirst .Name }})
{{- end }}
{{- end }}
}

// recordKey returns the primary key values of the current record and their text
func (t *{{ .StructName }}) recordKey() ([]interface{}, string) {
	values := []interface{}{ {{- range $i, $f := .Table.Fields }}{{ if $f.PrimaryKey }}{{ keyArg $f (printf "t.%s" (upperFirst $f.Name)) }}{{ if not (isLastPK $i $.Table.Fields) }}, {{ end }}{{ end }}{{ end -}} }
	parts := make([]string, len(values))
	for i, v := range values {
		parts[i] = fmt.Sprint(v)
	}
	return values, strings.Join(parts, ",")
}

// Mark marks (or with false unmarks) the current record and reports whether it is marked;
// without an argument it only reports it (Mark)
func (t *{{ .StructName }}) Mark(mark ...bool) bool {
	values, key := t.recordKey()
	if len(mark) > 0 {
		if mark[0] {
			if t.marks == nil {
				t.marks = map[string][]interface{}{}
			}
			t.marks[key] = values
		} else {
			delete(t.marks, key)
		}
	}
	_, marked := t.marks[key]
	return marked
}

// MarkedOnly limits the following reads to the marked records (MarkedOnly)
func (t *{{ .StructName }}) MarkedOnly(only bool) {
	t.markedOnly = only
}

// ClearMarks removes all marks (ClearMarks)
func (t *{{ .StructName }}) ClearMarks() {
	t.marks = nil
}

// RecordId returns the table and the primary key of the current record: "Customer: C1"
func (t *{{ .StructName }}) RecordId() string {
	_, key := t.recordKey()
	return {{ .StructName }}TableName + ": " + key
}

// CountApprox returns the number of records matching the filters (the same as Count)
func (t *{{ .StructName }}) CountApprox() int {
	return t.Count()
}

// Rename gives the current (read) record a new primary key — the values in key order — and
// modifies it with the OnRename trigger; fields of other tables that refer to it follow
// (Rename). It reports false when a value has the wrong type or the modify fails.
func (t *{{ .StructName }}) Rename(newKey ...interface{}) bool {
	numbers := []int{ {{- range $i, $f := .Table.Fields }}{{ if $f.PrimaryKey }}{{ $f.No }}{{ if not (isLastPK $i $.Table.Fields) }}, {{ end }}{{ end }}{{ end -}} }
	if len(newKey) != len(numbers) {
		return false
	}
	for i, no := range numbers {
		if !t.SetFieldValue(no, newKey[i]) {
			return false
		}
	}
	return t.Modify(true)
}

// Copy copies another record's values and filters (Copy)
func (t *{{ .StructName }}) Copy(from *{{ .StructName }}) {
	t.TransferFields(from)
	t.CopyFilters(from)
	// The copy is the record as read: its stored values (xRec, the key a Modify matches) are
	// the source's — a Modify after Copy changes the copied record, never the one read before
	if from.oldValues != nil {
		t.oldValues = make(map[string]interface{}, len(from.oldValues))
		for k, v := range from.oldValues {
			t.oldValues[k] = v
		}
	} else {
		t.oldValues = nil
	}
}

// Find finds a record (Find) within the filters, in the current key and direction
// (SetCurrentKey, SetAscending): "-" the first, "+" the last, "=" the record with the
// current primary key, ">" the next record after the current one, "<" the one before it.
// "=<>" combine, tried in that order: "=<>" the current record, else the one before it, else
// the next one.
func (t *{{ .StructName }}) Find(which ...string) bool {
	how := "="
	if len(which) > 0 && which[0] != "" {
		how = which[0]
	}
	switch how {
	case "-":
		return t.findInKey("", false)
	case "+":
		return t.findInKey("", true)
	}
	if strings.Contains(how, "=") {
		// The record's key in a filter group of its own: the filters still apply (also those
		// on key fields)
		group := t.FilterGroup()
		t.FilterGroup(tables.FindKeyFilterGroup)
		t.filters = nil
		t.SetRecFilter()
		found := t.FindFirst()
		t.filters = nil
		t.FilterGroup(group)
		if found {
			return true
		}
	}
	if strings.Contains(how, "<") && t.findInKey("<", true) {
		return true
	}
	if strings.Contains(how, ">") && t.findInKey(">", false) {
		return true
	}
	return false
}

// findInKey reads the first record in the current key and direction — reversed for the
// last — and with seek ">"/"<" only after/before the current record (a row-value
// comparison on the key and primary key columns). A key with a FlowField cannot seek.
func (t *{{ .StructName }}) findInKey(seek string, reverse bool) bool {
	if seek != "" {
		var columns, params []string
		var values []interface{}
		for _, column := range t.keyColumns() {
			v, ok := t.columnValue(column)
			if !ok {
				fmt.Printf("Error: Find(%q) on {{ .Table.Name }} needs a key of stored fields\n", seek)
				return false
			}
			if t.isKeyDateColumn(column) {
				v = types.KeyDate(v)
			}
			columns = append(columns, t.cmpColumn(column))
			params = append(params, t.cmpParam(column))
			values = append(values, v)
		}
		after := (seek == ">") != t.descending
		op := "<"
		if after {
			op = ">"
		}
		t.seek = "(" + strings.Join(columns, ", ") + ") " + op + " (" + strings.Join(params, ", ") + ")"
		t.seekArgs = values
	}
	savedLimit, savedOffset, savedDescending := t.limit, t.offset, t.descending
	t.limit, t.offset = 1, 0
	if reverse {
		t.descending = !t.descending
	}
	found := t.FindSet()
	// One record read: a later Next seeks from it in the key (inside a transaction FindSet
	// buffers its rows — a one-row buffer would end the Next at once)
	if t.currentRows != nil {
		t.currentRows.Close()
		t.currentRows = nil
	}
	t.bufferedRecords = nil
	t.limit, t.offset, t.descending = savedLimit, savedOffset, savedDescending
	t.seek, t.seekArgs = "", nil
	return found
}

// keyColumns returns the columns of the current key followed by the primary key's (the
// sort order of FindSet)
func (t *{{ .StructName }}) keyColumns() []string {
	columns := slices.Clone(t.orderByFields)
	for _, column := range []string{ {{- range $i, $f := .Table.Fields }}{{ if $f.PrimaryKey }}"{{ $f.DBName }}", {{ end }}{{ end -}} } {
		if !slices.Contains(columns, column) {
			columns = append(columns, column)
		}
	}
	return columns
}

// columnValue returns the value of a stored field by its column, as an SQL argument
func (t *{{ .StructName }}) columnValue(column string) (interface{}, bool) {
	switch column {
{{- range .Table.Fields }}
{{- if not .FlowField }}
	case "{{ .DBName }}":
{{- if eq .Type "Option" }}
		return int(t.{{ upperFirst .Name }}), true
{{- else }}
		return t.{{ upperFirst .Name }}, true
{{- end }}
{{- end }}
{{- end }}
	}
	return nil, false
}

// SetSelection keeps the records selected on the list a codeunit is run from (the API sets
// it from the list's selection; keys are field name → value); SetSelectionFilter applies it
func (t *{{ .StructName }}) SetSelection(keys []map[string]interface{}) {
	t.selection = nil
	for _, key := range keys {
		var r {{ .StructName }}
		r.FromMap(key)
		values, text := r.recordKey()
		if t.selection == nil {
			t.selection = map[string][]interface{}{}
		}
		t.selection[text] = values
	}
}

// SetSelectionFilter limits the record to the records selected on the list the codeunit was
// run from (SetSelectionFilter): several selected records are marked (MarkedOnly), else the
// filter is on the current record (SetRecFilter)
func (t *{{ .StructName }}) SetSelectionFilter() {
	if len(t.selection) <= 1 {
		for _, values := range t.selection {
			var r {{ .StructName }}
			r.setKeyValues(values)
			r.SetRecFilter()
			t.CopyFilters(&r)
			return
		}
		t.SetRecFilter()
		return
	}
	t.marks = make(map[string][]interface{}, len(t.selection))
	for key, values := range t.selection {
		t.marks[key] = values
	}
	t.markedOnly = true
}

// setKeyValues sets the primary key fields from their values (recordKey order)
func (t *{{ .StructName }}) setKeyValues(values []interface{}) {
	i := 0
{{- range .Table.Fields }}
{{- if .PrimaryKey }}
	if v, ok := values[i].({{ if eq .Type "Option" }}{{ $.StructName }}{{ upperFirst .Name }}{{ else }}{{ .Type }}{{ end }}); ok {
		t.{{ upperFirst .Name }} = v
	}
	i++
{{- end }}
{{- end }}
	_ = i
}

// Init clears the fields that are not part of the primary key and sets their initial values
// from the definition (Init); the primary key keeps its values
func (t *{{ .StructName }}) Init() {
{{- range .Table.Fields }}
{{- if and (not .PrimaryKey) (not .FlowField) }}
{{- if eq .Type "Option" }}
	t.{{ upperFirst .Name }} = 0
{{- else }}
	t.{{ upperFirst .Name }} = *new({{ .Type }})
{{- end }}
{{- end }}
{{- end }}
	t.applyDefaults()
}

// Validate sets a field and runs its OnValidate triggers (the same as ValidateField)
func (t *{{ .StructName }}) Validate(fieldName string, value interface{}) error {
	return t.ValidateField(fieldName, value)
}

// TableCaption returns the table's caption in a language (default en-US)
func (t *{{ .StructName }}) TableCaption(language ...string) string {
	return t.GetCaption(captionLanguage(language))
}

// FieldCaption returns a field's caption in a language (default en-US)
func (t *{{ .StructName }}) FieldCaption(fieldName string, language ...string) string {
	return t.GetFieldCaption(fieldName, captionLanguage(language))
}

// ========================================
// Interface Implementation (tables.Table)
// ========================================

// ClearFilters removes all filters (alias for Reset)
func (t *{{ .StructName }}) ClearFilters() {
	t.filters = nil
	t.otherFilters = nil
	t.filterGroup = 0
	t.orderByFields = nil
	// Note: Don't clear oldValues or iteration state here
}

// ToMap converts the current record to a map for JSON serialization
func (t *{{ .StructName }}) ToMap() map[string]interface{} {
	return map[string]interface{}{
{{- range .Table.Fields }}
{{- if not .FlowField }}
{{- if eq .Type "types.Code" }}
		"{{ .DBName }}": t.{{ upperFirst .Name }}.String(),
{{- else if eq .Type "types.Text" }}
		"{{ .DBName }}": t.{{ upperFirst .Name }}.String(),
{{- else if eq .Type "types.Decimal" }}
		"{{ .DBName }}": t.{{ upperFirst .Name }}.String(),
{{- else if eq .Type "types.Date" }}
		"{{ .DBName }}": t.{{ upperFirst .Name }}.String(),
{{- else if eq .Type "types.DateTime" }}
		"{{ .DBName }}": t.{{ upperFirst .Name }}.String(),
{{- else if eq .Type "Option" }}
		"{{ .DBName }}": int(t.{{ upperFirst .Name }}),
{{- else }}
		"{{ .DBName }}": t.{{ upperFirst .Name }},
{{- end }}
{{- else }}
		// FlowField: {{ .DBName }}
{{- if eq .Type "types.Decimal" }}
		"{{ .DBName }}": t.{{ upperFirst .Name }}.String(),
{{- else }}
		"{{ .DBName }}": t.{{ upperFirst .Name }},
{{- end }}
{{- end }}
{{- end }}
	}
}

// FromMap populates the record fields from a map (for API POST/PUT)
func (t *{{ .StructName }}) FromMap(data map[string]interface{}) {
{{- range .Table.Fields }}
{{- if not .FlowField }}
	if v, ok := data["{{ .DBName }}"]; ok && v != nil {
{{- if eq .Type "types.Code" }}
		if s, ok := v.(string); ok {
			t.{{ upperFirst .Name }} = types.NewCode(s)
		}
{{- else if eq .Type "types.Text" }}
		if s, ok := v.(string); ok {
			t.{{ upperFirst .Name }} = types.NewText(s)
		}
{{- else if eq .Type "types.Decimal" }}
		switch val := v.(type) {
		case string:
			t.{{ upperFirst .Name }}, _ = types.NewDecimalFromString(val)
		case float64:
			t.{{ upperFirst .Name }} = types.NewDecimal(val)
		case int:
			t.{{ upperFirst .Name }} = types.NewDecimalFromInt(int64(val))
		case int64:
			t.{{ upperFirst .Name }} = types.NewDecimalFromInt(val)
		}
{{- else if eq .Type "types.Date" }}
		if s, ok := v.(string); ok {
			t.{{ upperFirst .Name }}, _ = types.NewDateFromString(s)
		}
{{- else if eq .Type "types.DateTime" }}
		if s, ok := v.(string); ok {
			t.{{ upperFirst .Name }}, _ = types.NewDateTimeFromString(s)
		}
{{- else if eq .Type "Option" }}
		switch val := v.(type) {
		case float64:
			t.{{ upperFirst .Name }} = {{ $.StructName }}{{ upperFirst .Name }}(int(val))
		case int:
			t.{{ upperFirst .Name }} = {{ $.StructName }}{{ upperFirst .Name }}(val)
		case string:
			// Lookup string value in options
			options := []string{ {{- range $i, $opt := .Options }}{{- if $i }}, {{ end }}"{{ $opt }}"{{- end }} }
			for i, opt := range options {
				if opt == val {
					t.{{ upperFirst .Name }} = {{ $.StructName }}{{ upperFirst .Name }}(i)
					break
				}
			}
		}
{{- else if eq .Type "bool" }}
		if b, ok := v.(bool); ok {
			t.{{ upperFirst .Name }} = b
		}
{{- else if eq .Type "int" }}
		switch val := v.(type) {
		case float64:
			t.{{ upperFirst .Name }} = int(val)
		case int:
			t.{{ upperFirst .Name }} = val
		case string:
			// a record key from a URL or a list selection
			if n, err := strconv.Atoi(val); err == nil {
				t.{{ upperFirst .Name }} = n
			}
		}
{{- else if eq .Type "int64" }}
		switch val := v.(type) {
		case float64:
			t.{{ upperFirst .Name }} = int64(val)
		case int64:
			t.{{ upperFirst .Name }} = val
		case int:
			t.{{ upperFirst .Name }} = int64(val)
		}
{{- else if eq .Type "float64" }}
		if f, ok := v.(float64); ok {
			t.{{ upperFirst .Name }} = f
		}
{{- else if eq .Type "string" }}
		if s, ok := v.(string); ok {
			t.{{ upperFirst .Name }} = s
		}
{{- end }}
	}
{{- end }}
{{- end }}
}

// UpdateFromMap updates only the provided fields (for PATCH-style updates)
func (t *{{ .StructName }}) UpdateFromMap(data map[string]interface{}) {
	// Same as FromMap - only updates fields present in the map
	t.FromMap(data)
}

// GetPrimaryKeyField returns the name of the primary key field
func (t *{{ .StructName }}) GetPrimaryKeyField() string {
{{- if .FirstPrimaryKey }}
	return "{{ .FirstPrimaryKey.DBName }}"
{{- else }}
	return ""
{{- end }}
}

// GetPrimaryKeyValue returns the current primary key value as a string
func (t *{{ .StructName }}) GetPrimaryKeyValue() string {
{{- if .FirstPrimaryKey }}
{{- if eq .FirstPrimaryKey.Type "types.Code" }}
	return t.{{ upperFirst .FirstPrimaryKey.Name }}.String()
{{- else if eq .FirstPrimaryKey.Type "types.Text" }}
	return t.{{ upperFirst .FirstPrimaryKey.Name }}.String()
{{- else if eq .FirstPrimaryKey.Type "int" }}
	return fmt.Sprintf("%d", t.{{ upperFirst .FirstPrimaryKey.Name }})
{{- else if eq .FirstPrimaryKey.Type "int64" }}
	return fmt.Sprintf("%d", t.{{ upperFirst .FirstPrimaryKey.Name }})
{{- else if eq .FirstPrimaryKey.Type "string" }}
	return t.{{ upperFirst .FirstPrimaryKey.Name }}
{{- else }}
	return fmt.Sprintf("%v", t.{{ upperFirst .FirstPrimaryKey.Name }})
{{- end }}
{{- else }}
	return ""
{{- end }}
}

// GetFields returns metadata about all fields
func (t *{{ .StructName }}) GetFields() []tables.FieldInfo {
	return []tables.FieldInfo{
{{- range .Table.Fields }}
		{
			No:         {{ .No }},
			Name:       "{{ .DBName }}",
{{- if eq .Type "types.Code" }}
			Type:       tables.FieldTypeCode,
{{- else if eq .Type "types.Text" }}
			Type:       tables.FieldTypeText,
{{- else if eq .Type "int" }}
			Type:       tables.FieldTypeInteger,
{{- else if eq .Type "int64" }}
			Type:       tables.FieldTypeInteger,
{{- else if eq .Type "types.Decimal" }}
			Type:       tables.FieldTypeDecimal,
{{- else if eq .Type "float64" }}
			Type:       tables.FieldTypeDecimal,
{{- else if eq .Type "bool" }}
			Type:       tables.FieldTypeBoolean,
{{- else if eq .Type "types.Date" }}
			Type:       tables.FieldTypeDate,
{{- else if eq .Type "types.DateTime" }}
			Type:       tables.FieldTypeDateTime,
{{- else if eq .Type "Option" }}
			Type:       tables.FieldTypeOption,
{{- else if or (eq .Type "[]byte") (eq .Type "BLOB") }}
			Type:       tables.FieldTypeBlob,
{{- else }}
			Type:       tables.FieldTypeText,
{{- end }}
			Length:     {{ .Length }},
			Required:   {{ .Required }},
			Editable:   {{ not .PrimaryKey }},
			PrimaryKey: {{ .PrimaryKey }},
			FlowField:  {{ .FlowField }},
{{- if readOnly . }}
			ReadOnly:   true,
{{- end }}
{{- if .DecimalPlaces }}
			DecimalPlaces: "{{ .DecimalPlaces }}",
{{- end }}
{{- if .Sensitive }}
			Sensitive:  true,
{{- end }}
{{- if .Masked }}
			Masked:     true,
{{- end }}
		},
{{- end }}
	}
}

// SetFlowFilter sets a FlowFilter field's filter expression (SETFILTER on a
// FlowFilter field); FlowFields applying it use it from then on. "" clears it.
func (t *{{ .StructName }}) SetFlowFilter(field, expr string) error {
	switch field {
{{- range .FlowFilterFields }}
	case "{{ .DBName }}":
		if err := flowfilter.Validate(flowfilter.{{ flowFilterKind . }}, expr); err != nil {
			return err
		}
		t.{{ upperFirst .Name }} = expr
		return nil
{{- end }}
	}
	return fmt.Errorf("%q is not a FlowFilter field of {{ .Table.Name }}", field)
}

// GetFlowFilterFields returns the FlowFilter fields (name and flowfilter kind)
func (t *{{ .StructName }}) GetFlowFilterFields() []tables.FlowFilterFieldInfo {
	return []tables.FlowFilterFieldInfo{
{{- range .FlowFilterFields }}
		{Name: "{{ .DBName }}", Kind: string(flowfilter.{{ flowFilterKind . }})},
{{- end }}
	}
}

// GetFlowFields returns names of FlowFields that need CalcFields
func (t *{{ .StructName }}) GetFlowFields() []string {
	return []string{
{{- range .Table.Fields }}
{{- if .FlowField }}
		"{{ .DBName }}",
{{- end }}
{{- end }}
	}
}

// GetOptionFields returns Option field names mapped to their option values
func (t *{{ .StructName }}) GetOptionFields() map[string][]string {
	return map[string][]string{
{{- range .Table.Fields }}
{{- if eq .Type "Option" }}
		"{{ .DBName }}": { {{- range $i, $opt := .Options }}{{ if $i }}, {{ end }}"{{ $opt }}"{{- end }} },
{{- end }}
{{- end }}
	}
}

// GetTableRelationFields returns fields that have table relations (foreign keys)
func (t *{{ .StructName }}) GetTableRelationFields() map[string]tables.TableRelationInfo {
	return map[string]tables.TableRelationInfo{
{{- range .Table.Fields }}
{{- if .TableRelation }}
{{- if .TableRelation.Conditions }}
		"{{ .DBName }}": {
			Conditions: []tables.ConditionalRelationInfo{
{{- range .TableRelation.Conditions }}
				{Field: "{{ dbFieldName $.Table.Fields .When.Field }}", Value: {{ .When.Equal }}, Relation: tables.TableRelationInfo{
					Table: "{{ .Table }}", Field: "{{ .Field }}", DisplayField: "{{ .DisplayField }}",
{{- if .LookupColumns }}
					LookupColumns: []tables.LookupColumnInfo{ {{- range .LookupColumns }}{Source: "{{ .Source }}", Width: {{ .Width }}}, {{ end -}} },
{{- end }}
					SearchTimeout: {{ .SearchTimeout }},
{{- if not .ShouldValidate }}
					NoValidate: true,
{{- end }}
{{- if .Filters }}
					Filters: []tables.RelationFilterInfo{
{{- range .SortedFilters }}
						{Column: "{{ .Column }}", Field: "{{ .Field }}", Const: "{{ if ne (printf "%v" .Const) "<nil>" }}{{ .Const }}{{ end }}"},
{{- end }}
					},
{{- end }}
				}},
{{- end }}
			},
		},
{{- else }}
		"{{ .DBName }}": {
			Table:        "{{ .TableRelation.Table }}",
			Field:        "{{ .TableRelation.Field }}",
			DisplayField: "{{ .TableRelation.DisplayField }}",
{{- if .TableRelation.LookupColumns }}
			LookupColumns: []tables.LookupColumnInfo{
{{- range .TableRelation.LookupColumns }}
				{Source: "{{ .Source }}", Width: {{ .Width }}},
{{- end }}
			},
{{- end }}
			SearchTimeout: {{ .TableRelation.SearchTimeout }},
{{- if not .TableRelation.ShouldValidate }}
			NoValidate:    true,
{{- end }}
{{- if .TableRelation.Filters }}
			Filters: []tables.RelationFilterInfo{
{{- range .TableRelation.SortedFilters }}
				{Column: "{{ .Column }}", Field: "{{ .Field }}", Const: "{{ if ne (printf "%v" .Const) "<nil>" }}{{ .Const }}{{ end }}"},
{{- end }}
			},
{{- end }}
		},
{{- end }}
{{- end }}
{{- end }}
	}
}
`

const businessTemplate = `package tables

import (
	gtables "github.com/hansjlachmann/openerp/backend/generated/tables"
)

// {{ lowerFirst .StructName }}Logic is the business logic of table {{ .Table.ID }} {{ .Table.Name }}.
// Add a trigger when it is needed: the available triggers are listed at the
// top of generated/tables/{{ lower .StructName }}_gen.go. The field rules of the definition
// (required, length, range) are checked after OnInsert and OnModify.
type {{ lowerFirst .StructName }}Logic struct{}

func init() { gtables.Register{{ .StructName }}Logic({{ lowerFirst .StructName }}Logic{}) }

// OnInsert runs before a new record is inserted
func ({{ lowerFirst .StructName }}Logic) OnInsert(rec *gtables.{{ .StructName }}) error {
	return nil
}

// OnModify runs before a record is modified
func ({{ lowerFirst .StructName }}Logic) OnModify(rec *gtables.{{ .StructName }}) error {
	return nil
}

// OnDelete runs before a record is deleted
func ({{ lowerFirst .StructName }}Logic) OnDelete(rec *gtables.{{ .StructName }}) error {
	return nil
}

// OnRename runs before a primary key field is changed (rec.OldValue has the old value)
func ({{ lowerFirst .StructName }}Logic) OnRename(rec *gtables.{{ .StructName }}) error {
	return nil
}
`

// tablesTemplate is tables_gen.go: what concerns all tables — the table registry (the
// API creates records by table name) and the check of registered business logic.
const tablesTemplate = `// Code generated by tablegen. DO NOT EDIT.

package tables

import (
	"fmt"
	"reflect"
	"sort"
	"strings"

	"github.com/hansjlachmann/openerp/backend/foundation/database"
	"github.com/hansjlachmann/openerp/backend/foundation/tables"
)

// tableRegistry holds the factory of every table by name
var tableRegistry = map[string]tables.TableFactory{
{{- range . }}
	"{{ or .Table.APIName .Table.Name }}": func() tables.Table { return &{{ .StructName }}{} },
{{- end }}
}

// NewRecords returns a new record of every table by table ID (the object registry)
func NewRecords() map[int]tables.Table {
	return map[int]tables.Table{
{{- range . }}
		{{ .StructName }}TableID: &{{ .StructName }}{},
{{- end }}
	}
}

// RegisterTableFactory registers a table factory by name (tests)
func RegisterTableFactory(name string, id int, factory tables.TableFactory) {
	tableRegistry[name] = factory
}

// The relation check of Validate finds related tables in this registry
func init() { tables.RegisterRelatedTables(GetTableFactory) }

// GetTableFactory returns a table factory by name
func GetTableFactory(name string) (tables.TableFactory, bool) {
	factory, ok := tableRegistry[name]
	return factory, ok
}

// ListTableNames returns all registered table names
func ListTableNames() []string {
	names := make([]string, 0, len(tableRegistry))
	for name := range tableRegistry {
		names = append(names, name)
	}
	return names
}

// TableExists checks if a table name is registered
func TableExists(name string) bool {
	_, ok := tableRegistry[name]
	return ok
}

// EncryptStoredSecrets encrypts values of encrypted fields (encrypted: true in the table
// YAML) that are stored as typed — saved before the field was encrypted or while no key
// was set — in every table that has such fields: global tables once, company tables in
// each company. Run at startup; does nothing without an encryption key. Returns how many
// values it encrypted.
func EncryptStoredSecrets(db database.Executor, dbType database.DBType, companies []string) (int, error) {
	type secretTable interface {
		tables.Table
		IsGlobal() bool
		EncryptStoredSecrets() (int, error)
	}
	total := 0
	names := ListTableNames()
	sort.Strings(names)
	for _, name := range names {
		probe, ok := tableRegistry[name]().(secretTable)
		if !ok {
			continue // no encrypted fields
		}
		scopes := companies
		if probe.IsGlobal() {
			scopes = []string{""}
		}
		for _, company := range scopes {
			t := tableRegistry[name]().(secretTable)
			t.InitWithDBType(db, company, dbType)
			n, err := t.EncryptStoredSecrets()
			total += n
			if err != nil {
				return total, fmt.Errorf("encrypt stored secrets of %s (%s): %w", name, company, err)
			}
		}
	}
	return total, nil
}

// captionLanguage is the language of TableCaption/FieldCaption: the one given, else en-US
func captionLanguage(language []string) string {
	if len(language) > 0 && language[0] != "" {
		return language[0]
	}
	return "en-US"
}

// checkLogic panics when business logic registered for a table has a method named like a
// trigger ("On…") that the table does not have, or with the wrong signature: such a
// trigger would never run. The message names the method and the table.
func checkLogic(table, record string, logic any, trigger func(name string, logic any) (known, ok bool)) {
	if logic == nil {
		panic(fmt.Sprintf("business logic of table %s is nil", table))
	}
	typ := reflect.TypeOf(logic)
	for i := 0; i < typ.NumMethod(); i++ {
		name := typ.Method(i).Name
		if !strings.HasPrefix(name, "On") {
			continue
		}
		known, ok := trigger(name, logic)
		switch {
		case !known:
			panic(fmt.Sprintf("business logic %v: %s is not a trigger of table %s (misspelled, or the field was renamed or removed in the table definition)", typ, name, table))
		case !ok:
			panic(fmt.Sprintf("business logic %v: trigger %s of table %s must be func(rec *%s) error", typ, name, table, record))
		}
	}
}
`
