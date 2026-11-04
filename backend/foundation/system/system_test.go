package system

import (
	"testing"
	"time"

	"github.com/hansjlachmann/goeasyerp/backend/foundation/codeunits"
	"github.com/hansjlachmann/goeasyerp/backend/foundation/tables"
	"github.com/hansjlachmann/goeasyerp/backend/foundation/types"
)

func date(s string) types.Date { return types.MustDate(s) }

func TestCalcDate(t *testing.T) {
	sat := date("2026-01-31") // a Saturday
	cases := []struct {
		formula string
		from    types.Date
		want    string
	}{
		{"<1M>", sat, "2026-02-28"}, // February keeps its last day
		{"<-1D>", sat, "2026-01-30"},
		{"<2W>", sat, "2026-02-14"},
		{"<1Q>", sat, "2026-04-30"},
		{"<1Y>", date("2024-02-29"), "2025-02-28"},
		{"<CD>", sat, "2026-01-31"},
		{"<CM>", date("2026-01-15"), "2026-01-31"},
		{"<-CM>", sat, "2026-01-01"},
		{"<1M+CM>", sat, "2026-02-28"},
		{"<CM+1D>", sat, "2026-02-01"},
		{"<CQ>", sat, "2026-03-31"},
		{"<-CQ>", date("2026-05-20"), "2026-04-01"},
		{"<CY>", sat, "2026-12-31"},
		{"<-CY>", sat, "2026-01-01"},
		{"<CW>", sat, "2026-02-01"},  // Sunday of the ISO week
		{"<-CW>", sat, "2026-01-26"}, // Monday
		{"<WD1>", sat, "2026-02-02"}, // next Monday
		{"<WD6>", sat, "2026-01-31"}, // already a Saturday
		{"1m+cm", sat, "2026-02-28"}, // lower case, without < >
		{"<-1Y-CY>", sat, "2025-01-01"},
	}
	for _, c := range cases {
		got, err := CalcDate(c.formula, c.from)
		if err != nil || got.String() != c.want {
			t.Errorf("CalcDate(%q, %s) = %s, %v; want %s", c.formula, c.from, got, err, c.want)
		}
	}
	for _, bad := range []string{"", "<>", "<1X>", "<C>", "<M>", "<WD8>", "<-WD1>", "<1M+>"} {
		if _, err := CalcDate(bad, sat); err == nil {
			t.Errorf("CalcDate(%q) accepted", bad)
		}
	}
	if got, err := CalcDate("<1M>", types.Date{}); err != nil || !got.IsZero() {
		t.Errorf("blank date: %v, %v", got, err)
	}
	if got, _ := CalcDate("<0D>"); !got.Equal(Today()) {
		t.Errorf("default date is the work date (today): %v", got)
	}
}

func TestDMY2DateAndDate2DMY(t *testing.T) {
	d, err := DMY2Date(29, 2, 2024)
	if err != nil || d.String() != "2024-02-29" {
		t.Errorf("DMY2Date(29, 2, 2024) = %v, %v", d, err)
	}
	for _, bad := range [][3]int{{31, 2, 2026}, {0, 1, 2026}, {1, 13, 2026}} {
		if _, err := DMY2Date(bad[0], bad[1], bad[2]); err == nil {
			t.Errorf("DMY2Date%v accepted", bad)
		}
	}
	if d, _ := DMY2Date(1); d.Month() != time.Now().Month() || d.Year() != time.Now().Year() {
		t.Errorf("DMY2Date(1) = %v, want this month", d)
	}
	x := date("2026-10-20")
	if Date2DMY(x, 1) != 20 || Date2DMY(x, 2) != 10 || Date2DMY(x, 3) != 2026 || Date2DMY(x, 4) != 0 {
		t.Error("Date2DMY")
	}
}

type status int

func (s status) String() string { return []string{" ", "Open", "Blocked"}[s] }

