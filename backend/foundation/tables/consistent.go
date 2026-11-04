package tables

import (
	"database/sql"
	"sort"
	"strings"
	"sync"

	"github.com/hansjlachmann/openerp/backend/foundation/database"
	apperrors "github.com/hansjlachmann/openerp/backend/foundation/errors"
)

// Consistent: a table can be marked inconsistent within a transaction (record Consistent
// (false), e.g. G/L entries while a posting is not balanced yet); Commit refuses to commit the
// transaction while any table is inconsistent and rolls it back.
var (
	inconsistentMu sync.Mutex
	inconsistent   = map[database.Executor]map[string]bool{}
	txState        = map[database.Executor]map[string]any{}
	diagnoseMu     sync.Mutex
	diagnoses      = map[database.Executor]map[string]func() error{}
)

// OnInconsistent registers how to explain a table left inconsistent in the transaction exec:
// Commit calls diagnose before the rollback (the transaction's writes can still be read) and
// returns its error — e.g. the G/L entries that do not balance, by document and date, with the
// entries as the error's details. Without one Commit names the table. Outside a transaction
// it does nothing.
func OnInconsistent(exec database.Executor, table string, diagnose func() error) {
	_, ok := exec.(*sql.Tx)
	if !ok {
		return
	}
	diagnoseMu.Lock()
	defer diagnoseMu.Unlock()
	if diagnoses[exec] == nil {
		diagnoses[exec] = map[string]func() error{}
	}
	diagnoses[exec][table] = diagnose
}

// TransactionState is a value kept with the transaction exec under key (made by newValue the
// first time) and dropped when the transaction ends (Commit, Rollback) — e.g. the balance sums
// of the G/L entries posted in it, shared by all postings in the transaction. Outside a
// transaction (exec a database) there is nothing to keep it with: a new value every call.
func TransactionState(exec database.Executor, key string, newValue func() any) any {
	_, ok := exec.(*sql.Tx)
	if !ok {
		return newValue()
	}
	inconsistentMu.Lock()
	defer inconsistentMu.Unlock()
	if txState[exec] == nil {
		txState[exec] = map[string]any{}
	}
	value, ok := txState[exec][key]
	if !ok {
		value = newValue()
		txState[exec][key] = value
	}
	return value
}

// SetConsistent marks a table consistent or inconsistent in the transaction (or database) exec
func SetConsistent(exec database.Executor, table string, ok bool) {
	inconsistentMu.Lock()
	defer inconsistentMu.Unlock()
	if ok {
		delete(inconsistent[exec], table)
		if len(inconsistent[exec]) == 0 {
			delete(inconsistent, exec)
		}
		return
	}
	if inconsistent[exec] == nil {
		inconsistent[exec] = map[string]bool{}
	}
	inconsistent[exec][table] = true
}

// Commit commits a transaction unless a table was left inconsistent in it (Consistent(false)):
// then it rolls the transaction back, reports the refusal (OnRefusedCommit) and returns an
// error naming the tables
func Commit(tx *sql.Tx) error {
	tables, explain := clearConsistency(tx)
	if len(tables) > 0 {
		err := inconsistencyError(tables, explain)
		_ = tx.Rollback()
		refused(err)
		return err
	}
	return tx.Commit()
}

var (
	refusedMu    sync.Mutex
	refusedHooks []func(err error)
)

// OnRefusedCommit registers what happens when Commit refuses a transaction (after its
// rollback): the server logs the refused posting for an administrator (table 904 Refused
// Posting, in a transaction of its own). The hook runs in the goroutine of the refused
// commit — the request's user and company are its context.
func OnRefusedCommit(hook func(err error)) {
	refusedMu.Lock()
	defer refusedMu.Unlock()
	refusedHooks = append(refusedHooks, hook)
}

// refused runs the OnRefusedCommit hooks
func refused(err error) {
	refusedMu.Lock()
	hooks := append([]func(error){}, refusedHooks...)
	refusedMu.Unlock()
	for _, hook := range hooks {
		hook(err)
	}
}

// Inconsistency is the error Commit would return for the transaction now (nil: it would
// commit), with the tables' explanations run — without committing or forgetting anything: a
// posting preview shows it and then rolls back
func Inconsistency(tx *sql.Tx) error {
	inconsistentMu.Lock()
	var tables []string
	for table := range inconsistent[tx] {
		tables = append(tables, table)
	}
	inconsistentMu.Unlock()
	if len(tables) == 0 {
		return nil
	}
	sort.Strings(tables)
	diagnoseMu.Lock()
	explain := map[string]func() error{}
	for table, diagnose := range diagnoses[tx] {
		explain[table] = diagnose
	}
	diagnoseMu.Unlock()
	return inconsistencyError(tables, explain)
}

// inconsistencyError is the error of a refused commit: the first registered explanation of an
// inconsistent table (OnInconsistent), else the tables named
func inconsistencyError(tables []string, explain map[string]func() error) error {
	for _, table := range tables {
		diagnose, ok := explain[table]
		if ok {
			err := diagnose()
			if err != nil {
				return err
			}
		}
	}
	return apperrors.Inconsistent(strings.Join(tables, ", "))
}

// Rollback rolls a transaction back and forgets its inconsistent tables
func Rollback(tx *sql.Tx) error {
	clearConsistency(tx)
	return tx.Rollback()
}

// clearConsistency returns the inconsistent tables of exec (sorted) and their explanations,
// and forgets them and the transaction's state
func clearConsistency(exec database.Executor) ([]string, map[string]func() error) {
	inconsistentMu.Lock()
	var tables []string
	for table := range inconsistent[exec] {
		tables = append(tables, table)
	}
	delete(inconsistent, exec)
	delete(txState, exec)
	inconsistentMu.Unlock()
	sort.Strings(tables)

	diagnoseMu.Lock()
	explain := diagnoses[exec]
	delete(diagnoses, exec)
	diagnoseMu.Unlock()
	return tables, explain
}
