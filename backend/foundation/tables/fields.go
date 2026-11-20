package tables

import (
	"encoding/base64"
	"fmt"
	"reflect"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/hansjlachmann/goeasyerp/backend/foundation/types"
)

// The field functions of the generated records, written once for all tables: a table's
// fields are described by data (TableFields, generated as a static variable) and the record
// gives a pointer to a field by its index (the generated fieldRef). Generating these
// functions per table and field made the generated package too large to compile on a small
// server. Each function keeps the conversions the generated method it replaces had.

// Kind is the Go type of a field
type Kind uint8

const (
	KindCode     Kind = iota + 1 // types.Code
	KindText                     // types.Text
	KindDecimal                  // types.Decimal
	KindDate                     // types.Date
	KindDateTime                 // types.DateTime
	KindBlob                     // []byte
	KindBool                     // bool
	KindOption                   // the table's option type (an int)
	KindInt                      // int
)

// FieldMeta describes a field of a table for the generic record functions
type FieldMeta struct {
	Info        FieldInfo // what GetFields returns
	Kind        Kind
	Trigger     string   // the field's OnValidate trigger method (OnValidate_<Field>)
	Options     []string // an option field's values
	DateFormula bool     // a date formula field: checked when validated
	Relation    bool     // has a table relation: checked when validated
	App         string   // the extension app that adds the field ("" the base application)
}

// TableFields describes a table's fields (generated per table, static data)
type TableFields struct {
	Table       string      // the table's name (messages)
	Fields      []FieldMeta // the table's fields in definition order, FlowFields too
	FlowFilters []FieldInfo // the FlowFilter fields (number and name only)

	once       sync.Once
	byName     map[string]int // lower case name → index in Fields
	byNo       map[int]int    // number → index in Fields
	primaryKey []string       // the primary key's columns in key order
}

func (tf *TableFields) index() {
	tf.once.Do(func() {
		tf.byName = make(map[string]int, len(tf.Fields))
		tf.byNo = make(map[int]int, len(tf.Fields))
		for i := range tf.Fields {
			tf.byName[strings.ToLower(tf.Fields[i].Info.Name)] = i
			tf.byNo[tf.Fields[i].Info.No] = i
			if tf.Fields[i].Info.PrimaryKey {
				tf.primaryKey = append(tf.primaryKey, tf.Fields[i].Info.Name)
			}
		}
	})
}

// Index returns the index of a field by its name (case-insensitive), -1 when unknown
func (tf *TableFields) Index(name string) int {
	tf.index()
	if i, ok := tf.byName[strings.ToLower(name)]; ok {
		return i
	}
	return -1
}

// IndexOfNo returns the index of a field by its number, -1 when unknown
func (tf *TableFields) IndexOfNo(no int) int {
	tf.index()
	if i, ok := tf.byNo[no]; ok {
		return i
	}
	return -1
}

// storedIndex returns the index of a stored field (not a FlowField) by its exact name
func (tf *TableFields) storedIndex(name string) int {
	i := tf.Index(name)
	if i < 0 || tf.Fields[i].Info.FlowField || tf.Fields[i].Info.Name != name {
		return -1
	}
	return i
}

// Infos returns the fields' FieldInfo (GetFields): a new slice, the caller may change it
func (tf *TableFields) Infos() []FieldInfo {
	infos := make([]FieldInfo, len(tf.Fields))
	for i := range tf.Fields {
		infos[i] = tf.Fields[i].Info
	}
	return infos
}

// FieldNo returns the number of a field or FlowFilter field (case-insensitive), 0 when unknown
func (tf *TableFields) FieldNo(name string) int {
	if i := tf.Index(name); i >= 0 {
		return tf.Fields[i].Info.No
	}
	lower := strings.ToLower(name)
	for _, f := range tf.FlowFilters {
		if f.Name == lower {
			return f.No
		}
	}
	return 0
}

// FieldName returns the name of a field or FlowFilter field by its number, "" when unknown
func (tf *TableFields) FieldName(no int) string {
	if i := tf.IndexOfNo(no); i >= 0 {
		return tf.Fields[i].Info.Name
	}
	for _, f := range tf.FlowFilters {
		if f.No == no {
			return f.Name
		}
	}
	return ""
}

// ColumnName maps a field name (case-insensitive) to its database column: stored fields only
func (tf *TableFields) ColumnName(name string) (string, bool) {
	i := tf.Index(name)
	if i < 0 || tf.Fields[i].Info.FlowField {
		return "", false
	}
	return tf.Fields[i].Info.Name, true
}

