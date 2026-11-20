package tables

import (
	"database/sql"
	"fmt"
	"strconv"
	"strings"

	"github.com/hansjlachmann/goeasyerp/backend/foundation/database"
	"github.com/hansjlachmann/goeasyerp/backend/foundation/flowfilter"
	"github.com/hansjlachmann/goeasyerp/backend/foundation/i18n"
	"github.com/hansjlachmann/goeasyerp/backend/foundation/sift"
	"github.com/hansjlachmann/goeasyerp/backend/foundation/types"
)

// FlowFields (Sum, Count, Exist, Lookup), written once for all tables (see fields.go): a
// table's FlowFields are described by data (FlowFieldMeta in its TableFields) and CalcFields,
// CalcFieldsForRecords, FlowFieldFilterKind and flowFieldExpr of the generated record call
// these with its FlowRec.

// FlowFilterType is the type of a FlowField's filter on its source table
type FlowFilterType uint8

const (
	FlowConst      FlowFilterType = iota + 1 // a constant (or the user's language)
	FlowFieldValue                           // a field of the record (CalcFormula FIELD)
	FlowFilterSet                            // a FlowFilter field of the record, when set (FILTER)
)

// FlowFilterMeta is a FlowField's filter on its source table
type FlowFilterMeta struct {
	Type   FlowFilterType
	Column string // the source table's column
	// FlowConst: the value; Language: the user's language instead (i18n.SessionLanguageID —
	// Object Caption)
	Const    any
	Language bool
	// FlowFieldValue: the record's field; ValueColumn its column (list subqueries); Override
	// a field of the record holding a filter of Column (field_filter: a total account sums
	// its Totaling), with the filter's kind
	Value        string
	ValueColumn  string
	Override     string
	OverrideKind flowfilter.Kind
	// FlowFilterSet: the FlowFilter field (Value), its kind and whether only its upper end
	// applies (UpperLimit: balance up to a date)
	Kind       flowfilter.Kind
	UpperLimit bool
}

// FlowFieldMeta describes a FlowField
type FlowFieldMeta struct {
	Field        string // the FlowField
	Formula      string // Sum, Count, Exist, Lookup
	Source       string // the source table's name
	SourceGlobal bool   // the source table is global (no company prefix)
	SourceField  string // Sum: the summed column; Lookup: the column read
	ReverseSign  bool   // Sum: the total with the opposite sign
	// the SIFT key read without / with a FlowFilter set ("": the entries)
	SIFTKey         string
	SIFTKeyFiltered string
	Filters         []FlowFilterMeta
	// BatchKeyField / BatchKeyValue: the source column and the record field (as ToMap names
	// it) a list's FlowFields are grouped by ("": calculated per record)
	BatchKeyField string
	BatchKeyValue string
	// FieldFilterColumn: the record's column holding a filter (field_filter); such records are
	// calculated one by one
	FieldFilterColumn string
}

// FlowRec is what the FlowField functions need of a record (the generated flow())
type FlowRec struct {
	Fields     *TableFields
	Ref        FieldRef
	FlowFilter func(i int) *string // the FlowFilter fields, by index in Fields.FlowFilters
	DB         database.Executor   // the real database (also for a temporary record)
	Company    string
	Postgres   bool
	// Outer: the record's own table quoted, as the list subqueries refer to it
	Outer string
	// One calculates a FlowField (by index in Fields.FlowFields) for one record of a list —
	// a record of the table from the map with this record's FlowFilters — in its map form
	One func(rec map[string]any, flowField int) any
}

func (r *FlowRec) placeholders(query string, count int) string {
	if !r.Postgres {
		return query
	}
	for i := 1; i <= count; i++ {
		query = strings.Replace(query, "?", fmt.Sprintf("$%d", i), 1)
	}
	return query
}

// flowFilter returns the value of a FlowFilter field of the record
func (r *FlowRec) flowFilter(name string) string {
	for i, f := range r.Fields.FlowFilters {
		if f.Name == name {
			return *r.FlowFilter(i)
		}
	}
	return ""
}

