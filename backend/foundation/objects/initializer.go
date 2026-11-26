package objects

import (
	"database/sql"
	"fmt"
	"regexp"
	"sort"
	"strings"

	"github.com/hansjlachmann/goeasyerp/backend/foundation/database"
	ftables "github.com/hansjlachmann/goeasyerp/backend/foundation/tables"
)

// TableDefinition represents a table that can be initialized
type TableDefinition interface {
	GetTableID() int
	GetTableName() string
	GetTableSchema() string
	GetPostgresTableSchema() string
	CreateTable(db database.Executor, company string) error
	CreateTableWithDBType(db database.Executor, company string, dbType database.DBType) error
}

// KeySyncer is implemented by generated tables: SyncKeys creates missing indexes and
// builds, rebuilds or drops SIFT totals (keys with sum_index_fields).
type KeySyncer interface {
	SyncKeys(db database.Executor, company string, dbType database.DBType) error
}

// getSchemaForDBType returns the appropriate schema based on database type
func getSchemaForDBType(tableDef TableDefinition, dbType database.DBType) string {
	if dbType == database.DBTypePostgres {
		return tableDef.GetPostgresTableSchema()
	}
	return tableDef.GetTableSchema()
}

// InitializeCompanyTables creates all registered tables for a new company
// Also performs schema migration by adding missing columns to existing tables
func (or *ObjectRegistry) InitializeCompanyTables(db *sql.DB, companyName string) error {
	return or.InitializeCompanyTablesWithDBType(db, companyName, database.DBTypeSQLite)
}

