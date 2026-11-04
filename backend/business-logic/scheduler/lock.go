package scheduler

import (
	"fmt"
	"log"
	"time"

	"github.com/hansjlachmann/goeasyerp/backend/foundation/database"
)

// ensureLockTable creates the single-row distributed lock table if it does not
// exist. This mirrors _migration_lock and lets only one instance run each tick.
func (s *Scheduler) ensureLockTable() error {
	var ddl string
	switch s.dbType {
	case database.DBTypePostgres:
		ddl = `
			CREATE TABLE IF NOT EXISTS "_scheduler_lock" (
				id INTEGER PRIMARY KEY DEFAULT 1 CHECK (id = 1),
				locked_by TEXT,
				locked_at TIMESTAMP,
				expires_at TIMESTAMP
			);
			INSERT INTO "_scheduler_lock" (id) VALUES (1) ON CONFLICT (id) DO NOTHING;`
	default:
		ddl = `
			CREATE TABLE IF NOT EXISTS "_scheduler_lock" (
				id INTEGER PRIMARY KEY CHECK (id = 1),
				locked_by TEXT,
				locked_at TIMESTAMP,
				expires_at TIMESTAMP
			);
			INSERT OR IGNORE INTO "_scheduler_lock" (id) VALUES (1);`
	}
	_, err := s.db.Exec(ddl)
	if err != nil {
		return fmt.Errorf("create _scheduler_lock: %w", err)
	}
	return nil
}

// tryAcquireLock attempts to claim the lock for this tick. It succeeds if the
// lock is free or its lease has expired (a crashed holder's lock auto-frees
// after lockTTL). Returns false if another instance holds a live lease.
func (s *Scheduler) tryAcquireLock() (bool, error) {
	now := time.Now()
	expiresAt := now.Add(s.lockTTL)

	var (
		result interface {
			RowsAffected() (int64, error)
		}
		err error
	)
	switch s.dbType {
	case database.DBTypePostgres:
		result, err = s.db.Exec(`
			UPDATE "_scheduler_lock"
			SET locked_by = $1, locked_at = $2, expires_at = $3
			WHERE id = 1 AND (locked_by IS NULL OR expires_at < $4)
		`, s.podID, now, expiresAt, now)
	default:
		result, err = s.db.Exec(`
			UPDATE "_scheduler_lock"
			SET locked_by = ?, locked_at = ?, expires_at = ?
			WHERE id = 1 AND (locked_by IS NULL OR expires_at < ?)
		`, s.podID, now, expiresAt, now)
	}
	if err != nil {
		return false, fmt.Errorf("acquire _scheduler_lock: %w", err)
	}

	rows, err := result.RowsAffected()
	if err != nil {
		return false, fmt.Errorf("check lock result: %w", err)
	}
	return rows > 0, nil
}

// releaseLock frees the lock, but only if this instance still owns it.
func (s *Scheduler) releaseLock() {
	var err error
	switch s.dbType {
	case database.DBTypePostgres:
		_, err = s.db.Exec(`
			UPDATE "_scheduler_lock"
			SET locked_by = NULL, locked_at = NULL, expires_at = NULL
			WHERE id = 1 AND locked_by = $1
		`, s.podID)
	default:
		_, err = s.db.Exec(`
			UPDATE "_scheduler_lock"
			SET locked_by = NULL, locked_at = NULL, expires_at = NULL
			WHERE id = 1 AND locked_by = ?
		`, s.podID)
	}
	if err != nil {
		log.Printf("Job Queue scheduler: failed to release lock: %v", err)
	}
}
