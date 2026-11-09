package tables

import "github.com/hansjlachmann/goeasyerp/backend/foundation/objects"

// The classic codeunits whose functions live with the tables' logic (ABSOLUTE RULE: every
// business-logic object declares its type, number and name; the tables themselves are objects
// by their YAML)
func init() {
	objects.Codeunit(7, "GLBudget-Open")
	objects.Codeunit(216, "Project Entry-Set Appl. ID")
	objects.Codeunit(311, "Item-Check Avail.")
	objects.Codeunit(312, "Cust-Check Cr. Limit")
	objects.Codeunit(378, "Transfer Extended Text")
	objects.Codeunit(379, "Transfer Old Ext. Text Lines")
	objects.Codeunit(358, "DateFilter-Calc")
	objects.Codeunit(361, "MoveEntries")
	objects.Codeunit(5400, "Available Management")
	objects.Codeunit(5402, "Unit of Measure Management")
	objects.Codeunit(5623, "FA MoveEntries")
}
