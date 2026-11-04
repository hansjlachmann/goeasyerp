package migrations

import (
	"fmt"

	"github.com/hansjlachmann/goeasyerp/backend/foundation/database"
	fmigrations "github.com/hansjlachmann/goeasyerp/backend/foundation/migrations"
)

func init() {
	Register(&Migration009WidenJobQueueNotificationEmail{})
}

// Migration009WidenJobQueueNotificationEmail widens Job_Queue.notification_email from
// 100 to 250 characters: it holds one or more addresses separated by ";" now. Table
// sync only adds columns, so the existing VARCHAR(100) columns are altered here (on
// Postgres; SQLite does not enforce the length). Existing addresses are kept as they are.
type Migration009WidenJobQueueNotificationEmail struct{}

func (m *Migration009WidenJobQueueNotificationEmail) Version() int {
	return 9
}

func (m *Migration009WidenJobQueueNotificationEmail) Name() string {
	return "widen_job_queue_notification_email"
}

func (m *Migration009WidenJobQueueNotificationEmail) Description() string {
	return "Job_Queue.notification_email: 250 characters (several addresses)"
}

func (m *Migration009WidenJobQueueNotificationEmail) Up(ctx *fmigrations.Context) error {
	if ctx.DBType != database.DBTypePostgres {
		return nil
	}
	return ctx.ForEachCompanyTable("Job_Queue", func(table string) error {
		// A company entered for the first time gets its tables from the sync after this
		exists, err := ctx.TableExists(table)
		if err != nil || !exists {
			return err
		}
		err = ctx.ChangeColumnType(table, "notification_email", "VARCHAR(250)")
		if err != nil {
			return fmt.Errorf("widen notification_email: %w", err)
		}
		return nil
	})
}
