package fadepr

import "github.com/hansjlachmann/openerp/backend/foundation/objects"

// The objects this package implements, with their numbers and classic names (ABSOLUTE RULE:
// every business-logic package declares its objects — they appear in the Object table and on
// page 908 Objects)
func init() {
	objects.Codeunit(5610, "Calculate Depreciation")
	objects.Codeunit(5611, "Calculate Normal Depreciation")
	objects.Codeunit(5612, "Calculate Custom 1 Depr.")
	objects.Codeunit(5613, "Calculate Acq. Cost Depr.")
	objects.Codeunit(5617, "FA Date Calculation")
	objects.Codeunit(5618, "Table Depr. Calculation")
}
