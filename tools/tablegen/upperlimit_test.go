package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// A FlowField filter with upper_limit applies only the upper end of the FlowFilter
func TestUpperLimitFlowFilter(t *testing.T) {
	entry := &TableDef{}
	entry.Table.ID, entry.Table.Name = 17, "GL Entry"
	entry.Table.Fields = []Field{
		{No: 1, Name: "entry_no", Type: "int", DBName: "entry_no", PrimaryKey: true},
		{No: 3, Name: "account_no", Type: "types.Code", DBName: "account_no", Length: 20},
		{No: 4, Name: "posting_date", Type: "types.Date", DBName: "posting_date"},
		{No: 17, Name: "amount", Type: "types.Decimal", DBName: "amount"},
	}
	account := &TableDef{}
	account.Table.ID, account.Table.Name = 15, "GL Account"
	account.Table.Fields = []Field{
		{No: 1, Name: "no", Type: "types.Code", DBName: "no", PrimaryKey: true, Length: 20},
		{No: 28, Name: "date_filter", Type: "types.Date", FlowFilter: true},
		{No: 31, Name: "balance_at_date", Type: "types.Decimal", FlowField: true, CalcFormula: "Sum",
			SourceTable: "GLEntry", SourceField: "amount", FlowFilters: []FlowFilter{
				{Field: "account_no", Type: "field", Value: "no"},
				{Field: "posting_date", Type: "filter", Value: "date_filter", UpperLimit: true},
			}},
	}
	byStruct := map[string]*TableDef{"GLEntry": entry, "GLAccount": account}
	splitFlowFilterFields(account)
	if err := validateFlowFilters(account, byStruct); err != nil {
		t.Fatal(err)
	}
	resolveFlowFieldSIFT(account, byStruct)

	out := filepath.Join(t.TempDir(), "glaccount_gen.go")
	if err := generateFile(out, "gen", boilerplateTemplate, prepareTemplateData(account)); err != nil {
		t.Fatal(err) // generateFile formats the code: a template mistake fails here
	}
	code, _ := os.ReadFile(out)
	// foundation/tables applies the upper end of the Date Filter from the description
	if !strings.Contains(string(code), `Value: "date_filter", Kind: flowfilter.KindDate, UpperLimit: true}`) {
		t.Error("the FlowField's Date Filter is not described with its upper limit")
	}

	bad := account.Table.Fields[1].FlowFilters
	bad[0].UpperLimit = true
	if err := validateFlowFilters(account, byStruct); err == nil {
		t.Error("upper_limit on a flow filter of type field accepted")
	}
}
