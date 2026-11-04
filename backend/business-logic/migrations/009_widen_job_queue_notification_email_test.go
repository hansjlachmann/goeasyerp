package migrations

import (
	"database/sql"
	"fmt"
	"math/rand"
	"os"
	"strings"
	"testing"
	"time"

	_ "github.com/lib/pq"
	_ "github.com/mattn/go-sqlite3"

	"github.com/hansjlachmann/goeasyerp/backend/foundation/database"
	fmigrations "github.com/hansjlachmann/goeasyerp/backend/foundation/migrations"
)

// Runs on SQLite (nothing to do) and on Postgres when TEST_POSTGRES_DSN is set (in a
// schema of its own, dropped afterwards).
func TestWidenJobQueueNotificationEmail(t *testing.T) {
	type testDB struct {
		db     *sql.DB
		dbType database.DBType
	}
	lite, err := sql.Open("sqlite3", ":memory:")
	if err != nil {
		t.Fatal(err)
	}
	lite.SetMaxOpenConns(1)
	t.Cleanup(func() { _ = lite.Close() })
	dbs := []testDB{{lite, database.DBTypeSQLite}}
	if dsn := os.Getenv("TEST_POSTGRES_DSN"); dsn != "" {
		admin, err := sql.Open("postgres", dsn)
		if err != nil {
			t.Fatal(err)
		}
		schema := fmt.Sprintf("migration009_test_%d", rand.New(rand.NewSource(time.Now().UnixNano())).Int63())
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
		pg.SetMaxOpenConns(1)
		t.Cleanup(func() {
			_ = pg.Close()
			_, _ = admin.Exec(`DROP SCHEMA ` + schema + ` CASCADE`)
			_ = admin.Close()
		})
		dbs = append(dbs, testDB{pg, database.DBTypePostgres})
	}

	long := strings.Repeat("a", 90) + "@example.com; " + strings.Repeat("b", 90) + "@example.com"
	for _, d := range dbs {
		t.Run(string(d.dbType), func(t *testing.T) {
			// A table as created before: notification_email VARCHAR(100)
			old := `CREATE TABLE "c1$Job_Queue" (no VARCHAR(20) PRIMARY KEY, notification_email VARCHAR(100))`
			if _, err := d.db.Exec(old); err != nil {
				t.Fatal(err)
			}
			if _, err := d.db.Exec(`INSERT INTO "c1$Job_Queue" (no, notification_email) VALUES ('J1', 'ops@example.com')`); err != nil {
				t.Fatal(err)
			}

			tx, err := d.db.Begin()
			if err != nil {
				t.Fatal(err)
			}
			// c2 has no Job_Queue table yet (company not entered since it was created)
			ctx := &fmigrations.Context{DB: d.db, DBType: d.dbType, Companies: []string{"c1", "c2"}}
			ctx.SetTx(tx)
			if err := (&Migration009WidenJobQueueNotificationEmail{}).Up(ctx); err != nil {
				t.Fatal(err)
			}
			if err := tx.Commit(); err != nil {
				t.Fatal(err)
			}

			var email string
			if err := d.db.QueryRow(`SELECT notification_email FROM "c1$Job_Queue" WHERE no = 'J1'`).Scan(&email); err != nil || email != "ops@example.com" {
				t.Fatalf("existing address not kept: %q (%v)", email, err)
			}
			if _, err := d.db.Exec(`UPDATE "c1$Job_Queue" SET notification_email = $1 WHERE no = 'J1'`, long); err != nil {
				t.Errorf("a %d character list does not fit: %v", len(long), err)
			}
		})
	}
}
