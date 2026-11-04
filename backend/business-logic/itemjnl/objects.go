package itemjnl

import "github.com/hansjlachmann/openerp/backend/foundation/objects"

// The objects this package implements, with their numbers and classic names (ABSOLUTE RULE:
// every business-logic package declares its objects — they appear in the Object table and on
// page 908 Objects)
func init() {
	objects.Codeunit(21, "Item Jnl.-Check Line")
	objects.Codeunit(22, "Item Jnl.-Post Line")
	objects.Codeunit(23, "Item Jnl.-Post Batch")
	objects.Codeunit(240, "ItemJnlManagement")
}
