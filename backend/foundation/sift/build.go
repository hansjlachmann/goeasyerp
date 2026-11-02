package sift

import (
	"crypto/sha1"
	"encoding/hex"
	"fmt"
	"strings"

	"github.com/hansjlachmann/openerp/backend/foundation/database"
)

// version is part of every fingerprint: changing how totals are built rebuilds them all.
const version = "sift-v2" // v2: date key columns TIMESTAMP on Postgres (closing dates)

// Kind is the kind of value of a key or sum column.
type Kind string

const (
	KindText     Kind = "text"     // Code, Text
	KindInt      Kind = "int"      // int, Option
	KindBool     Kind = "bool"     // bool
	KindDate     Kind = "date"     // types.Date
	KindDateTime Kind = "datetime" // types.DateTime
	KindDecimal  Kind = "decimal"  // types.Decimal (sum fields)
)

// Column is a key or sum column of a SIFT key (the same column name as in the entry table).
type Column struct {
	Name string
	Kind Kind
}

// KeySpec describes a SIFT key: the key's fields and its sum index fields.
type KeySpec struct {
	Name   string
	Fields []Column
	Sums   []Column
}

// Fingerprint identifies a key definition, independent of company and database.
func (s KeySpec) Fingerprint() string {
	var b strings.Builder
	fmt.Fprintf(&b, "%s|%s|", version, s.Name)
	for _, c := range s.Fields {
		fmt.Fprintf(&b, "%s:%s,", c.Name, c.Kind)
	}
	b.WriteString("|")
	for _, c := range s.Sums {
		fmt.Fprintf(&b, "%s:%s,", c.Name, c.Kind)
	}
	sum := sha1.Sum([]byte(b.String()))
	return hex.EncodeToString(sum[:])
}

// BuildKey returns the SQL to create, maintain and drop the totals of one SIFT key of the
// entry table entryTable ("company$Table" or the global table name) for one database.
//
// Totals table: the key columns (NOT NULL, primary key), one column per sum field and
// cnt (number of entries). NULL key values are stored as the type's blank value (”, 0,
// false, blank date), so blank and NULL are the same. Triggers only apply differences
// (sum = sum + x): concurrent postings serialize on the totals row and none is lost; a
// rollback undoes the totals with the entry.
func BuildKey(dbType database.DBType, company, table, entryTable string, spec KeySpec) Key {
	g := generator{dbType: dbType, spec: spec, entry: entryTable, sift: TableName(company, table, spec.Name)}
	key := Key{Name: spec.Name, Fingerprint: spec.Fingerprint()}
	if dbType == database.DBTypePostgres {
		key.Create, key.Drop = g.postgres()
	} else {
		key.Create, key.Drop = g.sqlite()
	}
	return key
}

type generator struct {
	dbType database.DBType
	spec   KeySpec
	entry  string // entry table name
	sift   string // totals table name
}

func q(name string) string { return `"` + strings.ReplaceAll(name, `"`, `""`) + `"` }

func (g generator) pg() bool { return g.dbType == database.DBTypePostgres }

func (g generator) columnType(c Column) string {
	switch c.Kind {
	case KindInt:
		if g.pg() {
			return "BIGINT"
		}
		return "INTEGER"
	case KindBool:
		if g.pg() {
			return "BOOLEAN"
		}
		return "INTEGER"
	case KindDate:
		if g.pg() {
			return "TIMESTAMP" // a closing date is the date at 23:59:59
		}
		return "TEXT"
	case KindDateTime:
		if g.pg() {
			return "TIMESTAMP"
		}
		return "TEXT"
	case KindDecimal:
		if g.pg() {
			return "NUMERIC"
		}
		return "REAL"
	default:
		return "TEXT"
	}
}

// blank is the value a NULL key column is stored as.
func (g generator) blank(c Column) string {
	switch c.Kind {
	case KindInt:
		return "0"
	case KindBool:
		if g.pg() {
			return "FALSE"
		}
		return "0"
	case KindDate:
		if g.pg() {
			return "TIMESTAMP '0001-01-01 00:00:00'"
		}
		return "''"
	case KindDateTime:
		if g.pg() {
			return "TIMESTAMP '0001-01-01 00:00:00'"
		}
		return "''"
	default:
		return "''"
	}
}