func TestFormatAndEvaluate(t *testing.T) {
	cases := []struct {
		value any
		want  string
	}{
		{nil, ""},
		{"x", "x"},
		{types.Code("C1"), "C1"},
		{date("2026-10-20"), "2026-10-20"},
		{types.Date{}, ""},
		{types.MustDecimal("1234.50"), "1234.5"},
		{true, "true"},
		{42, "42"},
		{int64(-7), "-7"},
		{1.5, "1.5"},
		{status(1), "Open"},
	}
	for _, c := range cases {
		if got := Format(c.value); got != c.want {
			t.Errorf("Format(%#v) = %q, want %q", c.value, got, c.want)
		}
	}
	if got := Format("ab", 4); got != "ab  " {
		t.Errorf("left-aligned %q", got)
	}
	if got := Format(42, -5); got != "   42" {
		t.Errorf("right-aligned %q", got)
	}
	if got := Format("abcdef", 3); got != "abc" {
		t.Errorf("cut %q", got)
	}

	var d types.Date
	if !Evaluate(&d, "2026-10-20") || d.String() != "2026-10-20" {
		t.Errorf("date %v", d)
	}
	if Evaluate(&d, "20.10.26") || d.String() != "2026-10-20" {
		t.Error("an invalid date must return false and keep the value")
	}
	var dec types.Decimal
	if !Evaluate(&dec, "1234.5") || dec.String() != "1234.5" || Evaluate(&dec, "1.234,5") {
		t.Errorf("decimal %v", dec)
	}
	var code types.Code
	if !Evaluate(&code, " c-01 ") || code != "C-01" {
		t.Errorf("code %q", code)
	}
	var n int
	if !Evaluate(&n, "42") || n != 42 || Evaluate(&n, "x") {
		t.Errorf("int %d", n)
	}
	var s status
	if !Evaluate(&s, "2") || s != 2 {
		t.Errorf("option %d", s)
	}
	var b bool
	if !Evaluate(&b, "true") || !b || Evaluate(&b, "Yes") {
		t.Error("bool")
	}
	if Evaluate(nil, "1") {
		t.Error("nil target accepted")
	}
}

func TestTextFunctions(t *testing.T) {
	if got := StrSubstNo("%1 of %2", 3, "C1"); got != "3 of C1" {
		t.Errorf("StrSubstNo %q", got)
	}
	if got := StrSubstNo("No. #1#### is %2.", "C1"); got != "No. C1 is ." {
		t.Errorf("StrSubstNo # and a missing value %q", got)
	}
	if got := StrSubstNo("100% sure, %1", date("2026-10-20")); got != "100% sure, 2026-10-20" {
		t.Errorf("StrSubstNo %% %q", got)
	}
	if CopyStr("Ærlig talt", 1, 5) != "Ærlig" || CopyStr("abc", 2) != "bc" || CopyStr("abc", 9) != "" || CopyStr("abc", 2, 0) != "" {
		t.Error("CopyStr")
	}
	if StrLen("Ærø") != 3 || StrPos("aéb", "b") != 3 || StrPos("abc", "x") != 0 || StrPos("abc", "") != 0 {
		t.Error("StrLen/StrPos")
	}
	if PadStr("ab", 4) != "ab  " || PadStr("ab", 4, "*") != "ab**" || PadStr("abcdef", 3) != "abc" || PadStr("7", -3, "0") != "007" {
		t.Error("PadStr")
	}
	if DelChr("  a b  ") != "ab" || DelChr("  a b  ", "<>") != "a b" || DelChr("--a-b", "<", "-") != "a-b" || DelChr("C1, C2", "=", ", ") != "C1C2" {
		t.Error("DelChr")
	}
	if SelectStr(2, "a,b,c") != "b" || SelectStr(4, "a,b,c") != "" || SelectStr(0, "a") != "" {
		t.Error("SelectStr")
	}
	for in, want := range map[string]string{"INV0099": "INV0100", "A9": "A10", "X9Y": "X10Y", "ABC": "", "": "", "0": "1"} {
		if got := IncStr(in); got != want {
			t.Errorf("IncStr(%q) = %q, want %q", in, got, want)
		}
	}
	if UpperCase("abc") != "ABC" || LowerCase("ÆØÅ") != "æøå" {
		t.Error("UpperCase/LowerCase")
	}
}

