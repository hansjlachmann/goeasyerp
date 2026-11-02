package session

import (
	"bufio"
	"database/sql"
	"fmt"
	"strings"
	"time"

	"github.com/hansjlachmann/openerp/backend/foundation/database"
	"github.com/hansjlachmann/openerp/backend/foundation/tables"
)

// PermissionOperation represents a CRUD operation type
type PermissionOperation string

const (
	PermRead   PermissionOperation = "read"
	PermInsert PermissionOperation = "insert"
	PermModify PermissionOperation = "modify"
	PermDelete PermissionOperation = "delete"
)

// TablePermission represents the effective permissions for a single object: a table (Read,
// Insert, Modify, Delete) or a codeunit or report (Execute; key ObjectKey)
type TablePermission struct {
	Read    bool
	Insert  bool
	Modify  bool
	Delete  bool
	Execute bool
}

// Object types of execute permissions (Permission.object_type Report, Codeunit)
const (
	ObjectReport   = "REPORT"
	ObjectCodeunit = "CODEUNIT"
)

// ObjectKey is the Permission key of a codeunit or report: "CODEUNIT 6", "REPORT 93"
func ObjectKey(objectType string, id int) string {
	return fmt.Sprintf("%s %d", strings.ToUpper(objectType), id)
}

// Session represents a user session (similar to SESSION variable)
// Contains database connection, current company, and other session state
type Session struct {
	DB          *database.Database         // Database connection
	Company     string                     // Current company context
	Scanner     *bufio.Scanner             // Input scanner (for CLI)
	UserID      string                     // Current user ID/username
	UserName    string                     // Current user full name
	Language    string                     // Application language (e.g., "en-US", "de-DE")
	Menu        string                     // Assigned menu profile (e.g., "admin", "sales")
	IsSuper     bool                       // True if user has SUPER role (bypasses all permission checks)
	HasRoles    bool                       // True if user has any role memberships assigned
	Permissions map[string]TablePermission // Effective permissions per table name
	LoginTime   time.Time                  // When the user logged in to the company (the user time register)
	transaction *sql.Tx                    // Active transaction (nil if not in transaction)
}

// Global current session (like SESSION global variable)
var currentSession *Session

// NewSession creates a new session instance
func NewSession(db *database.Database, company string, scanner *bufio.Scanner) *Session {
	return &Session{
		DB:      db,
		Company: company,
		Scanner: scanner,
	}
}

// GetConnection returns the underlying SQL database connection
func (s *Session) GetConnection() *sql.DB {
	return s.DB.GetConnection()
}

// GetCompany returns the current company context
func (s *Session) GetCompany() string {
	return s.Company
}

// SetCompany changes the current company context
func (s *Session) SetCompany(company string) {
	s.Company = company
}

// GetDatabase returns the database instance
func (s *Session) GetDatabase() *database.Database {
	return s.DB
}

// GetScanner returns the input scanner
func (s *Session) GetScanner() *bufio.Scanner {
	return s.Scanner
}

// GetUserID returns the current user ID
func (s *Session) GetUserID() string {
	return s.UserID
}

// GetUserName returns the current user full name
func (s *Session) GetUserName() string {
	return s.UserName
}

// GetLanguage returns the current application language
func (s *Session) GetLanguage() string {
	return s.Language
}

// GetMenu returns the assigned menu profile
func (s *Session) GetMenu() string {
	return s.Menu
}

// SetUser sets the current user information
// Language is normalized to BCP-47 format (e.g., "NB-NO" → "nb-NO")
func (s *Session) SetUser(userID, userName, language, menu string) {
	s.UserID = userID
	s.UserName = userName
	s.Language = normalizeLanguage(language)
	s.Menu = menu
}

// SetLoginTime sets when the user logged in to the session's company
func (s *Session) SetLoginTime(t time.Time) { s.LoginTime = t }

// GetLoginTime returns when the user logged in to the session's company (zero: not known)
func (s *Session) GetLoginTime() time.Time { return s.LoginTime }

// normalizeLanguage converts language codes to proper BCP-47 format
// e.g., "NB-NO" → "nb-NO", "EN-US" → "en-US"
func normalizeLanguage(code string) string {
	if code == "" {
		return ""
	}
	parts := strings.Split(code, "-")
	if len(parts) == 2 {
		return strings.ToLower(parts[0]) + "-" + strings.ToUpper(parts[1])
	}
	return strings.ToLower(code)
}

