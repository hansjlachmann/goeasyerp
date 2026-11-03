// Package postingpreview is Preview Posting: a posting runs in a transaction that is always
// rolled back, and the entries it wrote are kept for a while (per user) to be looked at on the
// Posting Preview page (115) and the entries pages — nothing is saved, the number series and
// the document stay as they are. A posting whose G/L entries do not balance is previewed too:
// the preview carries the balance check's error, with the entries to find the difference.
package postingpreview

import (
	"crypto/rand"
	"encoding/hex"
	"strconv"
	"sync"
	"time"

	fcodeunits "github.com/hansjlachmann/openerp/backend/foundation/codeunits"
	"github.com/hansjlachmann/openerp/backend/foundation/database"
	ftables "github.com/hansjlachmann/openerp/backend/foundation/tables"
	gtables "github.com/hansjlachmann/openerp/backend/generated/tables"
)

// Lifetime: how long a preview is kept
const Lifetime = 10 * time.Minute

// PageID: page 115 Posting Preview
const PageID = 115

// EntryTable is an entry table a posting writes: its records are numbered by entry_no
type EntryTable struct {
	Name   string // the table's registry name (its API name where it has one)
	ID     int
	PageID int // the entries page the preview opens
}

// EntryTables are the tables a preview collects, in the order the preview lists them
var EntryTables = []EntryTable{
	{gtables.GLEntryTableName, gtables.GLEntryTableID, 20},
	{gtables.TaxEntryTableName, gtables.TaxEntryTableID, 315},
	{"Customer_ledger_entry", gtables.CustomerLedgerEntryTableID, 25},
	{"Vendor_ledger_entry", gtables.VendorLedgerEntryTableID, 29},
	{gtables.BankAccountLedgerEntryTableName, gtables.BankAccountLedgerEntryTableID, 372},
	{gtables.CheckLedgerEntryTableName, gtables.CheckLedgerEntryTableID, 374},
	{gtables.ItemLedgerEntryTableName, gtables.ItemLedgerEntryTableID, 38},
	{gtables.PhysInventoryLedgerEntryTableName, gtables.PhysInventoryLedgerEntryTableID, 390},
	{gtables.ResLedgerEntryTableName, gtables.ResLedgerEntryTableID, 202},
	{gtables.ProjectLedgerEntryTableName, gtables.ProjectLedgerEntryTableID, 92},
	{gtables.FALedgerEntryTableName, gtables.FALedgerEntryTableID, 5604},
	{gtables.MaintenanceLedgerEntryTableName, gtables.MaintenanceLedgerEntryTableID, 5641},
	{gtables.InsCoverageLedgerEntryTableName, gtables.InsCoverageLedgerEntryTableID, 5647},
	{gtables.BOMLedgerEntryTableName, gtables.BOMLedgerEntryTableID, 430},
}

// IsEntryTable reports whether a preview collects the table's entries
func IsEntryTable(tableName string) bool {
	for _, table := range EntryTables {
		if table.Name == tableName {
			return true
		}
	}
	return false
}

// Entries are the records a previewed posting wrote in one entry table
type Entries struct {
	Table EntryTable
	Rows  []map[string]interface{}
}

// Preview is a previewed posting
type Preview struct {
	ID      string
	UserID  string
	Company string
	Created time.Time
	Tables  []*Entries // the tables with entries, in EntryTables order
	// Inconsistency: the error the commit would have given (the G/L entries do not balance);
	// nil when the posting would have been saved
	Inconsistency error
}

// Result opens Posting Preview on the preview (Esc goes back to the document), or is the
// balance check's error of a posting that does not balance (its G/L entries with Show Entries)
func (p *Preview) Result() (fcodeunits.Result, error) {
	if p.Inconsistency != nil {
		return fcodeunits.Result{}, p.Inconsistency
	}
	return fcodeunits.Result{
		Success:  true,
		OpenPage: &fcodeunits.PageLink{PageID: PageID, Params: map[string]string{"preview": p.ID}, Return: true},
	}, nil
}

