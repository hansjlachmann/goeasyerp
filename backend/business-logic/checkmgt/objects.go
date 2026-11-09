package checkmgt

import "github.com/hansjlachmann/goeasyerp/backend/foundation/objects"

// The objects this package implements, with their numbers and classic names (ABSOLUTE RULE:
// every business-logic package declares its objects)
func init() {
	objects.Codeunit(367, "CheckManagement")
}
