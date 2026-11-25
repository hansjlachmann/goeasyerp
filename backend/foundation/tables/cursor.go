package tables

import (
	"database/sql"
	"fmt"
	"reflect"
	"strings"

	"github.com/hansjlachmann/goeasyerp/backend/foundation/database"
	"github.com/hansjlachmann/goeasyerp/backend/foundation/types"
)

// Reading records in a key, written once for all tables (see fields.go): FindFirst, FindLast,
// Count, FindSet, FindSetBuffered and Next of the generated records call these with their Rec.

// Cursor is a record's result set — open rows (FindSet) or the rows read into memory
// (FindSetBuffered, temporary tables, transactions) — and its paging window
type Cursor struct {
	rows   *sql.Rows
	buffer [][]any // the rows' field values (FlowFields blank); nil when not buffered
	pos    int

	// the paging window of FindSet/FindSetBuffered (a limit of 0: all rows)
	limit  int
	offset int
}

// Rec is what the read functions need of a record (the generated rec())
type Rec struct {
	Fields *TableFields
	Ref    FieldRef
	QT     QueryTable
	Q      *Query
	Cur    *Cursor
	Exec   database.Executor // the database of the record's own table
	// Buffered: read all at once — a temporary record (its in-memory database) and a record in
	// a transaction (one connection, which open rows would block for the statements of a loop)
	Buffered  bool
	Postgres  bool   // placeholders are $1, $2 …
	Table     string // the SQL table name (with the company prefix)
	ForUpdate string // " FOR UPDATE" after LockTable (PostgreSQL)
	Store     func() // StoreOldValues
	// what the change log needs (changelog.go): the table's number, the company ("" a global
	// table), and whether the record is temporary (never logged)
	TableID   int
	Company   string
	Temporary bool
}

// SelectList returns the stored fields' columns, comma separated, in the definition's order
func (tf *TableFields) SelectList() string {
	tf.index()
	return tf.selectList
}

// Placeholders turns ? into $1, $2 … on PostgreSQL
func (r *Rec) Placeholders(sql string, count int) string {
	if !r.Postgres {
		return sql
	}
	result := sql
	for i := 1; i <= count; i++ {
		result = strings.Replace(result, "?", fmt.Sprintf("$%d", i), 1)
	}
	return result
}

// SetPage sets the paging window of FindSet/FindSetBuffered: at most limit rows after the
// first offset; a limit of 0 is all rows, negative values are 0
func (c *Cursor) SetPage(limit, offset int) {
	if limit < 0 {
		limit = 0
	}
	if offset < 0 {
		offset = 0
	}
	c.limit = limit
	c.offset = offset
}

// LimitClause is the LIMIT/OFFSET clause of the paging window ("" without a limit); integer
// literals, which both SQLite and PostgreSQL accept
func (c *Cursor) LimitClause() string {
	if c.limit <= 0 {
		return ""
	}
	if c.offset > 0 {
		return fmt.Sprintf(" LIMIT %d OFFSET %d", c.limit, c.offset)
	}
	return fmt.Sprintf(" LIMIT %d", c.limit)
}

// CloseRows closes the open result set (FindSet)
func (c *Cursor) CloseRows() {
	if c.rows != nil {
		_ = c.rows.Close()
		c.rows = nil
	}
}

// readOne reads the first row of a query into the record; what names the read in an error
func (r *Rec) readOne(query string, args []interface{}, what string) bool {
	err := r.Fields.ScanRecord(r.Exec.QueryRow(query, args...), r.Ref)
	if err != nil {
		if err != sql.ErrNoRows {
			fmt.Printf("Error: Failed to %s %s: %v\n", what, r.Fields.Table, err)
		}
		return false
	}
	r.Store()
	return true
}

// Get reads the record with a primary key (values in key order, as SQL arguments)
func (r *Rec) Get(key []interface{}) bool {
	var where strings.Builder
	where.WriteString("1=1")
	for _, column := range r.Fields.PrimaryKey() {
		where.WriteString(" AND " + column + " = ?")
	}
	query := fmt.Sprintf(`SELECT %s FROM "%s" WHERE %s`, r.Fields.SelectList(), r.Table, where.String())
	query = r.Placeholders(query, len(key))
	query += r.ForUpdate
	return r.readOne(query, key, "get")
}

