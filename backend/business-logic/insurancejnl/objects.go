package insurancejnl

import "github.com/hansjlachmann/goeasyerp/backend/foundation/objects"

// The objects this package implements, with their numbers and classic names (ABSOLUTE RULE:
// every business-logic package declares its objects — they appear in the Object table and on
// page 908 Objects)
func init() {
	objects.Codeunit(5651, "Insurance Jnl.-Check Line")
	objects.Codeunit(5652, "Insurance Jnl.-Post Line")
	objects.Codeunit(5653, "Insurance Jnl.-Post Batch")
	objects.Codeunit(5656, "InsuranceJnlManagement")
	objects.Codeunit(5657, "Make Ins. Coverage Ledg. Entry")
}
