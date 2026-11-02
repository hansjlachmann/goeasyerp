package tables

import (
	gtables "github.com/hansjlachmann/openerp/backend/generated/tables"
)

// The requisition worksheet pages (Form ID of a template)
const (
	ReqWorksheetPageID          = 291
	RecurringReqWorksheetPageID = 296
	ReqWkshTemplateListPageID   = 292
)

// reqWkshTemplateLogic is the business logic of table 244 Req_Wksh_Template.
// Add a trigger when it is needed: the available triggers are listed at the
// top of generated/tables/reqwkshtemplate_gen.go. The field rules of the definition
// (required, length, range) are checked after OnInsert and OnModify.
type reqWkshTemplateLogic struct{}

func init() { gtables.RegisterReqWkshTemplateLogic(reqWkshTemplateLogic{}) }

// OnInsert: the template's page
func (reqWkshTemplateLogic) OnInsert(rec *gtables.ReqWkshTemplate) error {
	return rec.Validate("page_id", rec.Page_id)
}

// OnModify runs before a record is modified
func (reqWkshTemplateLogic) OnModify(rec *gtables.ReqWkshTemplate) error {
	return nil
}

// OnDelete deletes the template's worksheet lines and worksheets
func (reqWkshTemplateLogic) OnDelete(rec *gtables.ReqWkshTemplate) error {
	reqLine := gtables.NewRequisitionLine(rec)
	reqLine.SetRange("worksheet_template_name", rec.Name)
	reqLine.DeleteAll()
	reqWkshName := gtables.NewRequisitionWkshName(rec)
	reqWkshName.SetRange("worksheet_template_name", rec.Name)
	reqWkshName.DeleteAll()
	return nil
}

// OnRename runs before a primary key field is changed (rec.OldValue has the old value)
func (reqWkshTemplateLogic) OnRename(rec *gtables.ReqWkshTemplate) error {
	return nil
}

// OnValidate_Page_id: no page — the page of Recurring
func (reqWkshTemplateLogic) OnValidate_Page_id(rec *gtables.ReqWkshTemplate) error {
	if rec.Page_id == 0 {
		return rec.Validate("recurring", rec.Recurring)
	}
	return nil
}

// OnValidate_Recurring: the Recurring Req. Worksheet or the Req. Worksheet
func (reqWkshTemplateLogic) OnValidate_Recurring(rec *gtables.ReqWkshTemplate) error {
	if rec.Recurring {
		rec.Page_id = RecurringReqWorksheetPageID
	} else {
		rec.Page_id = ReqWorksheetPageID
	}
	return nil
}
