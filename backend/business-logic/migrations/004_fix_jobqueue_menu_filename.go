package migrations

import (
	"github.com/hansjlachmann/goeasyerp/backend/foundation/database"
	fmigrations "github.com/hansjlachmann/goeasyerp/backend/foundation/migrations"
)

func init() {
	Register(&Migration004FixJobQueueMenuFilename{})
}

// Migration004FixJobQueueMenuFilename adds .yaml extension to JOBQUEUE menu filename
type Migration004FixJobQueueMenuFilename struct{}

func (m *Migration004FixJobQueueMenuFilename) Version() int {
	return 4
}

func (m *Migration004FixJobQueueMenuFilename) Name() string {
	return "fix_jobqueue_menu_filename"
}

func (m *Migration004FixJobQueueMenuFilename) Description() string {
	return "Add .yaml extension to JOBQUEUE menu filename"
}

func (m *Migration004FixJobQueueMenuFilename) Up(ctx *fmigrations.Context) error {
	var query string
	if ctx.DBType == database.DBTypePostgres {
		query = `UPDATE "Menu" SET filename = $1 WHERE code = $2`
	} else {
		query = `UPDATE "Menu" SET filename = ? WHERE code = ?`
	}
	return ctx.ExecuteSQL(query, "jobqueue.yaml", "JOBQUEUE")
}
