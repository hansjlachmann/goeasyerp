package migrations

import (
	"fmt"

	fmigrations "github.com/hansjlachmann/goeasyerp/backend/foundation/migrations"
)

func init() {
	Register(&Migration053JournalReports{})
}

// Migration053JournalReports goes with the journals' test reports and Post and Print: a journal
// template's type (Recurring) now sets its Test Report ID and Posting Report ID; the templates
// written before (every company) get them where they have none.
type Migration053JournalReports struct{}

func (m *Migration053JournalReports) Version() int {
	return 53
}

func (m *Migration053JournalReports) Name() string {
	return "journal_reports"
}

func (m *Migration053JournalReports) Description() string {
	return "journal templates get their test report and posting report"
}

// journalTemplateReport: a template table with its test report and posting report
type journalTemplateReport struct {
	table         string
	test, posting int
}

// journalTemplateReports: per template table its test report and posting report
var journalTemplateReports = []journalTemplateReport{
	{"Gen_Journal_Template", 2, 3},
	{"Item_Journal_Template", 702, 703},
	{"Res_Journal_Template", 1102, 1103},
	{"Project_Journal_Template", 1202, 1203},
	{"BOM_Journal_Template", 802, 803},
}

func (m *Migration053JournalReports) Up(ctx *fmigrations.Context) error {
	return setJournalTemplateReports(ctx, journalTemplateReports)
}

// setJournalTemplateReports gives the templates of every company their test and posting report
// where they have none
func setJournalTemplateReports(ctx *fmigrations.Context, templates []journalTemplateReport) error {
	for _, t := range templates {
		err := ctx.ForEachCompanyTable(t.table, func(table string) error {
			exists, err := ctx.TableExists(table)
			if err != nil || !exists {
				return err
			}
			for column, id := range map[string]int{"test_report_id": t.test, "posting_report_id": t.posting} {
				err := ctx.ExecuteSQL(fmt.Sprintf(`UPDATE "%s" SET %s = %d WHERE %s = 0 OR %s IS NULL`, table, column, id, column, column))
				if err != nil {
					return fmt.Errorf("%s.%s: %w", table, column, err)
				}
			}
			return nil
		})
		if err != nil {
			return err
		}
	}
	return nil
}
