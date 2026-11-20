package tables

import (
	"errors"
	"testing"

	"github.com/hansjlachmann/goeasyerp/backend/foundation/types"
)

// testStatus is an option type as tablegen generates them
type testStatus int

// testRec is a record with one field of each kind, as a generated record has them
type testRec struct {
	Code    types.Code
	Name    types.Text
	Amount  types.Decimal
	Date    types.Date
	Picture []byte
	Blocked bool
	Status  testStatus
	Count   int
	Formula types.Code
	Balance types.Decimal // a FlowField
}

var testFields = &TableFields{
	Table: "Test",
	Fields: []FieldMeta{
		{Info: FieldInfo{No: 1, Name: "code", Type: FieldTypeCode, PrimaryKey: true}, Kind: KindCode, Trigger: "OnValidate_Code"},
		{Info: FieldInfo{No: 2, Name: "name", Type: FieldTypeText}, Kind: KindText, Trigger: "OnValidate_Name"},
		{Info: FieldInfo{No: 3, Name: "amount", Type: FieldTypeDecimal}, Kind: KindDecimal, Trigger: "OnValidate_Amount"},
		{Info: FieldInfo{No: 4, Name: "date", Type: FieldTypeDate}, Kind: KindDate, Trigger: "OnValidate_Date"},
		{Info: FieldInfo{No: 5, Name: "picture", Type: FieldTypeBlob}, Kind: KindBlob, Trigger: "OnValidate_Picture"},
		{Info: FieldInfo{No: 6, Name: "blocked", Type: FieldTypeBoolean}, Kind: KindBool, Trigger: "OnValidate_Blocked"},
		{Info: FieldInfo{No: 7, Name: "status", Type: FieldTypeOption}, Kind: KindOption, Trigger: "OnValidate_Status", Options: []string{" ", "Open", "Closed"}},
		{Info: FieldInfo{No: 8, Name: "count", Type: FieldTypeInteger}, Kind: KindInt, Trigger: "OnValidate_Count"},
		{Info: FieldInfo{No: 9, Name: "formula", Type: FieldTypeCode}, Kind: KindCode, Trigger: "OnValidate_Formula", DateFormula: true},
		{Info: FieldInfo{No: 10, Name: "balance", Type: FieldTypeDecimal, FlowField: true}, Kind: KindDecimal, Trigger: "OnValidate_Balance"},
	},
	FlowFilters: []FieldInfo{{No: 20, Name: "date_filter"}},
}

func (r *testRec) ref(i int) any {
	switch i {
	case 0:
		return &r.Code
	case 1:
		return &r.Name
	case 2:
		return &r.Amount
	case 3:
		return &r.Date
	case 4:
		return &r.Picture
	case 5:
		return &r.Blocked
	case 6:
		return &r.Status
	case 7:
		return &r.Count
	case 8:
		return &r.Formula
	case 9:
		return &r.Balance
	}
	return nil
}

func TestFieldNumbersAndNames(t *testing.T) {
	if n := testFields.FieldNo("Amount"); n != 3 {
		t.Errorf("FieldNo(Amount) = %d", n)
	}
	if n := testFields.FieldNo("DATE_FILTER"); n != 20 {
		t.Errorf("FieldNo(DATE_FILTER) = %d", n)
	}
	if n := testFields.FieldNo("nothing"); n != 0 {
		t.Errorf("FieldNo(nothing) = %d", n)
	}
	if name := testFields.FieldName(20); name != "date_filter" {
		t.Errorf("FieldName(20) = %q", name)
	}
	if name := testFields.FieldName(99); name != "" {
		t.Errorf("FieldName(99) = %q", name)
	}
	if c, ok := testFields.ColumnName("Code"); !ok || c != "code" {
		t.Errorf("ColumnName(Code) = %q, %v", c, ok)
	}
	if _, ok := testFields.ColumnName("balance"); ok {
		t.Error("a FlowField is not a column")
	}
}

func TestValidateSetConverts(t *testing.T) {
	r := &testRec{}
	set := func(field string, v any) {
		t.Helper()
		if _, err := testFields.ValidateSet(r.ref, field, v); err != nil {
			t.Fatalf("ValidateSet(%s, %v): %v", field, v, err)
		}
	}
	set("CODE", "c1")
	set("amount", 12)
	set("date", "2026-11-20")
	set("blocked", "1")
	set("status", "Closed")
	set("count", "7")
	set("picture", "aGk=")
	if r.Code != "C1" || r.Amount.String() != "12" || r.Date.String() != "2026-11-20" || !r.Blocked ||
		r.Status != 2 || r.Count != 7 || string(r.Picture) != "hi" {
		t.Fatalf("record = %+v", r)
	}
	set("status", testStatus(1))
	set("status", float64(0))
	if r.Status != 0 {
		t.Errorf("status = %d", r.Status)
	}
	for _, c := range []struct {
		field string
		value any
	}{
		{"status", 3},
		{"status", "Unknown"},
		{"status", true},
		{"count", "x"},
		{"amount", "x"},
		{"code", 1},
		{"balance", 1},
		{"nothing", 1},
		{"formula", "1X+"},
	} {
		if _, err := testFields.ValidateSet(r.ref, c.field, c.value); err == nil {
			t.Errorf("ValidateSet(%s, %v): no error", c.field, c.value)
		}
	}
}

