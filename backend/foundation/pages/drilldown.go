package pages

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/hansjlachmann/goeasyerp/backend/foundation/flowfilter"
)

// DrilldownFilter is one filter of the page a drilldown opens: a field of its table and the
// filter expression (as the list's ?filter=field=expression)
type DrilldownFilter struct {
	Field      string `json:"field"`
	Expression string `json:"expression"`
}

// FlowFieldDrilldownFilters gives the filters of a FlowField's drilldown, as the FlowField
// calculates (classic DrillDown on a FlowField: its source table filtered by the CalcFormula):
// a field filter takes the record's value (blank: ”), a field_filter the filter held in the
// record's field — when it has one it replaces the equality on the same field (a total account
// sums its Totaling) —, a filter the FlowFilter set on the record (flowFilters by name; only
// when set, UpperLimit: up to its upper end — kinds give the FlowFilters' kinds), a const its
// value. Language filters are left out.
func FlowFieldDrilldownFilters(def FlowFieldDef, record map[string]interface{}, flowFilters map[string]string, kinds map[string]flowfilter.Kind) []DrilldownFilter {
	held := map[string]bool{} // fields a field_filter with a value filters
	for _, f := range def.Filters {
		if f.Type == "field_filter" && drilldownValue(record[f.Value]) != "" {
			held[f.Field] = true
		}
	}
	var filters []DrilldownFilter
	for _, f := range def.Filters {
		switch f.Type {
		case "field":
			if held[f.Field] {
				continue
			}
			value := drilldownValue(record[f.Value])
			if value == "" {
				value = "''"
			}
			filters = append(filters, DrilldownFilter{Field: f.Field, Expression: value})
		case "field_filter":
			value := drilldownValue(record[f.Value])
			if value != "" {
				filters = append(filters, DrilldownFilter{Field: f.Field, Expression: value})
			}
		case "filter":
			expr := strings.TrimSpace(flowFilters[f.Value])
			if expr == "" {
				continue
			}
			if f.UpperLimit {
				expr = flowfilter.UpperLimit(kinds[f.Value], expr)
				if expr == "" {
					continue
				}
			}
			filters = append(filters, DrilldownFilter{Field: f.Field, Expression: expr})
		case "const":
			filters = append(filters, DrilldownFilter{Field: f.Field, Expression: f.Value})
		}
	}
	return filters
}

// drilldownValue: a record value as filter text ("" for none)
func drilldownValue(v interface{}) string {
	switch x := v.(type) {
	case nil:
		return ""
	case string:
		return strings.TrimSpace(x)
	case float64:
		return strconv.FormatFloat(x, 'f', -1, 64)
	case bool:
		if x {
			return "true"
		}
		return "false"
	default:
		return strings.TrimSpace(fmt.Sprint(x))
	}
}
