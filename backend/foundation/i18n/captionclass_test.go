package i18n

import "testing"

// A field caption with a declared caption expression is resolved with it — the dimension area
// puts the company's name for the dimension into the caption, the free text area replaces it —
// and a field caption without one keeps the generic name (field captions declare it; only the
// other texts about a dimension go by their key)
func TestCaptionClass(t *testing.T) {
	saved := DimensionLabel
	t.Cleanup(func() { DimensionLabel = saved })
	DimensionLabel = func(n int) string {
		if n == 1 {
			return "Department"
		}
		return ""
	}
	ts := GetInstance()
	declared := FieldKey("Customer", "dimension_1_code")
	SetCaptionClass(declared, "1,1,1")
	if got := ts.Translate(declared, "en-US"); got != "Department Code" {
		t.Errorf("declared field caption %q, want Department Code", got)
	}
	if got := ts.Translate(declared, "nb-NO"); got == ts.translate(declared, "nb-NO") {
		t.Errorf("nb-NO field caption %q not resolved", got)
	}
	// dimension 2 has no name: the generic caption
	declared2 := FieldKey("Customer", "dimension_2_code")
	SetCaptionClass(declared2, "1,1,2")
	if got, want := ts.Translate(declared2, "en-US"), ts.translate(declared2, "en-US"); got != want {
		t.Errorf("dimension 2 caption %q, want the generic %q", got, want)
	}
	// a field caption without an expression is not rewritten by its key
	undeclared := FieldKey("Vendor", "dimension_1_code")
	captionClassMu.Lock()
	delete(captionClasses, undeclared)
	captionClassMu.Unlock()
	if got, want := ts.Translate(undeclared, "en-US"), ts.translate(undeclared, "en-US"); got != want {
		t.Errorf("undeclared field caption %q, want %q", got, want)
	}
	// other texts about a dimension still go by their key
	if got := ts.Translate("tables.dimension_1.caption", "en-US"); got != "Department" {
		t.Errorf("table caption %q, want Department", got)
	}
	// free text
	if got := ts.ResolveCaptionClass("3,Amount, incl. tax", "Amount", "en-US"); got != "Amount, incl. tax" {
		t.Errorf("free text %q", got)
	}
}

func TestCheckCaptionClass(t *testing.T) {
	for expr, ok := range map[string]bool{
		"1,1,1": true, "1,3,2": true, "1,2,8": true, "3,Text": true,
		"1,1": false, "1,5,1": false, "1,1,9": false, "1,x,1": false, "3,": false, "2,0": false, "9,1": false, "": false,
	} {
		if got := CheckCaptionClass(expr) == ""; got != ok {
			t.Errorf("CheckCaptionClass(%q) valid = %v, want %v", expr, got, ok)
		}
	}
}
