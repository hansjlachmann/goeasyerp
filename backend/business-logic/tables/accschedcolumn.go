package tables

import (
	gtables "github.com/hansjlachmann/goeasyerp/backend/generated/tables"
)

// accSchedColumnLogic is the business logic of table 334 Acc. Sched. Column.
type accSchedColumnLogic struct{}

func init() { gtables.RegisterAccSchedColumnLogic(accSchedColumnLogic{}) }

// OnValidate_Formula: the formula of column numbers must parse
func (accSchedColumnLogic) OnValidate_Formula(rec *gtables.AccSchedColumn) error {
	return CheckAccSchedFormula(rec.Formula.String())
}
