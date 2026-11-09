package flowfilter

import (
	"fmt"
	"reflect"
	"testing"
)

func TestClause(t *testing.T) {
	tests := []struct {
		kind Kind
		expr string
		sql  string
		args string
	}{
		{KindDate, "", "", "[]"},
		{KindDate, "2026-01-01..2026-03-31", "posting_date BETWEEN ? AND ?", "[2026-01-01 2026-03-31]"},
		{KindDate, "..2026-03-31", "posting_date <= ?", "[2026-03-31]"},
		{KindDate, "2026-01-01..", "posting_date >= ?", "[2026-01-01]"},
		{KindDate, " 2026-05-17 ", "posting_date = ?", "[2026-05-17]"},
		{KindDate, "2026-01-01..2026-01-31|2026-03-01..2026-03-31", "(posting_date BETWEEN ? AND ? OR posting_date BETWEEN ? AND ?)", "[2026-01-01 2026-01-31 2026-03-01 2026-03-31]"},
		{KindDate, "<>2026-05-17", "posting_date <> ?", "[2026-05-17]"},
		{KindInt, "10..20|30", "(posting_date BETWEEN ? AND ? OR posting_date = ?)", "[10 20 30]"},
		{KindBool, "Yes", "posting_date = ?", "[true]"},
		{KindBool, "no", "posting_date = ?", "[false]"},
		{KindCode, "c0001*", `posting_date LIKE ? ESCAPE '\'`, "[C0001%]"},
		{KindCode, "c00010", "posting_date = ?", "[C00010]"},
		{KindText, "*oslo*", `posting_date LIKE ? ESCAPE '\'`, "[%oslo%]"},
		{KindText, "50%*", `posting_date LIKE ? ESCAPE '\'`, `[50\%%]`},
		{KindText, "A..M", "posting_date BETWEEN ? AND ?", "[A M]"},
		{KindCode, "''", "posting_date = ?", "[]"},
		{KindCode, "''|main", "(posting_date = ? OR posting_date = ?)", "[ MAIN]"},
		{KindCode, "<>''", "posting_date <> ?", "[]"},
	}
	for _, tt := range tests {
		sql, args, err := Clause("posting_date", tt.kind, tt.expr)
		if err != nil {
			t.Errorf("%s %q: %v", tt.kind, tt.expr, err)
			continue
		}
		if sql != tt.sql || fmt.Sprint(args) != tt.args {
			t.Errorf("%s %q = %q %v, want %q %s", tt.kind, tt.expr, sql, args, tt.sql, tt.args)
		}
	}
}

func TestClauseErrors(t *testing.T) {
	for _, tt := range []struct {
		kind Kind
		expr string
	}{
		{KindDate, "01.01.26"},       // not ISO (the frontend converts local dates)
		{KindDate, "2026-02-30"},     // no such day
		{KindDate, ".."},             // range without ends
		{KindDate, "2026-01-01|"},    // empty alternative
		{KindInt, "ten"},             // not a number
		{KindBool, "maybe"},          // not Yes/No
		{KindDate, "<>"},             // missing value
		{KindDate, "x'; DROP TABLE"}, // never reaches SQL text
	} {
		if err := Validate(tt.kind, tt.expr); err == nil {
			t.Errorf("%s %q: no error", tt.kind, tt.expr)
		}
	}
}

func TestComparisonsAndDecimals(t *testing.T) {
	for _, tc := range []struct {
		expr, clause string
		args         []interface{}
	}{
		{">10000", "x > ?", []interface{}{10000.0}},
		{">=1500,50", "x >= ?", []interface{}{1500.5}},
		{"<0", "x < ?", []interface{}{0.0}},
		{"<=-2.5", "x <= ?", []interface{}{-2.5}},
		{"0|>100", "(x = ? OR x > ?)", []interface{}{0.0, 100.0}},
		{"100..200", "x BETWEEN ? AND ?", []interface{}{100.0, 200.0}},
	} {
		clause, args, err := Clause("x", KindDecimal, tc.expr)
		if err != nil || clause != tc.clause || !reflect.DeepEqual(args, tc.args) {
			t.Errorf("%q: %q %v %v, want %q %v", tc.expr, clause, args, err, tc.clause, tc.args)
		}
	}
	for _, bad := range []string{"abc", ">", "1.000,50", "1e5", "NaN"} {
		if err := Validate(KindDecimal, bad); err == nil {
			t.Errorf("%q accepted as a decimal filter", bad)
		}
	}
	if clause, _, err := Clause("d", KindDate, ">2026-01-31"); err != nil || clause != "d > ?" {
		t.Errorf("date comparison: %q, %v", clause, err)
	}
}

func TestClauseForRepeatsColumnArgs(t *testing.T) {
	clause, args, err := ClauseFor("(SELECT SUM(a) FROM s WHERE k = ?)", []interface{}{"K"}, KindDecimal, "0|>100")
	if err != nil {
		t.Fatal(err)
	}
	want := "((SELECT SUM(a) FROM s WHERE k = ?) = ? OR (SELECT SUM(a) FROM s WHERE k = ?) > ?)"
	if clause != want || !reflect.DeepEqual(args, []interface{}{"K", 0.0, "K", 100.0}) {
		t.Errorf("got %q %v", clause, args)
	}
}

func TestUpperLimit(t *testing.T) {
	cases := []struct {
		kind Kind
		expr string
		want string
	}{
		{KindDate, "2026-01-01..2026-03-31", "..2026-03-31"},
		{KindDate, "2026-03-31", "..2026-03-31"},
		{KindDate, "..2026-03-31", "..2026-03-31"},
		{KindDate, "<=2026-03-31", "..2026-03-31"},
		{KindDate, "<2026-03-31", "<2026-03-31"},
		{KindDate, "2026-01-01..2026-01-31|2026-03-01..2026-03-31", "..2026-03-31"},
		{KindDate, "2026-03-31|<2026-03-31", "..2026-03-31"},
		{KindDate, "2026-01-01..", ""},
		{KindDate, ">2026-01-01", ""},
		{KindDate, "<>2026-01-01", ""},
		{KindDate, "", ""},
		{KindDate, "not a date", ""},
		{KindInt, "5..10|200", "..200"},
		{KindDecimal, "1,5..2,5", "..2,5"},
		{KindCode, "A*", ""},
	}
	for _, c := range cases {
		if got := UpperLimit(c.kind, c.expr); got != c.want {
			t.Errorf("UpperLimit(%s, %q) = %q, want %q", c.kind, c.expr, got, c.want)
		}
	}
}

// A closing date filters as the database keeps it: after its date, before the next day
func TestClosingDateFilter(t *testing.T) {
	clause, args, err := Clause("d", KindDate, "2026-01-01..C2026-12-31|C2025-12-31")
	if err != nil || clause != "(d BETWEEN ? AND ? OR d = ?)" || len(args) != 3 ||
		args[0] != "2026-01-01" || args[1] != "2026-12-31 23:59:59" || args[2] != "2025-12-31 23:59:59" {
		t.Errorf("clause %q args %v err %v", clause, args, err)
	}
	if UpperLimit(KindDate, "2026-01-01..C2026-12-31") != "..C2026-12-31" {
		t.Errorf("upper limit %q", UpperLimit(KindDate, "2026-01-01..C2026-12-31"))
	}
	if Validate(KindDate, "C") == nil || Validate(KindDate, "C2026-13-01") == nil {
		t.Error("an invalid closing date accepted")
	}
}
