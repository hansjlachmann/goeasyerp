package migrations

import (
	fmigrations "github.com/hansjlachmann/goeasyerp/backend/foundation/migrations"
)

func init() {
	Register(&Migration059SuperForUsersWithoutRoles{})
}

// Migration059SuperForUsersWithoutRoles goes with "a user without roles may do nothing" (before:
// everything). An installation without any SUPER member would otherwise lock everybody out
// who has no roles — often the administrator: those users become SUPER in every company, which
// is what they could do before. With a SUPER member there is an administrator to give roles,
// and users without roles get no access.
type Migration059SuperForUsersWithoutRoles struct{}

func (m *Migration059SuperForUsersWithoutRoles) Version() int {
	return 59
}

func (m *Migration059SuperForUsersWithoutRoles) Name() string {
	return "super_for_users_without_roles"
}

func (m *Migration059SuperForUsersWithoutRoles) Description() string {
	return "Users without roles become SUPER while the installation has no SUPER member"
}

func (m *Migration059SuperForUsersWithoutRoles) Up(ctx *fmigrations.Context) error {
	for _, table := range []string{"User", "User_Member"} {
		exists, err := ctx.TableExists(table)
		if err != nil || !exists {
			return err
		}
	}
	return ctx.ExecuteSQL(`INSERT INTO "User_Member" (user_id, role_id, company)
		SELECT u.user_id, 'SUPER', '' FROM "User" u
		WHERE NOT EXISTS (SELECT 1 FROM "User_Member" s WHERE UPPER(s.role_id) = 'SUPER')
			AND NOT EXISTS (SELECT 1 FROM "User_Member" m WHERE m.user_id = u.user_id)`)
}