// FieldRef gives a pointer to a record's field by its index in TableFields.Fields (generated)
type FieldRef func(i int) any

// Value returns the value a field pointer points to
func Value(ptr any) any {
	switch p := ptr.(type) {
	case *types.Code:
		return *p
	case *types.Text:
		return *p
	case *types.Decimal:
		return *p
	case *types.Date:
		return *p
	case *types.DateTime:
		return *p
	case *[]byte:
		return *p
	case *bool:
		return *p
	case *int:
		return *p
	}
	return reflect.ValueOf(ptr).Elem().Interface() // an option type
}

func optionValue(ptr any) reflect.Value {
	return reflect.ValueOf(ptr).Elem()
}

// FieldValue returns a stored field's value by its number (FlowFields are not stored)
func (tf *TableFields) FieldValue(ref FieldRef, no int) (any, bool) {
	i := tf.IndexOfNo(no)
	if i < 0 || tf.Fields[i].Info.FlowField {
		return nil, false
	}
	return Value(ref(i)), true
}

// FlowFieldValue returns a FlowField's value by its number, as CalcFields left it
func (tf *TableFields) FlowFieldValue(ref FieldRef, no int) (any, bool) {
	i := tf.IndexOfNo(no)
	if i < 0 || !tf.Fields[i].Info.FlowField {
		return nil, false
	}
	return Value(ref(i)), true
}

// ColumnValue returns a stored field's value by its column (exact name), as an SQL argument:
// an option as its number
func (tf *TableFields) ColumnValue(ref FieldRef, column string) (any, bool) {
	i := tf.storedIndex(column)
	if i < 0 {
		return nil, false
	}
	if tf.Fields[i].Kind == KindOption {
		return int(optionValue(ref(i)).Int()), true
	}
	return Value(ref(i)), true
}

// SetFieldValue sets a stored field by its number when the value has the field's type
// (options: any option value or int; codes and texts also a string); it reports whether the
// field was set
func (tf *TableFields) SetFieldValue(ref FieldRef, no int, value any) bool {
	i := tf.IndexOfNo(no)
	if i < 0 || tf.Fields[i].Info.FlowField {
		return false
	}
	m := &tf.Fields[i]
	switch p := ref(i).(type) {
	case *types.Code:
		switch v := value.(type) {
		case types.Code:
			*p = v
			return true
		case string:
			*p = types.NewCode(v)
			return true
		}
	case *types.Text:
		switch v := value.(type) {
		case types.Text:
			*p = v
			return true
		case string:
			*p = types.NewText(v)
			return true
		}
	case *types.Decimal:
		if v, ok := value.(types.Decimal); ok {
			*p = v
			return true
		}
	case *types.Date:
		if v, ok := value.(types.Date); ok {
			*p = v
			return true
		}
	case *types.DateTime:
		if v, ok := value.(types.DateTime); ok {
			*p = v
			return true
		}
	case *[]byte:
		if v, ok := value.([]byte); ok {
			*p = v
			return true
		}
	case *bool:
		if v, ok := value.(bool); ok {
			*p = v
			return true
		}
	case *int:
		if v, ok := value.(int); ok {
			*p = v
			return true
		}
	default:
		if n, ok := OptionIndex(value); ok && n >= 0 && n < len(m.Options) {
			optionValue(p).SetInt(int64(n))
			return true
		}
	}
	return false
}

// SetSameType sets a stored field to a value of exactly its type (XRec from the old values);
// it reports whether it did
func SetSameType(ptr any, value any) bool {
	switch p := ptr.(type) {
	case *types.Code:
		v, ok := value.(types.Code)
		if ok {
			*p = v
		}
		return ok
	case *types.Text:
		v, ok := value.(types.Text)
		if ok {
			*p = v
		}
		return ok
	case *types.Decimal:
		v, ok := value.(types.Decimal)
		if ok {
			*p = v
		}
		return ok
	case *types.Date:
		v, ok := value.(types.Date)
		if ok {
			*p = v
		}
		return ok
	case *types.DateTime:
		v, ok := value.(types.DateTime)
		if ok {
			*p = v
		}
		return ok
	case *[]byte:
		v, ok := value.([]byte)
		if ok {
			*p = v
		}
		return ok
	case *bool:
		v, ok := value.(bool)
		if ok {
			*p = v
		}
		return ok
	case *int:
		v, ok := value.(int)
		if ok {
			*p = v
		}
		return ok
	}
	rv := optionValue(ptr)
	if value == nil || reflect.TypeOf(value) != rv.Type() {
		return false
	}
	rv.Set(reflect.ValueOf(value))
	return true
}

