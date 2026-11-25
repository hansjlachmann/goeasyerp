package tables

import (
	"database/sql"
	"fmt"
	"log"
	"strings"
	"sync"
	"time"

	fcodeunits "github.com/hansjlachmann/goeasyerp/backend/foundation/codeunits"
	"github.com/hansjlachmann/goeasyerp/backend/foundation/database"
	apperrors "github.com/hansjlachmann/goeasyerp/backend/foundation/errors"
	"github.com/hansjlachmann/goeasyerp/backend/foundation/i18n"
	"github.com/hansjlachmann/goeasyerp/backend/foundation/system"
	ftables "github.com/hansjlachmann/goeasyerp/backend/foundation/tables"
	"github.com/hansjlachmann/goeasyerp/backend/foundation/types"
	gtables "github.com/hansjlachmann/goeasyerp/backend/generated/tables"
)

// The change log (S47): which changes are recorded (Change Log Setup 402–404, read into memory
// and read again every 30 s and soon after its setup changes) and how an entry is written
// (Change Log Entry 405, in the change's own transaction). foundation/tables calls it after
// every insert, modify and delete of a record (tables.SetChangeLogger).

// defaultChangeLogTables are recorded — every field, insertions, modifications and deletions —
// while the change log is activated and the table has no line in Change Log Setup (Table): the
// tables where a change matters for security or money
var defaultChangeLogTables = []int{
	gtables.UserTableID, gtables.UserMemberTableID, gtables.UserRoleTableID, gtables.PermissionTableID,
	gtables.CompanyTableID, gtables.InstalledAppTableID, gtables.AppBuildSetupTableID, gtables.SMTPSetupTableID,
	gtables.BankAccountTableID, gtables.VendorBankAccountTableID, gtables.CustomerBankAccountTableID,
	gtables.GeneralPostingSetupTableID, gtables.TaxPostingSetupTableID,
	gtables.CustomerPostingGroupTableID, gtables.VendorPostingGroupTableID, gtables.InventoryPostingGroupTableID,
	gtables.BankAccountPostingGroupTableID, gtables.FAPostingGroupTableID, gtables.ProjectPostingGroupTableID,
	gtables.GenBusinessPostingGroupTableID, gtables.GenProductPostingGroupTableID,
	gtables.ChangeLogSetupTableID, gtables.ChangeLogSetupTableTableID, gtables.ChangeLogSetupFieldTableID,
}

// defaultSkippedFields are fields of the default tables that change on their own (every login
// and log out) and are not recorded by the defaults
var defaultSkippedFields = map[int]map[string]bool{
	gtables.UserTableID: {"last_login": true, "sessions_valid_from": true},
}

// changeLogReload: how often the setup is read again (other servers' changes)
const changeLogReload = 30 * time.Second

// tableLogging is how one table is recorded: per change type nothing (nil), all fields, or
// some (the field numbers)
type tableLogging struct {
	all    [3]bool
	fields [3]map[int]bool
	skip   map[int]bool // field numbers never recorded (defaults)
}

func (t *tableLogging) wants(change ftables.ChangeType) func(no int) bool {
	if t.all[change] {
		if len(t.skip) == 0 {
			return func(int) bool { return true }
		}
		return func(no int) bool { return !t.skip[no] }
	}
	if fields := t.fields[change]; len(fields) > 0 {
		return func(no int) bool { return fields[no] }
	}
	return nil
}

// changeLog is the registered change logger
type changeLog struct {
	db     *sql.DB
	dbType database.DBType

	mu       sync.RWMutex
	tables   map[int]*tableLogging // nil: the change log is off
	loadedAt time.Time
	dirty    chan struct{}
	stop     chan struct{}
}

var (
	changeLogMu sync.Mutex
	current     *changeLog
)

// RegisterChangeLog starts the change log on the database (the API server, after the table
// sync): the setup is read now and again every 30 s and soon after it changes. The returned
// function stops it.
func RegisterChangeLog(db *sql.DB, dbType database.DBType) (stop func()) {
	c := &changeLog{db: db, dbType: dbType, dirty: make(chan struct{}, 1), stop: make(chan struct{})}
	err := c.reload()
	if err != nil {
		log.Printf("change log: reading the setup: %v", err)
	}
	changeLogMu.Lock()
	current = c
	changeLogMu.Unlock()
	ftables.SetChangeLogger(c)
	go c.watch()
	var once sync.Once
	return func() {
		once.Do(func() {
			close(c.stop)
			ftables.SetChangeLogger(nil)
			changeLogMu.Lock()
			if current == c {
				current = nil
			}
			changeLogMu.Unlock()
		})
	}
}

