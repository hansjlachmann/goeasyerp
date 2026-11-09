package migrations

import (
	fmigrations "github.com/hansjlachmann/goeasyerp/backend/foundation/migrations"
)

func init() {
	Register(&Migration055RenumberSMTPSetupPage{})
}

// RenumberedPages055: the SMTP Setup list moved from 409 to 910 (the base application's own
// pages are in 900–999); 409 is G/L Balance by Dimension 1, the classic form 409
var RenumberedPages055 = map[int]int{409: 910}

// Migration055RenumberSMTPSetupPage moves the users' saved layouts of the SMTP Setup list
// (User_Preferences.page_id) to its new number
type Migration055RenumberSMTPSetupPage struct{}

func (m *Migration055RenumberSMTPSetupPage) Version() int {
	return 55
}

func (m *Migration055RenumberSMTPSetupPage) Name() string {
	return "renumber_smtp_setup_page"
}

func (m *Migration055RenumberSMTPSetupPage) Description() string {
	return "SMTP Setup list renumbered from page 409 to 910: saved layouts follow"
}

func (m *Migration055RenumberSMTPSetupPage) Up(ctx *fmigrations.Context) error {
	return renumber(ctx, "User_Preferences", "page_id", RenumberedPages055)
}
