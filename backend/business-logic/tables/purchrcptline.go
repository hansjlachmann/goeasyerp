package tables

import (
	gtables "github.com/hansjlachmann/goeasyerp/backend/generated/tables"
)

// purchRcptLineLogic is the business logic of table 121 Purch. Rcpt. Line: a line of a posted purchase receipt
// (written by purchase posting, codeunit 90); it has no triggers of its own.
type purchRcptLineLogic struct{}

func init() { gtables.RegisterPurchRcptLineLogic(purchRcptLineLogic{}) }
