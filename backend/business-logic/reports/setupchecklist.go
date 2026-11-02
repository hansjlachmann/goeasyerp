package reports

import (
	"github.com/hansjlachmann/openerp/backend/business-logic/setupchecklist"
	gtables "github.com/hansjlachmann/openerp/backend/generated/tables"
)

// Report 26 Setup Checklist: the checklist lines in their order under their application area —
// each table's records in the company, status, who is responsible and when, the number of its
// comments; then the comments of those tables. Run from the Setup Checklist (531).
const SetupChecklistID = 26

func init() {
	Register(SetupChecklistID, "setup-checklist", func() Report { return &setupChecklistReport{} })
}

type setupChecklistReport struct {
	lastAreaName  string
	lastCommentNo int
	commentNos    map[int]int // table → its number in the comments
}

func (r *setupChecklistReport) Definition() *Definition {
	return &Definition{
		OnPreReport: func(run *Run) error {
			setReportHeadValues(run)
			r.commentNos = map[int]int{}
			return nil
		},
		DataItems: []*DataItem{
			{
				Name:                "Setup_Checklist_Line",
				Table:               gtables.SetupChecklistLineTableName,
				Key:                 []string{"line_no"},
				RequestFilterFields: []string{"status"},
				OnPreDataItem: Trigger(func(rec *gtables.SetupChecklistLine, run *Run) error {
					setFilterText(run, rec)
					return nil
				}),
				OnAfterGetRecord: Trigger(r.lineOnAfterGetRecord),
			},
			{
				Name:  "Extra_Setup_Checklist_Line",
				Table: gtables.SetupChecklistLineTableName,
				Key:   []string{"line_no"},
				OnPreDataItem: Trigger(func(rec *gtables.SetupChecklistLine, run *Run) error {
					if len(r.commentNos) == 0 {
						return Break()
					}
					return nil
				}),
				OnAfterGetRecord: Trigger(func(rec *gtables.SetupChecklistLine, run *Run) error {
					commentNo, ok := r.commentNos[rec.Table_id]
					if !ok {
						return Skip()
					}
					run.SetValue("comment_no", commentNo)
					return nil
				}),
				DataItems: []*DataItem{{
					Name:  "Setup_Checklist_Comment",
					Table: gtables.SetupChecklistCommentTableName,
					Key:   []string{"table_id", "line_no"},
					Link:  map[string]string{"table_id": "table_id"},
				}},
			},
		},
	}
}

// lineOnAfterGetRecord: the area heading before the area's first line, the table's records
// and the number of its comments
func (r *setupChecklistReport) lineOnAfterGetRecord(rec *gtables.SetupChecklistLine, run *Run) error {
	rec.CalcFields("application_area_name", "comments")
	areaName := rec.Application_area_name.String()
	run.SetValue("first_line", areaName != r.lastAreaName)
	run.SetValue("area_heading", areaName != r.lastAreaName && r.lastAreaName != "") // space before the next area
	r.lastAreaName = areaName
	run.SetValue("no_of_records", setupchecklist.NoOfRecords(run, run.Company, rec.Table_id))
	commentNo := 0
	if rec.Comments {
		r.lastCommentNo++
		commentNo = r.lastCommentNo
		r.commentNos[rec.Table_id] = commentNo
	}
	run.SetValue("comment_no", commentNo)
	return nil
}
