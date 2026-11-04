package tables

import (
	gtables "github.com/hansjlachmann/goeasyerp/backend/generated/tables"
)

// insuranceRegisterLogic is the business logic of table 5636 Insurance Register: one per
// insurance journal posting, the range of the coverage ledger entries it wrote (written by
// codeunit 5652 only).
type insuranceRegisterLogic struct{}

func init() { gtables.RegisterInsuranceRegisterLogic(insuranceRegisterLogic{}) }
