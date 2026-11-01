package tables

import (
	"github.com/hansjlachmann/openerp/backend/foundation/system"
	"github.com/hansjlachmann/openerp/backend/foundation/types"
	gtables "github.com/hansjlachmann/openerp/backend/generated/tables"
)

// depreciationTableLineLogic is the business logic of table 5643 Depreciation Table Line: a
// period's depreciation % — or, when the table has a Total No. of Units, its units (each kept in
// step with the other).
type depreciationTableLineLogic struct{}

func init() { gtables.RegisterDepreciationTableLineLogic(depreciationTableLineLogic{}) }

// OnValidate_Period_depreciation_percent: the period's units of the table's total
func (depreciationTableLineLogic) OnValidate_Period_depreciation_percent(rec *gtables.DepreciationTableLine) error {
	header := gtables.NewDepreciationTableHeader(rec)
	err := header.GetOrError(rec.Depreciation_table_code)
	if err != nil {
		return err
	}
	if !header.Total_no_of_units.IsZero() {
		rec.No_of_units_in_period = system.Round(header.Total_no_of_units.Mul(rec.Period_depreciation_percent).Div(types.NewDecimalFromInt(100)), types.MustDecimal("0.00001"))
	}
	return nil
}

// OnValidate_No_of_units_in_period: the period's depreciation % of the table's total units
func (depreciationTableLineLogic) OnValidate_No_of_units_in_period(rec *gtables.DepreciationTableLine) error {
	header := gtables.NewDepreciationTableHeader(rec)
	err := header.GetOrError(rec.Depreciation_table_code)
	if err != nil {
		return err
	}
	if !header.Total_no_of_units.IsZero() {
		rec.Period_depreciation_percent = system.Round(rec.No_of_units_in_period.Div(header.Total_no_of_units).Mul(types.NewDecimalFromInt(100)), types.MustDecimal("0.00000001"))
	}
	return nil
}

// NewDeprTableLine (NewRecord): a new line's period no. follows the table's last
func NewDeprTableLine(rec *gtables.DepreciationTableLine) {
	deprTableLine := gtables.NewDepreciationTableLine(rec)
	deprTableLine.SetRange("depreciation_table_code", rec.Depreciation_table_code)
	rec.Period_no = 1
	if deprTableLine.FindLast() {
		rec.Period_no = deprTableLine.Period_no + 1
	}
}
