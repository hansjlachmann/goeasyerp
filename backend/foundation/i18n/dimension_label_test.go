package i18n

import "testing"

// The texts about a dimension name it as the company's General Ledger Setup does
func TestDimensionLabels(t *testing.T) {
	s := GetInstance()
	saved := DimensionLabel
	t.Cleanup(func() { DimensionLabel = saved })
	DimensionLabel = func(n int) string {
		if n == 1 {
			return "Department"
		}
		return "Activity"
	}
	for _, c := range []struct{ key, language, want string }{
		{"tables.customer.fields.dimension_2_code", "en-US", "Activity Code"},
		{"tables.customer.fields.dimension_2_code", "nb-NO", "Activity-kode"},
		{"tables.dimension_1.caption", "en-US", "Department"},
		{"reports.25.texts.INCLUDES_DIMENSION_1", "en-US", "(Includes Department: %1)"},
		{"tables.customer.fields.name", "en-US", "Name"},
	} {
		if got := s.Translate(c.key, c.language); got != c.want {
			t.Errorf("%s (%s): %q, want %q", c.key, c.language, got, c.want)
		}
	}
	// a blank label keeps the generic name
	DimensionLabel = func(int) string { return "" }
	if got := s.Translate("tables.customer.fields.dimension_1_code", "en-US"); got != "Dimension 1 Code" {
		t.Errorf("blank label: %q", got)
	}
}