// keyValue is the key value of a row (prefix "NEW." / "OLD." / "" for the entry table).
func (g generator) keyValue(c Column, prefix string) string {
	return fmt.Sprintf("COALESCE(%s%s, %s)", prefix, c.Name, g.blank(c))
}

// sumValue is the summed value of a row; SQLite stores decimals as TEXT.
func (g generator) sumValue(c Column, prefix string) string {
	if !g.pg() && c.Kind == KindDecimal {
		return fmt.Sprintf("CAST(COALESCE(%s%s, 0) AS REAL)", prefix, c.Name)
	}
	return fmt.Sprintf("COALESCE(%s%s, 0)", prefix, c.Name)
}

func (g generator) names(cols []Column) []string {
	out := make([]string, len(cols))
	for i, c := range cols {
		out[i] = c.Name
	}
	return out
}

func (g generator) createTable() string {
	var cols []string
	for _, c := range g.spec.Fields {
		cols = append(cols, fmt.Sprintf("%s %s NOT NULL", c.Name, g.columnType(c)))
	}
	for _, c := range g.spec.Sums {
		cols = append(cols, fmt.Sprintf("%s %s NOT NULL DEFAULT 0", c.Name, g.columnType(c)))
	}
	cols = append(cols, "cnt BIGINT NOT NULL DEFAULT 0")
	cols = append(cols, "PRIMARY KEY ("+strings.Join(g.names(g.spec.Fields), ", ")+")")
	create := "CREATE TABLE " + q(g.sift) + " (" + strings.Join(cols, ", ") + ")"
	if g.pg() {
		create += " WITH (" + storageOptions + ")"
	}
	return create
}

// fill adds up the existing entries once (initial build / rebuild).
func (g generator) fill() string {
	var keys, sums []string
	for _, c := range g.spec.Fields {
		keys = append(keys, g.keyValue(c, ""))
	}
	for _, c := range g.spec.Sums {
		sums = append(sums, "SUM("+g.sumValue(c, "")+")")
	}
	cols := append(g.names(g.spec.Fields), g.names(g.spec.Sums)...)
	cols = append(cols, "cnt")
	return fmt.Sprintf("INSERT INTO %s (%s) SELECT %s, %s, COUNT(*) FROM %s GROUP BY %s",
		q(g.sift), strings.Join(cols, ", "), strings.Join(keys, ", "), strings.Join(sums, ", "),
		q(g.entry), strings.Join(keys, ", "))
}

// addRow adds the row (prefix NEW.) to its totals row, creating it when needed.
func (g generator) addRow(prefix string) string {
	var keys, sums, set []string
	for _, c := range g.spec.Fields {
		keys = append(keys, g.keyValue(c, prefix))
	}
	for _, c := range g.spec.Sums {
		sums = append(sums, g.sumValue(c, prefix))
		if g.pg() {
			set = append(set, fmt.Sprintf("%s = %s.%s + EXCLUDED.%s", c.Name, q(g.sift), c.Name, c.Name))
		} else {
			set = append(set, fmt.Sprintf("%s = %s + excluded.%s", c.Name, c.Name, c.Name))
		}
	}
	if g.pg() {
		set = append(set, fmt.Sprintf("cnt = %s.cnt + 1", q(g.sift)))
	} else {
		set = append(set, "cnt = cnt + 1")
	}
	cols := append(g.names(g.spec.Fields), g.names(g.spec.Sums)...)
	cols = append(cols, "cnt")
	return fmt.Sprintf("INSERT INTO %s (%s) VALUES (%s, %s, 1) ON CONFLICT (%s) DO UPDATE SET %s",
		q(g.sift), strings.Join(cols, ", "), strings.Join(keys, ", "), strings.Join(sums, ", "),
		strings.Join(g.names(g.spec.Fields), ", "), strings.Join(set, ", "))
}

// removeRow subtracts the row (prefix OLD.) from its totals row and deletes the totals row
// when no entries are left.
func (g generator) removeRow(prefix string) []string {
	var where, set []string
	for _, c := range g.spec.Fields {
		where = append(where, fmt.Sprintf("%s = %s", c.Name, g.keyValue(c, prefix)))
	}
	for _, c := range g.spec.Sums {
		set = append(set, fmt.Sprintf("%s = %s - %s", c.Name, c.Name, g.sumValue(c, prefix)))
	}
	set = append(set, "cnt = cnt - 1")
	w := strings.Join(where, " AND ")
	return []string{
		fmt.Sprintf("UPDATE %s SET %s WHERE %s", q(g.sift), strings.Join(set, ", "), w),
		fmt.Sprintf("DELETE FROM %s WHERE %s AND cnt <= 0", q(g.sift), w),
	}
}

