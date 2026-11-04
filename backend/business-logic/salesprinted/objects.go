package salesprinted

import "github.com/hansjlachmann/goeasyerp/backend/foundation/objects"

// The objects this package implements, with their numbers and classic names (ABSOLUTE RULE:
// every business-logic package declares its objects — they appear in the Object table and on
// page 908 Objects)
func init() {
	objects.Codeunit(313, "Sales-Printed")
	objects.Codeunit(314, "Sales Shpt.-Printed")
	objects.Codeunit(315, "Sales Inv.-Printed")
	objects.Codeunit(316, "Sales Cr. Memo-Printed")
}
