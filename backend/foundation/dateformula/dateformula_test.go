package dateformula

import (
	"testing"
	"time"

	"github.com/hansjlachmann/goeasyerp/backend/foundation/types"
)

func TestApply(t *testing.T) {
	d := types.NewDate(2026, time.January, 31)
	cases := map[string]string{
		"30D":   "2026-03-02",
		"<1M>":  "2026-02-28",
		"cm":    "2026-01-31",
		"-CM":   "2026-01-01",
		"1M+CM": "2026-02-28",
		" 1 M ": "2026-02-28",
		"CM+1D": "2026-02-01",
		"-1Y":   "2025-01-31",
		"WD1":   "2026-02-02",
		"1Q":    "2026-04-30",
		"CY":    "2026-12-31",
	}
	for formula, want := range cases {
		got, err := Apply(formula, d)
		if err != nil || got.Format("2006-01-02") != want {
			t.Errorf("Apply(%q) = %v, %v; want %s", formula, got, err, want)
		}
	}
	if got, err := Apply("1M", types.Date{}); err != nil || !got.IsZero() {
		t.Errorf("blank date: %v, %v", got, err)
	}
	if _, err := Apply("1X", types.Date{}); err == nil {
		t.Error("1X on a blank date must still be an error")
	}
}

func TestValid(t *testing.T) {
	for _, f := range []string{"", "  ", "30D", "<1M+CM>", "-CW", "WD7", "10D+WD5"} {
		if !Valid(f) {
			t.Errorf("Valid(%q) = false", f)
		}
	}
	for _, f := range []string{"30", "D", "1X", "C", "CX", "-WD1", "WD8", "1M+", "30 DAYS", "<>"} {
		if Valid(f) {
			t.Errorf("Valid(%q) = true", f)
		}
	}
}