// changed is the condition under which an UPDATE of an entry affects its totals.
func (g generator) changed() string {
	var conds []string
	for _, c := range append(append([]Column{}, g.spec.Fields...), g.spec.Sums...) {
		if g.pg() {
			conds = append(conds, fmt.Sprintf("OLD.%s IS DISTINCT FROM NEW.%s", c.Name, c.Name))
		} else {
			conds = append(conds, fmt.Sprintf("OLD.%s IS NOT NEW.%s", c.Name, c.Name))
		}
	}
	return strings.Join(conds, " OR ")
}

func (g generator) watchedColumns() string {
	return strings.Join(append(g.names(g.spec.Fields), g.names(g.spec.Sums)...), ", ")
}

func (g generator) postgres() (create, drop []string) {
	fn := ObjectName(g.sift + "$fn")
	truncFn := ObjectName(g.sift + "$tfn")
	trgRow := ObjectName(g.sift + "$trg")
	trgUpd := ObjectName(g.sift + "$upd")
	trgTrunc := ObjectName(g.sift + "$trunc")

	remove := strings.Join(g.removeRow("OLD."), ";\n    ")
	body := fmt.Sprintf(`CREATE OR REPLACE FUNCTION %s() RETURNS trigger LANGUAGE plpgsql AS $sift$
BEGIN
  IF TG_OP = 'DELETE' OR TG_OP = 'UPDATE' THEN
    %s;
  END IF;
  IF TG_OP = 'INSERT' OR TG_OP = 'UPDATE' THEN
    %s;
  END IF;
  RETURN NULL;
END
$sift$`, q(fn), remove, g.addRow("NEW."))

	create = []string{
		g.createTable(),
		body,
		fmt.Sprintf("CREATE TRIGGER %s AFTER INSERT OR DELETE ON %s FOR EACH ROW EXECUTE FUNCTION %s()", q(trgRow), q(g.entry), q(fn)),
		fmt.Sprintf("CREATE TRIGGER %s AFTER UPDATE OF %s ON %s FOR EACH ROW WHEN (%s) EXECUTE FUNCTION %s()",
			q(trgUpd), g.watchedColumns(), q(g.entry), g.changed(), q(fn)),
		fmt.Sprintf("CREATE OR REPLACE FUNCTION %s() RETURNS trigger LANGUAGE plpgsql AS $sift$\nBEGIN\n  TRUNCATE %s;\n  RETURN NULL;\nEND\n$sift$", q(truncFn), q(g.sift)),
		fmt.Sprintf("CREATE TRIGGER %s AFTER TRUNCATE ON %s FOR EACH STATEMENT EXECUTE FUNCTION %s()", q(trgTrunc), q(g.entry), q(truncFn)),
		g.fill(),
		"ANALYZE " + q(g.sift), // statistics for the planner right away (allowed in a transaction)
	}
	drop = []string{
		fmt.Sprintf("DROP TRIGGER IF EXISTS %s ON %s", q(trgRow), q(g.entry)),
		fmt.Sprintf("DROP TRIGGER IF EXISTS %s ON %s", q(trgUpd), q(g.entry)),
		fmt.Sprintf("DROP TRIGGER IF EXISTS %s ON %s", q(trgTrunc), q(g.entry)),
		fmt.Sprintf("DROP FUNCTION IF EXISTS %s()", q(fn)),
		fmt.Sprintf("DROP FUNCTION IF EXISTS %s()", q(truncFn)),
		fmt.Sprintf("DROP TABLE IF EXISTS %s", q(g.sift)),
	}
	return create, drop
}

