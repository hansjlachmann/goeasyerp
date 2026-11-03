package migrations

import (
	fmigrations "github.com/hansjlachmann/openerp/backend/foundation/migrations"
)

func init() {
	Register(&Migration049Projects{})
}

// Migration049Projects goes with the projects module's master data: Project 167, Phase/Task/
// Step 161–163, Project Budget Line 168 and Entry 211, Project Posting Group 208 and Projects
// Setup 315. The READER role may read them, as all business tables; the tables come from the
// table sync, and G/L Entry 41 Job No. becomes project_no there (same field number).
type Migration049Projects struct{}

func (m *Migration049Projects) Version() int {
	return 49
}

func (m *Migration049Projects) Name() string {
	return "projects"
}

func (m *Migration049Projects) Description() string {
	return "READER reads the projects, their budgets and setup"
}

func (m *Migration049Projects) Up(ctx *fmigrations.Context) error {
	return grantReaderRead(ctx, []string{"PROJECT", "PHASE", "TASK", "STEP", "PROJECT_BUDGET_LINE",
		"PROJECT_BUDGET_ENTRY", "PROJECT_POSTING_GROUP", "PROJECTS_SETUP"})
}
