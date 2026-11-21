package tables

import (
	"encoding/base64"
	"encoding/json"
	"fmt"

	fcodeunits "github.com/hansjlachmann/goeasyerp/backend/foundation/codeunits"
	"github.com/hansjlachmann/goeasyerp/backend/foundation/database"
	"github.com/hansjlachmann/goeasyerp/backend/foundation/functions"
	"github.com/hansjlachmann/goeasyerp/backend/foundation/system"
	ftables "github.com/hansjlachmann/goeasyerp/backend/foundation/tables"
	"github.com/hansjlachmann/goeasyerp/backend/foundation/types"
	gtables "github.com/hansjlachmann/goeasyerp/backend/generated/tables"
)

// bulkDeleteLogLogic is the business logic of table 2000000157 Bulk_Delete_Log: a log of every
// delete of several records at once, written by the delete in its own transaction (the table API
// refuses to insert or delete its records: posted)
type bulkDeleteLogLogic struct{}

func init() {
	gtables.RegisterBulkDeleteLogLogic(bulkDeleteLogLogic{})
	functions.Register("table", gtables.BulkDeleteLogTableName, "ExportKeys", functions.For(BulkDeleteLogExportKeys))
}

// LogBulkDelete writes a bulk delete into the log in the delete's transaction tx (so the delete
// and its log entry commit, or roll back, together): the user, the company, the table, the keys
// deleted and — when all of a list's records were deleted — its filters and search
func LogBulkDelete(tx database.Executor, dbType database.DBType, userID, company, tableName, filters string, keys []string) error {
	ftables.LockTable(tx, dbType, "", gtables.BulkDeleteLogTableName)
	bulkDeleteLog := gtables.NewBulkDeleteLog(ftables.DBSource(tx, "", dbType))
	entryNo := 1
	if bulkDeleteLog.FindLast() {
		entryNo = bulkDeleteLog.Entry_no + 1
	}
	data, err := json.Marshal(keys)
	if err != nil {
		return err
	}
	bulkDeleteLog.Init()
	bulkDeleteLog.Entry_no = entryNo
	bulkDeleteLog.Deleted_at = types.Now()
	bulkDeleteLog.User_id = types.Code(system.CopyStr(userID, 1, 50))
	bulkDeleteLog.Company = types.Text(system.CopyStr(company, 1, 100))
	bulkDeleteLog.Table_name = types.Text(system.CopyStr(tableName, 1, 100))
	bulkDeleteLog.No_of_records = len(keys)
	bulkDeleteLog.Filters = types.Text(system.CopyStr(filters, 1, 250))
	bulkDeleteLog.Deleted_keys = data
	return bulkDeleteLog.InsertOrError(false)
}

// BulkDeleteLogExportKeys gives the keys of a bulk delete's records as a text file, one key per
// line (to find the records in a backup)
func BulkDeleteLogExportKeys(rec *gtables.BulkDeleteLog) (fcodeunits.Result, error) {
	var keys []string
	if len(rec.Deleted_keys) > 0 {
		err := json.Unmarshal(rec.Deleted_keys, &keys)
		if err != nil {
			return fcodeunits.Result{}, err
		}
	}
	text := ""
	for _, key := range keys {
		text += key + "\n"
	}
	return fcodeunits.Result{Success: true, Data: map[string]interface{}{
		"file":     base64.StdEncoding.EncodeToString([]byte(text)),
		"filename": fmt.Sprintf("bulk-delete-%d-%s.txt", rec.Entry_no, rec.Table_name),
	}}, nil
}

// OnInsert runs before a new record is inserted
func (bulkDeleteLogLogic) OnInsert(rec *gtables.BulkDeleteLog) error {
	return nil
}

// OnModify runs before a record is modified
func (bulkDeleteLogLogic) OnModify(rec *gtables.BulkDeleteLog) error {
	return nil
}

// OnDelete runs before a record is deleted
func (bulkDeleteLogLogic) OnDelete(rec *gtables.BulkDeleteLog) error {
	return nil
}

// OnRename runs before a primary key field is changed (rec.OldValue has the old value)
func (bulkDeleteLogLogic) OnRename(rec *gtables.BulkDeleteLog) error {
	return nil
}
