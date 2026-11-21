package migrations

import (
	fmigrations "github.com/hansjlachmann/goeasyerp/backend/foundation/migrations"
)

func init() {
	Register(&Migration058BuildsByLocalFirst{})
}

// BuildsByLocalFirstVersion: the migration after which App Build Setup's Builds By holds the
// new option numbers (the server's launcher reads the field before migrations run)
const BuildsByLocalFirstVersion = 58

// Migration058BuildsByLocalFirst goes with App Build Setup's Builds By offering Local Builder
// first (and as the default): the options swapped places, so the stored values swap too —
// Local Builder 1 → 0, CI Pipeline 0 → 1
type Migration058BuildsByLocalFirst struct{}

func (m *Migration058BuildsByLocalFirst) Version() int {
	return BuildsByLocalFirstVersion
}

func (m *Migration058BuildsByLocalFirst) Name() string {
	return "builds_by_local_first"
}

func (m *Migration058BuildsByLocalFirst) Description() string {
	return "App Build Setup: Builds By lists Local Builder first; the stored values swap"
}

func (m *Migration058BuildsByLocalFirst) Up(ctx *fmigrations.Context) error {
	const table = "App_Build_Setup"
	exists, err := ctx.TableExists(table)
	if err != nil || !exists {
		return err
	}
	return ctx.ExecuteSQL(`UPDATE "App_Build_Setup" SET builds_by = 1 - builds_by WHERE builds_by IN (0, 1)`)
}