// OldValues returns the stored fields' values by name (StoreOldValues)
func (tf *TableFields) OldValues(ref FieldRef) map[string]any {
	old := make(map[string]any, len(tf.Fields))
	for i := range tf.Fields {
		if !tf.Fields[i].Info.FlowField {
			old[tf.Fields[i].Info.Name] = Value(ref(i))
		}
	}
	return old
}

// SetOldValues sets the stored fields from old values of their type (XRec)
func (tf *TableFields) SetOldValues(ref FieldRef, old map[string]any) {
	for i := range tf.Fields {
		if !tf.Fields[i].Info.FlowField {
			SetSameType(ref(i), old[tf.Fields[i].Info.Name])
		}
	}
}

// Changed reports whether a stored field (exact name) differs from its old value. Without old
// values, or without the field among them, it has changed; a BLOB always has. An old value of
// another type counts as unchanged, except for Yes/No and whole number fields.
func (tf *TableFields) Changed(ref FieldRef, old map[string]any, name string) bool {
	if old == nil {
		return true
	}
	oldValue, exists := old[name]
	if !exists {
		return true
	}
	i := tf.storedIndex(name)
	if i < 0 {
		return false
	}
	switch p := ref(i).(type) {
	case *types.Code:
		if o, ok := oldValue.(types.Code); ok {
			return !p.Equal(o)
		}
	case *types.Text:
		if o, ok := oldValue.(types.Text); ok {
			return !p.Equal(o)
		}
	case *types.Decimal:
		if o, ok := oldValue.(types.Decimal); ok {
			return !p.Equal(o)
		}
	case *types.Date:
		if o, ok := oldValue.(types.Date); ok {
			return !p.Equal(o)
		}
	case *types.DateTime:
		if o, ok := oldValue.(types.DateTime); ok {
			return !p.Equal(o)
		}
	case *[]byte:
		return true
	case *bool:
		if o, ok := oldValue.(bool); ok {
			return *p != o
		}
		return true
	case *int:
		if o, ok := oldValue.(int); ok {
			return *p != o
		}
		return true
	default:
		rv := optionValue(p)
		if oldValue != nil && reflect.TypeOf(oldValue) == rv.Type() {
			return rv.Int() != reflect.ValueOf(oldValue).Int()
		}
	}
	return false
}

// ToMap converts a record to a map for JSON: codes, texts, decimals and dates as text,
// options as their number; FlowFields as they are, decimals as text
func (tf *TableFields) ToMap(ref FieldRef) map[string]any {
	m := make(map[string]any, len(tf.Fields))
	for i := range tf.Fields {
		f := &tf.Fields[i]
		ptr := ref(i)
		if f.Info.FlowField {
			if d, ok := ptr.(*types.Decimal); ok {
				m[f.Info.Name] = d.String()
			} else {
				m[f.Info.Name] = Value(ptr)
			}
			continue
		}
		switch p := ptr.(type) {
		case *types.Code:
			m[f.Info.Name] = p.String()
		case *types.Text:
			m[f.Info.Name] = p.String()
		case *types.Decimal:
			m[f.Info.Name] = p.String()
		case *types.Date:
			m[f.Info.Name] = p.String()
		case *types.DateTime:
			m[f.Info.Name] = p.String()
		default:
			if f.Kind == KindOption {
				m[f.Info.Name] = int(optionValue(ptr).Int())
			} else {
				m[f.Info.Name] = Value(ptr)
			}
		}
	}
	return m
}

