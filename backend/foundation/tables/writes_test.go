package tables

import (
	"testing"

	"github.com/hansjlachmann/goeasyerp/backend/foundation/types"
)

func TestRecWrites(t *testing.T) {
	db := cursorDB(t)
	r := &testRec{}
	var q Query
	var c Cursor
	stores := 0
	rec := testRecOn(db, r, &q, &c, &stores)

	*r = testRec{Code: "D", Name: "new", Amount: types.NewDecimalFromInt(7), Status: 1}
	if res := rec.Insert(); res != Inserted {
		t.Fatalf("Insert = %v", res)
	}
	if res := rec.Insert(); res != KeyTaken {
		t.Errorf("Insert of a taken key = %v", res)
	}
	if n := rec.Count(); n != 4 {
		t.Errorf("Count after insert = %d", n)
	}

	// with old values only the changed fields are written, a changed key renames
	if !rec.Get([]interface{}{"D"}) {
		t.Fatal("Get D")
	}
	old := testFields.OldValues(r.ref)
	// a BLOB always counts as changed: the test table always has something to write
	if res := rec.Modify(old); res != Modified {
		t.Errorf("Modify without changes but a BLOB = %v", res)
	}
	noBlob := &TableFields{Table: "Test", Fields: testFields.Fields[:4]}
	noBlobRec := *rec
	noBlobRec.Fields = noBlob
	if res := noBlobRec.Modify(noBlob.OldValues(r.ref)); res != NothingToSave {
		t.Errorf("Modify without changes = %v", res)
	}
	r.Name, r.Code = "changed", "E"
	if !testFields.KeyChanged(r.ref, old) {
		t.Error("KeyChanged")
	}
	if res := rec.Modify(old); res != Modified {
		t.Fatalf("Modify = %v", res)
	}
	*r = testRec{}
	if rec.Get([]interface{}{"D"}) || !rec.Get([]interface{}{"E"}) || r.Name != "changed" || r.Status != 1 {
		t.Errorf("after rename: %+v", r)
	}
	// without old values every field but the key
	r.Count = 42
	if res := rec.Modify(nil); res != Modified {
		t.Fatalf("Modify(nil) = %v", res)
	}
	*r = testRec{}
	if !rec.Get([]interface{}{"E"}) || r.Count != 42 {
		t.Errorf("after Modify(nil): %+v", r)
	}
	if !rec.Delete() || rec.Get([]interface{}{"E"}) {
		t.Error("Delete")
	}

	q.SetRange(rec.QT, "status", 1)
	if n := rec.ModifyAll("Name", "all"); n != 1 {
		t.Errorf("ModifyAll = %d", n)
	}
	if n := rec.ModifyAll("nothing", "x"); n != 0 {
		t.Errorf("ModifyAll on an unknown field = %d", n)
	}
	if n := rec.DeleteAll(); n != 1 {
		t.Errorf("DeleteAll = %d", n)
	}
	q.Reset()
	if n := rec.Count(); n != 2 {
		t.Errorf("Count at the end = %d", n)
	}
}

func TestRowValuesKeyDate(t *testing.T) {
	tf := &TableFields{Table: "K", Fields: []FieldMeta{
		{Info: FieldInfo{No: 1, Name: "date", PrimaryKey: true}, Kind: KindDate},
		{Info: FieldInfo{No: 2, Name: "other"}, Kind: KindDate},
	}}
	var key, other types.Date
	ref := func(i int) any {
		if i == 0 {
			return &key
		}
		return &other
	}
	values := tf.RowValues(ref)
	if values[0] != types.BlankKeyDate {
		t.Errorf("a blank key date is %v", values[0])
	}
	if _, ok := values[1].(types.Date); !ok {
		t.Errorf("a blank date outside the key stays a date: %v", values[1])
	}
	if cols := tf.Columns(); len(cols) != 2 || cols[0] != "date" {
		t.Errorf("columns = %v", cols)
	}
}