func TestNumberFunctions(t *testing.T) {
	dec := types.MustDecimal
	cases := []struct {
		got  types.Decimal
		want string
	}{
		{Round(dec("1.235")), "1.24"},
		{Round(dec("-1.235")), "-1.24"}, // half away from zero
		{Round(dec("1.231"), 0.01, ">"), "1.24"},
		{Round(dec("1.239"), 0.01, "<"), "1.23"},
		{Round(dec("-1.231"), 0.01, "<"), "-1.24"},
		{Round(dec("12.5"), 1), "13"},
		{Round(dec("1250"), 100), "1300"},
		{Round(dec("1234.567"), dec("0.05")), "1234.55"},
		{Abs(dec("-3.5")), "3.5"},
		{Power(dec("2"), dec("10")), "1024"},
		{Power(dec("4"), dec("0.5")), "2"},
	}
	for i, c := range cases {
		if c.got.String() != c.want {
			t.Errorf("case %d: %s, want %s", i, c.got, c.want)
		}
	}
}

func TestDialog(t *testing.T) {
	if err := Error("Customer %1 is blocked", "C1"); err == nil || err.Error() != "Customer C1 is blocked" {
		t.Errorf("Error: %v", err)
	}
	if r := Message("%1 records", 3); r.Dialog == nil || r.Dialog.Message != "3 records" || !r.Success {
		t.Errorf("Message: %+v", r)
	}
}

func TestMoreTextFunctions(t *testing.T) {
	if ConvertStr("a-b-c", "-", "_") != "a_b_c" || ConvertStr("abc", "ab", "x") != "abc" || ConvertStr("ÆØ", "ÆØ", "AO") != "AO" {
		t.Error("ConvertStr")
	}
	if InsStr("ac", "b", 2) != "abc" || InsStr("ab", "c", 9) != "abc" || InsStr("bc", "a", 0) != "abc" {
		t.Error("InsStr")
	}
	if DelStr("abcdef", 2, 3) != "aef" || DelStr("abcdef", 4) != "abc" || DelStr("abc", 9, 1) != "abc" {
		t.Error("DelStr")
	}
	// EAN-13 check digit: weights 1313…, check digit of 400638133393 is 1
	if n, err := StrCheckSum("400638133393", "131313131313"); err != nil || n != 1 {
		t.Errorf("StrCheckSum EAN = %d, %v", n, err)
	}
	if n, err := StrCheckSum("1234", ""); err != nil || n != 0 { // 10 % 10 = 0
		t.Errorf("StrCheckSum = %d, %v", n, err)
	}
	if _, err := StrCheckSum("12a", ""); err == nil {
		t.Error("StrCheckSum accepted a letter")
	}
}

