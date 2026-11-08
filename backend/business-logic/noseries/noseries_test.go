package noseries_test

import (
	"database/sql"
	"testing"
	"time"

	_ "github.com/mattn/go-sqlite3"

	"github.com/hansjlachmann/goeasyerp/backend/business-logic/noseries"
	_ "github.com/hansjlachmann/goeasyerp/backend/business-logic/tables" // the No. Series Line triggers
	fcodeunits "github.com/hansjlachmann/goeasyerp/backend/foundation/codeunits"
	"github.com/hansjlachmann/goeasyerp/backend/foundation/database"
	apperrors "github.com/hansjlachmann/goeasyerp/backend/foundation/errors"
	ftables "github.com/hansjlachmann/goeasyerp/backend/foundation/tables"
	"github.com/hansjlachmann/goeasyerp/backend/foundation/types"
	gtables "github.com/hansjlachmann/goeasyerp/backend/generated/tables"
)

const company = "c1"

func newDB(t *testing.T) *sql.DB {
	t.Helper()
	db, err := sql.Open("sqlite3", ":memory:")
	if err != nil {
		t.Fatal(err)
	}
	db.SetMaxOpenConns(1)
	t.Cleanup(func() { _ = db.Close() })
	for _, tbl := range []interface {
		CreateTableWithDBType(database.Executor, string, database.DBType) error
	}{&gtables.NoSeries{}, &gtables.NoSeriesLine{}, &gtables.NoSeriesRelationship{}} {
		if err := tbl.CreateTableWithDBType(db, company, database.DBTypeSQLite); err != nil {
			t.Fatal(err)
		}
	}
	return db
}

func addSeries(t *testing.T, db *sql.DB, code string, defaultNos, manualNos, dateOrder bool) {
	t.Helper()
	var s gtables.NoSeries
	s.InitWithDBType(db, company, database.DBTypeSQLite)
	s.InitRecord()
	s.Code = types.Code(code)
	s.Default_nos, s.Manual_nos, s.Date_order = defaultNos, manualNos, dateOrder
	if !s.Insert(true) {
		t.Fatalf("insert series %s", code)
	}
}

func addLine(t *testing.T, db *sql.DB, code string, lineNo int, starting types.Date, startNo, endNo, warnNo string, incrementBy int) {
	t.Helper()
	var l gtables.NoSeriesLine
	l.InitWithDBType(db, company, database.DBTypeSQLite)
	l.InitRecord()
	l.Series_code, l.Line_no, l.Starting_date = types.Code(code), lineNo, starting
	l.Starting_no, l.Ending_no, l.Warning_no = types.Code(startNo), types.Code(endNo), types.Code(warnNo)
	if incrementBy > 0 {
		l.Increment_by_no = incrementBy
	}
	if !l.Insert(true) {
		t.Fatalf("insert line %s %d", code, lineNo)
	}
}

func code(t *testing.T, err error) apperrors.ErrorCode {
	t.Helper()
	appErr, ok := err.(*apperrors.AppError)
	if !ok {
		t.Fatalf("error %v is not an AppError", err)
	}
	return appErr.Code
}

func day(d int) types.Date { return types.NewDate(2026, time.January, d) }

func TestGetNextNo(t *testing.T) {
	db := newDB(t)
	addSeries(t, db, "CUST", true, true, false)
	addLine(t, db, "CUST", 10000, types.Date{}, "C00010", "C00030", "C00020", 10)
	m := noseries.New(ftables.DBSource(db, company, database.DBTypeSQLite))
	var warned error
	m.Warning = func(err error) { warned = err }

	for _, want := range []string{"C00010", "C00020"} {
		got, err := m.GetNextNo("CUST", day(5), true)
		if err != nil || got.String() != want {
			t.Fatalf("GetNextNo = %q, %v; want %q", got, err, want)
		}
	}
	if warned == nil || code(t, warned) != apperrors.ErrNoSeriesEndingNo {
		t.Errorf("warning at C00020 = %v", warned)
	}
	if got, err := m.GetNextNo("CUST", day(5), true); err != nil || got != "C00030" {
		t.Fatalf("last number = %q, %v", got, err)
	}
	var line gtables.NoSeriesLine
	line.InitWithDBType(db, company, database.DBTypeSQLite)
	if !line.GetByPK("CUST", 10000) || line.Open || line.Last_no_used != "C00030" || !line.Last_date_used.Equal(day(5)) {
		t.Errorf("line after the ending number = %+v (want closed, C00030)", line.ToMap())
	}
	if _, err := m.GetNextNo("CUST", day(5), true); err == nil || code(t, err) != apperrors.ErrNoSeriesNoLine {
		t.Errorf("closed line: %v", err) // the open filter stays: no line at all
	}
	if _, err := m.GetNextNo("NONE", day(5), true); err == nil || code(t, err) != apperrors.ErrRecordNotFound {
		t.Errorf("unknown series: %v", err)
	}
}