// Rows returns the preview's rows of a table (nil when it has none)
func (p *Preview) Rows(tableName string) []map[string]interface{} {
	for _, entries := range p.Tables {
		if entries.Table.Name == tableName {
			return entries.Rows
		}
	}
	return nil
}

// Run previews a posting: post runs on a transaction of db (as the posting codeunit would),
// the entries it wrote are collected, and the transaction is rolled back. An error of the
// posting itself (a missing setup, a check) is returned as it is; a posting that does not
// balance is previewed with the balance check's error in Inconsistency. The preview is kept
// for userID in company (Get).
func Run(db database.Executor, company string, dbType database.DBType, userID string, post func(src ftables.Source) error) (*Preview, error) {
	tx, err := ftables.BeginTransaction(db)
	if err != nil {
		return nil, err
	}
	defer func() { _ = ftables.Rollback(tx) }() // a preview is never saved
	src := ftables.DBSource(tx, company, dbType)

	lastEntryNo := map[string]int{}
	for _, table := range EntryTables {
		lastEntryNo[table.Name] = lastEntry(src, table.Name)
	}
	err = post(src)
	if err != nil {
		return nil, err
	}

	preview := &Preview{ID: newID(), UserID: userID, Company: company, Created: time.Now()}
	preview.Inconsistency = ftables.Inconsistency(tx)
	for _, table := range EntryTables {
		rows := entriesAfter(src, table.Name, lastEntryNo[table.Name])
		if len(rows) > 0 {
			preview.Tables = append(preview.Tables, &Entries{Table: table, Rows: rows})
		}
	}
	keep(preview)
	return preview, nil
}

// newRecord opens an entry table's record in src
func newRecord(src ftables.Source, tableName string) ftables.Table {
	factory, ok := gtables.GetTableFactory(tableName)
	if !ok {
		return nil
	}
	rec := factory()
	rec.InitWithDBType(src.GetDB(), src.GetCompany(), src.GetDBType())
	return rec
}

// lastEntry: the table's last entry no. (0 without entries)
func lastEntry(src ftables.Source, tableName string) int {
	rec := newRecord(src, tableName)
	if rec == nil {
		return 0
	}
	rec.SetCurrentKey("entry_no")
	rec.SetAscending(false)
	rec.SetPage(1, 0)
	if !rec.FindSet() {
		return 0
	}
	no, _ := rec.ToMap()["entry_no"].(int)
	return no
}

// entriesAfter: the table's entries after entry no. after (what the posting wrote)
func entriesAfter(src ftables.Source, tableName string, after int) []map[string]interface{} {
	rec := newRecord(src, tableName)
	if rec == nil {
		return nil
	}
	rec.SetFilter("entry_no", ">"+strconv.Itoa(after))
	rec.SetCurrentKey("entry_no")
	var rows []map[string]interface{}
	for found := rec.FindSet(); found; found = rec.Next() {
		rows = append(rows, rec.ToMap())
	}
	return rows
}

func newID() string {
	b := make([]byte, 12)
	_, _ = rand.Read(b)
	return hex.EncodeToString(b)
}

// --- the previews kept ---

var (
	mu       sync.Mutex
	previews = map[string]*Preview{}
)

// keep stores a preview and drops the expired ones
func keep(preview *Preview) {
	mu.Lock()
	defer mu.Unlock()
	for id, p := range previews {
		if time.Since(p.Created) > Lifetime {
			delete(previews, id)
		}
	}
	previews[preview.ID] = preview
}

// Get returns the preview with the id when it belongs to the user in the company and has not
// expired
func Get(id, userID, company string) (*Preview, bool) {
	mu.Lock()
	defer mu.Unlock()
	preview, ok := previews[id]
	if !ok || preview.UserID != userID || preview.Company != company || time.Since(preview.Created) > Lifetime {
		return nil, false
	}
	return preview, true
}
