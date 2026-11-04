package tables

import (
	"fmt"
	"strings"

	"github.com/hansjlachmann/goeasyerp/backend/foundation/database"
	apperrors "github.com/hansjlachmann/goeasyerp/backend/foundation/errors"
)

// relatedTables finds a table by its registry name (the generated table registry registers it)
var relatedTables func(name string) (TableFactory, bool)

// RegisterRelatedTables gives the relation check the table registry (generated tables_gen.go)
func RegisterRelatedTables(lookup func(name string) (TableFactory, bool)) {
	relatedTables = lookup
}

// CheckRelation is the TableRelation test of VALIDATE (the generated Validate/ValidateField
// run it after setting the field, before its OnValidate triggers): a non-blank value must exist
// in the related table — the relation of the record's type for a conditional relation, among
// the records matching the relation's filters (WHERE). Lookup-only relations (validate: false)
// accept any value. The error names the field: "Payment Terms Code 'X' does not exist".
func CheckRelation(rec Table, db database.Executor, company string, dbType database.DBType, field string) error {
	if db == nil || relatedTables == nil {
		return nil // no database (a record in a test): nothing to check against
	}
	relInfo, ok := rec.GetTableRelationFields()[field]
	if !ok {
		return nil
	}
	record := rec.ToMap()
	value := relationValue(record[field])
	if value == "" {
		return nil
	}
	if relInfo, ok = relInfo.For(record); !ok || relInfo.NoValidate {
		return nil // no relation for this record's type, or the lookup only
	}
	factory, ok := relatedTables(relInfo.Table)
	if !ok {
		return nil // related table not registered
	}
	related := factory()
	related.InitWithDBType(db, company, dbType)
	found := false
	if len(relInfo.Filters) > 0 {
		// A relation to part of a composite key: the related record with this value among those
		// matching the relation's filters (e.g. a level of the record's reminder terms)
		related.SetFilter(relInfo.Field, value)
		for _, f := range relInfo.Filters {
			related.SetFilter(f.Column, relationFilterValue(f, record))
		}
		found = related.Count() > 0
	} else {
		found = related.Get(value)
	}
	if !found {
		return apperrors.RelatedNotFound(rec.GetTableName(), field, value)
	}
	return nil
}

// relationValue is a relation field's value as text ("" for blank; a blank integer is 0)
func relationValue(v interface{}) string {
	switch v := v.(type) {
	case nil:
		return ""
	case int, int64, float64:
		if s := fmt.Sprint(v); s != "0" {
			return s
		}
		return ""
	}
	return fmt.Sprint(v)
}

// relationFilterValue is the value a relation filter compares with: the record's field, or the
// filter's constant
func relationFilterValue(f RelationFilterInfo, record map[string]interface{}) string {
	if f.Field == "" {
		return f.Const
	}
	if v := record[f.Field]; v != nil {
		return fmt.Sprint(v)
	}
	return ""
}

// FilterBool is the value of a filter on a Boolean field: Yes, true, 1 (and the Norwegian/Danish
// Ja) are true, everything else false
func FilterBool(s string) bool {
	switch strings.ToLower(strings.TrimSpace(s)) {
	case "yes", "true", "1", "ja":
		return true
	}
	return false
}
