package pages

import (
	"github.com/hansjlachmann/openerp/backend/business-logic/taxstatement"
	"github.com/hansjlachmann/openerp/backend/foundation/pages"
	"github.com/hansjlachmann/openerp/backend/foundation/system"
	"github.com/hansjlachmann/openerp/backend/foundation/types"
	gtables "github.com/hansjlachmann/openerp/backend/generated/tables"
)

// Page 317 Tax Statement opens a template and one of its statements (TaxStmtManagement); page 474
// Tax Statement Preview shows each row's amount for the Date Filter in its header (open tax
// entries before and within the period, as the classic preview starts)
func init() {
	pages.OnOpenPage(317, func(rec *gtables.TaxStatementLine, page *pages.Open) error {
		return taxstatement.OpenStatementPage(rec, page)
	})
	pages.OnAfterGetRecord(474, func(rec *gtables.TaxStatementLine, page *pages.Control) {
		c := &taxstatement.Calculator{Src: rec, DateFilter: system.PageParam("date_filter"),
			Selection: taxstatement.SelectionOpen, PeriodSelection: taxstatement.BeforeAndWithinPeriod}
		amount, err := c.LineTotal(rec)
		if err != nil {
			amount = types.Decimal{}
		}
		if rec.Print_with == gtables.TaxStatementLine_Print_with.OppositeSign {
			amount = amount.Neg()
		}
		page.SetValue("column_amount", amount)
	})
}
