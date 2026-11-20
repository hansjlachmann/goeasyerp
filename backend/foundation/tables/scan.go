package tables

import (
	"database/sql"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/hansjlachmann/goeasyerp/backend/foundation/types"
)

// Reading records, written once for all tables (see fields.go): the generated Get, FindFirst,
// FindLast, Next and FindSetBuffered build their query and call ScanRecord.

// RowScanner is a row to read: *sql.Row or *sql.Rows
type RowScanner interface {
	Scan(dest ...any) error
}

// ScanRecord reads a row of the stored fields — the columns in the order of the definition,
// FlowFields left out — into a record. Codes, texts, decimals and dates are read as text
// (NULL is blank), Yes/No as a nullable boolean; the record is set only when the row was read.
func (tf *TableFields) ScanRecord(row RowScanner, ref FieldRef) error {
	n := len(tf.Fields)
	dest := make([]any, 0, n)
	strs := make([]sql.NullString, n)
	bools := make([]sql.NullBool, n)
	ints := make([]int, n)
	blobs := make([][]byte, n)
	for i := range tf.Fields {
		f := &tf.Fields[i]
		if f.Info.FlowField {
			continue
		}
		switch f.Kind {
		case KindBool:
			dest = append(dest, &bools[i])
		case KindOption, KindInt:
			dest = append(dest, &ints[i])
		case KindBlob:
			dest = append(dest, &blobs[i])
		default:
			dest = append(dest, &strs[i])
		}
	}
	if err := row.Scan(dest...); err != nil {
		return err
	}
	for i := range tf.Fields {
		f := &tf.Fields[i]
		if f.Info.FlowField {
			continue
		}
		switch p := ref(i).(type) {
		case *types.Code:
			*p = types.NewCode(strs[i].String)
		case *types.Text:
			*p = types.NewText(strs[i].String)
		case *types.Decimal:
			*p, _ = types.NewDecimalFromString(strs[i].String)
		case *types.Date:
			*p, _ = types.NewDateFromString(strs[i].String)
		case *types.DateTime:
			*p, _ = types.NewDateTimeFromString(strs[i].String)
		case *bool:
			*p = bools[i].Bool
		case *int:
			*p = ints[i]
		case *[]byte:
			*p = blobs[i]
		default:
			optionValue(p).SetInt(int64(ints[i]))
		}
	}
	return nil
}

// CopyFields copies every field — FlowFields too — from one record to another of the table
func (tf *TableFields) CopyFields(to, from FieldRef) {
	for i := range tf.Fields {
		SetSameType(to(i), Value(from(i)))
	}
}

// KeyFromMap returns the primary key values from a map of field name → value (Get with a
// composite key; values as the API sends them, also as text): in key order, of the fields'
// types — an option as its number; a value that does not convert is the blank value
func (tf *TableFields) KeyFromMap(key map[string]any) []any {
	tf.index()
	values := make([]any, 0, len(tf.primaryKey))
	for i := range tf.Fields {
		f := &tf.Fields[i]
		if !f.Info.PrimaryKey {
			continue
		}
		v, exists := key[f.Info.Name]
		values = append(values, keyPart(f.Kind, v, exists))
	}
	return values
}

func keyPart(kind Kind, v any, exists bool) any {
	switch kind {
	case KindCode:
		var c types.Code
		if exists {
			switch val := v.(type) {
			case types.Code:
				c = val
			case string:
				c = types.NewCode(val)
			}
		}
		return c
	case KindText:
		var t types.Text
		if exists {
			switch val := v.(type) {
			case types.Text:
				t = val
			case string:
				t = types.NewText(val)
			}
		}
		return t
	case KindInt:
		n := 0
		if exists {
			switch val := v.(type) {
			case int:
				n = val
			case float64:
				n = int(val)
			case string:
				// the key part as text: the API's "CUSTOMER1,10000"
				if i, err := strconv.Atoi(strings.TrimSpace(val)); err == nil {
					n = i
				}
			}
		}
		return n
	case KindOption:
		n := 0
		if exists {
			if i, ok := OptionIndex(v); ok {
				n = i
			} else {
				switch val := v.(type) {
				case float64:
					n = int(val)
				case string:
					if i, err := strconv.Atoi(val); err == nil {
						n = i
					}
				}
			}
		}
		return n
	case KindDecimal:
		var d types.Decimal
		if exists {
			switch val := v.(type) {
			case types.Decimal:
				d = val
			case int:
				d = types.NewDecimalFromInt(int64(val))
			case float64:
				d = types.NewDecimal(val)
			case string:
				// the key part as text: the API's "CODE,10"
				d, _ = types.NewDecimalFromString(strings.TrimSpace(val))
			}
		}
		return d
	case KindDate:
		var d types.Date
		if exists {
			switch val := v.(type) {
			case types.Date:
				d = val
			case time.Time:
				d = types.NewDate(val.Year(), val.Month(), val.Day())
			case string:
				d, _ = types.NewDateFromString(val)
			}
		}
		return d
	case KindBool:
		b := false
		if exists {
			switch val := v.(type) {
			case bool:
				b = val
			case string:
				// the key part as text: the API's "true,PCS,…"
				b, _ = strconv.ParseBool(strings.TrimSpace(val))
			}
		}
		return b
	case KindDateTime:
		var d types.DateTime
		return d
	}
	return nil
}

// KeyFromValue returns the value of a table's single primary key field from a value of
// another form (Get with the key itself): a code or text also as a string, a whole number
// also as a float or text, a date also as a time or text; false for a value it does not take
func (tf *TableFields) KeyFromValue(v any) (any, bool) {
	tf.index()
	if len(tf.primaryKey) != 1 {
		return nil, false
	}
	switch tf.Fields[tf.byName[tf.primaryKey[0]]].Kind {
	case KindCode:
		switch pk := v.(type) {
		case types.Code:
			return pk, true
		case string:
			return types.NewCode(pk), true
		}
	case KindText:
		switch pk := v.(type) {
		case types.Text:
			return pk, true
		case string:
			return types.NewText(pk), true
		}
	case KindInt:
		switch pk := v.(type) {
		case int:
			return pk, true
		case float64:
			return int(pk), true
		case string:
			var n int
			if _, err := fmt.Sscanf(pk, "%d", &n); err == nil {
				return n, true
			}
		}
	case KindDate:
		switch pk := v.(type) {
		case types.Date:
			return pk, true
		case time.Time:
			return types.NewDate(pk.Year(), pk.Month(), pk.Day()), true
		case string:
			if d, err := types.NewDateFromString(pk); err == nil {
				return d, true
			}
		}
	}
	return nil, false
}
