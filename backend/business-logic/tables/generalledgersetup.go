package tables

import (
	fcodeunits "github.com/hansjlachmann/goeasyerp/backend/foundation/codeunits"
	"strconv"
	"strings"

	apperrors "github.com/hansjlachmann/goeasyerp/backend/foundation/errors"
	"github.com/hansjlachmann/goeasyerp/backend/foundation/system"
	"github.com/hansjlachmann/goeasyerp/backend/foundation/types"
	gtables "github.com/hansjlachmann/goeasyerp/backend/generated/tables"
)

// generalLedgerSetupLogic is the business logic of table 98 General_Ledger_Setup.
// Add a trigger when it is needed: the available triggers are listed at the
// top of generated/tables/generalledgersetup_gen.go. The field rules of the definition
// (required, length, range) are checked after OnInsert and OnModify.
//
// Not migrated yet (their tables come later): the tax setup checks of Unrealized Tax and
// Adjust for Payment Disc., Additional Reporting Currency's confirmation and recalculation
// report, the posted-entries check of the rounding precisions (RoundingErrorCheck).
type generalLedgerSetupLogic struct{}

func init() { gtables.RegisterGeneralLedgerSetupLogic(generalLedgerSetupLogic{}) }

// OnInsert runs before a new record is inserted
func (generalLedgerSetupLogic) OnInsert(rec *gtables.GeneralLedgerSetup) error {
	return nil
}

// OnModify runs before a record is modified
func (generalLedgerSetupLogic) OnModify(rec *gtables.GeneralLedgerSetup) error {
	// the dimension names (Dimension-1/-2) may have changed: the captions read them again
	fcodeunits.ForgetDimensionLabels(rec.GetCompany())
	return nil
}

// OnDelete runs before a record is deleted
func (generalLedgerSetupLogic) OnDelete(rec *gtables.GeneralLedgerSetup) error {
	return nil
}

// OnRename runs before a primary key field is changed (rec.OldValue has the old value)
func (generalLedgerSetupLogic) OnRename(rec *gtables.GeneralLedgerSetup) error {
	return nil
}

// OnValidate_Pmt_disc_excl_tax: excluding tax from the payment discount does not go with
// adjusting for it; without it there is no VAT tolerance
func (generalLedgerSetupLogic) OnValidate_Pmt_disc_excl_tax(rec *gtables.GeneralLedgerSetup) error {
	if rec.Pmt_disc_excl_tax {
		return rec.TestField("adjust_for_payment_disc", false)
	}
	return rec.TestField("vat_tolerance_percent", types.ZeroDecimal())
}

// OnValidate_Adjust_for_payment_disc: not with Pmt. Disc. Excl. Tax or a VAT tolerance
func (generalLedgerSetupLogic) OnValidate_Adjust_for_payment_disc(rec *gtables.GeneralLedgerSetup) error {
	if !rec.Adjust_for_payment_disc {
		return nil // the check of the tax setup comes with its tables
	}
	err := rec.TestField("pmt_disc_excl_tax", false)
	if err != nil {
		return err
	}
	return rec.TestField("vat_tolerance_percent", types.ZeroDecimal())
}

// OnValidate_Vat_tolerance_percent: a tolerance needs Pmt. Disc. Excl. Tax and no adjustment
func (generalLedgerSetupLogic) OnValidate_Vat_tolerance_percent(rec *gtables.GeneralLedgerSetup) error {
	if !rec.Vat_tolerance_percent.IsZero() {
		err := rec.TestField("adjust_for_payment_disc", false)
		if err != nil {
			return err
		}
		err = rec.TestField("pmt_disc_excl_tax", true)
		if err != nil {
			return err
		}
	}
	return nil
}

// OnValidate_Inv_rounding_precision_lcy: a multiple of the amount rounding precision
func (generalLedgerSetupLogic) OnValidate_Inv_rounding_precision_lcy(rec *gtables.GeneralLedgerSetup) error {
	if !rec.Amount_rounding_precision.IsZero() &&
		!rec.Inv_rounding_precision_lcy.Equal(system.Round(rec.Inv_rounding_precision_lcy, rec.Amount_rounding_precision)) {
		return apperrors.MustBeRoundedTo(gtables.GeneralLedgerSetupTableName, "inv_rounding_precision_lcy",
			system.Format(rec.Amount_rounding_precision))
	}
	return nil
}

// OnValidate_Amount_rounding_precision rounds the invoice rounding precision to it
func (generalLedgerSetupLogic) OnValidate_Amount_rounding_precision(rec *gtables.GeneralLedgerSetup) error {
	if !rec.Amount_rounding_precision.IsZero() {
		rec.Inv_rounding_precision_lcy = system.Round(rec.Inv_rounding_precision_lcy, rec.Amount_rounding_precision)
	}
	return nil
}

// OnValidate_Amount_decimal_places checks and tidies the format ("2:2")
func (generalLedgerSetupLogic) OnValidate_Amount_decimal_places(rec *gtables.GeneralLedgerSetup) error {
	return CheckDecimalPlacesFormat(gtables.GeneralLedgerSetupTableName, "amount_decimal_places", &rec.Amount_decimal_places)
}

// OnValidate_Unit_amount_decimal_places checks and tidies the format ("2:5")
func (generalLedgerSetupLogic) OnValidate_Unit_amount_decimal_places(rec *gtables.GeneralLedgerSetup) error {
	return CheckDecimalPlacesFormat(gtables.GeneralLedgerSetupTableName, "unit_amount_decimal_places", &rec.Unit_amount_decimal_places)
}

// CheckDecimalPlacesFormat checks a decimal places setting — one number or two numbers
// separated by a colon, each 0–9, the first not above the second ("2", "2:5") — and writes it
// back tidied ("02:05" becomes "2:5"). The General Ledger Setup's procedure, also used by the
// currencies' decimal places fields.
func CheckDecimalPlacesFormat(table, field string, value *types.Text) error {
	part := func(s string) (int, bool) {
		n, err := strconv.Atoi(s)
		return n, err == nil && s != "" && n >= 0 && n <= 9
	}
	min, max, colon := strings.Cut(value.String(), ":")
	a, ok := part(min)
	if !colon {
		if !ok {
			return apperrors.DecimalPlacesFormat(table, field)
		}
		*value = types.NewText(system.Format(a))
		return nil
	}
	b, ok2 := part(max)
	if !ok || !ok2 || a > b {
		return apperrors.DecimalPlacesFormat(table, field)
	}
	*value = types.NewText(system.StrSubstNo("%1:%2", a, b))
	return nil
}
