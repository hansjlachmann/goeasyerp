package tables

import (
	"errors"
	"strings"

	"github.com/hansjlachmann/goeasyerp/backend/foundation/flowfilter"
)

// Field filters as the list pages apply them (the filter pane), shared with the request pages
// of reports: a filter on a stored field the user may query (not sensitive or masked), or on
// a Sum/Count FlowField (its calculated value, the expression checked first).

// ErrFilterField: the filter is on a field that cannot be filtered (unknown, sensitive,
// masked); ErrFilterExpression: the expression is not valid for the field
var (
	ErrFilterField      = errors.New("the field cannot be filtered")
	ErrFilterExpression = errors.New("the filter expression is not valid")
)

// flowFieldFilterer: a record whose Sum/Count FlowFields can be filtered
type flowFieldFilterer interface {
	FlowFieldFilterKind(field string) (flowfilter.Kind, bool)
}

// ApplyFilter sets one field filter on a table as a list does; ErrFilterField or
// ErrFilterExpression when it cannot be applied (nothing is set then)
func ApplyFilter(table Table, f FieldFilter) error {
	if ff, ok := table.(flowFieldFilterer); ok {
		if kind, ok := ff.FlowFieldFilterKind(f.Field); ok {
			if err := flowfilter.Validate(kind, f.Expression); err != nil {
				return ErrFilterExpression
			}
			table.SetFilter(f.Field, f.Expression)
			return nil
		}
	}
	// A FlowFilter field (Date Filter, Location Filter …): the FlowFields that apply it use it
	for _, flowFilter := range table.GetFlowFilterFields() {
		if flowFilter.Name == f.Field {
			if err := table.SetFlowFilter(f.Field, f.Expression); err != nil {
				return ErrFilterExpression
			}
			return nil
		}
	}
	if !IsQueryableColumn(table, f.Field) {
		return ErrFilterField
	}
	table.SetFilter(f.Field, f.Expression)
	return nil
}

// FilterableFields returns the fields a list or request page can filter on: stored fields
// that are not sensitive or masked, and Sum/Count FlowFields — in field order
func FilterableFields(table Table) []FieldInfo {
	ff, _ := table.(flowFieldFilterer)
	var fields []FieldInfo
	for _, f := range table.GetFields() {
		switch {
		case f.FlowField:
			if ff != nil {
				if _, ok := ff.FlowFieldFilterKind(f.Name); ok {
					fields = append(fields, f)
				}
			}
		case IsQueryableColumn(table, f.Name):
			fields = append(fields, f)
		}
	}
	return fields
}

// FilterComparison returns the comparison operator a record filter part starts with (<=, >=,
// <, >) — "" for anything else, also for <> (not equal), which the filter handles itself
func FilterComparison(part string) string {
	if strings.HasPrefix(part, "<>") {
		return ""
	}
	for _, op := range []string{"<=", ">=", "<", ">"} {
		if strings.HasPrefix(part, op) {
			return op
		}
	}
	return ""
}

// FindKeyFilterGroup is the filter group Find("=") puts the record's key in, so the record's
// filters (also on key fields) still apply; code uses groups 0 and up
const FindKeyFilterGroup = -1
