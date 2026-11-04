package user

import (
	"database/sql"
	"fmt"
	"strings"

	"github.com/hansjlachmann/goeasyerp/backend/foundation/database"
)

// User represents a system user (global, no company prefix)
type User struct {
	Username     string
	PasswordHash string
	FullName     string
	Language     string
	Active       bool
}

// Manager handles user operations
type Manager struct {
	db *database.Database
}

// NewManager creates a new user manager
func NewManager(db *database.Database) *Manager {
	return &Manager{
		db: db,
	}
}

// InitializeUserTable creates the global User table (called once per database)
func InitializeUserTable(db *sql.DB) error {
	createSQL := `
		CREATE TABLE IF NOT EXISTS "User" (
			username TEXT PRIMARY KEY,
			password_hash TEXT NOT NULL,
			full_name TEXT,
			language TEXT DEFAULT 'en-US',
			active INTEGER DEFAULT 1
		)
	`

	_, err := db.Exec(createSQL)
	if err != nil {
		return fmt.Errorf("failed to create User table: %w", err)
	}

	return nil
}

// CreateUser creates a new user in the database
func (m *Manager) CreateUser(username, passwordHash, fullName, language string) error {
	if m.db.GetConnection() == nil {
		return fmt.Errorf("database not open")
	}

	// Validate username
	if strings.TrimSpace(username) == "" {
		return fmt.Errorf("username cannot be empty")
	}

	// Set default language if not provided
	if language == "" {
		language = "en-US"
	}

	// Create user record
	_, err := m.db.GetConnection().Exec(`
		INSERT INTO "User" (username, password_hash, full_name, language, active)
		VALUES (?, ?, ?, ?, 1)
	`, username, passwordHash, fullName, language)

	if err != nil {
		if strings.Contains(err.Error(), "UNIQUE constraint failed") || strings.Contains(err.Error(), "duplicate key") {
			return fmt.Errorf("user '%s' already exists", username)
		}
		return fmt.Errorf("failed to create user: %w", err)
	}

	return nil
}

// GetUser retrieves a user by username
func (m *Manager) GetUser(username string) (*User, error) {
	if m.db.GetConnection() == nil {
		return nil, fmt.Errorf("database not open")
	}

	var user User
	var active int

	err := m.db.GetConnection().QueryRow(`
		SELECT username, password_hash, full_name, language, active
		FROM "User"
		WHERE username = ?
	`, username).Scan(&user.Username, &user.PasswordHash, &user.FullName, &user.Language, &active)

	if err == sql.ErrNoRows {
		return nil, fmt.Errorf("user '%s' not found", username)
	}
	if err != nil {
		return nil, fmt.Errorf("failed to get user: %w", err)
	}

	user.Active = active != 0

	return &user, nil
}

// ListUsers retrieves all users
func (m *Manager) ListUsers() ([]*User, error) {
	if m.db.GetConnection() == nil {
		return nil, fmt.Errorf("database not open")
	}

	rows, err := m.db.GetConnection().Query(`
		SELECT username, password_hash, full_name, language, active
		FROM "User"
		ORDER BY username
	`)
	if err != nil {
		return nil, fmt.Errorf("failed to list users: %w", err)
	}
	defer rows.Close()

	var users []*User
	for rows.Next() {
		var user User
		var active int

		if err := rows.Scan(&user.Username, &user.PasswordHash, &user.FullName, &user.Language, &active); err != nil {
			return nil, fmt.Errorf("failed to scan user: %w", err)
		}

		user.Active = active != 0
		users = append(users, &user)
	}

	return users, nil
}

// UpdateUser updates user information
func (m *Manager) UpdateUser(username, fullName, language string, active bool) error {
	if m.db.GetConnection() == nil {
		return fmt.Errorf("database not open")
	}

	activeInt := 0
	if active {
		activeInt = 1
	}

	result, err := m.db.GetConnection().Exec(`
		UPDATE "User"
		SET full_name = ?, language = ?, active = ?
		WHERE username = ?
	`, fullName, language, activeInt, username)

	if err != nil {
		return fmt.Errorf("failed to update user: %w", err)
	}

	rowsAffected, _ := result.RowsAffected()
	if rowsAffected == 0 {
		return fmt.Errorf("user '%s' not found", username)
	}

	return nil
}

// DeleteUser deletes a user
func (m *Manager) DeleteUser(username string) error {
	if m.db.GetConnection() == nil {
		return fmt.Errorf("database not open")
	}

	result, err := m.db.GetConnection().Exec(`DELETE FROM "User" WHERE username = ?`, username)
	if err != nil {
		return fmt.Errorf("failed to delete user: %w", err)
	}

	rowsAffected, _ := result.RowsAffected()
	if rowsAffected == 0 {
		return fmt.Errorf("user '%s' not found", username)
	}

	return nil
}

// ValidateCredentials checks if username and password match
func (m *Manager) ValidateCredentials(username, passwordHash string) (*User, error) {
	user, err := m.GetUser(username)
	if err != nil {
		return nil, err
	}

	if !user.Active {
		return nil, fmt.Errorf("user '%s' is inactive", username)
	}

	if user.PasswordHash != passwordHash {
		return nil, fmt.Errorf("invalid credentials")
	}

	return user, nil
}
