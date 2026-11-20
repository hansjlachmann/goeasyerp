package tables

import (
	"fmt"
	"slices"
	"strings"

	"github.com/hansjlachmann/goeasyerp/backend/foundation/flowfilter"
	"github.com/hansjlachmann/goeasyerp/backend/foundation/types"
)

// The filters, search, sort and marks of a generated record, written once for all tables
// (see fields.go): the record keeps a Query and its methods SetRange, SetFilter,
// FilterGroup, SetCurrentKey … call it with the table's QueryTable.

// QueryTable is what a Query needs to know of the record's table
type QueryTable struct {
	Fields *TableFields
	// NumericDecimals: decimals are stored as text (SQLite, temporary tables), so filters,
	// keys and the order compare them as numbers — as text 100 lies between 0 and 16.2
	NumericDecimals bool
	// FlowFieldKind / FlowFieldExpr: the Sum/Count FlowFields lists filter and sort on (the
	// generated FlowFieldFilterKind and flowFieldExpr); nil when the table has none
	FlowFieldKind func(field string) (flowfilter.Kind, bool)
	FlowFieldExpr func(field string) (string, []interface{})
}

// FilterCondition is a filter on a field
type FilterCondition struct {
	fieldName    string
	minValue     interface{}
	maxValue     interface{}
	filterExpr   string // a SetFilter expression
	isExpression bool   // filterExpr instead of minValue/maxValue
	invalidField bool   // a filter on an unknown field: matches no rows (fail closed)
	blankDate    bool   // a date range from the blank date: blank dates (NULL) match too
	flowField    string // a filter on a FlowField (filterExpr applied to its subquery)
}

// Query is a record's filters (per filter group), free-text search, current key and
// direction, marks and Find seek condition
type Query struct {
	// the current filter group's filters, and the other groups'; all groups apply together
	filters      map[string]*FilterCondition
	filterGroup  int
	otherFilters map[int]map[string]*FilterCondition

	// Seek / SeekArgs: Find(">") / Find("<") — the condition for the record after/before
	// the current one
	Seek     string
	SeekArgs []interface{}

	// marks (Mark): primary key values by key text; MarkedOnly limits reads to them
	marks      map[string][]interface{}
	markedOnly bool

	// OrderBy: the current key's columns ("flowfield:<name>" for a FlowField);
	// Descending: its direction (SetAscending)
	OrderBy    []string
	Descending bool

	// free-text search (SetSearch): rows where any of the columns contains the text
	searchColumns []string
	searchText    string
}

