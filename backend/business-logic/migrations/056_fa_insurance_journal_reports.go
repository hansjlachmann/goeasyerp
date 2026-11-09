package migrations

import (
	fmigrations "github.com/hansjlachmann/goeasyerp/backend/foundation/migrations"
)

func init() {
	Register(&Migration056FAInsuranceJournalReports{})
}

// Migration056FAInsuranceJournalReports goes with the fixed asset and insurance journals' test
// reports and Post and Print: their templates' type sets Test Report ID and Posting Report ID
// (5602/5603, 5622/5623); the templates written before (every company) get them where they
// have none, as migration 053 did for the other journals.
type Migration056FAInsuranceJournalReports struct{}

func (m *Migration056FAInsuranceJournalReports) Version() int {
	return 56
}

func (m *Migration056FAInsuranceJournalReports) Name() string {
	return "fa_insurance_journal_reports"
}

func (m *Migration056FAInsuranceJournalReports) Description() string {
	return "fixed asset and insurance journal templates get their test report and posting report"
}

func (m *Migration056FAInsuranceJournalReports) Up(ctx *fmigrations.Context) error {
	return setJournalTemplateReports(ctx, []journalTemplateReport{
		{"FA_Journal_Template", 5602, 5603},
		{"Insurance_Journal_Template", 5622, 5623},
	})
}
