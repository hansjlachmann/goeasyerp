package tables

import (
	"testing"

	"github.com/hansjlachmann/goeasyerp/backend/foundation/database"
	"github.com/hansjlachmann/goeasyerp/backend/foundation/types"
	gtables "github.com/hansjlachmann/goeasyerp/backend/generated/tables"
)

// newEntries creates a ledger with entries (entry no, customer, posting date)
func newEntries(t *testing.T) *gtables.CustomerLedgerEntry {
	t.Helper()
	db := newTestDB(t)
	if err := (&gtables.CustomerLedgerEntry{}).CreateTableWithDBType(db, "c1", database.DBTypeSQLite); err != nil {
		t.Fatal(err)
	}
	for _, e := range []struct {
		no       int
		customer string
		date     string
	}{
		{1, "C1", "2026-03-01"}, {2, "C2", "2026-01-15"}, {3, "C1", "2026-01-15"},
		{4, "C1", "2026-02-01"}, {5, "C2", "2026-03-01"},
	} {
		r := &gtables.CustomerLedgerEntry{}
		r.InitWithDBType(db, "c1", database.DBTypeSQLite)
		r.Entry_no, r.Customer_no, r.Posting_date = e.no, types.NewCode(e.customer), types.MustDate(e.date)
		if !r.Insert(false) {
			t.Fatalf("insert %d", e.no)
		}
	}
	var r gtables.CustomerLedgerEntry
	r.InitWithDBType(db, "c1", database.DBTypeSQLite)
	return &r
}

// walk returns the entry numbers Find(first) then Find(next) visit
func walk(r *gtables.CustomerLedgerEntry, first, next string) []int {
	var nos []int
	for ok := r.Find(first); ok && len(nos) < 10; ok = r.Find(next) {
		nos = append(nos, r.Entry_no)
	}
	return nos
}

func same(a, b []int) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

// Find("-"/"+"/">"/"<") follow the current key and direction, within the filters
func TestFindNextAndPrevious(t *testing.T) {
	r := newEntries(t)
	if got := walk(r, "-", ">"); !same(got, []int{1, 2, 3, 4, 5}) {
		t.Errorf("primary key: %v", got)
	}
	r.SetCurrentKey("posting_date") // equal dates in primary key order
	if got := walk(r, "-", ">"); !same(got, []int{2, 3, 4, 1, 5}) {
		t.Errorf("posting date: %v", got)
	}
	if got := walk(r, "+", "<"); !same(got, []int{5, 1, 4, 3, 2}) {
		t.Errorf("posting date backwards: %v", got)
	}
	r.SetAscending(false)
	if got := walk(r, "-", ">"); !same(got, []int{5, 1, 4, 3, 2}) {
		t.Errorf("descending: %v", got)
	}
	r.SetAscending(true)
	r.SetRange("customer_no", "C1")
	if got := walk(r, "-", ">"); !same(got, []int{3, 4, 1}) {
		t.Errorf("filtered: %v", got)
	}
	// "=>": the current record or else the next one
	r.Entry_no, r.Customer_no, r.Posting_date = 2, "C2", types.MustDate("2026-01-15")
	if !r.Find("=>") || r.Entry_no != 3 {
		t.Errorf("=> from a record outside the filter: %d", r.Entry_no)
	}
	if !r.Find("=>") || r.Entry_no != 3 {
		t.Errorf("=> on a record in the filter: %d", r.Entry_no)
	}
	// Find(">") does not disturb paging or the filters
	if r.GetFilter("customer_no") != "C1" || r.Count() != 3 {
		t.Errorf("filters after Find: %q, %d", r.GetFilter("customer_no"), r.Count())
	}
}

// FilterGroup: each group has its own filters, all apply together
func TestFilterGroup(t *testing.T) {
	r := newEntries(t)
	r.FilterGroup(2)
	r.SetRange("customer_no", "C1") // set by code: the user cannot remove it in group 0
	if r.FilterGroup(0) != 0 || r.FilterGroup() != 0 {
		t.Fatal("FilterGroup(0)")
	}
	if r.GetFilter("customer_no") != "" {
		t.Errorf("group 0 sees group 2's filter %q", r.GetFilter("customer_no"))
	}
	r.SetRange("customer_no", "C2") // the user's filter: together with group 2 nothing matches
	if n := r.Count(); n != 0 {
		t.Errorf("C1 and C2: %d", n)
	}
	r.SetRange("customer_no") // removing the user's filter keeps group 2's
	if n := r.Count(); n != 3 {
		t.Errorf("group 2 only: %d", n)
	}
	r.SetFilter("posting_date", "2026-02-01..2026-03-31")
	if n := r.Count(); n != 2 {
		t.Errorf("group 2 and a date filter: %d", n)
	}

	var c gtables.CustomerLedgerEntry
	c.InitWithDBType(r.GetDB(), "c1", database.DBTypeSQLite)
	c.CopyFilters(r)
	if n := c.Count(); n != 2 {
		t.Errorf("CopyFilters copies all groups: %d", n)
	}
	c.FilterGroup(2)
	if c.GetFilter("customer_no") != "C1" {
		t.Errorf("copied group 2: %q", c.GetFilter("customer_no"))
	}

	r.Reset()
	if r.FilterGroup() != 0 || r.Count() != 5 {
		t.Errorf("Reset: group %d, %d entries", r.FilterGroup(), r.Count())
	}
}

// SetSelectionFilter: the records selected on the list, or else the current record
func TestSetSelectionFilter(t *testing.T) {
	r := newEntries(t)
	r.Entry_no = 4
	r.SetSelectionFilter() // nothing selected: the current record
	if r.Count() != 1 || !r.Find("-") || r.Entry_no != 4 {
		t.Errorf("current record: %d entries, entry %d", r.Count(), r.Entry_no)
	}

	r.Reset()
	r.SetSelection([]map[string]interface{}{{"entry_no": float64(5)}}) // JSON numbers
	r.SetSelectionFilter()
	if r.Count() != 1 || !r.Find("-") || r.Entry_no != 5 {
		t.Errorf("one selected: %d entries, entry %d", r.Count(), r.Entry_no)
	}

	r.Reset()
	r.SetSelection([]map[string]interface{}{{"entry_no": 1}, {"entry_no": 3}, {"entry_no": 5}})
	r.SetSelectionFilter()
	r.SetRange("customer_no", "C1")
	if got := walk(r, "-", ">"); !same(got, []int{1, 3}) {
		t.Errorf("three selected, filtered on C1: %v", got)
	}
}
