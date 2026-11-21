package tables

import (
	"strings"

	fcodeunits "github.com/hansjlachmann/goeasyerp/backend/foundation/codeunits"
	"github.com/hansjlachmann/goeasyerp/backend/foundation/session"
	ftables "github.com/hansjlachmann/goeasyerp/backend/foundation/tables"
	gtables "github.com/hansjlachmann/goeasyerp/backend/generated/tables"
)

// Users, their roles and the roles' permissions are administered by SUPER users only, so a
// user who may write one of these tables cannot give themselves more rights: a role in a
// company is given or removed by a user who is SUPER there (blank company: in all companies),
// roles and permissions — they hold in every company — by a user who is SUPER in all
// companies, and another user is changed (password, active, …) by a user who is SUPER
// wherever that user has roles. Code without a user (setup, migrations, tests) may.

// superIn reports whether the current user is SUPER in company — a SUPER role for that company
// or for all companies; company "" asks for all companies
func superIn(src ftables.Source, company string) bool {
	access := fcodeunits.CurrentAccess()
	if access == nil {
		return true
	}
	userMember := gtables.NewUserMember(src)
	userMember.SetRange("user_id", strings.ToUpper(fcodeunits.CurrentUserID()))
	userMember.SetRange("role_id", session.SuperRole)
	userMember.SetRange("company", "")
	if userMember.FindFirst() {
		return true
	}
	if company == "" {
		return false
	}
	userMember.SetRange("company", company)
	return userMember.FindFirst()
}

// isCurrentUser reports whether userID is the user working (no user: internal code)
func isCurrentUser(userID string) bool {
	return strings.EqualFold(userID, fcodeunits.CurrentUserID())
}

// canAdministerUser reports whether the current user may change another user: SUPER in every
// company where that user has a role (a user without roles: a SUPER user)
func canAdministerUser(src ftables.Source, userID string) bool {
	access := fcodeunits.CurrentAccess()
	if access == nil {
		return true
	}
	// the companies first: a query inside the loop would wait for the loop's connection
	userMember := gtables.NewUserMember(src)
	userMember.SetRange("user_id", strings.ToUpper(userID))
	var companies []string
	for found := userMember.FindSet(); found; found = userMember.Next() {
		companies = append(companies, userMember.Company.String())
	}
	if len(companies) == 0 {
		return access.IsSuper
	}
	for _, company := range companies {
		if !superIn(src, company) {
			return false
		}
	}
	return true
}
