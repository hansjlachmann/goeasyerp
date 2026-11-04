package tables

import (
	"errors"
	"strings"
	"testing"

	apperrors "github.com/hansjlachmann/goeasyerp/backend/foundation/errors"
	"github.com/hansjlachmann/goeasyerp/backend/foundation/types"
)

func message(t *testing.T, err error, language string) string {
	t.Helper()
	var appErr *apperrors.AppError
	if !errors.As(err, &appErr) {
		t.Fatalf("error %v is not an AppError", err)
	}
	return appErr.Message(language)
}

// The checks name the field by its caption in the user's language.
func TestChecksAreTranslated(t *testing.T) {
	err := CheckMaxLength("Customer", "post_code", strings.Repeat("1", 21), 20)
	if got := message(t, err, "en-US"); got != "Post Code cannot exceed 20 characters" {
		t.Errorf("en-US: %q", got)
	}
	if got := message(t, err, "nb-NO"); !strings.Contains(got, "20 tegn") || strings.Contains(got, "post_code") {
		t.Errorf("nb-NO: %q, want a Norwegian message with the field caption", got)
	}
	if got := message(t, CheckRequired("Payment Terms", "code", true), "en-US"); got != "Code is required" {
		t.Errorf("required: %q", got)
	}
	if got := message(t, CheckRange("Job_Queue", "minutes_between_run", 0, 1, 1440), "en-US"); !strings.Contains(got, "between 1 and 1440") {
		t.Errorf("range: %q", got)
	}
}

// Lengths count characters, as the database's VARCHAR does: 50 Norwegian letters fit in 50.
func TestCheckMaxLengthCountsCharacters(t *testing.T) {
	name := strings.Repeat("ø", 50)
	if err := CheckMaxLength("Customer", "name", name, 50); err != nil {
		t.Errorf("50 × ø (100 bytes) rejected for a 50-character field: %v", err)
	}
	if err := CheckMaxLength("Customer", "name", name+"x", 50); err == nil {
		t.Error("51 characters accepted for a 50-character field")
	}
	if err := CheckRequired("Customer", "no", false); err != nil {
		t.Errorf("a field with a value: %v", err)
	}
}

// A range with one bound says which bound the value broke; both bounds give the range.
func TestRangeChecks(t *testing.T) {
	d := func(s string) types.Decimal { return types.MustDecimal(s) }
	for _, c := range []struct {
		value    string
		min, max string
		want     string // "" = accepted
	}{
		{"0", "0", "", ""},
		{"0.01", "0", "", ""},
		{"-0.01", "0", "", "Amount Rounding Precision must not be less than 0"},
		{"101", "", "100", "Amount Rounding Precision must not be greater than 100"},
		{"100", "0", "100", ""},
		{"100.5", "0", "100", "Amount Rounding Precision must be between 0 and 100"},
		{"-1", "0", "100", "Amount Rounding Precision must be between 0 and 100"},
	} {
		err := CheckDecimalRange("Currency", "amount_rounding_precision", d(c.value), c.min, c.max)
		switch {
		case c.want == "" && err != nil:
			t.Errorf("%s in %s..%s: %v", c.value, c.min, c.max, err)
		case c.want != "" && err == nil:
			t.Errorf("%s in %s..%s accepted", c.value, c.min, c.max)
		case c.want != "" && message(t, err, "en-US") != c.want:
			t.Errorf("%s in %s..%s: %q, want %q", c.value, c.min, c.max, message(t, err, "en-US"), c.want)
		}
	}
	if err := CheckMin("Job_Queue", "minutes_between_run", 0, 1); err == nil || !strings.Contains(message(t, err, "en-US"), "less than 1") {
		t.Errorf("CheckMin: %v", err)
	}
	if err := CheckMax("Job_Queue", "minutes_between_run", 1441, 1440); err == nil || !strings.Contains(message(t, err, "en-US"), "greater than 1440") {
		t.Errorf("CheckMax: %v", err)
	}
	if CheckMin("Job_Queue", "minutes_between_run", 1, 1) != nil || CheckMax("Job_Queue", "minutes_between_run", 1440, 1440) != nil {
		t.Error("a value on the bound refused")
	}
}

func TestFilterComparison(t *testing.T) {
	for part, want := range map[string]string{"<3000": "<", "<=3000": "<=", ">1": ">", ">=1": ">=", "<>A": "", "A": "", "1..2": ""} {
		if got := FilterComparison(part); got != want {
			t.Errorf("%q: %q, want %q", part, got, want)
		}
	}
}
