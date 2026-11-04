package migrations

import (
	"fmt"

	fmigrations "github.com/hansjlachmann/goeasyerp/backend/foundation/migrations"
)

func init() {
	Register(&Migration054GoEasyERPThemes{})
}

// BrandThemes maps the codes of the former default themes (Classic Light/Dark) to the
// GoEasyERP themes that are the defaults now
var BrandThemes = map[string]string{
	"CLASSIC-LIGHT": "GOEASYERP-LIGHT",
	"CLASSIC-DARK":  "GOEASYERP-DARK",
}

// Migration054GoEasyERPThemes goes with the GoEasyERP brand colours becoming the default look:
// users on the former default themes (their appearance settings, User_Personalization
// light_theme / dark_theme) move to GoEasyERP Light/Dark once; Classic stays a built-in theme
// they can choose again.
type Migration054GoEasyERPThemes struct{}

func (m *Migration054GoEasyERPThemes) Version() int {
	return 54
}

func (m *Migration054GoEasyERPThemes) Name() string {
	return "goeasyerp_themes"
}

func (m *Migration054GoEasyERPThemes) Description() string {
	return "users on the Classic themes move to the GoEasyERP themes (the new defaults)"
}

func (m *Migration054GoEasyERPThemes) Up(ctx *fmigrations.Context) error {
	const table = "User_Personalization"
	exists, err := ctx.TableExists(table)
	if err != nil || !exists {
		return err
	}
	for old, code := range BrandThemes {
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