func (g generator) sqlite() (create, drop []string) {
	trgIns := ObjectName(g.sift + "$ins")
	trgDel := ObjectName(g.sift + "$del")
	trgUpd := ObjectName(g.sift + "$upd")
	remove := strings.Join(g.removeRow("OLD."), ";\n  ")

	create = []string{
		g.createTable(),
		fmt.Sprintf("CREATE TRIGGER %s AFTER INSERT ON %s BEGIN\n  %s;\nEND", q(trgIns), q(g.entry), g.addRow("NEW.")),
		fmt.Sprintf("CREATE TRIGGER %s AFTER DELETE ON %s BEGIN\n  %s;\nEND", q(trgDel), q(g.entry), remove),
		fmt.Sprintf("CREATE TRIGGER %s AFTER UPDATE OF %s ON %s WHEN %s BEGIN\n  %s;\n  %s;\nEND",
			q(trgUpd), g.watchedColumns(), q(g.entry), g.changed(), remove, g.addRow("NEW.")),
		g.fill(),
	}
	drop = []string{
		fmt.Sprintf("DROP TRIGGER IF EXISTS %s", q(trgIns)),
		fmt.Sprintf("DROP TRIGGER IF EXISTS %s", q(trgDel)),
		fmt.Sprintf("DROP TRIGGER IF EXISTS %s", q(trgUpd)),
		fmt.Sprintf("DROP TABLE IF EXISTS %s", q(g.sift)),
	}
	return create, drop
}

// expectedSelect is the fresh GROUP BY over the entries (what the totals must hold), with
// sums rounded on SQLite, whose REAL totals pick up float noise when maintained by deltas.
func (g generator) expectedSelect() string {
	var keys, sums []string
	for _, c := range g.spec.Fields {
		keys = append(keys, g.keyValue(c, ""))
	}
	for _, c := range g.spec.Sums {
		sums = append(sums, g.rounded("SUM("+g.sumValue(c, "")+")", c))
	}
	return fmt.Sprintf("SELECT %s, %s, COUNT(*) FROM %s GROUP BY %s",
		strings.Join(keys, ", "), strings.Join(sums, ", "), q(g.entry), strings.Join(keys, ", "))
}

// totalsSelect reads the totals table in the same column order as expectedSelect.
func (g generator) totalsSelect() string {
	var sums []string
	for _, c := range g.spec.Sums {
		sums = append(sums, g.rounded(c.Name, c))
	}
	return fmt.Sprintf("SELECT %s, %s, cnt FROM %s", strings.Join(g.names(g.spec.Fields), ", "), strings.Join(sums, ", "), q(g.sift))
}

func (g generator) rounded(expr string, c Column) string {
	if !g.pg() && c.Kind == KindDecimal {
		return "ROUND(" + expr + ", 4)"
	}
	return expr
}

// VerifyKey compares a SIFT key's totals with a fresh sum over the entries and returns the
// number of totals rows that differ (wrong sums or count, missing, or left over).
func VerifyKey(db database.Executor, dbType database.DBType, company, table, entryTable string, spec KeySpec) (int, error) {
	g := generator{dbType: dbType, spec: spec, entry: entryTable, sift: TableName(company, table, spec.Name)}
	query := fmt.Sprintf("SELECT COUNT(*) FROM (SELECT * FROM (%s EXCEPT %s) AS missing UNION ALL SELECT * FROM (%s EXCEPT %s) AS extra) AS differences",
		g.expectedSelect(), g.totalsSelect(), g.totalsSelect(), g.expectedSelect())
	var n int
	if err := db.QueryRow(query).Scan(&n); err != nil {
		return 0, err
	}
	return n, nil
}

// RebuildKey refills a SIFT key's totals from the entries in one transaction. On Postgres
// the entry table is locked against writes meanwhile, so no posting is missed or counted twice,
// and afterwards (when db is not a transaction) VACUUM ANALYZE removes the replaced rows.
func RebuildKey(db database.Executor, dbType database.DBType, company, table, entryTable string, spec KeySpec) error {
	g := generator{dbType: dbType, spec: spec, entry: entryTable, sift: TableName(company, table, spec.Name)}
	if err := inTx(db, func(tx database.Executor) error {
		if g.pg() {
			if _, err := tx.Exec("LOCK TABLE " + q(g.entry) + " IN SHARE ROW EXCLUSIVE MODE"); err != nil {
				return err
			}
		}
		if _, err := tx.Exec("DELETE FROM " + q(g.sift)); err != nil {
			return err
		}
		_, err := tx.Exec(g.fill())
		return err
	}); err != nil {
		return err
	}
	return vacuum(db, dbType, g.sift)
}

// VerifyResult is the outcome of checking one SIFT key's totals.
type VerifyResult struct {
	Table       string
	Key         string
	Differences int  // totals rows that did not match the entries
	Rebuilt     bool // the totals were rebuilt (repair)
}
