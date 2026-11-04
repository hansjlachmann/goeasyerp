package tables

import (
	"database/sql"
	"fmt"
	"os"
	"strings"
	"sync"
	"testing"
	"time"

	_ "github.com/lib/pq"
	_ "github.com/mattn/go-sqlite3"

	"github.com/hansjlachmann/openerp/backend/foundation/database"
)

func TestBeginTransaction(t *testing.T) {
	db, err := sql.Open("sqlite3", ":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	tx, err := BeginTransaction(db)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := BeginTransaction(tx); err == nil {
		t.Error("a transaction inside a transaction must be refused")
	}
	SetConsistent(tx, "G_L_Entry", false)
	if err := Commit(tx); err == nil {
		t.Error("commit with an inconsistent table must be refused")
	}
	// LockTable outside Postgres or outside a transaction does nothing
	LockTable(db, database.DBTypeSQLite, "c", "G_L_Entry")
}

// On Postgres LockTable makes a second transaction wait until the first ends (TEST_POSTGRES_DSN)
func TestLockTablePostgres(t *testing.T) {
	dsn := os.Getenv("TEST_POSTGRES_DSN")
	if dsn == "" {
		t.Skip("TEST_POSTGRES_DSN not set")
	}
	db, err := sql.Open("postgres", dsn)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	first, err := BeginTransaction(db)
	if err != nil {
		t.Fatal(err)
	}
	LockTable(first, database.DBTypePostgres, "locktest", "G_L_Entry")

	var mu sync.Mutex
	var order []string
	done := make(chan struct{})
	go func() {
		defer close(done)
		second, err := BeginTransaction(db)
		if err != nil {
			t.Error(err)
			return
		}
		LockTable(second, database.DBTypePostgres, "locktest", "G_L_Entry")
		mu.Lock()
		order = append(order, "second")
		mu.Unlock()
		_ = Rollback(second)
	}()
	time.Sleep(300 * time.Millisecond)
	mu.Lock()
	order = append(order, "first")
	mu.Unlock()
	_ = Rollback(first)
	<-done
	if len(order) != 2 || order[0] != "first" {
		t.Errorf("the second lock did not wait: %v", order)
	}
}

// A transaction's state is shared within the transaction and dropped when it ends
func TestTransactionState(t *testing.T) {
	db, err := sql.Open("sqlite3", ":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	newCounter := func() any { return new(int) }
	tx, err := BeginTransaction(db)
	if err != nil {
		t.Fatal(err)
	}
	*TransactionState(tx, "n", newCounter).(*int) += 5
	if got := *TransactionState(tx, "n", newCounter).(*int); got != 5 {
		t.Errorf("shared in the transaction: %d", got)
	}
	if err := Commit(tx); err != nil {
		t.Fatal(err)
	}
	if got := *TransactionState(tx, "n", newCounter).(*int); got != 0 {
		t.Errorf("dropped at commit: %d", got)
	}
	*TransactionState(db, "n", newCounter).(*int) += 1
	if got := *TransactionState(db, "n", newCounter).(*int); got != 0 {
		t.Errorf("outside a transaction nothing is kept: %d", got)
	}
}

// A refused commit returns the registered explanation of the inconsistent table, run while the
// transaction is still open; without one it names the table
func TestCommitExplainsInconsistency(t *testing.T) {
	db, err := sql.Open("sqlite3", ":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	db.SetMaxOpenConns(1)
	if _, err := db.Exec(`CREATE TABLE t (n INTEGER)`); err != nil {
		t.Fatal(err)
	}
	tx, err := BeginTransaction(db)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := tx.Exec(`INSERT INTO t VALUES (7)`); err != nil {
		t.Fatal(err)
	}
	SetConsistent(tx, "G_L_Entry", false)
	OnInconsistent(tx, "G_L_Entry", func() error {
		var n int
		if err := tx.QueryRow(`SELECT n FROM t`).Scan(&n); err != nil {
			return err
		}
		return fmt.Errorf("explained: the transaction wrote %d", n)
	})
	if err := Commit(tx); err == nil || err.Error() != "explained: the transaction wrote 7" {
		t.Errorf("commit: %v", err)
	}
	var rows int
	if err := db.QueryRow(`SELECT COUNT(*) FROM t`).Scan(&rows); err != nil || rows != 0 {
		t.Errorf("rolled back: %d rows (%v)", rows, err)
	}

	tx, err = BeginTransaction(db)
	if err != nil {
		t.Fatal(err)
	}
	SetConsistent(tx, "G_L_Entry", false)
	if err := Commit(tx); err == nil || !strings.Contains(err.Error(), "G_L_Entry") {
		t.Errorf("without an explanation the table is named: %v", err)
	}
}

// A refused commit is reported to the OnRefusedCommit hooks with its error (after the
// rollback); a commit that goes through is not
func TestOnRefusedCommit(t *testing.T) {
	db, err := sql.Open("sqlite3", ":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	var reported []error
	refusedMu.Lock()
	before := refusedHooks
	refusedMu.Unlock()
	t.Cleanup(func() {
		refusedMu.Lock()
		refusedHooks = before
		refusedMu.Unlock()
	})
	OnRefusedCommit(func(err error) { reported = append(reported, err) })

	tx, err := BeginTransaction(db)
	if err != nil {
		t.Fatal(err)
	}
	if err := Commit(tx); err != nil || len(reported) != 0 {
		t.Fatalf("a consistent commit: %v, reported %v", err, reported)
	}
	tx, err = BeginTransaction(db)
	if err != nil {
		t.Fatal(err)
	}
	SetConsistent(tx, "G_L_Entry", false)
	err = Commit(tx)
	if err == nil || len(reported) != 1 || reported[0] != err {
		t.Errorf("refused commit %v, reported %v", err, reported)
	}
}
