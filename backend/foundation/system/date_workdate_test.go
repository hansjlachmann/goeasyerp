package system

import (
	"testing"

	"github.com/hansjlachmann/goeasyerp/backend/foundation/codeunits"
	"github.com/hansjlachmann/goeasyerp/backend/foundation/types"
)

// The work date is the user's (My Settings), else today; a scheduled job's is today
func TestWorkDateOfTheUser(t *testing.T) {
	if !WorkDate().Equal(types.Today()) {
		t.Errorf("without a user: %v", WorkDate())
	}
	codeunits.RegionalLoader = func(userID string) codeunits.Regional {
		d, _ := types.NewDateFromString("2026-09-05")
		return codeunits.Regional{WorkDate: d}
	}
	defer func() { codeunits.RegionalLoader = nil }()
	codeunits.SetCurrentContext("HANS", "Hans", "cronus", "nb-NO")
	defer codeunits.ClearCurrentContext()
	if WorkDate().String() != "2026-09-05" {
		t.Errorf("the user's work date: %v", WorkDate())
	}
	codeunits.SetCurrentWorkDate(types.Date{})
	if !WorkDate().Equal(types.Today()) {
		t.Errorf("reset to today: %v", WorkDate())
	}
}
