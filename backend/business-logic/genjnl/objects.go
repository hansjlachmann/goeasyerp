package genjnl

import "github.com/hansjlachmann/goeasyerp/backend/foundation/objects"

// The objects this package implements, with their numbers and classic names (ABSOLUTE RULE:
// every business-logic package declares its objects — they appear in the Object table and on
// page 908 Objects)
func init() {
	objects.Codeunit(11, "Gen. Jnl.-Check Line")
	objects.Codeunit(12, "Gen. Jnl.-Post Line")
	objects.Codeunit(13, "Gen. Jnl.-Post Batch")
	objects.Codeunit(14, "Gen. Jnl.-Show Entries")
	objects.Codeunit(15, "Gen. Jnl.-Show Card")
	objects.Codeunit(101, "Cust. Entry-SetAppl.ID")
	objects.Codeunit(111, "Vend. Entry-SetAppl.ID")
	objects.Codeunit(225, "Gen. Jnl.-Apply")
	objects.Codeunit(230, "GenJnlManagement")
	objects.Codeunit(366, "Exchange Acc. G/L Journal Line")
}
