package migrations

import (
	fmigrations "github.com/hansjlachmann/openerp/backend/foundation/migrations"
)

func init() {
	Register(&Migration042ReportSelectionsRequisition{})
}

// Migration042ReportSelectionsRequisition goes with Report Selections 77, Printer Selection 78
// and the Object table (2000000001), Item Price Change 40, and the requisition worksheets
// (Req. Wksh. Template 244, Requisition Wksh. Name 245, Requisition Line 246, Requisition Method
// 5408). The READER role may read them, as all business tables; the tables come from the table
// sync.
type Migration042ReportSelectionsRequisition struct{}

func (m *Migration042ReportSelectionsRequisition) Version() int {
	return 42
}

func (m *Migration042ReportSelectionsRequisition) Name() string {
	return "report_selections_requisition"
}

func (m *Migration042ReportSelectionsRequisition) Description() string {
	return "READER reads the report and printer selections, item price changes and requisition worksheets"
}

func (m *Migration042ReportSelectionsRequisition) Up(ctx *fmigrations.Context) error {
	return grantReaderRead(ctx, []string{"REPORT_SELECTIONS", "PRINTER_SELECTION", "OBJECT", "ITEM_PRICE_CHANGE",
		"REQ_WKSH_TEMPLATE", "REQUISITION_WKSH_NAME", "REQUISITION_LINE", "REQUISITION_METHOD"})
}