func TestSetFieldValueAndTransfer(t *testing.T) {
	r := &testRec{}
	if !testFields.SetFieldValue(r.ref, 7, 2) || r.Status != 2 {
		t.Error("an option takes an int")
	}
	if testFields.SetFieldValue(r.ref, 7, 3) {
		t.Error("an option takes no value past its options")
	}
	if !testFields.SetFieldValue(r.ref, 1, "x") || r.Code != "X" {
		t.Error("a code takes a string")
	}
	if testFields.SetFieldValue(r.ref, 3, "5") {
		t.Error("a decimal takes only a decimal")
	}
	if testFields.SetFieldValue(r.ref, 10, types.NewDecimalFromInt(1)) {
		t.Error("a FlowField is not set")
	}
	if v, ok := testFields.FieldValue(r.ref, 7); !ok || v != testStatus(2) {
		t.Errorf("FieldValue(7) = %v (%T)", v, v)
	}
	if _, ok := testFields.FieldValue(r.ref, 10); ok {
		t.Error("FieldValue of a FlowField")
	}
	if _, ok := testFields.FlowFieldValue(r.ref, 10); !ok {
		t.Error("FlowFieldValue of a FlowField")
	}
	if v, ok := testFields.ColumnValue(r.ref, "status"); !ok || v != 2 {
		t.Errorf("ColumnValue(status) = %v (%T)", v, v)
	}
	if _, ok := testFields.ColumnValue(r.ref, "Status"); ok {
		t.Error("ColumnValue takes the exact column name")
	}
}

func TestOldValuesAndChanged(t *testing.T) {
	r := &testRec{Code: "A", Status: 1, Count: 3, Picture: []byte("x")}
	if !testFields.Changed(r.ref, nil, "code") {
		t.Error("without old values a field has changed")
	}
	old := testFields.OldValues(r.ref)
	if _, ok := old["balance"]; ok {
		t.Error("FlowFields have no old value")
	}
	for _, f := range []string{"code", "status", "count", "amount"} {
		if testFields.Changed(r.ref, old, f) {
			t.Errorf("%s changed", f)
		}
	}
	if !testFields.Changed(r.ref, old, "picture") {
		t.Error("a BLOB always counts as changed")
	}
	r.Status, r.Count, r.Code = 2, 4, "B"
	for _, f := range []string{"code", "status", "count"} {
		if !testFields.Changed(r.ref, old, f) {
			t.Errorf("%s not changed", f)
		}
	}
	x := &testRec{}
	testFields.SetOldValues(x.ref, old)
	if x.Code != "A" || x.Status != 1 || x.Count != 3 {
		t.Errorf("xRec = %+v", x)
	}
}

func TestToMapFromMap(t *testing.T) {
	r := &testRec{Code: "A", Status: 2, Count: 3, Blocked: true, Balance: types.NewDecimalFromInt(5)}
	m := testFields.ToMap(r.ref)
	if m["code"] != "A" || m["status"] != 2 || m["count"] != 3 || m["blocked"] != true || m["balance"] != "5" {
		t.Errorf("ToMap = %v", m)
	}
	x := &testRec{}
	testFields.FromMap(x.ref, map[string]any{"code": "b", "status": "Open", "count": "9", "amount": 1.5, "balance": "7", "picture": "aGk="})
	if x.Code != "B" || x.Status != 1 || x.Count != 9 || x.Amount.String() != "1.5" || !x.Balance.IsZero() || x.Picture != nil {
		t.Errorf("FromMap = %+v", x)
	}
}

type testLogic struct{ calls *[]string }

func (l testLogic) OnValidate_Code(rec *testRec) error {
	*l.calls = append(*l.calls, "code:"+string(rec.Code))
	return nil
}

func (l testLogic) OnValidate_Name(rec *testRec) error { return errors.New("refused") }

type wrongLogic struct{}

func (wrongLogic) OnValidate_Code(rec *testRec, extra int) error { return nil }

func TestOnValidateTriggers(t *testing.T) {
	var calls []string
	logic := testLogic{&calls}
	if known, ok := testFields.IsOnValidateTrigger("OnValidate_Code", logic, (*testRec)(nil)); !known || !ok {
		t.Errorf("OnValidate_Code: known %v, ok %v", known, ok)
	}
	if known, _ := testFields.IsOnValidateTrigger("OnValidate_Balance", logic, (*testRec)(nil)); known {
		t.Error("a FlowField has no OnValidate trigger")
	}
	if known, ok := testFields.IsOnValidateTrigger("OnValidate_Code", wrongLogic{}, (*testRec)(nil)); !known || ok {
		t.Errorf("wrong signature: known %v, ok %v", known, ok)
	}
	r := &testRec{Code: "A"}
	if err := RunOnValidate([]any{logic, wrongLogic{}, logic}, r, "OnValidate_Code"); err != nil {
		t.Fatal(err)
	}
	if len(calls) != 2 || calls[0] != "code:A" {
		t.Errorf("calls = %v", calls)
	}
	if err := RunOnValidate([]any{logic}, r, "OnValidate_Name"); err == nil || err.Error() != "refused" {
		t.Errorf("err = %v", err)
	}
}
