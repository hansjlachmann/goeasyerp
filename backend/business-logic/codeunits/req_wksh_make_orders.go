package codeunits

import (
	"github.com/hansjlachmann/goeasyerp/backend/business-logic/reports"
	fcodeunits "github.com/hansjlachmann/goeasyerp/backend/foundation/codeunits"
	apperrors "github.com/hansjlachmann/goeasyerp/backend/foundation/errors"
	"github.com/hansjlachmann/goeasyerp/backend/foundation/functions"
	"github.com/hansjlachmann/goeasyerp/backend/foundation/session"
	gtables "github.com/hansjlachmann/goeasyerp/backend/generated/tables"
)

// Make Orders on page 295 Req. Wksh. Names (classic form 295): report 493 Make Orders with its
// request page on the worksheet of the line — its requisition lines (template and name in filter
// group 2, as the worksheet page passes them)
func init() {
	functions.Register("table", gtables.RequisitionWkshNameTableName, "MakeOrders", functions.For(ReqWkshNameMakeOrders), functions.WithDialog())
}

// ReqWkshNameMakeOrders runs Make Orders on the worksheet (Execute on report 493 needed)
func ReqWkshNameMakeOrders(rec *gtables.RequisitionWkshName) (fcodeunits.Result, error) {
	if !fcodeunits.CanExecute(session.ObjectReport, reports.MakeOrdersID) {
		return fcodeunits.Result{}, apperrors.NoPermissionReport(reports.MakeOrdersID, reports.Caption(reports.MakeOrdersID, fcodeunits.CurrentLanguage()))
	}
	reqLine := gtables.NewRequisitionLine(rec)
	reqLine.FilterGroup(2)
	reqLine.SetRange("worksheet_template_name", rec.Worksheet_template_name)
	reqLine.SetRange("journal_batch_name", rec.Name)
	reqLine.FilterGroup(0)
	result, err := reports.Execute(reports.MakeOrdersID, rec.GetDB(), rec.GetCompany(), rec.GetDBType(), reports.RunOptions{RequestPage: true, Context: reqLine})
	if err != nil {
		return fcodeunits.Result{}, err
	}
	if result.Cancelled {
		return fcodeunits.Result{Success: true}, nil
	}
	return reports.CodeunitResult(result), nil
}
