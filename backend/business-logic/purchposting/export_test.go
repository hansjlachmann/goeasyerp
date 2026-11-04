package purchposting

import gtables "github.com/hansjlachmann/goeasyerp/backend/generated/tables"

// SetVendorEntryHook changes the vendor entry's journal line before it is posted (a test of the
// balance check); the returned function removes the hook
func SetVendorEntryHook(fn func(genJnlLine *gtables.GenJournalLine)) func() {
	onVendorEntry = fn
	return func() { onVendorEntry = nil }
}
