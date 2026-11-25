package tables

import (
	"bytes"
	"fmt"
	"strings"
	"sync/atomic"

	"github.com/hansjlachmann/goeasyerp/backend/foundation/database"
	"github.com/hansjlachmann/goeasyerp/backend/foundation/types"
)

// The change log (S47): after every insert, modify (also a rename) and delete of a record the
// registered ChangeLogger may record what changed — written on the record's own database or
// transaction, so a change that is rolled back leaves no entry. Temporary records are never
// logged; ModifyAll and DeleteAll (no record read) are not either. The logger decides which
// tables and fields it records (business-logic/tables: Change Log Setup).

// ChangeType is what happened to a record (the options of Change Log Entry's Type of Change)
type ChangeType int

const (
	ChangeInsertion ChangeType = iota
	ChangeModification
	ChangeDeletion
)

// FieldChange is one field of a change: its value before (blank for an insertion) and after
// (blank for a deletion), as text. A secret (sensitive, masked or encrypted field) or a picture
// is never written out: Secret is set and the values say only "(changed)".
type FieldChange struct {
	No       int
	Name     string
	Old, New string
	Secret   bool
}

// Change is one record's change
type Change struct {
	Exec    database.Executor // the record's database or transaction: write the entries there
	DBType  database.DBType
	TableID int
	Table   string // the table's name
	Company string // "" a global table
	Key     string // the primary key's values, comma separated (after a rename: the new key)
	Type    ChangeType
	Fields  []FieldChange
}

// ChangeLogger is what records changes
type ChangeLogger interface {
	// Logs: whether the change log records this kind of change of the table, and which fields
	// (nil fields: none) — called for every write, so it must be quick
	Logs(tableID int, change ChangeType) (field func(no int) bool)
	// Log writes the change's entries
	Log(change Change) error
}

var changeLogger atomic.Pointer[ChangeLogger]

// SetChangeLogger sets the change log (nil: none)
func SetChangeLogger(l ChangeLogger) {
	if l == nil {
		changeLogger.Store(nil)
		return
	}
	changeLogger.Store(&l)
}

// secretMarker is what the change log says of a secret's value
const secretMarker = "(changed)"

// logChange gives the change of the record to the change log (old: the record as read for a
// modification; the record's own values otherwise). A change log that fails does not undo
// the change: it is printed.
func (r *Rec) logChange(change ChangeType, old map[string]any) {
	lp := changeLogger.Load()
	if lp == nil || r.Temporary || r.TableID == 0 {
		return
	}
	logger := *lp
	wanted := logger.Logs(r.TableID, change)
	if wanted == nil {
		return
	}
	tf := r.Fields
	c := Change{Exec: r.Exec, DBType: database.DBTypeSQLite, TableID: r.TableID, Table: tf.Table, Company: r.Company, Type: change}
	if r.Postgres {
		c.DBType = database.DBTypePostgres
	}
	var key []string
	for i := range tf.Fields {
		f := &tf.Fields[i]
		value := Value(r.Ref(i))
		if f.Info.PrimaryKey {
			key = append(key, changeText(f, value))
		}
		if f.Info.FlowField || !wanted(f.Info.No) {
			continue
		}
		secret := f.Info.Sensitive || f.Info.Masked || f.Info.Type == FieldTypeBlob
		fc := FieldChange{No: f.Info.No, Name: f.Info.Name, Secret: secret}
		switch change {
		case ChangeInsertion:
			if isBlank(value) {
				continue
			}
			fc.New = changeText(f, value)
		case ChangeDeletion:
			if isBlank(value) {
				continue
			}
			fc.Old = changeText(f, value)
		case ChangeModification:
			if old == nil || !tf.Changed(r.Ref, old, f.Info.Name) {
				continue
			}
			if f.Info.Type == FieldTypeBlob && sameBytes(old[f.Info.Name], value) {
				continue // a picture written again unchanged
			}
			fc.Old = changeText(f, old[f.Info.Name])
			fc.New = changeText(f, value)
		}
		if secret {
			fc.Old, fc.New = "", secretMarker
		}
		c.Fields = append(c.Fields, fc)
	}
	if len(c.Fields) == 0 {
		return
	}
	c.Key = strings.Join(key, ",")
	if err := logger.Log(c); err != nil {
		fmt.Printf("Error: change log of %s %s: %v\n", tf.Table, c.Key, err)
	}
}

// sameBytes: two picture values hold the same bytes (nil and empty are the same)
func sameBytes(a, b any) bool {
	x, _ := a.([]byte)
	y, _ := b.([]byte)
	return bytes.Equal(x, y)
}

// isBlank: a value a new or deleted record does not need to show
func isBlank(v any) bool {
	switch x := v.(type) {
	case nil:
		return true
	case types.Code:
		return x == ""
	case types.Text:
		return x == ""
	case string:
		return x == ""
	case bool:
		return !x
	case int:
		return x == 0
	case int64:
		return x == 0
	case types.Decimal:
		return x.IsZero()
	case types.Date:
		return x.IsZero()
	case types.DateTime:
		return x.IsZero()
	case []byte:
		return len(x) == 0
	}
	return false
}

// changeText is a value as the change log shows it: an option by its name, Yes/No, dates ISO
func changeText(f *FieldMeta, v any) string {
	switch x := v.(type) {
	case nil:
		return ""
	case bool:
		if x {
			return "Yes"
		}
		return "No"
	case []byte:
		return ""
	case types.Date:
		return x.String()
	case types.DateTime:
		if x.IsZero() {
			return ""
		}
		return x.Format("2006-01-02 15:04:05")
	}
	if len(f.Options) > 0 {
		n, ok := optionNumber(v)
		if ok && n >= 0 && n < len(f.Options) {
			return f.Options[n]
		}
	}
	return fmt.Sprint(v)
}

// optionNumber: an option field's value as its number
func optionNumber(v any) (int, bool) {
	switch x := v.(type) {
	case int:
		return x, true
	case int64:
		return int(x), true
	case int32:
		return int(x), true
	}
	var n int
	_, err := fmt.Sscan(fmt.Sprint(v), &n)
	return n, err == nil
}