// FindFirst reads the first record in the current key and direction within the filters
// (classic FIND('-'))
func (r *Rec) FindFirst() bool {
	where, args := r.Q.Where(r.QT)
	orderBy, orderArgs := r.Q.OrderByClause(r.QT)
	args = append(args, orderArgs...)
	query := fmt.Sprintf(`SELECT %s FROM "%s" WHERE %s ORDER BY %s LIMIT 1`, r.Fields.SelectList(), r.Table, where, orderBy)
	query = r.Placeholders(query, len(args))
	query += r.ForUpdate
	return r.readOne(query, args, "find first")
}

// FindLast reads the last record in the current key and direction (classic FIND('+')): the
// key reversed, every column of it
func (r *Rec) FindLast() bool {
	where, args := r.Q.Where(r.QT)
	r.Q.Descending = !r.Q.Descending
	orderBy, orderArgs := r.Q.OrderByClause(r.QT)
	r.Q.Descending = !r.Q.Descending
	args = append(args, orderArgs...)
	query := fmt.Sprintf(`SELECT %s FROM "%s" WHERE %s ORDER BY %s LIMIT 1`, r.Fields.SelectList(), r.Table, where, orderBy)
	query = r.Placeholders(query, len(args))
	query += r.ForUpdate
	return r.readOne(query, args, "find last")
}

// Count returns the number of records within the filters
func (r *Rec) Count() int {
	where, args := r.Q.Where(r.QT)
	query := fmt.Sprintf(`SELECT COUNT(*) FROM "%s" WHERE %s`, r.Table, where)
	query = r.Placeholders(query, len(args))
	var count int
	if err := r.Exec.QueryRow(query, args...).Scan(&count); err != nil {
		fmt.Printf("Error: Failed to count %s: %v\n", r.Fields.Table, err)
		return 0
	}
	return count
}

// FindSet opens the records within the filters in the current key and direction and reads
// the first; Next reads the following ones. A buffered record reads them all at once.
func (r *Rec) FindSet() bool {
	if r.Buffered {
		return r.FindSetBuffered()
	}
	r.Cur.CloseRows()
	r.Cur.buffer = nil

	where, args := r.Q.Where(r.QT)
	orderBy, orderArgs := r.Q.OrderByClause(r.QT)
	args = append(args, orderArgs...)
	query := fmt.Sprintf(`SELECT %s FROM "%s" WHERE %s ORDER BY %s%s`, r.Fields.SelectList(), r.Table, where, orderBy, r.Cur.LimitClause())
	query = r.Placeholders(query, len(args))
	query += r.ForUpdate

	rows, err := r.Exec.Query(query, args...)
	if err != nil {
		fmt.Printf("Error: Failed to execute FindSet for %s: %v\n", r.Fields.Table, err)
		return false
	}
	r.Cur.rows = rows
	return r.Next(nil, nil)
}

// FindSetBuffered reads the records within the filters into memory, so Next can move
// backward and forward, and reads the first
func (r *Rec) FindSetBuffered() bool {
	r.Cur.CloseRows()
	r.Cur.buffer = nil
	r.Cur.pos = -1

	where, args := r.Q.Where(r.QT)
	orderBy, orderArgs := r.Q.OrderByClause(r.QT)
	args = append(args, orderArgs...)
	query := fmt.Sprintf(`SELECT %s FROM "%s" WHERE %s ORDER BY %s%s`, r.Fields.SelectList(), r.Table, where, orderBy, r.Cur.LimitClause())
	query = r.Placeholders(query, len(args))

	rows, err := r.Exec.Query(query, args...)
	if err != nil {
		fmt.Printf("Error: Failed to execute FindSetBuffered for %s: %v\n", r.Fields.Table, err)
		return false
	}
	defer func() { _ = rows.Close() }()

	// each row is read into the record and its values kept; the record gets its own values
	// back when a row cannot be read
	saved := r.Fields.values(r.Ref, false)
	for rows.Next() {
		if err := r.Fields.ScanRecord(rows, r.Ref); err != nil {
			fmt.Printf("Error: Failed to scan %s record: %v\n", r.Fields.Table, err)
			r.Fields.setValues(r.Ref, saved)
			return false
		}
		r.Cur.buffer = append(r.Cur.buffer, r.Fields.values(r.Ref, true))
	}
	if err := rows.Err(); err != nil {
		fmt.Printf("Error: Failed to iterate %s records: %v\n", r.Fields.Table, err)
		r.Fields.setValues(r.Ref, saved)
		return false
	}
	if len(r.Cur.buffer) == 0 {
		return false
	}
	r.Cur.pos = 0
	r.Fields.setValues(r.Ref, r.Cur.buffer[0])
	r.Store()
	return true
}

