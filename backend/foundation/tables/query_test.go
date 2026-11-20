package tables

import (
	"reflect"
	"strings"
	"testing"

	"github.com/hansjlachmann/goeasyerp/backend/foundation/flowfilter"
	"github.com/hansjlachmann/goeasyerp/backend/foundation/types"
)

func testQT(numeric bool) QueryTable {
	return QueryTable{Fields: testFields, NumericDecimals: numeric}
}

func TestQueryRangesAndExpressions(t *testing.T) {
	qt := testQT(true)
	var q Query
	if where, args := q.Where(qt); where != "1=1" || args != nil {
		t.Fatalf("no filters: %q %v", where, args)
	}
	q.SetRange(qt, "Code", "A")
	q.SetRange(qt, "amount", types.NewDecimalFromInt(1), types.NewDecimalFromInt(5))
	where, args := q.Where(qt)
	for _, want := range []string{"code BETWEEN ? AND ?", "CAST(amount AS REAL) BETWEEN CAST(? AS REAL) AND CAST(? AS REAL)"} {
		if !strings.Contains(where, want) {
			t.Errorf("where %q lacks %q", where, want)
		}
	}
	if len(args) != 4 {
		t.Errorf("args = %v", args)
	}
	if got := q.GetFilter(qt, "code"); got != "A" {
		t.Errorf("GetFilter(code) = %q", got)
	}
	if got := q.GetFilter(qt, "amount"); got != "1..5" {
		t.Errorf("GetFilter(amount) = %q", got)
	}
	if q.RangeMin(qt, "amount") == nil || q.RangeMax(qt, "Code") != "A" {
		t.Error("range ends")
	}

	q.Reset()
	q.SetFilter(qt, "blocked", "Yes")
	q.SetFilter(qt, "date", "..2026-12-31|''")
	where, args = q.Where(qt)
	if !strings.Contains(where, "(blocked = ?)") || !strings.Contains(where, "(date <= ? OR date IS NULL)") || !strings.Contains(where, "(date IS NULL OR date = ") {
		t.Errorf("where = %q", where)
	}
	if !reflect.DeepEqual(args[:1], []interface{}{true}) && !reflect.DeepEqual(args[len(args)-1:], []interface{}{true}) {
		t.Errorf("a boolean filter takes true: %v", args)
	}

	q.Reset()
	q.SetRange(qt, "nothing", 1)
	if where, _ := q.Where(qt); where != "1=0" {
		t.Errorf("an unknown field matches nothing: %q", where)
	}
	q.SetRange(qt, "nothing")
	q.SetRange(qt, "date", types.Date{}, types.Date{})
	if where, _ := q.Where(qt); !strings.Contains(where, "date IS NULL") {
		t.Errorf("0D..0D is the blank date: %q", where)
	}
}

func TestQueryDecimalsAsText(t *testing.T) {
	var q Query
	q.SetFilter(testQT(false), "amount", ">5")
	if where, _ := q.Where(testQT(false)); where != "(amount > ?)" {
		t.Errorf("where = %q", where)
	}
	if where, _ := q.Where(testQT(true)); where != "(CAST(amount AS REAL) > CAST(? AS REAL))" {
		t.Errorf("where = %q", where)
	}
}

func TestQueryFilterGroups(t *testing.T) {
	qt := testQT(false)
	var q Query
	q.FilterGroup(2)
	q.SetRange(qt, "code", "A")
	if g := q.FilterGroup(0); g != 0 {
		t.Fatalf("group %d", g)
	}
	q.SetRange(qt, "code", "B")
	if q.GetFilter(qt, "code") != "B" {
		t.Error("group 0 has its own filter")
	}
	where, args := q.Where(qt)
	if strings.Count(where, "code BETWEEN") != 2 || len(args) != 4 {
		t.Errorf("all groups apply: %q %v", where, args)
	}
	byGroup := q.FilterExpressionsByGroup()
	if byGroup[0][0].Expression != "B" || byGroup[2][0].Expression != "A" {
		t.Errorf("by group = %v", byGroup)
	}
	var c Query
	c.CopyFilters(&q)
	c.FilterGroup(2)
	c.SetRange(qt, "code")
	if q.FilterGroup(2); q.GetFilter(qt, "code") != "A" {
		t.Error("CopyFilters copies, it does not share")
	}
	q.ClearFilters()
	if len(q.FilterExpressions()) != 0 || q.FilterGroup() != 0 {
		t.Error("ClearFilters")
	}
}

func TestQueryOrderSearchMarks(t *testing.T) {
	qt := testQT(true)
	var q Query
	q.SetCurrentKey(qt, "Amount", "nothing")
	q.Descending = true
	if order, _ := q.OrderByClause(qt); order != "CAST(amount AS REAL) DESC, code DESC" {
		t.Errorf("order = %q", order)
	}
	if cols := q.KeyColumns(qt); !reflect.DeepEqual(cols, []string{"amount", "code"}) {
		t.Errorf("key columns = %v", cols)
	}
	q.SetSearch(qt, []string{"name", "balance"}, " 50%_ ")
	where, args := q.Where(qt)
	if where != "(LOWER(CAST(name AS TEXT)) LIKE ? ESCAPE '\\')" || args[0] != "%50\\%\\_%" {
		t.Errorf("search: %q %v", where, args)
	}
	q.Reset()
	q.Mark("A", []interface{}{"A"}, true)
	q.MarkedOnly(true)
	if where, args := q.Where(qt); where != "((code = ?))" || args[0] != "A" {
		t.Errorf("marks: %q %v", where, args)
	}
	q.ClearMarks()
	if where, _ := q.Where(qt); where != "1=0" {
		t.Errorf("marked only without marks: %q", where)
	}
}

func TestQueryFlowFieldFilter(t *testing.T) {
	qt := testQT(false)
	qt.FlowFieldKind = func(field string) (flowfilter.Kind, bool) {
		if strings.EqualFold(field, "balance") {
			return flowfilter.KindDecimal, true
		}
		return "", false
	}
	qt.FlowFieldExpr = func(field string) (string, []interface{}) {
		return "(SELECT SUM(x) FROM y WHERE z = ?)", []interface{}{"Z"}
	}
	var q Query
	q.SetFilter(qt, "Balance", ">100")
	where, args := q.Where(qt)
	if !strings.Contains(where, "(SELECT SUM(x) FROM y WHERE z = ?)") || len(args) != 2 {
		t.Errorf("flowfield filter: %q %v", where, args)
	}
	if f := q.FilterExpressions(); len(f) != 1 || f[0].Field != "balance" || f[0].Expression != ">100" {
		t.Errorf("expressions = %v", f)
	}
	q.SetCurrentKey(qt, "balance")
	if order, args := q.OrderByClause(qt); !strings.HasPrefix(order, "(SELECT") || len(args) != 1 {
		t.Errorf("order = %q %v", order, args)
	}
}
