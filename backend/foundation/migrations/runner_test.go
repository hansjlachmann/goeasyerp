package migrations

import (
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	_ "github.com/lib/pq"

	"github.com/hansjlachmann/goeasyerp/backend/foundation/database"
)

// The lock (S15): a holder that renews it keeps it past its time to live; one that stopped
// loses it once it expired; each runner has an ID of its own
// (SQLite, and Postgres when TEST_POSTGRES_DSN is set: a schema of its own)
func TestMigrationLockRenewal(t *testing.T) {
	lite, err := sql.Open("sqlite3", filepath.Join(t.TempDir(), "lock.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = lite.Close() })
	t.Run("sqlite", func(t *testing.T) { testLockRenewal(t, lite, database.DBTypeSQLite) })
	dsn := os.Getenv("TEST_POSTGRES_DSN")
	if dsn == "" {
		return
	}
	admin, err := sql.Open("postgres", dsn)
	if err != nil {
		t.Fatal(err)
	}
	schema := fmt.Sprintf("lock_test_%d", time.Now().UnixNano())
	if _, err := admin.Exec(`CREATE SCHEMA ` + schema); err != nil {
		t.Fatal(err)
	}
	sep := "?"
	if strings.Contains(dsn, "?") {
		sep = "&"
	}
	pg, err := sql.Open("postgres", dsn+sep+"search_path="+schema)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		_ = pg.Close()
		_, _ = admin.Exec(`DROP SCHEMA ` + schema + ` CASCADE`)
		_ = admin.Close()
	})
	t.Run("postgres", func(t *testing.T) { testLockRenewal(t, pg, database.DBTypePostgres) })
}

func testLockRenewal(t *testing.T, db *sql.DB, dbType database.DBType) {
	if err := EnsureInfrastructureTables(db, dbType); err != nil {
		t.Fatal(err)
	}
	noCompanies := func() ([]string, error) { return nil, nil }
	a := NewRunner(db, dbType, nil, noCompanies)
	b := NewRunner(db, dbType, nil, noCompanies)
	if a.podID == b.podID || !strings.Contains(a.podID, "-") {
		t.Fatalf("runner IDs %q and %q", a.podID, b.podID)
	}
	a.lockTTL, b.lockTTL = 300*time.Millisecond, 300*time.Millisecond

	if ok, err := a.tryAcquireLock(); err != nil || !ok {
		t.Fatalf("a: %v, %v", ok, err)
	}
	if held, err := b.renewLock(); err != nil || held {
		t.Errorf("b renewed a lock it does not hold: %v, %v", held, err)
	}
	stop := a.renewEvery(50 * time.Millisecond)
	time.Sleep(600 * time.Millisecond) // twice the time to live
	if ok, err := b.tryAcquireLock(); err != nil || ok {
		t.Fatalf("b took the lock a renews: %v, %v", ok, err)
	}
	stop()
	time.Sleep(400 * time.Millisecond)
	if ok, err := b.tryAcquireLock(); err != nil || !ok {
		t.Fatalf("b could not take the lock a let expire: %v, %v", ok, err)
	}
	if held, _ := a.renewLock(); held {
		t.Error("a renewed the lock b holds now")
	}
	b.releaseLock()
	if ok, _ := a.tryAcquireLock(); !ok {
		t.Error("the released lock could not be taken")
	}
}
