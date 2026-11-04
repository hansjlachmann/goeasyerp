package consolidation

import "github.com/hansjlachmann/goeasyerp/backend/foundation/objects"

// The objects this package implements, with their numbers and classic names (ABSOLUTE RULE:
// every business-logic package declares its objects — they appear in the Object table and on
// page 908 Objects)
func init() {
	objects.Table(220, "Business Unit")
	objects.Page(240, "Business Units")
}