// FromMap sets the stored fields present (and not nil) in a map from the API; a value that
// does not convert leaves the field as it is
func (tf *TableFields) FromMap(ref FieldRef, data map[string]any) {
	for i := range tf.Fields {
		f := &tf.Fields[i]
		if f.Info.FlowField {
			continue
		}
		v, ok := data[f.Info.Name]
		if !ok || v == nil {
			continue
		}
		switch p := ref(i).(type) {
		case *types.Code:
			if s, ok := v.(string); ok {
				*p = types.NewCode(s)
			}
		case *types.Text:
			if s, ok := v.(string); ok {
				*p = types.NewText(s)
			}
		case *types.Decimal:
			switch val := v.(type) {
			case string:
				*p, _ = types.NewDecimalFromString(val)
			case float64:
				*p = types.NewDecimal(val)
			case int:
				*p = types.NewDecimalFromInt(int64(val))
			case int64:
				*p = types.NewDecimalFromInt(val)
			}
		case *types.Date:
			if s, ok := v.(string); ok {
				*p, _ = types.NewDateFromString(s)
			}
		case *types.DateTime:
			if s, ok := v.(string); ok {
				*p, _ = types.NewDateTimeFromString(s)
			}
		case *bool:
			if b, ok := v.(bool); ok {
				*p = b
			}
		case *int:
			switch val := v.(type) {
			case float64:
				*p = int(val)
			case int:
				*p = val
			case string:
				// a record key from a URL or a list selection
				if n, err := strconv.Atoi(val); err == nil {
					*p = n
				}
			}
		case *[]byte:
			// not set from the API (see the blob endpoints)
		default:
			rv := optionValue(p)
			switch val := v.(type) {
			case float64:
				rv.SetInt(int64(int(val)))
			case int:
				rv.SetInt(int64(val))
			case string:
				for n, opt := range f.Options {
					if opt == val {
						rv.SetInt(int64(n))
						break
					}
				}
			}
		}
	}
}

// ValidateSet sets a stored field (case-insensitive name) to a value as VALIDATE takes it —
// converting text, numbers and option names — and checks a date formula; it returns the
// field's index. The caller then checks the relation and runs the OnValidate triggers.
func (tf *TableFields) ValidateSet(ref FieldRef, fieldName string, value any) (int, error) {
	i := tf.Index(fieldName)
	if i < 0 || tf.Fields[i].Info.FlowField {
		return -1, fmt.Errorf("field '%s' not found", fieldName)
	}
	f := &tf.Fields[i]
	name := f.Info.Name
	switch p := ref(i).(type) {
	case *types.Code:
		if v, ok := value.(types.Code); ok {
			*p = v
		} else if v, ok := value.(string); ok {
			*p = types.NewCode(v)
		} else {
			return i, fmt.Errorf("invalid type for field %s", name)
		}
		if f.DateFormula {
			if err := CheckDateFormula(tf.Table, name, string(*p)); err != nil {
				return i, err
			}
		}
	case *types.Text:
		if v, ok := value.(types.Text); ok {
			*p = v
		} else if v, ok := value.(string); ok {
			*p = types.NewText(v)
		} else {
			return i, fmt.Errorf("invalid type for field %s", name)
		}
	case *types.Decimal:
		if v, ok := value.(types.Decimal); ok {
			*p = v
		} else if v, ok := value.(string); ok {
			d, err := types.NewDecimalFromString(v)
			*p = d
			if err != nil {
				return i, fmt.Errorf("invalid decimal value for field %s: %w", name, err)
			}
		} else if v, ok := value.(float64); ok {
			*p = types.NewDecimal(v)
		} else if v, ok := value.(int); ok {
			*p = types.NewDecimalFromInt(int64(v))
		} else if v, ok := value.(int64); ok {
			*p = types.NewDecimalFromInt(v)
		} else {
			return i, fmt.Errorf("invalid type for field %s (expected Decimal, string, float64, int, or int64)", name)
		}
	case *types.Date:
		if v, ok := value.(types.Date); ok {
			*p = v
		} else if v, ok := value.(string); ok {
			d, err := types.NewDateFromString(v)
			*p = d
			if err != nil {
				return i, fmt.Errorf("invalid date value for field %s: %w", name, err)
			}
		} else if v, ok := value.(time.Time); ok {
			*p = types.NewDateFromTime(v)
		} else {
			return i, fmt.Errorf("invalid type for field %s (expected Date, string, or time.Time)", name)
		}
	case *types.DateTime:
		if v, ok := value.(types.DateTime); ok {
			*p = v
		} else if v, ok := value.(string); ok {
			d, err := types.NewDateTimeFromString(v)
			*p = d
			if err != nil {
				return i, fmt.Errorf("invalid datetime value for field %s: %w", name, err)
			}
		} else if v, ok := value.(time.Time); ok {
			*p = types.NewDateTimeFromTime(v)
		} else {
			return i, fmt.Errorf("invalid type for field %s (expected DateTime, string, or time.Time)", name)
		}
	case *[]byte:
		// nil, []byte, or base64-encoded text
		if value == nil {
			*p = nil
		} else if v, ok := value.([]byte); ok {
			*p = v
		} else if s, ok := value.(string); ok {
			if s == "" {
				*p = nil
			} else {
				decoded, err := base64.StdEncoding.DecodeString(s)
				if err != nil {
					return i, fmt.Errorf("invalid base64 value for field %s: %w", name, err)
				}
				*p = decoded
			}
		} else {
			return i, fmt.Errorf("invalid type for field %s (expected []byte, string, or nil)", name)
		}
	case *bool:
		if v, ok := value.(bool); ok {
			*p = v
		} else if v, ok := value.(string); ok {
			// text from JSON or the frontend
			*p = v == "true" || v == "1"
		} else {
			return i, fmt.Errorf("invalid type for field %s", name)
		}
	case *int:
		switch v := value.(type) {
		case int:
			*p = v
		case float64:
			*p = int(v)
		case string:
			if v == "" {
				*p = 0
			} else if n, err := strconv.Atoi(v); err == nil {
				*p = n
			} else {
				return i, fmt.Errorf("invalid integer value for field %s: %s", name, v)
			}
		default:
			return i, fmt.Errorf("invalid type for field %s", name)
		}
	default:
		if err := validateOption(f, optionValue(p), value); err != nil {
			return i, err
		}
	}
	return i, nil
}