// SetPermissions sets the effective permissions for the user
func (s *Session) SetPermissions(hasRoles, isSuper bool, permissions map[string]TablePermission) {
	s.HasRoles = hasRoles
	s.IsSuper = isSuper
	s.Permissions = permissions
}

// HasPermission checks if the current user has the specified permission on a table.
// If the user has no roles assigned, all access is allowed (permission system not configured).
func (s *Session) HasPermission(tableName string, op PermissionOperation) bool {
	if s.IsSuper {
		return true
	}
	if !s.HasRoles {
		return true
	}
	if s.Permissions == nil {
		return false
	}
	perm, exists := s.Permissions[strings.ToUpper(tableName)]
	if !exists {
		return false
	}
	switch op {
	case PermRead:
		return perm.Read
	case PermInsert:
		return perm.Insert
	case PermModify:
		return perm.Modify
	case PermDelete:
		return perm.Delete
	}
	return false
}

// CanExecute reports whether the user may run a codeunit or report (Execute permission on it,
// classic object permissions). SUPER may run everything, and so may a user without roles
// (permission system not configured for the user).
func (s *Session) CanExecute(objectType string, id int) bool {
	if s.IsSuper || !s.HasRoles {
		return true
	}
	return s.Permissions[ObjectKey(objectType, id)].Execute
}

// ========================================
// Global Session Management
// ========================================

// SetCurrent sets the global current session
func SetCurrent(s *Session) {
	currentSession = s
}

// GetCurrent returns the global current session
func GetCurrent() *Session {
	return currentSession
}

// ClearCurrent clears the global current session
func ClearCurrent() {
	currentSession = nil
}

// ========================================
// Transaction Management
// ========================================

// BeginTransaction starts a new database transaction
// Implicit transaction handling
// Returns error if a transaction is already active
func (s *Session) BeginTransaction() error {
	if s.transaction != nil {
		return fmt.Errorf("transaction already active")
	}

	tx, err := s.DB.GetConnection().Begin()
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}

	s.transaction = tx
	return nil
}

// Commit commits the current transaction (COMMIT)
// Makes all changes permanent since BeginTransaction
// Returns error if no transaction is active
func (s *Session) Commit() error {
	if s.transaction == nil {
		return fmt.Errorf("no active transaction to commit")
	}

	err := tables.Commit(s.transaction) // refused while a table is inconsistent (Consistent)
	s.transaction = nil                 // Clear transaction regardless of error

	if err != nil {
		return fmt.Errorf("failed to commit transaction: %w", err)
	}

	return nil
}

// Rollback rolls back the current transaction
// Undoes all changes since BeginTransaction
// Returns error if no transaction is active
func (s *Session) Rollback() error {
	if s.transaction == nil {
		return fmt.Errorf("no active transaction to rollback")
	}

	err := tables.Rollback(s.transaction)
	s.transaction = nil // Clear transaction regardless of error

	if err != nil {
		return fmt.Errorf("failed to rollback transaction: %w", err)
	}

	return nil
}

// InTransaction returns true if there is an active transaction
func (s *Session) InTransaction() bool {
	return s.transaction != nil
}

// GetExecutor returns the appropriate database executor
// Returns the active transaction if one exists, otherwise returns the DB connection
// This allows table operations to work seamlessly with or without transactions
func (s *Session) GetExecutor() database.Executor {
	if s.transaction != nil {
		return s.transaction
	}
	return s.DB.GetConnection()
}

// WithTransaction executes a function within a transaction with automatic rollback on error/panic
// This is the recommended way to use transactions - ensures cleanup even on errors
//
// Example:
//
//	err := sess.WithTransaction(func() error {
//	    customer.Insert(false)
//	    ledgerEntry.Insert(false)
//	    return nil  // Commits on success
//	})
func (s *Session) WithTransaction(fn func() error) error {
	// Begin transaction
	if err := s.BeginTransaction(); err != nil {
		return err
	}

	// Defer rollback - will execute if:
	// 1. Function returns error
	// 2. Function panics
	// 3. Commit fails
	// Will NOT execute if Commit succeeds (transaction is nil after commit)
	defer func() {
		if s.InTransaction() {
			_ = s.Rollback()
		}
	}()

	// Execute the function
	err := fn()
	if err != nil {
		return err // Deferred rollback will execute
	}

	// Commit if function succeeded
	return s.Commit()
}
