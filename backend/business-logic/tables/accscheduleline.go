package tables

import (
	"strings"

	apperrors "github.com/hansjlachmann/goeasyerp/backend/foundation/errors"
	"github.com/hansjlachmann/goeasyerp/backend/foundation/system"
	"github.com/hansjlachmann/goeasyerp/backend/foundation/types"
	gtables "github.com/hansjlachmann/goeasyerp/backend/generated/tables"
)

// accScheduleLineLogic is the business logic of table 85 Acc. Schedule Line.
type accScheduleLineLogic struct{}

func init() { gtables.RegisterAccScheduleLineLogic(accScheduleLineLogic{}) }

// OnInsert: the first line of a schedule that does not exist creates it
func (accScheduleLineLogic) OnInsert(rec *gtables.AccScheduleLine) error {
	xRec := rec.XRec()
	if xRec.Line_no == 0 && rec.Schedule_name != "" {
		accSchedName := gtables.NewAccScheduleName(rec)
		if !accSchedName.Get(rec.Schedule_name) {
			accSchedName.Init()
			accSchedName.Name = rec.Schedule_name
			err := accSchedName.InsertOrError(false)
			if err != nil {
				return err
			}
		}
	}
	return nil
}

// OnValidate_Totaling: a formula is upper case and must parse
func (accScheduleLineLogic) OnValidate_Totaling(rec *gtables.AccScheduleLine) error {
	switch rec.Totaling_type {
	case gtables.AccScheduleLine_Totaling_type.Formula:
		rec.Totaling = types.Text(system.UpperCase(rec.Totaling.String()))
		return CheckAccSchedFormula(rec.Totaling.String())
	}
	return nil
}

// OnValidate_Totaling_type validates Totaling again for the new type
func (accScheduleLineLogic) OnValidate_Totaling_type(rec *gtables.AccScheduleLine) error {
	return rec.Validate("totaling", rec.Totaling)
}

// CheckAccSchedFormula checks a formula of row or column numbers (classic CheckFormula of table
// 85): parentheses in pairs, no two operators in a row, an operand after each operator
func CheckAccSchedFormula(formula string) error {
	parenthesesLevel := 0
	hasOperator := false
	for i := 1; i <= len(formula); i++ {
		c := formula[i-1]
		if c == '(' {
			parenthesesLevel++
		} else {
			if c == ')' {
				parenthesesLevel--
			}
		}
		if parenthesesLevel < 0 {
			return apperrors.AccSchedFormula(apperrors.ErrAccSchedParenthesisMisplaced, i)
		}
		if strings.ContainsRune("+-*/^", rune(c)) {
			if hasOperator {
				return apperrors.AccSchedFormula(apperrors.ErrAccSchedTwoOperators, i)
			}
			hasOperator = true
			if i == len(formula) {
				return apperrors.AccSchedFormula(apperrors.ErrAccSchedOperandMissing, i)
			}
			if formula[i] == ')' {
				return apperrors.AccSchedFormula(apperrors.ErrAccSchedOperandMissing, i)
			}
		} else {
			hasOperator = false
		}
	}
	if parenthesesLevel > 0 {
		return apperrors.AccSchedFormula(apperrors.ErrAccSchedMoreLeftParentheses, 0)
	}
	if parenthesesLevel < 0 {
		return apperrors.AccSchedFormula(apperrors.ErrAccSchedMoreRightParentheses, 0)
	}
	return nil
}
