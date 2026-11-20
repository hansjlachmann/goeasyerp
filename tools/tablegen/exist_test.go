package main

import (
	"os"
	"path/filepath"
	"regexp"
	"testing"
)

// An Exist FlowField (classic CalcFormula=Exist) is true when a record of the source table
// is in its filters: a count above zero, on the card and in the list's batch; only for bool
func TestExistFlowField(t *testing.T) {
	comment := &TableDef{}
	comment.Table.ID, comment.Table.Name = 97, "Comment Line"
	comment.Table.Fields = []Field{
		{No: 1, Name: "table_name", Type: "int", DBName: "table_name", PrimaryKey: true},
		{No: 2, Name: "no", Type: "types.Code", DBName: "no", PrimaryKey: true, Length: 20},
		{No: 3, Name: "line_no", Type: "int", DBName: "line_no", PrimaryKey: true},
	}
	cust := &TableDef{}
	cust.Table.ID, cust.Table.Name = 18, "Cust"
	cust.Table.Fields = []Field{
		{No: 1, Name: "no", Type: "types.Code", DBName: "no", PrimaryKey: true, Length: 20},
		{No: 38, Name: "comment", Type: "bool", FlowField: true, CalcFormula: "Exist", SourceTable: "CommentLine", SourceField: "line_no",
			FlowFilters: []FlowFilter{{Field: "table_name", Type: "const", Value: "1"}, {Field: "no", Type: "field", Value: "no"}}},
	}
	byStruct := map[string]*TableDef{"CommentLine": comment, "Cust": cust}
	splitFlowFilterFields(cust)
	if err := validateFlowFilters(cust, byStruct); err != nil {
		t.Fatal(err)
	}
	resolveFlowFieldSIFT(cust, byStruct)
	out := filepath.Join(t.TempDir(), "cust_gen.go")
	if err := generateFile(out, "gen", boilerplateTemplate, prepareTemplateData(cust)); err != nil {
		t.Fatal(err)
	}
	code, _ := os.ReadFile(out)
	// foundation/tables calculates it from the description: Exist, counting the comment lines
	if !regexp.MustCompile(`Formula:\s+"Exist",`).Match(code) {
		t.Error("the Exist FlowField is not described as Exist")
	}

	cust.Table.Fields[1].Type = "types.Decimal"
	if err := validateFlowFilters(cust, byStruct); err == nil {
		t.Error("an Exist FlowField that is not a bool accepted")
	}
}
