package migrations

import (
	fmigrations "github.com/hansjlachmann/goeasyerp/backend/foundation/migrations"
)

func init() {
	Register(&Migration001Initial{})
}

// Migration001Initial establishes the baseline for the migration system
// This is a no-op migration that marks the existing schema as version 1
type Migration001Initial struct{}

func (m *Migration001Initial) Version() int {
	return 1
}

func (m *Migration001Initial) Name() string {
	return "initial_baseline"
}

func (m *Migration001Initial) Description() string {
	return "Baseline migration - marks existing schema as version 1"
}

func (m *Migration001Initial) Up(ctx *fmigrations.Context) error {
	// No-op - just establishes the baseline
	// The existing table sync (additive columns) continues to work
	// Future migrations will handle more complex schema changes
	return nil
}
