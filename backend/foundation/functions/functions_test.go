package functions

import (
	"fmt"
	"testing"

	"github.com/hansjlachmann/goeasyerp/backend/foundation/codeunits"
)

func TestParseCall(t *testing.T) {
	cases := map[string]string{
		`table:Sales_Line.ItemAvailability(0)`:      `table:Sales_Line.ItemAvailability [0]`,
		`table:Sales_Line.InsertExtendedText(true)`: `table:Sales_Line.InsertExtendedText [true]`,
		`table:X.F( 1 , -2.5, "a, b", 'c', FALSE )`: `table:X.F [1 -2.5 a, b c false]`,
		`table:X.F()`:       `table:X.F []`,
		`table:X.F`:         `table:X.F []`,
		`codeunit:900.Post`: `codeunit:900.Post []`,
	}
	for in, want := range cases {
		name, args, err := ParseCall(in)
		if got := fmt.Sprintf("%s %v", name, []any(args)); err != nil || got != want {
			t.Errorf("ParseCall(%q) = %q, %v; want %q", in, got, err, want)
		}
	}
	for _, bad := range []string{`table:X.F(1`, `table:X.F(abc)`, `table:X.F(1,)`, `table:X.F("x)`, `table:X.F(1 2)`} {
		if _, _, err := ParseCall(bad); err == nil {
			t.Errorf("ParseCall(%q) accepted", bad)
		}
	}
}

type rec struct{ n int }

func TestResolveChecksArguments(t *testing.T) {
	Register("table", "Test_Table", "Show", ForArgs(func(r *rec, a Args) (codeunits.Result, error) {
		return codeunits.Result{Message: fmt.Sprint(r.n, a.Int(0), a.Bool(1))}, nil
	}, "int", "bool"))
	f, args, err := Resolve("table:Test_Table.Show(2, true)")
	if err != nil {
		t.Fatal(err)
	}
	res, err := f.Codeunit(args).Run(&rec{n: 7})
	if err != nil || res.Message != "7 2 true" {
		t.Errorf("run: %q, %v", res.Message, err)
	}
	for _, bad := range []string{"table:Test_Table.Show", "table:Test_Table.Show(2)", "table:Test_Table.Show(true, 2)", "table:Test_Table.Nope(1, true)"} {
		if _, _, err := Resolve(bad); err == nil {
			t.Errorf("Resolve(%q) accepted", bad)
		}
	}
	// A decimal parameter takes a whole number too
	Register("table", "Test_Table", "Amount", ForArgs(func(r *rec, a Args) (codeunits.Result, error) {
		return codeunits.Result{Message: fmt.Sprint(a.Decimal(0))}, nil
	}, "decimal"))
	if _, _, err := Resolve("table:Test_Table.Amount(5)"); err != nil {
		t.Errorf("int for decimal: %v", err)
	}
}
