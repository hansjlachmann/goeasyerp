package database

import (
	"database/sql"
	"fmt"
	"strings"

	"github.com/mattn/go-sqlite3"
)

// SQLiteDriver is the driver name the application opens SQLite with: go-sqlite3 with a
// Unicode lower(). SQLite's built-in lower() only folds ASCII letters, so the list search
// (LOWER(column) LIKE '%text%', text lowered in Go) did not find "Ørsta" when typing "ø".
const SQLiteDriver = "sqlite3_openerp"

func init() {
	sql.Register(SQLiteDriver, &sqlite3.SQLiteDriver{
		ConnectHook: func(conn *sqlite3.SQLiteConn) error {
			return conn.RegisterFunc("lower", unicodeLower, true)
		},
	})
}

// unicodeLower is lower() for SQLite: Unicode case folding like Go's strings.ToLower (and
// Postgres' LOWER); NULL stays NULL, other values are lowered as their text.
func unicodeLower(v any) any {
	switch s := v.(type) {
	case string:
		return strings.ToLower(s)
	case []byte:
		if s == nil { // NULL
			return nil
		}
		return strings.ToLower(string(s))
	default:
		return strings.ToLower(fmt.Sprint(s))
	}
}

// DBType represents the type of database backend
type DBType string

const (
	DBTypeSQLite   DBType = "sqlite"
	DBTypePostgres DBType = "postgres"
)

// Database represents a database connection with session state
type Database struct {
	conn           *sql.DB
	dbType         DBType
	connString     string // connection string (path for SQLite, connection URL for Postgres)
	currentCompany string // Per-connection state for thread safety
}

// CreateDatabase creates a new SQLite database file
func CreateDatabase(path string) (*Database, error) {
	conn, err := sql.Open(SQLiteDriver, path)
	if err != nil {
		return nil, fmt.Errorf("failed to create database: %w", err)
	}

	// Enable foreign keys
	if _, err := conn.Exec("PRAGMA foreign_keys = ON"); err != nil {
		conn.Close()
		return nil, fmt.Errorf("failed to enable foreign keys: %w", err)
	}

	// Create Company table
	_, err = conn.Exec(`
		CREATE TABLE IF NOT EXISTS "Company" (
			name TEXT PRIMARY KEY,
			created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
		)
	`)
	if err != nil {
		conn.Close()
		return nil, fmt.Errorf("failed to create Company table: %w", err)
	}

	// Create User table (global, no company prefix)
	_, err = conn.Exec(`
		CREATE TABLE IF NOT EXISTS "User" (
			username TEXT PRIMARY KEY,
			password_hash TEXT NOT NULL,
			full_name TEXT,
			language TEXT DEFAULT 'en-US',
			active INTEGER DEFAULT 1
		)
	`)
	if err != nil {
		conn.Close()
		return nil, fmt.Errorf("failed to create User table: %w", err)
	}

	db := &Database{
		conn:           conn,
		dbType:         DBTypeSQLite,
		connString:     path,
		currentCompany: "",
	}

	return db, nil
}

// OpenDatabase opens an existing SQLite database file
func OpenDatabase(path string) (*Database, error) {
	conn, err := sql.Open(SQLiteDriver, path)
	if err != nil {
		return nil, fmt.Errorf("failed to open database: %w", err)
	}

	// Enable foreign keys
	if _, err := conn.Exec("PRAGMA foreign_keys = ON"); err != nil {
		conn.Close()
		return nil, fmt.Errorf("failed to enable foreign keys: %w", err)
	}

	// Verify Company table exists
	var tableName string
	err = conn.QueryRow("SELECT name FROM sqlite_master WHERE type='table' AND name='Company'").Scan(&tableName)
	if err == sql.ErrNoRows {
		conn.Close()
		return nil, fmt.Errorf("not a valid GoEasyERP database: Company table not found")
	}
	if err != nil {
		conn.Close()
		return nil, fmt.Errorf("failed to verify database: %w", err)
	}

	// Create User table if it doesn't exist (for database upgrade compatibility)
	_, err = conn.Exec(`
		CREATE TABLE IF NOT EXISTS "User" (
			username TEXT PRIMARY KEY,
			password_hash TEXT NOT NULL,
			full_name TEXT,
			language TEXT DEFAULT 'en-US',
			active INTEGER DEFAULT 1
		)
	`)
	if err != nil {
		conn.Close()
		return nil, fmt.Errorf("failed to create User table: %w", err)
	}

	db := &Database{
		conn:           conn,
		dbType:         DBTypeSQLite,
		connString:     path,
		currentCompany: "",
	}

	return db, nil
}

