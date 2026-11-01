package tables

import (
	gtables "github.com/hansjlachmann/openerp/backend/generated/tables"
)

// fAJournalSetupLogic is the business logic of table 5605 FA Journal Setup: per depreciation
// book and user (a blank user: everyone else), the FA journal and the general journal the batch
// functions (Calculate Depreciation …) fill.
type fAJournalSetupLogic struct{}

func init() { gtables.RegisterFAJournalSetupLogic(fAJournalSetupLogic{}) }

// OnValidate_Fa_jnl_template_name: another template, no batch
func (fAJournalSetupLogic) OnValidate_Fa_jnl_template_name(rec *gtables.FAJournalSetup) error {
	rec.Fa_jnl_batch_name = ""
	return nil
}

// OnValidate_Gen_jnl_template_name: another template, no batch
func (fAJournalSetupLogic) OnValidate_Gen_jnl_template_name(rec *gtables.FAJournalSetup) error {
	rec.Gen_jnl_batch_name = ""
	return nil
}