// Next moves steps records on (default 1; negative back): in the rows read into memory, in
// the open rows (forward only), else — after Get, Find, FindFirst, FindLast — from the
// current record in the current key within the filters, one step at a time, with findInKey
// (classic NEXT after FIND; a step that finds none leaves the record on the last one reached)
func (r *Rec) Next(steps []int, findInKey func(seek string, reverse bool) bool) bool {
	step := 1
	if len(steps) > 0 {
		step = steps[0]
	}
	c := r.Cur

	if c.buffer != nil {
		pos := c.pos + step
		if pos < 0 || pos >= len(c.buffer) {
			return false
		}
		c.pos = pos
		r.Fields.setValues(r.Ref, c.buffer[pos])
		r.Store()
		return true
	}

	if c.rows != nil {
		if step < 1 {
			fmt.Printf("Error: Backward navigation (Next(%d)) requires FindSetBuffered()\n", step)
			return false
		}
		for i := 0; i < step; i++ {
			if !c.rows.Next() {
				c.CloseRows()
				return false
			}
		}
		if err := r.Fields.ScanRecord(c.rows, r.Ref); err != nil {
			fmt.Printf("Error: Failed to scan %s record: %v\n", r.Fields.Table, err)
			c.CloseRows()
			return false
		}
		r.Store()
		return true
	}

	if step == 0 || findInKey == nil {
		return false
	}
	for i := 0; i != step; {
		if step > 0 {
			if !findInKey(">", false) {
				return false
			}
			i++
		} else {
			if !findInKey("<", true) {
				return false
			}
			i--
		}
	}
	return true
}

// FindInKey reads the first record in the current key and direction — reversed for the last —
// and with seek ">"/"<" only after/before the current record (a row-value comparison on the
// key and primary key columns). A key with a FlowField cannot seek.
func (r *Rec) FindInKey(seek string, reverse bool) bool {
	q, c := r.Q, r.Cur
	if seek != "" {
		var columns, params []string
		var values []interface{}
		for _, column := range q.KeyColumns(r.QT) {
			v, ok := r.Fields.ColumnValue(r.Ref, column)
			if !ok {
				fmt.Printf("Error: Find(%q) on %s needs a key of stored fields\n", seek, r.Fields.Table)
				return false
			}
			if r.Fields.IsKeyDateColumn(column) {
				v = types.KeyDate(v)
			}
			columns = append(columns, r.QT.CmpColumn(column))
			params = append(params, r.QT.CmpParam(column))
			values = append(values, v)
		}
		after := (seek == ">") != q.Descending
		op := "<"
		if after {
			op = ">"
		}
		q.Seek = "(" + strings.Join(columns, ", ") + ") " + op + " (" + strings.Join(params, ", ") + ")"
		q.SeekArgs = values
	}
	savedLimit, savedOffset, savedDescending := c.limit, c.offset, q.Descending
	c.limit, c.offset = 1, 0
	if reverse {
		q.Descending = !q.Descending
	}
	found := r.FindSet()
	// One record read: a later Next seeks from it in the key (a one-row buffer would end the
	// Next at once)
	c.CloseRows()
	c.buffer = nil
	c.limit, c.offset, q.Descending = savedLimit, savedOffset, savedDescending
	q.Seek, q.SeekArgs = "", nil
	return found
}

// values returns the record's field values; with blankFlowFields the FlowFields blank (a row
// read: its FlowFields are not calculated)
func (tf *TableFields) values(ref FieldRef, blankFlowFields bool) []any {
	values := make([]any, len(tf.Fields))
	for i := range tf.Fields {
		if blankFlowFields && tf.Fields[i].Info.FlowField {
			values[i] = zeroValue(ref(i))
		} else {
			values[i] = Value(ref(i))
		}
	}
	return values
}

// setValues sets every field from values of their types (values)
func (tf *TableFields) setValues(ref FieldRef, values []any) {
	for i := range tf.Fields {
		SetSameType(ref(i), values[i])
	}
}

// zeroValue is the blank value of a field's type
func zeroValue(ptr any) any {
	switch ptr.(type) {
	case *types.Code:
		return types.Code("")
	case *types.Text:
		return types.Text("")
	case *types.Decimal:
		return types.Decimal{}
	case *types.Date:
		return types.Date{}
	case *types.DateTime:
		return types.DateTime{}
	case *[]byte:
		return []byte(nil)
	case *bool:
		return false
	case *int:
		return 0
	}
	return reflect.Zero(reflect.TypeOf(ptr).Elem()).Interface()
}
