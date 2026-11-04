package tables

import (
	gtables "github.com/hansjlachmann/goeasyerp/backend/generated/tables"
)

// salesCrMemoLineLogic is the business logic of table 115 Sales Cr.Memo Line: a line of a posted sales credit memo (written by
// sales posting, codeunit 80); it has no triggers of its own.
type salesCrMemoLineLogic struct{}

func init() { gtables.RegisterSalesCrMemoLineLogic(salesCrMemoLineLogic{}) }
