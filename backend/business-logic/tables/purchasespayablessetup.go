package tables

import (
	gtables "github.com/hansjlachmann/goeasyerp/backend/generated/tables"
)

// purchasesPayablesSetupLogic is the business logic of table 312 Purchases & Payables Setup
// (no triggers in the classic table)
type purchasesPayablesSetupLogic struct{}

func init() { gtables.RegisterPurchasesPayablesSetupLogic(purchasesPayablesSetupLogic{}) }
