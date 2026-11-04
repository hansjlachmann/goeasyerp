package fajnl

import "github.com/hansjlachmann/openerp/backend/foundation/objects"

// The objects this package implements, with their numbers and classic names (ABSOLUTE RULE:
// every business-logic package declares its objects — they appear in the Object table and on
// page 908 Objects)
func init() {
	objects.Codeunit(5600, "FA Insert Ledger Entry")
	objects.Codeunit(5601, "FA Insert G/L Account")
	objects.Codeunit(5602, "FA Get G/L Account No.")
	objects.Codeunit(5604, "Make FA Ledger Entry")
	objects.Codeunit(5605, "Calculate Disposal")
	objects.Codeunit(5606, "FA Check Consistency")
	objects.Codeunit(5607, "FA Automatic Entry")
	objects.Codeunit(5624, "Cancel FA Ledger Entries")
	objects.Codeunit(5631, "FA Jnl.-Check Line")
	objects.Codeunit(5632, "FA Jnl.-Post Line")
	objects.Codeunit(5633, "FA Jnl.-Post Batch")
	objects.Codeunit(5638, "FAJnlManagement")
	objects.Codeunit(5639, "FA Get Journal")
	objects.Codeunit(5640, "Duplicate Depr. Book")
	objects.Codeunit(5647, "Make Maintenance Ledger Entry")
}
