package tables

import (
	apperrors "github.com/hansjlachmann/goeasyerp/backend/foundation/errors"
	"github.com/hansjlachmann/goeasyerp/backend/foundation/system"
	"github.com/hansjlachmann/goeasyerp/backend/foundation/types"
	gtables "github.com/hansjlachmann/goeasyerp/backend/generated/tables"
)

// currencyLogic is the business logic of table 4 Currency.
// Add a trigger when it is needed: the available triggers are listed at the
// top of generated/tables/currency_gen.go. The field rules of the definition
// (required, length, range) are checked after OnInsert and OnModify.
//
// Not migrated yet (their tables come later): the G/L account fields and their checks.
type currencyLogic struct{}

func init() { gtables.RegisterCurrencyLogic(currencyLogic{}) }

// OnInsert runs before a new record is inserted
func (currencyLogic) OnInsert(rec *gtables.Currency) error {
	return nil
}

// OnModify runs before a record is modified
func (currencyLogic) OnModify(rec *gtables.Currency) error {
	rec.Last_date_modified = system.Today()
	return nil
}

// OnDelete deletes the currency's exchange rates
func (currencyLogic) OnDelete(rec *gtables.Currency) error {
	currExchRate := gtables.NewCurrencyExchangeRate(rec)
	currExchRate.SetRange("currency_code", rec.Code)
	currExchRate.DeleteAll()
	return nil
}

// OnRename runs before a primary key field is changed (rec.OldValue has the old value)
func (currencyLogic) OnRename(rec *gtables.Currency) error {
	rec.Last_date_modified = system.Today()
	return nil
}

// OnValidate_Invoice_rounding_precision: the invoice rounding precision must be a multiple of
// the amount rounding precision
func (currencyLogic) OnValidate_Invoice_rounding_precision(rec *gtables.Currency) error {
	if !rec.Amount_rounding_precision.IsZero() &&
		!rec.Invoice_rounding_precision.Equal(system.Round(rec.Invoice_rounding_precision, rec.Amount_rounding_precision)) {
		return apperrors.MustBeRoundedTo(gtables.CurrencyTableName, "invoice_rounding_precision",
			system.Format(rec.Amount_rounding_precision))
	}
	return nil
}

// OnValidate_Amount_rounding_precision rounds the invoice rounding precision to the new
// amount rounding precision
func (currencyLogic) OnValidate_Amount_rounding_precision(rec *gtables.Currency) error {
	if !rec.Amount_rounding_precision.IsZero() {
		rec.Invoice_rounding_precision = system.Round(rec.Invoice_rounding_precision, rec.Amount_rounding_precision)
	}
	return nil
}

// OnValidate_Amount_decimal_places checks and tidies the format ("2:2")
func (currencyLogic) OnValidate_Amount_decimal_places(rec *gtables.Currency) error {
	return CheckDecimalPlacesFormat(gtables.CurrencyTableName, "amount_decimal_places", &rec.Amount_decimal_places)
}

// OnValidate_Unit_amount_decimal_places checks and tidies the format ("2:5")
func (currencyLogic) OnValidate_Unit_amount_decimal_places(rec *gtables.Currency) error {
	return CheckDecimalPlacesFormat(gtables.CurrencyTableName, "unit_amount_decimal_places", &rec.Unit_amount_decimal_places)
}

// InitRoundingPrecision sets a currency's amount and unit-amount rounding precisions to the
// General Ledger Setup's, or 0.01 / 0.00001 when the setup has none (a currency record used
// for the local currency)
func InitRoundingPrecision(currency *gtables.Currency) {
	glSetup := gtables.GetGeneralLedgerSetup(currency)
	if !glSetup.Amount_rounding_precision.IsZero() {
		currency.Amount_rounding_precision = glSetup.Amount_rounding_precision
	} else {
		currency.Amount_rounding_precision = types.MustDecimal("0.01")
	}
	if !glSetup.Unit_amount_rounding_precision.IsZero() {
		currency.Unit_amount_rounding_precision = glSetup.Unit_amount_rounding_precision
	} else {
		currency.Unit_amount_rounding_precision = types.MustDecimal("0.00001")
	}
}
