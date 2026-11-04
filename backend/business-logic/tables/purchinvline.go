package tables

import (
	gtables "github.com/hansjlachmann/goeasyerp/backend/generated/tables"
)

// purchInvLineLogic is the business logic of table 123 Purch. Inv. Line: a line of a posted purchase invoice
// (written by purchase posting, codeunit 90); it has no triggers of its own.
type purchInvLineLogic struct{}

func init() { gtables.RegisterPurchInvLineLogic(purchInvLineLogic{}) }
