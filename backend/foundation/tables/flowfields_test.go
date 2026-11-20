package tables

import (
	"database/sql"
	"strings"
	"testing"

	"github.com/hansjlachmann/goeasyerp/backend/foundation/flowfilter"
	"github.com/hansjlachmann/goeasyerp/backend/foundation/types"
)

// accRec is an account with FlowFields over its entries, as a generated record has them
type accRec struct {
	No         types.Code
	Totaling   types.Code
	Balance    types.Decimal // Sum(entry.amount WHERE account_no = No, date <= upper end of DateFilter)
	Entries    int           // Count(entry WHERE account_no = No)
	HasOpen    bool          // Exist(entry WHERE account_no = No, open = true)
	FirstText  types.Text    // Lookup(entry.text WHERE account_no = No)
	DateFilter string        // FlowFilter
}

var accFields = &TableFields{
	Table: "Acc",
	Fields: []FieldMeta{
		{Info: FieldInfo{No: 1, Name: "no", PrimaryKey: true}, Kind: KindCode},
		{Info: FieldInfo{No: 2, Name: "totaling"}, Kind: KindCode},
		{Info: FieldInfo{No: 3, Name: "balance", FlowField: true}, Kind: KindDecimal},
		{Info: FieldInfo{No: 4, Name: "entries", FlowField: true}, Kind: KindInt},
		{Info: FieldInfo{No: 5, Name: "has_open", FlowField: true}, Kind: KindBool},
		{Info: FieldInfo{No: 6, Name: "first_text", FlowField: true}, Kind: KindText},
	},
	FlowFilters: []FieldInfo{{No: 20, Name: "date_filter"}},
	FlowFields: []FlowFieldMeta{
		{Field: "balance", Formula: "Sum", Source: "Entry", SourceField: "amount", BatchKeyField: "account_no", BatchKeyValue: "no",
			FieldFilterColumn: "totaling",
			Filters: []FlowFilterMeta{
				{Type: FlowFieldValue, Column: "account_no", Value: "no", ValueColumn: "no", Override: "totaling", OverrideKind: flowfilter.KindCode},
				{Type: FlowFilterSet, Column: "date", Value: "date_filter", Kind: flowfilter.KindDate, UpperLimit: true},
			}},
		{Field: "entries", Formula: "Count", Source: "Entry", BatchKeyField: "account_no", BatchKeyValue: "no",
			Filters: []FlowFilterMeta{{Type: FlowFieldValue, Column: "account_no", Value: "no", ValueColumn: "no"}}},
		{Field: "has_open", Formula: "Exist", Source: "Entry",
			Filters: []FlowFilterMeta{
				{Type: FlowFieldValue, Column: "account_no", Value: "no", ValueColumn: "no"},
				{Type: FlowConst, Column: "open", Const: true},
			}},
		{Field: "first_text", Formula: "Lookup", Source: "Entry", SourceField: "text",
			Filters: []FlowFilterMeta{{Type: FlowFieldValue, Column: "account_no", Value: "no", ValueColumn: "no"}}},
	},
}

func (a *accRec) ref(i int) any {
	return []any{&a.No, &a.Totaling, &a.Balance, &a.Entries, &a.HasOpen, &a.FirstText}[i]
}

func (a *accRec) flowFilterRef(int) *string { return &a.DateFilter }

func (a *accRec) flow(db *sql.DB) *FlowRec {
	r := &FlowRec{Fields: accFields, Ref: a.ref, FlowFilter: a.flowFilterRef, DB: db, Company: "c", Outer: `"c$Acc"`}
	r.One = func(rec map[string]any, flowField int) any {
		one := &accRec{DateFilter: a.DateFilter}
		accFields.FromMap(one.ref, rec)
		f := one.flow(db)
		f.Calc(flowField)
		return f.MapValue(flowField)
	}
	return r
}

func flowDB(t *testing.T) *sql.DB {
	t.Helper()
	db, err := sql.Open("sqlite3", ":memory:")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close() })
	db.SetMaxOpenConns(1)
	if _, err := db.Exec(`CREATE TABLE "c$Entry" (account_no TEXT, date TEXT, amount REAL, open BOOLEAN, text TEXT)`); err != nil {
		t.Fatal(err)
	}
	for _, e := range []string{
		`('1000', '2026-01-10', 100, 1, 'first')`, `('1000', '2026-02-10', 50, 0, 'second')`,
		`('2000', '2026-01-15', -30, 0, 'other')`,
	} {
		if _, err := db.Exec(`INSERT INTO "c$Entry" VALUES ` + e); err != nil {
			t.Fatal(err)
		}
	}
	return db
}

