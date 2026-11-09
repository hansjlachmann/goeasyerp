package bomjnl

import "github.com/hansjlachmann/goeasyerp/backend/foundation/objects"

// The objects this package implements, with their numbers and classic names (ABSOLUTE RULE:
// every business-logic package declares its objects — they appear in the Object table and on
// page 908 Objects)
func init() {
	objects.Codeunit(31, "BOM Jnl.-Check Line")
	objects.Codeunit(32, "BOM Jnl.-Post Line")
	objects.Codeunit(33, "BOM Jnl.-Post Batch")
	objects.Codeunit(51, "BOM-Explode BOM")
	objects.Codeunit(54, "BOM-Calculate Cost")
	objects.Codeunit(63, "Sales-Explode BOM")
	objects.Codeunit(73, "Purch.-Explode BOM")
	objects.Codeunit(246, "Item Jnl.-Explode BOM")
	objects.Codeunit(250, "BOMJnlManagement")
	objects.Codeunit(255, "BOM Reg.-Show Item Ledger")
	objects.Codeunit(256, "BOM Reg.-Show Res. Ledger")
}
