package tables

import (
	"testing"

	gtables "github.com/hansjlachmann/goeasyerp/backend/generated/tables"
)

// Field numbers (field IDs) from the table definitions reach the generated code
func TestFieldNumbers(t *testing.T) {
	var c gtables.Customer
	if gtables.Customer_FieldNo.Name != 2 || c.FieldNo("name") != 2 || c.FieldName(2) != "name" {
		t.Errorf("Customer.Name: %d / %d / %q", gtables.Customer_FieldNo.Name, c.FieldNo("name"), c.FieldName(2))
	}
	// FlowFilter fields have numbers too (Date Filter = 55)
	if c.FieldNo("date_filter") != 55 || c.FieldName(55) != "date_filter" {
		t.Errorf("date_filter: %d / %q", c.FieldNo("date_filter"), c.FieldName(55))
	}
	if c.FieldNo("bogus") != 0 || c.FieldName(99999) != "" {
		t.Error("unknown field or number must give 0 / \"\"")
	}
	// Every table: each field has a unique number, and FieldNo/FieldName agree with GetFields
	for _, name := range gtables.ListTableNames() {
		factory, _ := gtables.GetTableFactory(name)
		rec := factory()
		lookup := rec.(interface {
			FieldNo(string) int
			FieldName(int) string
		})
		seen := map[int]string{}
		for _, f := range rec.GetFields() {
			if f.No <= 0 || seen[f.No] != "" {
				t.Errorf("%s.%s: number %d (also %q)", name, f.Name, f.No, seen[f.No])
			}
			seen[f.No] = f.Name
			if lookup.FieldNo(f.Name) != f.No || lookup.FieldName(f.No) != f.Name {
				t.Errorf("%s.%s: FieldNo %d, FieldName(%d) %q", name, f.Name, lookup.FieldNo(f.Name), f.No, lookup.FieldName(f.No))
			}
		}
	}
}