// InitializeCompanyTablesWithDBType creates all registered tables for a new company with the given database type
// Also performs schema migration by adding missing columns to existing tables
func (or *ObjectRegistry) InitializeCompanyTablesWithDBType(db *sql.DB, companyName string, dbType database.DBType) error {
	// Get all registered table IDs
	tableIDs := or.ListTables()

	if len(tableIDs) == 0 {
		return fmt.Errorf("no tables registered in object registry")
	}

	if err := ensureFieldDefinitionTable(db); err != nil {
		return fmt.Errorf("field numbers: %w", err)
	}

	successCount := 0
	migratedCount := 0
	failedTables := []string{}

	// Create each registered table
	for _, tableID := range tableIDs {
		tableInterface, ok := or.GetTable(tableID)
		if !ok {
			continue
		}

		// Type assert to TableDefinition
		tableDef, ok := tableInterface.(TableDefinition)
		if !ok {
			// Skip if table doesn't implement TableDefinition
			// This allows us to have table structs registered that don't need initialization
			continue
		}

		// Build full table name (Company$TableName)
		fullTableName := ftables.CompanyTable(companyName, tableDef.GetTableName())

		// Check if table exists (company-scoped: "company$Table")
		exists, err := tableExists(db, fullTableName, dbType)
		if err != nil {
			failedTables = append(failedTables, fmt.Sprintf("Table %d (%s): failed to check existence: %v", tableID, tableDef.GetTableName(), err))
			continue
		}

		// If company-prefixed table doesn't exist, check for global table (no prefix)
		// Global tables (User, User_Role, etc.) use just the table name without company prefix
		if !exists {
			globalExists, globalErr := tableExists(db, tableDef.GetTableName(), dbType)
			if globalErr != nil {
				failedTables = append(failedTables, fmt.Sprintf("Table %d (%s): failed to check global existence: %v", tableID, tableDef.GetTableName(), globalErr))
				continue
			}
			if globalExists {
				exists = true
				fullTableName = tableDef.GetTableName() // Use non-prefixed name for schema sync
			}
		}

		if exists {
			// Table exists - perform schema migration (add missing columns)
			schema := getSchemaForDBType(tableDef, dbType)
			schemaColumns := parseSchemaColumns(schema)
			existingColumns, err := getTableColumns(db, fullTableName, dbType)
			if err != nil {
				failedTables = append(failedTables, fmt.Sprintf("Table %d (%s): failed to get columns: %v", tableID, tableDef.GetTableName(), err))
				continue
			}

			// Fields renamed in the YAML keep their number: rename their column first, so
			// it is not added again as a new, empty column
			numbered, hasNumbers := tableInterface.(fieldNumberer)
			if hasNumbers {
				if err := renameByFieldNumber(db, dbType, fullTableName, tableDef.GetTableName(), storedColumns(numbered), existingColumns); err != nil {
					failedTables = append(failedTables, fmt.Sprintf("Table %d (%s): %v", tableID, tableDef.GetTableName(), err))
					continue
				}
			}

			// Add missing columns
			err = addMissingColumns(db, fullTableName, schemaColumns, existingColumns, dbType)
			if err != nil {
				failedTables = append(failedTables, fmt.Sprintf("Table %d (%s): migration failed: %v", tableID, tableDef.GetTableName(), err))
				continue
			}

			// Also fix any existing NULL values in TEXT/INTEGER columns
			var fieldTypes map[string]ftables.FieldType
			if hasNumbers {
				fieldTypes = fieldTypesOf(numbered)
			}
			err = fixNullValues(db, fullTableName, schemaColumns, existingColumns, fieldTypes)
			if err != nil {
				failedTables = append(failedTables, fmt.Sprintf("Table %d (%s): failed to fix NULL values: %v", tableID, tableDef.GetTableName(), err))
				continue
			}

			// Keys added later (indexes) and SIFT totals (sum_index_fields): after the column
			// sync, so a newly added sum field's column exists. New tables get them from
			// CreateTableWithDBType.
			if ks, ok := tableInterface.(KeySyncer); ok {
				if err := ks.SyncKeys(db, companyName, dbType); err != nil {
					failedTables = append(failedTables, fmt.Sprintf("Table %d (%s): key sync failed: %v", tableID, tableDef.GetTableName(), err))
					continue
				}
			}

			if hasNumbers {
				if err := recordFieldNumbers(db, dbType, fullTableName, storedColumns(numbered)); err != nil {
					failedTables = append(failedTables, fmt.Sprintf("Table %d (%s): %v", tableID, tableDef.GetTableName(), err))
					continue
				}
			}

			migratedCount++
		} else {
			// Table doesn't exist - create it
			err := tableDef.CreateTableWithDBType(db, companyName, dbType)
			if err != nil {
				failedTables = append(failedTables, fmt.Sprintf("Table %d (%s): %v", tableID, tableDef.GetTableName(), err))
				continue
			}
			if numbered, ok := tableInterface.(fieldNumberer); ok {
				physical := fullTableName
				if g, ok := tableInterface.(interface{ IsGlobal() bool }); ok && g.IsGlobal() {
					physical = tableDef.GetTableName()
				}
				if err := recordFieldNumbers(db, dbType, physical, storedColumns(numbered)); err != nil {
					failedTables = append(failedTables, fmt.Sprintf("Table %d (%s): %v", tableID, tableDef.GetTableName(), err))
					continue
				}
			}

			successCount++
		}
	}

	if len(failedTables) > 0 {
		errorMsg := fmt.Sprintf("Failed to initialize %d table(s):\n", len(failedTables))
		for _, failure := range failedTables {
			errorMsg += "  - " + failure + "\n"
		}
		return fmt.Errorf("%s", errorMsg)
	}

	if successCount == 0 && migratedCount == 0 {
		return fmt.Errorf("no tables were initialized (registered tables may not implement TableDefinition interface)")
	}

	if successCount > 0 {
		fmt.Printf("✓ Created %d new table(s)\n", successCount)
	}
	if migratedCount > 0 {
		fmt.Printf("✓ Migrated %d existing table(s)\n", migratedCount)
	}

	return nil
}

// fixNullValues updates NULL values in existing columns to the blank value of their type. The
// type is the field's (fieldTypes: the table definition), else the column's declared type —
// never the column's name (S15: "realized_gains_acc TEXT" is no REAL column, and on SQLite a
// date is a TEXT column whose NULL is the blank date and must stay NULL). Dates, date-times,
// blobs and unknown types keep their NULLs.
func fixNullValues(db *sql.DB, tableName string, schemaColumns map[string]string, existingColumns map[string]bool, fieldTypes map[string]ftables.FieldType) error {
	names := make([]string, 0, len(schemaColumns))
	for columnName := range schemaColumns {
		names = append(names, columnName)
	}
	sort.Strings(names)
	for _, columnName := range names {
		if !existingColumns[columnName] {
			continue // the column does not exist yet
		}
		defaultValue := blankValue(columnName, schemaColumns[columnName], fieldTypes)
		if defaultValue == "" {
			continue
		}
		updateSQL := fmt.Sprintf(`UPDATE "%s" SET "%s" = %s WHERE "%s" IS NULL`, tableName, columnName, defaultValue, columnName)
		_, err := db.Exec(updateSQL)
		if err != nil {
			return fmt.Errorf("failed to fix NULL values for column %s: %w", columnName, err)
		}
	}
	return nil
}