func TestMoreDateFunctions(t *testing.T) {
	d := date("2026-10-20") // Tuesday, ISO week 43
	if Date2DWY(d, 1) != 2 || Date2DWY(d, 2) != 43 || Date2DWY(d, 3) != 2026 || Date2DWY(d, 4) != 0 {
		t.Error("Date2DWY")
	}
	if Date2DWY(date("2027-01-01"), 2) != 53 || Date2DWY(date("2027-01-01"), 3) != 2026 {
		t.Error("Date2DWY at the year end: 1 January 2027 is in week 53 of 2026")
	}
	if got, err := DWY2Date(2, 43, 2026); err != nil || got.String() != "2026-10-20" {
		t.Errorf("DWY2Date = %v, %v", got, err)
	}
	if got, err := DWY2Date(1, 1, 2026); err != nil || got.String() != "2025-12-29" {
		t.Errorf("DWY2Date week 1 = %v, %v", got, err)
	}
	if _, err := DWY2Date(1, 53, 2025); err == nil {
		t.Error("2025 has no week 53")
	}
	if _, err := DWY2Date(8, 1, 2026); err == nil {
		t.Error("weekday 8 accepted")
	}
	at := time.Date(0, 1, 1, 14, 30, 0, 0, time.Local)
	dt := CreateDateTime(d, at)
	if DT2Date(dt).String() != "2026-10-20" || DT2Time(dt).Hour() != 14 || DT2Time(dt).Minute() != 30 {
		t.Errorf("CreateDateTime/DT2Date/DT2Time: %v", dt)
	}
	if !CreateDateTime(types.Date{}, at).IsZero() || !DT2Date(types.DateTime{}).IsZero() {
		t.Error("blank date-time")
	}
	if Time().Year() != 0 {
		t.Error("Time is a time of day")
	}
}

func TestMoreSystemFunctions(t *testing.T) {
	for range 100 {
		if n := Random(6); n < 1 || n > 6 {
			t.Fatalf("Random(6) = %d", n)
		}
	}
	if Random(0) != 1 {
		t.Error("Random(0)")
	}
	g := CreateGuid()
	if !guidPattern.MatchString(g) || g[14] != '4' || g == CreateGuid() || IsNullGuid(g) {
		t.Errorf("CreateGuid %q", g)
	}
	if !IsNullGuid("") || !IsNullGuid("00000000-0000-0000-0000-000000000000") || !IsNullGuid("{00000000-0000-0000-0000-000000000000}") || !IsNullGuid("x") {
		t.Error("IsNullGuid")
	}
	if Confirm("Delete %1?", "C1") {
		t.Error("Confirm without a dialog must be false")
	}
	_ = UserId() // no session context: ""
}

// StrMenu asks through the codeunit's dialog and returns the chosen option's number
func TestStrMenu(t *testing.T) {
	if StrMenu("Ship,Invoice", 1) != 0 {
		t.Error("StrMenu without a dialog must be 0")
	}
	d := codeunits.OpenDialogWithID("", "strmenu-test")
	codeunits.SetCurrentDialog(d)
	defer codeunits.ClearCurrentDialog()

	answer := func(value string) {
		go func() {
			ev := <-d.Events()
			fields, _ := ev.Data["fields"].([]codeunits.InputField)
			if ev.EventType != "request_input" || ev.Message != "Post?" || len(fields) != 1 ||
				fields[0].Type != "option" || len(fields[0].Options) != 3 || fields[0].Default != "3" {
				t.Errorf("event %+v", ev)
			}
			for !d.RespondToInput(ev.ConfirmID, map[string]string{"option": value}) {
				time.Sleep(time.Millisecond)
			}
		}()
	}
	answer("2")
	if n := StrMenu("Ship,Invoice,Ship and Invoice", 3, "Post?"); n != 2 {
		t.Errorf("chosen 2: %d", n)
	}
	answer("") // cancelled: the dialog sends no values
	if n := StrMenu("Ship,Invoice,Ship and Invoice", 3, "Post?"); n != 0 {
		t.Errorf("cancelled: %d", n)
	}
}

func TestCompressArray(t *testing.T) {
	lines := []types.Text{"", "Main Street 1", "  ", "", "0150 Oslo"}
	if n := CompressArray(lines); n != 2 || lines[0] != "Main Street 1" || lines[1] != "0150 Oslo" ||
		lines[2] != "" || lines[3] != "" || lines[4] != "" {
		t.Errorf("CompressArray = %d, %q", n, lines)
	}
	if CompressArray([]string{}) != 0 {
		t.Error("empty array")
	}
}

