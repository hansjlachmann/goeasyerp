package pages

import (
	"github.com/hansjlachmann/openerp/backend/business-logic/accsched"
	"github.com/hansjlachmann/openerp/backend/foundation/pages"
	gtables "github.com/hansjlachmann/openerp/backend/generated/tables"
)

// Page 104 Account Schedule and 489 Account Schedule Column Layout open a schedule / column
// layout (AccSchedManagement OpenSchedule / OpenColumns); page 490 Acc. Schedule Overview is the
// schedule's lines against its column layout's columns, calculated by codeunit 8
func init() {
	pages.OnOpenPage(accsched.SchedulePageID, func(rec *gtables.AccScheduleLine, page *pages.Open) error {
		return accsched.OpenSchedulePage(rec, page)
	})
	pages.OnOpenPage(accsched.ColumnLayoutPageID, func(rec *gtables.AccSchedColumn, page *pages.Open) error {
		return accsched.OpenColumnsPage(rec, page)
	})
	pages.OnMatrixCalculate(accsched.OverviewPageID, func(accSchedName *gtables.AccScheduleName, accSchedLine *gtables.AccScheduleLine) ([]pages.MatrixColumn, map[string][]string, error) {
		return accsched.Overview(accSchedName, accSchedLine)
	})
}