// fieldValue returns the value of a field of the record by its name
func (r *FlowRec) fieldValue(name string) any {
	i := r.Fields.Index(name)
	if i < 0 {
		return nil
	}
	return Value(r.Ref(i))
}

// entries is the source table's name in the record's company (or without prefix: global)
func (r *FlowRec) entries(ff *FlowFieldMeta) string {
	if ff.SourceGlobal {
		return ff.Source
	}
	return fmt.Sprintf("%s$%s", r.Company, ff.Source)
}

// source is the table a Sum/Count reads: the SIFT totals of a key covering the filters, else
// the entries; a FlowFilter set needs a key with the filtered fields too
func (r *FlowRec) source(ff *FlowFieldMeta) (table string, useSIFT bool) {
	table, useSIFT = r.entries(ff), false
	if ff.SIFTKey != "" {
		table, useSIFT = sift.TableName(r.Company, ff.Source, ff.SIFTKey), true
	}
	filterSet := false
	for _, f := range ff.Filters {
		if f.Type == FlowFilterSet && r.flowFilter(f.Value) != "" {
			filterSet = true
		}
	}
	if filterSet {
		if ff.SIFTKeyFiltered != "" {
			table, useSIFT = sift.TableName(r.Company, ff.Source, ff.SIFTKeyFiltered), true
		} else {
			table, useSIFT = r.entries(ff), false
		}
	}
	return table, useSIFT
}

// whereMode is which of the record's filters a query applies
type whereMode int

const (
	whereRecord whereMode = iota // constants and the record's fields (with a field_filter)
	whereLookup                  // constants and the record's fields
	whereBatch                   // constants only: the record field is the grouping
	whereOuter                   // constants and the record's columns of the outer query
)

func (r *FlowRec) where(ff *FlowFieldMeta, mode whereMode) ([]string, []any) {
	var clauses []string
	var args []any
	for _, f := range ff.Filters {
		switch f.Type {
		case FlowConst:
			clauses = append(clauses, f.Column+" = ?")
			if f.Language {
				args = append(args, i18n.SessionLanguageID())
			} else {
				args = append(args, f.Const)
			}
		case FlowFieldValue:
			switch mode {
			case whereBatch:
			case whereOuter:
				clauses = append(clauses, f.Column+" = "+r.Outer+"."+f.ValueColumn)
			case whereRecord:
				if f.Override != "" {
					if s := strings.TrimSpace(fmt.Sprint(r.fieldValue(f.Override))); s != "" {
						// the field holds a filter of the column (a total account sums its range)
						if clause, filterArgs, err := flowfilter.Clause(f.Column, f.OverrideKind, s); err == nil && clause != "" {
							clauses = append(clauses, clause)
							args = append(args, filterArgs...)
						} else {
							clauses = append(clauses, "1=0")
						}
						continue
					}
				}
				fallthrough
			default:
				clauses = append(clauses, f.Column+" = ?")
				args = append(args, r.fieldValue(f.Value))
			}
		}
	}
	for _, f := range ff.Filters {
		if f.Type != FlowFilterSet {
			continue
		}
		value := r.flowFilter(f.Value)
		if value == "" {
			continue
		}
		if f.UpperLimit {
			value = flowfilter.UpperLimit(f.Kind, value)
		}
		if clause, filterArgs, _ := flowfilter.Clause(f.Column, f.Kind, value); clause != "" {
			clauses = append(clauses, clause)
			args = append(args, filterArgs...)
		}
	}
	return clauses, args
}

func joinWhere(clauses []string) string {
	if len(clauses) == 0 {
		return "1=1"
	}
	return strings.Join(clauses, " AND ")
}

