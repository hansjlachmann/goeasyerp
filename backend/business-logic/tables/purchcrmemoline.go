package tables

import (
	gtables "github.com/hansjlachmann/goeasyerp/backend/generated/tables"
)

// purchCrMemoLineLogic is the business logic of table 125 Purch. Cr. Memo Line: a line of a posted purchase credit memo
// (written by purchase posting, codeunit 90); it has no triggers of its own.
type purchCrMemoLineLogic struct{}

func init() { gtables.RegisterPurchCrMemoLineLogic(purchCrMemoLineLogic{}) }
