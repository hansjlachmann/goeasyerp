package tables

import (
	"fmt"
	"reflect"

	"github.com/hansjlachmann/goeasyerp/backend/foundation/database"
	"github.com/hansjlachmann/goeasyerp/backend/foundation/types"
)

// FieldValuer is a record whose stored fields can be read by field number (generated
// FieldValue): the source of TransferFields
type FieldValuer interface {
	GetFields() []FieldInfo
	FieldValue(no int) (any, bool)
}

// FilterTarget is a record whose filters can be set (generated SetRange/SetFilter): the
// target of CopyFilter, also of another table
type FilterTarget interface {
	SetRange(fieldName string, values ...interface{})
	SetFilter(fieldName, filterExpr string)
}

// FieldFilter is a filter on a field in the filter syntax (generated FilterExpressions)
type FieldFilter struct {
	Field      string `json:"field"`
	Expression string `json:"expression"`
}

// OptionIndex returns the index of an option value — an int-based option type of any table,
// or an int — so option fields can be copied between tables (TransferFields)
func OptionIndex(v any) (int, bool) {
	rv := reflect.ValueOf(v)
	switch rv.Kind() {
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		return int(rv.Int()), true
	}
	return 0, false
}

// IsEmptyValue reports whether a field value is blank: "", 0, false, a blank date, an empty
// BLOB (TestField without a value)
func IsEmptyValue(v any) bool {
	if v == nil {
		return true
	}
	if z, ok := v.(interface{ IsZero() bool }); ok {
		return z.IsZero()
	}
	rv := reflect.ValueOf(v)
	if rv.Kind() == reflect.Slice {
		return rv.Len() == 0
	}
	return rv.IsZero()
}

// SameValue reports whether a field value equals a given value (TestField with a value):
// decimals and dates by value, options by index or by name, the rest as text
func SameValue(field, value any) bool {
	switch f := field.(type) {
	case types.Decimal:
		switch v := value.(type) {
		case types.Decimal:
			return f.Equal(v)
		case int:
			return f.Equal(types.NewDecimalFromInt(int64(v)))
		case float64:
			return f.Equal(types.NewDecimal(v))
		}
	case types.Date:
		if v, ok := value.(types.Date); ok {
			return f.Equal(v)
		}
	}
	if fi, ok := OptionIndex(field); ok {
		if vi, ok := OptionIndex(value); ok {
			return fi == vi
		}
	}
	return fmt.Sprint(field) == fmt.Sprint(value)
}

// CompanyExists reports whether a company exists (ChangeCompany)
func CompanyExists(db database.Executor, dbType database.DBType, name string) bool {
	query := `SELECT COUNT(*) FROM "Company" WHERE name = ?`
	if dbType == database.DBTypePostgres {
		query = `SELECT COUNT(*) FROM "Company" WHERE name = $1`
	}
	var n int
	if err := db.QueryRow(query, name).Scan(&n); err != nil {
		return false
	}
	return n > 0
}