func TestFlowFieldsOneRecord(t *testing.T) {
	db := flowDB(t)
	a := &accRec{No: "1000"}
	f := a.flow(db)
	f.CalcFields(nil)
	if a.Balance.String() != "150" || a.Entries != 2 || !a.HasOpen || a.FirstText != "first" {
		t.Fatalf("1000: %+v", a)
	}
	// the date filter's upper end only: balance at date
	a.DateFilter = "2026-02-01..2026-01-31"
	f.CalcFields([]string{"balance"})
	if a.Balance.String() != "100" {
		t.Errorf("balance up to 2026-01-31 = %s", a.Balance)
	}
	a.DateFilter = ""
	// a total account sums its range (Totaling)
	a.Totaling = "1000..2000"
	f.CalcFields([]string{"balance", "nothing"})
	if a.Balance.String() != "120" {
		t.Errorf("total 1000..2000 = %s", a.Balance)
	}
	b := &accRec{No: "3000"}
	b.flow(db).CalcFields(nil)
	if !b.Balance.IsZero() || b.Entries != 0 || b.HasOpen || b.FirstText != "" {
		t.Errorf("3000: %+v", b)
	}
}

func TestFlowFieldsForRecords(t *testing.T) {
	db := flowDB(t)
	a := &accRec{}
	records := []map[string]any{
		{"no": "1000", "totaling": ""},
		{"no": "2000", "totaling": ""},
		{"no": "3000", "totaling": ""},
		{"no": "9000", "totaling": "1000..2000"},
	}
	a.flow(db).CalcFieldsForRecords(records, nil)
	want := []struct {
		balance string
		entries int
		open    bool
		text    any
	}{{"150", 2, true, types.Text("first")}, {"-30", 1, false, types.Text("other")}, {"0", 0, false, types.Text("")}, {"120", 0, false, types.Text("")}}
	for i, w := range want {
		r := records[i]
		if r["balance"] != w.balance || r["entries"] != w.entries || r["has_open"] != w.open || r["first_text"] != w.text {
			t.Errorf("record %d: %v", i, r)
		}
	}
}

func TestFlowFieldReverseSign(t *testing.T) {
	db := flowDB(t)
	reversed := &TableFields{Table: accFields.Table, Fields: accFields.Fields, FlowFilters: accFields.FlowFilters,
		FlowFields: append([]FlowFieldMeta(nil), accFields.FlowFields...)}
	reversed.FlowFields[0].ReverseSign = true
	reversed.FlowFields[0].FieldFilterColumn = ""
	a := &accRec{No: "1000"}
	f := a.flow(db)
	f.Fields = reversed
	f.CalcFields([]string{"balance"})
	if a.Balance.String() != "-150" {
		t.Errorf("reversed balance = %s", a.Balance)
	}
	records := []map[string]any{{"no": "1000"}, {"no": "2000"}}
	f.CalcFieldsForRecords(records, []string{"balance"})
	if records[0]["balance"] != "-150" || records[1]["balance"] != "30" {
		t.Errorf("reversed list = %v", records)
	}
	if expr, _ := f.Expr("balance"); !strings.Contains(expr, "-COALESCE(SUM(amount), 0)") {
		t.Errorf("reversed expr = %q", expr)
	}
}

func TestFlowFieldListFilter(t *testing.T) {
	db := flowDB(t)
	a := &accRec{}
	f := a.flow(db)
	if k, ok := accFields.FlowFieldFilterKind("ENTRIES"); !ok || k != flowfilter.KindInt {
		t.Errorf("entries: %v %v", k, ok)
	}
	if _, ok := accFields.FlowFieldFilterKind("balance"); ok {
		t.Error("a FlowField with a field filter is not filterable")
	}
	if _, ok := accFields.FlowFieldFilterKind("has_open"); ok {
		t.Error("an Exist FlowField is not filterable")
	}
	expr, args := f.Expr("entries")
	if expr != `(SELECT COUNT(*) FROM "c$Entry" WHERE account_no = "c$Acc".no)` || len(args) != 0 {
		t.Errorf("expr = %q %v", expr, args)
	}
	if expr, _ := f.Expr("balance"); expr != "NULL" {
		t.Errorf("balance expr = %q", expr)
	}
	if !strings.Contains(expr, "c$Acc") {
		t.Error("the subquery refers to the outer table")
	}
}