// watch reads the setup again every 30 s and soon after it changed
func (c *changeLog) watch() {
	ticker := time.NewTicker(changeLogReload)
	defer ticker.Stop()
	for {
		select {
		case <-c.stop:
			return
		case <-ticker.C:
		case <-c.dirty:
			time.Sleep(time.Second) // the change's transaction commits first
		}
		err := c.reload()
		if err != nil {
			log.Printf("change log: reading the setup: %v", err)
		}
	}
}

// changeLogSetupChanged makes the change log read its setup again (the setup tables' triggers)
func changeLogSetupChanged() {
	changeLogMu.Lock()
	c := current
	changeLogMu.Unlock()
	if c == nil {
		return
	}
	select {
	case c.dirty <- struct{}{}:
	default:
	}
}

// reload reads the setup into memory
func (c *changeLog) reload() error {
	src := ftables.DBSource(c.db, "", c.dbType)
	tables, err := readChangeLogSetup(src)
	if err != nil {
		return err
	}
	c.mu.Lock()
	c.tables = tables
	c.loadedAt = time.Now()
	c.mu.Unlock()
	return nil
}

// readChangeLogSetup: the tables the change log records (nil: it is off)
func readChangeLogSetup(src ftables.Source) (map[int]*tableLogging, error) {
	changeLogSetup := gtables.GetChangeLogSetup(src)
	if !changeLogSetup.Change_log_activated {
		return nil, nil
	}
	result := map[int]*tableLogging{}
	for _, id := range defaultChangeLogTables {
		logging := &tableLogging{all: [3]bool{true, true, true}}
		for name := range defaultSkippedFields[id] {
			if logging.skip == nil {
				logging.skip = map[int]bool{}
			}
			logging.skip[fieldNo(id, name)] = true
		}
		result[id] = logging
	}

	setupTable := gtables.NewChangeLogSetupTable(src)
	for found := setupTable.FindSet(); found; found = setupTable.Next() {
		logging := &tableLogging{}
		options := []gtables.ChangeLogSetupTableLog_insertion{
			setupTable.Log_insertion,
			gtables.ChangeLogSetupTableLog_insertion(setupTable.Log_modification),
			gtables.ChangeLogSetupTableLog_insertion(setupTable.Log_deletion),
		}
		for change, option := range options {
			logging.all[change] = option == gtables.ChangeLogSetupTable_Log_insertion.AllFields
			if option == gtables.ChangeLogSetupTable_Log_insertion.SomeFields {
				logging.fields[change] = map[int]bool{}
			}
		}
		result[setupTable.Table_no] = logging // a line replaces the default
	}

	setupField := gtables.NewChangeLogSetupField(src)
	for found := setupField.FindSet(); found; found = setupField.Next() {
		logging, ok := result[setupField.Table_no]
		if !ok {
			continue
		}
		marks := []bool{setupField.Log_insertion, setupField.Log_modification, setupField.Log_deletion}
		for change, marked := range marks {
			if marked && logging.fields[change] != nil {
				logging.fields[change][setupField.Field_no] = true
			}
		}
	}
	// the entries themselves are never recorded
	delete(result, gtables.ChangeLogEntryTableID)
	return result, nil
}

// fieldNo: a field's number by its name (0 unknown)
func fieldNo(tableID int, name string) int {
	table, ok := gtables.NewRecords()[tableID]
	if !ok {
		return 0
	}
	for _, f := range table.GetFields() {
		if f.Name == name {
			return f.No
		}
	}
	return 0
}

// Logs: which fields of the table the change log records for the change (nil: none)
func (c *changeLog) Logs(tableID int, change ftables.ChangeType) func(no int) bool {
	c.mu.RLock()
	defer c.mu.RUnlock()
	logging, ok := c.tables[tableID]
	if !ok {
		return nil
	}
	return logging.wants(change)
}

// Log writes a change's entries, one per field, in the change's own transaction
func (c *changeLog) Log(change ftables.Change) error {
	return writeChangeLogEntries(change, fcodeunits.CurrentUserID(), time.Now())
}

