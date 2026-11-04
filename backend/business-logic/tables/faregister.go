package tables

import (
	gtables "github.com/hansjlachmann/goeasyerp/backend/generated/tables"
)

// fARegisterLogic is the business logic of table 5617 FA Register: written by the fixed asset
// posting only (one per posting, its FA and maintenance ledger entry ranges).
type fARegisterLogic struct{}

func init() { gtables.RegisterFARegisterLogic(fARegisterLogic{}) }
