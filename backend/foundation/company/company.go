package company

import (
	"database/sql"
	"fmt"
	"strings"

	"github.com/hansjlachmann/goeasyerp/backend/foundation/database"
	"github.com/hansjlachmann/goeasyerp/backend/foundation/migrations"
)

// MigrationRunner interface for running migrations
type MigrationRunner interface {
	Run() error
}

// Manager handles company operations
type Manager struct {
	db              *database.Database
	registry        interface {
		InitializeCompanyTables(db *sql.DB, companyName string) error
		InitializeCompanyTablesWithDBType(db *sql.DB, companyName string, dbType database.DBType) error
		GetTableCount() int
	}
	migrationRunner MigrationRunner
}

// NewManager creates a new company manager
func NewManager(db *database.Database, registry interface {
	InitializeCompanyTables(db *sql.DB, companyName string) error
	InitializeCompanyTablesWithDBType(db *sql.DB, companyName string, dbType database.DBType) error
	GetTableCount() int
}) *Manager {
	return &Manager{
		db:       db,
		registry: registry,
	}
}

// NewManagerWithMigrations creates a new company manager with migration support
func NewManagerWithMigrations(db *database.Database, registry interface {
	InitializeCompanyTables(db *sql.DB, companyName string) error
	InitializeCompanyTablesWithDBType(db *sql.DB, companyName string, dbType database.DBType) error
	GetTableCount() int
}, migrationsList []migrations.Migration) *Manager {
	m := &Manager{
		db:       db,
		registry: registry,
	}

	// Create migration runner if migrations are provided
	if len(migrationsList) > 0 {
		m.migrationRunner = migrations.NewRunner(
			db.GetConnection(),
			db.GetDBType(),
			migrationsList,
			m.ListCompanies,
		)
	}

	return m
}

// CreateCompany creates a new company in the database and initializes all tables
func (m *Manager) CreateCompany(name string) error {
	if m.db.GetConnection() == nil {
		return fmt.Errorf("database not open")
	}

	// Validate company name
	if err := database.ValidateCompanyName(name); err != nil {
		return err
	}

	// Create company record
	_, err := m.db.GetConnection().Exec(`INSERT INTO "Company" (name) VALUES ($1)`, name)
	if err != nil {
		if strings.Contains(err.Error(), "duplicate key") {
			return fmt.Errorf("company '%s' already exists", name)
		}
		return fmt.Errorf("failed to create company: %w", err)
	}

	// Initialize all registered tables for this company
	if m.registry != nil {
		tableCount := m.registry.GetTableCount()
		if tableCount > 0 {
			err = m.registry.InitializeCompanyTablesWithDBType(m.db.GetConnection(), name, m.db.GetDBType())
			if err != nil {
				// Rollback: delete the company if table initialization fails
				_, _ = m.db.GetConnection().Exec(`DELETE FROM "Company" WHERE name = $1`, name)
				return fmt.Errorf("failed to initialize tables for company '%s': %w", name, err)
			}
		}
	}

	return nil
}