// SetRange sets a range filter on a field: one value an exact match, two a range, none
// removes the filter on the field
func (q *Query) SetRange(qt QueryTable, fieldName string, values ...interface{}) {
	if q.filters == nil {
		q.filters = make(map[string]*FilterCondition)
	}

	var minValue, maxValue interface{}

	switch len(values) {
	case 0:
		if column, ok := qt.Fields.ColumnName(fieldName); ok {
			delete(q.filters, column)
		}
		delete(q.filters, fieldName)
		return
	case 1:
		minValue = values[0]
		maxValue = values[0]
	case 2:
		minValue = values[0]
		maxValue = values[1]
	default:
		fmt.Printf("Error: SetRange requires 1 or 2 values, got %d\n", len(values))
		return
	}

	column, ok := qt.Fields.ColumnName(fieldName)
	if !ok {
		// Unknown field: fail closed (no rows) rather than drop the filter or put
		// the name into SQL
		fmt.Printf("Error: SetRange on unknown field %q of %s\n", fieldName, qt.Fields.Table)
		q.filters[fieldName] = &FilterCondition{invalidField: true}
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

	q.filters[column] = &FilterCondition{
		fieldName: column,
		minValue:  minValue,
		maxValue:  maxValue,
		blankDate: blankDate,
	}
}

// SetFilter sets a filter expression on a field ("100..200|500", "<>0", "A*", two single
// quotes for the blank value); on a Sum/Count FlowField it applies to the FlowField's value
func (q *Query) SetFilter(qt QueryTable, fieldName, filterExpr string) {
	if q.filters == nil {
		q.filters = make(map[string]*FilterCondition)
	}
	if qt.FlowFieldKind != nil {
		if _, ok := qt.FlowFieldKind(fieldName); ok {
			// A FlowField is not a column: the filter applies to its subquery (flowFieldExpr)
			name := strings.ToLower(fieldName)
			q.filters["flowfield:"+name] = &FilterCondition{
				flowField:    name,
				filterExpr:   filterExpr,
				isExpression: true,
			}
			return
		}
	}
	column, ok := qt.Fields.ColumnName(fieldName)
	if !ok {
		// Unknown field: fail closed (no rows) rather than drop the filter or put
		// the name into SQL
		fmt.Printf("Error: SetFilter on unknown field %q of %s\n", fieldName, qt.Fields.Table)
		q.filters[fieldName] = &FilterCondition{invalidField: true}
		return
	}
	q.filters[column] = &FilterCondition{
		fieldName:    column,
		filterExpr:   filterExpr,
		isExpression: true,
	}
}

// SetCurrentKey sets the sort order; unknown fields are ignored (the primary key order is
// used if none remain), a Sum/Count FlowField sorts on its calculated value
func (q *Query) SetCurrentKey(qt QueryTable, fields ...string) {
	q.OrderBy = nil
	for _, field := range fields {
		if qt.FlowFieldKind != nil {
			if _, ok := qt.FlowFieldKind(field); ok {
				q.OrderBy = append(q.OrderBy, "flowfield:"+strings.ToLower(field))
				continue
			}
		}
		column, ok := qt.Fields.ColumnName(field)
		if !ok {
			fmt.Printf("Error: SetCurrentKey on unknown field %q of %s\n", field, qt.Fields.Table)
			continue
		}
		q.OrderBy = append(q.OrderBy, column)
	}
}

// SetSearch limits the records to those where any of the given fields contains text,
// case-insensitive; unknown fields are ignored, an empty text clears the search
func (q *Query) SetSearch(qt QueryTable, fields []string, text string) {
	q.searchColumns = nil
	q.searchText = strings.ToLower(strings.TrimSpace(text))
	if q.searchText == "" {
		return
	}
	for _, field := range fields {
		if column, ok := qt.Fields.ColumnName(field); ok {
			q.searchColumns = append(q.searchColumns, column)
		}
	}
}

// Reset clears the filters of all groups (back to group 0), the seek, key, direction and search
func (q *Query) Reset() {
	q.filters = nil
	q.otherFilters = nil
	q.filterGroup = 0
	q.Seek, q.SeekArgs = "", nil
	q.OrderBy = nil
	q.Descending = false
	q.searchColumns = nil
	q.searchText = ""
}

// ClearFilters clears the filters of all groups (back to group 0) and the key
func (q *Query) ClearFilters() {
	q.filters = nil
	q.otherFilters = nil
	q.filterGroup = 0
	q.OrderBy = nil
}

// ClearGroupFilters clears the current filter group's filters
func (q *Query) ClearGroupFilters() {
	q.filters = nil
}

// Where builds the WHERE clause of the filters, search, marks and seek, with its arguments
func (q *Query) Where(qt QueryTable) (string, []interface{}) {
	filters := q.allFilters()
	if len(filters) == 0 && q.searchText == "" && !q.markedOnly && q.Seek == "" {
		return "1=1", nil
	}

	var conditions []string
	var args []interface{}

	if q.searchText != "" {
		if len(q.searchColumns) == 0 {
			conditions = append(conditions, "1=0") // searching, but in no valid column
		} else {
			// LIKE wildcards in the text are matched literally
			pattern := "%" + strings.NewReplacer("\\", "\\\\", "%", "\\%", "_", "\\_").Replace(q.searchText) + "%"
			var ors []string
			for _, column := range q.searchColumns {
				ors = append(ors, "LOWER(CAST("+column+" AS TEXT)) LIKE ? ESCAPE '\\'")
				args = append(args, pattern)
			}
			conditions = append(conditions, "("+strings.Join(ors, " OR ")+")")
		}
	}

	for _, filter := range filters {
		if filter.invalidField {
			conditions = append(conditions, "1=0")
		} else if filter.flowField != "" {
			// filter syntax on the FlowField's value; an invalid expression matches no rows
			// (the API validates it first and reports it)
			if qt.FlowFieldKind == nil || qt.FlowFieldExpr == nil {
				conditions = append(conditions, "1=0")
				continue
			}
			kind, _ := qt.FlowFieldKind(filter.flowField)
			expr, exprArgs := qt.FlowFieldExpr(filter.flowField)
			clause, clauseArgs, err := flowfilter.ClauseFor(expr, exprArgs, kind, filter.filterExpr)
			if err != nil {
				conditions = append(conditions, "1=0")
			} else if clause != "" {
				conditions = append(conditions, clause)
				args = append(args, clauseArgs...)
			}
		} else if filter.isExpression {
			clause, exprArgs := q.filterClause(qt, filter.fieldName, filter.filterExpr)
			conditions = append(conditions, clause)
			args = append(args, exprArgs...)
		} else {
			// a range filter
			if filter.blankDate && filter.maxValue != nil {
				conditions = append(conditions, fmt.Sprintf("(%s <= ? OR %s IS NULL)", filter.fieldName, filter.fieldName))
				args = append(args, filter.maxValue)
			} else if filter.blankDate {
				conditions = append(conditions, fmt.Sprintf("(%s IS NULL OR %s = '%s')", filter.fieldName, filter.fieldName, types.BlankKeyDate))
			} else if filter.minValue != nil && filter.maxValue != nil {
				conditions = append(conditions, fmt.Sprintf("%s BETWEEN %s AND %s", qt.CmpColumn(filter.fieldName), qt.CmpParam(filter.fieldName), qt.CmpParam(filter.fieldName)))
				args = append(args, filter.minValue, filter.maxValue)
			} else if filter.minValue != nil {
				conditions = append(conditions, fmt.Sprintf("%s >= %s", qt.CmpColumn(filter.fieldName), qt.CmpParam(filter.fieldName)))
				args = append(args, filter.minValue)
			} else if filter.maxValue != nil {
				conditions = append(conditions, fmt.Sprintf("%s <= %s", qt.CmpColumn(filter.fieldName), qt.CmpParam(filter.fieldName)))
				args = append(args, filter.maxValue)
			}
		}
	}

	// MarkedOnly: only the marked records
	if q.markedOnly {
		if len(q.marks) == 0 {
			conditions = append(conditions, "1=0")
		} else {
			var keyParts []string
			for _, column := range qt.Fields.PrimaryKey() {
				keyParts = append(keyParts, column+" = ?")
			}
			match := "(" + strings.Join(keyParts, " AND ") + ")"
			var keys []string
			for _, key := range q.marks {
				keys = append(keys, match)
				args = append(args, key...)
			}
			conditions = append(conditions, "("+strings.Join(keys, " OR ")+")")
		}
	}

	// Find(">") / Find("<"): after/before the current record in the current key
	if q.Seek != "" {
		conditions = append(conditions, q.Seek)
		args = append(args, q.SeekArgs...)
	}

	where := strings.Join(conditions, " AND ")
	if where == "" {
		where = "1=1"
	}

	return where, args
}

// allFilters returns the filters of all filter groups, the current group's first
func (q *Query) allFilters() []*FilterCondition {
	filters := make([]*FilterCondition, 0, len(q.filters))
	for _, f := range q.filters {
		filters = append(filters, f)
	}
	groups := make([]int, 0, len(q.otherFilters))
	for g := range q.otherFilters {
		groups = append(groups, g)
	}
	slices.Sort(groups)
	for _, g := range groups {
		for _, f := range q.otherFilters[g] {
			filters = append(filters, f)
		}
	}
	return filters
}

// FilterGroup switches to filter group group and returns the current one; without an
// argument it only returns it. Each group has its own filters — SetRange, SetFilter,
// GetFilter and CopyFilter work on the current group — and the filters of all groups apply
// together.
func (q *Query) FilterGroup(group ...int) int {
	if len(group) == 0 || group[0] == q.filterGroup {
		return q.filterGroup
	}
	if q.otherFilters == nil {
		q.otherFilters = map[int]map[string]*FilterCondition{}
	}
	if len(q.filters) > 0 {
		q.otherFilters[q.filterGroup] = q.filters
	} else {
		delete(q.otherFilters, q.filterGroup)
	}
	q.filterGroup = group[0]
	q.filters = q.otherFilters[q.filterGroup]
	delete(q.otherFilters, q.filterGroup)
	return q.filterGroup
}

// PrimaryKey returns the primary key's columns in key order (shared: do not change it)
func (tf *TableFields) PrimaryKey() []string {
	tf.index()
	return tf.primaryKey
}

// storedKind returns the kind of a stored field by its column (0 for none)
func (tf *TableFields) storedKind(column string) Kind {
	i := tf.storedIndex(column)
	if i < 0 {
		return 0
	}
	return tf.Fields[i].Kind
}

// IsKeyDateColumn: a Date column of the primary key (a blank date is types.BlankKeyDate)
func (tf *TableFields) IsKeyDateColumn(column string) bool {
	i := tf.storedIndex(column)
	return i >= 0 && tf.Fields[i].Info.PrimaryKey && tf.Fields[i].Kind == KindDate
}

// numericDecimal: a decimal column compared as a number (decimals stored as text)
func (qt QueryTable) numericDecimal(column string) bool {
	return qt.NumericDecimals && qt.Fields.storedKind(column) == KindDecimal
}

// CmpColumn is a column as filters, keys and the order compare it (a decimal stored as text
// as a number)
func (qt QueryTable) CmpColumn(column string) string {
	if qt.numericDecimal(column) {
		return "CAST(" + column + " AS REAL)"
	}
	return column
}

// CmpParam is the placeholder of a value compared with a column (a decimal stored as text as
// a number)
func (qt QueryTable) CmpParam(column string) string {
	if qt.numericDecimal(column) {
		return "CAST(? AS REAL)"
	}
	return "?"
}

// filterClause turns a filter expression on a column into SQL: "100..200" (range), "..200",
// "100..", "100|200" (or), two single quotes (blank), "<3000", "<>0", "A*" (wildcard), else
// equal
func (q *Query) filterClause(qt QueryTable, fieldName, expr string) (string, []interface{}) {
	var conditions []string
	var args []interface{}
	// Blank dates are stored as NULL and come before every date
	isDate := qt.Fields.storedKind(fieldName) == KindDate
	orBlankDate := func(condition string) string {
		if isDate {
			return fmt.Sprintf("(%s OR %s IS NULL)", condition, fieldName)
		}
		return condition
	}

	// a decimal on SQLite compares as a number
	col, param := qt.CmpColumn(fieldName), qt.CmpParam(fieldName)

	for _, part := range strings.Split(expr, "|") {
		part = strings.TrimSpace(part)

		// a range (..); an open end ("..x", "x..") has one bound
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
		} else if op := FilterComparison(part); op != "" {
			// a comparison: <=, >=, <, > (a classic filter such as <3000)
			if op == "<" || op == "<=" {
				conditions = append(conditions, orBlankDate(fmt.Sprintf("%s %s %s", col, op, param)))
			} else {
				conditions = append(conditions, fmt.Sprintf("%s %s %s", col, op, param))
			}
			args = append(args, strings.TrimSpace(strings.TrimPrefix(part, op)))
		} else if strings.Contains(part, "*") {
			// a wildcard: * is %
			conditions = append(conditions, fmt.Sprintf("%s LIKE ?", fieldName))
			args = append(args, strings.ReplaceAll(part, "*", "%"))
		} else if strings.HasPrefix(part, "<>") {
			// not equal
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
			// equal (a boolean column takes Yes/No, true/false, 1/0)
			conditions = append(conditions, fmt.Sprintf("%s = %s", col, param))
			if qt.Fields.storedKind(fieldName) == KindBool {
				args = append(args, FilterBool(part))
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

	return "(" + strings.Join(conditions, " OR ") + ")", args
}

// OrderBy builds the ORDER BY clause of the current key and direction, with the arguments of
// its placeholders (a FlowField key sorts on its subquery, which may have some; they follow
// the WHERE arguments). The primary key columns always follow the current key, so the order is
// unique and paging never repeats or skips rows when the key has duplicate values.
func (q *Query) OrderByClause(qt QueryTable) (string, []interface{}) {
	var columns []string
	var args []interface{}
	for _, column := range q.OrderBy {
		if name, ok := strings.CutPrefix(column, "flowfield:"); ok && qt.FlowFieldExpr != nil {
			expr, exprArgs := qt.FlowFieldExpr(name)
			columns = append(columns, expr)
			args = append(args, exprArgs...)
			continue
		}
		columns = append(columns, qt.CmpColumn(column))
	}
	for _, column := range qt.Fields.PrimaryKey() {
		if !slices.Contains(q.OrderBy, column) {
			columns = append(columns, qt.CmpColumn(column))
		}
	}
	if q.Descending {
		for i := range columns {
			columns[i] += " DESC"
		}
	}
	return strings.Join(columns, ", "), args
}

// KeyColumns returns the columns of the current key followed by the primary key's (the sort
// order of FindSet)
func (q *Query) KeyColumns(qt QueryTable) []string {
	columns := slices.Clone(q.OrderBy)
	for _, column := range qt.Fields.PrimaryKey() {
		if !slices.Contains(columns, column) {
			columns = append(columns, column)
		}
	}
	return columns
}

// CopyFilters copies another record's filters (every group) and current key
func (q *Query) CopyFilters(from *Query) {
	q.filters = copyFilterMap(from.filters)
	q.filterGroup = from.filterGroup
	q.otherFilters = nil
	for g, filters := range from.otherFilters {
		if q.otherFilters == nil {
			q.otherFilters = map[int]map[string]*FilterCondition{}
		}
		q.otherFilters[g] = copyFilterMap(filters)
	}
	if len(from.OrderBy) > 0 {
		q.OrderBy = make([]string, len(from.OrderBy))
		copy(q.OrderBy, from.OrderBy)
	} else {
		q.OrderBy = nil
	}
}

// copyFilterMap returns a deep copy of a filter group's filters (nil for none)
func copyFilterMap(from map[string]*FilterCondition) map[string]*FilterCondition {
	if from == nil {
		return nil
	}
	filters := make(map[string]*FilterCondition, len(from))
	for key, filter := range from {
		c := *filter
		filters[key] = &c
	}
	return filters
}

// GetFilters returns the current group's filters as text (debugging and logging)
func (q *Query) GetFilters() string {
	if len(q.filters) == 0 {
		return ""
	}
	var parts []string
	for _, filter := range q.filters {
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

// GetFilter returns the current group's filter on a field in the filter syntax ("" for none)
func (q *Query) GetFilter(qt QueryTable, fieldName string) string {
	column, ok := qt.Fields.ColumnName(fieldName)
	if !ok {
		return ""
	}
	filter, ok := q.filters[column]
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

// FilterExpressions returns the filters of all groups as field and expression
func (q *Query) FilterExpressions() []FieldFilter {
	var result []FieldFilter
	for _, f := range q.allFilters() {
		filter, ok := f.expression()
		if ok {
			result = append(result, filter)
		}
	}
	return result
}

// FilterExpressionsByGroup returns the filters of each filter group (sorted by field)
func (q *Query) FilterExpressionsByGroup() map[int][]FieldFilter {
	result := map[int][]FieldFilter{}
	add := func(group int, filters map[string]*FilterCondition) {
		for _, f := range filters {
			filter, ok := f.expression()
			if ok {
				result[group] = append(result[group], filter)
			}
		}
		slices.SortFunc(result[group], func(a, b FieldFilter) int { return strings.Compare(a.Field, b.Field) })
	}
	add(q.filterGroup, q.filters)
	for group, filters := range q.otherFilters {
		if group != q.filterGroup {
			add(group, filters)
		}
	}
	return result
}

// expression is a filter as field and expression; false for one that has none
func (f *FilterCondition) expression() (FieldFilter, bool) {
	expr := ""
	switch {
	case f.invalidField:
		return FieldFilter{}, false
	case f.isExpression:
		expr = f.filterExpr
	case f.minValue != nil && f.maxValue != nil && fmt.Sprint(f.minValue) == fmt.Sprint(f.maxValue):
		expr = fmt.Sprint(f.minValue)
	case f.minValue != nil && f.maxValue != nil:
		expr = fmt.Sprintf("%v..%v", f.minValue, f.maxValue)
	default:
		return FieldFilter{}, false
	}
	field := f.fieldName
	if f.flowField != "" {
		field = f.flowField
	}
	return FieldFilter{Field: field, Expression: expr}, true
}

// CopyFilter copies the current group's filter on a field to a field of another record
func (q *Query) CopyFilter(qt QueryTable, fieldName string, to FilterTarget, toFieldName string) {
	column, ok := qt.Fields.ColumnName(fieldName)
	if !ok {
		fmt.Printf("Error: CopyFilter from unknown field %q of %s\n", fieldName, qt.Fields.Table)
		return
	}
	filter, ok := q.filters[column]
	switch {
	case !ok:
		to.SetRange(toFieldName)
	case filter.isExpression:
		to.SetFilter(toFieldName, filter.filterExpr)
	default:
		to.SetRange(toFieldName, filter.minValue, filter.maxValue)
	}
}

// RangeMin / RangeMax return the ends of the SetRange filter on a field (nil without one)
func (q *Query) RangeMin(qt QueryTable, fieldName string) interface{} {
	if column, ok := qt.Fields.ColumnName(fieldName); ok {
		if filter, ok := q.filters[column]; ok && !filter.isExpression {
			return filter.minValue
		}
	}
	return nil
}

func (q *Query) RangeMax(qt QueryTable, fieldName string) interface{} {
	if column, ok := qt.Fields.ColumnName(fieldName); ok {
		if filter, ok := q.filters[column]; ok && !filter.isExpression {
			return filter.maxValue
		}
	}
	return nil
}

// Mark marks (or with false unmarks) a record by its key and reports whether it is marked;
// without mark it only reports it
func (q *Query) Mark(key string, values []interface{}, mark ...bool) bool {
	if len(mark) > 0 {
		if mark[0] {
			if q.marks == nil {
				q.marks = map[string][]interface{}{}
			}
			q.marks[key] = values
		} else {
			delete(q.marks, key)
		}
	}
	_, marked := q.marks[key]
	return marked
}

// MarkedOnly limits the following reads to the marked records
func (q *Query) MarkedOnly(only bool) {
	q.markedOnly = only
}

// ClearMarks removes all marks
func (q *Query) ClearMarks() {
	q.marks = nil
}

// MarkOnly marks exactly the given records (key text → key values) and limits the reads to them
func (q *Query) MarkOnly(keys map[string][]interface{}) {
	q.marks = make(map[string][]interface{}, len(keys))
	for key, values := range keys {
		q.marks[key] = values
	}
	q.markedOnly = true
}
