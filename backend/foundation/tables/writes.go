package tables

import (
	"fmt"
	"strings"

	"github.com/hansjlachmann/goeasyerp/backend/foundation/types"
)

// Writing records, written once for all tables (see fields.go): the generated Insert, Modify,
// Delete, InsertAll, ModifyAll and DeleteAll run their triggers and call these with their Rec.

// keyArg is a field's value as an SQL argument: a blank date of the primary key is
// types.BlankKeyDate (key columns are never NULL)
func (tf *TableFields) keyArg(i int, v any) any {
	if tf.Fields[i].Info.PrimaryKey && tf.Fields[i].Kind == KindDate {
		return types.KeyDate(v)
	}
	return v
}

// RowValues returns the stored fields' values as SQL arguments, in the columns' order
func (tf *TableFields) RowValues(ref FieldRef) []any {
	values := make([]any, 0, len(tf.Fields))
	for i := range tf.Fields {
		if !tf.Fields[i].Info.FlowField {
			values = append(values, tf.keyArg(i, Value(ref(i))))
		}
	}
	return values
}

// Columns returns the stored fields' columns in the definition's order
func (tf *TableFields) Columns() []string {
	var columns []string
	for i := range tf.Fields {
		if !tf.Fields[i].Info.FlowField {
			columns = append(columns, tf.Fields[i].Info.Name)
		}
	}
	return columns
}

// keyWhere is the WHERE clause matching the primary key ("1=1 AND a = ? AND b = ?")
func (tf *TableFields) keyWhere() string {
	var where strings.Builder
	where.WriteString("1=1")
	for _, column := range tf.PrimaryKey() {
		where.WriteString(" AND " + column + " = ?")
	}
	return where.String()
}

// KeyChanged reports whether a primary key field differs from its old value (a rename)
func (tf *TableFields) KeyChanged(ref FieldRef, old map[string]any) bool {
	if old == nil {
		return false
	}
	changed := false
	for _, column := range tf.PrimaryKey() {
		if tf.Changed(ref, old, column) {
			changed = true
		}
	}
	return changed
}

// InsertResult is how an Insert ended
type InsertResult int

const (
	Inserted     InsertResult = iota
	InsertFailed              // the statement failed (printed)
	KeyTaken                  // a record with the primary key exists: nothing was inserted
)

// Insert writes the record; a taken key inserts nothing (ON CONFLICT … DO NOTHING) instead of
// failing — on PostgreSQL a failed INSERT would abort the transaction
func (r *Rec) Insert() InsertResult {
	args := r.Fields.RowValues(r.Ref)
	placeholders := strings.TrimSuffix(strings.Repeat("?, ", len(args)), ", ")
	query := fmt.Sprintf(`INSERT INTO "%s" (%s) VALUES (%s) ON CONFLICT (%s) DO NOTHING`,
		r.Table, r.Fields.SelectList(), placeholders, strings.Join(r.Fields.PrimaryKey(), ", "))
	query = r.Placeholders(query, len(args))
	result, err := r.Exec.Exec(query, args...)
	if err != nil {
		fmt.Printf("Error: Failed to insert %s: %v\n", r.Fields.Table, err)
		return InsertFailed
	}
	if n, err := result.RowsAffected(); err == nil && n == 0 {
		return KeyTaken
	}
	r.logChange(ChangeInsertion, nil)
	return Inserted
}

// ModifyResult is how a Modify ended
type ModifyResult int

const (
	Modified      ModifyResult = iota
	ModifyFailed               // the statement failed (printed)
	NothingToSave              // no field changed since the record was read: no statement
)

// Modify writes the record: with old values (the record as read) only the fields that
// changed — a changed primary key field is renamed in place, SET to the new value while the
// WHERE clause matches the old one —, without them every field but the key
func (r *Rec) Modify(old map[string]any) ModifyResult {
	tf := r.Fields
	var setClauses []string
	var values []any
	if old != nil {
		for i := range tf.Fields {
			f := &tf.Fields[i]
			if f.Info.FlowField || !tf.Changed(r.Ref, old, f.Info.Name) {
				continue
			}
			setClauses = append(setClauses, f.Info.Name+" = ?")
			values = append(values, tf.keyArg(i, Value(r.Ref(i))))
		}
		if len(setClauses) == 0 {
			return NothingToSave
		}
	} else {
		for i := range tf.Fields {
			f := &tf.Fields[i]
			if f.Info.PrimaryKey || f.Info.FlowField {
				continue
			}
			setClauses = append(setClauses, f.Info.Name+" = ?")
			values = append(values, Value(r.Ref(i)))
		}
	}
	// the primary key as loaded, so a renamed key still matches
	for i := range tf.Fields {
		f := &tf.Fields[i]
		if !f.Info.PrimaryKey {
			continue
		}
		if v, ok := old[f.Info.Name]; ok {
			values = append(values, tf.keyArg(i, v))
		} else {
			values = append(values, tf.keyArg(i, Value(r.Ref(i))))
		}
	}
	query := fmt.Sprintf(`UPDATE "%s" SET %s WHERE %s`, r.Table, strings.Join(setClauses, ", "), tf.keyWhere())
	query = r.Placeholders(query, len(values))
	if _, err := r.Exec.Exec(query, values...); err != nil {
		fmt.Printf("Error: Failed to modify %s: %v\n", tf.Table, err)
		return ModifyFailed
	}
	r.logChange(ChangeModification, old)
	return Modified
}

// Delete deletes the record with the primary key; false when the statement failed (printed)
func (r *Rec) Delete() bool {
	tf := r.Fields
	var args []any
	for i := range tf.Fields {
		if tf.Fields[i].Info.PrimaryKey {
			args = append(args, tf.keyArg(i, Value(r.Ref(i))))
		}
	}
	query := fmt.Sprintf(`DELETE FROM "%s" WHERE %s`, r.Table, tf.keyWhere())
	query = r.Placeholders(query, len(args))
	result, err := r.Exec.Exec(query, args...)
	if err != nil {
		fmt.Printf("Error: Failed to delete %s: %v\n", tf.Table, err)
		return false
	}
	if n, err := result.RowsAffected(); err != nil || n > 0 {
		r.logChange(ChangeDeletion, nil)
	}
	return true
}

// ModifyAll sets a field of every record within the filters; it returns how many
func (r *Rec) ModifyAll(fieldName string, newValue any) int {
	column, ok := r.Fields.ColumnName(fieldName)
	if !ok {
		fmt.Printf("Error: ModifyAll on unknown field %q of %s\n", fieldName, r.Fields.Table)
		return 0
	}
	where, args := r.Q.Where(r.QT)
	query := fmt.Sprintf(`UPDATE "%s" SET %s = ? WHERE %s`, r.Table, column, where)
	allArgs := append([]any{newValue}, args...)
	query = r.Placeholders(query, len(allArgs))
	result, err := r.Exec.Exec(query, allArgs...)
	if err != nil {
		fmt.Printf("Error: Failed to modify all %s: %v\n", r.Fields.Table, err)
		return 0
	}
	n, _ := result.RowsAffected()
	return int(n)
}

// DeleteAll deletes every record within the filters; it returns how many
func (r *Rec) DeleteAll() int {
	where, args := r.Q.Where(r.QT)
	query := fmt.Sprintf(`DELETE FROM "%s" WHERE %s`, r.Table, where)
	query = r.Placeholders(query, len(args))
	result, err := r.Exec.Exec(query, args...)
	if err != nil {
		fmt.Printf("Error: Failed to delete all %s: %v\n", r.Fields.Table, err)
		return 0
	}
	n, _ := result.RowsAffected()
	return int(n)
}
