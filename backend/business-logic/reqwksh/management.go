// Package reqwksh is the requisition worksheet: codeunit 330 ReqJnlManagement (the template and
// worksheet selection, the footer's description and vendor name, a new line's defaults) and
// codeunit 333 Req. Wksh.-Make Order (purchase orders of the worksheet's lines).
package reqwksh

import (
	"github.com/hansjlachmann/openerp/backend/business-logic/tables"
	fcodeunits "github.com/hansjlachmann/openerp/backend/foundation/codeunits"
	"github.com/hansjlachmann/openerp/backend/foundation/i18n"
	"github.com/hansjlachmann/openerp/backend/foundation/pages"
	"github.com/hansjlachmann/openerp/backend/foundation/types"
	gtables "github.com/hansjlachmann/openerp/backend/generated/tables"
)

// OpenJournalPage opens a requisition worksheet page (TemplateSelection and OpenJnl): the
// template in the page's parameters, the only template of the kind (recurring or not), a new
// one when there is none (REQ / RECURRING), or the user picks one (page 292); then the
// worksheet — the given one, else the template's first, a new DEFAULT one when it has none —
// as the page's filters
func OpenJournalPage(rec *gtables.RequisitionLine, page *pages.Open, recurringJnl bool) error {
	language := fcodeunits.CurrentLanguage()
	reqTemplate := gtables.NewReqWkshTemplate(rec)
	if page.Param("template") != "" {
		err := reqTemplate.GetOrError(types.Code(page.Param("template")))
		if err != nil {
			return err
		}
	} else {
		reqTemplate.SetRange("recurring", recurringJnl)
		switch reqTemplate.Count() {
		case 0:
			reqTemplate.Init()
			reqTemplate.Recurring = recurringJnl
			if !recurringJnl {
				reqTemplate.Name = "REQ."
				reqTemplate.Description = types.Text(i18n.GetInstance().Message("REQ_WKSH_TEMPLATE_DESCRIPTION", language))
			} else {
				reqTemplate.Name = "RECURRING"
				reqTemplate.Description = types.Text(i18n.GetInstance().Message("RECURRING_REQ_WKSH_TEMPLATE_DESCRIPTION", language))
			}
			err := reqTemplate.Validate("page_id", 0)
			if err != nil {
				return err
			}
			err = reqTemplate.InsertOrError(true)
			if err != nil {
				return err
			}
		case 1:
			reqTemplate.FindFirst()
		default:
			page.SelectRecord(tables.ReqWkshTemplateListPageID, "template", "name", pages.FixedFilter{Field: "recurring", Value: recurringJnl})
			return nil
		}
	}
	page.SetParam("template", reqTemplate.Name.String())
	page.SetFilter("worksheet_template_name", reqTemplate.Name.String())

	currentJnlBatchName := types.Code(page.Param("batch"))
	err := checkTemplateName(rec, reqTemplate.Name, &currentJnlBatchName)
	if err != nil {
		return err
	}
	page.SetParam("batch", currentJnlBatchName.String())
	page.SetFilter("journal_batch_name", currentJnlBatchName.String())
	return nil
}

// checkTemplateName (CheckTemplateName): the worksheet exists in the template, or the
// template's first worksheet is taken — a new DEFAULT worksheet when the template has none
func checkTemplateName(rec *gtables.RequisitionLine, currentJnlTemplateName types.Code, currentJnlBatchName *types.Code) error {
	reqWkshName := gtables.NewRequisitionWkshName(rec)
	reqWkshName.SetRange("worksheet_template_name", currentJnlTemplateName)
	if !reqWkshName.Get(map[string]interface{}{"worksheet_template_name": currentJnlTemplateName, "name": *currentJnlBatchName}) {
		if !reqWkshName.FindFirst() {
			reqWkshName.Init()
			reqWkshName.Worksheet_template_name = currentJnlTemplateName
			reqWkshName.Name = "DEFAULT"
			reqWkshName.Description = types.Text(i18n.GetInstance().Message("GEN_JNL_DEFAULT_BATCH", fcodeunits.CurrentLanguage()))
			err := reqWkshName.InsertOrError(true)
			if err != nil {
				return err
			}
		}
		*currentJnlBatchName = reqWkshName.Name
	}
	return nil
}

// GetDescriptionAndRcptName (GetDescriptionAndRcptName): the description of the line's account or
// item and the name of its vendor (the worksheet's footer)
func GetDescriptionAndRcptName(reqWkshLine *gtables.RequisitionLine) (description, buyFromVendorName string) {
	if reqWkshLine.No != "" {
		switch reqWkshLine.Type {
		case gtables.RequisitionLine_Type.AccountGL:
			glAcc := gtables.NewGLAccount(reqWkshLine)
			if glAcc.Get(reqWkshLine.No) {
				description = glAcc.Name.String()
			}
		case gtables.RequisitionLine_Type.Item:
			item := gtables.NewItem(reqWkshLine)
			if item.Get(reqWkshLine.No) {
				description = item.Description.String()
			}
		}
	}
	if reqWkshLine.Vendor_no != "" {
		vend := gtables.NewVendor(reqWkshLine)
		if vend.Get(reqWkshLine.Vendor_no) {
			buyFromVendorName = vend.Name.String()
		}
	}
	return description, buyFromVendorName
}

// SetUpNewLine (SetUpNewLine): a new line takes the type, recurring method and order date of the
// line above
func SetUpNewLine(reqWkshLine, lastReqWkshLine *gtables.RequisitionLine) {
	reqWkshLine.Type = lastReqWkshLine.Type
	reqWkshLine.Recurring_method = lastReqWkshLine.Recurring_method
	reqWkshLine.Order_date = lastReqWkshLine.Order_date
}
