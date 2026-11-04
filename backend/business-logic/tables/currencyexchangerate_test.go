package tables

import (
	"testing"

	"github.com/hansjlachmann/goeasyerp/backend/foundation/database"
	"github.com/hansjlachmann/goeasyerp/backend/foundation/types"
	gtables "github.com/hansjlachmann/goeasyerp/backend/generated/tables"
)

func newRateDB(t *testing.T) (*gtables.Currency, func(cur, date, exch, rel, relCur string, fix gtables.CurrencyExchangeRateFix_exchange_rate_amount)) {
	t.Helper()
	db := newTestDB(t)
	for _, tbl := range []interface {
		CreateTableWithDBType(database.Executor, string, database.DBType) error
	}{&gtables.Currency{}, &gtables.CurrencyExchangeRate{}, &gtables.GeneralLedgerSetup{}} {
		if err := tbl.CreateTableWithDBType(db, "c1", database.DBTypeSQLite); err != nil {
			t.Fatal(err)
		}
	}
	var cur gtables.Currency
	cur.InitWithDBType(db, "c1", database.DBTypeSQLite)
	// the currencies the rates relate to (Relational Currency Code checks them)
	for _, code := range []string{"EUR", "USD"} {
		var c gtables.Currency
		c.InitWithDBType(db, "c1", database.DBTypeSQLite)
		c.InitRecord()
		c.Code = types.Code(code)
		c.Insert(false)
	}
	add := func(code, date, exch, rel, relCur string, fix gtables.CurrencyExchangeRateFix_exchange_rate_amount) {
		var r gtables.CurrencyExchangeRate
		r.InitWithDBType(db, "c1", database.DBTypeSQLite)
		r.Currency_code = types.NewCode(code)
		r.Starting_date = types.MustDate(date)
		r.Exchange_rate_amount = types.MustDecimal(exch)
		r.Adjustment_exch_rate_amount = types.MustDecimal(exch)
		r.Relational_exch_rate_amount = types.MustDecimal(rel)
		r.Relational_adjmt_exch_rate_amt = types.MustDecimal(rel)
		r.Relational_currency_code = types.NewCode(relCur)
		r.Fix_exchange_rate_amount = fix
		if !r.Insert(false) {
			t.Fatalf("insert rate %s %s", code, date)
		}
	}
	return &cur, add
}

func round(d types.Decimal) string { return d.Decimal.Round(6).String() }

// Worked examples: 100 EUR = 1150 local (from 2026-01-01; 1100 before), 744 DKK = 100 EUR
func TestExchangeCalculations(t *testing.T) {
	cur, add := newRateDB(t)
	fix := gtables.CurrencyExchangeRate_Fix_exchange_rate_amount
	add("EUR", "2025-01-01", "100", "1100", "", fix.Currency)
	add("EUR", "2026-01-01", "100", "1150", "", fix.Currency)
	add("DKK", "2026-01-01", "744", "100", "EUR", fix.Currency)
	add("USD", "2026-01-01", "100", "1000", "", fix.Both)
	rates := ExchangeRates(cur)
	d := types.MustDate
	n := types.MustDecimal

	f, err := rates.ExchangeRate(d("2026-03-01"), "EUR")
	if err != nil || round(f) != "0.086957" {
		t.Errorf("ExchangeRate EUR = %v %v, want 100/1150", round(f), err)
	}
	if f, _ := rates.ExchangeRate(d("2025-06-30"), "EUR"); round(f) != "0.090909" {
		t.Errorf("ExchangeRate EUR 2025 = %v, want 100/1100 (the rate valid then)", round(f))
	}
	if f, _ := rates.ExchangeRate(d("2026-03-01"), ""); f.String() != "1" {
		t.Errorf("ExchangeRate of the local currency = %v", f)
	}
	if f, _ := rates.ExchangeRate(d("2026-03-01"), "DKK"); round(f) != "0.646957" {
		t.Errorf("ExchangeRate DKK via EUR = %v, want 744/1150", round(f))
	}

	factor, _ := rates.ExchangeRate(d("2026-03-01"), "EUR")
	if a, _ := rates.ExchangeAmtFCYToLCY(d("2026-03-01"), "EUR", n("100"), factor); round(a) != "1150" {
		t.Errorf("100 EUR in LCY = %v", round(a))
	}
	if a, _ := rates.ExchangeAmtLCYToFCY(d("2026-03-01"), "EUR", n("1150"), factor); round(a) != "100" {
		t.Errorf("1150 LCY in EUR = %v", round(a))
	}
	// Fix Both: the rate's amounts, whatever the factor
	if a, _ := rates.ExchangeAmtFCYToLCY(d("2026-03-01"), "USD", n("100"), n("0.5")); round(a) != "1000" {
		t.Errorf("100 USD (fixed both) in LCY = %v", round(a))
	}
	if a, _ := rates.ExchangeAmtLCYToFCY(d("2026-03-01"), "USD", n("1000"), n("0.5")); round(a) != "100" {
		t.Errorf("1000 LCY in USD (fixed both) = %v", round(a))
	}
	if a, _ := rates.ExchangeAmtFCYToLCYAdjmt(d("2026-03-01"), "USD", n("100"), n("0.5")); round(a) != "1000" {
		t.Errorf("adjustment rates: %v", round(a))
	}
	if a := rates.ExchangeAmtLCYToFCYOnlyFactor(n("1000"), n("0.1")); round(a) != "100" {
		t.Errorf("only factor: %v", round(a))
	}

	for _, c := range []struct {
		from, to types.Code
		amount   string
		want     string
	}{
		{"DKK", "", "744", "1150"}, // via EUR
		{"", "DKK", "1150", "744"},
		{"EUR", "DKK", "100", "744"}, // DKK is quoted in EUR
		{"DKK", "EUR", "744", "100"},
		{"EUR", "", "100", "1150"},
		{"USD", "EUR", "100", "86.956522"}, // 1000 LCY in EUR
		{"EUR", "EUR", "5", "5"},
	} {
		if a, err := rates.ExchangeAmtFCYToFCY(d("2026-03-01"), c.from, c.to, n(c.amount)); err != nil || round(a) != c.want {
			t.Errorf("%s %s → %s = %v %v, want %s", c.amount, c.from, c.to, round(a), err, c.want)
		}
	}

	if _, err := rates.ExchangeRate(d("2024-12-31"), "EUR"); err == nil || message(t, err, "en-US") != "There is no exchange rate for currency EUR on or before 2024-12-31." {
		t.Errorf("no rate: %v", err)
	}
}

