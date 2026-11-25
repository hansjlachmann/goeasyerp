package apps

import (
	fcodeunits "github.com/hansjlachmann/goeasyerp/backend/foundation/codeunits"
	"github.com/hansjlachmann/goeasyerp/backend/foundation/securitylog"
)

// recordAdministration notes an app action only SUPER users may take as a security event (S48):
// the user and company of the run, what was done
func recordAdministration(details string) {
	securitylog.Record(securitylog.Event{Kind: securitylog.Administration, UserID: fcodeunits.CurrentUserID(),
		Company: fcodeunits.CurrentCompany(), Details: details})
}

// recordRefused notes a SUPER-only app action refused to another user
func recordRefused(details string) {
	securitylog.Record(securitylog.Event{Kind: securitylog.AccessRefused, UserID: fcodeunits.CurrentUserID(),
		Company: fcodeunits.CurrentCompany(), Details: details})
}