// blankValue: the SQL blank value of a column ("" to leave its NULLs alone)
func blankValue(columnName, columnDef string, fieldTypes map[string]ftables.FieldType) string {
	if fieldType, ok := fieldTypes[columnName]; ok {
		switch fieldType {
		case ftables.FieldTypeBoolean:
			return "FALSE"
		case ftables.FieldTypeInteger, ftables.FieldTypeOption, ftables.FieldTypeDecimal:
			return "0"
		case ftables.FieldTypeCode, ftables.FieldTypeText:
			return "''"
		default: // dates, date-times (NULL is the blank value), blobs
			return ""
		}
	}
	// no field information: the declared type (the word after the column's name)
	parts := strings.Fields(strings.ToUpper(columnDef))
	if len(parts) < 2 {
		return ""
	}
	declared := parts[1]
	if k := strings.IndexByte(declared, '('); k >= 0 {
		declared = declared[:k]
	}
	switch declared {
	case "BOOLEAN", "BOOL":
		return "FALSE"
	case "INTEGER", "INT", "BIGINT", "SMALLINT", "NUMERIC", "DECIMAL", "DOUBLE", "REAL":
		return "0"
	case "VARCHAR":
		return "''"
	default: // TEXT (also a date on SQLite), TIMESTAMP, DATE, BLOB, BYTEA …
		return ""
	}
}

// fieldTypesOf: the stored fields' types by column
func fieldTypesOf(t fieldNumberer) map[string]ftables.FieldType {
	out := map[string]ftables.FieldType{}
	for _, f := range t.GetFields() {
		if !f.FlowField {
			out[strings.ToLower(f.Name)] = f.Type
		}
	}
	return out
}

// GetTableCount returns the number of registered tables
func (or *ObjectRegistry) GetTableCount() int {
	return len(or.tables)
}

// ========================================
// Schema Migration Helpers
// ========================================

// tableExists checks if a table exists in the database
func tableExists(db *sql.DB, tableName string, dbType database.DBType) (bool, error) {
	var name string
	var err error

	switch dbType {
	case database.DBTypePostgres:
		err = db.QueryRow(`
			SELECT table_name FROM information_schema.tables
			WHERE table_schema = 'public' AND table_name = $1
		`, tableName).Scan(&name)
	default: // SQLite
		err = db.QueryRow(`
			SELECT name FROM sqlite_master
			WHERE type='table' AND name=?
		`, tableName).Scan(&name)
	}

	if err == sql.ErrNoRows {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	return true, nil
}

// getTableColumns returns a map of existing column names in the table
func getTableColumns(db *sql.DB, tableName string, dbType database.DBType) (map[string]bool, error) {
	columns := make(map[string]bool)

	switch dbType {
	case database.DBTypePostgres:
		rows, err := db.Query(`
			SELECT column_name FROM information_schema.columns
			WHERE table_schema = 'public' AND table_name = $1
		`, tableName)
		if err != nil {
			return nil, fmt.Errorf("failed to get table columns: %w", err)
		}
		defer rows.Close()

		for rows.Next() {
			var name string
			if err := rows.Scan(&name); err != nil {
				return nil, fmt.Errorf("failed to scan column name: %w", err)
			}
			columns[strings.ToLower(name)] = true
		}
	default: // SQLite
		rows, err := db.Query(fmt.Sprintf(`PRAGMA table_info("%s")`, tableName))
		if err != nil {
			return nil, fmt.Errorf("failed to get table info: %w", err)
		}
		defer rows.Close()

		for rows.Next() {
			var cid int
			var name string
			var dataType string
			var notNull int
			var defaultValue sql.NullString
			var pk int

			err := rows.Scan(&cid, &name, &dataType, &notNull, &defaultValue, &pk)
			if err != nil {
				return nil, fmt.Errorf("failed to scan column info: %w", err)
			}
			columns[strings.ToLower(name)] = true
		}
	}

	return columns, nil
}

// parseSchemaColumns extracts column definitions from CREATE TABLE schema
// Returns map[columnName]columnDefinition
func parseSchemaColumns(schema string) map[string]string {
	columns := make(map[string]string)

	// Remove leading/trailing whitespace and newlines
	schema = strings.TrimSpace(schema)

	// Split by commas (but not commas inside parentheses)
	lines := strings.Split(schema, "\n")

	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}

		// Remove trailing comma
		line = strings.TrimSuffix(line, ",")

		// Extract column name (first word)
		parts := strings.Fields(line)
		if len(parts) >= 2 {
			columnName := strings.ToLower(parts[0])
			columnDef := line

			// Skip PRIMARY KEY constraints (they're on the original column)
			if strings.Contains(strings.ToUpper(line), "PRIMARY KEY") && !strings.Contains(strings.ToUpper(parts[0]), "PRIMARY") {
				// This is an inline PRIMARY KEY definition
				columnDef = line
			}

			columns[columnName] = columnDef
		}
	}

	return columns
}