// Sum calculates a Sum FlowField for the record
func (r *FlowRec) Sum(ff *FlowFieldMeta) types.Decimal {
	table, _ := r.source(ff)
	clauses, args := r.where(ff, whereRecord)
	sign := ""
	if ff.ReverseSign {
		sign = "-"
	}
	query := fmt.Sprintf(`SELECT %sCOALESCE(SUM(%s), 0) FROM "%s" WHERE %s`, sign, ff.SourceField, table, joinWhere(clauses))
	query = r.placeholders(query, len(args))
	var sumStr string
	if err := r.DB.QueryRow(query, args...).Scan(&sumStr); err != nil {
		fmt.Printf("Error: Failed to calculate sum for %s: %v\n", ff.Field, err)
		return types.ZeroDecimal()
	}
	sum, _ := types.NewDecimalFromString(sumStr)
	return sum
}

// Count calculates a Count (or Exist) FlowField for the record
func (r *FlowRec) Count(ff *FlowFieldMeta) int {
	table, useSIFT := r.source(ff)
	clauses, args := r.where(ff, whereRecord)
	agg := "COUNT(*)"
	if useSIFT {
		agg = "COALESCE(SUM(cnt), 0)" // the totals hold the entry count per key value
	}
	query := fmt.Sprintf(`SELECT %s FROM "%s" WHERE %s`, agg, table, joinWhere(clauses))
	query = r.placeholders(query, len(args))
	var count int
	if err := r.DB.QueryRow(query, args...).Scan(&count); err != nil {
		fmt.Printf("Error: Failed to calculate count for %s: %v\n", ff.Field, err)
		return 0
	}
	return count
}

// Lookup reads a Lookup FlowField's source column of the first record within its filters
// ("" when there is none)
func (r *FlowRec) Lookup(ff *FlowFieldMeta) string {
	clauses, args := r.where(ff, whereLookup)
	query := r.placeholders(fmt.Sprintf(`SELECT %s FROM "%s" WHERE %s LIMIT 1`, ff.SourceField, r.entries(ff), joinWhere(clauses)), len(args))
	var value sql.NullString
	if err := r.DB.QueryRow(query, args...).Scan(&value); err != nil && err != sql.ErrNoRows {
		fmt.Printf("Error: Failed to look up %s: %v\n", ff.Field, err)
	}
	return value.String
}

// flowFieldIndex returns the index of a FlowField in Fields.FlowFields, -1 when unknown
func (tf *TableFields) flowFieldIndex(name string) int {
	for i := range tf.FlowFields {
		if tf.FlowFields[i].Field == name {
			return i
		}
	}
	return -1
}

// Calc calculates a FlowField (by index in Fields.FlowFields) into the record
func (r *FlowRec) Calc(flowField int) {
	ff := &r.Fields.FlowFields[flowField]
	i := r.Fields.Index(ff.Field)
	if i < 0 {
		return
	}
	switch p := r.Ref(i).(type) {
	case *types.Decimal:
		if ff.Formula == "Sum" {
			*p = r.Sum(ff)
		}
	case *int:
		if ff.Formula == "Count" {
			*p = r.Count(ff)
		}
	case *bool:
		if ff.Formula == "Exist" {
			*p = r.Count(ff) > 0
		}
	case *types.Code:
		if ff.Formula == "Lookup" {
			*p = types.NewCode(r.Lookup(ff))
		}
	case *types.Text:
		if ff.Formula == "Lookup" {
			*p = types.NewText(r.Lookup(ff))
		}
	}
}

// CalcFields calculates the named FlowFields into the record (all without names); other
// names are ignored
func (r *FlowRec) CalcFields(names []string) {
	if len(names) == 0 {
		for i := range r.Fields.FlowFields {
			r.Calc(i)
		}
		return
	}
	for _, name := range names {
		if i := r.Fields.flowFieldIndex(name); i >= 0 {
			r.Calc(i)
		}
	}
}

// MapValue is a FlowField's value as CalcFieldsForRecords puts it into a record's map: a
// decimal as text, a count as a number, Exist as Yes/No, a lookup as its value
func (r *FlowRec) MapValue(flowField int) any {
	ff := &r.Fields.FlowFields[flowField]
	i := r.Fields.Index(ff.Field)
	if i < 0 {
		return nil
	}
	if d, ok := r.Ref(i).(*types.Decimal); ok {
		return d.String()
	}
	return Value(r.Ref(i))
}

