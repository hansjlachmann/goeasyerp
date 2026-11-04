package tables

import (
	"github.com/hansjlachmann/goeasyerp/backend/foundation/system"
	"github.com/hansjlachmann/goeasyerp/backend/foundation/types"
	gtables "github.com/hansjlachmann/goeasyerp/backend/generated/tables"
)

// reportSelectionsLogic is the business logic of table 77 Report_Selections.
// Add a trigger when it is needed: the available triggers are listed at the
// top of generated/tables/reportselections_gen.go. The field rules of the definition
// (required, length, range) are checked after OnInsert and OnModify.
type reportSelectionsLogic struct{}

func init() { gtables.RegisterReportSelectionsLogic(reportSelectionsLogic{}) }

// OnInsert runs before a new record is inserted
func (reportSelectionsLogic) OnInsert(rec *gtables.ReportSelections) error {
	return nil
}

// OnModify runs before a record is modified
func (reportSelectionsLogic) OnModify(rec *gtables.ReportSelections) error {
	return nil
}

// OnDelete runs before a record is deleted
func (reportSelectionsLogic) OnDelete(rec *gtables.ReportSelections) error {
	return nil
}

// OnRename runs before a primary key field is changed (rec.OldValue has the old value)
func (reportSelectionsLogic) OnRename(rec *gtables.ReportSelections) error {
	return nil
}

// NewReportSelection (NewRecord): a new line of a usage takes the sequence after the usage's
// last line, the first '1'
func NewReportSelection(rec *gtables.ReportSelections) {
	reportSelection2 := gtables.NewReportSelections(rec)
	reportSelection2.SetRange("usage", rec.Usage)
	if reportSelection2.FindLast() && reportSelection2.Sequence != "" {
		rec.Sequence = types.Code(system.IncStr(reportSelection2.Sequence.String()))
	} else {
		rec.Sequence = "1"
	}
}
