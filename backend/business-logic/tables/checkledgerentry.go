package tables

import (
	gtables "github.com/hansjlachmann/goeasyerp/backend/generated/tables"
)

// checkLedgerEntryLogic is the business logic of table 272 Check Ledger Entry (no triggers in
// the classic table)
type checkLedgerEntryLogic struct{}

func init() { gtables.RegisterCheckLedgerEntryLogic(checkLedgerEntryLogic{}) }