// Without a Warning function the user is told: the warning comes with the request's notes
func TestGetNextNoWarningNotifiesTheUser(t *testing.T) {
	db := newDB(t)
	addSeries(t, db, "CUST", true, true, false)
	addLine(t, db, "CUST", 10000, types.Date{}, "C00010", "C00030", "C00020", 10)
	m := noseries.New(ftables.DBSource(db, company, database.DBTypeSQLite))

	fcodeunits.StartNotes()
	for range 2 {
		_, err := m.GetNextNo("CUST", day(5), true)
		if err != nil {
			t.Fatal(err)
		}
	}
	notes := fcodeunits.TakeNotes()
	want := apperrors.NoSeriesEndingNo("C00030", "CUST").Message("en-US")
	if len(notes) != 1 || notes[0] != want {
		t.Errorf("notes = %q, want [%q]", notes, want)
	}
}

func TestGetNextNoStartingDateAndDateOrder(t *testing.T) {
	db := newDB(t)
	addSeries(t, db, "INV", true, false, true)
	addLine(t, db, "INV", 10000, day(1), "I1-001", "", "", 0)
	addLine(t, db, "INV", 20000, day(20), "I2-001", "", "", 0)
	m := noseries.New(ftables.DBSource(db, company, database.DBTypeSQLite))
	if got, _ := m.GetNextNo("INV", day(10), true); got != "I1-001" {
		t.Errorf("before the 2nd line = %q", got)
	}
	if got, _ := m.GetNextNo("INV", day(25), true); got != "I2-001" {
		t.Errorf("after the 2nd line's starting date = %q", got)
	}
	if got, _ := m.GetNextNo("INV", day(25), true); got != "I2-002" {
		t.Errorf("next = %q", got)
	}
	if _, err := m.GetNextNo("INV", day(21), true); err == nil || code(t, err) != apperrors.ErrNoSeriesDateOrder {
		t.Errorf("date order: %v", err)
	}

	addSeries(t, db, "EMPTY", true, false, false)
	if _, err := m.GetNextNo("EMPTY", day(1), true); err == nil || code(t, err) != apperrors.ErrNoSeriesNoLine {
		t.Errorf("no lines: %v", err)
	}
}

func TestGetNextNoWithoutModify(t *testing.T) {
	db := newDB(t)
	addSeries(t, db, "S", true, false, false)
	addLine(t, db, "S", 10000, types.Date{}, "100", "", "", 0)
	m := noseries.New(ftables.DBSource(db, company, database.DBTypeSQLite))
	if no := m.TryGetNextNo("S", day(1)); no != "100" {
		t.Errorf("TryGetNextNo = %q", no)
	}
	a, _ := m.GetNextNo("S", day(1), false)
	b, _ := m.GetNextNo("S", day(1), false)
	if a != "100" || b != "101" {
		t.Errorf("without modify = %q %q", a, b)
	}
	var line gtables.NoSeriesLine
	line.InitWithDBType(db, company, database.DBTypeSQLite)
	if line.GetByPK("S", 10000); line.Last_no_used != "" {
		t.Errorf("stored before SaveNoSeries: %q", line.Last_no_used)
	}
	if err := m.SaveNoSeries(); err != nil {
		t.Fatal(err)
	}
	if line.GetByPK("S", 10000); line.Last_no_used != "101" {
		t.Errorf("after SaveNoSeries: %q", line.Last_no_used)
	}
}

func TestInitSeriesAndTestManual(t *testing.T) {
	db := newDB(t)
	addSeries(t, db, "CUST", true, false, false)
	addSeries(t, db, "CUST-X", true, true, false)
	addSeries(t, db, "AUTO-OFF", false, true, false)
	addLine(t, db, "CUST", 10000, types.Date{}, "C001", "", "", 0)
	addLine(t, db, "CUST-X", 10000, types.Date{}, "X001", "", "", 0)
	var rel gtables.NoSeriesRelationship
	rel.InitWithDBType(db, company, database.DBTypeSQLite)
	rel.Code, rel.Series_code = "CUST", "CUST-X"
	if !rel.Insert(true) {
		t.Fatal("insert relationship")
	}
	m := noseries.New(ftables.DBSource(db, company, database.DBTypeSQLite))

	var no, series types.Code
	if err := m.InitSeries("CUST", "", day(1), &no, &series); err != nil || no != "C001" || series != "CUST" {
		t.Errorf("InitSeries = %q %q %v", no, series, err)
	}
	// the record's old series is related to the default: it is used
	no, series = "", ""
	if err := m.InitSeries("CUST", "CUST-X", day(1), &no, &series); err != nil || no != "X001" || series != "CUST-X" {
		t.Errorf("InitSeries old related = %q %q %v", no, series, err)
	}
	// an unrelated old series: the default
	no, series = "", ""
	if err := m.InitSeries("CUST", "AUTO-OFF", day(1), &no, &series); err != nil || no != "C002" || series != "CUST" {
		t.Errorf("InitSeries old unrelated = %q %q %v", no, series, err)
	}
	if err := m.TestSeries("CUST", "CUST-X"); err != nil {
		t.Errorf("TestSeries related: %v", err)
	}
	if err := m.TestSeries("CUST", "AUTO-OFF"); err == nil {
		t.Error("TestSeries unrelated: no error")
	}

	no, series = "", ""
	if err := m.InitSeries("AUTO-OFF", "", day(1), &no, &series); err == nil || code(t, err) != apperrors.ErrNoSeriesDefault {
		t.Errorf("no default nos: %v", err)
	} else if msg := err.(*apperrors.AppError).Message("en-US"); msg != "It is not possible to assign numbers automatically. If you want the program to assign numbers automatically, please activate Default Nos. in No. Series AUTO-OFF." {
		t.Errorf("message = %q", msg)
	}
	no = "MANUAL1"
	if err := m.InitSeries("CUST", "", day(1), &no, &series); err == nil || code(t, err) != apperrors.ErrNoSeriesManual {
		t.Errorf("manual number in CUST: %v", err)
	}
	if err := m.TestManual("CUST-X"); err != nil {
		t.Errorf("TestManual CUST-X: %v", err)
	}
	if err := m.TestManual(""); err != nil {
		t.Errorf("TestManual without series: %v", err)
	}

	var def types.Code
	if err := m.SetDefaultSeries(&def, "AUTO-OFF"); err != nil || def != "" {
		t.Errorf("SetDefaultSeries AUTO-OFF = %q %v", def, err)
	}
	if err := m.SetDefaultSeries(&def, "CUST"); err != nil || def != "CUST" {
		t.Errorf("SetDefaultSeries CUST = %q %v", def, err)
	}
}

