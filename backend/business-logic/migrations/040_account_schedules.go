package migrations

import (
	fmigrations "github.com/hansjlachmann/openerp/backend/foundation/migrations"
)

func init() {
	Register(&Migration040AccSchedules{})
}

// Migration040AccSchedules goes with the account schedules: Acc. Schedule Name 84, Acc. Schedule
// Line 85, Acc. Sched. Column Layout Name 333 and Acc. Sched. Column 334. The READER role may
// read them, as all business tables; the tables come from the table sync.
type Migration040AccSchedules struct{}

func (m *Migration040AccSchedules) Version() int {
	return 40
}

func (m *Migration040AccSchedules) Name() string {
	return "account-schedules"
}

func (m *Migration040AccSchedules) Description() string {
	return "READER reads the account schedules and column layouts"
}

func (m *Migration040AccSchedules) Up(ctx *fmigrations.Context) error {
	return grantReaderRead(ctx, []string{"ACC_SCHEDULE_NAME", "ACC_SCHEDULE_LINE", "ACC_SCHED_COLUMN_LAYOUT_NAME", "ACC_SCHED_COLUMN"})
}
