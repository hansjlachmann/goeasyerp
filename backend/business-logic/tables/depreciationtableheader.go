package tables

import (
	gtables "github.com/hansjlachmann/openerp/backend/generated/tables"
)

// depreciationTableHeaderLogic is the business logic of table 5642 Depreciation Table Header: a
// user-defined depreciation by periods (month, accounting period, quarter, year).
type depreciationTableHeaderLogic struct{}

func init() { gtables.RegisterDepreciationTableHeaderLogic(depreciationTableHeaderLogic{}) }

// OnDelete deletes the table's lines
func (depreciationTableHeaderLogic) OnDelete(rec *gtables.DepreciationTableHeader) error {
	deprTableLine := gtables.NewDepreciationTableLine(rec)
	deprTableLine.LockTable()
	deprTableLine.SetRange("depreciation_table_code", rec.Code)
	for deprTableLine.FindFirst() {
		err := deprTableLine.DeleteOrError(true)
		if err != nil {
			return err
		}
	}
	return nil
}