// CloseDatabase closes the database connection
func (db *Database) CloseDatabase() error {
	if db.conn == nil {
		return fmt.Errorf("database already closed")
	}

	// Exit company session if active
	if db.currentCompany != "" {
		db.currentCompany = ""
	}

	err := db.conn.Close()
	if err != nil {
		return fmt.Errorf("failed to close database: %w", err)
	}

	db.conn = nil
	return nil
}

// WrapConnection wraps an existing *sql.DB and DBType into a Database instance.
// Used by the auth middleware to create session-scoped Database wrappers.
func WrapConnection(conn *sql.DB, dbType DBType) *Database {
	return &Database{
		conn:   conn,
		dbType: dbType,
	}
}

// GetConnection returns the underlying database connection
func (db *Database) GetConnection() *sql.DB {
	return db.conn
}

// GetCurrentCompany returns the current company context
func (db *Database) GetCurrentCompany() string {
	return db.currentCompany
}

// SetCurrentCompany sets the current company context (internal use)
func (db *Database) SetCurrentCompany(company string) {
	db.currentCompany = company
}

// GetDatabasePath returns the database connection string (path for SQLite, connection URL for Postgres)
func (db *Database) GetDatabasePath() string {
	return db.connString
}

// GetDBType returns the database type (sqlite or postgres)
func (db *Database) GetDBType() DBType {
	return db.dbType
}

// GetFullTableName returns the company-prefixed table name
// Format: CompanyName$TableName (classic ERP style)
func (db *Database) GetFullTableName(tableName string) (string, error) {
	if db.currentCompany == "" {
		return "", fmt.Errorf("no company context set - use EnterCompany() first")
	}
	return fmt.Sprintf("%s$%s", db.currentCompany, tableName), nil
}

// TableExists checks if a table exists in the database
func (db *Database) TableExists(tableName string) (bool, error) {
	if db.conn == nil {
		return false, fmt.Errorf("database not open")
	}

	var name string
	var err error

	switch db.dbType {
	case DBTypePostgres:
		err = db.conn.QueryRow(`
			SELECT table_name FROM information_schema.tables
			WHERE table_schema = 'public' AND table_name = $1
		`, tableName).Scan(&name)
	default: // SQLite
		err = db.conn.QueryRow(`
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

// CreateTable creates a new table with the given schema
func (db *Database) CreateTable(tableName, schema string) error {
	if db.conn == nil {
		return fmt.Errorf("database not open")
	}

	if db.currentCompany == "" {
		return fmt.Errorf("no company context set - use EnterCompany() first")
	}

	fullTableName, err := db.GetFullTableName(tableName)
	if err != nil {
		return err
	}

	// Check if table already exists
	exists, err := db.TableExists(fullTableName)
	if err != nil {
		return err
	}

	if exists {
		return fmt.Errorf("table %s already exists", fullTableName)
	}

	// Create table
	createSQL := fmt.Sprintf(`CREATE TABLE "%s" (%s)`, fullTableName, schema)
	_, err = db.conn.Exec(createSQL)
	if err != nil {
		return fmt.Errorf("failed to create table: %w", err)
	}

	return nil
}

// DropTable drops a table from the database
func (db *Database) DropTable(tableName string) error {
	if db.conn == nil {
		return fmt.Errorf("database not open")
	}

	if db.currentCompany == "" {
		return fmt.Errorf("no company context set - use EnterCompany() first")
	}

	fullTableName, err := db.GetFullTableName(tableName)
	if err != nil {
		return err
	}

	_, err = db.conn.Exec(fmt.Sprintf(`DROP TABLE IF EXISTS "%s"`, fullTableName))
	return err
}

// ValidateCompanyName checks if a company name is valid
func ValidateCompanyName(name string) error {
	if name == "" {
		return fmt.Errorf("company name cannot be empty")
	}

	// Validate company name (no special characters that could break table names)
	if strings.ContainsAny(name, " $\"'`\\;-") {
		return fmt.Errorf("company name contains invalid characters (spaces, $, quotes, backslashes, semicolons, or hyphens not allowed)")
	}

	return nil
}
