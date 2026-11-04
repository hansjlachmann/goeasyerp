package bankrecon

import "github.com/hansjlachmann/goeasyerp/backend/foundation/objects"

// The objects this package implements, with their numbers and classic names (ABSOLUTE RULE:
// every business-logic package declares its objects — they appear in the Object table and on
// page 908 Objects)
func init() {
	objects.Codeunit(370, "Bank Acc. Reconciliation Post")
	objects.Codeunit(374, "Bank Acc. Recon. Apply Entries")
	objects.Codeunit(375, "Bank Acc. Entry Set Recon.-No.")
	objects.Codeunit(376, "Check Entry Set Recon.-No.")
}
