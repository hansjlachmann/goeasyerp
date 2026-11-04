package migrations

import (
	fmigrations "github.com/hansjlachmann/goeasyerp/backend/foundation/migrations"
)

func init() {
	Register(&Migration015PostingGroups{})
}

// Migration015PostingGroups goes with the posting groups (tables 250, 251, 321, 323, 324) and
// the tax areas (318): the
// READER role may read them, as all business tables. The tables come from the table sync.
type Migration015PostingGroups struct{}

func (m *Migration015PostingGroups) Version() int {
	return 15
}

func (m *Migration015PostingGroups) Name() string {
	return "posting_groups"
}

func (m *Migration015PostingGroups) Description() string {
	return "READER reads the general and tax posting groups, the tax groups and the tax areas"
}

// postingGroupTables: the new tables (permission keys: upper case API names)
var postingGroupTables = []string{
	"GEN_BUSINESS_POSTING_GROUP", "GEN_PRODUCT_POSTING_GROUP", "TAX_GROUP",
	"TAX_BUSINESS_POSTING_GROUP", "TAX_PRODUCT_POSTING_GROUP", "TAX_AREA",
}

func (m *Migration015PostingGroups) Up(ctx *fmigrations.Context) error {
	return grantReaderRead(ctx, postingGroupTables)
}
