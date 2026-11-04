package migrations

import (
	"database/sql"
	"fmt"
	"os"
	"sort"
	"time"

	"github.com/hansjlachmann/goeasyerp/backend/foundation/database"
)

// Runner executes migrations with distributed locking
type Runner struct {
	db         *sql.DB
	dbType     database.DBType
	migrations []Migration
	lockTTL    time.Duration
	podID      string
	companies  func() ([]string, error) // Function to get company list
}

// NewRunner creates a new migration runner
func NewRunner(db *sql.DB, dbType database.DBType, migrations []Migration, companiesFunc func() ([]string, error)) *Runner {
	// Get pod ID from hostname or environment
	podID, _ := os.Hostname()
	if envPodID := os.Getenv("HOSTNAME"); envPodID != "" {
		podID = envPodID
	}
	if podID == "" {
		podID = fmt.Sprintf("pod-%d", time.Now().UnixNano())
	}

	return &Runner{
		db:         db,
		dbType:     dbType,
		migrations: migrations,
		lockTTL:    5 * time.Minute,
		podID:      podID,
		companies:  companiesFunc,
	}
}

// Run executes all pending migrations
func (r *Runner) Run() error {
	// 1. Ensure infrastructure tables exist
	if err := EnsureInfrastructureTables(r.db, r.dbType); err != nil {
		return fmt.Errorf("failed to ensure infrastructure tables: %w", err)
	}

	// 2. Acquire distributed lock
	if err := r.acquireLock(); err != nil {
		return fmt.Errorf("failed to acquire migration lock: %w", err)
	}
	defer r.releaseLock()

	// 3. Get applied versions
	applied, err := GetAppliedVersions(r.db)
	if err != nil {
		return fmt.Errorf("failed to get applied versions: %w", err)
	}

	// 4. Sort migrations by version
	sorted := make([]Migration, len(r.migrations))
	copy(sorted, r.migrations)
	sort.Slice(sorted, func(i, j int) bool {
		return sorted[i].Version() < sorted[j].Version()
	})

	// 5. Get companies list
	companies, err := r.companies()
	if err != nil {
		return fmt.Errorf("failed to get companies: %w", err)
	}

	// 6. Run pending migrations
	pendingCount := 0
	for _, m := range sorted {
		if applied[m.Version()] {
			continue
		}
		pendingCount++

		fmt.Printf("Running migration %d: %s...\n", m.Version(), m.Name())

		if err := r.runMigration(m, companies); err != nil {
			return fmt.Errorf("migration %d (%s) failed: %w", m.Version(), m.Name(), err)
		}

		fmt.Printf("  ✓ Migration %d completed\n", m.Version())
	}

	if pendingCount == 0 {
		fmt.Println("No pending migrations")
	} else {
		fmt.Printf("✓ Applied %d migration(s)\n", pendingCount)
	}

	return nil
}

// runMigration executes a single migration in a transaction
func (r *Runner) runMigration(m Migration, companies []string) error {
	startTime := time.Now()

	// Begin transaction
	tx, err := r.db.Begin()
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer func() {
		if tx != nil {
			_ = tx.Rollback()
		}
	}()

	// Create migration context
	ctx := &Context{
		DB:        r.db,
		DBType:    r.dbType,
		Companies: companies,
	}
	ctx.SetTx(tx)

	// Run the migration
	if err := m.Up(ctx); err != nil {
		// Record failed migration
		_ = RecordMigration(tx, Record{
			Version:      m.Version(),
			Name:         m.Name(),
			Description:  m.Description(),
			AppliedAt:    startTime,
			DurationMs:   time.Since(startTime).Milliseconds(),
			Status:       "failed",
			ErrorMessage: err.Error(),
		})
		return err
	}

	// Record successful migration
	if err := RecordMigration(tx, Record{
		Version:     m.Version(),
		Name:        m.Name(),
		Description: m.Description(),
		AppliedAt:   startTime,
		DurationMs:  time.Since(startTime).Milliseconds(),
		Status:      "completed",
	}); err != nil {
		return fmt.Errorf("failed to record migration: %w", err)
	}

	// Commit transaction
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("failed to commit transaction: %w", err)
	}
	tx = nil // Prevent deferred rollback

	return nil
}

// acquireLock attempts to acquire the distributed migration lock
func (r *Runner) acquireLock() error {
	maxRetries := 30
	retryInterval := 2 * time.Second

	for i := 0; i < maxRetries; i++ {
		acquired, err := r.tryAcquireLock()
		if err != nil {
			return err
		}
		if acquired {
			fmt.Printf("  ✓ Acquired migration lock (pod: %s)\n", r.podID)
			return nil
		}

		if i < maxRetries-1 {
			fmt.Printf("  Waiting for migration lock (attempt %d/%d)...\n", i+1, maxRetries)
			time.Sleep(retryInterval)
		}
	}

	return fmt.Errorf("failed to acquire migration lock after %d attempts", maxRetries)
}

// tryAcquireLock attempts to acquire the lock once
func (r *Runner) tryAcquireLock() (bool, error) {
	now := time.Now()
	expiresAt := now.Add(r.lockTTL)

	// Try to acquire lock (only if not locked or lock expired)
	var result sql.Result
	var err error

	switch r.dbType {
	case database.DBTypePostgres:
		result, err = r.db.Exec(`
			UPDATE "_migration_lock"
			SET locked_by = $1, locked_at = $2, expires_at = $3
			WHERE id = 1 AND (locked_by IS NULL OR expires_at < $4)
		`, r.podID, now, expiresAt, now)
	default:
		result, err = r.db.Exec(`
			UPDATE "_migration_lock"
			SET locked_by = ?, locked_at = ?, expires_at = ?
			WHERE id = 1 AND (locked_by IS NULL OR expires_at < ?)
		`, r.podID, now, expiresAt, now)
	}

	if err != nil {
		return false, fmt.Errorf("failed to acquire lock: %w", err)
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return false, fmt.Errorf("failed to check lock result: %w", err)
	}

	return rowsAffected > 0, nil
}

// releaseLock releases the distributed migration lock
func (r *Runner) releaseLock() {
	var err error

	switch r.dbType {
	case database.DBTypePostgres:
		_, err = r.db.Exec(`
			UPDATE "_migration_lock"
			SET locked_by = NULL, locked_at = NULL, expires_at = NULL
			WHERE id = 1 AND locked_by = $1
		`, r.podID)
	default:
		_, err = r.db.Exec(`
			UPDATE "_migration_lock"
			SET locked_by = NULL, locked_at = NULL, expires_at = NULL
			WHERE id = 1 AND locked_by = ?
		`, r.podID)
	}

	if err != nil {
		fmt.Printf("Warning: failed to release migration lock: %v\n", err)
	} else {
		fmt.Println("  ✓ Released migration lock")
	}
}

// GetPendingCount returns the number of pending migrations
func (r *Runner) GetPendingCount() (int, error) {
	if err := EnsureInfrastructureTables(r.db, r.dbType); err != nil {
		return 0, err
	}

	applied, err := GetAppliedVersions(r.db)
	if err != nil {
		return 0, err
	}

	count := 0
	for _, m := range r.migrations {
		if !applied[m.Version()] {
			count++
		}
	}

	return count, nil
}