func validateOption(f *FieldMeta, rv reflect.Value, value any) error {
	name, count := f.Info.Name, len(f.Options)
	if value != nil && reflect.TypeOf(value) == rv.Type() {
		rv.Set(reflect.ValueOf(value))
		return nil
	}
	switch v := value.(type) {
	case int:
		if v < 0 || v >= count {
			return fmt.Errorf("invalid option value %d for field %s (valid range: 0-%d)", v, name, count-1)
		}
		rv.SetInt(int64(v))
	case float64:
		n := int(v)
		if n < 0 || n >= count {
			return fmt.Errorf("invalid option value %d for field %s (valid range: 0-%d)", n, name, count-1)
		}
		rv.SetInt(int64(n))
	case string:
		if v == "" {
			// blank: the first option
			rv.SetInt(0)
			return nil
		}
		for n, opt := range f.Options {
			if opt == v {
				rv.SetInt(int64(n))
				return nil
			}
		}
		return fmt.Errorf("invalid option '%s' for field %s (valid options: %v)", v, name, f.Options)
	default:
		return fmt.Errorf("invalid type for field %s (expected %s, int, or string)", name, rv.Type().Name())
	}
	return nil
}

// OnValidate triggers: a business logic type's method OnValidate_<Field>(rec *T) error

var errorType = reflect.TypeOf((*error)(nil)).Elem()

type triggerKey struct {
	logic  reflect.Type
	method string
}

// triggerMethods caches, per logic type and trigger, the method's index (-1: the logic does
// not have it with the record's signature)
var triggerMethods sync.Map

func triggerMethod(logic any, method string, rec reflect.Type) int {
	typ := reflect.TypeOf(logic)
	key := triggerKey{typ, method}
	if i, ok := triggerMethods.Load(key); ok {
		return i.(int)
	}
	index := -1
	if m, ok := typ.MethodByName(method); ok {
		ft := m.Type // func(logic, rec) error
		if ft.NumIn() == 2 && ft.In(1) == rec && ft.NumOut() == 1 && ft.Out(0) == errorType {
			index = m.Index
		}
	}
	triggerMethods.Store(key, index)
	return index
}

// IsOnValidateTrigger reports whether name is a field's OnValidate trigger of the table and
// whether logic has it with the signature func(rec *T) error (rec: a *T)
func (tf *TableFields) IsOnValidateTrigger(name string, logic any, rec any) (known, ok bool) {
	for i := range tf.Fields {
		if !tf.Fields[i].Info.FlowField && tf.Fields[i].Trigger == name {
			return true, triggerMethod(logic, name, reflect.TypeOf(rec)) >= 0
		}
	}
	return false, false
}

// RunOnValidate runs a field's OnValidate trigger of every logic that has it, in order; the
// first error stops
func RunOnValidate[P any](logic []any, rec P, method string) error {
	recType := reflect.TypeOf(rec)
	for _, l := range logic {
		i := triggerMethod(l, method, recType)
		if i < 0 {
			continue
		}
		fn, ok := reflect.ValueOf(l).Method(i).Interface().(func(P) error)
		if !ok {
			continue
		}
		if err := fn(rec); err != nil {
			return err
		}
	}
	return nil
}
