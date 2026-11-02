package tables

import (
	apperrors "github.com/hansjlachmann/openerp/backend/foundation/errors"
	gtables "github.com/hansjlachmann/openerp/backend/generated/tables"
)

// requisitionWkshNameLogic is the business logic of table 245 Requisition_Wksh_Name.
// Add a trigger when it is needed: the available triggers are listed at the
// top of generated/tables/requisitionwkshname_gen.go. The field rules of the definition
// (required, length, range) are checked after OnInsert and OnModify.
type requisitionWkshNameLogic struct{}

func init() { gtables.RegisterRequisitionWkshNameLogic(requisitionWkshNameLogic{}) }

// OnInsert: the worksheet's template exists
func (requisitionWkshNameLogic) OnInsert(rec *gtables.RequisitionWkshName) error {
	rec.LockTable()
	reqTemplate := gtables.NewReqWkshTemplate(rec)
	return reqTemplate.GetOrError(rec.Worksheet_template_name)
}

// OnModify runs before a record is modified
func (requisitionWkshNameLogic) OnModify(rec *gtables.RequisitionWkshName) error {
	return nil
}

// OnDelete deletes the worksheet's lines
func (requisitionWkshNameLogic) OnDelete(rec *gtables.RequisitionWkshName) error {
	reqLine := gtables.NewRequisitionLine(rec)
	reqLine.SetRange("worksheet_template_name", rec.Worksheet_template_name)
	reqLine.SetRange("journal_batch_name", rec.Name)
	reqLine.DeleteAll()
	return nil
}

// OnRename: the worksheet's lines move with it
func (requisitionWkshNameLogic) OnRename(rec *gtables.RequisitionWkshName) error {
	xRec := rec.XRec()
	reqLine := gtables.NewRequisitionLine(rec)
	reqLine.SetRange("worksheet_template_name", xRec.Worksheet_template_name)
	reqLine.SetRange("journal_batch_name", xRec.Name)
	for reqLine.FindFirst() {
		if !reqLine.Rename(rec.Worksheet_template_name, rec.Name, reqLine.Line_no) {
			err := reqLine.TriggerError()
			if err == nil {
				err = apperrors.TableModifyFailed(gtables.RequisitionLineTableName, reqLine.Journal_batch_name.String())
			}
			return err
		}
	}
	return nil
}
