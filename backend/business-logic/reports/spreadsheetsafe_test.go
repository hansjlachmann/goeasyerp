package reports

import "testing"

func TestSpreadsheetSafe(t *testing.T) {
	for value, want := range map[string]string{
		`=HYPERLINK("http://x.no")`: `'=HYPERLINK("http://x.no")`,
		"+cmd":                      "'+cmd",
		"@SUM(A1)":                  "'@SUM(A1)",
		"-2+3":                      "'-2+3",
		"-1234.50":                  "-1234.50",
		"+12":                       "+12",
		"Rent":                      "Rent",
		"":                          "",
	} {
		got := spreadsheetSafe(value)
		if got != want {
			t.Errorf("spreadsheetSafe(%q) = %q, want %q", value, got, want)
		}
		if back := spreadsheetText(got); back != value {
			t.Errorf("spreadsheetText(%q) = %q, want %q", got, back, value)
		}
	}
	if got := spreadsheetText("'Rent"); got != "'Rent" {
		t.Errorf("a ' before a plain text stays: %q", got)
	}
}