// Hyperlink and the progress window go to the codeunit's dialog
func TestHyperlinkAndWindow(t *testing.T) {
	for _, bad := range []string{"", "javascript:alert(1)", "file:///etc/passwd", "www.example.com", "https://", "mailto:"} {
		if Hyperlink(bad) == nil {
			t.Errorf("Hyperlink(%q) must be an error", bad)
		}
	}
	if err := Hyperlink("https://example.com/a?b=1"); err != nil { // no dialog: nothing happens
		t.Errorf("without a dialog: %v", err)
	}

	d := codeunits.OpenDialogWithID("", "window-test")
	codeunits.SetCurrentDialog(d)
	defer codeunits.ClearCurrentDialog()

	if err := Hyperlink(" mailto:info@example.com "); err != nil {
		t.Fatal(err)
	}
	if ev := <-d.Events(); ev.EventType != "hyperlink" || ev.Message != "mailto:info@example.com" {
		t.Errorf("hyperlink event %+v", ev)
	}

	w := OpenWindow("Posting customers\nCustomer #1########## of #3###\n@2@@@@@@@@@@@@", nil, nil, 20)
	if ev := <-d.Events(); ev.Message != "Posting customers\nCustomer  of 20" || ev.Value != 0 {
		t.Errorf("open: %+v", ev)
	}
	w.Update(1, types.NewCode("C10"))
	<-d.Events()
	w.Update(2, 2500)
	if ev := <-d.Events(); ev.Message != "Posting customers\nCustomer C10 of 20" || ev.Value != 25 {
		t.Errorf("update: %+v", ev)
	}
	w.Update(2, 20000) // capped at 10000
	if ev := <-d.Events(); ev.Value != 100 {
		t.Errorf("capped: %+v", ev)
	}
	w.Close()
	w.Update(1, "C11")
	select {
	case ev := <-d.Events():
		t.Errorf("update after Close: %+v", ev)
	default:
	}
}

// A closing date lies between a date and the next day
func TestClosingDate(t *testing.T) {
	d := date("2026-12-31")
	c := ClosingDate(d)
	if !c.IsClosing() || d.IsClosing() || c.Equal(d) || !c.After(d) || !c.Before(date("2027-01-01")) {
		t.Errorf("closing date %v of %v", c, d)
	}
	if !NormalDate(c).Equal(d) || !NormalDate(d).Equal(d) || !ClosingDate(types.Date{}).IsZero() {
		t.Error("NormalDate")
	}
	if Format(c) != "C2026-12-31" || Format(d) != "2026-12-31" {
		t.Errorf("Format %q", Format(c))
	}
	var e types.Date
	if !Evaluate(&e, "c2026-12-31") || !e.Equal(c) || Evaluate(&e, "C") {
		t.Errorf("Evaluate %v", e)
	}
	if Date2DMY(c, 1) != 31 || Date2DMY(c, 3) != 2026 {
		t.Error("Date2DMY of a closing date")
	}
	// stored as the date at 23:59:59: after the date, before the next day, read back closing
	if v, err := c.Value(); err != nil || v != "2026-12-31 23:59:59" {
		t.Errorf("closing date stored as %v (%v)", v, err)
	}
	var back types.Date
	for _, stored := range []interface{}{"2026-12-31 23:59:59", "2026-12-31T23:59:59Z", time.Date(2026, 12, 31, 23, 59, 59, 0, time.UTC)} {
		if err := back.Scan(stored); err != nil || !back.Equal(c) {
			t.Errorf("read back %v as %v (%v)", stored, back, err)
		}
	}
	for _, stored := range []interface{}{"2026-12-31", "2026-12-31T00:00:00Z", time.Date(2026, 12, 31, 0, 0, 0, 0, time.UTC)} {
		if err := back.Scan(stored); err != nil || !back.Equal(d) {
			t.Errorf("read back %v as %v (%v)", stored, back, err)
		}
	}
	if types.FilterArg("C2026-12-31") != "2026-12-31 23:59:59" || types.FilterArg("2026-12-31") != "2026-12-31" || types.FilterArg("x") != "x" {
		t.Error("FilterArg")
	}
}

