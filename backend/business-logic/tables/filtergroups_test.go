package tables

import (
	"testing"

	gtables "github.com/hansjlachmann/goeasyerp/backend/generated/tables"
)

// FilterExpressionsByGroup gives each filter group's filters — a filter on one field in two
// groups twice; the record stays in the group it was in
func TestFilterExpressionsByGroup(t *testing.T) {
	var cust gtables.Customer
	cust.FilterGroup(2)
	cust.SetFilter("no", "C01..C05")
	cust.SetRange("city", "Oslo")
	cust.FilterGroup(0)
	cust.SetFilter("no", "C02..")
	byGroup := cust.FilterExpressionsByGroup()
	g0, g2 := byGroup[0], byGroup[2]
	if len(byGroup) != 2 || len(g0) != 1 || g0[0].Field != "no" || g0[0].Expression != "C02.." ||
		len(g2) != 2 || g2[0].Field != "city" || g2[0].Expression != "Oslo" || g2[1].Field != "no" || g2[1].Expression != "C01..C05" {
		t.Errorf("by group %+v", byGroup)
	}
	if cust.FilterGroup() != 0 || len(cust.FilterExpressions()) != 3 {
		t.Errorf("group %d, all %+v", cust.FilterGroup(), cust.FilterExpressions())
	}
}