// CalcFieldsForRecords calculates FlowFields for many records at once (list pages): one
// grouped query per FlowField instead of one per record; records are ToMap() results and get
// the values under the field names. Without names, all FlowFields.
func (r *FlowRec) CalcFieldsForRecords(records []map[string]any, names []string) {
	if len(records) == 0 {
		return
	}
	if len(names) == 0 {
		for i := range r.Fields.FlowFields {
			names = append(names, r.Fields.FlowFields[i].Field)
		}
	}
	for _, name := range names {
		if i := r.Fields.flowFieldIndex(name); i >= 0 {
			r.forRecords(i, records)
		}
	}
}

func (r *FlowRec) forRecords(flowField int, records []map[string]any) {
	ff := &r.Fields.FlowFields[flowField]
	counted := ff.Formula == "Count" || ff.Formula == "Exist"
	if ff.FieldFilterColumn != "" {
		// records whose column holds a filter (total accounts) are calculated one by one,
		// with this record's FlowFilters; the others in the grouped query below
		var grouped []map[string]any
		for _, rec := range records {
			if s := strings.TrimSpace(fmt.Sprint(rec[ff.FieldFilterColumn])); s == "" || s == "<nil>" {
				grouped = append(grouped, rec)
				continue
			}
			rec[ff.Field] = r.One(rec, flowField)
		}
		records = grouped
		if len(records) == 0 {
			return
		}
	}
	if ff.BatchKeyField == "" || (ff.Formula != "Sum" && !counted) {
		// no single key field to group by: per record, with this record's FlowFilters
		for _, rec := range records {
			rec[ff.Field] = r.One(rec, flowField)
		}
		return
	}

	table, useSIFT := r.source(ff)
	// the records' distinct key values
	seen := make(map[string]bool, len(records))
	var keys []any
	for _, rec := range records {
		k := fmt.Sprint(rec[ff.BatchKeyValue])
		if !seen[k] {
			seen[k] = true
			keys = append(keys, rec[ff.BatchKeyValue])
		}
	}
	// One grouped query for the keys (IN list). With more keys than chunkSize (a long list) one
	// grouped query over the whole source table instead: much cheaper than many IN queries,
	// and it keeps the bind parameters bounded.
	counts := make(map[string]int, len(keys))
	sums := make(map[string]types.Decimal, len(keys))
	const chunkSize = 500
	chunks := [][]any{nil} // nil chunk: no IN filter, all keys
	if len(keys) <= chunkSize {
		chunks = [][]any{keys}
	}
	for _, chunk := range chunks {
		clauses, args := r.where(ff, whereBatch)
		if chunk != nil {
			clauses = append(clauses, ff.BatchKeyField+" IN ("+strings.TrimSuffix(strings.Repeat("?, ", len(chunk)), ", ")+")")
			args = append(args, chunk...)
		}
		var query string
		if counted {
			agg := "COUNT(*)"
			if useSIFT {
				agg = "COALESCE(SUM(cnt), 0)" // the totals hold the entry count per key value
			}
			query = fmt.Sprintf(`SELECT %s, %s FROM "%s" WHERE %s GROUP BY %s`, ff.BatchKeyField, agg, table, joinWhere(clauses), ff.BatchKeyField)
		} else {
			sign := ""
			if ff.ReverseSign {
				sign = "-"
			}
			query = fmt.Sprintf(`SELECT %s, %sCOALESCE(SUM(%s), 0) FROM "%s" WHERE %s GROUP BY %s`, ff.BatchKeyField, sign, ff.SourceField, table, joinWhere(clauses), ff.BatchKeyField)
		}
		query = r.placeholders(query, len(args))
		rows, err := r.DB.Query(query, args...)
		if err != nil {
			fmt.Printf("Error: Failed to calculate %s: %v\n", ff.Field, err)
			return
		}
		for rows.Next() {
			var key string
			if counted {
				var count int
				if err := rows.Scan(&key, &count); err == nil {
					counts[key] = count
				}
			} else {
				var sumStr string
				if err := rows.Scan(&key, &sumStr); err == nil {
					sums[key], _ = types.NewDecimalFromString(sumStr)
				}
			}
		}
		_ = rows.Close()
	}
	// records without entries get zero
	for _, rec := range records {
		k := fmt.Sprint(rec[ff.BatchKeyValue])
		switch {
		case ff.Formula == "Exist":
			rec[ff.Field] = counts[k] > 0
		case counted:
			rec[ff.Field] = counts[k]
		default:
			v, ok := sums[k]
			if !ok {
				v = types.ZeroDecimal()
			}
			rec[ff.Field] = v.String()
		}
	}
}

