package fareclass

import "github.com/hansjlachmann/openerp/backend/foundation/objects"

// The objects this package implements, with their numbers and classic names (ABSOLUTE RULE:
// every business-logic package declares its objects — they appear in the Object table and on
// page 908 Objects)
func init() {
	objects.Codeunit(5641, "FA Reclass. Check Line")
	objects.Codeunit(5642, "FA Reclass. Transfer Line")
	objects.Codeunit(5643, "FA Reclass. Transfer Batch")
	objects.Codeunit(5646, "FAReclassJnlManagement")
}
