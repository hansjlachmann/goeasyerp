package salesposting

import gtables "github.com/hansjlachmann/goeasyerp/backend/generated/tables"

// SetCustomerEntryHook changes the customer entry's journal line before it is posted (a test of
// the balance check); the returned function removes the hook
func SetCustomerEntryHook(fn func(genJnlLine *gtables.GenJournalLine)) func() {
	onCustomerEntry = fn
	return func() { onCustomerEntry = nil }
}
