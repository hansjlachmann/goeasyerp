package pages

import (
	"github.com/hansjlachmann/openerp/backend/business-logic/tables"
	"github.com/hansjlachmann/openerp/backend/foundation/pages"
	"github.com/hansjlachmann/openerp/backend/foundation/types"
	gtables "github.com/hansjlachmann/openerp/backend/generated/tables"
)

// Page 212 Project Budget shows the budget lines of one project: the page parameter's, else
// the first project (classic OnOpenForm / SelectCurrentJobNo). Page 215 Project Quantity
// Allocated: a quantity typed into a cell is the budget line's Quantity in the cell's period
// (SetProjectBudgetLineQuantity).
func init() {
	pages.OnOpenPage(212, func(rec *gtables.ProjectBudgetLine, page *pages.Open) error {
		currentProjectNo := types.Code(page.Param("project"))
		project := gtables.NewProject(rec)
		if currentProjectNo != "" {
			if !project.Get(currentProjectNo) {
				currentProjectNo = ""
			}
		}
		if currentProjectNo == "" {
			if project.FindFirst() {
				currentProjectNo = project.No
			}
		}
		page.SetParam("project", currentProjectNo.String())
		page.SetFilter("project_no", currentProjectNo.String())
		return nil
	})
	pages.OnMatrixValidate(215, func(rec *gtables.ProjectBudgetLine, value types.Decimal) error {
		return tables.SetProjectBudgetLineQuantity(rec, value)
	})
}
