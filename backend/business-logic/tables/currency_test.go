package tables

import (
	"testing"

	"github.com/hansjlachmann/goeasyerp/backend/foundation/database"
	apperrors "github.com/hansjlachmann/goeasyerp/backend/foundation/errors"
	"github.com/hansjlachmann/goeasyerp/backend/foundation/system"
	"github.com/hansjlachmann/goeasyerp/backend/foundation/types"
	gtables "github.com/hansjlachmann/goeasyerp/backend/generated/tables"
)

func TestCurrencyLastDateModified(t *testing.T) {
	db := newTestDB(t)
	if err := (&gtables.Currency{}).CreateTableWithDBType(db, "c1", database.DBTypeSQLite); err != nil {
		t.Fatal(err)
	}
	var c gtables.Currency
	c.InitWithDBType(db, "c1", database.DBTypeSQLite)
	c.InitRecord()
	c.Code = types.NewCode("EUR")
	if !c.Insert(true) {
		t.Fatalf("insert: %v", c.TriggerError())
	}
	if !c.Last_date_modified.IsZero() {
		t.Errorf("insert set Last Date Modified to %v", c.Last_date_modified)
	}
	c.Description = types.NewText("Euro")
	if !c.Modify(true) {
		t.Fatalf("modify: %v", c.TriggerError())
	}
	var read gtables.Currency
	read.InitWithDBType(db, "c1", database.DBTypeSQLite)
	if !read.Get("EUR") || !read.Last_date_modified.Equal(system.Today()) {
		t.Errorf("after modify Last Date Modified = %v, want today", read.Last_date_modified)
	}
}

func TestCurrencyRoundingPrecision(t *testing.T) {
	d := types.MustDecimal
	var c gtables.Currency
	c.InitRecord() // invoice 1, amount 0.01
	if err := c.Validate("invoice_rounding_precision", d("0.05")); err != nil {
		t.Errorf("0.05 to the nearest 0.01: %v", err)
	}
	err := c.Validate("invoice_rounding_precision", d("0.005"))
	if err == nil || message(t, err, "en-US") != "Invoice Rounding Precision must be rounded to the nearest 0.01." {
		t.Errorf("0.005: %v", err)
	}
	c.Invoice_rounding_precision = d("0.75")
	if err := c.Validate("amount_rounding_precision", d("0.5")); err != nil {
		t.Fatal(err)
	}
	if !c.Invoice_rounding_precision.Equal(d("1")) {
		t.Errorf("invoice precision after amount precision 0.5 = %v, want 1", c.Invoice_rounding_precision)
	}
	c.Invoice_rounding_precision = d("0.75")
	if err := c.Validate("amount_rounding_precision", d("0")); err != nil || !c.Invoice_rounding_precision.Equal(d("0.75")) {
		t.Errorf("amount precision 0 changed the invoice precision: %v %v", err, c.Invoice_rounding_precision)
	}
}

func TestCheckDecimalPlacesFormat(t *testing.T) {
	for in, want := range map[string]string{"2": "2", "02": "2", "2:5": "2:5", "02:05": "2:5", "0:9": "0:9", "3:3": "3:3"} {
		v := types.NewText(in)
		if err := CheckDecimalPlacesFormat("Currency", "amount_decimal_places", &v); err != nil || v.String() != want {
			t.Errorf("%q: %q %v, want %q", in, v.String(), err, want)
		}
	}
	for _, in := range []string{"", ":", "2:", ":5", "5:2", "10", "2:10", "-1", "a:b", "2.5"} {
		v := types.NewText(in)
		if err := CheckDecimalPlacesFormat("Currency", "amount_decimal_places", &v); err == nil {
			t.Errorf("%q accepted", in)
		} else if got := message(t, err, "en-US"); got != "Amount Decimal Places: enter one number or two numbers separated by a colon, from 0 to 9 (e.g. 2:5)." {
			t.Errorf("%q: %q", in, got)
		}
	}
	var c gtables.Currency
	if err := c.Validate("unit_amount_decimal_places", types.NewText("02:05")); err != nil || c.Unit_amount_decimal_places.String() != "2:5" {
		t.Errorf("validate: %v %q", err, c.Unit_amount_decimal_places.String())
	}
}

func message(t *testing.T, err error, language string) string {
	t.Helper()
	appErr, ok := err.(*apperrors.AppError)
	if !ok {
		t.Fatalf("error %v is not an AppError", err)
	}
	return appErr.Message(language)
}