func TestCurrencyExchangeRateTriggers(t *testing.T) {
	cur, add := newRateDB(t)
	fix := gtables.CurrencyExchangeRate_Fix_exchange_rate_amount
	add("EUR", "2026-01-01", "100", "1150", "", fix.Both)

	var r gtables.CurrencyExchangeRate
	r.InitWithDBType(cur.GetDB(), cur.GetCompany(), cur.GetDBType())
	r.Currency_code = "EUR"
	if err := r.Validate("starting_date", types.MustDate("2026-02-01")); err != nil {
		t.Fatal(err)
	}
	if r.Relational_exch_rate_amount.String() != "1150" || r.Fix_exchange_rate_amount != fix.Both {
		t.Errorf("a new starting date did not take the previous rate: %v %v", r.Relational_exch_rate_amount, r.Fix_exchange_rate_amount)
	}
	if err := r.Validate("exchange_rate_amount", types.ZeroDecimal()); err == nil {
		t.Error("exchange rate amount 0 accepted")
	}
	if err := r.Validate("relational_currency_code", types.NewCode("EUR")); err == nil ||
		message(t, err, "en-US") != "Relational Currency Code cannot be the same as the currency code." {
		t.Errorf("same currency: %v", err)
	}

	// Deleting a currency deletes its rates
	if !cur.Get("EUR") || !cur.Delete(true) {
		t.Fatal("currency insert/delete failed")
	}
	var left gtables.CurrencyExchangeRate
	left.InitWithDBType(cur.GetDB(), cur.GetCompany(), cur.GetDBType())
	left.SetRange("currency_code", "EUR")
	if left.FindFirst() {
		t.Error("the currency's exchange rates were not deleted")
	}
}

func TestGeneralLedgerSetup(t *testing.T) {
	cur, _ := newRateDB(t)
	var s gtables.GeneralLedgerSetup
	s.InitRecord()
	if s.Amount_decimal_places.String() != "2:2" || s.Local_cont_addr_format != gtables.GeneralLedgerSetup_Local_cont_addr_format.AfterCompanyName {
		t.Errorf("defaults: %q %v", s.Amount_decimal_places, s.Local_cont_addr_format)
	}
	if err := s.Validate("pmt_disc_excl_tax", true); err != nil {
		t.Error(err)
	}
	if err := s.Validate("adjust_for_payment_disc", true); err == nil {
		t.Error("Adjust for Payment Disc. accepted with Pmt. Disc. Excl. Tax")
	}
	s.Adjust_for_payment_disc = false
	if err := s.Validate("vat_tolerance_percent", types.MustDecimal("2")); err != nil {
		t.Errorf("VAT tolerance with Pmt. Disc. Excl. Tax: %v", err)
	}
	if err := s.Validate("pmt_disc_excl_tax", false); err == nil {
		t.Error("Pmt. Disc. Excl. Tax off accepted with a VAT tolerance")
	}
	s.Amount_rounding_precision = types.MustDecimal("0.01")
	if err := s.Validate("inv_rounding_precision_lcy", types.MustDecimal("0.005")); err == nil {
		t.Error("invoice rounding 0.005 accepted with amount rounding 0.01")
	}
	s.Inv_rounding_precision_lcy = types.MustDecimal("0.75")
	if err := s.Validate("amount_rounding_precision", types.MustDecimal("0.5")); err != nil || s.Inv_rounding_precision_lcy.String() != "1" {
		t.Errorf("amount rounding rounds the invoice rounding: %v %v", s.Inv_rounding_precision_lcy, err)
	}
	if err := s.Validate("unit_amount_decimal_places", types.NewText("02:05")); err != nil || s.Unit_amount_decimal_places.String() != "2:5" {
		t.Errorf("decimal places: %v %q", err, s.Unit_amount_decimal_places)
	}

	// InitRoundingPrecision: the setup's precisions, else the defaults
	InitRoundingPrecision(cur)
	if cur.Amount_rounding_precision.String() != "0.01" || cur.Unit_amount_rounding_precision.String() != "0.00001" {
		t.Errorf("without a setup: %v %v", cur.Amount_rounding_precision, cur.Unit_amount_rounding_precision)
	}
	var setup gtables.GeneralLedgerSetup
	setup.InitWithDBType(cur.GetDB(), cur.GetCompany(), cur.GetDBType())
	setup.InitRecord()
	setup.Amount_rounding_precision = types.MustDecimal("1")
	setup.Unit_amount_rounding_precision = types.MustDecimal("0.001")
	if !setup.Insert(false) {
		t.Fatal("insert setup")
	}
	InitRoundingPrecision(cur)
	if cur.Amount_rounding_precision.String() != "1" || cur.Unit_amount_rounding_precision.String() != "0.001" {
		t.Errorf("from the setup: %v %v", cur.Amount_rounding_precision, cur.Unit_amount_rounding_precision)
	}
}
