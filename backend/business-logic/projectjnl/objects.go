package projectjnl

import "github.com/hansjlachmann/openerp/backend/foundation/objects"

// The objects this package implements, with their numbers and classic names (ABSOLUTE RULE:
// every business-logic package declares its objects — they appear in the Object table and on
// page 908 Objects)
func init() {
	objects.Codeunit(201, "Project Jnl.-Check Line")
	objects.Codeunit(202, "Project Jnl.-Post Line")
	objects.Codeunit(203, "Project Jnl.-Post Batch")
	objects.Codeunit(280, "ProjectJnlManagement")
}