// EnterCompany sets the current company context (session-based)
func (m *Manager) EnterCompany(name string) error {
	if m.db.GetConnection() == nil {
		return fmt.Errorf("database not open")
	}

	if name == "" {
		return fmt.Errorf("company name cannot be empty")
	}

	// Check if already in a company
	if m.db.GetCurrentCompany() != "" {
		return fmt.Errorf("already in company '%s' - exit first before entering another", m.db.GetCurrentCompany())
	}

	// Verify company exists
	var companyName string
	err := m.db.GetConnection().QueryRow(`SELECT name FROM "Company" WHERE name = $1`, name).Scan(&companyName)
	if err == sql.ErrNoRows {
		return fmt.Errorf("company '%s' does not exist", name)
	}
	if err != nil {
		return fmt.Errorf("failed to verify company: %w", err)
	}

	// Run versioned migrations BEFORE table sync
	// This handles complex schema changes (renames, drops, data transformations)
	if m.migrationRunner != nil {
		fmt.Println("\nRunning database migrations...")
		if err := m.migrationRunner.Run(); err != nil {
			return fmt.Errorf("migration failed: %w", err)
		}
	}

	// Auto-sync: Create any missing tables for ALL companies
	// This ensures new tables are automatically created across all companies
	// This handles additive changes (new tables, new columns)
	if m.registry != nil {
		tableCount := m.registry.GetTableCount()
		if tableCount > 0 {
			// Get all companies
			companies, err := m.ListCompanies()
			if err != nil {
				return fmt.Errorf("failed to list companies for sync: %w", err)
			}

			// Sync tables for each company
			fmt.Println("\nSynchronizing tables across all companies...")
			for _, companyName := range companies {
				err = m.registry.InitializeCompanyTablesWithDBType(m.db.GetConnection(), companyName, m.db.GetDBType())
				if err != nil {
					return fmt.Errorf("failed to sync tables for company '%s': %w", companyName, err)
				}
			}
			fmt.Println()
		}
	}

	// Set current company (per-connection state)
	m.db.SetCurrentCompany(name)
	return nil
}

// ExitCompany clears the current company context
func (m *Manager) ExitCompany() error {
	if m.db.GetConnection() == nil {
		return fmt.Errorf("database not open")
	}

	if m.db.GetCurrentCompany() == "" {
		return fmt.Errorf("no company session active")
	}

	m.db.SetCurrentCompany("")
	return nil
}

// DeleteCompany deletes a company and all its Company$Tables
func (m *Manager) DeleteCompany(name string) error {
	if m.db.GetConnection() == nil {
		return fmt.Errorf("database not open")
	}

	if name == "" {
		return fmt.Errorf("company name cannot be empty")
	}

	// Verify company exists
	var companyName string
	err := m.db.GetConnection().QueryRow(`SELECT name FROM "Company" WHERE name = $1`, name).Scan(&companyName)
	if err == sql.ErrNoRows {
		return fmt.Errorf("company '%s' does not exist", name)
	}
	if err != nil {
		return fmt.Errorf("failed to verify company: %w", err)
	}

	// If this is the current company, exit it first
	if m.db.GetCurrentCompany() == name {
		m.db.SetCurrentCompany("")
	}

	// Its tables with their SIFT triggers, functions and definitions, so a company created
	// later with the same name starts empty and gets its totals built again
	if err := DropObjects(m.db.GetConnection(), m.db.GetDBType(), name); err != nil {
		return fmt.Errorf("failed to drop company tables: %w", err)
	}

	// Delete field definitions for this company
	_, err = m.db.GetConnection().Exec(`DELETE FROM "FieldDefinition" WHERE company = $1`, name)
	if err != nil {
		return fmt.Errorf("failed to delete field definitions: %w", err)
	}

	// Delete company record
	_, err = m.db.GetConnection().Exec(`DELETE FROM "Company" WHERE name = $1`, name)
	if err != nil {
		return fmt.Errorf("failed to delete company: %w", err)
	}

	return nil
}

// ListCompanies returns all companies in the database
func (m *Manager) ListCompanies() ([]string, error) {
	if m.db.GetConnection() == nil {
		return nil, fmt.Errorf("database not open")
	}

	rows, err := m.db.GetConnection().Query(`SELECT name FROM "Company" ORDER BY name`)
	if err != nil {
		return nil, fmt.Errorf("failed to list companies: %w", err)
	}
	defer rows.Close()

	var companies []string
	for rows.Next() {
		var name string
		if err := rows.Scan(&name); err != nil {
			return nil, fmt.Errorf("failed to read company name: %w", err)
		}
		companies = append(companies, name)
	}

	return companies, nil
}

// GetCurrentCompany returns the current company context
func (m *Manager) GetCurrentCompany() string {
	return m.db.GetCurrentCompany()
}
