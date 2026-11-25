package tables

import (
	"database/sql"

	"github.com/hansjlachmann/goeasyerp/backend/foundation/database"
	"github.com/hansjlachmann/goeasyerp/backend/foundation/securitylog"
	ftables "github.com/hansjlachmann/goeasyerp/backend/foundation/tables"
	"github.com/hansjlachmann/goeasyerp/backend/foundation/types"
	gtables "github.com/hansjlachmann/goeasyerp/backend/generated/tables"
)

// securityEventLogic is the business logic of table 2000000158 Security_Event: the security
// events the server records (foundation/securitylog), written in batches by
// SecurityEventWriter; the table API refuses to insert or delete its records (posted)
type securityEventLogic struct{}

func init() {
	gtables.RegisterSecurityEventLogic(securityEventLogic{})
}

// SecurityEventWriter writes a batch of security events into the table, in a transaction of
// its own (an event is kept also when the request it comes from is rolled back), the entry
// numbers following the last one under a table lock
func SecurityEventWriter(db *sql.DB, dbType database.DBType) securitylog.Writer {
	return func(events []securitylog.Event) error {
		tx, err := ftables.BeginTransaction(db)
		if err != nil {
			return err
		}
		err = insertSecurityEvents(tx, dbType, events)
		if err != nil {
			_ = ftables.Rollback(tx)
			return err
		}
		return ftables.Commit(tx)
	}
}

// insertSecurityEvents inserts the events after the last entry
func insertSecurityEvents(tx database.Executor, dbType database.DBType, events []securitylog.Event) error {
	ftables.LockTable(tx, dbType, "", gtables.SecurityEventTableName)
	securityEvent := gtables.NewSecurityEvent(ftables.DBSource(tx, "", dbType))
	entryNo := 0
	if securityEvent.FindLast() {
		entryNo = securityEvent.Entry_no
	}
	records := make([]*gtables.SecurityEvent, 0, len(events))
	for _, e := range events {
		entryNo++
		record := gtables.NewSecurityEvent(securityEvent)
		record.Entry_no = entryNo
		record.Occurred_at = types.NewDateTimeFromTime(e.Time)
		record.Event = gtables.SecurityEventEvent(e.Kind)
		record.User_id = types.NewCode(e.UserID)
		record.Address = types.Text(e.Address)
		record.Company = types.Text(e.Company)
		record.Details = types.Text(e.Details)
		records = append(records, record)
	}
	return securityEvent.InsertAll(records, false)
}

// OnInsert runs before a new record is inserted
func (securityEventLogic) OnInsert(rec *gtables.SecurityEvent) error {
	return nil
}

// OnModify runs before a record is modified
func (securityEventLogic) OnModify(rec *gtables.SecurityEvent) error {
	return nil
}

// OnDelete runs before a record is deleted
func (securityEventLogic) OnDelete(rec *gtables.SecurityEvent) error {
	return nil
}

// OnRename runs before a primary key field is changed (rec.OldValue has the old value)
func (securityEventLogic) OnRename(rec *gtables.SecurityEvent) error {
	return nil
}
