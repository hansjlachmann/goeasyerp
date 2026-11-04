package reports

import "github.com/hansjlachmann/openerp/backend/foundation/objects"

// The classic codeunits whose functions live with the reports (ABSOLUTE RULE: every
// business-logic object declares its type, number and name; the reports themselves are objects
// by Register)
func init() {
	objects.Codeunit(5626, "FA General Report")
}
