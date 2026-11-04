package migrations

import (
	"fmt"

	fmigrations "github.com/hansjlachmann/goeasyerp/backend/foundation/migrations"
)

func init() {
	Register(&Migration011RenameClassicThemes{})
}

// RenamedThemes maps the old codes of the built-in default themes to the new ones
var RenamedThemes = map[string]string{
	"BC-LIGHT": "CLASSIC-LIGHT",
	"BC-DARK":  "CLASSIC-DARK",
}

// Migration011RenameClassicThemes renames the codes of the built-in default themes in the
// users' appearance settings (User_Personalization.light_theme / dark_theme).
type Migration011RenameClassicThemes struct{}

func (m *Migration011RenameClassicThemes) Version() int {
	return 11
}

func (m *Migration011RenameClassicThemes) Name() string {
	return "rename_classic_themes"
}

func (m *Migration011RenameClassicThemes) Description() string {
	return "Built-in default themes renamed to Classic Light / Classic Dark"
}

func (m *Migration011RenameClassicThemes) Up(ctx *fmigrations.Context) error {
	const table = "User_Personalization"
	exists, err := ctx.TableExists(table)
	if err != nil || !exists {
		return err
	}
	for old, code := range RenamedThemes {
		for _, column := range []string{"light_theme", "dark_theme"} {
			stmt := fmt.Sprintf(`UPDATE "%s" SET %s = '%s' WHERE %s = '%s'`, table, column, code, column, old)
			err := ctx.ExecuteSQL(stmt)
			if err != nil {
				return fmt.Errorf("%s.%s: %w", table, column, err)
			}
		}
	}
	return nil
}
