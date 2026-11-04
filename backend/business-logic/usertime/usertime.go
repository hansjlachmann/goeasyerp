// Package usertime registers the time users work in a company (the classic log in/log out of
// codeunit 1, LogInEnd): with Register Time in the user's User Setup — or, without one, in the
// General Ledger Setup — the minutes from logging in to logging out are added to the User Time
// Register (table 51) of the login date.
package usertime

import (
	"math"
	"time"

	ftables "github.com/hansjlachmann/goeasyerp/backend/foundation/tables"
	"github.com/hansjlachmann/goeasyerp/backend/foundation/types"
	gtables "github.com/hansjlachmann/goeasyerp/backend/generated/tables"
)

// Register (LogInEnd) adds the minutes of a session that ended to the user's time register —
// when the user registers time; at least one minute
func Register(src ftables.Source, userID string, loginAt, logoutAt time.Time) error {
	if userID == "" || loginAt.IsZero() {
		return nil
	}
	registerTime := false
	userSetup := gtables.NewUserSetup(src)
	if userSetup.Get(types.Code(userID)) {
		registerTime = userSetup.Register_time
	} else {
		registerTime = gtables.GetGeneralLedgerSetup(src).Register_time
	}
	if !registerTime {
		return nil
	}

	minutes := 0
	if logoutAt.After(loginAt) {
		minutes = int(math.Round(logoutAt.Sub(loginAt).Minutes()))
	}
	if minutes == 0 {
		minutes = 1
	}
	loginDate := loginAt.Local()
	userTimeRegister := gtables.NewUserTimeRegister(src)
	userTimeRegister.Init()
	userTimeRegister.User_id = types.Code(userID)
	userTimeRegister.Date = types.NewDate(loginDate.Year(), loginDate.Month(), loginDate.Day())
	if userTimeRegister.GetByPK(userTimeRegister.User_id, userTimeRegister.Date) {
		userTimeRegister.Minutes = userTimeRegister.Minutes.Add(types.NewDecimalFromInt(int64(minutes)))
		return userTimeRegister.ModifyOrError(false)
	}
	userTimeRegister.Minutes = types.NewDecimalFromInt(int64(minutes))
	return userTimeRegister.InsertOrError(false)
}
