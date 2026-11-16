package docmail

import "github.com/hansjlachmann/goeasyerp/backend/foundation/objects"

// The objects this package implements (ABSOLUTE RULE: every business-logic package declares its
// objects — they appear in the Object table and on page 908 Objects)
func init() {
	objects.Codeunit(908, "Document E-mail")
}