func TestIncrementNoText(t *testing.T) {
	for _, c := range []struct {
		no   string
		by   int
		want string
	}{{"CUST00010", 10, "CUST00020"}, {"A-0099-X", 5, "A-0104-X"}, {"99", 3, "102"}} {
		got, err := noseries.IncrementNoText(types.Code(c.no), c.by)
		if err != nil || got.String() != c.want {
			t.Errorf("IncrementNoText(%s, %d) = %q, %v; want %s", c.no, c.by, got, err, c.want)
		}
	}
	if _, err := noseries.IncrementNoText("ABCDEFGHIJKLMNOPQR99", 1); err == nil {
		t.Error("over 20 characters: no error")
	}
}

func TestNoSeriesLineNumberFormat(t *testing.T) {
	db := newDB(t)
	var l gtables.NoSeriesLine
	l.InitWithDBType(db, company, database.DBTypeSQLite)
	l.InitRecord()
	if err := l.Validate("starting_no", types.Code("S-0001")); err != nil {
		t.Fatal(err)
	}
	// Ending No. takes the format of the new number
	if err := l.Validate("ending_no", types.Code("S-9999")); err != nil || l.Ending_no != "S-9999" {
		t.Fatalf("ending no = %q, %v", l.Ending_no, err)
	}
	if err := l.Validate("last_no_used", types.Code("S-9999")); err != nil || l.Open {
		t.Errorf("last no used = ending no: open %v, %v", l.Open, err)
	}
	if err := l.Validate("ending_no", types.Code("")); err != nil || l.Warning_no != "" || !l.Open {
		t.Errorf("ending no cleared: warning %q open %v, %v", l.Warning_no, l.Open, err)
	}
	if err := l.Validate("warning_no", types.Code("S-0900")); err == nil || code(t, err) != apperrors.ErrRequiredField {
		t.Errorf("warning no without ending no: %v", err)
	}
	if err := l.Validate("starting_no", types.Code("NONUMBER")); err == nil || code(t, err) != apperrors.ErrNoSeriesNoNumber {
		t.Errorf("no number: %v", err)
	} else if msg := err.(*apperrors.AppError).Message("en-US"); msg != "Starting No. must contain a number." {
		t.Errorf("message = %q", msg)
	}

	// a longer number part pads the other numbers (the field already has the new number)
	var w gtables.NoSeriesLine
	w.InitWithDBType(db, company, database.DBTypeSQLite)
	w.InitRecord()
	w.Starting_no, w.Ending_no = "A1", "A0010"
	if err := noseries.UpdateNoSeriesLine(&w, w.Ending_no, "ending_no"); err != nil {
		t.Fatal(err)
	}
	if w.Starting_no != "A0001" || w.Ending_no != "A0010" {
		t.Errorf("padded = %q %q", w.Starting_no, w.Ending_no)
	}
	// Last No. Used would change: the formats differ
	w.Last_no_used, w.Ending_no = "A0005", "B-0010"
	if err := noseries.UpdateNoSeriesLine(&w, w.Ending_no, "ending_no"); err == nil || code(t, err) != apperrors.ErrNoSeriesNumberFormat {
		t.Errorf("other format: %v", err)
	} else if msg := err.(*apperrors.AppError).Message("en-US"); msg != "The number format in Ending No. must be the same as the number format in Last No. Used." {
		t.Errorf("message = %q", msg)
	}
}
