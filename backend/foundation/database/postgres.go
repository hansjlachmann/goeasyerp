package database

import (
	"database/sql"
	"fmt"

	_ "github.com/lib/pq"
)

// PostgresConfig holds PostgreSQL connection configuration
type PostgresConfig struct {
	Host     string
	Port     string
	User     string
	Password string
	DBName   string
	SSLMode  string
}

// DefaultPostgresConfig returns a default PostgreSQL configuration
func DefaultPostgresConfig() *PostgresConfig {
	return &PostgresConfig{
		Host:     "localhost",
		Port:     "5432",
		User:     "openerp",
		Password: "openerp",
		DBName:   "openerp",
		SSLMode:  "disable",
	}
}

// ConnectionString returns the PostgreSQL connection string
func (c *PostgresConfig) ConnectionString() string {
	return fmt.Sprintf(
		"host=%s port=%s user=%s password=%s dbname=%s sslmode=%s",
		c.Host, c.Port, c.User, c.Password, c.DBName, c.SSLMode,
	)
}

// CreatePostgresDatabase creates a new PostgreSQL database connection and initializes schema
func CreatePostgresDatabase(config *PostgresConfig) (*Database, error) {
	connString := config.ConnectionString()
	conn, err := sql.Open("postgres", connString)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to PostgreSQL: %w", err)
	}

	// Test the connection
	if err := conn.Ping(); err != nil {
		conn.Close()
		return nil, fmt.Errorf("failed to ping PostgreSQL: %w", err)
	}

	// Create Company table
	_, err = conn.Exec(`
		CREATE TABLE IF NOT EXISTS "Company" (
			name VARCHAR(100) PRIMARY KEY,
			created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
		)
	`)
	if err != nil {
		conn.Close()
		return nil, fmt.Errorf("failed to create Company table: %w", err)
	}

	// Note: User table is now created by the object registry when company tables are initialized
	// This is because User has global: true in the YAML definition

	db := &Database{
		conn:           conn,
		dbType:         DBTypePostgres,
		connString:     connString,
		currentCompany: "",
	}

	return db, nil
}

// OpenPostgresDatabase opens an existing PostgreSQL database connection
func OpenPostgresDatabase(config *PostgresConfig) (*Database, error) {
	connString := config.ConnectionString()
	conn, err := sql.Open("postgres", connString)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to PostgreSQL: %w", err)
	}

	// Test the connection
	if err := conn.Ping(); err != nil {
		conn.Close()
		return nil, fmt.Errorf("failed to ping PostgreSQL: %w", err)
	}

	// Verify Company table exists
	var tableName string
	err = conn.QueryRow(`
		SELECT table_name FROM information_schema.tables
		WHERE table_schema = 'public' AND table_name = 'Company'
	`).Scan(&tableName)
	if err == sql.ErrNoRows {
		conn.Close()
		return nil, fmt.Errorf("not a valid GoEasyERP database: Company table not found")
	}
	if err != nil {
		conn.Close()
		return nil, fmt.Errorf("failed to verify database: %w", err)
	}

	// Note: User table is now created by the object registry when company tables are initialized
	// This is because User has global: true in the YAML definition

	db := &Database{
		conn:           conn,
		dbType:         DBTypePostgres,
		connString:     connString,
		currentCompany: "",
	}

	return db, nil
}

// CreateOrOpenPostgresDatabase creates or opens a PostgreSQL database
// It first tries to open, and if Company table doesn't exist, creates the schema
func CreateOrOpenPostgresDatabase(config *PostgresConfig) (*Database, error) {
	db, err := OpenPostgresDatabase(config)
	if err == nil {
		return db, nil
	}

	// If open failed, try to create
	return CreatePostgresDatabase(config)
}
