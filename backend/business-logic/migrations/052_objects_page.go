package migrations

import (
	fmigrations "github.com/hansjlachmann/goeasyerp/backend/foundation/migrations"
)

func init() {
	Register(&Migration052ObjectsPage{})
}

// Migration052ObjectsPage goes with page 358 Objects becoming page 908 Objects (the system pages'
// series 900–999): the stored page numbers follow — a user's saved list layout
// (User_Preferences.page_id) and the Setup Checklist's pages (every company). The Object table's
// new Runnable field and the relations to Object come from the table sync; the Object rows are
// written at startup.
type Migration052ObjectsPage struct{}

func (m *Migration052ObjectsPage) Version() int {
	return 52
}

func (m *Migration052ObjectsPage) Name() string {
	return "objects_page"
}

func (m *Migration052ObjectsPage) Description() string {
	return "page 358 Objects is page 908: saved layouts and setup checklist pages follow"
}

// renumberedObjectsPage: the Objects page's old and new number
var renumberedObjectsPage = map[int]int{358: 908}

func (m *Migration052ObjectsPage) Up(ctx *fmigrations.Context) error {
	err := renumber(ctx, "User_Preferences", "page_id", renumberedObjectsPage)
	if err != nil {
		return err
	}
	return ctx.ForEachCompanyTable("Setup_Checklist_Line", func(table string) error {
		return renumber(ctx, table, "page_id", renumberedObjectsPage)
	})
}