// pageRecord is a record RunModal opens a page with
type pageRecord struct {
	key  string
	read string
}

func (r *pageRecord) FilterExpressions() []tables.FieldFilter {
	return []tables.FieldFilter{{Field: "city", Expression: "Oslo"}}
}
func (r *pageRecord) KeyText() string { return r.key }
func (r *pageRecord) GetFields() []tables.FieldInfo {
	return []tables.FieldInfo{{Name: "no", PrimaryKey: true}, {Name: "name"}}
}
func (r *pageRecord) Get(primaryKey interface{}) bool {
	if primaryKey == "C99" {
		return false
	}
	r.read, r.key = primaryKey.(string), primaryKey.(string)
	return true
}

// RunModal opens a page through the codeunit's dialog; in lookup mode the picked record is read
func TestRunModal(t *testing.T) {
	rec := &pageRecord{key: "C01"}
	if RunModal(22, rec, true) != ActionLookupCancel || RunModal(22, rec, false) != ActionCancel {
		t.Error("without a dialog: cancel")
	}
	d := codeunits.OpenDialogWithID("", "runmodal-test")
	codeunits.SetCurrentDialog(d)
	defer codeunits.ClearCurrentDialog()

	answer := func(values map[string]string) {
		go func() {
			ev := <-d.Events()
			filters, _ := ev.Data["filters"].([]tables.FieldFilter)
			if ev.EventType != "run_page" || ev.Data["page_id"] != 22 || ev.Data["select"] != "C01" ||
				len(filters) != 1 || filters[0].Expression != "Oslo" {
				t.Errorf("event %+v", ev)
			}
			for !d.RespondToInput(ev.ConfirmID, values) {
				time.Sleep(time.Millisecond)
			}
		}()
	}
	answer(map[string]string{"action": "LookupOK", "key": "C02"})
	if RunModal(22, rec, true) != ActionLookupOK || rec.read != "C02" {
		t.Errorf("picked C02: read %q", rec.read)
	}
	rec.key = "C01"
	answer(map[string]string{"action": "LookupOK", "key": "C99"}) // deleted meanwhile
	if RunModal(22, rec, true) != ActionLookupCancel {
		t.Error("a record that cannot be read")
	}
	answer(map[string]string{}) // Esc
	if RunModal(22, rec, true) != ActionLookupCancel {
		t.Error("Esc")
	}
	answer(map[string]string{"action": "OK"})
	if RunModal(22, rec, false) != ActionOK {
		t.Error("closed, not in lookup mode")
	}

	// A page variable: SetTableView, SetRecord, LookupMode, RunModal, GetRecord
	page := NewPage(22)
	rec.key = "C05"
	page.SetTableView(rec)
	page.SetRecord(rec)
	page.LookupMode(true)
	go func() {
		ev := <-d.Events()
		if ev.Data["select"] != "C05" || ev.Data["lookup_mode"] != true {
			t.Errorf("page variable event %+v", ev)
		}
		for !d.RespondToInput(ev.ConfirmID, map[string]string{"action": "LookupOK", "key": "C07"}) {
			time.Sleep(time.Millisecond)
		}
	}()
	if page.RunModal() != ActionLookupOK || rec.read == "C07" {
		t.Error("RunModal must not change the record before GetRecord")
	}
	if !page.GetRecord(rec) || rec.read != "C07" {
		t.Errorf("GetRecord: %q", rec.read)
	}
	go func() { // Cancel, the close button or Esc
		ev := <-d.Events()
		for !d.RespondToInput(ev.ConfirmID, map[string]string{}) {
			time.Sleep(time.Millisecond)
		}
	}()
	if page.RunModal() != ActionLookupCancel || page.GetRecord(rec) {
		t.Error("LookupCancel: GetRecord has nothing")
	}
}