// writeChangeLogEntries writes the entries of a change by the user at a time
func writeChangeLogEntries(change ftables.Change, userID string, at time.Time) error {
	src := ftables.DBSource(change.Exec, "", change.DBType)
	entryNos, err := nextChangeLogEntryNos(change.Exec, change.DBType, len(change.Fields))
	if err != nil {
		return err
	}
	ts := i18n.GetInstance()
	tableName := englishCaption(ts.TableCaption(change.Table, "en-US"), change.Table)
	changeLogEntry := gtables.NewChangeLogEntry(src)
	records := make([]*gtables.ChangeLogEntry, 0, len(change.Fields))
	for i, f := range change.Fields {
		fieldName := englishCaption(ts.FieldCaption(change.Table, f.Name, "en-US"), f.Name)
		record := gtables.NewChangeLogEntry(src)
		record.Entry_no = entryNos[i]
		record.Date_and_time = types.NewDateTimeFromTime(at)
		record.User_id = types.NewCode(system.CopyStr(userID, 1, 50))
		record.Company = types.Text(system.CopyStr(change.Company, 1, 100))
		record.Table_no = change.TableID
		record.Table_name = types.Text(system.CopyStr(tableName, 1, 100))
		record.Primary_key = types.Text(system.CopyStr(change.Key, 1, 250))
		record.Field_no = f.No
		record.Field_name = types.Text(system.CopyStr(fieldName, 1, 100))
		record.Type_of_change = gtables.ChangeLogEntryType_of_change(change.Type)
		record.Old_value = types.Text(system.CopyStr(f.Old, 1, 250))
		record.New_value = types.Text(system.CopyStr(f.New, 1, 250))
		records = append(records, record)
	}
	return changeLogEntry.InsertAll(records, false)
}

// englishCaption: the caption, or the name ("password_hash" → "Password Hash") when the
// translation files have none (the translation key came back)
func englishCaption(caption, name string) string {
	if !strings.HasPrefix(caption, "tables.") {
		return caption
	}
	words := strings.Fields(strings.ReplaceAll(name, "_", " "))
	for i, w := range words {
		words[i] = strings.ToUpper(w[:1]) + w[1:]
	}
	return strings.Join(words, " ")
}

// changeLogEntrySequence numbers the entries on PostgreSQL: a sequence hands out numbers to
// concurrent transactions without waiting for each other (a rolled back change leaves a gap)
const changeLogEntrySequence = "Change_Log_Entry$entry_no"

// nextChangeLogEntryNos: n new entry numbers
func nextChangeLogEntryNos(exec database.Executor, dbType database.DBType, n int) ([]int, error) {
	nos := make([]int, 0, n)
	if dbType == database.DBTypePostgres {
		rows, err := exec.Query(`SELECT nextval('"`+changeLogEntrySequence+`"') FROM generate_series(1, $1)`, n)
		if err != nil {
			return nil, err
		}
		defer func() { _ = rows.Close() }()
		for rows.Next() {
			var no int
			err = rows.Scan(&no)
			if err != nil {
				return nil, err
			}
			nos = append(nos, no)
		}
		return nos, rows.Err()
	}
	var last sql.NullInt64
	err := exec.QueryRow(`SELECT MAX(entry_no) FROM "` + gtables.ChangeLogEntryTableName + `"`).Scan(&last)
	if err != nil {
		return nil, err
	}
	for i := 1; i <= n; i++ {
		nos = append(nos, int(last.Int64)+i)
	}
	return nos, nil
}

// EnsureChangeLogSequence makes the PostgreSQL sequence of the entry numbers, starting after
// the last entry (startup, after the table sync)
func EnsureChangeLogSequence(db *sql.DB, dbType database.DBType) error {
	if dbType != database.DBTypePostgres {
		return nil
	}
	_, err := db.Exec(`CREATE SEQUENCE IF NOT EXISTS "` + changeLogEntrySequence + `"`)
	if err != nil {
		return err
	}
	// the next number follows the last entry and the last number handed out, whichever is higher
	_, err = db.Exec(fmt.Sprintf(`SELECT setval('"%[1]s"', GREATEST(
		(SELECT COALESCE(MAX(entry_no), 0) FROM "%[2]s"),
		(SELECT CASE WHEN is_called THEN last_value ELSE last_value - 1 END FROM "%[1]s")) + 1, false)`,
		changeLogEntrySequence, gtables.ChangeLogEntryTableName))
	return err
}

// changeLogSuperOnly refuses a change of the change log setup by a user who is not SUPER
// (code without a user — setup, tests — may)
func changeLogSuperOnly() error {
	access := fcodeunits.CurrentAccess()
	if access != nil && !access.IsSuper {
		return apperrors.ChangeLogSetupSuperOnly()
	}
	return nil
}
