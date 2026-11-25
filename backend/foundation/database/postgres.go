package database

import (
	"database/sql"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	_ "github.com/lib/pq"
)

// The server's connection pool (S31): at most DefaultMaxConns connections at once (DB_MAX_CONNS),
// so a burst of requests or long jobs waits for a free connection instead of using up
// PostgreSQL's max_connections (100 by default) — which would refuse everybody, the scheduler,
// the app builder, backups and psql included. Idle connections are kept for a while and closed
// after IdleConnTime.
const (
	DefaultMaxConns  = 40
	DefaultIdleConns = 10
	IdleConnTime     = 5 * time.Minute
)

// MaxConnsFromEnv: DB_MAX_CONNS when it is a whole number from 2 up, else DefaultMaxConns (a
// pool of one connection would hang a request that opens a transaction and reads beside it)
func MaxConnsFromEnv() int {
	n, err := strconv.Atoi(os.Getenv("DB_MAX_CONNS"))
	if err != nil || n < 2 {
		return DefaultMaxConns
	}
	return n
}

// limitPool sets the pool limits on a connection the server works with
func limitPool(conn *sql.DB) {
	maxConns := MaxConnsFromEnv()
	conn.SetMaxOpenConns(maxConns)
	conn.SetMaxIdleConns(min(DefaultIdleConns, maxConns))
	conn.SetConnMaxIdleTime(IdleConnTime)
}

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

// PostgresConfigFromEnv: the PostgreSQL settings of the environment (DB_HOST — empty: no
// PostgreSQL —, DB_PORT, DB_USER, DB_PASSWORD, DB_NAME, DB_SSLMODE), as the server and the app
// builder read them
func PostgresConfigFromEnv() (*PostgresConfig, bool) {
	host := os.Getenv("DB_HOST")
	if host == "" {
		return nil, false
	}
	get := func(key, def string) string {
		value := os.Getenv(key)
		if value == "" {
			return def
		}
		return value
	}
	return &PostgresConfig{
		Host:     host,
		Port:     get("DB_PORT", "5432"),
		User:     get("DB_USER", "openerp"),
		Password: get("DB_PASSWORD", "openerp"),
		DBName:   get("DB_NAME", "openerp"),
		SSLMode:  get("DB_SSLMODE", "disable"),
	}, true
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
	limitPool(conn)

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
	limitPool(conn)

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

// knownPasswords are database passwords anyone can read in the repository's compose files and
// docs (S6)
var knownPasswords = []string{"openerp", "postgres", "password", "changeme"}

// PasswordRefusal: why the server refuses to start when its database password (DB_PASSWORD) is
// one anyone knows (S6) — "" when it is its own, without a database server (SQLite), or on a
// development database (DB_KEEP_SUPERUSER, whose password the tests use)
func PasswordRefusal() string {
	if os.Getenv("DB_HOST") == "" || os.Getenv(EnvKeepSuperuser) == "true" {
		return ""
	}
	return passwordRefusal(os.Getenv("DB_PASSWORD"))
}

func passwordRefusal(password string) string {
	password = strings.TrimSpace(password)
	if password == "" {
		password = "openerp" // PostgresConfigFromEnv's default
	}
	for _, known := range knownPasswords {
		if strings.EqualFold(password, known) {
			return fmt.Sprintf("the database password (DB_PASSWORD) is %q, which everyone can read in GoEasyERP's files — anyone who reaches the database could read and change all data. Give the database a password of its own and set it in DB_PASSWORD (POSTGRES_PASSWORD in prod.env): docs/operations.md, \"Changing the database password\". The one-computer trial's install script does it by itself.", known)
		}
	}
	return ""
}