// filterable reports whether lists filter and sort on a FlowField: Sum and Count without a
// field_filter
func (ff *FlowFieldMeta) filterable() bool {
	return (ff.Formula == "Sum" || ff.Formula == "Count") && ff.FieldFilterColumn == ""
}

// FlowFieldFilterKind reports whether a list can filter and sort on a FlowField and the kind
// of its values
func (tf *TableFields) FlowFieldFilterKind(field string) (flowfilter.Kind, bool) {
	i := tf.flowFieldIndex(strings.ToLower(field))
	if i < 0 || !tf.FlowFields[i].filterable() {
		return "", false
	}
	if tf.FlowFields[i].Formula == "Count" {
		return flowfilter.KindInt, true
	}
	return flowfilter.KindDecimal, true
}

// Expr is the SQL expression computing a FlowField for each row of a query on the record's
// table: a correlated subquery reading the source like CalcFieldsForRecords, with the
// record's FlowFilters ("NULL" for a FlowField lists cannot filter on)
func (r *FlowRec) Expr(field string) (string, []any) {
	i := r.Fields.flowFieldIndex(strings.ToLower(field))
	if i < 0 || !r.Fields.FlowFields[i].filterable() {
		return "NULL", nil
	}
	ff := &r.Fields.FlowFields[i]
	table, useSIFT := r.source(ff)
	clauses, args := r.where(ff, whereOuter)
	var agg string
	if ff.Formula == "Count" {
		agg = "COUNT(*)"
		if useSIFT {
			agg = "COALESCE(SUM(cnt), 0)"
		}
	} else {
		agg = "COALESCE(SUM(" + ff.SourceField + "), 0)"
		if ff.ReverseSign {
			agg = "-" + agg
		}
	}
	return "(SELECT " + agg + " FROM \"" + table + "\" WHERE " + joinWhere(clauses) + ")", args
}

// CalcSums sums decimal and whole number fields over the records within the filters into
// the record (CalcSums)
func (r *Rec) CalcSums(fieldNames []string) error {
	where, args := r.Q.Where(r.QT)
	for _, name := range fieldNames {
		i := r.Fields.Index(name)
		if i < 0 || r.Fields.Fields[i].Info.FlowField || (r.Fields.Fields[i].Kind != KindDecimal && r.Fields.Fields[i].Kind != KindInt) {
			return fmt.Errorf("CalcSums: %s is not a decimal or integer field of %s", name, r.Fields.Table)
		}
		column := r.Fields.Fields[i].Info.Name
		query := r.Placeholders(fmt.Sprintf(`SELECT CAST(COALESCE(SUM(%s), 0) AS TEXT) FROM "%s" WHERE %s`, column, r.Table, where), len(args))
		var total string
		if err := r.Exec.QueryRow(query, args...).Scan(&total); err != nil {
			return fmt.Errorf("CalcSums %s.%s: %w", r.Fields.Table, column, err)
		}
		switch p := r.Ref(i).(type) {
		case *types.Decimal:
			d, err := types.NewDecimalFromString(total)
			if err != nil {
				return fmt.Errorf("CalcSums %s.%s: %w", r.Fields.Table, column, err)
			}
			*p = d
		case *int:
			f, err := strconv.ParseFloat(total, 64)
			if err != nil {
				return fmt.Errorf("CalcSums %s.%s: %w", r.Fields.Table, column, err)
			}
			*p = int(f)
		}
	}
	return nil
}
