package resprice

import "github.com/hansjlachmann/openerp/backend/foundation/objects"

// The objects this package implements, with their numbers and classic names (ABSOLUTE RULE:
// every business-logic package declares its objects — they appear in the Object table and on
// page 908 Objects)
func init() {
	objects.Codeunit(220, "Resource-Find Cost")
	objects.Codeunit(221, "Resource-Find Price")
}