// addMissingColumns adds columns that exist in schema but not in table
func addMissingColumns(db *sql.DB, tableName string, schemaColumns map[string]string, existingColumns map[string]bool, dbType database.DBType) error {
	addedCount := 0

	for columnName, columnDef := range schemaColumns {
		// Skip if column already exists
		if existingColumns[columnName] {
			continue
		}

		// Skip PRIMARY KEY constraint lines
		if strings.Contains(strings.ToUpper(columnDef), "PRIMARY KEY") && !strings.Contains(columnDef, columnName) {
			continue
		}

		// Remove PRIMARY KEY from column definition for ALTER TABLE
		// SQLite doesn't allow adding PRIMARY KEY columns via ALTER TABLE
		cleanDef := columnDef
		re := regexp.MustCompile(`(?i)\s+PRIMARY KEY`)
		cleanDef = re.ReplaceAllString(cleanDef, "")

		// Add DEFAULT value to prevent NULL values in existing rows
		// This prevents "converting NULL to string is unsupported" errors
		upperDef := strings.ToUpper(cleanDef)
		if !strings.Contains(upperDef, "DEFAULT") {
			// Determine default value based on type
			switch {
			case strings.Contains(upperDef, "BOOLEAN"):
				cleanDef = cleanDef + " DEFAULT FALSE"
			case strings.Contains(upperDef, "INTEGER"):
				cleanDef = cleanDef + " DEFAULT 0"
			case strings.Contains(upperDef, "NUMERIC") || strings.Contains(upperDef, "DECIMAL") || strings.Contains(upperDef, "DOUBLE") || strings.Contains(upperDef, "REAL"):
				cleanDef = cleanDef + " DEFAULT 0"
			case strings.Contains(upperDef, "TEXT") || strings.Contains(upperDef, "VARCHAR"):
				cleanDef = cleanDef + " DEFAULT ''"
				// TIMESTAMP and DATE don't get defaults, allowing NULL
			}
		}

		// Execute ALTER TABLE ADD COLUMN
		var alterSQL string
		if dbType == database.DBTypePostgres {
			// PostgreSQL uses ADD COLUMN without the COLUMN keyword issue
			alterSQL = fmt.Sprintf(`ALTER TABLE "%s" ADD COLUMN %s`, tableName, cleanDef)
		} else {
			alterSQL = fmt.Sprintf(`ALTER TABLE "%s" ADD COLUMN %s`, tableName, cleanDef)
		}
		_, err := db.Exec(alterSQL)
		if err != nil {
			return fmt.Errorf("failed to add column %s: %w", columnName, err)
		}

		// Update existing rows to set default value (ALTER TABLE DEFAULT doesn't update existing rows)
		var defaultValue string
		switch {
		case strings.Contains(upperDef, "BOOLEAN"):
			defaultValue = "FALSE"
		case strings.Contains(upperDef, "INTEGER"):
			defaultValue = "0"
		case strings.Contains(upperDef, "NUMERIC") || strings.Contains(upperDef, "DECIMAL") || strings.Contains(upperDef, "DOUBLE") || strings.Contains(upperDef, "REAL"):
			defaultValue = "0"
		case strings.Contains(upperDef, "TEXT") || strings.Contains(upperDef, "VARCHAR"):
			defaultValue = "''"
			// TIMESTAMP and DATE don't get default values
		}

		if defaultValue != "" {
			updateSQL := fmt.Sprintf(`UPDATE "%s" SET %s = %s WHERE %s IS NULL`, tableName, columnName, defaultValue, columnName)
			_, err = db.Exec(updateSQL)
			if err != nil {
				return fmt.Errorf("failed to update default values for column %s: %w", columnName, err)
			}
		}

		fmt.Printf("  + Added column: %s.%s\n", tableName, columnName)
		addedCount++
	}

	if addedCount > 0 {
		fmt.Printf("  ✓ Added %d column(s) to %s\n", addedCount, tableName)
	}

	return nil
}
