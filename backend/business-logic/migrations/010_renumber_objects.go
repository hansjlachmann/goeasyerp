package migrations

import (
	"fmt"
	"sort"
	"strings"

	fmigrations "github.com/hansjlachmann/goeasyerp/backend/foundation/migrations"
)

func init() {
	Register(&Migration010RenumberObjects{})
}

// RenumberedCodeunits and RenumberedPages map the old object numbers to the new ones: the base
// application uses its number series (application.yaml: objects 1–999) with the classic numbers
// where such an object exists. Tables were renumbered too; their numbers are not stored.
var (
	RenumberedCodeunits = map[int]int{
		50010: 900, // Generate Ledger Entries
		50100: 901, // Create Demo Data
		50110: 902, // Verify SIFT
		50022: 903, // NAV Report Runner
		50020: 990, // Hello World
		50021: 991, // Progress Demo
	}
	RenumberedPages = map[int]int{
		26: 4,   // Payment Terms (form 4)
		12: 10,  // Countries (form 10)
		11: 357, // Companies (form 357)
		30: 59,  // Users (form 59)
		10: 900, // Menus
		31: 901, // User Card
		32: 902, // User Roles
		33: 903, // User Members
		34: 904, // User Role Card
		35: 905, // Permissions
		36: 906, // Themes
	}
)

// Migration010RenumberObjects moves the stored object numbers to the renumbered objects: the
// codeunit a Job Queue entry runs (object_id_to_run, every company) and the page a user's
// saved list layout belongs to (User_Preferences.page_id).
type Migration010RenumberObjects struct{}

func (m *Migration010RenumberObjects) Version() int {
	return 10
}

func (m *Migration010RenumberObjects) Name() string {
	return "renumber_objects"
}

func (m *Migration010RenumberObjects) Description() string {
	return "Codeunits and pages renumbered into the base application's series: Job Queue and saved layouts follow"
}

func (m *Migration010RenumberObjects) Up(ctx *fmigrations.Context) error {
	err := ctx.ForEachCompanyTable("Job_Queue", func(table string) error {
		return renumber(ctx, table, "object_id_to_run", RenumberedCodeunits)
	})
	if err != nil {
		return err
	}
	return renumber(ctx, "User_Preferences", "page_id", RenumberedPages)
}

// renumber changes the numbers in a column of a table (nothing when the table does not exist
// yet). Old and new numbers overlap (page 10 becomes 900, page 12 becomes 10), and page_id is
// part of a primary key: the old numbers are first moved out of the way, then to the new ones.
func renumber(ctx *fmigrations.Context, table, column string, numbers map[int]int) error {
	exists, err := ctx.TableExists(table)
	if err != nil || !exists {
		return err
	}
	const parked = 1000000000
	olds := make([]int, 0, len(numbers))
	for old := range numbers {
		olds = append(olds, old)
	}
	sort.Ints(olds)
	in := make([]string, len(olds))
	cases := make([]string, len(olds))
	for i, old := range olds {
		in[i] = fmt.Sprint(old)
		cases[i] = fmt.Sprintf("WHEN %d THEN %d", old+parked, numbers[old])
	}
	park := fmt.Sprintf(`UPDATE "%s" SET %s = %s + %d WHERE %s IN (%s)`, table, column, column, parked, column, strings.Join(in, ", "))
	err = ctx.ExecuteSQL(park)
	if err != nil {
		return fmt.Errorf("%s.%s: %w", table, column, err)
	}
	move := fmt.Sprintf(`UPDATE "%s" SET %s = CASE %s %s END WHERE %s >= %d`, table, column, column, strings.Join(cases, " "), column, parked)
	err = ctx.ExecuteSQL(move)
	if err != nil {
		return fmt.Errorf("%s.%s: %w", table, column, err)
	}
	return nil
}
