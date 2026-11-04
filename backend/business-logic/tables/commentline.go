package tables

import (
	"github.com/hansjlachmann/goeasyerp/backend/foundation/system"
	ftables "github.com/hansjlachmann/goeasyerp/backend/foundation/tables"
	"github.com/hansjlachmann/goeasyerp/backend/foundation/types"
	gtables "github.com/hansjlachmann/goeasyerp/backend/generated/tables"
)

// commentLineLogic is the business logic of table 97 Comment Line
type commentLineLogic struct{}

func init() { gtables.RegisterCommentLineLogic(commentLineLogic{}) }

// SetUpNewLine gives the first comment of a record the work date (Comment Sheet, OnNewRecord)
func SetUpNewLine(rec *gtables.CommentLine) {
	commentLine := gtables.NewCommentLine(rec)
	commentLine.SetRange("table_name", rec.Table_name)
	commentLine.SetRange("no", rec.No)
	if !commentLine.FindFirst() {
		rec.Date = system.WorkDate()
	}
}

// DeleteComments deletes the comment lines of a record (a table name, the record's No.) — the
// OnDelete of the tables with comments
func DeleteComments(from ftables.Source, tableName gtables.CommentLineTable_name, no types.Code) {
	commentLine := gtables.NewCommentLine(from)
	commentLine.SetRange("table_name", tableName)
	commentLine.SetRange("no", no)
	commentLine.DeleteAll()
}
